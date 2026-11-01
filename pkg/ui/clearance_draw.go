package ui

import (
	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/render"
	"github.com/telesapr/telesapr/pkg/res"
	"github.com/telesapr/telesapr/pkg/srf"
)

// How a gap Analyze > Clearance and Walls found is drawn. A gap is
// often a fraction of a millimetre, a few pixels across, and its
// nearest points are wherever the gap is least: for a pin in a bore,
// anywhere along it, as often at the far end, inside the solid, as at
// the near one. A thick line drawn above everything made a dot there
// that looked like a mark on the face in front of it. So the gap is a
// thin line between its two points with a tick across each end, a
// leader from it to its size beside it, and each of them is drawn
// solid where it is seen and dashed where the solid hides it: a gap
// behind a face is a dashed mark with a dashed leader.
//
// A wall check finds the thinnest wall and every point thinner than
// asked for, thousands of them along a thin wall. The thinnest is marked
// as a gap is, with its thickness; the thin points are lines across the
// wall, thin and fainter where the wall's faces hide them, which a line
// through the wall mostly is, so that they shade where the wall is thin
// without covering the thinnest's mark.

// gapMark is a gap drawn on the model: its nearest points on the two
// parts, and its size.
type gapMark struct {
	A, B  geom.Vector
	Label string
}

// The sizes of a gap mark, in pixels.
const (
	gapTick     = 6  // half the length of the tick across each end
	gapLeader   = 28 // how far the leader runs up and to the right
	gapShoulder = 8  // the level part of the leader under the label
)

// clearClearanceMarks takes the marks of the last check off the model.
func (gw *graphicsWindow) clearClearanceMarks() {
	gw.gapMarks = nil
	gw.thinSpots.Clear()
}

// drawClearanceMarks draws the gaps and the thin walls Analyze >
// Clearance and Walls found.
func (gw *graphicsWindow) drawClearanceMarks(canvas render.Canvas) {
	gw.drawThinSpots(canvas, &gw.thinSpots)
	gw.drawGapMarks(canvas)
}

// drawThinSpots draws the lines across the walls thinner than asked
// for: solid where they are seen, faint where the solid hides them.
func (gw *graphicsWindow) drawThinSpots(canvas render.Canvas, spots *srf.SEdgeList) {
	if len(spots.Edges) == 0 {
		return
	}
	seen := gw.ss.styleStroke(hstyle(core.StyleDrawError))
	seen.Unit, seen.Width = render.UnitPX, 1
	seen.ZIndex = 1
	seen.Color = seen.Color.WithAlpha(140)
	hidden := seen
	hidden.Layer = render.LayerOccluded
	hidden.Color = hidden.Color.WithAlpha(50)
	canvas.DrawEdges(spots, canvas.InternStroke(seen))
	canvas.DrawEdges(spots, canvas.InternStroke(hidden))
}

// drawGapMarks draws the gaps, and the thinnest wall, as marks with
// their sizes.
func (gw *graphicsWindow) drawGapMarks(canvas render.Canvas) {
	if len(gw.gapMarks) == 0 {
		return
	}
	ss := gw.ss
	camera := canvas.Camera()

	seen := ss.styleStroke(hstyle(core.StyleDrawError))
	seen.Unit, seen.Width = render.UnitPX, 3
	seen.ZIndex = 2
	hidden := seen
	hidden.Layer = render.LayerOccluded
	hidden.Width = 2
	hidden.StipplePattern = core.StippleDash
	hidden.StippleScale = 4
	label := seen
	label.Layer = render.LayerFront
	label.Width = 1
	hcs := []render.HStroke{canvas.InternStroke(seen), canvas.InternStroke(hidden)}
	hcsLabel := canvas.InternStroke(label)
	line := func(a, b geom.Vector) {
		for _, h := range hcs {
			canvas.DrawLine(a, b, h)
		}
	}

	px := 1 / camera.Scale
	r, u := camera.ProjRight, camera.ProjUp
	h := ss.defaultTextHeight() / camera.Scale
	for _, m := range gw.gapMarks {
		d := m.B.Minus(m.A)
		// The tick is across the gap as it is seen.
		across := r.ScaledBy(-d.Dot(u)).Plus(u.ScaledBy(d.Dot(r)))
		if across.Magnitude() < px {
			// The gap points at the eye, or the parts touch: a cross.
			c1 := r.Plus(u).ScaledBy(gapTick * px)
			c2 := r.Minus(u).ScaledBy(gapTick * px)
			line(m.A.Minus(c1), m.A.Plus(c1))
			line(m.A.Minus(c2), m.A.Plus(c2))
		} else {
			t := across.WithMagnitude(gapTick * px)
			line(m.A, m.B)
			line(m.A.Minus(t), m.A.Plus(t))
			line(m.B.Minus(t), m.B.Plus(t))
		}
		// The leader runs up and towards the middle of the view, so
		// that a mark near the edge keeps its label on the screen.
		mid := m.A.Plus(m.B).ScaledBy(0.5)
		side := r
		if camera.ProjectPoint(mid).U > 0 {
			side = r.ScaledBy(-1)
		}
		elbow := mid.Plus(side.Plus(u).ScaledBy(gapLeader * px))
		end := elbow.Plus(side.ScaledBy(gapShoulder * px))
		line(mid, elbow)
		line(elbow, end)
		at := end.Plus(side.ScaledBy(3 * px))
		if side != r {
			at = at.Minus(r.ScaledBy(res.BuiltinVectorFont().Width(h, m.Label)))
		}
		canvas.DrawVectorText(m.Label, h, at.Minus(u.ScaledBy(h/2)), r, u, hcsLabel)
	}
}
