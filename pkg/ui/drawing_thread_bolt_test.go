package ui

import (
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/script/api"
	"github.com/telesapr/telesapr/pkg/srf"
)

// boltModel is an M10 bolt as examples/scripts/bolt makes it: a shank
// 10 mm across and 30 long standing on a hexagon head, threaded 18
// from its free end towards the head, through that end when through,
// which turns the shank down past the thread too, and with a run-out.
func boltModel(t *testing.T, through bool, runOut float64) (*Application, *api.Doc) {
	t.Helper()
	return boltModelWith(t, api.ThreadOptions{Through: through, RunOut: runOut})
}

// boltModelWith is boltModel with the thread's other options o.
func boltModelWith(t *testing.T, o api.ThreadOptions) (*Application, *api.Doc) {
	t.Helper()
	ss, _ := newTestApp(t)
	doc := api.NewDoc(ss.ScriptHost())
	doc.Sketch.Circle(api.V(0, 0, 0), 5)
	rod := doc.Features.Extrude(30, "shank")
	doc.Features.SketchIn(doc.Workplane("XY"), "head-sketch")
	var corners []api.Vec
	for i := range 6 {
		a := math.Pi / 6 * float64(2*i+1)
		r := 8 / math.Cos(math.Pi/6)
		corners = append(corners, api.V(r*math.Cos(a), r*math.Sin(a), 0))
	}
	doc.Sketch.Polyline(true, corners...)
	doc.Features.Extrude(-7, "head")
	var end api.Entity
	for _, e := range rod.Entities() {
		if e.Kind() == "circle" && e.Center().Pos().Z > 15 {
			end = e
		}
	}
	o.Designation, o.Length, o.Reverse = "M10", 18, true
	doc.Features.Thread(end, o)
	d := doc.Drawing.Get()
	d.Hidden = true
	doc.Drawing.Set(d)
	return ss, doc
}

// covers reports whether the edges at x, upright in the view, cover y
// from lo to hi with no gap.
func covers(el srf.SEdgeList, x, lo, hi float64) bool {
	at := lo
	for moved := true; moved && at < hi-1e-6; {
		moved = false
		for _, e := range el.Edges {
			if math.Abs(e.A.X-x) > 1e-2 || math.Abs(e.B.X-x) > 1e-2 {
				continue
			}
			a, b := min(e.A.Y, e.B.Y), max(e.A.Y, e.B.Y)
			if a <= at+1e-6 && b > at+1e-6 {
				at, moved = b, true
			}
		}
	}
	return at >= hi-1e-6
}

// TestDrawingThreadBolt draws a bolt by the convention. Across the
// axis the shank's outline runs on into the crest's with no gap and
// no fragment of the ridge's ends, seen or hidden, about where the
// thread ends at the shoulder; along the axis the root is three
// quarters of a thin circle and nothing else is drawn at its radius.
func TestDrawingThreadBolt(t *testing.T) {
	ss, doc := boltModel(t, false, 0)
	d := doc.Drawing.Get()
	top := viewOf(t, ss, "top", &d)
	// The shank and the thread, from the head at y = 0 to the free end
	// at y = -30.
	for _, x := range []float64{-5, 5} {
		if !covers(top[core.StyleSolidEdge], x, -30, 0) {
			t.Errorf("the outline at x = %g has a gap", x)
		}
	}
	for _, e := range top[core.StyleHiddenEdge].Edges {
		if e.A.Y < -0.1 || e.B.Y < -0.1 {
			t.Errorf("a hidden edge along the shank, from %v to %v", e.A, e.B)
		}
	}
	// Between the root and the crest there is nothing but the thread's
	// end lines, across.
	for _, e := range top[core.StyleSolidEdge].Edges {
		for _, p := range [][2]float64{{e.A.X, e.A.Y}, {e.B.X, e.B.Y}} {
			if x := math.Abs(p[0]); x > 4.1 && x < 4.95 && p[1] < -0.1 && math.Abs(e.A.Y-e.B.Y) > 1e-9 {
				t.Errorf("an edge of the ridge from %v to %v", e.A, e.B)
			}
		}
	}
	if thin := top[core.StyleDimSolid]; len(thin.Edges) != 2 || !covers(thin, 4.188, -30, -12) || !covers(thin, -4.188, -30, -12) {
		t.Errorf("the root: %d thin edges, want two the thread's length", len(thin.Edges))
	}

	front := viewOf(t, ss, "front", &d)
	if n := len(front[core.StyleDimSolid].Edges); n != threadSides*3/4 {
		t.Errorf("front: %d thin edges, want the root's three quarters", n)
	}
	for st, el := range front {
		for _, e := range el.Edges {
			if st != core.StyleDimSolid && math.Abs(math.Hypot(e.A.X, e.A.Y)-4.188) < 0.05 {
				t.Errorf("front: an edge in style %d at the root's radius, from %v to %v", st, e.A, e.B)
				break
			}
		}
	}
}

