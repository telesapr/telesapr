package sketch_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/internal/fixtures"
	"github.com/telesapr/telesapr/pkg/persist/tlspr"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// bolt loads the bolt fixture: g002 head sketch, g003 head, g004 shank
// sketch, g005 shank, then the thread's g006 to g009 on the shank.
func bolt(t *testing.T) *sketch.Sketch {
	t.Helper()
	doc, err := tlspr.LoadFile(fixtures.Path("thread/bolt_m8x30.tlspr"))
	if err != nil {
		t.Fatal(err)
	}
	return doc.Sketch
}

func hg(v uint32) core.HGroup { return core.HGroup{V: v} }

// orderOf lists the groups' handles in history order.
func orderOf(sk *sketch.Sketch) []uint32 {
	var out []uint32
	sk.GenerateAllTyped(sketch.GenerateRegen)
	for _, h := range sk.GroupOrder {
		out = append(out, h.V)
	}
	return out
}

func TestGroupDependencies(t *testing.T) {
	t.Parallel()
	sk := bolt(t)
	got := sk.GroupDependencies(hg(7))
	want := []sketch.Dependency{{Group: hg(5), Why: "origin"}, {Group: hg(6), Why: "operand"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("g007 depends on %v, want %v", got, want)
	}
	if got := sk.GroupDependencies(hg(1)); len(got) != 0 {
		t.Fatalf("the references depend on %v", got)
	}
	if got := sk.GroupDependencies(hg(99)); got != nil {
		t.Fatalf("a missing group depends on %v", got)
	}
	if got := sk.GroupDependents(hg(5)); !reflect.DeepEqual(got, []core.HGroup{hg(6), hg(7), hg(8), hg(9)}) {
		t.Fatalf("g005 is used by %v", got)
	}

	// A constraint that names an entity of another group.
	var shank core.HRequest
	for _, r := range sk.Requests {
		if r.Group == hg(4) {
			shank = r.H
		}
	}
	hc := sk.AddConstraint(core.Constraint{
		Type: core.ConstraintPointsCoincident, Group: hg(4),
		PtA: shank.Entity(1), PtB: hg(3).Entity(5)})
	deps := sk.GroupDependencies(hg(4))
	if len(deps) != 2 || deps[1].Group != hg(3) || !strings.HasPrefix(deps[1].Why, "constraint c") {
		t.Fatalf("g004 with constraint %v depends on %v", hc, deps)
	}

	// A copy's remap names its source, even without an operand.
	g := sk.Group(hg(3))
	g.OpA, g.Predef.EntityB = core.HGroup{}, core.HEntity{}
	if deps := sk.GroupDependencies(hg(3)); len(deps) == 0 || deps[len(deps)-1].Why != "copied entity" {
		t.Fatalf("the head without its operand depends on %v", deps)
	}
	// but a linked group's remap names entities of its file.
	g.Type = sketch.GroupLinked
	for _, d := range sk.GroupDependencies(hg(3)) {
		if d.Why == "copied entity" {
			t.Fatalf("a linked group depends on %v", d)
		}
	}
}

func TestMoveGroup(t *testing.T) {
	t.Parallel()
	sk := bolt(t)
	sk.GenerateAll()
	before := orderOf(sk)
	orders := map[uint32]int{}
	for _, g := range sk.Groups {
		orders[g.H.V] = g.Order
	}

	// The shank sketch needs only the references: it can go first.
	if err := sk.MoveGroup(hg(4), hg(2)); err != nil {
		t.Fatal(err)
	}
	if got, want := orderOf(sk), []uint32{1, 4, 2, 3, 5, 6, 7, 8, 9}; !reflect.DeepEqual(got, want) {
		t.Fatalf("order %v, want %v", got, want)
	}
	if sk.Group(hg(4)).Clean || sk.Group(hg(9)).Clean || !sk.Group(hg(1)).Clean {
		t.Fatal("the move should dirty the groups from where the history changed")
	}
	if !sk.Unsaved {
		t.Fatal("the move should mark the file changed")
	}
	// Back where it was, with the order numbers it had.
	if err := sk.MoveGroup(hg(4), hg(5)); err != nil {
		t.Fatal(err)
	}
	if got := orderOf(sk); !reflect.DeepEqual(got, before) {
		t.Fatalf("order %v after moving back, want %v", got, before)
	}
	for _, g := range sk.Groups {
		if g.Order != orders[g.H.V] {
			t.Fatalf("%s has order %d, had %d", g.DescriptionString(), g.Order, orders[g.H.V])
		}
	}
}

func TestMoveGroupRefused(t *testing.T) {
	t.Parallel()
	sk := bolt(t)
	sk.GenerateAll()
	before := orderOf(sk)
	for _, c := range []struct {
		h, before uint32
		msg       string
	}{
		{5, 4, "g005-shank can't move before g004-shank-sketch, which it uses (operand)"},
		{4, 6, "g004-shank-sketch can't move after g005-shank, which uses it (operand)"},
		{8, 0, "g008-M8-profile can't move after g009-M8-thread, which uses it (operand)"},
		{6, 2, "g006-M8-blank can't move before g005-shank, which it uses (origin)"},
		{1, 3, "the references group stays first"},
		{3, 1, "the references group stays first"},
		{0x99, 3, "no group g099"},
		{3, 0x99, "no group g099"},
	} {
		err := sk.CheckMoveGroup(hg(c.h), hg(c.before))
		if err == nil || err.Error() != c.msg {
			t.Errorf("checking g%03x before g%03x: %v, want %q", c.h, c.before, err, c.msg)
		}
		err = sk.MoveGroup(hg(c.h), hg(c.before))
		if err == nil || err.Error() != c.msg {
			t.Errorf("moving g%03x before g%03x: %v, want %q", c.h, c.before, err, c.msg)
		}
		if got := orderOf(sk); !reflect.DeepEqual(got, before) {
			t.Fatalf("refused move changed the order to %v", got)
		}
	}

	var me *sketch.MoveError
	if err := sk.MoveGroup(hg(5), hg(4)); !errors.As(err, &me) ||
		me.Moved != hg(5) || me.Group != hg(5) || me.On != hg(4) || me.Why != "operand" {
		t.Fatalf("MoveError %+v", me)
	}

	// Moves to where the group is change nothing and dirty nothing.
	for _, g := range sk.Groups {
		g.Clean = true
	}
	sk.Unsaved = false
	for _, m := range [][2]uint32{{4, 4}, {4, 5}, {9, 0}} {
		if err := sk.MoveGroup(hg(m[0]), hg(m[1])); err != nil {
			t.Fatal(err)
		}
	}
	if sk.Unsaved {
		t.Fatal("a move to where the group is changed the file")
	}
}

// TestMoveGroupSameOrders moves a group in a history whose file gave
// two groups one order number.
func TestMoveGroupSameOrders(t *testing.T) {
	t.Parallel()
	sk := bolt(t)
	sk.Group(hg(4)).Order = sk.Group(hg(2)).Order
	if got, want := orderOf(sk), []uint32{1, 2, 4, 3, 5, 6, 7, 8, 9}; !reflect.DeepEqual(got, want) {
		t.Fatalf("order %v, want %v", got, want)
	}
	if err := sk.MoveGroup(hg(3), hg(4)); err != nil {
		t.Fatal(err)
	}
	if got, want := orderOf(sk), []uint32{1, 2, 3, 4, 5, 6, 7, 8, 9}; !reflect.DeepEqual(got, want) {
		t.Fatalf("order %v, want %v", got, want)
	}
	seen := map[int]bool{}
	for _, g := range sk.Groups {
		if seen[g.Order] {
			t.Fatalf("order %d twice", g.Order)
		}
		seen[g.Order] = true
	}
}

// A group moves with the groups only it needs; refusals name the
// reference and move nothing.
func TestMoveGroupWithSources(t *testing.T) {
	t.Parallel()
	sk := bolt(t)
	sk.GenerateAll()
	if got := sk.OwnSources(hg(3)); !reflect.DeepEqual(got, []core.HGroup{hg(2), hg(3)}) {
		t.Fatalf("the head's own sources are %v", got)
	}
	// The shank sketch is the shank's own; the shank is used by the
	// thread, so it is no one's but its own.
	if got := sk.OwnSources(hg(5)); !reflect.DeepEqual(got, []core.HGroup{hg(4), hg(5)}) {
		t.Fatalf("the shank's own sources are %v", got)
	}
	if sk.OwnSources(hg(0x99)) != nil {
		t.Fatal("a missing group has sources")
	}
	before := orderOf(sk)
	for _, c := range []struct {
		h, before uint32
		msg       string
	}{
		{5, 0, "g005-shank can't move after g006-M8-blank, which uses g005-shank (origin)"},
		{3, 2, "g003-head can't move before g002-sketch-in-plane, which moves with it"},
		{1, 0, "the references group stays first"},
		{3, 1, "the references group stays first"},
		{0x99, 0, "no group g099"},
		{3, 0x99, "no group g099"},
	} {
		if err := sk.CheckMoveGroupWithSources(hg(c.h), hg(c.before)); err == nil || err.Error() != c.msg {
			t.Errorf("checking g%03x before g%03x: %v, want %q", c.h, c.before, err, c.msg)
		}
		if _, err := sk.MoveGroupWithSources(hg(c.h), hg(c.before)); err == nil || err.Error() != c.msg {
			t.Errorf("moving g%03x before g%03x: %v, want %q", c.h, c.before, err, c.msg)
		}
		if got := orderOf(sk); !reflect.DeepEqual(got, before) {
			t.Fatalf("refused move changed the order to %v", got)
		}
	}
	// A move to where they are changes nothing.
	if _, err := sk.MoveGroupWithSources(hg(9), hg(0)); err != nil {
		t.Fatal(err)
	}
	moved, err := sk.MoveGroupWithSources(hg(3), hg(0))
	if err != nil || !reflect.DeepEqual(moved, []core.HGroup{hg(2), hg(3)}) {
		t.Fatalf("moved %v, %v", moved, err)
	}
	if got, want := orderOf(sk), []uint32{1, 4, 5, 6, 7, 8, 9, 2, 3}; !reflect.DeepEqual(got, want) {
		t.Fatalf("order %v, want %v", got, want)
	}
	if _, err := sk.MoveGroupWithSources(hg(3), hg(4)); err != nil {
		t.Fatal(err)
	}
	if got := orderOf(sk); !reflect.DeepEqual(got, before) {
		t.Fatalf("moving back gave %v, want %v", got, before)
	}
}

// After groups are deleted, the active group is the one that was, if
// it is left, else the nearest one before it that is.
func TestActivateSurvivor(t *testing.T) {
	t.Parallel()
	sk := bolt(t)
	order := append([]core.HGroup(nil), sk.GroupOrder...)
	sk.Active = hg(9)
	if sk.ActivateSurvivor(order, hg(9)) || sk.Active != hg(9) {
		t.Fatal("a group that is left changed")
	}
	sk.Active = hg(2)
	if !sk.ActivateSurvivor(order, hg(9)) || sk.Active != hg(9) {
		t.Fatalf("active %v, want the one that was", sk.Active)
	}
	keep := func(hs ...uint32) {
		var gs []sketch.Group
		for _, g := range sk.Groups {
			for _, h := range hs {
				if g.H.V == h {
					gs = append(gs, g)
				}
			}
		}
		sk.Groups = gs
	}
	keep(1, 2, 3, 4, 9)
	if !sk.ActivateSurvivor(order, hg(8)) || sk.Active != hg(4) {
		t.Fatalf("active %v, want g004", sk.Active)
	}
	keep(1, 9)
	if !sk.ActivateSurvivor(order, hg(3)) || sk.Active != hg(9) {
		t.Fatalf("active %v, want g009, the first left after it", sk.Active)
	}
	keep(1)
	if sk.ActivateSurvivor(order, hg(3)) {
		t.Fatal("activated the references")
	}
}

// Steps of undo that a change remembered itself fold into the one
// before them.
func TestUndoDropNewest(t *testing.T) {
	t.Parallel()
	sk := bolt(t)
	write := sk.Undo.Write
	sk.UndoRemember()
	sk.UndoRemember()
	if n := sk.UndoPushedSince(write); n != 2 {
		t.Fatalf("%d pushed, want 2", n)
	}
	sk.UndoDropNewest(1)
	if sk.Undo.Cnt != 1 || sk.UndoPushedSince(write) != 1 {
		t.Fatalf("%d steps", sk.Undo.Cnt)
	}
	sk.UndoUndo()
	if n := sk.UndoPushedSince(write + 1); n != 0 {
		t.Fatalf("an undo counts as %d pushed", n)
	}
	sk.UndoDropNewest(5)
	if sk.Undo.Cnt != 0 {
		t.Fatal("dropped below empty")
	}
}
