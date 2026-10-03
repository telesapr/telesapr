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

// hermite is the cubic from p0 to p1 with derivatives d0 and d1 over
// its parameter.
func hermite(p0, d0, p1, d1 geom.Vector) srf.SBezier {
	return srf.SBezierFrom4(p0, p0.Plus(d0.ScaledBy(1.0/3)), p1.Minus(d1.ScaledBy(1.0/3)), p1)
}

// helixPath is a line of length lead along the helix's start, then the
// helix of radius r and pitch about the z axis, turns times round, as
// cubics through eight points a turn.
func helixPath(r, pitch, turns, lead float64) []srf.SBezier {
	at := func(a float64) (geom.Vector, geom.Vector) {
		return v(r*math.Cos(a), r*math.Sin(a), pitch*a/(2*math.Pi)),
			v(-r*math.Sin(a), r*math.Cos(a), pitch/(2*math.Pi))
	}
	p0, d0 := at(0)
	path := []srf.SBezier{srf.SBezierFrom2(p0.Minus(d0.WithMagnitude(lead)), p0)}
	n := int(math.Round(8 * turns))
	da := 2 * math.Pi * turns / float64(n)
	for k := 0; k < n; k++ {
		a, b := float64(k)*da, float64(k+1)*da
		pa, ta := at(a)
		pb, tb := at(b)
		path = append(path, hermite(pa, ta.ScaledBy(da), pb, tb.ScaledBy(da)))
	}
	return path
}

// wanderPath is a line of length lead, then cubics joined smoothly
// through random points about 30 apart, rising, bending gently.
func wanderPath(r *rand.Rand, n int, lead float64, planar bool) []srf.SBezier {
	p := v(0, 0, 0)
	d := v(30, 0, 0)
	path := []srf.SBezier{srf.SBezierFrom2(p.Minus(d.WithMagnitude(lead)), p)}
	for k := 0; k < n; k++ {
		turn := 0.9 * (r.Float64() - 0.5)
		lift := 0.0
		if !planar {
			lift = 0.6 * (r.Float64() - 0.3)
		}
		nd := v(d.X*math.Cos(turn)-d.Y*math.Sin(turn), d.X*math.Sin(turn)+d.Y*math.Cos(turn), 0)
		nd = nd.WithMagnitude(30).Plus(v(0, 0, 30*lift)).WithMagnitude(30)
		q := p.Plus(d.Plus(nd).ScaledBy(0.5))
		path = append(path, hermite(p, d, q, nd))
		p, d = q, nd
	}
	return path
}

// profileAcross is a profile in the plane across the path's start,
// about its start point: kind 0 a circle of radius 3, 1 a rectangle 6
// by 4, 2 a square of side 7 with a round hole of radius 1.5. Its area
// comes back with it.
func profileAcross(path []srf.SBezier, kind int) ([][]srf.SBezier, float64) {
	o := path[0].Start()
	t := path[0].TangentAt(0).WithMagnitude(1)
	e1 := t.NormalDir(0).WithMagnitude(1)
	e2 := t.Cross(e1)
	at := func(x, y float64) geom.Vector { return o.Plus(e1.ScaledBy(x)).Plus(e2.ScaledBy(y)) }
	circ := func(rad float64) []srf.SBezier {
		var out []srf.SBezier
		for k := 0; k < 4; k++ {
			a0 := float64(k) * math.Pi / 2
			sb := srf.SBezierFrom3(at(rad*math.Cos(a0), rad*math.Sin(a0)),
				at(rad*math.Sqrt2*math.Cos(a0+math.Pi/4), rad*math.Sqrt2*math.Sin(a0+math.Pi/4)),
				at(rad*math.Cos(a0+math.Pi/2), rad*math.Sin(a0+math.Pi/2)))
			sb.Weight[1] = math.Sqrt2 / 2
			out = append(out, sb)
		}
		return out
	}
	rect := func(w, h float64) []srf.SBezier {
		c := []geom.Vector{at(-w/2, -h/2), at(w/2, -h/2), at(w/2, h/2), at(-w/2, h/2)}
		var out []srf.SBezier
		for i := range c {
			out = append(out, srf.SBezierFrom2(c[i], c[(i+1)%4]))
		}
		return out
	}
	switch kind {
	case 0:
		return [][]srf.SBezier{circ(3)}, math.Pi * 9
	case 1:
		return [][]srf.SBezier{rect(6, 4)}, 24
	}
	return [][]srf.SBezier{rect(7, 7), circ(1.5)}, 49 - math.Pi*2.25
}

