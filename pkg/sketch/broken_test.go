package sketch_test

import (
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/sketch"
)

func brokenLines(sk *sketch.Sketch) string {
	var lines []string
	for _, b := range sk.BrokenReferences {
		lines = append(lines, b.String())
	}
	return strings.Join(lines, "\n")
}

// deleteGroup deletes group h as the interface does: out of the list,
// then a regeneration prunes what used it.
func deleteGroup(sk *sketch.Sketch, h core.HGroup) {
	for i := range sk.Groups {
		if sk.Groups[i].H == h {
			sk.Groups = append(sk.Groups[:i], sk.Groups[i+1:]...)
			break
		}
	}
	sk.GenerateAll()
}

func TestBrokenReferencesDeleteGroup(t *testing.T) {
	t.Parallel()
	sk := translateND(t)
	if len(sk.BrokenReferences) != 0 {
		t.Fatalf("broken on load: %s", brokenLines(sk))
	}
	sk.UndoRemember()
	deleteGroup(sk, hg(4))
	want := "deleted g005-extrude, which used g004-sketch-in-plane\n" +
		"deleted g006-translate, which used g005-extrude\n" +
		"deleted g007-translate, which used g006-translate"
	if got := brokenLines(sk); got != want {
		t.Fatalf("broken:\n%s\nwant:\n%s", got, want)
	}
	if b := sk.BrokenReferences[0]; b.Group != hg(5) || b.GroupName != "g005-extrude" ||
		b.Item != "g005-extrude" || b.Missing != "g004-sketch-in-plane" {
		t.Fatalf("%+v", b)
	}

	// A regeneration that deletes nothing keeps the list; undo clears it.
	sk.GenerateAll()
	if len(sk.BrokenReferences) != 3 {
		t.Fatal("a regeneration dropped the list")
	}
	sk.UndoUndo()
	if len(sk.BrokenReferences) != 0 {
		t.Fatal("undo kept the list")
	}
	sk.GenerateAll()
	if len(sk.Groups) != 7 || len(sk.BrokenReferences) != 0 {
		t.Fatalf("after undo: %d groups, broken %s", len(sk.Groups), brokenLines(sk))
	}
	deleteGroup(sk, hg(7))
	if len(sk.BrokenReferences) != 0 {
		t.Fatalf("deleting the last group broke %s", brokenLines(sk))
	}
	sk.ClearBrokenReferences()
}

// TestBrokenReferencesDeleteRequest deletes a line of the bolt head's
// sketch that a constraint of the shank's sketch uses.
func TestBrokenReferencesDeleteRequest(t *testing.T) {
	t.Parallel()
	sk := bolt(t)
	sk.GenerateAll()
	var shank core.HRequest
	for _, r := range sk.Requests {
		if r.Group == hg(4) {
			shank = r.H
		}
	}
	line := core.HRequest{V: 4}
	hc := sk.AddConstraint(core.Constraint{
		Type: core.ConstraintPointsCoincident, Group: hg(4),
		PtA: shank.Entity(1), PtB: line.Entity(1)})
	sk.GenerateAll()
	if sk.Constraint(hc) == nil || len(sk.BrokenReferences) != 0 {
		t.Fatalf("the constraint didn't take: %s", brokenLines(sk))
	}
	for i := range sk.Requests {
		if sk.Requests[i].H == line {
			sk.Requests = append(sk.Requests[:i], sk.Requests[i+1:]...)
			break
		}
	}
	sk.GenerateAll()
	got := brokenLines(sk)
	want := "deleted c02e-pts-coincident of g004-shank-sketch, which used r004-line-segment of g002-sketch-in-plane"
	if !strings.Contains(got, want) || len(sk.BrokenReferences) != 10 {
		t.Fatalf("broken:\n%s\nwant 10 lines with:\n%s", got, want)
	}
	for _, b := range sk.BrokenReferences {
		if b.Group != hg(2) && b.Group != hg(4) {
			t.Fatalf("%+v", b)
		}
	}
}

// TestBrokenReferencesNames: what is gone is named as it was, or by its
// handle if it was never seen.
func TestBrokenReferencesNames(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		origin core.HEntity
		want   string
	}{
		{core.HRequest{V: 0x77}.Entity(1), "deleted g004-sketch-in-plane, which used r077"},
		{core.HGroup{V: 0x55}.Entity(3), "deleted g004-sketch-in-plane, which used entity 80550003 of g055"},
	} {
		sk := translateND(t)
		// The sketch on the face gets an origin that never was.
		sk.Group(hg(4)).Predef.Origin = c.origin
		sk.GenerateAll()
		if got := brokenLines(sk); !strings.HasPrefix(got, c.want+"\n") || len(sk.BrokenReferences) != 4 {
			t.Fatalf("broken:\n%s\nwant first: %s", got, c.want)
		}
	}
}
