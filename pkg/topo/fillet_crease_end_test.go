package topo_test

import (
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/topo"
)

// TestFilletSaddleOne: one of a saddle's four edges rounded alone, its
// end at the saddle's middle running into the two faces across it (the
// crease end), against OpenCASCADE's makeFillet
// (testdata/topo/fillet-saddle-one.txt,
// scripts/fillet-saddle-sharp-fixtures.py): each made, sound and within
// the meshes' chords of OpenCASCADE's volume, the two ends being the
// same surface: the chords grow with the radius, 0.023 mm³ at radius 2
// and slopes 0.8, which shrinks to 0.0058 at chord 0.0005% and 0.0029 at
// 0.0001% (2026-11-04). Before 2026-11-04 every one was refused.
func TestFilletSaddleOne(t *testing.T) {
	t.Parallel()
	saddleOne(t, "fillet-saddle-one.txt", false, func(r float64) float64 { return saddleChords * max(1, r) })
}

// TestChamferSaddleOne is TestFilletSaddleOne for chamfers, against
// OpenCASCADE's makeChamfer (testdata/topo/chamfer-saddle-one.txt):
// every face flat, so the volumes agree to rounding.
func TestChamferSaddleOne(t *testing.T) {
	t.Parallel()
	saddleOne(t, "chamfer-saddle-one.txt", true, func(float64) float64 { return 1e-6 })
}

// TestFilletSaddlePair: two opposite edges of a saddle rounded, its two
// convex edges or its two concave ones, the other two left sharp: the two
// fillets meet in a mitre in the plane of the two sharp edges (the crease
// pair), against OpenCASCADE's makeFillet
// (testdata/topo/fillet-saddle-pair.txt), within the meshes' chords and
// OpenCASCADE's mitre, a fitted B-spline where ours is the exact ellipse
// the two cylinders share: at radius 2 and slopes 0.5 and 0.8 0.056 mm³
// apart, 0.014 at chord 0.0005% and still 0.014 at 0.0001% (2026-11-04).
// Before 2026-11-04 every one was refused.
func TestFilletSaddlePair(t *testing.T) {
	t.Parallel()
	saddleOne(t, "fillet-saddle-pair.txt", false, func(r float64) float64 { return saddleChords*max(1, r) + 0.004*r*r*r })
}

// TestChamferSaddlePair is TestFilletSaddlePair for chamfers, against
// OpenCASCADE's makeChamfer (testdata/topo/chamfer-saddle-pair.txt).
func TestChamferSaddlePair(t *testing.T) {
	t.Parallel()
	saddleOne(t, "chamfer-saddle-pair.txt", true, func(float64) float64 { return 1e-6 })
}

// saddleOne checks the saddles of name, each line sx, sy, the radius or
// distance, the edges rounded (as mixedSaddle numbers them),
// OpenCASCADE's volume and its count of faces: made, sound and within
// allow of its volume.
func saddleOne(t *testing.T, name string, flat bool, allow func(r float64) float64) {
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "topo", name))
	if err != nil {
		t.Fatal(err)
	}
	var made atomic.Int32
	t.Run("lines", func(t *testing.T) {
		for _, line := range saddleLines(strings.Split(strings.TrimSpace(string(data)), "\n"), saddleEdgesRadii) {
			t.Run(line, func(t *testing.T) {
				t.Parallel()
				fs := strings.Fields(line)
				x := make([]float64, len(fs)-1)
				for i, f := range fs[:len(fs)-1] {
					y, err := strconv.ParseFloat(f, 64)
					if err != nil {
						t.Fatalf("%q: %v", line, err)
					}
					x[i] = y
				}
				r, want := x[2], x[len(x)-1]
				sh, all, err := mixedSaddle(x[0], x[1], [4]float64{r, r, r, r}, srf.Tol(0.002, 20))
				if err != nil {
					t.Fatal(err)
				}
				var picks []topo.FilletPick
				for _, e := range x[3 : len(x)-1] {
					picks = append(picks, all[int(e)])
				}
				if err := topo.FilletPicks(sh, picks, r, 1<<30, flat); err != nil {
					t.Errorf("refused: %v", err)
					return
				}
				got, err := soundVolume(sh)
				if err != nil {
					t.Fatal(err)
				}
				if d := math.Abs(got - want); d > allow(r) {
					t.Errorf("volume %.6f, OpenCASCADE's %.6f: %.6f off", got, want, d)
				}
				made.Add(1)
			})
		}
	})
	t.Logf("%d made", made.Load())
}
