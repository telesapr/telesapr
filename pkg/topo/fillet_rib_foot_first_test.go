package topo_test

import (
	"fmt"
	"math"
	"slices"
	"testing"

	"github.com/telesapr/telesapr/pkg/check"
	"github.com/telesapr/telesapr/pkg/internal/fixtures"
	"github.com/telesapr/telesapr/pkg/internal/sweep"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/topo"
)

// ribFootFilled is the rib rb at tol with its foot filled at radius r,
// in an edit of its own.
func ribFootFilled(t *testing.T, rb rib, r float64, tol srf.Tolerance) *srf.SShell {
	t.Helper()
	sh := rb.shell(tol)
	sides, ends := rb.foot()
	if err := topo.FilletEdges(sh, append(sides, ends...), r, 1<<30); err != nil {
		t.Fatal(err)
	}
	return sh
}

// ribTopAfterFoot are picks of the rib's top's five edges on a shell
// whose foot is filled, which shortened the uprights and the slope's
// edges to where the foot's fillets reach up them: each edge whose
// points lie on one of the top's lines, in the order of rib.top.
func ribTopAfterFoot(t *testing.T, sh *srf.SShell, rb rib) []topo.FilletPick {
	t.Helper()
	tp, err := topo.Build(sh)
	if err != nil {
		t.Fatal(err)
	}
	var out []topo.FilletPick
	for _, seg := range rb.top() {
		d := seg[1].Minus(seg[0])
		n := len(out)
		for e := range tp.Edges {
			pts := tp.Edges[e].Points
			on := len(pts) >= 2
			for _, p := range pts {
				s := p.Minus(seg[0]).Dot(d) / d.Dot(d)
				on = on && p.Minus(seg[0].Plus(d.ScaledBy(s))).Magnitude() < 1e-6 && s > -1e-6 && s < 1+1e-6
			}
			if on {
				out = append(out, topo.FilletPick{A: pts[0], B: pts[len(pts)-1]})
			}
		}
		if len(out) != n+1 {
			t.Fatalf("%d edges along the top's line from %v, want 1", len(out)-n, seg[0])
		}
	}
	return out
}

// TestFilletRibFootFirstOracle fills a rib's foot, then rounds its top's
// five edges in an edit of their own. The uprights and the slope's edges
// end on the foot's fillets, where these meet in a mitre below them or
// where the side's fillet runs on past the slope; each such corner is set
// back as the one edit of all nine edges sets back a corner of convex and
// concave fillets (footCornerAt), and comes out the same solid. Against
// OpenCASCADE, made the same way (scripts/fillet-rib-foot-first-fixtures.py),
// the top's fillets take more: OpenCASCADE's four-sided patches at the
// four corners lie nearer the corner than the setback's, by 1.4 r³ in all
// at h 20 to 4.8 r³ at h 5, whose slope meets the plate at 20 degrees and
// whose front corners are set back the furthest. The ordinary run takes
// 6 of the 18 rows (ribFootFirstRows), TELESAPR_SWEEP=1 all; heavy (HEAVY
// in scripts/go-test.sh): the 18 take some 9 minutes of processor.
func TestFilletRibFootFirstOracle(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("rounds 18 ribs' tops after their feet, some 15 s each")
	}
	for _, o := range readRibOracleFile(t, "../../testdata/topo/fillet-rib-foot-first.txt") {
		if fixtures.RaceDetector && (o.h != 10 || o.lean != 2 || o.r != 1) {
			// One row under the race detector: the rest round on the
			// same goroutines.
			continue
		}
		if !sweep.Full() && !ribFootFirstRows(o) {
			continue
		}
		t.Run(fmt.Sprintf("h%g_lean%g_r%g", o.h, o.lean, o.r), func(t *testing.T) {
			t.Parallel()
			if o.refused {
				t.Fatal("OpenCASCADE made none")
			}
			rb := rib{o.h, o.lean}
			tol := srf.Tol(0.0003*o.r, 400)
			sh := ribFootFilled(t, rb, o.r, tol)
			foot := meshVolume(sh)
			// The foot's fillets' chords lose less than the tolerance
			// over them, some 50 round a rib of 14 by 10, and the top's
			// as much where they cut them.
			allow := 2*0.0003*o.r*math.Pi/2*o.r*(2*(ribFoot+ribWide)+8*o.r) + 0.002
			if math.Abs(foot-o.first) > allow {
				t.Errorf("the foot filled is %.6f, OpenCASCADE's %.6f", foot, o.first)
			}
			if err := topo.FilletPicks(sh, ribTopAfterFoot(t, sh, rb), o.r, 1<<29, false); err != nil {
				t.Fatal(err)
			}
			both := soundRib(t, sh)
			gap := (foot - both - (o.first - o.both)) / (o.r * o.r * o.r)
			if gap < 0 || gap > 5 {
				t.Errorf("the top's fillets take %.4f, OpenCASCADE's %.4f: %.3g r³ more, want 0 to 5",
					foot-both, o.first-o.both, gap)
			}
			// The same nine edges in one edit, a setback at each corner of
			// both kinds.
			one := rb.shell(tol)
			var picks []topo.FilletPick
			sides, ends := rb.foot()
			for _, s := range append(append(sides, ends...), rb.top()...) {
				picks = append(picks, topo.FilletPick{A: s[0], B: s[1]})
			}
			if err := topo.FilletPicksSetbackMixed(one, picks, o.r, 1<<30); err != nil {
				t.Fatal(err)
			}
			if v := soundRib(t, one); math.Abs(v-both) > allow {
				t.Errorf("foot first %.6f, the one edit's setbacks %.6f", both, v)
			}
		})
	}
}

