package topo_test

import (
	"math"
	"slices"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/topo"
)

// cubicPrism is the prism of height 5 on the outline of cubics, each
// given by its four control points in the plane z = 0, turning
// anticlockwise; consecutive cubics meet at sharp corners. It is
// extruded at tol.
func cubicPrism(cubics [][4]geom.Vector, tol srf.Tolerance) *srf.SShell {
	var ls srf.SBezierLoopSet
	for _, c := range cubics {
		ls.Outer.Curves = append(ls.Outer.Curves, srf.SBezierFrom4(c[0], c[1], c[2], c[3]))
	}
	ls.Normal = v(0, 0, -1)
	sh := srf.SShell{Tol: tol}
	sh.MakeFromExtrusionOf(&ls, v(0, 0, 0), v(0, 0, 5), core.NewRGBA(100, 100, 100, 255))
	return &sh
}

// lensCubics is a lens of two cubics meeting at sharp corners at (0, 0)
// and (20, 0), symmetric about the x axis.
func lensCubics() [][4]geom.Vector {
	return [][4]geom.Vector{
		{v(0, 0, 0), v(5, -8, 0), v(15, -8, 0), v(20, 0, 0)},
		{v(20, 0, 0), v(15, 8, 0), v(5, 8, 0), v(0, 0, 0)},
	}
}

// threeCubics is a profile of three cubics meeting at sharp corners at
// (0, 0), (20, 0) and (10, 17).
func threeCubics() [][4]geom.Vector {
	return [][4]geom.Vector{
		{v(0, 0, 0), v(6, -4, 0), v(14, -4, 0), v(20, 0, 0)},
		{v(20, 0, 0), v(18, 6, 0), v(14, 12, 0), v(10, 17, 0)},
		{v(10, 17, 0), v(6, 12, 0), v(2, 6, 0), v(0, 0, 0)},
	}
}

// cubicMiddle is the point of the cubic halfway along, lifted to z.
func cubicMiddle(c [4]geom.Vector, z float64) geom.Vector {
	p := c[0].Plus(c[1].ScaledBy(3)).Plus(c[2].ScaledBy(3)).Plus(c[3]).ScaledBy(1.0 / 8)
	return v(p.X, p.Y, z)
}

// topPicks are the top edges of cubicPrism's solid, one per cubic.
func topPicks(cubics [][4]geom.Vector) []topo.FilletPick {
	var picks []topo.FilletPick
	for _, c := range cubics {
		picks = append(picks, topo.FilletPick{At: cubicMiddle(c, 5), Arc: true})
	}
	return picks
}

// hasVertex reports whether sh's solid has a vertex within 1e-4 of p.
func hasVertex(t *testing.T, sh *srf.SShell, p geom.Vector) bool {
	t.Helper()
	tp := build(t, sh)
	for _, x := range tp.Vertices {
		if x.P.Minus(p).Magnitude() < 1e-4 {
			return true
		}
	}
	return false
}

// A rim that turns sharply at each crease of its wall is a chain of
// edges between each two creases, and the chains, picked together, are
// filleted together, mitred at the creases (rollingchains.go). The
// lens's two top edges, at radius 1: OpenCASCADE's makeFillet of the
// same edges has its mitres from (0, 0, 4) to (1.195537, 0, 5) and from
// (20, 0, 4) to (18.804463, 0, 5), takes 10.146650 mm³ against our
// 10.146885 (its Volume of our STEP export), and every point of our
// fillet lies within 1.2e-4 mm of its (sampled 2026-10-11). A profile
// of three cubics has one chain that fits the mitre at one end and
// copies the other chain's at the other: within 6e-6 of OpenCASCADE's
// volume at radii 0.5 and 1, its points within 2e-4 mm.
func TestFilletChainsMitred(t *testing.T) {
	if testing.Short() {
		t.Skip("rolling-ball fillets of several chains, slow")
	}
	t.Parallel()
	tol := fine(0.002)
	lens := lensCubics()
	sh := cubicPrism(lens, tol)
	if err := topo.FilletPicks(sh, topPicks(lens), 1, 1<<30, false); err != nil {
		t.Fatalf("lens: %v", err)
	}
	sound(t, sh)
	for _, p := range []geom.Vector{v(0, 0, 4), v(1.195537, 0, 5), v(20, 0, 4), v(18.804463, 0, 5)} {
		if !hasVertex(t, sh, p) {
			t.Errorf("lens: no vertex at %v", p)
		}
	}

	three := threeCubics()
	sh = cubicPrism(three, tol)
	if err := topo.FilletPicks(sh, topPicks(three), 0.5, 1<<30, false); err != nil {
		t.Fatalf("three cubics: %v", err)
	}
	sound(t, sh)
	if !hasVertex(t, sh, v(0, 0, 4.5)) {
		t.Errorf("three cubics: no mitre down the crease at the origin")
	}
}

