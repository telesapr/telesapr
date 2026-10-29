package topo_test

import (
	"math"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/topo"
)

// block is the box [-20, 20]² × [0, h] at tol.
func block(h float64, tol srf.Tolerance) *srf.SShell {
	sh := &srf.SShell{Tol: tol}
	sh.MakePrism([]geom.Vector{v(-20, -20, 0), v(20, -20, 0), v(20, 20, 0), v(-20, 20, 0)}, v(0, 0, 0), v(0, 0, h),
		core.NewRGBA(100, 100, 100, 255))
	return sh
}

// A block with a round pocket cut into its top, hollowed open at its
// top or its bottom: the inside is the block's moved in less the pocket's
// cutter grown by the thickness, so a wall of the thickness stands round
// the pocket and under its floor.
func TestShellJoinedPocket(t *testing.T) {
	t.Parallel()
	tol := srf.Tol(0.001, 20)
	body := block(20, tol)
	cutter := &srf.SShell{Tol: tol}
	cutter.MakeFromTransformationOf(cylinder(5, 15, tol), v(0, 0, 10), geom.QuaternionIdentity, 1)
	sh := &srf.SShell{Tol: tol}
	sh.MakeFromDifferenceOf(body, cutter)
	parts := []topo.ShellPart{{Shell: body}, {Shell: cutter, Combine: srf.SurfaceDiff}}
	solid := 32000 - 250*math.Pi
	for _, c := range []struct {
		name string
		at   geom.Vector
		want float64
	}{
		// Open at the top, the inside runs from 1 to 20, round a cutter
		// grown to a radius of 6 from 9 up.
		{"top", v(15, 15, 20), solid - (38*38*19 - 36*math.Pi*11)},
		// Open at the bottom, from 0 to 19, the grown cutter from 9 to 19.
		{"bottom", v(15, 15, 0), solid - (38*38*19 - 36*math.Pi*10)},
		// Open at the pocket's floor, the cutter's floor is moved up into
		// the pocket, and the inside, from 1 to 19, reaches through it
		// round a ring of the thickness.
		{"floor", v(0, 0, 10), solid - (38*38*18 - 36*math.Pi*8 - 25*math.Pi)},
	} {
		out, err := topo.ShellJoined(sh, parts, facesAt(t, sh, c.at), 1, 1000)
		if err != nil {
			t.Errorf("open at the %s: %v", c.name, err)
			continue
		}
		if r := soundness(out); !r.Sound() || math.Abs(r.Volume-c.want) > 0.002*c.want {
			t.Errorf("open at the %s: %s, want %g", c.name, r.String(), c.want)
		}
	}
}

// A ball with its bottom cut flat by intersecting it with a box, hollowed
// open at the flat: the inside is the ball moved in, cut by the same
// plane.
func TestShellJoinedIntersection(t *testing.T) {
	t.Parallel()
	tol := srf.Tol(0.001, 20)
	ball, rev := turned(meridian(t, [][2]float64{{0, -10}, {10, 0}, {0, 10}},
		map[int][2]float64{0: {0, 0}, 1: {0, 0}}, tol), 0, 2*math.Pi, tol)
	box := &srf.SShell{Tol: tol}
	box.MakePrism([]geom.Vector{v(-20, -20, -5), v(20, -20, -5), v(20, 20, -5), v(-20, 20, -5)}, v(0, 0, 0), v(0, 0, 25),
		core.NewRGBA(100, 100, 100, 255))
	sh := &srf.SShell{Tol: tol}
	sh.MakeFromIntersectionOf(ball, box)
	parts := []topo.ShellPart{{Rev: &rev}, {Shell: box, Combine: srf.SurfaceInter}}
	out, err := topo.ShellJoined(sh, parts, facesAt(t, sh, v(0, 0, -5)), 1, 1000)
	if err != nil {
		t.Fatal(err)
	}
	// The ball of 10 above -5 less the ball of 9 above -5.
	want := 1125*math.Pi - 2548*math.Pi/3
	if r := soundness(out); !r.Sound() || math.Abs(r.Volume-want) > 0.002*want {
		t.Fatalf("a ball cut flat: %s, want %g", r.String(), want)
	}
}

// A partial revolve taken away is refused by name, and so is a first part
// that is not a union.
func TestShellJoinedDiffRefusals(t *testing.T) {
	t.Parallel()
	tol := srf.Tol(0.001, 20)
	body := block(20, tol)
	_, rev := turned(meridian(t, [][2]float64{{0, 15}, {5, 15}, {5, 25}, {0, 25}}, nil, tol), 0, math.Pi, tol)
	cut, _ := turned(rev.Profile, 0, math.Pi, tol)
	sh := &srf.SShell{Tol: tol}
	sh.MakeFromDifferenceOf(body, cut)
	open := facesAt(t, sh, v(15, 15, 0))
	if _, err := topo.ShellJoined(sh, []topo.ShellPart{{Shell: body}, {Rev: &rev, Combine: srf.SurfaceDiff}}, open, 1, 1000); err == nil {
		t.Error("a half turn taken away: no error")
	}
	if _, err := topo.ShellJoined(sh, []topo.ShellPart{{Shell: body, Combine: srf.SurfaceDiff}, {Shell: cut}}, open, 1, 1000); err == nil {
		t.Error("a first part taken away: no error")
	}
	// A pocket turned into the block's top, open at its floor: the floor
	// lies on the lathe taken away.
	_, pocket := turned(meridian(t, [][2]float64{{0, 10}, {5, 10}, {5, 25}, {0, 25}}, nil, tol), 0, 2*math.Pi, tol)
	lathed, _ := turned(pocket.Profile, 0, 2*math.Pi, tol)
	sh = &srf.SShell{Tol: tol}
	sh.MakeFromDifferenceOf(body, lathed)
	_, err := topo.ShellJoined(sh, []topo.ShellPart{{Shell: body}, {Rev: &pocket, Combine: srf.SurfaceDiff}},
		facesAt(t, sh, v(1, 1, 10)), 1, 1000)
	if err == nil || !strings.Contains(err.Error(), "lies on a lathe or revolve taken away") {
		t.Errorf("open at a lathed pocket's floor: %v", err)
	}
	// Open at the block's top, it is made.
	if _, err := topo.ShellJoined(sh, []topo.ShellPart{{Shell: body}, {Rev: &pocket, Combine: srf.SurfaceDiff}},
		facesAt(t, sh, v(15, 15, 20)), 1, 1000); err != nil {
		t.Errorf("a lathed pocket open at the top: %v", err)
	}
}

// A round post half sunk into the side of a block, as high as it: half
// its side lies inside the block and half is a wall, and a prism's sides
// all move alike, so the shell is refused, naming the side.
func TestShellJoinedHalfBuriedSide(t *testing.T) {
	t.Parallel()
	tol := srf.Tol(0.001, 20)
	body := &srf.SShell{Tol: tol}
	body.MakePrism([]geom.Vector{v(-20, -20, 0), v(0, -20, 0), v(0, 20, 0), v(-20, 20, 0)}, v(0, 0, 0), v(0, 0, 10),
		core.NewRGBA(100, 100, 100, 255))
	post := cylinder(3, 10, tol)
	sh := &srf.SShell{Tol: tol}
	sh.MakeFromUnionOf(body, post)
	_, err := topo.ShellJoined(sh, []topo.ShellPart{{Shell: body}, {Shell: post}}, facesAt(t, sh, v(-10, 0, 10)), 1, 1000)
	if err == nil || !strings.Contains(err.Error(), "a side of a prism lies inside") {
		t.Errorf("a post half sunk: %v", err)
	}
}
