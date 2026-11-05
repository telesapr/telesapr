package sketch_test

import (
	"reflect"
	"testing"

	"github.com/telesapr/telesapr/pkg/script/api"
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

// TestKernelPaceFeatures checks that the pace reaches a feature's own
// work, not only the group's Boolean: a fillet (its rolling ball and
// the check of its result) and a shell (its offsets) pace while they
// run, make the solids made without a pace, and leave the groups'
// shells with none.
func TestKernelPaceFeatures(t *testing.T) {
	t.Parallel()
	fillet := func(d *api.Doc) api.Group {
		ex := block(d, 0, 0, 20, 10, 10)
		edge := lineAlong(t, ex, api.V(0, 0, 10), api.V(20, 0, 10))
		step(d)
		return d.Features.Fillet([]api.Entity{edge}, 2, "")
	}
	shell := func(d *api.Doc) api.Group {
		ex := block(d, 0, 0, 20, 10, 10)
		top := ex.Solid().FaceAt(api.V(10, 5, 10))
		step(d)
		return d.Features.Shell([]api.Entity{top.Entity()}, 1, "")
	}
	for name, build := range map[string]func(*api.Doc) api.Group{"fillet": fillet, "shell": shell} {
		d := api.New()
		g := build(d)
		d.Edit.Regen()
		if p := g.Problem(); p != "" {
			t.Fatalf("%s: %s", name, p)
		}
		sk := d.Host().Sketch()
		// Made again, not taken from the cache of solids, unpaced and
		// then paced: the first solids hold what the searches filled.
		sk.NoSolidCache = true
		again := func() {
			for i := range sk.Groups {
				sk.Groups[i].Clean = false
			}
			sk.GenerateAllTyped(sketch.GenerateAll)
		}
		again()
		want := sk.Group(g.H).RunningShell
		p := &pacer{}
		sk.Hooks = p
		again()
		if p.paces == 0 {
			t.Errorf("%s: never paced", name)
		}
		for i := range sk.Groups {
			gr := &sk.Groups[i]
			if gr.ThisShell.Pace() != nil || gr.RunningShell.Pace() != nil {
				t.Errorf("%s: group %d kept a pace", name, gr.H.V)
			}
		}
		if got := sk.Group(g.H).RunningShell; !reflect.DeepEqual(got, want) {
			t.Errorf("%s: paced, it made another solid", name)
		}
	}
}
