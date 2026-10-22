package sketch_test

import (
	"testing"

	"github.com/telesapr/telesapr/pkg/script/api"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// steppedBlock is a plate with a boss on it, the boss's height held.
func steppedBlock(keep bool) (*api.Doc, api.Group, api.Group) {
	d := api.New()
	sketchOf(d).NoSolidCache = !keep
	d.Sketch.Rect(api.V(0, 0, 0), api.V(40, 30, 0))
	plate := d.Features.Extrude(5, "plate")
	step(d)
	d.Features.SketchIn(d.Workplane("XY"), "boss sketch")
	d.Sketch.Circle(api.V(20, 15, 0), 6)
	boss := d.Features.Extrude(12, "boss")
	step(d)
	return d, plate, boss
}

// sketchOf is d's sketch, to read without a step of undo.
func sketchOf(d *api.Doc) *sketch.Sketch { return d.Host().(*api.SketchHost).Sketch() }

// solids is what each group's solid is, as text, and how many solids
// of groups that make one the regenerations so far made.
func solids(d *api.Doc) (text string, made int) {
	d.Groups() // a read: what is pending is regenerated first
	sk := sketchOf(d)
	for _, h := range sk.GroupOrder {
		text += shellText(&sk.Group(h).RunningShell)
	}
	if sk.Timings != nil {
		for _, g := range sk.Timings.Groups() {
			// A sketch passes the solid so far through: nothing is kept.
			if t := sk.Group(g.Group).Type; t != sketch.GroupDrawing3D && t != sketch.GroupDrawingWorkplane {
				made += g.Solids
			}
		}
	}
	return text, made
}

// An undo and a redo take every solid back from the cache, made from
// what each group was made from before, and give the solids that making
// them again gives.
func TestSolidCacheUndoRedo(t *testing.T) {
	t.Parallel()
	results := map[bool][]string{}
	for _, keep := range []bool{true, false} {
		d, plate, _ := steppedBlock(keep)
		sketchOf(d).Timings = &sketch.Timings{}
		before, _ := solids(d)
		plate.AsExtrude().SetDepth(8)
		step(d)
		after, was := solids(d)
		d.Edit.Undo()
		undone, made := solids(d)
		made -= was
		if undone != before {
			t.Errorf("kept %v: after the undo the solids are not those before the edit", keep)
		}
		if keep && made > 0 {
			t.Errorf("an undo made %d solids again, kept as they were", made)
		}
		d.Edit.Redo()
		redone, _ := solids(d)
		if redone != after {
			t.Errorf("kept %v: after the redo the solids are not those after the edit", keep)
		}
		hits, _ := sketchOf(d).SolidCacheStats()
		if keep != (hits > 0) {
			t.Errorf("kept %v: %d solids from the cache", keep, hits)
		}
		results[keep] = []string{before, after, undone, redone}
	}
	for i := range results[true] {
		if results[true][i] != results[false][i] {
			t.Errorf("step %d: kept and made differ", i)
		}
	}
}

// A ShellJob's copy of the sketch takes the solids made before from the
// sketch's cache, and those it makes are there for the sketch after.
func TestSolidCacheShellJob(t *testing.T) {
	t.Parallel()
	d, _, boss := steppedBlock(true)
	sk := sketchOf(d)
	before, _ := solids(d)
	was, _ := sk.SolidCacheStats()
	j := sk.NewShellJob()
	j.Run()
	if !sk.Apply(j) {
		t.Fatal("the job's solids weren't taken")
	}
	if hits, _ := sk.SolidCacheStats(); hits <= was {
		t.Errorf("the job took no solid from the cache: %d hits, %d before", hits, was)
	}
	if got, _ := solids(d); got != before {
		t.Error("the job's solids are not those the sketch made")
	}
	// The job makes the solids of an edit; the sketch then takes them.
	boss.AsExtrude().SetDepth(20)
	sk.MarkGroupDirty(boss.H, false)
	sk.DeferShells = true
	sk.GenerateAllTyped(sketch.GenerateDirty)
	sk.DeferShells = false
	j = sk.NewShellJob()
	j.Run()
	was, _ = sk.SolidCacheStats()
	sk.GenerateAll()
	if hits, _ := sk.SolidCacheStats(); hits <= was {
		t.Error("the sketch took nothing the job made")
	}
}

// With NoSolidCache nothing is kept or looked for.
func TestNoSolidCache(t *testing.T) {
	t.Parallel()
	d, plate, _ := steppedBlock(false)
	plate.AsExtrude().SetDepth(8)
	step(d)
	plate.AsExtrude().SetDepth(5)
	step(d)
	if hits, misses := sketchOf(d).SolidCacheStats(); hits != 0 || misses != 0 {
		t.Errorf("%d hits and %d misses with no cache", hits, misses)
	}
}
