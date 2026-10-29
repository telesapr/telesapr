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

	"github.com/telesapr/telesapr/pkg/check"
	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/internal/sweep"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/topo"
)

// blendLedge is scripts/fillet-blend-fixtures.py's ledge: the prism 10
// high on the outline (x0, 0), (20, 0), (20, 10), (10, 10), b, (x0, b.y),
// b 10 from (10, 10) along phi degrees, with the picks of its top edge
// along x at c, convex, and the upright at (10, 10) at k, concave; the
// top edge towards b stays sharp.
func blendLedge(phi, c, k float64, tol srf.Tolerance) (*srf.SShell, []topo.FilletPick) {
	b := v(10+10*math.Cos(phi*math.Pi/180), 10+10*math.Sin(phi*math.Pi/180), 0)
	x0 := math.Min(0, b.X) - 5
	sh := srf.SShell{Tol: tol}
	sh.MakePrism([]geom.Vector{v(x0, 0, 0), v(20, 0, 0), v(20, 10, 0), v(10, 10, 0), b, v(x0, b.Y, 0)},
		v(0, 0, 0), v(0, 0, 10), core.NewRGBA(100, 100, 100, 255))
	return &sh, []topo.FilletPick{{A: v(20, 10, 10), B: v(10, 10, 10), R: c}, {A: v(10, 10, 0), B: v(10, 10, 10), R: k}}
}

// blendRib is the fixtures' rib: the plate (-5, -5, 0) to (20, 20, 5)
// with the block (0, 0, 5) to (10, 10, 15) on it, with the picks of the
// block's upright edge at (10, 10) at c, convex, and its foot edge along
// -x at k, concave; the foot edge along -y stays sharp.
func blendRib(c, k float64, tol srf.Tolerance) (*srf.SShell, []topo.FilletPick) {
	plate, block, sh := srf.SShell{Tol: tol}, srf.SShell{Tol: tol}, srf.SShell{Tol: tol}
	plate.MakePrism([]geom.Vector{v(-5, -5, 0), v(20, -5, 0), v(20, 20, 0), v(-5, 20, 0)}, v(0, 0, 0), v(0, 0, 5),
		core.NewRGBA(100, 100, 100, 255))
	block.MakePrism([]geom.Vector{v(0, 0, 4), v(10, 0, 4), v(10, 10, 4), v(0, 10, 4)}, v(0, 0, 0), v(0, 0, 11),
		core.NewRGBA(100, 100, 100, 255))
	sh.MakeFromUnionOf(&plate, &block)
	return &sh, []topo.FilletPick{{A: v(10, 10, 5), B: v(10, 10, 15), R: c}, {A: v(10, 10, 5), B: v(0, 10, 5), R: k}}
}

// blendAllow is how far a blend's volume may lie from OpenCASCADE's:
// the two fill the same hole with different patches, OpenCASCADE's of
// degree 7 with its fillet's end cut where the angle round the concave
// fillet runs evenly, and the hole is a few square millimetres at these
// radii. Measured 2026-10-29: within 0.15 mm³ on every line.
const blendAllow = 0.2

// Convex and concave fillets meeting above an edge left sharp, against
// OpenCASCADE's makeFillet (testdata/topo/fillet-blend-corners.txt,
// scripts/fillet-blend-fixtures.py): a ledge's top edge with the
// concave upright below it, at corners of 60 to 105 degrees, and a
// rib's upright with one of its foot's edges. Each is sound, has
// OpenCASCADE's count of faces, one patch filling the corner, and its
// volume within blendAllow. Where the radii are far apart one way the
// patch folds and the corner is refused (TestFilletBlendSweep), which
// OpenCASCADE makes: the rib's 0.5 and 3 is left out.
func TestFilletBlendCornerOracle(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "topo", "fillet-blend-corners.txt"))
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
			var sh *srf.SShell
			var picks []topo.FilletPick
			if f[0] == "ledge" {
				sh, picks = blendLedge(x[0], x[1], x[2], srf.Tol(0.002, 20))
			} else {
				sh, picks = blendRib(x[0], x[1], srf.Tol(0.002, 20))
			}
			if err := topo.FilletPicks(sh, picks, 1, 1<<30, false); err != nil {
				t.Fatal(err)
			}
			got := sound(t, sh)
			if n := len(build(t, sh).Faces); n != faces {
				t.Errorf("%d faces, OpenCASCADE's %d", n, faces)
			}
			if math.Abs(got-want) > blendAllow {
				t.Errorf("volume %.4f, OpenCASCADE's %.4f", got, want)
			}
		})
	}
}

// TestFilletBlendSweep rounds a ledge's top edge, convex, and the
// concave upright below it over the corner's angle and the two radii:
// each is sound, or refused by name. The rate is what the blend left
// refused when it was built, 2026-10-29: 16 of 80, where the convex
// fillet's radius is several times how far the concave one reaches
// along the top (at 120 degrees that is from about 1.7 times), its
// patch a long sliver along the convex fillet's end that folds.
func TestFilletBlendSweep(t *testing.T) {
	t.Parallel()
	grid := sweep.Grid{
		Parallel: true,
		Axes: []sweep.Axis{
			sweep.Floats("phi", 60, 75, 90, 105, 120),
			sweep.Floats("c", 0.5, 1, 2, 3),
			sweep.Floats("k", 0.5, 1, 2, 3),
		},
		MaxFailRate: 16.0 / 80,
		Sample:      16,
	}
	var bad atomic.Int32
	grid.Run(func(p sweep.Point) error {
		sh, picks := blendLedge(p.Float("phi"), p.Float("c"), p.Float("k"), srf.Tol(0.002, 20))
		if err := topo.FilletPicks(sh, picks, 1, 1<<30, false); err != nil {
			if !errors.As(err, new(*topo.FilletError)) {
				bad.Add(1)
				t.Errorf("%v: refused without a place: %v", p, err)
			}
			return err
		}
		var m srf.SMesh
		sh.TriangulateInto(&m)
		if rep := check.Solid(sh, &m); !rep.Sound() {
			bad.Add(1)
			t.Errorf("%v: unsound: %s", p, rep.String())
			return fmt.Errorf("unsound")
		}
		return nil
	}).Require(t)
	if n := bad.Load(); n > 0 {
		t.Errorf("%d corners made unsound", n)
	}
}
