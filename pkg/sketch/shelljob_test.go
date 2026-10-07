package sketch_test

import (
	"reflect"
	"sync"
	"testing"

	"github.com/telesapr/telesapr/pkg/script/api"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// jobModel is a block with a hole cut through it, regenerated.
func jobModel(t *testing.T) (*api.Doc, *sketch.Sketch, api.Group) {
	t.Helper()
	d := api.New()
	block(d, 0, 0, 20, 10, 10)
	d.SketchIn(d.Workplane("XY"), "")
	d.Circle(api.V(5, 5, 0), 2)
	cut := d.Extrude(10, "cut")
	cut.SetCombine(api.Difference)
	d.Regen()
	return d, d.Host().Sketch(), cut
}

// Solids off the thread (roadmap 17.3): a regeneration that defers
// them leaves the solids as they were, marked; a job on a copy, run on
// another goroutine while the sketch is regenerated again on this one,
// makes the solids a regeneration in full makes, and Apply puts them in.
// A sketch changed since, or a job cancelled, applies nothing.
func TestShellJob(t *testing.T) {
	d, sk, cut := jobModel(t)
	want := sk.Group(cut.H).RunningShell

	// The hole moved: the entities follow, the solid waits.
	for i := range sk.Groups {
		sk.Groups[i].Clean = false
	}
	sk.DeferShells = true
	sk.GenerateAllTyped(sketch.GenerateAll)
	sk.DeferShells = false
	if !sk.AnyShellsStale() {
		t.Fatal("nothing is stale after a deferred regeneration")
	}
	job := sk.NewShellJob()
	var wg sync.WaitGroup
	wg.Go(func() {
		job.Run()
	})
	// The owner regenerates meanwhile, deferring again, as dragging does.
	sk.DeferShells = true
	sk.GenerateAllTyped(sketch.GenerateDirty)
	sk.DeferShells = false
	wg.Wait()
	if !sk.Apply(job) {
		t.Fatal("the job wasn't applied")
	}
	if sk.AnyShellsStale() {
		t.Error("stale after the job")
	}
	if got := sk.Group(cut.H).RunningShell; !reflect.DeepEqual(got, want) {
		t.Error("the job's solid differs from a regeneration in full")
	}
	if v := cut.Check().Volume; v <= 0 {
		t.Errorf("volume %g after the job", v)
	}

	// Changed since: refused.
	job = sk.NewShellJob()
	job.Run()
	cut.SetCombine(api.Union)
	if sk.Apply(job) {
		t.Error("a job of a sketch changed since was applied")
	}
	// Cancelled: refused.
	job = sk.NewShellJob()
	job.Cancel()
	job.Run()
	if !job.Cancelled || sk.Apply(job) {
		t.Error("a cancelled job was applied")
	}
	_ = d
}
