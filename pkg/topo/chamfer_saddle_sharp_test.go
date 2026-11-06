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

// TestChamferSaddleSharp chamfers three of a saddle's four edges at its
// corner, the fourth left sharp, over a grid of slopes and distances and
// each edge left sharp in turn: made and sound, or refused at the
// corner, as the fillets of TestFilletSaddleSharp are (saddleSharpAt).
// Before 2026-11-04 every one was refused; since, all 108 are made:
// the steep ones whose ladder folds for chamfers in two stages, by
// kind, each stage's one edge or two opposite ones ending on the faces
// across the middle (fillet_crease_end.go).
func TestChamferSaddleSharp(t *testing.T) {
	t.Parallel()
	grid := sweep.Grid{
		Parallel: true,
		Axes: []sweep.Axis{
			sweep.Floats("sx", 0.2, 0.5, 0.8),
			sweep.Floats("sy", 0.2, 0.5, 0.8),
			sweep.Floats("d", 0.5, 1, 2),
			sweep.Floats("sharp", 0, 1, 2, 3),
		},
		MaxFailRate: 0,
		Sample:      12,
	}
	var made, bad atomic.Int32
	grid.Run(func(p sweep.Point) error {
		sx, sy, d, sharp := p.Float("sx"), p.Float("sy"), p.Float("d"), int(p.Float("sharp"))
		sh, picks, err := saddleWithout(sx, sy, d, sharp, srf.Tol(0.002, 20))
		if err != nil {
			return err
		}
		if err := topo.FilletPicks(sh, picks, d, 1<<30, true); err != nil {
			if _, ok := errors.AsType[*topo.FilletError](err); !ok {
				bad.Add(1)
				t.Errorf("%v: refused without a place: %v", p, err)
			}
			return nil
		}
		if _, err := soundVolume(sh); err != nil {
			bad.Add(1)
			t.Errorf("%v: made unsound: %v", p, err)
			return fmt.Errorf("unsound: %w", err)
		}
		made.Add(1)
		return nil
	}).Require(t)
	t.Logf("%d made", made.Load())
	if bad.Load() > 0 {
		t.Errorf("%d saddles made wrong", bad.Load())
	}
}

// TestChamferSaddleSharpOracle: the saddles OpenCASCADE's makeChamfer
// cuts with an edge left sharp (testdata/topo/chamfer-saddle-sharp.txt,
// scripts/chamfer-saddle-sharp-fixtures.py), each made or refused here;
// where made, its volume within chamferSaddleAllow of OpenCASCADE's.
// Both fill the corner with a curved patch, ours set back further, so
// the volumes differ by up to 2.3 d², ours the smaller where a convex
// edge is left sharp and the larger where a concave one is; the
// steepest saddles (slopes 0.8 and 0.8), made in stages, by up to 3.5
// d². Measured 2026-11-04 over all nine slope pairs: 108 of 108 made.
func TestChamferSaddleSharpOracle(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "topo", "chamfer-saddle-sharp.txt"))
	if err != nil {
		t.Fatal(err)
	}
	var made atomic.Int32
	t.Run("lines", func(t *testing.T) {
		// One line per mirror image, and under -short and the race
		// detector three (saddleLines).
		for _, line := range saddleLines(strings.Split(strings.TrimSpace(string(data)), "\n"), saddleSharpRadii) {
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
				if err := topo.FilletPicks(sh, picks, x[2], 1<<30, true); err != nil {
					if !errors.As(err, new(*topo.FilletError)) {
						t.Errorf("refused without a place: %v", err)
					}
					return
				}
				got, err := soundVolume(sh)
				if err != nil {
					t.Fatal(err)
				}
				allow := chamferSaddleAllow
				if x[0] >= 0.8 && x[1] >= 0.8 {
					allow = chamferSaddleSteepAllow
				}
				if d := math.Abs(got - x[4]); d > allow*x[2]*x[2] {
					t.Errorf("volume %.6f, OpenCASCADE's %.6f: %.3f d² off", got, x[4], d/(x[2]*x[2]))
				}
				made.Add(1)
			})
		}
	})
	t.Logf("%d made", made.Load())
}

// chamferSaddleAllow is how far, in d², a chamfered saddle's volume may
// lie from OpenCASCADE's: the two corners' patches differ (measured up
// to 2.29 d², 2026-11-04). It is not the chord tolerance's: the
// volumes are the same at a quarter of it.
const chamferSaddleAllow = 2.5

// chamferSaddleSteepAllow is chamferSaddleAllow for the steepest
// saddles (both slopes 0.8), made in two stages, each edge ending on
// the faces across, not by one corner's patches: up to 3.47 d² from
// OpenCASCADE's, where its convex edge is left sharp at d 2
// (2026-11-04).
const chamferSaddleSteepAllow = 3.6
