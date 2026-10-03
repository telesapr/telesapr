package topo_test

import (
	"fmt"
	"math"
	"math/rand"
	"testing"

	"github.com/telesapr/telesapr/pkg/check"
	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/topo"
)

const lidW, lidD = 40.0, 30.0

// clampedKnots is a clamped knot vector over [0, 1] for n control
// points of degree deg, its inner knots spaced unevenly, each mult
// times.
func clampedKnots(r *rand.Rand, deg, n, mult int) []float64 {
	inner := (n - deg - 1) / mult
	if inner*mult != n-deg-1 {
		panic(fmt.Sprintf("%d control points of degree %d take no whole number of knots %d times", n, deg, mult))
	}
	ts := make([]float64, inner)
	t := 0.0
	for i := range ts {
		t += 0.5 + r.Float64()
		ts[i] = t
	}
	total := t + 0.5 + r.Float64()
	k := make([]float64, 0, n+deg+1)
	for i := 0; i <= deg; i++ {
		k = append(k, 0)
	}
	for _, x := range ts {
		for m := 0; m < mult; m++ {
			k = append(k, x/total)
		}
	}
	for i := 0; i <= deg; i++ {
		k = append(k, 1)
	}
	return k
}

// greville is the average of control point i's deg knots: points
// there make a B-spline linear in its parameter.
func greville(k []float64, deg, i int) float64 {
	s := 0.0
	for j := 1; j <= deg; j++ {
		s += k[i+j]
	}
	return s / float64(deg)
}

// lidSurface is a B-spline over the 40 x 30 rectangle, x = 40 u and
// y = 30 v exactly, its height 10 with bumps of up to 2 either way.
func lidSurface(r *rand.Rand, du, dv, nu, nv, mult int) *srf.BSplineSurface {
	s := &srf.BSplineSurface{DegU: du, DegV: dv,
		KnotsU: clampedKnots(r, du, nu, mult), KnotsV: clampedKnots(r, dv, nv, mult)}
	for i := 0; i < nu; i++ {
		var row []geom.Vector
		var w []float64
		for j := 0; j < nv; j++ {
			row = append(row, geom.Vector{X: lidW * greville(s.KnotsU, du, i), Y: lidD * greville(s.KnotsV, dv, j),
				Z: 10 + 4*(r.Float64()-0.5)})
			w = append(w, 1)
		}
		s.Ctrl = append(s.Ctrl, row)
		s.Weight = append(s.Weight, w)
	}
	return s
}

// lidVolume is the volume under a non-rational lid surface: each
// control point's height times the integrals of its basis functions,
// (k[i+deg+1] - k[i]) / (deg + 1) each way, over the rectangle.
func lidVolume(s *srf.BSplineSurface) float64 {
	v := 0.0
	for i := range s.Ctrl {
		iu := (s.KnotsU[i+s.DegU+1] - s.KnotsU[i]) / float64(s.DegU+1)
		for j := range s.Ctrl[i] {
			iv := (s.KnotsV[j+s.DegV+1] - s.KnotsV[j]) / float64(s.DegV+1)
			v += s.Ctrl[i][j].Z * iu * iv
		}
	}
	return v * lidW * lidD
}

