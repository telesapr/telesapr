package topo_test

import (
	"fmt"
	"math"
	"math/rand"
	"testing"

	"github.com/telesapr/telesapr/pkg/check"
	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/internal/par"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/topo"
)

// polygonAt is a regular polygon of n sides about (cx, cy) at height z,
// its corners at radius r, the first at angle rot.
func polygonAt(n int, r, cx, cy, z, rot float64) []srf.SBezier {
	var p []geom.Vector
	for i := range n {
		a := rot + 2*math.Pi*float64(i)/float64(n)
		p = append(p, v(cx+r*math.Cos(a), cy+r*math.Sin(a), z))
	}
	var out []srf.SBezier
	for i := range p {
		out = append(out, srf.SBezierFrom2(p[i], p[(i+1)%n]))
	}
	return out
}

// circleAt is a circle of radius r about (cx, cy) at height z.
func circleAt(r, cx, cy, z float64) []srf.SBezier {
	out := circle(r, z, 0)
	for i := range out {
		for k := 0; k <= out[i].Degree; k++ {
			out[i].Ctrl[k] = out[i].Ctrl[k].Plus(v(cx, cy, 0))
		}
	}
	return out
}

// blobAt is a closed loop of four cubics about (cx, cy) at height z,
// through (±a, 0) and (0, ±b), leaving each at right angles with arms
// of length k.
func blobAt(a, b, k, cx, cy, z float64) []srf.SBezier {
	p := []geom.Vector{v(a, 0, 0), v(0, b, 0), v(-a, 0, 0), v(0, -b, 0)}
	tan := []geom.Vector{v(0, 1, 0), v(-1, 0, 0), v(0, -1, 0), v(1, 0, 0)}
	o := v(cx, cy, z)
	var out []srf.SBezier
	for i := range p {
		q0, q1 := p[i], p[(i+1)%4]
		out = append(out, srf.SBezierFrom4(q0.Plus(o), q0.Plus(tan[i].ScaledBy(k)).Plus(o),
			q1.Minus(tan[(i+1)%4].ScaledBy(k)).Plus(o), q1.Plus(o)))
	}
	return out
}

// loopArea is the area a horizontal loop encloses, by Green's theorem.
func loopArea(loop []srf.SBezier) float64 {
	a := 0.0
	for _, sb := range loop {
		a += integrate(0, 1, 2, func(u float64) float64 {
			p, d := sb.PointAt(u), sb.TangentAt(u)
			return p.X*d.Y - p.Y*d.X
		})
	}
	return math.Abs(a) / 2
}

// partials are the surface's derivatives at (u, v), by central
// differences kept inside [lo, hi] along v.
func partials(bs *srf.BSplineSurface, u, vv, lo, hi float64) (su, sv geom.Vector) {
	const h = 1e-6
	u0, u1 := math.Max(u-h, 0), math.Min(u+h, 1)
	v0, v1 := math.Max(vv-h, lo), math.Min(vv+h, hi)
	su = bs.PointAt(u1, vv).Minus(bs.PointAt(u0, vv)).ScaledBy(1 / (u1 - u0))
	sv = bs.PointAt(u, v1).Minus(bs.PointAt(u, v0)).ScaledBy(1 / (v1 - v0))
	return su, sv
}

// loftVolumeBelow is the volume of a loft of horizontal sections, the
// first at height 0 with loop bottom, below height zc, by the
// divergence theorem with the field (0, 0, z - zc): the cut at zc adds
// nothing, the bottom zc times its area, and the side the integral of
// (z - zc) n_z over the surface up to where it reaches zc.
func loftVolumeBelow(bs *srf.BSplineSurface, bottom []srf.SBezier, zc float64) float64 {
	n := len(breaksOf(bs.KnotsU)) - 1
	bv := breaksOf(bs.KnotsV)
	side := integrate(0, 1, n, func(u float64) float64 {
		// Where this line across reaches zc, by bisection.
		lo, hi := 1.0, 1.0
		if bs.PointAt(u, 1).Z > zc {
			lo = 0
			for range 60 {
				m := (lo + hi) / 2
				if bs.PointAt(u, m).Z < zc {
					lo = m
				} else {
					hi = m
				}
			}
		}
		vc := (lo + hi) / 2
		s := 0.0
		for i := 0; i+1 < len(bv) && bv[i] < vc; i++ {
			a, b := bv[i], math.Min(bv[i+1], vc)
			s += integrate(a, b, 2, func(vv float64) float64 {
				su, sv := partials(bs, u, vv, a, b)
				return (bs.PointAt(u, vv).Z - zc) * su.Cross(sv).Z
			})
		}
		return s
	})
	return side + zc*loopArea(bottom)
}

