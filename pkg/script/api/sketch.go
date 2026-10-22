package api

import (
	"path/filepath"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/persist/link"
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/ttf"
)

// SketchMenu holds the calls of the Sketch menu: what is drawn in the
// active group, and the workplane it is drawn in. A script reaches it
// as doc.Sketch.
type SketchMenu struct{ d *Doc }

// ActiveWorkplane is the workplane that is drawn in; it isn't Valid
// when drawing freely in 3d.
func (m SketchMenu) ActiveWorkplane() (wp Entity) {
	d := m.d
	d.read(func(sk *sketch.Sketch) {
		wp.d = d
		if h := sk.ActiveWorkplane(); !core.IsFreeIn3D(h) {
			wp.H = h
		}
	})
	return wp
}

// InWorkplane makes wp the workplane that is drawn in, in the active
// group.
func (m SketchMenu) InWorkplane(wp Entity) {
	d := m.d
	d.read(func(sk *sketch.Sketch) {
		if !wp.get(sk).IsWorkplane() {
			fail("%s isn't a workplane", wp.describe(sk))
		}
		sk.Group(sk.ActiveGroup()).ActiveWorkplane = wp.H
	})
}

// Anywhere3D draws freely in 3d, in no workplane, in the active group.
func (m SketchMenu) Anywhere3D() {
	d := m.d
	d.read(func(sk *sketch.Sketch) {
		sk.Group(sk.ActiveGroup()).ActiveWorkplane = core.HEntity{}
	})
}

// UV is the point at (u, v) in the active workplane.
func (m SketchMenu) UV(u, v float64) (p Vec) {
	d := m.d
	d.read(func(sk *sketch.Sketch) {
		h := sk.ActiveWorkplane()
		if core.IsFreeIn3D(h) {
			fail("UV needs a workplane to be active")
		}
		wp := sk.MustEntity(h)
		n := wp.NormalEntity(sk)
		p = wp.WorkplaneGetOffset(sk).
			Plus(n.NormalU(sk).ScaledBy(u)).
			Plus(n.NormalV(sk).ScaledBy(v))
	})
	return p
}

// Point adds a datum point at p. Unlike the other drawing calls it may
// be drawn in a group that isn't a sketch, as an extrusion, as the
// window's Datum Point may: a point to measure from or to put a
// dimension on, which changes no solid.
func (m SketchMenu) Point(p Vec) Entity {
	d := m.d
	return d.request(sketch.RequestDatumPoint, nil, func(sk *sketch.Sketch, hr core.HRequest) {
		forceTo(sk, hr.Entity(0), p)
	})
}

// Line adds a line segment from a to b. In a workplane the points are
// put in it.
func (m SketchMenu) Line(a, b Vec) Entity {
	d := m.d
	return d.request(sketch.RequestLineSegment, nil, func(sk *sketch.Sketch, hr core.HRequest) {
		forceTo(sk, hr.Entity(1), a)
		forceTo(sk, hr.Entity(2), b)
	})
}

// Circle adds a circle. In a workplane it lies in it; in 3d it is
// parallel to the XY plane.
func (m SketchMenu) Circle(center Vec, radius float64) Entity {
	d := m.d
	if radius < 0 {
		fail("a circle can't have the radius %g", radius)
	}
	return d.request(sketch.RequestCircle, nil, func(sk *sketch.Sketch, hr core.HRequest) {
		forceTo(sk, hr.Entity(1), center)
		if normal := sk.MustEntity(hr.Entity(32)); normal.Type == core.EntityNormalIn3D {
			normal.NormalForceTo(sk, geom.QuaternionFromBasis(V(1, 0, 0), V(0, 1, 0)))
		}
		sk.MustEntity(hr.Entity(64)).DistanceForceTo(sk, radius)
	})
}

// Arc adds an arc of a circle around center, from start
// counter-clockwise to end, in the active workplane. The end is put
// at the start's distance from the centre.
func (m SketchMenu) Arc(center, start, end Vec) Entity {
	d := m.d
	return d.request(sketch.RequestArcOfCircle, needWorkplane("an arc"),
		func(sk *sketch.Sketch, hr core.HRequest) {
			forceTo(sk, hr.Entity(1), center)
			forceTo(sk, hr.Entity(2), start)
			forceTo(sk, hr.Entity(3), end)
		})
}

