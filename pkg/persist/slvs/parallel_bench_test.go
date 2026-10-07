package slvs

import (
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/srf"
)

// benchmarkRegenerate times generating a fixture over again, with the
// step and repeat groups named making the given number of copies, in
// each of the modes.
func benchmarkRegenerate(b *testing.B, name string, copies map[uint32]float64) {
	for _, mode := range []struct {
		name string
		mode srf.ParallelMode
	}{{"off", srf.ParallelOff}, {"on", srf.ParallelOn}} {
		b.Run(mode.name, func(b *testing.B) {
			srf.SetParallelMode(mode.mode)
			defer srf.SetParallelMode(srf.ParallelOn)
			doc, err := LoadFile(fixture(name))
			if err != nil {
				b.Fatal(err)
			}
			sk := doc.Sketch
			for h, n := range copies {
				sk.Group(core.HGroup{V: h}).ValA = n
			}
			sk.GenerateAllTyped(sketch.GenerateAll)
			b.ResetTimer()
			for b.Loop() {
				sk.GenerateAllTyped(sketch.GenerateAll)
			}
		})
	}
}

// Extruded text: many surfaces, most of them curved.
func BenchmarkRegenerateText(b *testing.B) {
	benchmarkRegenerate(b, "request/ttf_text/extrude.slvs", nil)
}

// A step and repeat of a step and repeat, 12 by 12 copies.
func BenchmarkRegenerateTranslate(b *testing.B) {
	benchmarkRegenerate(b, "group/translate_nd/normal.slvs", map[uint32]float64{6: 12, 7: 12})
}

// A Boolean of solids with curved surfaces.
func BenchmarkRegenerateFillet(b *testing.B) {
	benchmarkRegenerate(b, "group/boolean_tangent_fillet/normal.slvs", nil)
}
