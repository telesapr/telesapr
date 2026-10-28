package topo_test

import (
	"bufio"
	"cmp"
	"fmt"
	"math"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/check"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/topo"
)

// top are the rib's five edges that stick out: the two along its slope,
// the two up its back and the one across its back.
func (rb rib) top() [][2]geom.Vector {
	h, l := rb.h, rb.lean
	return [][2]geom.Vector{
		{v(0, 0, h), v(ribFoot, 0, 0)},
		{v(l, ribWide, h), v(ribFoot+l, ribWide, 0)},
		{v(0, 0, h), v(l, ribWide, h)},
		{v(0, 0, 0), v(0, 0, h)},
		{v(l, ribWide, 0), v(l, ribWide, h)},
	}
}

// ribOracle is a line of testdata/topo/fillet-rib-all-over.txt: what
// OpenCASCADE's volumes are with the rib's top rounded, and with its foot
// filled after; refused where it made none.
type ribOracle struct {
	h, lean, r float64
	top, both  float64
	refused    bool
}

func readRibOracle(t *testing.T) []ribOracle {
	t.Helper()
	f, err := os.Open("../../testdata/topo/fillet-rib-all-over.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	var out []ribOracle
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fs := strings.Fields(sc.Text())
		if len(fs) < 4 {
			continue
		}
		var x [5]float64
		o := ribOracle{refused: fs[3] == "refused"}
		for i, s := range fs {
			if i < 3 || !o.refused {
				if x[i], err = strconv.ParseFloat(s, 64); err != nil {
					t.Fatal(err)
				}
			}
		}
		o.h, o.lean, o.r, o.top, o.both = x[0], x[1], x[2], x[3], x[4]
		out = append(out, o)
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// ribAllOverKnown are the cases of TestFilletRibAllOverOracle that are
// unsound today, with why; the list may only shrink.
var ribAllOverKnown = map[string]string{
	"h10_lean-4_r0.5": "three mesh edges cross at the front corners, where the foot's fillet " +
		"passes from the slope edge's fillet onto the slope; sound at chord 0.0001",
	"h5_lean-4_r0.5": "five mesh edges cross, as h10_lean-4_r0.5's",
}

// TestFilletRibAllOverOracle rounds a rib all over: its top's edges and
// its foot's picked together, in one edit. The top's convex edges are
// rounded first (filletInStages), which leaves its foot one closed chain
// going round from its flat sides onto the fillets of its uprights and,
// where they lean, of its slope's edges; the rolling ball fills it
// (filletOneChain). The volume the foot adds is checked against
// OpenCASCADE's, made the same way (scripts/fillet-rib-fixtures.py),
// where the rib's width stands square; and everywhere, OpenCASCADE's
// refusals too, the fillet's height over the plate against the balls
// that roll round the foot (ballHeight).
func TestFilletRibAllOverOracle(t *testing.T) {
	t.Parallel()
	for _, o := range readRibOracle(t) {
		if testing.Short() && (o.h != 10 || o.lean != 2 || o.r != 1) {
			// Each case takes 10 to 20 s; the short run keeps one.
			continue
		}
		name := fmt.Sprintf("h%g_lean%g_r%g", o.h, o.lean, o.r)
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if why, ok := ribAllOverKnown[name]; ok {
				t.Skip(why)
			}
			rb := rib{o.h, o.lean}
			sides, ends := rb.foot()
			tol := srf.Tol(0.0003*o.r, 400)
			topOnly := rb.shell(tol)
			if err := topo.FilletEdges(topOnly, rb.top(), o.r, 1<<30); err != nil {
				t.Fatal(err)
			}
			sh := rb.shell(tol)
			all := append(append(append([][2]geom.Vector{}, sides...), ends...), rb.top()...)
			if err := topo.FilletEdges(sh, all, o.r, 1<<30); err != nil {
				t.Fatal(err)
			}
			var m srf.SMesh
			sh.TriangulateInto(&m)
			rep := check.Solid(sh, &m)
			tp, err := topo.Build(sh)
			if err != nil {
				t.Fatal(err)
			}
			if !rep.Sound() {
				t.Fatalf("unsound: %s", rep.String())
			}
			if p := tp.Validate(); len(p) > 0 {
				t.Fatalf("topology: %s", p[0].Msg)
			}
			var top srf.SMesh
			topOnly.TriangulateInto(&top)
			if o.lean == 0 && !o.refused {
				added, want := rep.Volume-meshVolume(topOnly), o.both-o.top
				// The foot's fillet runs about 46 round a rib of 14 by
				// 10; its chords lose less than the tolerance over it,
				// and the top's fillets' chords, meshed again where the
				// foot's cut them, as much.
				allow := 2*0.0003*o.r*math.Pi/2*o.r*(2*(ribFoot+ribWide)+8*o.r) + 0.002
				if math.Abs(added-want) > allow {
					t.Errorf("the foot added %.6f, OpenCASCADE %.6f, allowed %.4f", added, want, allow)
				}
			}
			// Where the width leans, OpenCASCADE's fillet lies inside the
			// balls' envelope beside the front corners, up to 0.009 of a
			// radius of 1 at h 40 (its volume 1.5% off): the heights are
			// checked against the balls themselves.
			for _, q := range ribBallPoints(rb, o.r) {
				if !math.IsInf(firstHeight(&top, q), 1) {
					// Under the rib.
					continue
				}
				want := ballHeight(&top, q, o.r)
				if want <= 0.01*o.r || want >= 0.95*o.r {
					continue
				}
				got := firstHeight(&m, q)
				// Compared up or sideways, as the ball's centre is found:
				// near the walls the fillet stands steep, and its height
				// moves by many times a shift sideways.
				side := func(z float64) float64 { return math.Sqrt(math.Max(0, o.r*o.r-(o.r-z)*(o.r-z))) }
				allow := 4 * 0.0003 * o.r
				if math.Abs(got-want) > allow && math.Abs(side(got)-side(want)) > allow {
					t.Errorf("at (%.2f, %.2f) the fillet is %.5f high, the balls' envelope %.5f", q.X, q.Y, got, want)
				}
			}
		})
	}
}