// breaksOf are a clamped knot vector's distinct knots.
func breaksOf(k []float64) []float64 {
	var out []float64
	for _, x := range k {
		if len(out) == 0 || x > out[len(out)-1] {
			out = append(out, x)
		}
	}
	return out
}

// vAtHeight is where the loft's surface is at height z, by bisection.
func vAtHeight(bs *srf.BSplineSurface, z float64) float64 {
	lo, hi := 0.0, 1.0
	for range 60 {
		m := (lo + hi) / 2
		if bs.PointAt(0, m).Z < z {
			lo = m
		} else {
			hi = m
		}
	}
	return (lo + hi) / 2
}

// The loft goes through each section, and through nothing else at
// their heights.
func TestSmoothLoftThroughSections(t *testing.T) {
	t.Parallel()
	loops := [][]srf.SBezier{circleAt(10, 0, 0, 0), circleAt(14, 2, 0, 10), polygonAt(4, 9, 0, 1, 20, 0.3),
		blobAt(12, 8, 6, 0, 0, 32)}
	bs, err := topo.SmoothLoftSurface(loops, 0)
	if err != nil {
		t.Fatal(err)
	}
	_, ts, err := srf.Skin(nil)
	if err == nil || ts != nil {
		t.Errorf("a skin of nothing made something")
	}
	if bs.DegU != 3 || bs.DegV != 3 {
		t.Errorf("degrees %d by %d, want 3 by 3", bs.DegU, bs.DegV)
	}
	// Each section's height is reached at one v, where the surface lies
	// on the section's curves.
	for i, l := range loops {
		z := l[0].Start().Z
		vz := 0.0
		if i > 0 {
			vz = vAtHeight(bs, z)
		}
		for k := range 41 {
			p := bs.PointAt(float64(k)/40, vz)
			best := math.Inf(1)
			for _, sb := range l {
				tt, _ := sb.ClosestPointTo(p, false)
				best = math.Min(best, sb.PointAt(tt).Minus(p).Magnitude())
			}
			if best > 1e-6 {
				t.Errorf("section %d: the surface at u %.3f is %.3g off its curves", i, float64(k)/40, best)
				break
			}
		}
	}
}

// A circle's loft stays round: circles about one axis interpolate to
// circles, in homogeneous coordinates.
func TestSmoothLoftCircles(t *testing.T) {
	t.Parallel()
	rs := []float64{10, 14, 8, 12}
	zs := []float64{0, 10, 20, 35}
	var loops [][]srf.SBezier
	for i := range rs {
		loops = append(loops, circleAt(rs[i], 0, 0, zs[i]))
	}
	bs, err := topo.SmoothLoftSurface(loops, 0)
	if err != nil {
		t.Fatal(err)
	}
	// The quadrature the sweep checks against, by the divergence
	// theorem, agrees with the volume of revolution, pi R^2 dZ.
	rev := 0.0
	bv := breaksOf(bs.KnotsV)
	for i := 0; i+1 < len(bv); i++ {
		rev += integrate(bv[i], bv[i+1], 2, func(vv float64) float64 {
			p := bs.PointAt(0, vv)
			_, sv := partials(bs, 0, vv, bv[i], bv[i+1])
			return math.Pi * (p.X*p.X + p.Y*p.Y) * sv.Z
		})
	}
	if got := loftVolumeBelow(bs, loops[0], zs[len(zs)-1]); math.Abs(got-rev) > 1e-6*rev {
		t.Errorf("the quadrature gives %.9f, the volume of revolution %.9f", got, rev)
	}
	for k := range 21 {
		vv := float64(k) / 20
		r0 := math.Hypot(bs.PointAt(0, vv).X, bs.PointAt(0, vv).Y)
		for j := 1; j <= 16; j++ {
			p := bs.PointAt(float64(j)/16, vv)
			if math.Abs(math.Hypot(p.X, p.Y)-r0) > 1e-9 {
				t.Fatalf("at v %.2f the section is not round: %g against %g", vv, math.Hypot(p.X, p.Y), r0)
			}
		}
	}
}

