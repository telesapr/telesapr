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

// radiiPairShape is scripts/fillet-radii-pair-fixtures.py's shape of
// kind with its two picks at a and b, the third edge at their corner
// left sharp: the box's two top edges, the ledge's at phi, or the rib's
// two foot edges.
func radiiPairShape(kind string, phi, a, b float64, tol srf.Tolerance) (*srf.SShell, []topo.FilletPick) {
	switch kind {
	case "box":
		return box(tol), []topo.FilletPick{{A: v(0, 0, 30), B: v(10, 0, 30), R: a}, {A: v(10, 0, 30), B: v(10, 20, 30), R: b}}
	case "ledge":
		sh, picks := mixedLedge(phi, a, b, 1, tol)
		return sh, []topo.FilletPick{picks[0], picks[2]}
	}
	sh, picks := mixedRib(1, a, b, tol)
	return sh, picks[1:]
}

// radiiPairAllow is how far a corner's volume may lie from
// OpenCASCADE's: the corners are alike, two cylinders meeting along
// their quartic, so what is left is the meshes' chords, which at the
// chord of 0.002% and 20 segments miss up to 0.054 mm³ where a radius is
// 3. Measured 2026-10-29 at 0.0005% and 200 segments: within 0.014 mm³;
// at 0.0001% and 1000, within 0.004 on the five lines furthest off.
const radiiPairAllow = 0.02

// Two fillets of different radii above an edge left sharp, against
// OpenCASCADE's makeFillet (testdata/topo/fillet-radii-pairs.txt,
// scripts/fillet-radii-pair-fixtures.py): a box's two top edges, a
// ledge's two top edges over its concave upright at corners of 60 to
// 120 degrees, and a rib's two foot edges under its convex upright.
// Each is sound, has OpenCASCADE's count of faces and its volume
// within radiiPairAllow.
func TestFilletRadiiPairOracle(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "topo", "fillet-radii-pairs.txt"))
	if err != nil {
		t.Fatal(err)
	}
	for line := range strings.SplitSeq(strings.TrimSpace(string(data)), "\n") {
		t.Run(line, func(t *testing.T) {
			t.Parallel()
			f := strings.Fields(line)
			var x []float64
			for _, s := range f[1 : len(f)-1] {
				y, err := strconv.ParseFloat(s, 64)
				if err != nil {
					t.Fatalf("%q: %v", line, err)
				}
				x = append(x, y)
			}
			want := x[len(x)-1]
			faces, _ := strconv.Atoi(f[len(f)-1])
			phi, a, b := 0.0, x[0], x[1]
			if f[0] == "ledge" {
				phi, a, b = x[0], x[1], x[2]
			}
			sh, picks := radiiPairShape(f[0], phi, a, b, srf.Tol(0.0005, 200))
			if err := topo.FilletPicks(sh, picks, 1, 1<<30, false); err != nil {
				t.Fatal(err)
			}
			got := sound(t, sh)
			if n := len(build(t, sh).Faces); n != faces {
				t.Errorf("%d faces, OpenCASCADE's %d", n, faces)
			}
			if math.Abs(got-want) > radiiPairAllow {
				t.Errorf("volume %.4f, OpenCASCADE's %.4f", got, want)
			}
		})
	}
}

// TestFilletRadiiPairSweep rounds the two edges above a sharp one over
// the shape, the corner's angle and both radii at a fine and a coarse
// chord tolerance: each is sound, or refused by name.
func TestFilletRadiiPairSweep(t *testing.T) {
	t.Parallel()
	grid := sweep.Grid{
		Parallel: true,
		Axes: []sweep.Axis{
			sweep.Strings("shape", "box", "ledge", "rib"),
			sweep.Floats("phi", 45, 60, 90, 120, 150),
			sweep.Floats("a", 0.25, 0.5, 1, 3),
			sweep.Floats("b", 0.5, 2, 3),
			sweep.Floats("chord", 0.002, 0.02),
		},
		MaxFailRate: 0,
		Sample:      16,
	}
	var bad atomic.Int32
	grid.Run(func(p sweep.Point) error {
		if p.String("shape") != "ledge" && p.Float("phi") != 90 {
			return nil
		}
		sh, picks := radiiPairShape(p.String("shape"), p.Float("phi"), p.Float("a"), p.Float("b"), srf.Tol(p.Float("chord"), 20))
		if err := topo.FilletPicks(sh, picks, 1, 1<<30, false); err != nil {
			if !errors.As(err, new(*topo.FilletError)) {
				bad.Add(1)
				t.Errorf("%v: refused without a place: %v", p, err)
			}
			return err
		}
		if _, err := soundVolume(sh); err != nil {
			bad.Add(1)
			t.Errorf("%v: %v", p, err)
			return fmt.Errorf("unsound")
		}
		return nil
	}).Require(t)
	if n := bad.Load(); n > 0 {
		t.Errorf("%d corners made unsound", n)
	}
}
