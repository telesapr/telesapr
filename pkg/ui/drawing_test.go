package ui

import (
	"bytes"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/script/api"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// drawingModel is a 120 x 60 x 40 block with a hole down through it,
// in a new application.
func drawingModel(t *testing.T) (*SolveSpaceUI, *api.Doc) {
	t.Helper()
	ss, _ := newTestApp(t)
	doc := api.NewDoc(ss.ScriptHost())
	doc.Rect(api.V(0, 0, 0), api.V(120, 60, 0))
	doc.Extrude(40, "block")
	doc.SketchIn(doc.Workplane("XY"), "")
	doc.Circle(api.V(30, 30, 0), 8)
	hole := doc.Extrude(40, "hole")
	hole.SetCombine(api.Difference)
	return ss, doc
}

// The standard views (roadmap 18.1): third angle has the top view above
// the front and the right view on its right; first angle the top view
// below and the left view on the right. The scale fitted is the largest
// standard one at which the views fit above the title block.
func TestDrawingLayout(t *testing.T) {
	ss, _ := drawingModel(t)
	for _, c := range []struct {
		first bool
		side  string
		above bool
	}{{false, "right", true}, {true, "left", false}} {
		d := &sketch.Drawing{Sheet: "A3", FirstAngle: c.first}
		_, views, scale, err := ss.DrawingSheet(d)
		if err != nil {
			t.Fatal(err)
		}
		at := map[string]sketch.DrawingView{}
		for _, v := range views {
			at[v.Kind] = v
		}
		front, top, side := at["front"], at["top"], at[c.side]
		if (top.Y > front.Y) != c.above || top.X != front.X || side.Y != front.Y || side.X <= front.X {
			t.Errorf("first angle %v: front %+v, top %+v, %s %+v", c.first, front, top, c.side, side)
		}
		if scale != 1 {
			t.Errorf("first angle %v: scale %g on A3", c.first, scale)
		}
	}
	if _, _, scale, _ := ss.DrawingSheet(&sketch.Drawing{Sheet: "A4"}); scale != 0.5 {
		t.Errorf("scale %g on A4, want 1:2", scale)
	}
	if _, _, _, err := ss.DrawingSheet(&sketch.Drawing{Sheet: "B5"}); err == nil {
		t.Error("a sheet there isn't")
	}
}

// The drawing written as SVG, PDF and DXF: the canvas is the sheet in
// mm, the hole's hidden edges are dashed in the front view, the title
// block holds its fields, and the same drawing writes the same bytes.
func TestExportDrawing(t *testing.T) {
	ss, doc := drawingModel(t)
	d := doc.Drawing()
	d.Title, d.Number, d.Hidden = "BRACKET", "TS-001", true
	doc.SetDrawing(d)
	dir := t.TempDir()
	for _, ext := range []string{"svg", "pdf", "dxf"} {
		path := filepath.Join(dir, "sheet."+ext)
		if err := doc.ExportDrawing(path); err != nil {
			t.Fatal(err)
		}
		first, err := os.ReadFile(path)
		if err != nil || len(first) == 0 {
			t.Fatalf("%s: %v", ext, err)
		}
		if err := doc.ExportDrawing(path); err != nil {
			t.Fatal(err)
		}
		if again, _ := os.ReadFile(path); !bytes.Equal(first, again) {
			t.Errorf("%s: written twice, the bytes differ", ext)
		}
		if ext == "svg" {
			s := string(first)
			if !strings.Contains(s, "width='420.000mm' height='297.000mm'") {
				t.Errorf("the SVG's canvas isn't the A3 sheet: %.300s", s)
			}
			if !strings.Contains(s, "stroke-dasharray") {
				t.Error("no dashed (hidden) edges in the SVG")
			}
		}
	}
	// The sheet has hidden edges, and the frame and title block in the
	// outline style.
	sheet, views, _, err := ss.DrawingSheet(ss.SK.Drawing)
	if err != nil || len(views) != 4 {
		t.Fatalf("%d views, %v", len(views), err)
	}
	hidden, frame := 0, 0
	for _, e := range sheet.Edges {
		switch e.AuxA {
		case core.StyleHiddenEdge:
			hidden++
		case core.StyleOutline:
			frame++
		}
	}
	if hidden == 0 || frame != 8 {
		t.Errorf("%d hidden edges, %d edges of the frame and the title block's box", hidden, frame)
	}
}

// A section of the top view through the hole, and a detail of the hole
// in the front view, twice its size (roadmap 18.3): the section's cut
// face is hatched, its plane drawn dashed on the front and right views,
// the detail's circle on the front view and about the detail.
func TestDrawingSectionDetail(t *testing.T) {
	ss, _ := drawingModel(t)
	plain := &sketch.Drawing{Sheet: "A2", Hidden: false}
	base, _, _, err := ss.DrawingSheet(plain)
	if err != nil {
		t.Fatal(err)
	}
	withSection := plain.Clone()
	withSection.Sections = []sketch.DrawingSection{{View: "top"}}
	sec, _, scale, err := ss.DrawingSheet(withSection)
	if err != nil {
		t.Fatal(err)
	}
	dashed := 0
	for _, e := range sec.Edges {
		if e.AuxA == core.StyleHiddenEdge {
			dashed++
		}
	}
	// No hidden edges are drawn: the two cutting planes are what is
	// dashed.
	if dashed != 2 {
		t.Errorf("%d dashed edges, want the plane on the front and the right view", dashed)
	}
	// The hatch: the cut face, 120 x 40 less the hole's 16, every 3 mm
	// at 45 degrees, is many lines.
	if added := len(sec.Edges) - len(base.Edges); added < 40 {
		t.Errorf("the section adds %d edges", added)
	}
	if scale != 1 {
		t.Errorf("scale %g", scale)
	}

	withDetail := plain.Clone()
	withDetail.Details = []sketch.DrawingDetail{{View: "front", CX: -30, CY: 0, R: 15, Scale: 2}}
	det, _, _, err := ss.DrawingSheet(withDetail)
	if err != nil {
		t.Fatal(err)
	}
	dashed = 0
	for _, e := range det.Edges {
		if e.AuxA == core.StyleHiddenEdge {
			dashed++
		}
	}
	if dashed != 72 {
		t.Errorf("%d dashed edges, want the detail's circle on the front view", dashed)
	}
	for name, bad := range map[string]*sketch.Drawing{
		"section of iso": {Sheet: "A3", Sections: []sketch.DrawingSection{{View: "iso"}}},
		"detail of none": {Sheet: "A3", Details: []sketch.DrawingDetail{{View: "front", R: 0, Scale: 2}}},
	} {
		if _, _, _, err := ss.DrawingSheet(bad); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}

// Dimensions on the front view (roadmap 18.4): the block's top edge
// (120), the distance from its bottom edge to its top (60), the hole's
// diameter (16). They follow the model: the block made wider, the
// edge's length with it. The hole taken away, its dimension can't be
// measured, and says why; the others still can.
func TestDrawingDimensions(t *testing.T) {
	ss, doc := drawingModel(t)
	var block, hole api.Group
	for _, g := range doc.Groups() {
		switch g.Name() {
		case "block":
			block = g
		case "hole":
			hole = g
		}
	}
	solid := hole.Solid()
	var top, bottom, rim api.SolidEdge
	for _, e := range solid.Edges() {
		a, b := e.Ends()
		switch {
		case e.Straight() && a.Z == 40 && b.Z == 40 && a.Y == 60 && b.Y == 60:
			top = e
		case e.Straight() && a.Z == 40 && b.Z == 40 && a.Y == 0 && b.Y == 0:
			bottom = e
		case !e.Straight() && a.Z == 40 && rim.Name() == "edge 0 0 0":
			rim = e
		}
	}
	doc.AddDrawingDimension("front", "length", 10, top)
	doc.AddDrawingDimension("front", "distance", -12, bottom, top)
	doc.AddDrawingDimension("front", "diameter", 0, rim)
	values, errs := doc.DrawingDimensions()
	want := []float64{120, 60, 16}
	for i := range want {
		if errs[i] != nil || math.Abs(values[i]-want[i]) > 1e-6 {
			t.Errorf("dimension %d: %g, %v, want %g", i, values[i], errs[i], want[i])
		}
	}
	sheet, _, _, err := ss.DrawingSheet(ss.SK.Drawing)
	if err != nil || len(sheet.Edges) == 0 {
		t.Fatal(err)
	}

	// Wider: the block's sketch made 150 wide by its constraint-free
	// corner; the length follows.
	_ = block
	for i := range ss.SK.Params {
		p := &ss.SK.Params[i]
		if p.Val == 120 {
			p.Val = 150
		}
	}
	doc.Regen()
	values, errs = doc.DrawingDimensions()
	if errs[0] != nil || math.Abs(values[0]-150) > 1e-6 {
		t.Errorf("after the block grew: %g, %v", values[0], errs[0])
	}

	hole.Delete()
	_, errs = doc.DrawingDimensions()
	if errs[2] == nil || errs[0] != nil {
		t.Errorf("without the hole: %v", errs)
	}
}