// ribFootFirstRows are the rows of TestFilletRibFootFirstOracle in the
// ordinary run: each height, each lean and each radius twice or more, the
// race detector's row among them.
func ribFootFirstRows(o ribOracle) bool {
	return slices.Contains([][3]float64{{5, -4, 0.5}, {5, 2, 1}, {10, 0, 1}, {10, 2, 1}, {20, -4, 1}, {20, 0, 0.5}},
		[3]float64{o.h, o.lean, o.r})
}

// soundRib is the volume of sh, which must be sound and its topology
// whole.
func soundRib(t *testing.T, sh *srf.SShell) float64 {
	t.Helper()
	var m srf.SMesh
	sh.TriangulateInto(&m)
	rep := check.Solid(sh, &m)
	if !rep.Sound() {
		t.Fatalf("unsound: %s", rep.String())
	}
	tp, err := topo.Build(sh)
	if err != nil {
		t.Fatal(err)
	}
	if p := tp.Validate(); len(p) > 0 {
		t.Fatalf("topology: %s", p[0].Msg)
	}
	return rep.Volume
}

// TestFilletRibFootFirstEach rounds each of a rib's top's edges alone
// after its foot was filled, and the uprights at a radius unlike the
// foot's: the corner on the foot is set back whichever edge reaches it
// and whatever the two radii.
func TestFilletRibFootFirstEach(t *testing.T) {
	t.Parallel()
	rb := rib{10, 2}
	tol := srf.Tol(0.002, 400)
	cases := []struct {
		name    string
		foot, r float64
		edges   []int
	}{
		{"slope", 1, 1, []int{0}},
		{"upright", 1, 1, []int{3}},
		{"uprights 0.5 on 1", 1, 0.5, []int{3, 4}},
		{"uprights 1.5 on 0.5", 0.5, 1.5, []int{3, 4}},
	}
	if fixtures.RaceDetector {
		// An upright and a slope's edge under the race detector: the
		// rest set back the same corners.
		cases = cases[:2]
	} else if !testing.Short() {
		cases = append(cases, struct {
			name    string
			foot, r float64
			edges   []int
		}{"slope's other", 1, 1, []int{1}}, struct {
			name    string
			foot, r float64
			edges   []int
		}{"upright's other", 1, 1, []int{4}})
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			sh := ribFootFilled(t, rb, c.foot, tol)
			before := meshVolume(sh)
			top := ribTopAfterFoot(t, sh, rb)
			var picks []topo.FilletPick
			for _, i := range c.edges {
				picks = append(picks, top[i])
			}
			if err := topo.FilletPicks(sh, picks, c.r, 1<<29, false); err != nil {
				t.Fatal(err)
			}
			if v := soundRib(t, sh); !(v < before) {
				t.Errorf("volume %.4f, not less than the foot's %.4f", v, before)
			}
		})
	}
}
