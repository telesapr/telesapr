package api

import (
	"github.com/spacesolve/spacesolve/pkg/core"
	"github.com/spacesolve/spacesolve/pkg/geom"
	"github.com/spacesolve/spacesolve/pkg/sketch"
)

// request adds a request to the active group, in the active
// workplane, and lets place put its points; nothing is solved until
// they are where they should start.
func (d *Doc) request(typ sketch.RequestType, check func(sk *sketch.Sketch),
	place func(sk *sketch.Sketch, hr core.HRequest)) (e Entity) {
	d.change(func(sk *sketch.Sketch) {
		if g := sk.GetGroup(sk.ActiveGroup()); g == nil ||
			(g.Type != sketch.GroupDrawing3D && g.Type != sketch.GroupDrawingWorkplane) {
			fail("%s isn't a group to draw in; make one with SketchIn or Sketch3D, or Activate one",
				groupDescription(g))
		}
		if check != nil {
			check(sk)
		}
		// What is drawn is placed against the sketch as it is.
		if sk.RegenerationPending() {
			d.h.Regenerate()
			sk = d.h.Sketch()
		}
		sk.Active = sk.ActiveGroup()
		hr := sk.AddRequestOfType(typ, false)
		place(sk, hr)
		e = Entity{d, hr.Entity(0)}
	})
	return e
}

func needWorkplane(what string) func(sk *sketch.Sketch) {
	return func(sk *sketch.Sketch) {
		if core.IsFreeIn3D(sk.ActiveWorkplane()) {
			fail("can't draw %s in 3d; a workplane must be active", what)
		}
	}
}

func forceTo(sk *sketch.Sketch, h core.HEntity, p Vec) {
	sk.MustGetEntity(h).PointForceTo(sk, p)
}

// Point adds a datum point at p.
func (d *Doc) Point(p Vec) Entity {
	return d.request(sketch.RequestDatumPoint, nil, func(sk *sketch.Sketch, hr core.HRequest) {
		forceTo(sk, hr.Entity(0), p)
	})
}

// Line adds a line segment from a to b. In a workplane the points are
// put in it.
func (d *Doc) Line(a, b Vec) Entity {
	return d.request(sketch.RequestLineSegment, nil, func(sk *sketch.Sketch, hr core.HRequest) {
		forceTo(sk, hr.Entity(1), a)
		forceTo(sk, hr.Entity(2), b)
	})
}

// Circle adds a circle. In a workplane it lies in it; in 3d it is
// parallel to the XY plane.
func (d *Doc) Circle(center Vec, radius float64) Entity {
	if radius < 0 {
		fail("a circle can't have the radius %g", radius)
	}
	return d.request(sketch.RequestCircle, nil, func(sk *sketch.Sketch, hr core.HRequest) {
		forceTo(sk, hr.Entity(1), center)
		if normal := sk.MustGetEntity(hr.Entity(32)); normal.Type == core.EntityNormalIn3D {
			normal.NormalForceTo(sk, geom.QuaternionFromBasis(V(1, 0, 0), V(0, 1, 0)))
		}
		sk.MustGetEntity(hr.Entity(64)).DistanceForceTo(sk, radius)
	})
}

// Arc adds an arc of a circle around center, from start
// counter-clockwise to end, in the active workplane. The end is put
// at the start's distance from the centre.
func (d *Doc) Arc(center, start, end Vec) Entity {
	return d.request(sketch.RequestArcOfCircle, needWorkplane("an arc"),
		func(sk *sketch.Sketch, hr core.HRequest) {
			forceTo(sk, hr.Entity(1), center)
			forceTo(sk, hr.Entity(2), start)
			forceTo(sk, hr.Entity(3), end)
		})
}

// Cubic adds a cubic Bézier curve from p0 to p3, with the control
// points p1 and p2.
func (d *Doc) Cubic(p0, p1, p2, p3 Vec) Entity {
	return d.request(sketch.RequestCubic, nil, func(sk *sketch.Sketch, hr core.HRequest) {
		for i, p := range []Vec{p0, p1, p2, p3} {
			forceTo(sk, hr.Entity(1+i), p)
		}
	})
}

// NewWorkplane adds a workplane through origin, with u to its right
// and v up, to a group drawn freely in 3d.
func (d *Doc) NewWorkplane(origin, u, v Vec) Entity {
	if u.Magnitude() == 0 || u.Cross(v).Magnitude() == 0 {
		fail("the directions of a workplane must differ")
	}
	u = u.WithMagnitude(1)
	v = v.Minus(u.ScaledBy(v.Dot(u))).WithMagnitude(1)
	return d.request(sketch.RequestWorkplane, func(sk *sketch.Sketch) {
		if !core.IsFreeIn3D(sk.ActiveWorkplane()) {
			fail("a workplane is drawn in 3d, not in a workplane; see Free3D")
		}
	}, func(sk *sketch.Sketch, hr core.HRequest) {
		forceTo(sk, hr.Entity(1), origin)
		sk.MustGetEntity(hr.Entity(32)).NormalForceTo(sk, geom.QuaternionFromBasis(u, v))
	})
}

// Polyline adds the line segments through points, each starting
// where the last ended, and back to the first if closed.
func (d *Doc) Polyline(closed bool, points ...Vec) (lines []Entity) {
	if len(points) < 2 || (closed && len(points) < 3) {
		fail("a polyline of %d points", len(points))
	}
	d.Batch(func() {
		n := len(points) - 1
		if closed {
			n = len(points)
		}
		for i := 0; i < n; i++ {
			lines = append(lines, d.Line(points[i], points[(i+1)%len(points)]))
		}
		for i := 1; i < len(lines); i++ {
			d.Coincident(lines[i-1].End(), lines[i].Start())
		}
		if closed {
			d.Coincident(lines[n-1].End(), lines[0].Start())
		}
	})
	return lines
}

// Rect adds a rectangle with the corners a and b and sides along the
// active workplane's axes: four line segments, joined, two horizontal
// and two vertical, starting with the bottom one and going around.
func (d *Doc) Rect(a, b Vec) (lines []Entity) {
	d.Batch(func() {
		var wp Entity
		if wp = d.ActiveWorkplane(); !wp.Valid() {
			fail("can't draw a rectangle in 3d; a workplane must be active")
		}
		var origin, u, v Vec
		d.read(func(sk *sketch.Sketch) {
			w := wp.get(sk)
			n := w.NormalEntity(sk)
			origin, u, v = w.WorkplaneGetOffset(sk), n.NormalU(sk), n.NormalV(sk)
		})
		ua, va := a.Minus(origin).Dot(u), a.Minus(origin).Dot(v)
		ub, vb := b.Minus(origin).Dot(u), b.Minus(origin).Dot(v)
		if ua > ub {
			ua, ub = ub, ua
		}
		if va > vb {
			va, vb = vb, va
		}
		at := func(s, t float64) Vec { return origin.Plus(u.ScaledBy(s)).Plus(v.ScaledBy(t)) }
		corners := []Vec{at(ua, va), at(ub, va), at(ub, vb), at(ua, vb)}
		lines = d.Polyline(true, corners...)
		d.Horizontal(lines[0])
		d.Vertical(lines[1])
		d.Horizontal(lines[2])
		d.Vertical(lines[3])
	})
	return lines
}
