package topo_test

import (
	"testing"

	"github.com/telesapr/telesapr/pkg/internal/fixtures"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/topo"
)

// TestFilletSaddleSide: a ridge and a valley side by side of a saddle's
// four edges rounded, the other two left sharp, against OpenCASCADE's
// makeFillet (testdata/topo/fillet-saddle-side.txt,
// scripts/fillet-saddle-sharp-fixtures.py), which fills the corner with
// one five-sided patch: each made, sound and within the allowance of
// the saddles with one edge left sharp, the two corners being different
// surfaces; at most 0.051 r³ apart, at slopes 0.8 and 0.5 (2026-11-04).
// Before 2026-11-04 every one was refused.
func TestFilletSaddleSide(t *testing.T) {
	t.Parallel()
	saddleOne(t, "fillet-saddle-side.txt", false, func(r float64) float64 { return saddleSharpAllow(r) + saddleChords })
}

// TestChamferSaddleSide is TestFilletSaddleSide for chamfers, against
// OpenCASCADE's makeChamfer (testdata/topo/chamfer-saddle-side.txt): the
// patches between the chamfers are curved as a fillet's, OpenCASCADE's
// a flatter B-spline, so the corners are up to 1.47 d² apart at slopes
// 0.8 and d 2, within chamferSaddleAllow as the saddles with one edge
// left sharp (2026-11-04).
func TestChamferSaddleSide(t *testing.T) {
	t.Parallel()
	saddleOne(t, "chamfer-saddle-side.txt", true, func(d float64) float64 { return chamferSaddleAllow * d * d })
}

// TestFilletSaddleSideUnlike: the two side by side at unlike radii, both
// kinds, both ways round, OpenCASCADE gives no fixture for: each made
// and sound, its volume the like radii's but for what the grown edge
// adds or takes away rounded alone, within the allowance of the
// corners, which differ.
func TestFilletSaddleSideUnlike(t *testing.T) {
	t.Parallel()
	flats, twos := []bool{false, true}, [][2]int{{0, 2}, {0, 3}, {1, 2}, {1, 3}}
	if fixtures.RaceDetector {
		// Four saddles rounded a case, minutes there: one of each kind.
		twos = twos[:1]
	}
	for _, flat := range flats {
		for _, two := range twos {
			vol := func(rs [4]float64, edges ...int) float64 {
				sh, all, err := mixedSaddle(0.5, 0.5, rs, srf.Tol(0.002, 20))
				if err != nil {
					t.Fatal(err)
				}
				var picks []topo.FilletPick
				for _, e := range edges {
					picks = append(picks, all[e])
				}
				if err := topo.FilletPicks(sh, picks, 1, 1<<30, flat); err != nil {
					t.Fatalf("flat %v edges %v radii %v: %v", flat, edges, rs, err)
				}
				got, err := soundVolume(sh)
				if err != nil {
					t.Fatalf("flat %v edges %v radii %v: %v", flat, edges, rs, err)
				}
				return got
			}
			one, grown := [4]float64{1, 1, 1, 1}, [4]float64{1, 1, 1, 1}
			grown[two[1]] = 2
			got := vol(grown, two[0], two[1])
			want := vol(one, two[0], two[1]) + vol(grown, two[1]) - vol(one, two[1])
			allow := saddleSharpAllow(2) + saddleChords
			if flat {
				allow = chamferSaddleAllow * 4
			}
			if d := got - want; d > allow || d < -allow {
				t.Errorf("flat %v edges %v: volume %.4f, want %.4f", flat, two, got, want)
			}
		}
	}
}