// ribBallPoints are points of the plate round the rib's foot where its
// fillet's height is checked: round its front corners, where the fillets
// of the slope's edges lean, and across the middle of each side.
func ribBallPoints(rb rib, r float64) []geom.Vector {
	var out []geom.Vector
	for _, c := range []geom.Vector{v(ribFoot, 0, 0), v(ribFoot+rb.lean, ribWide, 0), v(0, 0, 0)} {
		for i := range 9 {
			for j := range 9 {
				out = append(out, c.Plus(v((float64(i)-4)*0.4*r, (float64(j)-4)*0.4*r, 0)))
			}
		}
	}
	for i := range 6 {
		d := float64(i) * 0.2 * r
		out = append(out, v(ribFoot/2, -d, 0), v(ribFoot/2+rb.lean, ribWide+d, 0))
	}
	return out
}

// firstHeight is where a ray up from q, on the plate, first leaves the
// solid meshed by m: the fillet's height over the plate there.
func firstHeight(m *srf.SMesh, q geom.Vector) float64 {
	best := math.Inf(1)
	for _, tr := range m.Triangles {
		d := (tr.B.Y-tr.C.Y)*(tr.A.X-tr.C.X) + (tr.C.X-tr.B.X)*(tr.A.Y-tr.C.Y)
		if math.Abs(d) < 1e-15 {
			continue
		}
		l1 := ((tr.B.Y-tr.C.Y)*(q.X-tr.C.X) + (tr.C.X-tr.B.X)*(q.Y-tr.C.Y)) / d
		l2 := ((tr.C.Y-tr.A.Y)*(q.X-tr.C.X) + (tr.A.X-tr.C.X)*(q.Y-tr.C.Y)) / d
		if l1 < 0 || l2 < 0 || l1+l2 > 1 {
			continue
		}
		if z := l1*tr.A.Z + l2*tr.B.Z + (1-l1-l2)*tr.C.Z; z > 1e-6 && z < best {
			best = z
		}
	}
	return best
}

