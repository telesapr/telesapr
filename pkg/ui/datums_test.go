package ui

import (
	"math"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/script/api"
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/topo"
)

// The datums screen names a fillet's face, which no entity makes, and a
// straight edge of the solid, selected on the model, as a script's
// Datums.NameFace and NameEdge do, each one step of undo; then mates
// two of a linked model's names picked on the screen, as a script's
// Assembly.Concentric does.
func TestDatumScreenNamesTheSolidAndMates(t *testing.T) {
	t.Parallel()
	ss, _ := newTestApp(t)
	doc := api.NewDoc(ss.ScriptHost())
	doc.Sketch.Rect(api.V(0, 0, 0), api.V(40, 30, 0))
	b := doc.Features.Extrude(10, "plate")
	r := doc.Features.FilletSolid([]api.SolidEdge{b.Solid().EdgeAt(api.V(40, 30, 5))}, 6, "fillet")
	k := 6 * (1 - math.Sqrt(0.5))
	round := r.Solid().FaceAt(api.V(40-k, 30-k, 5))
	front := r.Solid().EdgeAt(api.V(20, 0, 0))

	ss.MenuDatums()
	tw := ss.TW
	gw := ss.GW
	gw.selection = append(gw.selection, Selection{Face: srf.FaceID(round.ID())})
	if _, edge, ok := ss.selectedSolidForDatum(); !ok || edge != nil {
		t.Fatalf("the fillet's face is not one to name: %v %v", edge, ok)
	}
	tw.ScreenChangeDatum('f', 0)
	if tw.edit.meaning != editDatumSolid {
		t.Fatalf("the edit is %d", tw.edit.meaning)
	}
	tw.EditControlDoneForDatums("corner")
	gw.selection = append(gw.selection[:0], Selection{Edge: solidEdgeOf(t, front)})
	tw.ScreenChangeDatum('f', 0)
	tw.EditControlDoneForDatums("front")
	if got := doc.Datums.Names(); !slices.Equal(got, []string{"corner", "front"}) {
		t.Fatalf("named %q", got)
	}
	g := ss.SK.Group(ss.SK.Datum("corner").Entity.Group())
	if g == nil || g.Subtype != sketch.GroupWorkplaneByDatum || len(g.SolidFaces) != 1 {
		t.Fatalf("the corner's group %+v", g)
	}
	if !doc.Edit.Undo() || slices.Contains(doc.Datums.Names(), "front") {
		t.Errorf("the edge's datum not undone: %q", doc.Datums.Names())
	}

	// The plate linked twice into a new model; two of the names picked
	// on the screen, the first of the part that stays, and mated
	// concentric, as a script would by name.
	path := filepath.Join(t.TempDir(), "rounded.tlspr")
	if err := doc.File.Save(path); err != nil {
		t.Fatal(err)
	}
	ss, _ = newTestApp(t)
	tw, doc = ss.TW, api.NewDoc(ss.ScriptHost())
	if _, err := doc.Assembly.Link(path, "a"); err != nil {
		t.Fatal(err)
	}
	lb, err := doc.Assembly.Link(path, "b")
	if err != nil {
		t.Fatal(err)
	}
	lb.AsLink().Move(api.V(70, 50, 0))
	tw.ScreenPickDatumForMate('m', 0)
	tw.ScreenMateDatums('x', 0)
	if len(tw.datumMate) != 0 {
		t.Fatalf("cleared, picked %q", tw.datumMate)
	}
	tw.ScreenPickDatumForMate('m', 0)
	tw.ScreenPickDatumForMate('m', 1)
	if !slices.Equal(tw.datumMate, []string{"a.corner", "b.corner"}) {
		t.Fatalf("picked %q", tw.datumMate)
	}
	tw.ScreenMateDatums('c', 0)
	ca, cb := doc.Datums.Get("a.corner").Center().Pos(), doc.Datums.Get("b.corner").Center().Pos()
	if math.Hypot(ca.X-cb.X, ca.Y-cb.Y) > 1e-6 || len(tw.datumMate) != 0 {
		t.Errorf("the corners' axes at %v and %v; picked %q", ca, cb, tw.datumMate)
	}
}

// solidEdgeOf is the name of an edge of the solid, as the window
// selects it.
func solidEdgeOf(t *testing.T, e api.SolidEdge) topo.Name {
	t.Helper()
	n, err := topo.ParseName(e.Name())
	if err != nil {
		t.Fatal(err)
	}
	return n
}

// The Assembly menu's Mate Concentric and Mate Flush take two datums on
// the solids selected, as linked normals, in the order selected; a
// straight edge's datum, a line, is refused as a plane, with the reason.
func TestMateMenuTakesDatums(t *testing.T) {
	t.Parallel()
	ss, _ := newTestApp(t)
	doc := api.NewDoc(ss.ScriptHost())
	doc.Sketch.Rect(api.V(0, 0, 0), api.V(40, 30, 0))
	b := doc.Features.Extrude(10, "plate")
	r := doc.Features.FilletSolid([]api.SolidEdge{b.Solid().EdgeAt(api.V(40, 30, 5))}, 6, "fillet")
	k := 6 * (1 - math.Sqrt(0.5))
	doc.Datums.NameFace("corner", r.Solid().FaceAt(api.V(40-k, 30-k, 5)))
	doc.Datums.NameEdge("front", r.Solid().EdgeAt(api.V(20, 0, 0)))
	path := filepath.Join(t.TempDir(), "rounded.tlspr")
	if err := doc.File.Save(path); err != nil {
		t.Fatal(err)
	}

	ss, _ = newTestApp(t)
	doc = api.NewDoc(ss.ScriptHost())
	if _, err := doc.Assembly.Link(path, "a"); err != nil {
		t.Fatal(err)
	}
	lb, err := doc.Assembly.Link(path, "b")
	if err != nil {
		t.Fatal(err)
	}
	lb.AsLink().Move(api.V(70, 50, 0))
	gw := ss.GW
	pick := func(names ...string) {
		gw.selection = gw.selection[:0]
		for _, n := range names {
			gw.selection = append(gw.selection, Selection{Entity: doc.Datums.Get(n).H})
		}
	}
	// The script host takes the window's messages while it runs.
	var said strings.Builder
	ss.ScriptHost().Messages = &said
	before := len(ss.SK.Constraints)
	pick("a.front", "b.front")
	ss.MenuMate(cmdMateFlush)
	if len(ss.SK.Constraints) != before || !strings.Contains(said.String(), "datum of an edge") {
		t.Fatalf("an edge's datum mated flush: %d constraints, said %q", len(ss.SK.Constraints)-before, said.String())
	}
	pick("a.corner", "b.corner")
	ss.MenuMate(cmdMateConcentric)
	if len(ss.SK.Constraints) == before {
		t.Fatal("no concentric mate of the corners")
	}
	ca, cb := doc.Datums.Get("a.corner").Center().Pos(), doc.Datums.Get("b.corner").Center().Pos()
	if math.Hypot(ca.X-cb.X, ca.Y-cb.Y) > 1e-6 {
		t.Errorf("the corners' axes at %v and %v", ca, cb)
	}
}
