package sketch_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/spacesolve/spacesolve/pkg/core"
	"github.com/spacesolve/spacesolve/pkg/persist/slvs"
	"github.com/spacesolve/spacesolve/pkg/sketch"
)

// bolt loads the bolt fixture: g002 head sketch, g003 head, g004 shank
// sketch, g005 shank, then the thread's g006 to g009 on the shank.
func bolt(t *testing.T) *sketch.Sketch {
	t.Helper()
	doc, err := slvs.LoadFile(fixturesPath(t, "thread", "bolt_m8x30.slvs"))
	if err != nil {
		t.Fatal(err)
	}
	return doc.Sketch
}

func hg(v uint32) core.HGroup { return core.HGroup{V: v} }

func TestGroupDependencies(t *testing.T) {
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
	hc := sk.AddConstraint(core.Constraint{ConstraintBase: core.ConstraintBase{
		Type: core.ConstraintPointsCoincident, Group: hg(4), Workplane: core.FreeIn3D,
		PtA: shank.Entity(1), PtB: hg(3).Entity(5),
	}})
	deps := sk.GroupDependencies(hg(4))
	if len(deps) != 2 || deps[1].Group != hg(3) || !strings.HasPrefix(deps[1].Why, "constraint c") {
		t.Fatalf("g004 with constraint %v depends on %v", hc, deps)
	}

	// A copy's remap names its source, even without an operand.
	g := sk.GetGroup(hg(3))
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
