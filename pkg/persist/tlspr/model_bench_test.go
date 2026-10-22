package tlspr_test

import (
	"os"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/persist/tlspr"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// BenchmarkRegenerateModel regenerates the model TELESAPR_BENCH_MODEL
// names, for a model too large to keep under testdata: the scaled
// step-and-repeat models of doc/notes/performance.md, which are
// made from testdata/group/translate_nd/normal.slvs as that note says,
// converted.
// It is skipped when the variable is not set.
func BenchmarkRegenerateModel(b *testing.B) {
	file := os.Getenv("TELESAPR_BENCH_MODEL")
	if file == "" {
		b.Skip("TELESAPR_BENCH_MODEL names no model")
	}
	doc, err := tlspr.LoadFile(file)
	if err != nil {
		b.Fatal(err)
	}
	sk := doc.Sketch
	sk.GenerateAll()
	b.ResetTimer()
	for range b.N {
		sk.GenerateAll()
	}
}

// editModel is the model TELESAPR_BENCH_MODEL names, or a screw with
// three threads, regenerated, with a dimension of its first sketch
// that every solid after it follows.
func editModel(b *testing.B, keep bool) (*sketch.Sketch, *core.Constraint) {
	file := os.Getenv("TELESAPR_BENCH_MODEL")
	if file == "" {
		file = "../../../testdata/thread/triple_m12.tlspr"
	}
	doc, err := tlspr.LoadFile(file)
	if err != nil {
		b.Fatal(err)
	}
	sk := doc.Sketch
	sk.NoSolidCache = !keep
	sk.GenerateAll()
	for i := range sk.Constraints {
		if c := &sk.Constraints[i]; c.ValA > 0 && !c.Reference {
			return sk, c
		}
	}
	b.Skip("the model has no dimension")
	return nil, nil
}

// BenchmarkEditBack sets a dimension of the model's first sketch to one
// value and then back, regenerating after each: an edit tried and taken
// back, with the solids kept by what they were made from and without.
func BenchmarkEditBack(b *testing.B) {
	for _, keep := range []bool{true, false} {
		b.Run(map[bool]string{true: "kept", false: "made"}[keep], func(b *testing.B) {
			sk, c := editModel(b, keep)
			h, was := c.H, c.ValA
			b.ResetTimer()
			for i := range b.N {
				c := sk.Constraint(h)
				c.ValA = was * (1 + 0.05*float64(i%2))
				sk.MarkGroupDirty(c.Group, false)
				sk.GenerateAllTyped(sketch.GenerateDirty)
			}
		})
	}
}

// BenchmarkUndoRedo undoes an edit of the model's first sketch and
// redoes it, each a full regeneration.
func BenchmarkUndoRedo(b *testing.B) {
	for _, keep := range []bool{true, false} {
		b.Run(map[bool]string{true: "kept", false: "made"}[keep], func(b *testing.B) {
			sk, c := editModel(b, keep)
			sk.UndoRemember()
			c.ValA *= 1.05
			sk.MarkGroupDirty(c.Group, false)
			sk.GenerateAllTyped(sketch.GenerateDirty)
			b.ResetTimer()
			for range b.N {
				sk.UndoUndo()
				sk.UndoRedo()
			}
		})
	}
}