// TestDrawingThreadThrough draws a bolt whose thread runs through its
// far end: the shank is turned down to the root past the thread, a
// groove a pitch long that the drawing shows as it is, and the turned
// surface, which the mesh spans from the groove into the thread, is
// not drawn over the thread.
func TestDrawingThreadThrough(t *testing.T) {
	ss, doc := boltModel(t, true, 0)
	d := doc.Drawing.Get()
	top := viewOf(t, ss, "top", &d)
	for _, e := range top[core.StyleHiddenEdge].Edges {
		if e.A.Y < -0.1 || e.B.Y < -0.1 {
			t.Errorf("a hidden edge along the shank, from %v to %v", e.A, e.B)
		}
	}
	groove := 0
	for _, e := range top[core.StyleSolidEdge].Edges {
		if math.Abs(math.Abs(e.A.X)-4.188) > 1e-3 || math.Abs(e.B.X-e.A.X) > 1e-9 {
			continue
		}
		if min(e.A.Y, e.B.Y) < -12-1e-6 {
			t.Errorf("the turned surface drawn over the thread, from %v to %v", e.A, e.B)
		}
		groove++
	}
	if groove != 2 {
		t.Errorf("%d lines of the groove, want two", groove)
	}
	for _, x := range []float64{-5, 5} {
		if !covers(top[core.StyleSolidEdge], x, -30, -12) || !covers(top[core.StyleSolidEdge], x, -10.5, 0) {
			t.Errorf("the outline at x = %g has a gap", x)
		}
	}
}

// TestDrawingThreadRunOutEnd draws a thread with a run-out: a thick
// line across where the full thread ends, the root's lines slanting
// out from there to the crest at the shoulder.
func TestDrawingThreadRunOutEnd(t *testing.T) {
	ss, doc := boltModel(t, false, 3)
	d := doc.Drawing.Get()
	top := viewOf(t, ss, "top", &d)
	across := 0.0
	for _, e := range top[core.StyleSolidEdge].Edges {
		if math.Abs(e.A.Y+15) < 1e-6 && math.Abs(e.B.Y+15) < 1e-6 {
			across += math.Abs(e.B.X - e.A.X)
		}
	}
	if math.Abs(across-10) > 0.05 {
		t.Errorf("the end of the full thread is %g long across, want the crest's 10", across)
	}
	slants := 0
	for _, e := range top[core.StyleDimSolid].Edges {
		if math.Abs(e.A.Y-e.B.Y) > 1e-6 && math.Abs(e.A.X-e.B.X) > 0.5 {
			slants++
		}
	}
	if slants != 2 {
		t.Errorf("%d slanting root lines, want two", slants)
	}
}

// TestDrawingThreadWeights writes the bolt's drawing and checks the
// weights of its lines: the outlines thick, a thread's root thin, as
// ISO 6410 has them, in SVG and DXF alike.
func TestDrawingThreadWeights(t *testing.T) {
	_, doc := boltModel(t, false, 0)
	dir := t.TempDir()
	svg := filepath.Join(dir, "bolt.svg")
	if err := doc.Drawing.Export(svg); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(svg)
	if err != nil {
		t.Fatal(err)
	}
	width := func(class string) string {
		m := regexp.MustCompile(`\.` + class + ` \{[^}]*stroke-width:([0-9.]+);`).FindSubmatch(b)
		if m == nil {
			return ""
		}
		return string(m[1])
	}
	// SolidEdge is style 5, DimSolid 13 (d), HiddenEdge 14 (e), Outline
	// 15 (f).
	for class, want := range map[string]string{"s5": "0.350000", "sd": "0.180000", "se": "0.180000", "sf": "0.500000"} {
		if got := width(class); got != want {
			t.Errorf("SVG: .%s is %s wide, want %s", class, got, want)
		}
	}
	if !strings.Contains(string(b), "class='sd'") {
		t.Error("SVG: no thin lines")
	}

	dxf := filepath.Join(dir, "bolt.dxf")
	if err := doc.Drawing.Export(dxf); err != nil {
		t.Fatal(err)
	}
	if b, err = os.ReadFile(dxf); err != nil {
		t.Fatal(err)
	}
	weights := map[string]bool{}
	lines := strings.Split(strings.ReplaceAll(string(b), "\r\n", "\n"), "\n")
	for i := 0; i+1 < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "370" {
			weights[strings.TrimSpace(lines[i+1])] = true
		}
	}
	for _, w := range []string{"35", "18", "50"} {
		if !weights[w] {
			t.Errorf("DXF: no line %s hundredths of a mm wide, has %v", w, weights)
		}
	}
}