// pathLength is the length of a chain of curves, by quadrature.
func pathLength(path []srf.SBezier) float64 {
	l := 0.0
	for _, sb := range path {
		l += integrate(0, 1, 4, func(t float64) float64 { return sb.TangentAt(t).Magnitude() })
	}
	return l
}

func newPath(t testing.TB, path []srf.SBezier, opts topo.PathSweepOptions) *topo.SweepPath {
	t.Helper()
	t0 := path[0].TangentAt(0)
	sp, err := topo.NewSweepPath(path, t0.NormalDir(0), opts)
	if err != nil {
		t.Fatal(err)
	}
	return sp
}

// Rotation-minimising frames along a helix turn at its constant rate
// about the tangent: the normal's angle to the principal normal grows
// as the torsion's integral, and a frame kept to the axis faces it.
func TestSweepFrames(t *testing.T) {
	const r, pitch = 20.0, 15.0
	path := helixPath(r, pitch, 1, 0)[1:]
	sp := newPath(t, path, topo.PathSweepOptions{})
	c := pitch / (2 * math.Pi)
	// A circle's quarter by a cubic is within 3e-4 of its radius; the
	// length the same within that.
	if want := 2 * math.Pi * math.Hypot(r, c); math.Abs(sp.Length()-want) > 1e-3*want {
		t.Errorf("length %g, want %g", sp.Length(), want)
	}
	// A frame's motion keeps lengths, and takes the start to the point.
	m := sp.Motion(sp.End() / 2)
	a, b := path[0].Start(), path[0].Start().Plus(v(1, 2, 3))
	if d := m(b).Minus(m(a)).Magnitude(); math.Abs(d-math.Sqrt(14)) > 1e-9 {
		t.Errorf("the motion stretches: %g", d)
	}
	if !m(a).EqualsTol(path[4].Start(), 1e-9) {
		t.Errorf("the start moves to %v, not %v", m(a), path[4].Start())
	}
	// Torsion tau = c / (r² + c²) over the length: the minimising frame
	// turns against the Frenet frame by tau times the arc length.
	tau := c / (r*r + c*c)
	want := tau * sp.Length()
	tangent := func(s float64) geom.Vector {
		mm := sp.Motion(s)
		return mm(path[0].Start().Plus(path[0].TangentAt(0).WithMagnitude(1))).Minus(mm(path[0].Start()))
	}
	inward := func(s float64) geom.Vector {
		// The principal normal of a helix points at its axis.
		mm := sp.Motion(s)
		p := mm(path[0].Start())
		return v(-p.X, -p.Y, 0).WithMagnitude(1)
	}
	start := path[0].Start()
	n0 := v(-start.X, -start.Y, 0).WithMagnitude(1)
	end := sp.End()
	mEnd := sp.Motion(end)
	got := mEnd(start.Plus(n0)).Minus(mEnd(start))
	angle := math.Acos(math.Max(-1, math.Min(1, got.Dot(inward(end)))))
	if math.Abs(angle-want) > 0.01 {
		t.Errorf("the frame turned by %g about the tangent, want %g", angle, want)
	}
	if d := tangent(end).Dot(path[len(path)-1].TangentAt(1).WithMagnitude(1)); math.Abs(d-1) > 1e-9 {
		t.Errorf("the frame's tangent is off the path's by %g", d)
	}
	// Up along the axis: the profile's inward direction stays inward, as
	// far as cubics follow the helix.
	up := newPath(t, path, topo.PathSweepOptions{Up: v(0, 0, 1)})
	if _, err := topo.NewSweepPath(path, v(0, 0, 1), topo.PathSweepOptions{Up: path[0].TangentAt(0)}); err == nil {
		t.Error("an up direction along the tangent made frames")
	}
	for _, s := range []float64{1.3, 4.5, end} {
		mm := up.Motion(s)
		in := mm(start.Plus(n0)).Minus(mm(start))
		if d := in.Dot(inward(s)); math.Abs(d-1) > 1e-4 {
			t.Errorf("held up, at %g the inward direction is %g of the way in", s, d)
		}
	}
	// A twist of a whole turn brings the frame back at the end.
	tw := newPath(t, path, topo.PathSweepOptions{Twist: 2 * math.Pi})
	if a, b := tw.Motion(end)(start.Plus(n0)), mEnd(start.Plus(n0)); !a.EqualsTol(b, 1e-9) {
		t.Errorf("a whole twist ends at %v, not %v", a, b)
	}
	half := newPath(t, path, topo.PathSweepOptions{Twist: math.Pi})
	if a, b := half.Motion(end)(start.Plus(n0)).Minus(mEnd(start)), got; math.Abs(a.Dot(b)+1) > 1e-9 {
		t.Errorf("half a twist ends at %v, not against %v", a, b)
	}
}

