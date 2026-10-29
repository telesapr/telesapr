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

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/internal/sweep"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/topo"
)

// mixedLedge is scripts/fillet-mixed-fixtures.py's ledge: blendLedge's
// prism with its top edge along x at a and the top edge towards b at b,
// convex, and the upright below them at k, concave.
func mixedLedge(phi, a, b, k float64, tol srf.Tolerance) (*srf.SShell, []topo.FilletPick) {
	sh, picks := blendLedge(phi, a, k, tol)
	end := v(10+10*math.Cos(phi*math.Pi/180), 10+10*math.Sin(phi*math.Pi/180), 10)
	return sh, append(picks, topo.FilletPick{A: v(10, 10, 10), B: end, R: b})
}

// mixedRib is the fixtures' rib: blendRib's with its upright at c,
// convex, and its foot edges along -x at k1 and along -y at k2, concave.
func mixedRib(c, k1, k2 float64, tol srf.Tolerance) (*srf.SShell, []topo.FilletPick) {
	sh, picks := blendRib(c, k1, tol)
	return sh, append(picks, topo.FilletPick{A: v(10, 10, 5), B: v(10, 0, 5), R: k2})
}

// mixedSaddle is the fixtures' saddle: the block on the square (0, 0)
// to (20, 20) from z 0 up to z = 10 + sx |x - 10| - sy |y - 10|, with
// the picks of the edges from (10, 10, 10) towards +x and -x, convex, at
// rs[0] and rs[1], and towards +y and -y, concave, at rs[2] and rs[3].
func mixedSaddle(sx, sy float64, rs [4]float64, tol srf.Tolerance) (*srf.SShell, []topo.FilletPick, error) {
	at := func(x, y float64) geom.Vector { return v(x, y, 10+sx*math.Abs(x-10)-sy*math.Abs(y-10)) }
	corners := func(xy ...[2]float64) []geom.Vector {
		var out []geom.Vector
		for _, p := range xy {
			out = append(out, at(p[0], p[1]))
		}
		return out
	}
	faces := [][]geom.Vector{
		corners([2]float64{10, 10}, [2]float64{20, 10}, [2]float64{20, 20}, [2]float64{10, 20}),
		corners([2]float64{10, 10}, [2]float64{10, 20}, [2]float64{0, 20}, [2]float64{0, 10}),
		corners([2]float64{10, 10}, [2]float64{0, 10}, [2]float64{0, 0}, [2]float64{10, 0}),
		corners([2]float64{10, 10}, [2]float64{10, 0}, [2]float64{20, 0}, [2]float64{20, 10}),
	}
	for _, side := range [][3][2]float64{{{0, 0}, {10, 0}, {20, 0}}, {{20, 0}, {20, 10}, {20, 20}},
		{{20, 20}, {10, 20}, {0, 20}}, {{0, 20}, {0, 10}, {0, 0}}} {
		f := corners(side[0], side[1], side[2])
		faces = append(faces, append(f, v(side[2][0], side[2][1], 0), v(side[0][0], side[0][1], 0)))
	}
	faces = append(faces, []geom.Vector{v(0, 0, 0), v(20, 0, 0), v(20, 20, 0), v(0, 20, 0)})
	sh := &srf.SShell{Tol: tol}
	for _, f := range faces {
		var l []srf.SBezier
		for i := range f {
			l = append(l, srf.SBezierFrom2(f[i], f[(i+1)%len(f)]))
		}
		s := srf.FromPlane(f[0].Minus(f[1].Minus(f[0])).Minus(f[len(f)-1].Minus(f[0])),
			f[len(f)-1].Minus(f[0]).ScaledBy(3), f[1].Minus(f[0]).ScaledBy(3))
		if _, err := topo.AddFace(sh, s, [][]srf.SBezier{l}); err != nil {
			return nil, nil, err
		}
	}
	for i := range sh.Surface {
		sh.Surface[i].Reverse()
	}
	var picks []topo.FilletPick
	for i, end := range corners([2]float64{20, 10}, [2]float64{0, 10}, [2]float64{10, 20}, [2]float64{10, 0}) {
		picks = append(picks, topo.FilletPick{A: at(10, 10), B: end, R: rs[i]})
	}
	return sh, picks, nil
}

// varyingFaces are the faces whose surface is of degree 5 along the
// arc and at most 2 across: fillets whose radius varies.
func varyingFaces(tp *topo.Topology) []int {
	var out []int
	for f := range tp.Faces {
		if s := tp.Surface(f); s != nil && s.DegN == 5 && s.DegM <= 2 {
			out = append(out, f)
		}
	}
	return out
}

// tangentAlong reports whether edge e's two faces have one normal at
// each of its points, and where not. Topology.Smooth samples the
// edge's middle by length, on a chord, which on a fillet of radius 0.5
// is off its surface's normal by more than Smooth allows.
func tangentAlong(tp *topo.Topology, e int) (geom.Vector, bool) {
	fs := tp.EdgeFaces(e)
	a, b := tp.Surface(fs[0]), tp.Surface(fs[1])
	for _, p := range tp.Edges[e].Points {
		var ua, va, ub, vb float64
		a.ClosestPointToUV(p, &ua, &va, true)
		b.ClosestPointToUV(p, &ub, &vb, true)
		if a.NormalAt(ua, va).WithMagnitude(1).Dot(b.NormalAt(ub, vb).WithMagnitude(1)) < 1-1e-6 {
			return p, false
		}
	}
	return geom.Vector{}, true
}

