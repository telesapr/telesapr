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
	"github.com/telesapr/telesapr/pkg/internal/fixtures"
	"github.com/telesapr/telesapr/pkg/internal/sweep"
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

// ribOracle is a line of testdata/topo/fillet-rib-all-over.txt or
// fillet-rib-foot-first.txt: what OpenCASCADE's volumes are with the
// rib's top rounded (or its foot filled) first, and with the rest
// rounded after; refused where it made none.
type ribOracle struct {
	h, lean, r  float64
	first, both float64
	refused     bool
}

func readRibOracle(t *testing.T) []ribOracle {
	t.Helper()
	return readRibOracleFile(t, "../../testdata/topo/fillet-rib-all-over.txt")
}

// readRibOracleFile reads the lines of a rib's oracle file.
func readRibOracleFile(t *testing.T, name string) []ribOracle {
	t.Helper()
	f, err := os.Open(name)
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
		o.h, o.lean, o.r, o.first, o.both = x[0], x[1], x[2], x[3], x[4]
		out = append(out, o)
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// TestFilletRibAllOverOracleCDT rounds a rib all over: its top's edges
// and its foot's picked together, in one edit, every curved face meshed
// by the CDT, the default mesher. The top's convex edges are rounded
// first (filletInStages), which leaves its foot one closed chain going
// round from its flat sides onto the fillets of its uprights and, where
// they lean, of its slope's edges; the rolling ball fills it
// (filletOneChain). The volume the foot adds is checked against
// OpenCASCADE's, made the same way (scripts/fillet-rib-fixtures.py),
// where the rib's width stands square; and everywhere, OpenCASCADE's
// refusals too, the fillet's height over the plate against the balls
// that roll round the foot (ballHeight). The ordinary run takes 12 of the
// 24 rows (ribAllOverCDTRows), TELESAPR_SWEEP=1 all; heavy (HEAVY in
// scripts/go-test.sh): the 24 take some 25 minutes of processor.
func TestFilletRibAllOverOracleCDT(t *testing.T) {
	t.Parallel()
	ribAllOverRows(t, srf.MesherCDT, ribAllOverCDTRows)
}

// TestFilletRibAllOverOracleEarClip is TestFilletRibAllOverOracleCDT
// with the curved faces ear-clipped, the mesher a Tolerance can still ask
// for and the CDT's mender falls back to: the same B-rep, other meshes,
// which the fillet's own soundness checks mesh as it goes. Four rows in
// the ordinary run, one of each height (ribAllOverEarClipRows), all 24
// under TELESAPR_SWEEP=1; not under -short or the race detector, where
// the CDT's test keeps its row.
func TestFilletRibAllOverOracleEarClip(t *testing.T) {
	t.Parallel()
	if testing.Short() || fixtures.RaceDetector {
		t.Skip("ear-clipped ribs rounded all over, minutes; the CDT's test keeps a row")
	}
	ribAllOverRows(t, srf.MesherEarClip, ribAllOverEarClipRows)
}

// ribAllOverCDTRows are the rows of the CDT's test in the ordinary run:
// the eight whose width stands square, the only ones whose volume is
// checked against OpenCASCADE's, OpenCASCADE's refusal at h 5 lean -4 r
// 0.5, and a leaning row of each other height, both leans and both
// radii among them.
func ribAllOverCDTRows(o ribOracle) bool {
	return o.lean == 0 || slices.Contains([][3]float64{{5, -4, 0.5}, {10, 2, 1}, {20, -4, 1}, {40, 2, 0.5}}, [3]float64{o.h, o.lean, o.r})
}

// ribAllOverEarClipRows are the rows of the ear clipper's test in the
// ordinary run: one of each height at radius 1, two of them square.
func ribAllOverEarClipRows(o ribOracle) bool {
	return slices.Contains([][3]float64{{5, 0, 1}, {10, 2, 1}, {20, -4, 1}, {40, 0, 1}}, [3]float64{o.h, o.lean, o.r})
}

// ribAllOverRows runs ribAllOver over the oracle's rows that keep says
// to, every row under TELESAPR_SWEEP=1 (sweep.Full). Under -short and the
// race detector only h 10 lean 2 r 1: each row takes 10 to 40 s, the 24
// took half of the package's time under the race detector, and h 5 lean
// -4 r 0.5 alone eleven minutes beside the rest of the stage
// (2026-11-04).
func ribAllOverRows(t *testing.T, mesher srf.Mesher, keep func(ribOracle) bool) {
	t.Helper()
	for _, o := range readRibOracle(t) {
		if testing.Short() || fixtures.RaceDetector {
			if o.h != 10 || o.lean != 2 || o.r != 1 {
				continue
			}
		} else if !sweep.Full() && !keep(o) {
			continue
		}
		name := fmt.Sprintf("h%g_lean%g_r%g", o.h, o.lean, o.r)
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ribAllOver(t, o, mesher)
		})
	}
}