// lid is the solid under the surface: the surface's patches on top,
// flat sides whose top edges are its boundary curves, and a bottom.
func lid(t testing.TB, s *srf.BSplineSurface) *srf.SShell {
	t.Helper()
	sh := &srf.SShell{}
	col := core.NewRgbaColor(100, 100, 100, 255)
	if _, err := topo.AddBSplineFace(sh, s, srf.OriginFor(9, 100), 0x777, col); err != nil {
		t.Fatal(err)
	}
	ps, err := s.Patches()
	if err != nil {
		t.Fatal(err)
	}
	// The boundary curves, in pieces, from corner (0, 0) round by +x.
	var front, right, back, left []srf.SBezier
	for a := range ps {
		front = append(front, ps[a][0].EdgeCurve(0))
		back = append(back, ps[a][len(ps[a])-1].EdgeCurve(2))
	}
	for b := range ps[0] {
		left = append(left, ps[0][b].EdgeCurve(3))
		right = append(right, ps[len(ps)-1][b].EdgeCurve(1))
	}
	rev := func(cs []srf.SBezier) []srf.SBezier {
		out := make([]srf.SBezier, len(cs))
		for i, c := range cs {
			c.Reverse()
			out[len(cs)-1-i] = c
		}
		return out
	}
	line := srf.SBezierFrom2
	z0 := func(p geom.Vector) geom.Vector { return geom.Vector{X: p.X, Y: p.Y} }
	side := func(top []srf.SBezier, id int) {
		// Along the top from one corner to the other, down, back along
		// the bottom and up.
		a, b := top[0].Start(), top[len(top)-1].Finish()
		loop := append(append([]srf.SBezier(nil), top...), line(b, z0(b)), line(z0(b), z0(a)), line(z0(a), a))
		if _, err := topo.AddPlanarFace(sh, [][]srf.SBezier{loop}, srf.OriginFor(9, id), col); err != nil {
			t.Fatal(err)
		}
	}
	side(front, 1)
	side(right, 2)
	side(rev(back), 3)
	side(rev(left), 4)
	c := []geom.Vector{{}, {X: lidW}, {X: lidW, Y: lidD}, {Y: lidD}}
	bottom := []srf.SBezier{line(c[0], c[1]), line(c[1], c[2]), line(c[2], c[3]), line(c[3], c[0])}
	if _, err := topo.AddPlanarFace(sh, [][]srf.SBezier{bottom}, srf.OriginFor(9, 5), col); err != nil {
		t.Fatal(err)
	}
	return sh
}

// checkSolid meshes a shell, checks it sound with the volume wanted,
// and gives its report.
func checkSolid(t *testing.T, name string, sh *srf.SShell, want, rel float64) check.Report {
	t.Helper()
	var m srf.SMesh
	sh.TriangulateInto(&m)
	r := check.Solid(sh, &m)
	if !r.Sound() {
		t.Errorf("%s: %s", name, r.String())
		for _, p := range r.Topology {
			t.Logf("  %s at %v", p.Msg, p.At)
		}
	}
	if math.Abs(r.Volume-want) > rel*math.Abs(want) {
		t.Errorf("%s: volume %.6f, want %.6f (%.2g)", name, r.Volume, want, (r.Volume-want)/want)
	}
	return r
}

// gauss are the nodes and weights of 16-point Gauss-Legendre
// quadrature on [0, 1].
var gaussX, gaussW = func() (x, w [16]float64) {
	// Newton on the Legendre polynomial of degree 16.
	const n = 16
	for i := 0; i < n; i++ {
		z := math.Cos(math.Pi * (float64(i) + 0.75) / (n + 0.5))
		var pp float64
		for it := 0; it < 100; it++ {
			p1, p2 := 1.0, 0.0
			for j := 1; j <= n; j++ {
				p1, p2 = ((2*float64(j)-1)*z*p1-(float64(j)-1)*p2)/float64(j), p1
			}
			pp = n * (z*p1 - p2) / (z*z - 1)
			z1 := z
			z = z1 - p1/pp
			if math.Abs(z-z1) < 1e-15 {
				break
			}
		}
		x[i] = (1 - z) / 2
		w[i] = 1 / ((1 - z*z) * pp * pp)
	}
	return x, w
}()

// integrate is the integral of f over [a, b], by Gauss-Legendre on n
// equal pieces, which a piecewise polynomial between knots needs.
func integrate(a, b float64, n int, f func(float64) float64) float64 {
	gx, gw := gaussX, gaussW
	sum := 0.0
	h := (b - a) / float64(n)
	for k := 0; k < n; k++ {
		for i := range gx {
			sum += gw[i] * f(a+h*(float64(k)+gx[i]))
		}
	}
	return sum * h
}

// heightAt is the lid's height over (x, y).
func heightAt(s *srf.BSplineSurface, x, y float64) float64 {
	return s.PointAt(x/lidW, y/lidD).Z
}

