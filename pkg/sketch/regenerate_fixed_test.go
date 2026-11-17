package sketch_test

import (
	"fmt"
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/internal/sweep"
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/srf"
)

// solidPrint is what a regeneration gave a sketch, to the bit: its
// solved values and each group's solid's mesh. Each solid is meshed
// twice, and must give the same mesh both times.
func solidPrint(t *testing.T, sk *sketch.Sketch) []string {
	t.Helper()
	var out []string
	for _, p := range sk.Params {
		out = append(out, fmt.Sprintf("param %x %x", p.H.V, math.Float64bits(p.Val)))
	}
	for i := range sk.Groups {
		g := &sk.Groups[i]
		var mesh [2]string
		for k := range mesh {
			var m srf.SMesh
			g.RunningShell.TriangulateInto(&m)
			mesh[k] = fmt.Sprintf("group %x: %d triangles, volume %x",
				g.H.V, len(m.Triangles), math.Float64bits(m.CalculateVolume()))
		}
		if mesh[0] != mesh[1] {
			t.Errorf("meshed again: %s, was %s", mesh[1], mesh[0])
		}
		out = append(out, mesh[0])
	}
	return out
}

// TestRegenerateIsFixedPoint regenerates models of the corpus twice
// and requires the same solved values and the same solids, meshed the
// same each time they are meshed. A solve that moved the last bits at
// each regeneration, or a mesh or a Boolean steered by where the last
// projection on a surface ended (srf's closest-point guess), made each
// regeneration of a model as a pen cap's tube a little different, its
// volume flipping between two values.
func TestRegenerateIsFixedPoint(t *testing.T) {
	t.Parallel()
	for _, rel := range sweep.Pick(corpus(t), 40) {
		t.Run(rel, func(t *testing.T) {
			t.Parallel()
			doc, err := openCorpus(rel)
			if err != nil {
				t.Fatal(err)
			}
			sk := doc.Sketch
			sk.GenerateAll()
			first := solidPrint(t, sk)
			sk.GenerateAll()
			again := solidPrint(t, sk)
			if len(first) != len(again) {
				t.Fatalf("%d lines, then %d", len(first), len(again))
			}
			for i := range first {
				if first[i] != again[i] {
					t.Fatalf("regenerated again: %s, was %s", again[i], first[i])
				}
			}
		})
	}
}