// Picks that don't make chains meeting in mitres are each a rolling
// ball's alone: a chain that meets smaller ones of two radii at its two
// ends, its sections kinked differently at each (rollingkink.go), is
// refused; a top edge with a bottom one lie apart, and are
// rounded one after the other (filletApart): each fillet's vertices
// are those it has alone, and the two take what each takes alone,
// within what the meshes' chords of the cut walls differ by.
func TestFilletChainsNotMitred(t *testing.T) {
	t.Parallel()
	tol := fine(0.002)
	lens := lensCubics()
	three := topPicks(threeCubics())
	three[0].R, three[2].R = 0.5, 0.75
	sh := cubicPrism(threeCubics(), tol)
	err := topo.FilletPicks(sh, three, 1, 1<<30, false)
	if err == nil || !strings.Contains(err.Error(), "pick it alone") {
		t.Errorf("three radii: %v", err)
	}
	two := topPicks(lens)

	bottom := []topo.FilletPick{two[0], {At: cubicMiddle(lens[1], 0), Arc: true}}
	before := meshVolume(cubicPrism(lens, tol))
	took := 0.0
	var made []geom.Vector
	orig := build(t, cubicPrism(lens, tol)).Vertices
	for _, p := range bottom {
		sh := cubicPrism(lens, tol)
		if err := topo.FilletPicks(sh, []topo.FilletPick{p}, 1, 1<<30, false); err != nil {
			t.Fatalf("alone at %v: %v", p.At, err)
		}
		took += before - sound(t, sh)
		for _, x := range build(t, sh).Vertices {
			if !slices.ContainsFunc(orig, func(o topo.Vertex) bool { return o.P.Minus(x.P).Magnitude() < 1e-4 }) {
				made = append(made, x.P)
			}
		}
	}
	sh = cubicPrism(lens, tol)
	if err := topo.FilletPicks(sh, bottom, 1, 1<<30, false); err != nil {
		t.Fatalf("top and bottom: %v", err)
	}
	if got := before - sound(t, sh); math.Abs(got-took) > 1e-3*took {
		t.Errorf("top and bottom took %.9g, %.9g alone", got, took)
	}
	if len(made) == 0 {
		t.Error("the fillets alone made no vertices")
	}
	got := build(t, sh).Vertices
	for _, p := range made {
		if !slices.ContainsFunc(got, func(o topo.Vertex) bool { return o.P.Minus(p).Magnitude() < 1e-4 }) {
			t.Errorf("top and bottom: no vertex at %v, which the fillet alone has", p)
		}
	}
}

// fourCubics is a profile of four cubics bulging out between sharp
// corners at (0, 0), (16, 0), (16, 13) and (0, 13).
func fourCubics() [][4]geom.Vector {
	return [][4]geom.Vector{
		{v(0, 0, 0), v(5, -3, 0), v(11, -3, 0), v(16, 0, 0)},
		{v(16, 0, 0), v(19, 4, 0), v(19, 9, 0), v(16, 13, 0)},
		{v(16, 13, 0), v(11, 16, 0), v(5, 16, 0), v(0, 13, 0)},
		{v(0, 13, 0), v(-3, 9, 0), v(-3, 4, 0), v(0, 0, 0)},
	}
}

// Some of a rim's chains picked: each is mitred where it meets another
// picked one and ends on the wall of an unpicked one at its other end,
// fitted there to its cuts along the curved edge as a lone chain is
// (rollingBallFilletsAt, partial). Two and three of the four cubics'
// top edges at radius 1, refused as "can't be fitted" before
// 2026-10-28: every vertex OpenCASCADE's makeFillet of the same edges
// makes is one of ours, within 7e-6 mm, its Volume of our STEP export
// within 3e-5 of its own, and every point of either solid within
// 5.4e-4 mm of the other (sampled 2026-10-28).
func TestFilletChainsPartlyMitred(t *testing.T) {
	if testing.Short() {
		t.Skip("rolling-ball fillets of several chains, slow")
	}
	t.Parallel()
	four := fourCubics()
	all := topPicks(four)
	for _, c := range []struct {
		picks []int
		verts []geom.Vector
	}{
		{[]int{0, 1}, []geom.Vector{v(0, 0, 4), v(16, 0, 4), v(16, 13, 4),
			v(-1.259445, 2.094329, 5), v(15.315749, 0.757104, 5), v(13.529656, 14.210272, 5)}},
		{[]int{1, 2, 3}, []geom.Vector{v(0, 0, 4), v(16, 0, 4), v(0, 13, 4), v(16, 13, 4),
			v(2.470344, -1.210272, 5), v(13.529656, -1.210272, 5), v(0.684251, 12.242896, 5), v(15.315749, 12.242896, 5)}},
	} {
		var picks []topo.FilletPick
		for _, i := range c.picks {
			picks = append(picks, all[i])
		}
		sh := cubicPrism(four, fine(0.002))
		if err := topo.FilletPicks(sh, picks, 1, 1<<30, false); err != nil {
			t.Fatalf("%v: %v", c.picks, err)
		}
		sound(t, sh)
		for _, p := range c.verts {
			if !hasVertex(t, sh, p) {
				t.Errorf("%v: no vertex at %v", c.picks, p)
			}
		}
	}
}