func TestBSplineLid(t *testing.T) {
	old, oldSegs := srf.ChordTolMm(), srf.GetMaxSegments()
	srf.SetChordTolMm(0.002)
	srf.SetMaxSegments(100)
	defer func() { srf.SetChordTolMm(old); srf.SetMaxSegments(oldSegs) }()
	r := rand.New(rand.NewSource(11))
	for _, c := range []struct {
		du, dv, nu, nv, mult int
	}{
		{3, 3, 6, 5, 1},
		{2, 3, 5, 6, 1},
		{srf.MaxDegree, 4, 8, 6, 1},
		{3, 3, 8, 8, 2},
		// A crease: knots three times in a cubic, so the patches meet at
		// an angle and are faces of their own.
		{3, 3, 7, 4, 3},
	} {
		s := lidSurface(r, c.du, c.dv, c.nu, c.nv, c.mult)
		name := fmt.Sprintf("degree %dx%d, %dx%d points, knots %d times", c.du, c.dv, c.nu, c.nv, c.mult)
		sh := lid(t, s)
		want := lidVolume(s)
		checkSolid(t, name, sh, want, 1e-4)
		tp := build(t, sh)
		us, vs := s.Breaks()
		patches := (len(us) - 1) * (len(vs) - 1)
		lidFaces := 0
		for _, sht := range tp.Sheets {
			if tp.Surface(sht.Faces[0]).Sheet == 0x777 {
				lidFaces++
			}
		}
		wantFaces := 1
		if c.mult == c.du {
			wantFaces = len(us) - 1
		}
		if len(tp.Faces) != patches+5 || lidFaces != wantFaces {
			t.Errorf("%s: %d patches, the lid %d faces; want %d patches, the lid %d faces",
				name, len(tp.Faces), lidFaces, patches+5, wantFaces)
		}
		// Every edge round the lid's patches, but the seams, has a name
		// that finds it again.
		for _, e := range tp.SolidEdges() {
			n, err := tp.NameOf(e)
			if err != nil {
				t.Errorf("%s: edge %d: %v", name, e, err)
				continue
			}
			if x, err := tp.Resolve(n); err != nil || x != e {
				t.Errorf("%s: %v resolves to %d, %v", name, n, x, err)
			}
		}
	}
}

// TestBSplineLidBooleans cuts a hole through a bicubic lid, puts a
// block through its top, and unites a cylinder that crosses its knot
// lines, against the volumes under the surface by quadrature.
func TestBSplineLidBooleans(t *testing.T) {
	old, oldSegs := srf.ChordTolMm(), srf.GetMaxSegments()
	srf.SetChordTolMm(0.005)
	srf.SetMaxSegments(100)
	defer func() { srf.SetChordTolMm(old); srf.SetMaxSegments(oldSegs) }()
	r := rand.New(rand.NewSource(12))
	for _, deg := range [][2]int{{3, 3}, {srf.MaxDegree, 4}} {
		s := lidSurface(r, deg[0], deg[1], deg[0]+4, deg[1]+3, 1)
		vol := lidVolume(s)
		name := fmt.Sprintf("degree %dx%d", deg[0], deg[1])

		// A hole of radius 5 at (17, 13): less the volume under the
		// surface over the disc, in polar coordinates.
		const hr, hx, hy = 5.0, 17.0, 13.0
		under := integrate(0, 2*math.Pi, 32, func(th float64) float64 {
			return integrate(0, hr, 8, func(rho float64) float64 {
				return rho * heightAt(s, hx+rho*math.Cos(th), hy+rho*math.Sin(th))
			})
		})
		var hole srf.SShell
		hole.MakeFromTransformationOf(cylinder(hr, 30), geom.Vector{X: hx, Y: hy, Z: -5}, geom.QuaternionIdentity, 1)
		var holed srf.SShell
		holed.MakeFromDifferenceOf(lid(t, s), &hole)
		checkSolid(t, name+" less a hole", &holed, vol-under, 2e-4)

		// A block from (8, 6, 4) to (28, 22, 20) through the top: the
		// union adds what of the block stands above the surface.
		above := integrate(8, 28, 8, func(x float64) float64 {
			return integrate(6, 22, 8, func(y float64) float64 { return 20 - heightAt(s, x, y) })
		})
		var blk srf.SShell
		blk.MakePrism([]geom.Vector{{X: 8, Y: 6, Z: 4}, {X: 28, Y: 6, Z: 4}, {X: 28, Y: 22, Z: 4}, {X: 8, Y: 22, Z: 4}},
			geom.Vector{}, geom.Vector{Z: 16}, core.NewRgbaColor(100, 100, 100, 255))
		var joined srf.SShell
		joined.MakeFromUnionOf(lid(t, s), &blk)
		checkSolid(t, name+" with a block", &joined, vol+above, 2e-4)

		// And the block taken away: what of it lies under the surface.
		var cut srf.SShell
		cut.MakeFromDifferenceOf(lid(t, s), &blk)
		below := 20*16*16 - above
		checkSolid(t, name+" less a block", &cut, vol-below, 2e-4)
	}
}