// ArcIn3D adds an arc of a circle drawn in 3d, in no workplane: around
// center, from start counter-clockwise to end about the normal u cross
// v, as a DXF import draws one. The window draws an arc only in a
// workplane, whose plane its clicks are in; here u and v give the
// plane, and the arc's normal turns as SetOrientation turns it.
func (m SketchMenu) ArcIn3D(center, start, end, u, v Vec) Entity {
	d := m.d
	checkFinite("an orientation", u)
	checkFinite("an orientation", v)
	if u.Magnitude() == 0 || u.Cross(v).Magnitude() == 0 {
		fail("the directions of an arc's plane must differ")
	}
	u = u.WithMagnitude(1)
	v = v.Minus(u.ScaledBy(v.Dot(u))).WithMagnitude(1)
	return d.request(sketch.RequestArcOfCircle, func(sk *sketch.Sketch) {
		if !core.IsFreeIn3D(sk.ActiveWorkplane()) {
			fail("ArcIn3D draws in 3d; in a workplane, Arc draws the arc")
		}
	}, func(sk *sketch.Sketch, hr core.HRequest) {
		forceTo(sk, hr.Entity(1), center)
		forceTo(sk, hr.Entity(2), start)
		forceTo(sk, hr.Entity(3), end)
		sk.MustEntity(hr.Entity(32)).NormalForceTo(sk, geom.QuaternionFromBasis(u, v))
	})
}

// Cubic adds a cubic Bézier curve from p0 to p3, with the control
// points p1 and p2.
func (m SketchMenu) Cubic(p0, p1, p2, p3 Vec) Entity {
	d := m.d
	return d.request(sketch.RequestCubic, nil, func(sk *sketch.Sketch, hr core.HRequest) {
		for i, p := range []Vec{p0, p1, p2, p3} {
			forceTo(sk, hr.Entity(1+i), p)
		}
	})
}

// Spline adds an open cubic spline with the given points, in order:
// the start, the control point beside it, the points it passes through,
// then the end's control point and the end; at least four and at most
// twelve. With four it is the Bézier curve Cubic draws.
func (m SketchMenu) Spline(points ...Vec) Entity {
	return m.spline(sketch.RequestCubic, 4, points)
}

// PeriodicSpline adds a closed cubic spline through the given points,
// in order, at least three and at most twelve: the curve goes round
// from the last back to the first, as the window's spline does when a
// click lands on its first point again.
func (m SketchMenu) PeriodicSpline(points ...Vec) Entity {
	return m.spline(sketch.RequestCubicPeriodic, 3, points)
}

func (m SketchMenu) spline(typ sketch.RequestType, least int, points []Vec) Entity {
	if len(points) < least || len(points) > core.MaxPointsInEntity {
		fail("a spline of this kind takes %d to %d points, not %d", least, core.MaxPointsInEntity, len(points))
	}
	return m.d.request(typ, nil, func(sk *sketch.Sketch, hr core.HRequest) {
		if err := sk.SetSplinePoints(hr, points); err != nil {
			fail("%v", err)
		}
	})
}

// NewWorkplane adds a workplane through origin, with u to its right
// and v up, to a group drawn freely in 3d.
func (m SketchMenu) NewWorkplane(origin, u, v Vec) Entity {
	d := m.d
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
		sk.MustEntity(hr.Entity(32)).NormalForceTo(sk, geom.QuaternionFromBasis(u, v))
	})
}

// NewWorkplaneIn adds a workplane in the active workplane, as a DXF
// import draws one: its origin is a point of the active workplane,
// which the solver keeps there, and it lies parallel to it, its normal
// the active workplane's. The window draws a workplane only in 3d,
// where NewWorkplane draws one turned any way.
func (m SketchMenu) NewWorkplaneIn(origin Vec) Entity {
	d := m.d
	return d.request(sketch.RequestWorkplane, func(sk *sketch.Sketch) {
		if core.IsFreeIn3D(sk.ActiveWorkplane()) {
			fail("no workplane is active; NewWorkplane draws one in 3d")
		}
	}, func(sk *sketch.Sketch, hr core.HRequest) {
		forceTo(sk, hr.Entity(1), origin)
	})
}

// Polyline adds the line segments through points, each starting
// where the last ended, and back to the first if closed.
func (m SketchMenu) Polyline(closed bool, points ...Vec) (lines []Entity) {
	d := m.d
	if len(points) < 2 || (closed && len(points) < 3) {
		fail("a polyline of %d points", len(points))
	}
	d.Batch(func() {
		n := len(points) - 1
		if closed {
			n = len(points)
		}
		for i := range n {
			lines = append(lines, d.Sketch.Line(points[i], points[(i+1)%len(points)]))
		}
		for i := 1; i < len(lines); i++ {
			d.Constrain.Coincident(lines[i-1].End(), lines[i].Start())
		}
		if closed {
			d.Constrain.Coincident(lines[n-1].End(), lines[0].Start())
		}
	})
	return lines
}

