package ui

import (
	"io"
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/render"
	"github.com/telesapr/telesapr/pkg/srf"
)

// gapCanvas keeps the lines and the text a gap mark draws.
type gapCanvas struct {
	*recordingCanvas
	lines []gapLine
	texts []gapText
	edges []gapEdges
}

type gapEdges struct {
	N      int
	Stroke render.Stroke
}

type gapLine struct {
	A, B   geom.Vector
	Stroke render.Stroke
}

type gapText struct {
	Text   string
	At     geom.Vector
	Stroke render.Stroke
}

func (gc *gapCanvas) DrawLine(a, b geom.Vector, hcs render.HStroke) {
	gc.lines = append(gc.lines, gapLine{a, b, *gc.Stroke(hcs)})
}

func (gc *gapCanvas) DrawVectorText(text string, _ float64, o, _, _ geom.Vector, hcs render.HStroke) {
	gc.texts = append(gc.texts, gapText{text, o, *gc.Stroke(hcs)})
}

func (gc *gapCanvas) DrawEdges(el *srf.SEdgeList, hcs render.HStroke) {
	gc.edges = append(gc.edges, gapEdges{len(el.Edges), *gc.Stroke(hcs)})
}

// A gap is drawn as a line between its two points with a tick across
// each end and a leader to its size, each twice: solid where it is
// seen and dashed where a solid hides it, as a pin's gap at the bottom
// of its bore is from above. Its size is drawn above everything.
func TestGapMarkDrawing(t *testing.T) {
	t.Parallel()
	ss, _ := newTestApp(t)
	a, b := geom.Vector{X: 5, Z: 0}, geom.Vector{X: 4.5, Z: 0}
	ss.GW.gapMarks = []gapMark{{A: a, B: b, Label: "0.50"}}
	gc := &gapCanvas{recordingCanvas: newRecordingCanvas(io.Discard)}
	gc.camera = render.Camera{Scale: 5, ProjRight: geom.Vector{X: 1}, ProjUp: geom.Vector{Y: 1}, Width: 400, Height: 300, PixelRatio: 1}
	ss.GW.drawGapMarks(gc)

	same := func(p, q geom.Vector) bool { return p.Minus(q).Magnitude() < 1e-12 }
	layers := map[render.Layer]int{}
	var across, ticks, leaders int
	for _, l := range gc.lines {
		layers[l.Stroke.Layer]++
		switch {
		case same(l.A, a) && same(l.B, b):
			across++
		case same(l.A.Plus(l.B).ScaledBy(0.5), a) || same(l.A.Plus(l.B).ScaledBy(0.5), b):
			// Across the gap as seen: along the screen's up, 12 px.
			if d := l.B.Minus(l.A); math.Abs(d.X) > 1e-12 || math.Abs(d.Magnitude()-2*gapTick/5.0) > 1e-12 {
				t.Errorf("tick %v to %v", l.A, l.B)
			}
			ticks++
		default:
			leaders++
		}
	}
	if across != 2 || ticks != 4 || leaders != 4 {
		t.Errorf("%d lines across, %d ticks, %d leader lines: %+v", across, ticks, leaders, gc.lines)
	}
	if layers[render.LayerNormal] != 5 || layers[render.LayerOccluded] != 5 {
		t.Errorf("lines by layer %v, want as many seen as hidden", layers)
	}
	for _, l := range gc.lines {
		if l.Stroke.Layer == render.LayerOccluded && l.Stroke.StipplePattern == 0 {
			t.Errorf("a hidden line drawn solid: %+v", l.Stroke)
		}
	}
	if len(gc.texts) != 1 || gc.texts[0].Text != "0.50" || gc.texts[0].Stroke.Layer != render.LayerFront {
		t.Fatalf("label %+v", gc.texts)
	}
	// Right of the middle of the view, the label is left of its mark,
	// towards the middle; left of it, right of its mark.
	if x := gc.texts[0].At.X; x >= b.X {
		t.Errorf("the label of a mark right of the middle at x %g", x)
	}
	ss.GW.gapMarks = []gapMark{{A: a.ScaledBy(-1), B: b.ScaledBy(-1), Label: "0.50"}}
	gc.lines, gc.texts = nil, nil
	ss.GW.drawGapMarks(gc)
	if x := gc.texts[0].At.X; x <= -b.X {
		t.Errorf("the label of a mark left of the middle at x %g", x)
	}

	// Parts that touch are a cross, with "touch" beside it.
	ss.GW.gapMarks = []gapMark{{A: a, B: a, Label: "touch"}}
	gc.lines, gc.texts = nil, nil
	ss.GW.drawGapMarks(gc)
	if len(gc.lines) != 8 || len(gc.texts) != 1 || gc.texts[0].Text != "touch" {
		t.Errorf("touching: %d lines, %+v", len(gc.lines), gc.texts)
	}
}

// The thin spots of a wall check are thin lines across the wall, drawn
// where they are seen and, fainter, where the wall hides them, with no
// label: the thinnest wall alone is a mark with its thickness.
func TestThinSpotDrawing(t *testing.T) {
	t.Parallel()
	ss, _ := newTestApp(t)
	ss.GW.thinSpots.AddEdge(geom.Vector{}, geom.Vector{X: 1.2}, 0, 0, 0)
	ss.GW.thinSpots.AddEdge(geom.Vector{Y: 1}, geom.Vector{X: 1.2, Y: 1}, 0, 0, 0)
	ss.GW.gapMarks = []gapMark{{A: geom.Vector{}, B: geom.Vector{X: 1.2}, Label: "1.20"}}
	gc := &gapCanvas{recordingCanvas: newRecordingCanvas(io.Discard)}
	gc.camera = render.Camera{Scale: 5, ProjRight: geom.Vector{X: 1}, ProjUp: geom.Vector{Y: 1}, Width: 400, Height: 300, PixelRatio: 1}
	ss.GW.drawClearanceMarks(gc)
	if len(gc.edges) != 2 || gc.edges[0].N != 2 || gc.edges[1].N != 2 {
		t.Fatalf("thin spots drawn %+v", gc.edges)
	}
	seen, hidden := gc.edges[0].Stroke, gc.edges[1].Stroke
	if seen.Layer != render.LayerNormal || hidden.Layer != render.LayerOccluded ||
		seen.WidthPx(&gc.camera) != 1 || hidden.Color.Alpha >= seen.Color.Alpha {
		t.Errorf("thin spots seen %+v, hidden %+v", seen, hidden)
	}
	if len(gc.texts) != 1 || gc.texts[0].Text != "1.20" {
		t.Errorf("labels %+v", gc.texts)
	}
	ss.GW.clearClearanceMarks()
	if len(ss.GW.thinSpots.Edges) != 0 || ss.GW.gapMarks != nil {
		t.Error("the marks not cleared")
	}
}