// The fitted surface stays within its tolerance of the true sweep
// between the sections it was measured at, and passes through the
// profile at each section; a tighter tolerance takes more sections, once
// the four a piece it starts with are not enough.
func TestSweepFitting(t *testing.T) {
	path := helixPath(20, 15, 1.5, 10)
	loops, _ := profileAcross(path, 0)
	prev := 0
	for _, tol := range []float64{1e-2, 1e-3, 1e-4} {
		sp := newPath(t, path, topo.PathSweepOptions{Tol: tol})
		surfs, pieces, err := sp.SweptSurfaces(loops)
		if err != nil {
			t.Fatal(err)
		}
		sections := 0
		for _, ss := range pieces {
			sections += len(ss)
		}
		if sections < prev || (tol == 1e-4 && sections == prev) {
			t.Errorf("tolerance %g took %d sections, against %d for the one before", tol, sections, prev)
		}
		prev = sections
		n := float64(len(loops[0]))
		worst := 0.0
		for pi, ss := range pieces {
			bs := surfs[0][pi]
			// The nearest point of the curve across at u to p: a scan,
			// then a ternary search about the best.
			nearest := func(u float64, p geom.Vector) float64 {
				const steps = 300
				best, bv := math.Inf(1), 0.0
				for j := 0; j <= steps; j++ {
					if d := bs.PointAt(u, float64(j)/steps).Minus(p).Magnitude(); d < best {
						best, bv = d, float64(j)/steps
					}
				}
				lo, hi := math.Max(bv-1.0/steps, 0), math.Min(bv+1.0/steps, 1)
				for range 60 {
					m1, m2 := lo+(hi-lo)/3, hi-(hi-lo)/3
					if bs.PointAt(u, m1).Minus(p).Magnitude() < bs.PointAt(u, m2).Minus(p).Magnitude() {
						hi = m2
					} else {
						lo = m1
					}
				}
				return bs.PointAt(u, (lo+hi)/2).Minus(p).Magnitude()
			}
			for k := 0; k+1 < len(ss); k += 2 {
				for _, f := range []float64{0, 0.37, 0.71} {
					m := sp.Motion(ss[k] + f*(ss[k+1]-ss[k]))
					for i, sb := range loops[0] {
						for _, ut := range []float64{0.1, 0.6} {
							d := nearest((float64(i)+ut)/n, m(sb.PointAt(ut)))
							if f == 0 && d > 1e-9 {
								t.Errorf("tolerance %g: section %d of piece %d is %g off the profile", tol, k, pi, d)
							}
							worst = math.Max(worst, d)
						}
					}
				}
			}
		}
		t.Logf("tolerance %g: %d sections, off by %.3g at most", tol, sections, worst)
		if worst > 2*tol {
			t.Errorf("tolerance %g: the surface is off the sweep by %g", tol, worst)
		}
	}
}

// pathSweepCase is a path and profile for the sweep: a helix, a planar
// wander, or one that climbs, each led by a straight line.
func pathSweepCase(r *rand.Rand, c int) []srf.SBezier {
	switch c % 3 {
	case 0:
		return helixPath(14+10*r.Float64(), 12+8*r.Float64(), 0.75+r.Float64(), 10)
	case 1:
		return wanderPath(r, 2+r.Intn(3), 10, true)
	}
	return wanderPath(r, 2+r.Intn(3), 10, false)
}

