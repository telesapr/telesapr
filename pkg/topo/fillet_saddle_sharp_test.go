package topo_test

import (
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/telesapr/telesapr/pkg/internal/sweep"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/topo"
)

// saddleWithout is mixedSaddle at radius r with the edge sharp (0 and
// 1 its convex edges towards +x and -x, 2 and 3 its concave ones towards
// +y and -y) left sharp: the saddle's other three picks.
func saddleWithout(sx, sy, r float64, sharp int, tol srf.Tolerance) (*srf.SShell, []topo.FilletPick, error) {
	sh, picks, err := mixedSaddle(sx, sy, [4]float64{r, r, r, r}, tol)
	if err != nil {
		return nil, nil, err
	}
	return sh, append(append([]topo.FilletPick{}, picks[:sharp]...), picks[sharp+1:]...), nil
}

// TestFilletSaddleSharp rounds three of a saddle's four edges at its
// corner, the fourth left sharp, over a grid of slopes and radii and
// each edge left sharp in turn: made and sound, holding more than the
// saddle with all four rounded where a convex edge is left sharp and
// less where a concave one is, or refused at the corner. Since
// 2026-11-04 84 of the full grid's 108 are made (saddleSharpAt), and
// with the steep ones' ladders since, all 108; before, every one was
// refused.
func TestFilletSaddleSharp(t *testing.T) {
	t.Parallel()
	grid := sweep.Grid{
		Parallel: true,
		Axes: []sweep.Axis{
			sweep.Floats("sx", 0.2, 0.5, 0.8),
			sweep.Floats("sy", 0.2, 0.5, 0.8),
			sweep.Floats("r", 0.5, 1, 2),
			sweep.Floats("sharp", 0, 1, 2, 3),
		},
		MaxFailRate: 0,
		Sample:      12,
	}
	var made, bad atomic.Int32
	grid.Run(func(p sweep.Point) error {
		sx, sy, r, sharp := p.Float("sx"), p.Float("sy"), p.Float("r"), int(p.Float("sharp"))
		tol := srf.Tol(0.002, 20)
		all, allPicks, _ := mixedSaddle(sx, sy, [4]float64{r, r, r, r}, tol)
		if err := topo.FilletPicks(all, allPicks, r, 1<<30, false); err != nil {
			return fmt.Errorf("all four rounded: %w", err)
		}
		four, err := soundVolume(all)
		if err != nil {
			return fmt.Errorf("all four rounded: %w", err)
		}
		sh, picks, err := saddleWithout(sx, sy, r, sharp, tol)
		if err != nil {
			return err
		}
		if err := topo.FilletPicks(sh, picks, r, 1<<30, false); err != nil {
			if _, ok := errors.AsType[*topo.FilletError](err); !ok {
				bad.Add(1)
				t.Errorf("%v: refused without a place: %v", p, err)
			}
			return nil
		}
		vol, err := soundVolume(sh)
		if err != nil {
			bad.Add(1)
			t.Errorf("%v: made unsound: %v", p, err)
			return err
		}
		// A convex edge left sharp keeps what its fillet would take
		// away; a concave one lacks what its fillet would fill.
		if convex := sharp < 2; convex != (vol > four) {
			bad.Add(1)
			t.Errorf("%v: volume %.4f against %.4f with all four rounded", p, vol, four)
		}
		made.Add(1)
		return nil
	}).Require(t)
	t.Logf("%d made", made.Load())
	if bad.Load() > 0 {
		t.Errorf("%d saddles made wrong", bad.Load())
	}
}

// TestFilletSaddleSharpOracle: the saddles OpenCASCADE's makeFillet
// rounds with an edge left sharp (testdata/topo/fillet-saddle-sharp.txt,
// scripts/fillet-saddle-sharp-fixtures.py), each made or refused here;
// where made, its volume within saddleSharpAllow of OpenCASCADE's,
// whose corner is one vertex blend against our three patches or four,
// and the meshes' chords besides (saddleChords). Measured 2026-11-04:
// all 58 of the five gentler slopes made, within 0.056 r³ at radius 1
// and over, and at 0.5 within 0.016 mm³, which at chord 0.0001% shrinks
// to 0.002; with the steep slopes since (ladders, saddleSharpAt), all
// 106 of OpenCASCADE's lines made, within 0.0643 r³.
func TestFilletSaddleSharpOracle(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "topo", "fillet-saddle-sharp.txt"))
	if err != nil {
		t.Fatal(err)
	}
	var made atomic.Int32
	t.Run("lines", func(t *testing.T) {
		// Under the race detector three lines (saddleLines): the
		// corner is made on the same goroutines for each.
		for _, line := range saddleLines(strings.Split(strings.TrimSpace(string(data)), "\n")) {
			t.Run(line, func(t *testing.T) {
				t.Parallel()
				var x [5]float64
				for i, f := range strings.Fields(line)[:5] {
					y, err := strconv.ParseFloat(f, 64)
					if err != nil {
						t.Fatalf("%q: %v", line, err)
					}
					x[i] = y
				}
				sh, picks, err := saddleWithout(x[0], x[1], x[2], int(x[3]), srf.Tol(0.002, 20))
				if err != nil {
					t.Fatal(err)
				}
				if err := topo.FilletPicks(sh, picks, x[2], 1<<30, false); err != nil {
					if !errors.As(err, new(*topo.FilletError)) {
						t.Errorf("refused without a place: %v", err)
					}
					return
				}
				got, err := soundVolume(sh)
				if err != nil {
					t.Fatal(err)
				}
				if d := math.Abs(got - x[4]); d > saddleSharpAllow(x[2])+saddleChords {
					t.Errorf("volume %.6f, OpenCASCADE's %.6f: %.4f r³ off", got, x[4], d/(x[2]*x[2]*x[2]))
				}
				made.Add(1)
			})
		}
	})
	t.Logf("%d made", made.Load())
}

