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

// TestFilletSaddlePairUnlike: two opposite edges of a saddle rounded at
// unlike radii, the other two left sharp: the two fillets' ends lie apart
// in the plane of the sharp edges, and the plane between them is a step
// (creaseStep), where OpenCASCADE's makeFillet blends them with a B-spline
// (testdata/topo/fillet-saddle-pair-unlike.txt). Each is made and sound;
// the saddle is the same either side of that plane, so its volume is the
// mean of the two pairs at one radius each, within the meshes' chords;
// and it is within stepAllow of OpenCASCADE's, whose blend reaches past
// the plane: up to 0.94 mm³ (0.12 r³) apart, at slopes 0.8 and 0.5,
// radii 0.5 and 2, where the step is 0.03 from the two pairs' mean
// (2026-11-05). Before 2026-11-05 every one was refused.
func TestFilletSaddlePairUnlike(t *testing.T) {
	t.Parallel()
	saddlePairUnlike(t, "fillet-saddle-pair-unlike.txt", false, func(r float64) float64 { return saddleChords * max(1, r) },
		func(r float64) float64 { return 0.13 * r * r * r })
}

// TestChamferSaddlePairUnlike is TestFilletSaddlePairUnlike for
// chamfers at unlike distances, against OpenCASCADE's makeChamfer
// (testdata/topo/chamfer-saddle-pair-unlike.txt): every face flat, so the
// mean of the two pairs to rounding, and up to 0.39 mm³ (0.049 d³) from
// OpenCASCADE's, whose corner is set back less (2026-11-05).
func TestChamferSaddlePairUnlike(t *testing.T) {
	t.Parallel()
	saddlePairUnlike(t, "chamfer-saddle-pair-unlike.txt", true, func(float64) float64 { return 1e-6 },
		func(r float64) float64 { return 0.055 * r * r * r })
}

// saddlePairUnlike checks the saddles of name, each line sx, sy, the two
// edges rounded (as mixedSaddle numbers them), their radii or distances,
// OpenCASCADE's volume and its count of faces: made, sound, within mean
// of the larger radius of the mean of the pairs at one radius each, and
// within occ of OpenCASCADE's volume.
func saddlePairUnlike(t *testing.T, name string, flat bool, mean, occ func(r float64) float64) {
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "topo", name))
	if err != nil {
		t.Fatal(err)
	}
	var made atomic.Int32
	t.Run("lines", func(t *testing.T) {
		for _, line := range saddleLines(strings.Split(strings.TrimSpace(string(data)), "\n"), saddlePairUnlikeRadii) {
			t.Run(line, func(t *testing.T) {
				t.Parallel()
				fs := strings.Fields(line)
				x := make([]float64, len(fs))
				for i, f := range fs {
					y, err := strconv.ParseFloat(f, 64)
					if err != nil {
						t.Fatalf("%q: %v", line, err)
					}
					x[i] = y
				}
				e0, e1, r0, r1, want := int(x[2]), int(x[3]), x[4], x[5], x[6]
				rs := [4]float64{1, 1, 1, 1}
				rs[e0], rs[e1] = r0, r1
				sh, all, err := mixedSaddle(x[0], x[1], rs, srf.Tol(0.002, 20))
				if err != nil {
					t.Fatal(err)
				}
				if err := topo.FilletPicks(sh, []topo.FilletPick{all[e0], all[e1]}, r0, 1<<30, flat); err != nil {
					t.Errorf("refused: %v", err)
					return
				}
				got, err := soundVolume(sh)
				if err != nil {
					t.Fatal(err)
				}
				var pairs float64
				for _, r := range []float64{r0, r1} {
					rs := [4]float64{r, r, r, r}
					sh, all, err := mixedSaddle(x[0], x[1], rs, srf.Tol(0.002, 20))
					if err != nil {
						t.Fatal(err)
					}
					if err := topo.FilletPicks(sh, []topo.FilletPick{all[e0], all[e1]}, r, 1<<30, flat); err != nil {
						t.Fatalf("the pair at %g refused: %v", r, err)
					}
					v, err := soundVolume(sh)
					if err != nil {
						t.Fatal(err)
					}
					pairs += v / 2
				}
				r := max(r0, r1)
				if d := math.Abs(got - pairs); d > mean(r) {
					t.Errorf("volume %.6f, the pairs' mean %.6f: %.6f off", got, pairs, d)
				}
				if d := math.Abs(got - want); d > occ(r) {
					t.Errorf("volume %.6f, OpenCASCADE's %.6f: %.6f off", got, want, d)
				}
				made.Add(1)
			})
		}
	})
	t.Logf("%d made", made.Load())
}
