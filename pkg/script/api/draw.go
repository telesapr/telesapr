package api

import (
	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// request adds a request to the active group, in the active
// workplane, and lets place put its points; nothing is solved until
// they are where they should start.
func (d *Doc) request(typ sketch.RequestType, check func(sk *sketch.Sketch),
	place func(sk *sketch.Sketch, hr core.HRequest)) (e Entity) {
	d.change(func(sk *sketch.Sketch) {
		if g := sk.Group(sk.ActiveGroup()); g == nil || (typ != sketch.RequestDatumPoint &&
			g.Type != sketch.GroupDrawing3D && g.Type != sketch.GroupDrawingWorkplane) {
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
	sk.MustEntity(h).PointForceTo(sk, p)
}

// box places the four points of a text or an image request: its
// bottom left corner at, height up the active workplane's v axis, and
// as wide as aspect says.
func box(sk *sketch.Sketch, hr core.HRequest, at Vec, height, aspect float64) {
	n := sk.MustEntity(sk.ActiveWorkplane()).NormalEntity(sk)
	up, right := n.NormalV(sk).ScaledBy(height), n.NormalU(sk).ScaledBy(height*aspect)
	// The points are the top left, the bottom left, the bottom right
	// and the top right corners.
	for i, p := range []Vec{at.Plus(up), at, at.Plus(right), at.Plus(right).Plus(up)} {
		forceTo(sk, hr.Entity(1+i), p)
	}
}

// edit runs one of the engine's edits of the drawing, which remembers
// its own undo step (folded into the script's), and fails with what it
// says.
func (d *Doc) edit(fn func(sk *sketch.Sketch) error) {
	d.h.Run(func() {
		if d.h.Sketch().RegenerationPending() {
			d.h.Regenerate()
		}
		sk := d.h.Sketch()
		sk.Active = sk.ActiveGroup()
		undo := sk.Undo.Write
		err := fn(sk)
		if n := sk.UndoPushedSince(undo); n > 0 {
			sk.Unsaved = true
			d.h.Remembered(n)
		}
		if err != nil {
			fail("%s", err.Error())
		}
	})
}
