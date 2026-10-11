package topo_test

import (
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/topo"
)

// cubicPrism is the prism of height 5 on the outline of cubics, each
// given by its four control points in the plane z = 0, turning
// anticlockwise; consecutive cubics meet at sharp corners.
func cubicPrism(cubics [][4]geom.Vector) *srf.SShell {
	var ls srf.SBezierLoopSet
	for _, c := range cubics {
		ls.Outer.Curves = append(ls.Outer.Curves, srf.SBezierFrom4(c[0], c[1], c[2], c[3]))
	}
	ls.Normal = v(0, 0, -1)
	var sh srf.SShell
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
	fine(t, 0.002)
	lens := lensCubics()
	sh := cubicPrism(lens)
	if err := topo.FilletPicks(sh, topPicks(lens), 1, 1<<30, false); err != nil {
		t.Fatalf("lens: %v", err)
	}
	sound(t, sh)
	for _, p := range []geom.Vector{v(0, 0, 4), v(1.195537, 0, 5), v(20, 0, 4), v(18.804463, 0, 5)} {
		if !hasVertex(t, sh, p) {
			t.Errorf("lens: no vertex at %v", p)
		}
	}

	three := [][4]geom.Vector{
		{v(0, 0, 0), v(6, -4, 0), v(14, -4, 0), v(20, 0, 0)},
		{v(20, 0, 0), v(18, 6, 0), v(14, 12, 0), v(10, 17, 0)},
		{v(10, 17, 0), v(6, 12, 0), v(2, 6, 0), v(0, 0, 0)},
	}
	sh = cubicPrism(three)
	if err := topo.FilletPicks(sh, topPicks(three), 0.5, 1<<30, false); err != nil {
		t.Fatalf("three cubics: %v", err)
	}
	sound(t, sh)
	if !hasVertex(t, sh, v(0, 0, 4.5)) {
		t.Errorf("three cubics: no mitre down the crease at the origin")
	}
}

// Picks that don't make chains meeting in mitres are each a rolling
// ball's alone, as before: the lens's top edges at two radii, and a top
// edge with a bottom one. Chamfers that would meet in mitres are
// refused, saying so.
func TestFilletChainsNotMitred(t *testing.T) {
	fine(t, 0.002)
	lens := lensCubics()
	two := topPicks(lens)
	two[1].R = 0.5
	bottom := []topo.FilletPick{two[0], {At: cubicMiddle(lens[1], 0), Arc: true}}
	for name, picks := range map[string][]topo.FilletPick{"two radii": two, "top and bottom": bottom} {
		sh := cubicPrism(lens)
		err := topo.FilletPicks(sh, picks, 1, 1<<30, false)
		if err == nil || !strings.Contains(err.Error(), "pick it alone") {
			t.Errorf("%s: %v", name, err)
		}
	}
	// Chamfers: a mitre is found from the ball's centres.
	sh := cubicPrism(lens)
	err := topo.FilletPicks(sh, topPicks(lens), 1, 1<<30, true)
	if err == nil || !strings.Contains(err.Error(), "are not mitred") {
		t.Errorf("chamfers: %v", err)
	}
}
