package sketch_test

import (
	"reflect"
	"testing"

	"github.com/telesapr/telesapr/pkg/sketch"
)

// pacer is the Hooks of a sketch whose owner has one thread, as a
// browser page has: it counts how often a group's Boolean gives it back.
type pacer struct {
	sketch.NoHooks
	paces int
}

func (p *pacer) KernelPace() func() { return func() { p.paces++ } }

// TestKernelPace checks that a sketch whose Hooks are a KernelPacer
// has the Booleans of its groups call it while they run, not only
// between groups, and that the solids are those made without it.
func TestKernelPace(t *testing.T) {
	t.Parallel()
	_, sk, cut := jobModel(t)
	want := sk.Group(cut.H).RunningShell

	// Made again, not taken from the cache of solids.
	sk.NoSolidCache = true
	p := &pacer{}
	sk.Hooks = p
	for i := range sk.Groups {
		sk.Groups[i].Clean = false
	}
	sk.GenerateAllTyped(sketch.GenerateAll)
	if p.paces == 0 {
		t.Fatal("the cut's Boolean never paced")
	}
	if got := sk.Group(cut.H).RunningShell; !reflect.DeepEqual(got, want) {
		t.Error("paced, the cut made another solid")
	}
}
