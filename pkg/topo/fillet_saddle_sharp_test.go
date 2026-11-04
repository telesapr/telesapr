package topo_test

import (
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
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
// 2026-11-04 84 of the full grid's 108 are made (saddleSharpAt); before,
// every one was refused.
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
// where made, its volume within saddleAllow of OpenCASCADE's, whose
// corner is one vertex blend against our three patches, and the
// meshes' chords besides (saddleChords). Measured 2026-11-04: all 58
// made, within 0.056 r³ at radius 1 and over, and at 0.5 within
// 0.016 mm³, which at chord 0.0001% shrinks to 0.002.
func TestFilletSaddleSharpOracle(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "topo", "fillet-saddle-sharp.txt"))
	if err != nil {
		t.Fatal(err)
	}
	var made atomic.Int32
	t.Run("lines", func(t *testing.T) {
		for line := range strings.SplitSeq(strings.TrimSpace(string(data)), "\n") {
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
				if d := math.Abs(got - x[4]); d > saddleAllow(x[2])+saddleChords {
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
