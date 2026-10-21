package ui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/script/api"
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/srf"
)

// threadedModel is a rod 8 mm across and 30 long with an M8 thread 12
// long at its foot, standing on the XY plane, and a nut beside it.
func threadedModel(t *testing.T, internal bool) (*Application, *api.Doc) {
	t.Helper()
	ss, _ := newTestApp(t)
	doc := api.NewDoc(ss.ScriptHost())
	if internal {
		outside := doc.Sketch.Circle(api.V(0, 0, 0), 7)
		doc.Features.Extrude(10, "nut")
		doc.Features.Thread(outside, api.ThreadOptions{Designation: "M8", Internal: true, Through: true, Length: 10})
	} else {
		rod := doc.Sketch.Circle(api.V(0, 0, 0), 4)
		doc.Features.Extrude(30, "rod")
		doc.Features.Thread(rod, api.ThreadOptions{Designation: "M8", Length: 12})
	}
	return ss, doc
}

// viewOf is the edges of the drawing's view of kind, by style.
func viewOf(t *testing.T, ss *Application, kind string, d *sketch.Drawing) map[int]srf.SEdgeList {
	t.Helper()
	r, u, _ := sketch.ViewDirection(kind)
	g := ss.SK.Group(ss.GW.activeGroupH())
	el := ss.viewEdges(g, vec(r), vec(u), d.Hidden, d.ModelledThreads)
	out := map[int]srf.SEdgeList{}
	for _, e := range el.Edges {
		l := out[e.AuxA]
		l.Edges = append(l.Edges, e)
		out[e.AuxA] = l
	}
	return out
}

// TestDrawingThreadConvention draws a screw and a nut by the drafting
// convention: in a view across the axis the screw's crest is two solid
// lines and its root two thin ones, with no edge of the ridge's turns
// left; along the axis the root is three quarters of a thin circle. A
// nut's root is hidden in the part. ModelledThreads draws the turns.
func TestDrawingThreadConvention(t *testing.T) {
	ss, doc := threadedModel(t, false)
	d := doc.Drawing.Get()
	d.Hidden = true
	doc.Drawing.Set(d)

	// The top view looks across the axis, which runs down it from the
	// rod's foot; the front view looks along it.
	across := viewOf(t, ss, "top", &d)
	thin := across[core.StyleDimSolid]
	if len(thin.Edges) != 2 {
		t.Errorf("top view: %d thin edges, want the root's two lines", len(thin.Edges))
	}
	for _, e := range thin.Edges {
		// At x = ±3.32 mm, the root's radius, 12 long.
		if dx := e.B.X - e.A.X; dx*dx > 1e-12 || abs(abs(e.A.X)-3.3234) > 1e-3 || abs(abs(e.B.Y-e.A.Y)-12) > 1e-6 {
			t.Errorf("a root line from %v to %v", e.A, e.B)
		}
	}
	// No solid edge but the thread's ends crosses the crest's outline
	// over the thread: the ridge's turns are gone.
	for _, e := range across[core.StyleSolidEdge].Edges {
		mid := e.A.Plus(e.B).ScaledBy(0.5)
		if abs(mid.X) < 3.9 && mid.Y < -0.1 && mid.Y > -11.9 {
			t.Errorf("an edge of the ridge from %v to %v", e.A, e.B)
		}
	}

	along := viewOf(t, ss, "front", &d)
	if n := len(along[core.StyleDimSolid].Edges); n != threadSides*3/4 {
		t.Errorf("front view: %d thin edges, want the root's three quarters, %d", n, threadSides*3/4)
	}

	d.ModelledThreads = true
	modelled := viewOf(t, ss, "top", &d)
	if len(modelled[core.StyleDimSolid].Edges) != 0 ||
		len(modelled[core.StyleSolidEdge].Edges) < 4*len(across[core.StyleSolidEdge].Edges) {
		t.Errorf("modelled: %d solid edges, %d by the convention", len(modelled[core.StyleSolidEdge].Edges),
			len(across[core.StyleSolidEdge].Edges))
	}

	nss, ndoc := threadedModel(t, true)
	nd := ndoc.Drawing.Get()
	nd.Hidden = true
	nut := viewOf(t, nss, "top", &nd)
	if len(nut[core.StyleDimSolid].Edges) != 0 || len(nut[core.StyleHiddenEdge].Edges) == 0 {
		t.Errorf("nut: %d thin edges in sight and %d hidden, want its root hidden",
			len(nut[core.StyleDimSolid].Edges), len(nut[core.StyleHiddenEdge].Edges))
	}
	if n := len(viewOf(t, nss, "front", &nd)[core.StyleDimSolid].Edges); n != threadSides*3/4 {
		t.Errorf("nut's front view: %d thin edges, want the root's three quarters", n)
	}

	if dir := os.Getenv("TELESAPR_RENDER_DIR"); dir != "" {
		for name, doc := range map[string]*api.Doc{"screw": doc, "nut": ndoc} {
			dr := doc.Drawing.Get()
			dr.Hidden = true
			doc.Drawing.Set(dr)
			if err := doc.Drawing.Export(filepath.Join(dir, "thread-"+name+".svg")); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