// E10.4: profiles swept along helices and cubics, on their own, with a
// block on the straight lead, and cut by it, against the centroid
// theorem: a profile about the path keeps its area times the path's
// length, planar or not, twisted or not; rate 0.
func TestPathSweepSweep(t *testing.T) {
	if testing.Short() {
		t.Skip("a sweep")
	}
	old, oldSegs := srf.ChordTolMm(), srf.GetMaxSegments()
	srf.SetChordTolMm(0.005)
	srf.SetMaxSegments(100)
	defer func() { srf.SetChordTolMm(old); srf.SetMaxSegments(oldSegs) }()
	r := rand.New(rand.NewSource(10))
	col := core.NewRgbaColor(100, 100, 100, 255)
	points, failed := 0, 0
	for c := 0; c < 9; c++ {
		path := pathSweepCase(r, c)
		kind := c % 3
		loops, area := profileAcross(path, (c/3+kind)%3)
		perimeter := 0.0
		for _, l := range loops {
			perimeter += pathLength(l)
		}
		opts := topo.PathSweepOptions{Tol: 5e-4}
		if c%2 == 1 {
			opts.Twist = math.Pi / 2
		}
		sp := newPath(t, path, opts)
		whole := area * pathLength(path)
		// A block 14 across, from 6 behind the start to 5 along the lead.
		t0 := path[0].TangentAt(0).WithMagnitude(1)
		e1 := t0.NormalDir(0).WithMagnitude(1)
		e2 := t0.Cross(e1)
		o := path[0].Start().Minus(t0.ScaledBy(6))
		var sq []geom.Vector
		for _, xy := range [][2]float64{{-7, -7}, {7, -7}, {7, 7}, {-7, 7}} {
			sq = append(sq, o.Plus(e1.ScaledBy(xy[0])).Plus(e2.ScaledBy(xy[1])))
		}
		const blockVol, inside = 14 * 14 * 11.0, 5.0
		for op := 0; op < 4; op++ {
			points++
			name := fmt.Sprintf("case %d, op %d", c, op)
			sh, err := topo.MakePathSweep(sp, loops, col, srf.OriginFor(9, 0), srf.OriginFor(9, 1), srf.OriginFor(9, 2))
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			var block, out srf.SShell
			block.MakePrism(sq, geom.Vector{}, t0.ScaledBy(11), col)
			want := whole
			switch op {
			case 0:
				out = *sh
			case 1:
				out.MakeFromUnionOf(sh, &block)
				want = whole + blockVol - area*inside
			case 2:
				out.MakeFromDifferenceOf(sh, &block)
				want = whole - area*inside
			case 3:
				out.MakeFromDifferenceOf(&block, sh)
				want = blockVol - area*inside
			}
			var m srf.SMesh
			out.TriangulateInto(&m)
			rep := check.Solid(&out, &m)
			// The mesh lies within the chord tolerance of the surface,
			// inside where it is convex: the volume within the
			// tolerance times the surface's area, the profile's
			// perimeter along the path and the block's faces.
			allow := srf.ChordTolMm() * (perimeter*pathLength(path) + 6*14*14)
			if !rep.Sound() || math.Abs(rep.Volume-want) > allow {
				failed++
				t.Errorf("%s: %s, want volume %.4f within %.3g", name, rep.String(), want, allow)
				for _, pr := range rep.Topology {
					t.Logf("  %s at %v", pr.Msg, pr.At)
				}
			}
		}
	}
	t.Logf("%d points, %d failed", points, failed)
}

func TestPathSweepRefusals(t *testing.T) {
	path := helixPath(20, 15, 1, 10)
	loops, _ := profileAcross(path, 0)
	if _, err := topo.NewSweepPath(nil, v(1, 0, 0), topo.PathSweepOptions{}); err == nil {
		t.Error("no path made frames")
	}
	broken := append([]srf.SBezier{path[0]}, path[2:]...)
	if _, err := topo.NewSweepPath(broken, v(0, 0, 1), topo.PathSweepOptions{}); err == nil {
		t.Error("a broken path made frames")
	}
	sp := newPath(t, path, topo.PathSweepOptions{})
	// A profile the path runs along.
	o := path[0].Start()
	t0 := path[0].TangentAt(0).WithMagnitude(1)
	n := t0.NormalDir(0).WithMagnitude(1)
	flat := []srf.SBezier{srf.SBezierFrom2(o, o.Plus(t0)), srf.SBezierFrom2(o.Plus(t0), o.Plus(n)), srf.SBezierFrom2(o.Plus(n), o)}
	if _, _, err := sp.SweptSurfaces([][]srf.SBezier{flat}); err == nil {
		t.Error("a profile along the path swept")
	}
	// Too few sections for the tolerance.
	tight := newPath(t, path, topo.PathSweepOptions{Tol: 1e-9, MaxSections: 20})
	if _, _, err := tight.SweptSurfaces(loops); err == nil {
		t.Error("a sweep fitted within 1e-9 in 20 sections")
	}
}

func BenchmarkPathSweep(b *testing.B) {
	path := helixPath(20, 15, 1.5, 10)
	loops, _ := profileAcross(path, 2)
	col := core.NewRgbaColor(100, 100, 100, 255)
	for b.Loop() {
		sp, err := topo.NewSweepPath(path, path[0].TangentAt(0).NormalDir(0), topo.PathSweepOptions{Tol: 5e-4})
		if err != nil {
			b.Fatal(err)
		}
		sh, err := topo.MakePathSweep(sp, loops, col, 1<<30, 1<<30+1, 1<<30+2)
		if err != nil {
			b.Fatal(err)
		}
		var m srf.SMesh
		sh.TriangulateInto(&m)
	}
}
