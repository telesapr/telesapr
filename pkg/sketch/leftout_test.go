package sketch

import (
	"errors"
	"slices"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
)

// damagedSketch is a loaded sketch whose drawing group holds a request
// and a constraint left without what they use: a request in group 9,
// which isn't there, and a distance to a point of request 7, which
// isn't either.
func damagedSketch(t *testing.T) *Sketch {
	t.Helper()
	sk := NewSketch()
	sk.AddReferences()
	hg := sk.AddGroup(Group{Type: GroupDrawing3D, Order: 1, Scale: 1, Name: "drawing"})
	sk.GroupOrder = []core.HGroup{{V: core.HGroupReferences}, hg}
	sk.Requests = append(sk.Requests,
		Request{H: core.HRequest{V: 4}, Type: RequestDatumPoint, Group: hg, Workplane: core.FreeIn3D},
		Request{H: core.HRequest{V: 6}, Type: RequestDatumPoint, Group: core.HGroup{V: 9}, Workplane: core.FreeIn3D})
	var c core.Constraint
	c.H = core.HConstraint{V: 5}
	c.Group = hg
	c.Type = core.ConstraintPtPtDistance
	c.PtA = core.HRequest{V: 4}.Entity(0)
	c.PtB = core.HRequest{V: 7}.Entity(0)
	c.ValA = 10
	sk.Constraints = append(sk.Constraints, c)
	sk.Params = append(sk.Params,
		core.Param{H: core.HRequest{V: 6}.Param(0), Val: 3},
		core.Param{H: core.HRequest{V: 4}.Param(0), Val: 1})
	sk.UpgradeLegacyData()
	return sk
}

// The load keeps what it leaves out, with its values, named for the
// user; nothing in the model holds it.
func TestLeftOutKept(t *testing.T) {
	t.Parallel()
	sk := damagedSketch(t)
	want := []string{"r006-datum-point of g009", "c005-pt-pt-distance of g002-drawing"}
	if got := sk.LeftOutNames(); !slices.Equal(got, want) {
		t.Fatalf("left out %q, want %q", got, want)
	}
	if sk.Request(core.HRequest{V: 6}) != nil || sk.Constraint(core.HConstraint{V: 5}) != nil {
		t.Error("what was left out is still in the model")
	}
	if p := sk.LeftOut.Params; len(p) != 1 || p[0].H != (core.HRequest{V: 6}.Param(0)) || p[0].Val != 3 {
		t.Errorf("left-out values %v, want request 6's", p)
	}
}

// A new item takes no handle of what was left out, nor of the group a
// left-out request was in, which would bring it back.
func TestLeftOutHandles(t *testing.T) {
	t.Parallel()
	sk := damagedSketch(t)
	if h := sk.AddRequest(Request{Group: core.HGroup{V: 2}}); h.V != 7 {
		t.Errorf("new request %x, want 7", h.V)
	}
	if h := sk.AddConstraint(core.Constraint{}); h.V != 6 {
		t.Errorf("new constraint %x, want 6", h.V)
	}
	if h := sk.AddGroup(Group{}); h.V != 10 {
		t.Errorf("new group %x, want 10, past the left-out request's", h.V)
	}
}

// Discarding what was left out is one undo step, and comes back on
// redo as it was.
func TestDiscardLeftOutUndo(t *testing.T) {
	t.Parallel()
	sk := damagedSketch(t)
	kept := sk.LeftOut
	if err := sk.UndoRememberFor(sk.DiscardLeftOut); err != nil {
		t.Fatal(err)
	}
	if sk.LeftOut != nil {
		t.Fatal("still kept after a discard")
	}
	if err := sk.UndoRememberFor(sk.DiscardLeftOut); !errors.Is(err, ErrNothingLeftOut) {
		t.Errorf("a second discard: %v", err)
	}
	if !sk.UndoUndo() || sk.LeftOut != kept {
		t.Fatal("undo didn't bring it back")
	}
	if !sk.UndoRedo() || sk.LeftOut != nil {
		t.Error("redo didn't discard it again")
	}
	sk.Clear()
	if sk.LeftOut != nil {
		t.Error("a cleared sketch keeps it")
	}
}
