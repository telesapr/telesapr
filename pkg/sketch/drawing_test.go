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
	d := api.New()
	block(d, 0, 0, 20, 10, 5)
	step(d)
	before := saved(t, d)
	dr := d.Drawing()
	dr.Sheet, dr.Portrait, dr.Scale, dr.FirstAngle = "A2", true, 0.5, true
	dr.Title, dr.Author, dr.Number, dr.Material = "Plate", "M. S.", "TS-7", "steel"
	dr.Views = []api.DrawingView{{Kind: "front", X: 100, Y: 200}, {Kind: "iso", X: 300, Y: 400}}
	dr.Sections = []sketch.DrawingSection{{View: "top", Offset: 2.5}}
	dr.Details = []sketch.DrawingDetail{{View: "front", CX: 1, CY: 2, R: 3, Scale: 4, X: 50, Y: 60}}
	dr.Dimensions = []sketch.DrawingDimension{{View: "front", Kind: "distance", Offset: -5,
		Edges: []topo.Name{{A: 0x10002, B: 0x10003, N: 0, Of: 1}, {A: 0x10004, B: 0x10005, N: 1, Of: 2}}}}
	d.SetDrawing(dr)
	undoRedo(t, d, before)
	back := roundTrip(t, d)
	got := back.Drawing()
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
			d.SetDrawing(bad)
		}()
	}
}