func TestSmoothLoftRefusals(t *testing.T) {
	t.Parallel()
	col := core.NewRGBA(100, 100, 100, 255)
	for _, c := range []struct {
		name  string
		loops [][]srf.SBezier
	}{
		{"one profile", [][]srf.SBezier{square(10, 0)}},
		{"open profile", [][]srf.SBezier{square(10, 0), square(10, 10)[:3], square(10, 20)}},
		{"on each other", [][]srf.SBezier{square(10, 0), square(10, 0), square(10, 20)}},
	} {
		if _, err := topo.MakeSmoothLoft(c.loops, 0, col, 1<<30, srf.Tolerance{}); err == nil {
			t.Errorf("%s: lofted", c.name)
		}
	}
}

// Profiles of unequal counts are matched by splitting the longest
// curves.
func TestMatchCounts(t *testing.T) {
	t.Parallel()
	got := topo.MatchCounts([][]srf.SBezier{polygonAt(3, 10, 0, 0, 0, 0), square(10, 5), polygonAt(6, 10, 0, 0, 10, 0)})
	for i, l := range got {
		if len(l) != 6 {
			t.Errorf("loop %d has %d curves, want 6", i, len(l))
		}
		for j := range l {
			if !l[j].Finish().Equals(l[(j+1)%len(l)].Start()) {
				t.Errorf("loop %d broken after curve %d", i, j)
			}
		}
	}
}

// smoothLoftCase is a random loft of 3 to 6 horizontal sections of
// polygons, circles and splines, rising, each about a centre near the
// axis, all of them holding the disc of radius 2 about it.
func smoothLoftCase(r *rand.Rand) [][]srf.SBezier {
	k := 3 + r.Intn(4)
	z := 0.0
	var loops [][]srf.SBezier
	for range k {
		cx, cy := 2*(r.Float64()-0.5), 2*(r.Float64()-0.5)
		rad := 8 + 6*r.Float64()
		switch r.Intn(3) {
		case 0:
			loops = append(loops, polygonAt(3+r.Intn(4), rad, cx, cy, z, r.Float64()))
		case 1:
			loops = append(loops, circleAt(rad, cx, cy, z))
		default:
			loops = append(loops, blobAt(rad, rad*(0.7+0.3*r.Float64()), rad*(0.4+0.2*r.Float64()), cx, cy, z))
		}
		z += 6 + 6*r.Float64()
	}
	return loops
}

// E8.4: smooth lofts on their own, cut by a block above a height,
// joined to one there, and holed by a cylinder, against their volumes by
// quadrature of the surface; rate 0. The lofts are drawn in order, so
// that the random numbers stay the same, and built side by side.
func TestSmoothLoftSweep(t *testing.T) {
	if testing.Short() {
		t.Skip("a sweep")
	}
	t.Parallel()
	tol := srf.Tol(0.005, 100)
	r := rand.New(rand.NewSource(20))
	const cases = 12
	loops := make([][][]srf.SBezier, cases)
	cut := make([]float64, cases) // the block's height, a share of the loft's
	for c := range cases {
		loops[c] = smoothLoftCase(r)
		cut[c] = 0.3 + 0.4*r.Float64()
	}
	// What each case's four solids found wrong, logged in order
	// afterwards.
	found := make([][][]string, cases)
	par.For(cases, func(c int) {
		found[c] = smoothLoftPoint(loops[c], cut[c], tol)
	})
	failed := 0
	for c := range cases {
		for op, lines := range found[c] {
			if len(lines) == 0 {
				continue
			}
			failed++
			t.Errorf("case %d (%d sections), op %d: %s", c, len(loops[c]), op, lines[0])
			for _, l := range lines[1:] {
				t.Logf("  %s", l)
			}
		}
	}
	t.Logf("%d points, %d failed", 4*cases, failed)
}

