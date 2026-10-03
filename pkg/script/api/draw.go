package api

import (
	"path/filepath"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/persist/link"
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/ttf"
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
// and two vertical, returned starting with the bottom one and going
// around counter-clockwise. It makes the same requests and constraints
// as the interface's rectangle tool dragged from the top left corner.
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
		// What the interface's rectangle tool makes when the top left
		// corner is clicked and the bottom right one dragged to: line i
		// runs from corner i to corner i-1, corner 2 is the click and
		// corner 0 the drag; the lines are joined, then made vertical,
		// horizontal, vertical, horizontal.
		corner := []Vec{at(ub, va), at(ua, va), at(ua, vb), at(ub, vb)}
		var lns [4]Entity
		for i := range lns {
			lns[i] = d.Line(corner[i], corner[(i+3)%4])
		}
		for i := range lns {
			d.Coincident(lns[i].Start(), lns[(i+1)%4].End())
		}
		for i := range lns {
			if i%2 == 0 {
				d.Vertical(lns[i])
			} else {
				d.Horizontal(lns[i])
			}
		}
		lines = []Entity{lns[1], lns[0], lns[3], lns[2]}
	})
	return lines
}

// box places the four points of a text or an image request: its
// bottom left corner at, height up the active workplane's v axis, and
// as wide as aspect says.
func box(sk *sketch.Sketch, hr core.HRequest, at Vec, height, aspect float64) {
	n := sk.MustGetEntity(sk.ActiveWorkplane()).NormalEntity(sk)
	up, right := n.NormalV(sk).ScaledBy(height), n.NormalU(sk).ScaledBy(height*aspect)
	// The points are the top left, the bottom left, the bottom right
	// and the top right corners.
	for i, p := range []Vec{at.Plus(up), at, at.Plus(right), at.Plus(right).Plus(up)} {
		forceTo(sk, hr.Entity(1+i), p)
	}
}

// Text adds text in a TrueType font in the active workplane, its
// bottom left corner at at, and height high; it is as wide as the
// font makes it. font is the name of a font file the program knows,
// as the text window lists them, or "" for the one built in.
func (d *Doc) Text(text, font string, at Vec, height float64) Entity {
	if height <= 0 {
		fail("text can't be %g high", height)
	}
	if font == "" {
		font = ttf.EmbeddedFont
	}
	return d.request(sketch.RequestTTFText, needWorkplane("text"), func(sk *sketch.Sketch, hr core.HRequest) {
		r := sk.GetRequest(hr)
		r.Str, r.Font = text, font
		aspect := ttf.Fonts.AspectRatio(font, text, false)
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
func (d *Doc) Image(path string, at Vec, height float64) Entity {
	if height <= 0 {
		fail("an image can't be %g high", height)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		fail("%v", err)
	}
	return d.request(sketch.RequestImage, needWorkplane("an image"), func(sk *sketch.Sketch, hr core.HRequest) {
		if !link.LoadLinkedImage(sk, "", abs) {
			fail("can't read the image '%s'", path)
		}
		img := sk.Images[abs]
		r := sk.GetRequest(hr)
		r.File, r.Construction = abs, true
		box(sk, hr, at, height, float64(img.Width)/float64(img.Height))
	})
}

// edit runs one of the engine's edits of the drawing, which keeps its
// own undo step, and fails with what it says.
func (d *Doc) edit(fn func(sk *sketch.Sketch) error) {
	d.h.Run(func() {
		if d.h.Sketch().RegenerationPending() {
			d.h.Regenerate()
		}
		sk := d.h.Sketch()
		sk.Active = sk.ActiveGroup()
		undo := sk.Undo.Write
		err := fn(sk)
		if sk.Undo.Write != undo {
			sk.Unsaved = true
		}
		if err != nil {
			fail("%s", err.Error())
		}
	})
}

// TangentArc rounds the corner at a point where two line segments or
// arcs of the active workplane meet, with an arc tangent to both, as
// Sketch > Tangent Arc at Point does. radius 0 lets the program choose
// it. With trim, the two are cut back to the arc; without, they become
// construction and trimmed copies are added. It is its own step of
// undo.
func (d *Doc) TangentArc(point Entity, radius float64, trim bool) {
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
// it takes the one nearest near. It is its own step of undo.
func (d *Doc) Split(a, b Entity, near Vec) {
	d.edit(func(sk *sketch.Sketch) error {
		a.get(sk)
		b.get(sk)
		sel := []sketch.Selection{{Entity: a.H}, {Entity: b.H}}
		return sk.SplitLinesOrCurves(sel, sk.ProjectPoint(near))
	})
}
