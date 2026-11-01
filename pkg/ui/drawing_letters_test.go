package ui

import (
	"testing"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// The letters of two cutting planes, one seen as a line across the
// front and the right views, the other as a line up the front and the
// top views through their names, stand clear of the views' names, of
// the dimensions' texts and of each other; a name a letter would cover
// is moved below it.
func TestCuttingPlaneLettersClear(t *testing.T) {
	t.Parallel()
	ss, doc := drawingModel(t)
	var right, back = -1, -1
	solid := doc.Active().Solid()
	edges := solid.Edges()
	for i, e := range edges {
		a, b := e.Ends()
		if !e.Straight() {
			continue
		}
		// The front view's right upright, and an edge across its top.
		if a.X == 120 && b.X == 120 && a.Z == 40 && b.Z == 40 {
			right = i
		}
		if a.Y == 60 && b.Y == 60 && a.Z == 0 && b.Z == 0 {
			back = i
		}
	}
	if right < 0 || back < 0 {
		t.Fatalf("no edges to dimension: %d, %d", right, back)
	}
	doc.Drawing.AddDimension("front", "length", 0, edges[right])
	doc.Drawing.AddDimension("front", "length", 0, edges[back])
	d := doc.Drawing.Get()
	d.Sections = []sketch.DrawingSection{{View: "top"}, {View: "right"}}
	ds, err := ss.drawSheet(&d)
	if err != nil {
		t.Fatal(err)
	}
	if len(ds.letters) != 8 {
		t.Fatalf("%d letters, want two at each end of a line on two views for each section", len(ds.letters))
	}
	for i, l := range ds.letters {
		for _, n := range ds.names {
			if l.overlaps(n) {
				t.Errorf("letter %d %v is on a view's name %v", i, l, n)
			}
		}
		for _, x := range ds.texts {
			if l.overlaps(x) {
				t.Errorf("letter %d %v is on a dimension's text %v", i, l, x)
			}
		}
		for j, o := range ds.letters[:i] {
			if l.overlaps(o) {
				t.Errorf("letters %d and %d overlap", i, j)
			}
		}
	}
	for _, n := range ds.names {
		for _, x := range ds.dims {
			if n.overlaps(x) {
				t.Errorf("a view's name %v is on a dimension %v", n, x)
			}
		}
	}
}

// Where the end of a cutting plane's line is free, its letter goes past
// it; where it is taken, beside it.
func TestPlaceLetter(t *testing.T) {
	t.Parallel()
	e, o := geom.Vector{X: 10}, geom.Vector{X: 1}
	b := placeLetter(e, o, 2, 3, nil)
	if b.lo.X < e.X || b.lo.Y > 0 || b.hi.Y < 0 {
		t.Errorf("free: %v, want past %v on the line", b, e)
	}
	taken := boxAbout(geom.Vector{X: 14}, 6, 6)
	b = placeLetter(e, o, 2, 3, []sheetBox{taken})
	if b.overlaps(taken) || b.hi.X > e.X+1e-9 {
		t.Errorf("past the end taken: %v, want beside %v", b, e)
	}
	all := boxAbout(e, 40, 40)
	if b = placeLetter(e, o, 2, 3, []sheetBox{all}); b.lo.X < e.X {
		t.Errorf("all taken: %v, want past the end", b)
	}
}

// A length of an edge inside a view's outline stands clear of the
// outline: an orthographic view's out from the view's middle, a
// pictorial one's on the nearer side.
func TestLengthClearOfOutline(t *testing.T) {
	t.Parallel()
	square := []geom.Vector{{X: -10, Y: -10}, {X: 10, Y: -10}, {X: 10, Y: 10}, {X: -10, Y: 10}}
	for _, c := range []struct {
		y         float64
		pictorial bool
		above     bool
	}{{3, false, true}, {-3, false, false}, {-3, true, false}, {7, true, true}} {
		v := dimView{right: geom.Vector{X: 1}, up: geom.Vector{Y: 1}, scale: 1, pictorial: c.pictorial, outline: square}
		var out dimOut
		out.linearDimClear(v, geom.Vector{X: -5, Y: c.y}, geom.Vector{X: 5, Y: c.y}, 10, "10")
		if len(out.texts) != 1 {
			t.Fatalf("%d texts", len(out.texts))
		}
		tx := out.texts[0]
		if c.above && tx.lo.Y < 20 || !c.above && tx.hi.Y > -20 {
			t.Errorf("edge at y %g, pictorial %v: text %v, want 10 past the outline %s", c.y, c.pictorial, tx,
				map[bool]string{true: "above", false: "below"}[c.above])
		}
	}
	// In toward the middle, it stays where it is put.
	var out dimOut
	out.linearDimClear(dimView{right: geom.Vector{X: 1}, up: geom.Vector{Y: 1}, scale: 1, outline: square},
		geom.Vector{X: -5, Y: 3}, geom.Vector{X: 5, Y: 3}, -2, "10")
	if tx := out.texts[0]; tx.hi.Y > 3 {
		t.Errorf("in toward the middle: text %v, want below the edge", tx)
	}
}

// A view's name is moved below a box it would cover, and left where it
// is otherwise.
func TestViewNameMovedBelow(t *testing.T) {
	t.Parallel()
	l := viewLabel{text: "FRONT", x: 0, y: -7}
	var sheet dimOut
	got := drawViewLabels(&sheet.el, []viewLabel{l}, nil)
	if got[0].lo.Y != -7 {
		t.Errorf("free: name at %v", got[0])
	}
	letter := boxAbout(geom.Vector{Y: -6}, 2, 3.5)
	got = drawViewLabels(&sheet.el, []viewLabel{l}, []sheetBox{letter})
	if got[0].overlaps(letter) || got[0].hi.Y > letter.lo.Y {
		t.Errorf("under a letter %v: name at %v", letter, got[0])
	}
}