// ballHeight is how high the fillet of radius r at the foot of the solid
// meshed by m stands over q, on the plate: the lowest point over q that
// a ball on the plate, clear of the solid, reaches. The ball's centre
// nearest q sideways is found along directions round it, as far as the
// ball is clear (closer, it would cut into the solid).
func ballHeight(m *srf.SMesh, q geom.Vector, r float64) float64 {
	var near []srf.STriangle
	for _, tr := range m.Triangles {
		// By its box: a large triangle passes near with its corners far.
		hi, lo := tr.A, tr.A
		tr.B.MakeMaxMin(&hi, &lo)
		tr.C.MakeMaxMin(&hi, &lo)
		dx := math.Max(0, math.Max(lo.X-q.X, q.X-hi.X))
		dy := math.Max(0, math.Max(lo.Y-q.Y, q.Y-hi.Y))
		if hi.Z > 1e-9 && lo.Z < 3*r && dx < 3*r && dy < 3*r {
			near = append(near, tr)
		}
	}
	free := func(a, d float64) bool {
		c := v(q.X+d*math.Cos(a), q.Y+d*math.Sin(a), r)
		for i := range near {
			if triangleDistance(&near[i], c) < r-1e-9 {
				return false
			}
		}
		return true
	}
	reach := func(a float64) float64 {
		if !free(a, r) {
			return r
		}
		lo, hi := 0.0, r
		for range 30 {
			if mid := (lo + hi) / 2; free(a, mid) {
				hi = mid
			} else {
				lo = mid
			}
		}
		return hi
	}
	// Every 5 degrees round, then finer about the three best directions.
	type dir struct{ a, d float64 }
	var coarse []dir
	for k := range 72 {
		a := float64(k) * math.Pi / 36
		coarse = append(coarse, dir{a, reach(a)})
	}
	slices.SortFunc(coarse, func(x, y dir) int { return cmp.Compare(x.d, y.d) })
	best := coarse[0].d
	for _, c := range coarse[:3] {
		ba := c.a
		for _, step := range []float64{math.Pi / 360, math.Pi / 3600} {
			b0 := ba
			for k := -10; k <= 10; k++ {
				if d := reach(b0 + float64(k)*step); d < best {
					best, ba = d, b0+float64(k)*step
				}
			}
		}
	}
	return r - math.Sqrt(math.Max(0, r*r-best*best))
}

// triangleDistance is how far p lies from triangle tr.
func triangleDistance(tr *srf.STriangle, p geom.Vector) float64 {
	a, b, c := tr.A, tr.B, tr.C
	n := b.Minus(a).Cross(c.Minus(a))
	if n.Magnitude() > 0 {
		n = n.WithMagnitude(1)
		f := p.Minus(n.ScaledBy(p.Minus(a).Dot(n)))
		in := true
		for _, e := range [][2]geom.Vector{{a, b}, {b, c}, {c, a}} {
			if e[1].Minus(e[0]).Cross(f.Minus(e[0])).Dot(n) < 0 {
				in = false
			}
		}
		if in {
			return math.Abs(p.Minus(a).Dot(n))
		}
	}
	best := math.Inf(1)
	for _, e := range [][2]geom.Vector{{a, b}, {b, c}, {c, a}} {
		ab := e[1].Minus(e[0])
		s := 0.0
		if l2 := ab.Dot(ab); l2 > 0 {
			s = math.Max(0, math.Min(1, p.Minus(e[0]).Dot(ab)/l2))
		}
		best = math.Min(best, p.Minus(e[0].Plus(ab.ScaledBy(s))).Magnitude())
	}
	return best
}

// TestFilletRibFootAfterTop fills the foot of a rib whose top was
// rounded before, in an edit of its own: the foot's eight edges picked,
// or one of them, the rest going on from it round the chain.
func TestFilletRibFootAfterTop(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("fills a rib's foot twice, about 15 s")
	}
	rb := rib{10, 2}
	tol := srf.Tol(0.0003, 400)
	before := rb.shell(tol)
	if err := topo.FilletEdges(before, rb.top(), 1, 1<<30); err != nil {
		t.Fatal(err)
	}
	tp, err := topo.Build(before)
	if err != nil {
		t.Fatal(err)
	}
	var loop []topo.FilletPick
	for e := range tp.Edges {
		pts := tp.Edges[e].Points
		on := len(pts) >= 2
		for _, p := range pts {
			on = on && math.Abs(p.Z) < 1e-6 && p.X > -6 && p.X < 24 && p.Y > -6 && p.Y < 16
		}
		switch {
		case !on:
		case tp.Straight(e):
			loop = append(loop, topo.FilletPick{A: pts[0], B: pts[len(pts)-1]})
		default:
			loop = append(loop, topo.FilletPick{Arc: true, At: pts[len(pts)/2]})
		}
	}
	if len(loop) != 8 {
		t.Fatalf("the foot has %d edges, want 8", len(loop))
	}
	for _, c := range []struct {
		name  string
		picks []topo.FilletPick
	}{{"all", loop}, {"one", loop[1:2]}} {
		var sh srf.SShell
		sh.MakeFromCopyOf(before)
		if err := topo.FilletPicks(&sh, c.picks, 1, 1<<29, false); err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		var m srf.SMesh
		sh.TriangulateInto(&m)
		if rep := check.Solid(&sh, &m); !rep.Sound() {
			t.Errorf("%s: unsound: %s", c.name, rep.String())
		}
	}
}