// saddleChords is how far the meshes' chords at 0.002% move a saddle's
// volume: 0.016 mm³ at radius 0.5 against OpenCASCADE's, 0.004 at
// 0.0005%, 0.002 at 0.0001%.
const saddleChords = 0.02

// saddleSharpAllow is how far a saddle's volume with an edge left sharp
// may be from OpenCASCADE's at radius r: its vertex blend and our
// patches are different surfaces, ours set back further, 0.0643 r³
// apart at the most of 2026-11-04 (radius 2, slopes 0.8).
func saddleSharpAllow(r float64) float64 { return 0.07 * r * r * r }

// TestFilletSaddleSharpUnlike: the saddles of unlike radii OpenCASCADE's
// makeFillet rounds with an edge left sharp
// (testdata/topo/fillet-saddle-sharp-unlike.txt,
// scripts/fillet-saddle-sharp-fixtures.py), the two fillets beside the
// sharp edge of unlike radii, so reaching it at two points: each made,
// sound and within saddleSharpAllow of OpenCASCADE's at the largest
// radius. Before 2026-11-04 every one was refused; since, all 112 are
// made (saddleSharpAt's ladder).
func TestFilletSaddleSharpUnlike(t *testing.T) {
	t.Parallel()
	saddleSharpUnlike(t, "fillet-saddle-sharp-unlike.txt", false, saddleSharpAllow)
}

// TestChamferSaddleSharpUnlike is TestFilletSaddleSharpUnlike for
// chamfers of unlike distances, against OpenCASCADE's makeChamfer
// (testdata/topo/chamfer-saddle-sharp-unlike.txt), within
// chamferSaddleAllow.
func TestChamferSaddleSharpUnlike(t *testing.T) {
	t.Parallel()
	saddleSharpUnlike(t, "chamfer-saddle-sharp-unlike.txt", true,
		func(d float64) float64 { return chamferSaddleAllow * d * d })
}

// occSaddleMirrors are OpenCASCADE's lines whose volume is not its own
// mirror image's, which by the saddle's symmetry it must be: each is
// checked against the mirror's volume instead (y to -y, which swaps
// edges 2 and 3; 6.3 mm³ apart at slopes 0.8, 2026-11-04).
var occSaddleMirrors = map[string]string{
	"0.8 0.8 1 2 1 0 3": "0.8 0.8 1 2 0 1 2",
}

// saddleSharpUnlike checks the saddles of name, each line sx, sy, the
// four edges' radii or distances (0 for the one left sharp), its number,
// and OpenCASCADE's volume: made, sound and within allow at the largest.
func saddleSharpUnlike(t *testing.T, name string, flat bool, allow func(float64) float64) {
	if testing.Short() {
		t.Skip("rounds a hundred saddles of unlike radii; skipped under -short")
	}
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "topo", name))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	occ := map[string]float64{}
	for _, line := range lines {
		fs := strings.Fields(line)
		v, err := strconv.ParseFloat(fs[7], 64)
		if err != nil {
			t.Fatalf("%q: %v", line, err)
		}
		occ[strings.Join(fs[:7], " ")] = v
	}
	var made atomic.Int32
	t.Run("lines", func(t *testing.T) {
		for _, line := range saddleLines(lines) {
			t.Run(line, func(t *testing.T) {
				t.Parallel()
				fs := strings.Fields(line)
				var x [7]float64
				for i, f := range fs[:7] {
					y, err := strconv.ParseFloat(f, 64)
					if err != nil {
						t.Fatalf("%q: %v", line, err)
					}
					x[i] = y
				}
				want := occ[strings.Join(fs[:7], " ")]
				if m, ok := occSaddleMirrors[strings.Join(fs[:7], " ")]; ok && !flat {
					want = occ[m]
				}
				rs := [4]float64{x[2], x[3], x[4], x[5]}
				sharp := int(x[6])
				rmax := slices.Max(rs[:])
				sh, picks, err := mixedSaddle(x[0], x[1], rs, srf.Tol(0.002, 20))
				if err != nil {
					t.Fatal(err)
				}
				picks = append(append([]topo.FilletPick{}, picks[:sharp]...), picks[sharp+1:]...)
				if err := topo.FilletPicks(sh, picks, rmax, 1<<30, flat); err != nil {
					t.Errorf("refused: %v", err)
					return
				}
				got, err := soundVolume(sh)
				if err != nil {
					t.Fatal(err)
				}
				if d := math.Abs(got - want); d > allow(rmax)+saddleChords {
					t.Errorf("volume %.6f, OpenCASCADE's %.6f: %.4f of the allowance off", got, want, d/allow(rmax))
				}
				made.Add(1)
			})
		}
	})
	t.Logf("%d made", made.Load())
}