// smoothLoftPoint lofts loops at tol on its own (op 0), less a block
// above the share cut of its height (1), with that block (2) and less a
// cylinder along its axis (3), and gives for each op what is wrong: the
// report and the volume wanted, then its problems; nothing when it is
// sound with the volume wanted.
func smoothLoftPoint(loops [][]srf.SBezier, cut float64, tol srf.Tolerance) [][]string {
	col := core.NewRGBA(100, 100, 100, 255)
	found := make([][]string, 4)
	top := loops[len(loops)-1][0].Start().Z
	bs, err := topo.SmoothLoftSurface(loops, 0)
	if err != nil {
		for op := range found {
			found[op] = []string{err.Error()}
		}
		return found
	}
	whole := loftVolumeBelow(bs, loops[0], top)
	zc := top * cut
	below := loftVolumeBelow(bs, loops[0], zc)
	for op := range 4 {
		sh, err := topo.MakeSmoothLoft(loops, 0, col, srf.OriginFor(9, 0), tol)
		if err != nil {
			found[op] = []string{err.Error()}
			continue
		}
		tool, out := srf.SShell{Tol: tol}, srf.SShell{Tol: tol}
		want := whole
		box := func(z0, z1 float64) {
			tool.MakePrism([]geom.Vector{v(-30, -30, z0), v(30, -30, z0), v(30, 30, z0), v(-30, 30, z0)},
				geom.Vector{}, v(0, 0, z1-z0), col)
		}
		switch op {
		case 0:
			out = *sh
		case 1:
			box(zc, top+5)
			out.MakeFromDifferenceOf(sh, &tool)
			want = below
		case 2:
			box(zc, top+5)
			out.MakeFromUnionOf(sh, &tool)
			want = below + 60*60*(top+5-zc)
		case 3:
			const hr = 2.0
			tool.MakeFromTransformationOf(cylinder(hr, top+10, tol), v(0, 0, -5), geom.QuaternionIdentity, 1)
			out.MakeFromDifferenceOf(sh, &tool)
			want = whole - math.Pi*hr*hr*top
		}
		var m srf.SMesh
		out.TriangulateInto(&m)
		rep := check.Solid(&out, &m)
		// The mesh lies within the chord tolerance of the surface,
		// about 12 mm round: 1e-3 of the volume is some 0.005 mm over
		// the whole of it.
		if !rep.Sound() || math.Abs(rep.Volume-want) > 1e-3*want {
			found[op] = []string{fmt.Sprintf("%s, want volume %.4f", rep.String(), want)}
			for _, pr := range rep.Topology {
				found[op] = append(found[op], fmt.Sprintf("%s at %v", pr.Msg, pr.At))
			}
		}
	}
	return found
}

func BenchmarkSmoothLoft(b *testing.B) {
	loops := [][]srf.SBezier{circleAt(10, 0, 0, 0), circleAt(14, 2, 0, 10), polygonAt(4, 9, 0, 1, 20, 0.3),
		blobAt(12, 8, 6, 0, 0, 32)}
	col := core.NewRGBA(100, 100, 100, 255)
	for b.Loop() {
		sh, err := topo.MakeSmoothLoft(loops, 0, col, 1<<30, srf.Tolerance{})
		if err != nil {
			b.Fatal(err)
		}
		var m srf.SMesh
		sh.TriangulateInto(&m)
	}
}
