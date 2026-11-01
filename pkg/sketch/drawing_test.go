package sketch_test

import (
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/script/api"
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/topo"
)

// The drawing is part of the model (roadmap 18.1, 18.2): set, it goes
// through undo, redo and a round trip; a sheet or view there isn't is
// refused.
func TestDrawingSaved(t *testing.T) {
	t.Parallel()
	d := api.New()
	block(d, 0, 0, 20, 10, 5)
	step(d)
	before := saved(t, d)
	dr := d.Drawing.Get()
	dr.Sheet, dr.Portrait, dr.Scale, dr.FirstAngle = "A2", true, 0.5, true
	dr.Title, dr.Author, dr.Number, dr.Material = "Plate", "M. S.", "TS-7", "steel"
	dr.Views = []api.DrawingView{{Kind: "front", X: 100, Y: 200}, {Kind: "iso", X: 300, Y: 400}}
	dr.Sections = []sketch.DrawingSection{{View: "top", Offset: 2.5}}
	dr.Details = []sketch.DrawingDetail{{View: "front", CX: 1, CY: 2, R: 3, Scale: 4, X: 50, Y: 60}}
	dr.Dimensions = []sketch.DrawingDimension{{View: "front", Kind: "distance", Offset: -5,
		Edges: []topo.Name{{A: 0x10002, B: 0x10003, N: 0, Of: 1}, {A: 0x10004, B: 0x10005, N: 1, Of: 2}}}}
	d.Drawing.Set(dr)
	undoRedo(t, d, before)
	back := roundTrip(t, d)
	got := back.Drawing.Get()
	if got.Sheet != "A2" || !got.Portrait || got.Scale != 0.5 || !got.FirstAngle || got.Title != "Plate" ||
		got.Material != "steel" || len(got.Views) != 2 || got.Views[1].Kind != "iso" || got.Views[1].Y != 400 ||
		len(got.Sections) != 1 || got.Sections[0].Offset != 2.5 || len(got.Details) != 1 || got.Details[0].Scale != 4 ||
		len(got.Dimensions) != 1 || got.Dimensions[0].Edges[1].Of != 2 || got.Dimensions[0].Offset != -5 {
		t.Errorf("reopened: %+v", got)
	}
	if !strings.Contains(string(saved(t, d)), "drawing {") {
		t.Error("the file has no drawing")
	}
	for name, bad := range map[string]api.Drawing{
		"sheet":     {Sheet: "B5"},
		"view":      {Sheet: "A4", Views: []api.DrawingView{{Kind: "sideways"}}},
		"scale":     {Sheet: "A4", Scale: -1},
		"dimension": {Sheet: "A4", Dimensions: []sketch.DrawingDimension{{View: "front", Kind: "distance"}}},
	} {
		func() {
			defer func() {
				if _, ok := recover().(error); !ok {
					t.Errorf("a %s there isn't: no error", name)
				}
			}()
			d.Drawing.Set(bad)
		}()
	}
}

// The standard views stand 25 mm apart, room for a dimension beside
// one and a cutting plane's letter beside the next, in third angle and
// first.
func TestLayoutSpacing(t *testing.T) {
	t.Parallel()
	sizes := map[string][2]float64{"front": {100, 50}, "top": {100, 40}, "right": {40, 50}, "left": {40, 50}, "iso": {90, 80}}
	for _, first := range []bool{false, true} {
		d := &sketch.Drawing{Sheet: "A3", FirstAngle: first}
		views, scale := d.Layout(sizes)
		at := map[string]sketch.DrawingView{}
		for _, v := range views {
			at[v.Kind] = v
		}
		side := map[bool]string{false: "right", true: "left"}[first]
		front, top, sv := at["front"], at["top"], at[side]
		gapX := (sv.X - scale*sizes[side][0]/2) - (front.X + scale*sizes["front"][0]/2)
		gapY := (top.Y - scale*sizes["top"][1]/2) - (front.Y + scale*sizes["front"][1]/2)
		if first {
			gapY = (front.Y - scale*sizes["front"][1]/2) - (top.Y + scale*sizes["top"][1]/2)
		}
		if gapX < 25-1e-9 || gapY < 25-1e-9 {
			t.Errorf("first angle %v, scale %g: %g mm across and %g mm up between the views, want 25", first, scale, gapX, gapY)
		}
	}
}

// The scale fitted leaves room for the sections and details right of
// the views: placed there, each stands clear of the views and within
// the frame, where without them the views would be drawn larger.
func TestLayoutRoomForSections(t *testing.T) {
	t.Parallel()
	sizes := map[string][2]float64{"front": {60, 26}, "top": {60, 30}, "right": {30, 26}, "iso": {63.6, 62}}
	bare := &sketch.Drawing{Sheet: "A3"}
	_, bareScale := bare.Layout(sizes)
	d := &sketch.Drawing{Sheet: "A3", Sections: []sketch.DrawingSection{{View: "top", Offset: 8}, {View: "front"}},
		Details: []sketch.DrawingDetail{{View: "front", R: 5, Scale: 2}}}
	views, scale := d.Layout(sizes)
	if scale >= bareScale {
		t.Errorf("scale %g with sections, %g without: the views fill the room the sections need", scale, bareScale)
	}
	right := 0.0
	for _, v := range views {
		right = max(right, v.X+scale*sizes[v.Kind][0]/2)
	}
	on := [][2]float64{{scale * 60, scale * 30}, {scale * 60, scale * 26}, {scale * 20, scale * 20}}
	w, _ := d.Size()
	for i, p := range d.PlaceExtras(right, on) {
		if lo, hi := p[0]-on[i][0]/2, p[0]+on[i][0]/2; lo < right+10 || hi > w-sketch.SheetBorder+1e-9 {
			t.Errorf("extra %d from %g to %g: the views end at %g, the frame at %g", i, lo, hi, right, w-sketch.SheetBorder)
		}
	}
}
