package topo_test

import (
	"fmt"
	"slices"
	"sync"
	"testing"

	"github.com/telesapr/telesapr/pkg/check"
	"github.com/telesapr/telesapr/pkg/internal/sweep"
	"github.com/telesapr/telesapr/pkg/srf"
)

// TestChamferConeSweep chamfers some of the edges at a pyramid's apex,
// convex, and at a pyramid's pocket's, concave, the rest left sharp, and
// all three at a ledge's top and a rib's foot, where convex edges meet
// a concave one, or a ledge's top edge alone, or with the concave edge
// below it, and a rib's upright with a foot edge: each made, sound and
// closed. The rate allowed, 0, was
// fixed before the sweep first ran.
func TestChamferConeSweep(t *testing.T) {
	t.Parallel()
	grid := sweep.Grid{
		Parallel: true,
		Axes: []sweep.Axis{
			sweep.Strings("corner", "apexsome", "pocketsome", "ledgeall", "riball", "ledgeone", "ledgetwo", "ribtwo"),
			sweep.Strings("picks", "1000", "1100", "1010", "1110"),
			sweep.Floats("d1", 0.5, 1.25, 2),
			sweep.Floats("d2", 0.5, 1.25, 2),
			sweep.Floats("d3", 0.5, 2),
		},
		MaxFailRate: 0,
		Sample:      24,
	}
	grid.Run(func(p sweep.Point) error {
		shape := p.String("corner")
		d := []float64{p.Float("d1"), p.Float("d2"), p.Float("d3")}
		if shape == "apexsome" || shape == "pocketsome" {
			d = append(d, 0)
			for i, c := range p.String("picks") {
				if c == '0' {
					d[i] = 0
				}
			}
		}
		sh, err := chamferOracleCase(shape, d, srf.Tol(0.002, 20))
		if err != nil {
			return err
		}
		var m srf.SMesh
		sh.TriangulateInto(&m)
		if rep := check.Solid(sh, &m); !rep.Sound() {
			return fmt.Errorf("unsound: %s", rep.String())
		}
		return nil
	}).Require(t)
}

// TestChamferConeSameEveryRun builds corners of chamfers with edges left
// sharp, at a pyramid's apex and its pocket's, several times at once,
// and requires the same solid to the bit each time: the corner was once
// clipped by its planes in a map's order, which changed from run to run.
func TestChamferConeSameEveryRun(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		shape string
		d     []float64
	}{
		{"apexsome", []float64{1, 1, 1, 0}},
		{"apexsome", []float64{1, 0, 0.5, 0}},
		{"apexsome", []float64{0.5, 1, 1.5, 0}},
		{"pocketsome", []float64{1, 1, 1, 0}},
		{"pocketsome", []float64{2, 0, 1, 0}},
	} {
		t.Run(fmt.Sprint(c.shape, c.d), func(t *testing.T) {
			t.Parallel()
			const runs = 6
			shells := make([]*srf.SShell, runs)
			errs := make([]error, runs)
			var wg sync.WaitGroup
			for i := range runs {
				wg.Go(func() { shells[i], errs[i] = chamferOracleCase(c.shape, c.d, srf.Tol(0.002, 20)) })
			}
			wg.Wait()
			for i := range runs {
				if errs[i] != nil {
					t.Fatal(errs[i])
				}
				if i > 0 && !sameShell(shells[0], shells[i]) {
					t.Fatalf("run %d made a different solid from run 0's", i)
				}
			}
		})
	}
}

// sameShell reports whether a and b have the same surfaces, trims,
// curves and mesh, to the bit.
func sameShell(a, b *srf.SShell) bool {
	if len(a.Surface) != len(b.Surface) || len(a.Curve) != len(b.Curve) {
		return false
	}
	for i := range a.Surface {
		s, o := &a.Surface[i], &b.Surface[i]
		if s.DegM != o.DegM || s.DegN != o.DegN || s.Ctrl != o.Ctrl || s.Weight != o.Weight || s.Face != o.Face ||
			!slices.Equal(s.Trim, o.Trim) {
			return false
		}
	}
	for i := range a.Curve {
		if !slices.Equal(a.Curve[i].Points, b.Curve[i].Points) {
			return false
		}
	}
	var ma, mb srf.SMesh
	a.TriangulateInto(&ma)
	b.TriangulateInto(&mb)
	return slices.Equal(ma.Triangles, mb.Triangles)
}