// BenchmarkBSplineLid builds a bicubic lid of 12 patches and cuts a
// hole through it.
func BenchmarkBSplineLid(b *testing.B) {
	r := rand.New(rand.NewSource(13))
	s := lidSurface(r, 3, 3, 7, 6, 1)
	var hole srf.SShell
	hole.MakeFromTransformationOf(cylinder(5, 30), geom.Vector{X: 17, Y: 13, Z: -5}, geom.QuaternionIdentity, 1)
	for b.Loop() {
		var holed srf.SShell
		holed.MakeFromDifferenceOf(lid(b, s), &hole)
		var m srf.SMesh
		holed.TriangulateInto(&m)
	}
}

// TestBSplineSweep is step 11.2's sweep: lids of degrees 2 to 5, their
// knots once or twice, each with a hole, a block put through its top,
// or a block taken out of it, placed at random, against the volumes by
// quadrature; 96 points. The rate set before: 0 failures.
func TestBSplineSweep(t *testing.T) {
	if testing.Short() {
		t.Skip("a sweep")
	}
	old, oldSegs := srf.ChordTolMm(), srf.GetMaxSegments()
	srf.SetChordTolMm(0.005)
	srf.SetMaxSegments(100)
	defer func() { srf.SetChordTolMm(old); srf.SetMaxSegments(oldSegs) }()
	r := rand.New(rand.NewSource(21))
	col := core.NewRgbaColor(100, 100, 100, 255)
	points, failed := 0, 0
	for _, deg := range [][2]int{{2, 2}, {3, 3}, {4, 3}, {srf.MaxDegree, srf.MaxDegree}} {
		for _, mult := range []int{1, 2} {
			for op := 0; op < 3; op++ {
				for k := 0; k < 4; k++ {
					points++
					// Enough points for two or three inner knots, a whole
					// number of them for mult.
					nu, nv := deg[0]+1+2*mult, deg[1]+1+mult*(1+k%2)
					s := lidSurface(r, deg[0], deg[1], nu, nv, mult)
					vol := lidVolume(s)
					x, y := 8+24*r.Float64(), 6+18*r.Float64()
					name := fmt.Sprintf("degree %v knots %d times, op %d at (%.2f, %.2f)", deg, mult, op, x, y)
					var tool, out srf.SShell
					want := 0.0
					switch op {
					case 0:
						const hr = 4.0
						tool.MakeFromTransformationOf(cylinder(hr, 30), geom.Vector{X: x, Y: y, Z: -5}, geom.QuaternionIdentity, 1)
						want = vol - integrate(0, 2*math.Pi, 32, func(th float64) float64 {
							return integrate(0, hr, 8, func(rho float64) float64 {
								return rho * heightAt(s, x+rho*math.Cos(th), y+rho*math.Sin(th))
							})
						})
						out.MakeFromDifferenceOf(lid(t, s), &tool)
					default:
						const w, d = 10.0, 8.0
						x0, y0 := x-w/2, y-d/2
						tool.MakePrism([]geom.Vector{{X: x0, Y: y0, Z: 4}, {X: x0 + w, Y: y0, Z: 4}, {X: x0 + w, Y: y0 + d, Z: 4}, {X: x0, Y: y0 + d, Z: 4}},
							geom.Vector{}, geom.Vector{Z: 16}, col)
						above := integrate(x0, x0+w, 8, func(px float64) float64 {
							return integrate(y0, y0+d, 8, func(py float64) float64 { return 20 - heightAt(s, px, py) })
						})
						if op == 1 {
							want = vol + above
							out.MakeFromUnionOf(lid(t, s), &tool)
						} else {
							want = vol - (w*d*16 - above)
							out.MakeFromDifferenceOf(lid(t, s), &tool)
						}
					}
					var m srf.SMesh
					out.TriangulateInto(&m)
					rep := check.Solid(&out, &m)
					if !rep.Sound() || math.Abs(rep.Volume-want) > 2e-4*want {
						failed++
						t.Errorf("%s: %s, want volume %.4f", name, rep.String(), want)
						for _, pr := range rep.Topology {
							t.Logf("  %s at %v", pr.Msg, pr.At)
						}
						for _, e := range rep.Problems.Edges {
							t.Logf("  naked %v %v", e.A, e.B)
						}
					}
				}
			}
		}
	}
	t.Logf("%d points, %d failed", points, failed)
}