// ribAllOver rounds the rib of o all over at o's radius, its curved faces
// meshed by mesher, and checks it against o and the balls' envelope.
func ribAllOver(t *testing.T, o ribOracle, mesher srf.Mesher) {
	t.Helper()
	rb := rib{o.h, o.lean}
	sides, ends := rb.foot()
	tol := srf.Tol(0.0003*o.r, 400).WithMesher(mesher)
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
	// The top alone, meshed once: its volume, where the width stands
	// square, and the triangles the balls roll against.
	var top srf.SMesh
	topOnly.TriangulateInto(&top)
	if o.lean == 0 && !o.refused {
		added, want := rep.Volume-top.CalculateVolume(), o.both-o.first
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
	g := ballGrid(&top, o.r)
	for _, q := range ribBallPoints(rb, o.r) {
		if !math.IsInf(firstHeight(&top, q), 1) {
			// Under the rib.
			continue
		}
		want := ballHeight(g, q, o.r)
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
// whose triangles near the plate are g (ballGrid) stands over q, on the
// plate: the lowest point over q that a ball on the plate, clear of the
// solid, reaches. The ball's centre nearest q sideways is found along
// directions round it, as far as the ball is clear (closer, it would cut
// into the solid).
func ballHeight(g *triGrid, q geom.Vector, r float64) float64 {
	free := func(a, d float64) bool {
		return !g.within(v(q.X+d*math.Cos(a), q.Y+d*math.Sin(a), r), r-1e-9)
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

// ballGrid is the triangles of m that a ball of radius r resting on the
// plate can touch: those above the plate and below 3r, in cells of r/2.
func ballGrid(m *srf.SMesh, r float64) *triGrid {
	var near []srf.STriangle
	for _, tr := range m.Triangles {
		hi, lo := tr.A, tr.A
		tr.B.MakeMaxMin(&hi, &lo)
		tr.C.MakeMaxMin(&hi, &lo)
		if hi.Z > 1e-9 && lo.Z < 3*r {
			near = append(near, tr)
		}
	}
	return newTriGrid(near, r/2, 3*r)
}

// triGrid sorts triangles into cubic cells by their boxes, so that what
// lies near a point is found without walking them all: ballHeight asks
// thousands of times per point whether a ball is clear, and the CDT's
// mesh of the rib has some 90 000 triangles.
type triGrid struct {
	tris  []srf.STriangle
	boxes [][2]geom.Vector
	cell  float64
	cells map[[3]int][]int32
	seen  []int32
	stamp int32
}

// newTriGrid sorts tris into cells of size cell; below zTop, as no
// point higher is asked about.
func newTriGrid(tris []srf.STriangle, cell, zTop float64) *triGrid {
	g := &triGrid{tris: tris, cell: cell, cells: map[[3]int][]int32{}, seen: make([]int32, len(tris))}
	for i, tr := range tris {
		hi, lo := tr.A, tr.A
		tr.B.MakeMaxMin(&hi, &lo)
		tr.C.MakeMaxMin(&hi, &lo)
		g.boxes = append(g.boxes, [2]geom.Vector{lo, hi})
		a, b := g.key(lo), g.key(hi)
		b[2] = min(b[2], g.key(v(0, 0, zTop))[2])
		for x := a[0]; x <= b[0]; x++ {
			for y := a[1]; y <= b[1]; y++ {
				for z := a[2]; z <= b[2]; z++ {
					k := [3]int{x, y, z}
					g.cells[k] = append(g.cells[k], int32(i))
				}
			}
		}
	}
	return g
}

func (g *triGrid) key(p geom.Vector) [3]int {
	return [3]int{int(math.Floor(p.X / g.cell)), int(math.Floor(p.Y / g.cell)), int(math.Floor(p.Z / g.cell))}
}

// within reports whether a triangle comes closer to c than d.
func (g *triGrid) within(c geom.Vector, d float64) bool {
	g.stamp++
	a, b := g.key(c.Minus(v(d, d, d))), g.key(c.Plus(v(d, d, d)))
	for x := a[0]; x <= b[0]; x++ {
		for y := a[1]; y <= b[1]; y++ {
			for z := a[2]; z <= b[2]; z++ {
				for _, i := range g.cells[[3]int{x, y, z}] {
					if g.seen[i] == g.stamp {
						continue
					}
					g.seen[i] = g.stamp
					lo, hi := g.boxes[i][0], g.boxes[i][1]
					dx := math.Max(0, math.Max(lo.X-c.X, c.X-hi.X))
					dy := math.Max(0, math.Max(lo.Y-c.Y, c.Y-hi.Y))
					dz := math.Max(0, math.Max(lo.Z-c.Z, c.Z-hi.Z))
					if dx*dx+dy*dy+dz*dz >= d*d {
						continue
					}
					if triangleDistance(&g.tris[i], c) < d {
						return true
					}
				}
			}
		}
	}
	return false
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
	if fixtures.RaceDetector {
		// 6 minutes there for one chain at 0.0003, which no coarser
		// tolerance makes; the chains' fill runs on the same goroutines
		// in TestFilletChainsMitred and the rest under the race detector.
		t.Skip("a rib's foot filled at a fine tolerance, too slow under the race detector")
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
	// The two fills side by side, each on a copy made here, before
	// either starts: one after the other they were pkg/topo's longest
	// wait, 63 s.
	for _, c := range []struct {
		name  string
		picks []topo.FilletPick
	}{{"all", loop}, {"one", loop[1:2]}} {
		var sh srf.SShell
		sh.MakeFromCopyOf(before)
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if err := topo.FilletPicks(&sh, c.picks, 1, 1<<29, false); err != nil {
				t.Fatal(err)
			}
			var m srf.SMesh
			sh.TriangulateInto(&m)
			if rep := check.Solid(&sh, &m); !rep.Sound() {
				t.Errorf("unsound: %s", rep.String())
			}
		})
	}
}