// mixedAllow is how far a corner's volume may lie from OpenCASCADE's
// where the radius varies along an arc: the two vary it by different
// laws. Measured 2026-10-29: within 0.035 mm³ on every line, about
// as far as where the radii are one (0.013 on the ledge at 90°).
const mixedAllow = 0.05

// saddleAllow is how far a saddle's volume may lie from OpenCASCADE's,
// for its largest radius r: the corner's hole is filled by different
// patches, ours a setback's, OpenCASCADE's one of degree 7, and the
// hole grows as r² and the difference as r³. Measured 2026-10-29: up to
// 0.046 r³, where the two convex edges' radii differ.
func saddleAllow(r float64) float64 { return 0.05 * r * r * r }

// Corners of rounded edges of both kinds, against OpenCASCADE's
// makeFillet (testdata/topo/fillet-mixed-corners.txt,
// scripts/fillet-mixed-fixtures.py). Of three edges whose two of one
// kind differ in radius: a ledge's two top edges, convex, over the
// concave upright, at corners of 60 to 120 degrees, and a rib's two
// foot edges, concave, under its upright. Each is sound, has
// OpenCASCADE's count of faces, one face whose radius goes from one's
// to the other's tangent to every face round it, and its volume within
// mixedAllow. Of four, a saddle's two convex and two concave edges: each
// is sound, its volume within saddleAllow; its hole is filled by four
// patches where OpenCASCADE's is one.
func TestFilletMixedCornerOracle(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "topo", "fillet-mixed-corners.txt"))
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
			tol := srf.Tol(0.002, 20)
			var sh *srf.SShell
			var picks []topo.FilletPick
			switch f[0] {
			case "ledge":
				sh, picks = mixedLedge(x[0], x[1], x[2], x[3], tol)
			case "rib":
				sh, picks = mixedRib(x[0], x[1], x[2], tol)
			default:
				var err error
				sh, picks, err = mixedSaddle(x[0], x[1], [4]float64{x[2], x[3], x[4], x[5]}, tol)
				if err != nil {
					t.Fatal(err)
				}
			}
			if err := topo.FilletPicks(sh, picks, 1, 1<<30, false); err != nil {
				t.Fatal(err)
			}
			got := sound(t, sh)
			if f[0] == "saddle" {
				if allow := saddleAllow(slices.Max(x[2:6])); math.Abs(got-want) > allow {
					t.Errorf("volume %.4f, OpenCASCADE's %.4f, allowed %.3g", got, want, allow)
				}
				return
			}
			tp := build(t, sh)
			if n := len(tp.Faces); n != faces {
				t.Errorf("%d faces, OpenCASCADE's %d", n, faces)
			}
			vf := varyingFaces(tp)
			if len(vf) != 1 {
				t.Fatalf("%d faces of a varying radius, want 1", len(vf))
			}
			for _, e := range tp.FaceEdges(vf[0]) {
				if p, ok := tangentAlong(tp, e); !ok {
					t.Errorf("the varying fillet's edge is not tangent at %v", p)
				}
			}
			if math.Abs(got-want) > mixedAllow {
				t.Errorf("volume %.4f, OpenCASCADE's %.4f", got, want)
			}
		})
	}
}

// TestFilletMixedCornerSweep rounds a ledge's two top edges and the
// upright below them over the corner's angle and the three radii, and a
// rib's upright and its two foot edges over the three radii (phi 0), at
// a fine and a coarse chord tolerance: each is sound, or refused by
// name. None is refused: the ledge at 150 degrees whose upright is
// rounded at 0.5 at the coarse tolerance, which the stages refuse with
// the radii all one too (6 of 180), is set back.
func TestFilletMixedCornerSweep(t *testing.T) {
	t.Parallel()
	grid := sweep.Grid{
		Parallel: true,
		Axes: []sweep.Axis{
			sweep.Floats("phi", 0, 60, 90, 120, 150),
			sweep.Floats("a", 0.5, 1, 3),
			sweep.Floats("b", 0.5, 2),
			sweep.Floats("k", 0.5, 1, 3),
			sweep.Floats("chord", 0.002, 0.02),
		},
		MaxFailRate: 0,
		Sample:      16,
	}
	var bad atomic.Int32
	grid.Run(func(p sweep.Point) error {
		tol := srf.Tol(p.Float("chord"), 20)
		sh, picks := mixedLedge(p.Float("phi"), p.Float("a"), p.Float("b"), p.Float("k"), tol)
		if p.Float("phi") == 0 {
			sh, picks = mixedRib(p.Float("a"), p.Float("b"), p.Float("k"), tol)
		}
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

// TestFilletSaddleSweep rounds the four edges of a saddle's corner over
// its slopes and the radii of one convex and one concave edge, the other
// two at 1: each is sound, or refused by name.
func TestFilletSaddleSweep(t *testing.T) {
	t.Parallel()
	grid := sweep.Grid{
		Parallel: true,
		Axes: []sweep.Axis{
			sweep.Floats("sx", 0.2, 0.5, 0.8),
			sweep.Floats("sy", 0.2, 0.5, 0.8),
			sweep.Floats("a", 0.5, 2),
			sweep.Floats("c", 0.5, 2),
		},
		MaxFailRate: 0,
		Sample:      6,
	}
	var bad atomic.Int32
	grid.Run(func(p sweep.Point) error {
		sh, picks, err := mixedSaddle(p.Float("sx"), p.Float("sy"), [4]float64{p.Float("a"), 1, p.Float("c"), 1}, srf.Tol(0.002, 20))
		if err != nil {
			bad.Add(1)
			t.Errorf("%v: the saddle can't be built: %v", p, err)
			return err
		}
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
