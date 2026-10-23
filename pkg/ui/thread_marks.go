package ui

import (
	"math"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/render"
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/srf"
)

// A cosmetic thread has no turns of its own, so the window draws them
// on its part: a thin helix per start on the crest's cylinder, the
// part's surface there, a little out of the material so that the
// shaded faces, meshed inside the true cylinder by up to the chord
// tolerance, don't hide it. It costs a polyline, against the thousands
// of triangles of a modelled thread.

// threadMarkSides are the segments of a turn of a cosmetic thread's
// helix.
const threadMarkSides = 24

// cosmeticThreadMarks are the helices of the cosmetic threads of the
// solid of group g.
func cosmeticThreadMarks(sk *sketch.Sketch, g *sketch.Group) srf.SEdgeList {
	var el srf.SEdgeList
	for _, b := range sk.ThreadBands(g.H) {
		if !b.Cosmetic {
			continue
		}
		o, err := sk.ThreadOf(b.Helix)
		if err != nil {
			continue
		}
		lead := o.Spec.Lead()
		off := sk.Tol().ChordTolMm() + 0.01*o.Spec.Pitch
		r := b.Crest + off
		if b.Internal {
			r = b.Crest - off
		}
		hand := 1.0
		if o.Spec.LeftHand {
			hand = -1
		}
		u, v := b.Axis.NormalDir(0), b.Axis.NormalDir(1)
		at := func(a, t float64) geom.Vector {
			return b.Origin.Plus(u.ScaledBy(r * math.Cos(a))).Plus(v.ScaledBy(r * math.Sin(a))).Plus(b.Axis.ScaledBy(t))
		}
		n := max(1, int(math.Ceil(b.Length/lead*threadMarkSides)))
		starts := o.Spec.NumStarts()
		for s := range starts {
			a0 := 2 * math.Pi * float64(s) / float64(starts)
			prev := at(a0, 0)
			for i := 1; i <= n; i++ {
				t := b.Length * float64(i) / float64(n)
				p := at(a0+hand*2*math.Pi*t/lead, t)
				el.AddEdge(prev, p, core.StyleSolidEdge, 0, 0)
				prev = p
			}
		}
	}
	return el
}

// drawCosmeticThreads draws the turns of the cosmetic threads of the
// solid of group g, thinner than its edges.
func (gw *graphicsWindow) drawCosmeticThreads(g *sketch.Group, canvas render.Canvas) {
	el := cosmeticThreadMarks(gw.sk(), g)
	if len(el.Edges) == 0 {
		return
	}
	stroke := gw.ss.styleStroke(hstyle(core.StyleSolidEdge))
	stroke.Width /= 2
	stroke.ZIndex = 1
	canvas.DrawEdges(&el, canvas.InternStroke(stroke))
}
