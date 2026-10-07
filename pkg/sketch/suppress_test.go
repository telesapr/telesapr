package sketch_test

import (
	"bytes"
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/persist/slvs"
	"github.com/telesapr/telesapr/pkg/persist/tlspr"
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/srf"
)

// translateND loads a fixture that is a fixed point of the solver: g002
// sketch, g003 extrude, g004 sketch on its face, g005 extrude, g006 and
// g007 step translating.
func translateND(t *testing.T) *sketch.Sketch {
	t.Helper()
	srf.SetChordTolMm(0)
	doc, err := slvs.LoadFile(fixturesPath(t, "group", "translate_nd", "normal.slvs"))
	if err != nil {
		t.Fatal(err)
	}
	doc.Sketch.GenerateAll()
	return doc.Sketch
}

func volume(sk *sketch.Sketch, h core.HGroup) float64 {
	var m srf.SMesh
	sk.Group(h).RunningShell.TriangulateInto(&m)
	return m.CalculateVolume()
}

func TestSuppressAll(t *testing.T) {
	sk := translateND(t)
	sk.Active = hg(7)
	sk.GenerateAll()
	saved := tlspr.Encode(sk, tlspr.Options{})
	r, c, g, e := len(sk.Requests), len(sk.Constraints), len(sk.Groups), len(sk.Entities)
	head := volume(sk, hg(3))

	sk.Unsaved = false
	if err := sk.SetSuppressAll(hg(4), true); err != nil {
		t.Fatal(err)
	}
	if sk.Group(hg(4)).Clean || !sk.Unsaved {
		t.Fatal("suppressing should dirty the group and the file")
	}
	sk.GenerateAll()
	if len(sk.Requests) != r || len(sk.Constraints) != c || len(sk.Groups) != g {
		t.Fatal("suppressing pruned something")
	}
	if len(sk.Entities) >= e {
		t.Fatalf("%d entities, %d before", len(sk.Entities), e)
	}
	for _, en := range sk.Entities {
		if en.Group.V >= 4 {
			t.Fatalf("entity %x of a suppressed group", en.H.V)
		}
	}
	want := map[uint32]string{
		3: "",
		4: "suppressed",
		5: "uses g004-sketch-in-plane, which is suppressed",
		6: "uses g004-sketch-in-plane, which is suppressed",
		7: "uses g004-sketch-in-plane, which is suppressed",
	}
	for h, w := range want {
		if got := sk.SuppressedReason(hg(h)); got != w {
			t.Errorf("g%03x: %q, want %q", h, got, w)
		}
		if gg := sk.Group(hg(h)); h >= 4 && (gg.IsVisible() || !gg.IsSolvedOkay() || !gg.Clean) {
			t.Errorf("g%03x left out: visible %v, solved %v, clean %v", h, gg.IsVisible(), gg.IsSolvedOkay(), gg.Clean)
		}
	}
	if sk.Active != hg(3) {
		t.Fatalf("active group g%03x, want the last one generated", sk.Active.V)
	}
	if got := volume(sk, hg(7)); math.Abs(got-head) > 1e-9*head {
		t.Fatalf("the solid is %v, want the first extrusion's %v", got, head)
	}

	// Saved, loaded and brought back, the model is what it was, once
	// the group that was active is again.
	data := tlspr.Encode(sk, tlspr.Options{})
	doc, err := tlspr.Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	if !doc.Sketch.Group(hg(4)).SuppressAll {
		t.Fatal("the save lost the flag")
	}
	for i, back := range []*sketch.Sketch{sk, doc.Sketch} {
		if err := back.SetSuppressAll(hg(4), false); err != nil {
			t.Fatal(err)
		}
		back.Active = hg(7)
		back.GenerateAll()
		if got := tlspr.Encode(back, tlspr.Options{}); !bytes.Equal(got, saved) {
			t.Fatalf("suppressing and bringing back changed the file (%d)", i)
		}
	}
}

func TestSuppressAllUndo(t *testing.T) {
	sk := translateND(t)
	sk.UndoRemember()
	if err := sk.SetSuppressAll(hg(5), true); err != nil {
		t.Fatal(err)
	}
	sk.GenerateAll()
	if sk.Group(hg(6)).SuppressedBy != hg(5) || sk.Group(hg(4)).SuppressedBy.V != 0 {
		t.Fatal("suppressing g005 should leave out the copies and nothing before")
	}
	if !sk.UndoUndo() {
		t.Fatal("no undo")
	}
	sk.GenerateAll()
	for _, g := range sk.Groups {
		if g.SuppressAll || g.SuppressedBy.V != 0 {
			t.Fatalf("%s still suppressed after undo", g.DescriptionString())
		}
	}
	if !sk.UndoRedo() || !sk.Group(hg(5)).SuppressAll {
		t.Fatal("redo should suppress again")
	}
}

func TestSuppressAllRefused(t *testing.T) {
	sk := translateND(t)
	if err := sk.SetSuppressAll(hg(1), true); err == nil || err.Error() != "the references can't be suppressed" {
		t.Fatalf("suppressing the references: %v", err)
	}
	if err := sk.SetSuppressAll(hg(0x99), true); err == nil || err.Error() != "no group g099" {
		t.Fatalf("suppressing a missing group: %v", err)
	}
	sk.Unsaved = false
	if err := sk.SetSuppressAll(hg(4), false); err != nil || sk.Unsaved {
		t.Fatalf("bringing back a group that is there: %v, unsaved %v", err, sk.Unsaved)
	}
	// A flag on the references, as a file may hold, is ignored.
	sk.Group(hg(1)).SuppressAll = true
	sk.GenerateAll()
	if sk.SuppressedReason(hg(1)) != "" || sk.SuppressedReason(hg(2)) != "" {
		t.Fatal("the references were left out")
	}
}

// TestSuppressAllActive: the active group moves to the nearest group
// still in the model, earlier if there is one but the references.
func TestSuppressAllActive(t *testing.T) {
	sk := bolt(t)
	sk.Active = hg(3)
	if err := sk.SetSuppressAll(hg(2), true); err != nil {
		t.Fatal(err)
	}
	sk.GenerateAll()
	if sk.Active != hg(4) {
		t.Fatalf("active g%03x, want g004, the first after the head", sk.Active.V)
	}
	sk.Active = hg(9)
	if err := sk.SetSuppressAll(hg(8), true); err != nil {
		t.Fatal(err)
	}
	sk.GenerateAll()
	if sk.Active != hg(7) {
		t.Fatalf("active g%03x, want g007, the last before the profile", sk.Active.V)
	}
	for _, h := range []uint32{4, 5, 6, 7} {
		if sk.Group(hg(h)).SuppressedBy.V != 0 {
			t.Fatalf("g%03x left out", h)
		}
	}
	if err := sk.SetSuppressAll(hg(4), true); err != nil {
		t.Fatal(err)
	}
	sk.GenerateAll()
	if sk.Active != hg(1) {
		t.Fatalf("active g%03x, want the references: nothing else is left", sk.Active.V)
	}
}