// Chamfers of a rim's chains meet in mitres as fillets do, where the
// surfaces they sweep, carried on, meet: a point's height over the
// mate's is over its line in the section through the point, and the
// mitre's foot on the face both share is a root there too, where the
// lines cross it. The lens's two top edges at distance 1, and three of
// the four cubics' (two mitres, two ends on the unpicked one's wall):
// OpenCASCADE's makeChamfer of the same edges has every vertex of ours
// at the mitres within 4e-6 mm, takes 23.279179 mm³ from the lens
// against our 23.279349 (its Volume of our STEP export), and every point
// of either solid lies within 2.4e-4 mm of the other (sampled
// 2026-10-28). Its ends on the wall are 4.5e-4 mm from ours, where the
// faces are carried on 2.4 mm past the edge's end.
func TestChamferChainsMitred(t *testing.T) {
	if testing.Short() {
		t.Skip("rolling-ball chamfers of several chains, slow")
	}
	t.Parallel()
	for _, c := range []struct {
		name  string
		prof  [][4]geom.Vector
		picks []int
		verts []geom.Vector
		ends  []geom.Vector
	}{
		{"lens", lensCubics(), []int{0, 1},
			[]geom.Vector{v(0, 0, 4), v(20, 0, 4), v(1.19554, 0, 5), v(18.80446, 0, 5)}, nil},
		{"four", fourCubics(), []int{1, 2, 3},
			[]geom.Vector{v(0, 0, 4), v(16, 0, 4), v(0, 13, 4), v(16, 13, 4), v(0.68425, 12.24290, 5), v(15.31575, 12.24290, 5)},
			[]geom.Vector{v(2.46653, -1.20879, 5), v(13.53347, -1.20879, 5)}},
	} {
		all := topPicks(c.prof)
		var picks []topo.FilletPick
		for _, i := range c.picks {
			picks = append(picks, all[i])
		}
		sh := cubicPrism(c.prof, fine(0.002))
		if err := topo.FilletPicks(sh, picks, 1, 1<<30, true); err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		sound(t, sh)
		got := build(t, sh).Vertices
		for _, p := range c.verts {
			if !hasVertex(t, sh, p) {
				t.Errorf("%s: no vertex at %v", c.name, p)
			}
		}
		for _, p := range c.ends {
			if !slices.ContainsFunc(got, func(x topo.Vertex) bool { return x.P.Minus(p).Magnitude() < 1e-3 }) {
				t.Errorf("%s: no vertex near %v", c.name, p)
			}
		}
	}
}

// Chains of two radii meet in mitres too (rollingkink.go): the smaller
// end is a mitre against the larger's swept surface, ending on its own
// wall at its depth, and the larger end's mitre turns there onto that
// wall, down to the crease at its depth. The lens's top edges at 1 and
// 0.5, and the four cubics' at 1, 0.5, 1 and 0.5: every vertex of
// OpenCASCADE's makeFillet of the same edges at those radii is one of
// ours within 6.4e-6 mm, its Volume of our STEP export within 3.1e-5 of
// its own, and every point of either solid lies within 2e-4 mm of the
// other (sampled 2026-10-28).
func TestFilletChainsRadii(t *testing.T) {
	if testing.Short() {
		t.Skip("rolling-ball fillets of several chains, slow")
	}
	t.Parallel()
	for _, c := range []struct {
		name  string
		prof  [][4]geom.Vector
		radii []float64
		verts []geom.Vector
	}{
		{"lens", lensCubics(), []float64{1, 0.5}, []geom.Vector{v(0, 0, 4), v(20, 0, 4),
			v(19.920511, 0.125852, 4.5), v(0.079489, 0.125852, 4.5), v(0.899091, 0.448841, 5), v(19.100909, 0.448841, 5)}},
		{"four", fourCubics(), []float64{1, 0.5, 1, 0.5}, []geom.Vector{v(0, 0, 4), v(16, 0, 4), v(0, 13, 4), v(16, 13, 4),
			v(-0.206827, 12.715949, 4.5), v(-0.206827, 0.284051, 4.5), v(16.206827, 0.284051, 4.5), v(16.206827, 12.715949, 4.5),
			v(-0.334764, 1.405090, 5), v(16.334764, 1.405090, 5), v(16.334764, 11.594910, 5), v(-0.334764, 11.594910, 5)}},
	} {
		picks := topPicks(c.prof)
		for i := range picks {
			picks[i].R = c.radii[i]
		}
		for _, tol := range []float64{0.01, 0.002} {
			sh := cubicPrism(c.prof, fine(tol))
			if err := topo.FilletPicks(sh, picks, c.radii[0], 1<<30, false); err != nil {
				t.Fatalf("%s at %g: %v", c.name, tol, err)
			}
			sound(t, sh)
			for _, p := range c.verts {
				if !hasVertex(t, sh, p) {
					t.Errorf("%s at %g: no vertex at %v", c.name, tol, p)
				}
			}
		}
	}
}
