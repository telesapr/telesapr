package api

import (
	"testing"
)

// zigzag draws a polyline of n segments, each one horizontal or vertical in turn, which leaves the start point free.
func zigzag(n int) *Doc {
	d := New()
	pts := make([]Vec, n+1)
	for i := range pts {
		pts[i] = V(float64(i/2*10+i%2*10), float64((i+1)/2*10), 0)
	}
	d.Batch(func() {
		segs := d.Sketch.Polyline(false, pts...)
		for i, s := range segs {
			if i%2 == 0 {
				d.Constrain.Horizontal(s)
			} else {
				d.Constrain.Vertical(s)
			}
		}
	})
	d.Edit.Regen()
	return d
}

// BenchmarkFreedom150 is what colouring by constraint state adds to a
// regeneration of a sketch of 150 segments (about 600 unknowns);
// BenchmarkRegen150 is a regeneration of it.
func BenchmarkFreedom150(b *testing.B) {
	d := zigzag(150)
	sk := d.h.Sketch()
	hg := sk.ActiveGroup()
	b.ResetTimer()
	for range b.N {
		sk.Generation++
		if sk.GroupFreedom(hg) == nil {
			b.Fatal("no freedom")
		}
	}
}

func BenchmarkRegen150(b *testing.B) {
	d := zigzag(150)
	sk := d.h.Sketch()
	b.ResetTimer()
	for range b.N {
		sk.GenerateAll()
	}
}