// Rect adds a rectangle with the corners a and b and sides along the
// active workplane's axes: four line segments, joined, two horizontal
// and two vertical, returned starting with the bottom one and going
// around counter-clockwise. It makes the same requests and constraints
// as the interface's rectangle tool dragged from the top left corner.
func (m SketchMenu) Rect(a, b Vec) (lines []Entity) {
	d := m.d
	d.Batch(func() {
		var wp Entity
		if wp = d.Sketch.ActiveWorkplane(); !wp.Valid() {
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
		// What the interface's rectangle tool makes when the top left
		// corner is clicked and the bottom right one dragged to: line i
		// runs from corner i to corner i-1, corner 2 is the click and
		// corner 0 the drag; the lines are joined, then made vertical,
		// horizontal, vertical, horizontal.
		corner := []Vec{at(ub, va), at(ua, va), at(ua, vb), at(ub, vb)}
		var lns [4]Entity
		for i := range lns {
			lns[i] = d.Sketch.Line(corner[i], corner[(i+3)%4])
		}
		for i := range lns {
			d.Constrain.Coincident(lns[i].Start(), lns[(i+1)%4].End())
		}
		for i := range lns {
			if i%2 == 0 {
				d.Constrain.Vertical(lns[i])
			} else {
				d.Constrain.Horizontal(lns[i])
			}
		}
		lines = []Entity{lns[1], lns[0], lns[3], lns[2]}
	})
	return lines
}

// Text adds text in a TrueType font in the active workplane, its
// bottom left corner at at, and height high; it is as wide as the
// font makes it. font is the name of a font file the program knows,
// as the text window lists them, or "" for the one built in.
func (m SketchMenu) Text(text, font string, at Vec, height float64) Entity {
	d := m.d
	if height <= 0 {
		fail("text can't be %g high", height)
	}
	if font == "" {
		font = ttf.EmbeddedFont
	}
	return d.request(sketch.RequestTTFText, needWorkplane("text"), func(sk *sketch.Sketch, hr core.HRequest) {
		r := sk.Request(hr)
		r.Str, r.Font = text, font
		aspect := d.h.Sketch().FontList().AspectRatio(font, text, false)
		if aspect == 0 {
			fail("the font '%s' isn't there, or has none of '%s'", font, text)
		}
		box(sk, hr, at, height, aspect)
	})
}

// Image adds a picture from a file in the active workplane, for
// tracing: its bottom left corner at at, height high and as wide as
// the picture's proportions make it. It is construction, as the
// interface makes it.
func (m SketchMenu) Image(path string, at Vec, height float64) Entity {
	d := m.d
	if height <= 0 {
		fail("an image can't be %g high", height)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		fail("%v", err)
	}
	if err := d.checkFile(path, false); err != nil {
		fail("%v", err)
	}
	return d.request(sketch.RequestImage, needWorkplane("an image"), func(sk *sketch.Sketch, hr core.HRequest) {
		if !link.LoadLinkedImage(sk, "", abs) {
			fail("can't read the image '%s'", path)
		}
		img := sk.Images[abs]
		r := sk.Request(hr)
		r.File, r.Construction = abs, true
		box(sk, hr, at, height, float64(img.Width)/float64(img.Height))
	})
}

// ImageFrame adds a picture as Image does, but sized by the caller: its
// bottom left corner at at, height high and width wide. Its pixels are
// read from path when the file is there, and then its own proportions
// hold; when it isn't, the frame stays width wide with the path, as
// opening a drawing whose picture has gone keeps it, and File > Export
// > Script writes such a picture with this.
func (m SketchMenu) ImageFrame(path string, at Vec, height, width float64) Entity {
	d := m.d
	if height <= 0 || width <= 0 {
		fail("an image can't be %g by %g", width, height)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		fail("%v", err)
	}
	if err := d.checkFile(path, false); err != nil {
		fail("%v", err)
	}
	return d.request(sketch.RequestImage, needWorkplane("an image"), func(sk *sketch.Sketch, hr core.HRequest) {
		aspect := width / height
		if link.LoadLinkedImage(sk, "", abs) {
			img := sk.Images[abs]
			aspect = float64(img.Width) / float64(img.Height)
		}
		r := sk.Request(hr)
		r.File, r.Construction, r.AspectRatio = abs, true, aspect
		box(sk, hr, at, height, aspect)
	})
}

// TangentArc rounds the corner at a point where two line segments or
// arcs of the active workplane meet, with an arc tangent to both, as
// Sketch > Tangent Arc at Point does. radius 0 lets the program choose
// it. With trim, the two are cut back to the arc; without, they become
// construction and trimmed copies are added.
func (m SketchMenu) TangentArc(point Entity, radius float64, trim bool) {
	d := m.d
	if radius < 0 {
		fail("a tangent arc can't have the radius %g", radius)
	}
	d.edit(func(sk *sketch.Sketch) error {
		h := point.must(sk, "a point", isPoint)
		return sk.MakeTangentArc(h, sketch.TangentArcSettings{Manual: radius > 0, Radius: radius, Modify: trim})
	})
}

// Split cuts two curves where they cross, or a curve at a point on it,
// as Sketch > Split Curves at Intersection does. Of several crossings
// it takes the one nearest near.
func (m SketchMenu) Split(a, b Entity, near Vec) {
	d := m.d
	d.edit(func(sk *sketch.Sketch) error {
		a.get(sk)
		b.get(sk)
		sel := []sketch.Selection{{Entity: a.H}, {Entity: b.H}}
		return sk.SplitLinesOrCurves(sel, sk.ProjectPoint(near))
	})
}
