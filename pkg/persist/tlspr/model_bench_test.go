package tlspr_test

import (
	"os"
	"testing"

	"github.com/telesapr/telesapr/pkg/persist/tlspr"
)

// BenchmarkRegenerateModel regenerates the model TELESAPR_BENCH_MODEL
// names, for a model too large to keep under testdata: the scaled
// step-and-repeat models of doc/notes/performance-study.md, which are
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
