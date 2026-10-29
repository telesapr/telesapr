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

// A first part that is not a union is refused.
func TestShellJoinedDiffRefusals(t *testing.T) {
	t.Parallel()
	tol := srf.Tol(0.001, 20)
	body := block(20, tol)
	_, rev := turned(meridian(t, [][2]float64{{0, 15}, {5, 15}, {5, 25}, {0, 25}}, nil, tol), 0, math.Pi, tol)
	cut, _ := turned(rev.Profile, 0, math.Pi, tol)
	sh := &srf.SShell{Tol: tol}
	sh.MakeFromDifferenceOf(body, cut)
	open := facesAt(t, sh, v(15, 15, 0))
	if _, err := topo.ShellJoined(sh, []topo.ShellPart{{Shell: body, Combine: srf.SurfaceDiff}, {Shell: cut}}, open, 1, 1000); err == nil {
		t.Error("a first part taken away: no error")
	}
}

// A pocket turned into a block's top, open at its floor, which lies on
// the lathe taken away: the lathe's floor is moved up into the pocket,
// and the inside reaches through it round a ring of the thickness, as a
// pocket extruded does (TestShellJoinedPocket). Open at the block's top
// it is made as before.
func TestShellJoinedLathedPocketFloor(t *testing.T) {
	t.Parallel()
	tol := srf.Tol(0.001, 20)
	body := block(20, tol)
	_, pocket := turned(meridian(t, [][2]float64{{0, 10}, {5, 10}, {5, 25}, {0, 25}}, nil, tol), 0, 2*math.Pi, tol)
	lathed, _ := turned(pocket.Profile, 0, 2*math.Pi, tol)
	sh := &srf.SShell{Tol: tol}
	sh.MakeFromDifferenceOf(body, lathed)
	parts := []topo.ShellPart{{Shell: body}, {Rev: &pocket, Combine: srf.SurfaceDiff}}
	solid := 32000 - 250*math.Pi
	for _, c := range []struct {
		name string
		at   geom.Vector
		want float64
	}{
		{"floor", v(1, 1, 10), solid - (38*38*18 - 36*math.Pi*8 - 25*math.Pi)},
		{"top", v(15, 15, 20), solid - (38*38*19 - 36*math.Pi*11)},
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

// A cylinder turned from a lathe, intersected with a block larger than
// it, open at its top, which lies on the lathe: the lathe's top is moved
// out of it, so the inside runs up through the opening.
func TestShellJoinedLatheIntersectedOpen(t *testing.T) {
	t.Parallel()
	tol := srf.Tol(0.001, 20)
	box := &srf.SShell{Tol: tol}
	box.MakePrism([]geom.Vector{v(-20, -20, -5), v(20, -20, -5), v(20, 20, -5), v(-20, 20, -5)}, v(0, 0, 0), v(0, 0, 30),
		core.NewRGBA(100, 100, 100, 255))
	lathe, rev := turned(meridian(t, [][2]float64{{0, 0}, {10, 0}, {10, 20}, {0, 20}}, nil, tol), 0, 2*math.Pi, tol)
	sh := &srf.SShell{Tol: tol}
	sh.MakeFromIntersectionOf(box, lathe)
	parts := []topo.ShellPart{{Shell: box}, {Rev: &rev, Combine: srf.SurfaceInter}}
	out, err := topo.ShellJoined(sh, parts, facesAt(t, sh, v(1, 1, 20)), 1, 1000)
	if err != nil {
		t.Fatal(err)
	}
	want := math.Pi * (100*20 - 81*19)
	if r := soundness(out); !r.Sound() || math.Abs(r.Volume-want) > 0.002*want {
		t.Fatalf("a lathe intersected, open at its top: %s, want %g", r.String(), want)
	}
}

// discArea is the area of the disc of radius r about the origin where in
// holds, by rows.
func discArea(r float64, in func(x, y float64) bool) float64 {
	const n = 2000
	h := 2 * r / n
	a := 0.0
	for i := range n {
		for j := range n {
			x, y := -r+(float64(i)+0.5)*h, -r+(float64(j)+0.5)*h
			if x*x+y*y <= r*r && in(x, y) {
				a += h * h
			}
		}
	}
	return a
}

// Revolves of less than a whole turn cut into a block's top, the block
// open at its bottom: the cutter is grown by the thickness, its round
// side moved out and its ends' planes moved out too, so a wall of the
// thickness stands round the cut. A half turn is grown across its ends'
// plane, a quarter turn past both its ends', and three quarters of a
// turn everywhere but what lies past both.
func TestShellJoinedPartialRevolveTakenAway(t *testing.T) {
	t.Parallel()
	tol := srf.Tol(0.001, 20)
	body := block(20, tol)
	profile := meridian(t, [][2]float64{{0, 15}, {5, 15}, {5, 25}, {0, 25}}, nil, tol)
	for _, c := range []struct {
		name string
		turn float64
		in   func(x, y float64) bool
	}{
		{"half", math.Pi, func(_, y float64) bool { return y >= -1 }},
		{"quarter", math.Pi / 2, func(x, y float64) bool { return x >= -1 && y >= -1 }},
		{"three quarters", 3 * math.Pi / 2, func(x, y float64) bool { return x <= 1 || y >= -1 }},
	} {
		cut, rev := turned(profile, 0, c.turn, tol)
		sh := &srf.SShell{Tol: tol}
		sh.MakeFromDifferenceOf(body, cut)
		parts := []topo.ShellPart{{Shell: body}, {Rev: &rev, Combine: srf.SurfaceDiff}}
		out, err := topo.ShellJoined(sh, parts, facesAt(t, sh, v(15, 15, 0)), 1, 1000)
		if err != nil {
			t.Errorf("%s turn: %v", c.name, err)
			continue
		}
		solid := 32000 - 25*c.turn/2*5
		want := solid - (38*38*19 - 5*discArea(6, c.in))
		if r := soundness(out); !r.Sound() || math.Abs(r.Volume-want) > 0.002*want {
			t.Errorf("%s turn: %s, want %g", c.name, r.String(), want)
		}
	}
}

// A round post half sunk into the side of a block, lower than it, open
// at the block's top: half its side lies inside the block and half is a wall,
// and the two halves run on smoothly into each other, so they can't
// move apart: the side is moved in as a wall all round, and the post's
// inside reaches past the block's side, moved in, into the block's.
func TestShellJoinedHalfBuriedSide(t *testing.T) {
	t.Parallel()
	tol := srf.Tol(0.001, 20)
	body := &srf.SShell{Tol: tol}
	body.MakePrism([]geom.Vector{v(-20, -20, 0), v(0, -20, 0), v(0, 20, 0), v(-20, 20, 0)}, v(0, 0, 0), v(0, 0, 10),
		core.NewRGBA(100, 100, 100, 255))
	post := cylinder(3, 6, tol)
	sh := &srf.SShell{Tol: tol}
	sh.MakeFromUnionOf(body, post)
	out, err := topo.ShellJoined(sh, []topo.ShellPart{{Shell: body}, {Shell: post}}, facesAt(t, sh, v(-10, 0, 10)), 1, 1000)
	if err != nil {
		t.Fatal(err)
	}
	// The block's inside to x = -1, and of the post's, of radius 2 from
	// 1 to 5 high, what lies past that.
	want := 8000 + 27*math.Pi - (9*18*38 + 4*(8*math.Pi/3+math.Sqrt(3)))
	if r := soundness(out); !r.Sound() || math.Abs(r.Volume-want) > 0.002*want {
		t.Fatalf("a post half sunk: %s, want %g", r.String(), want)
	}
}

// A square post lower than a block sunk into its side by less than
// twice the thickness, open at the block's top: the post's side inside
// the block is moved out, into the block, and the sides across the
// block's face, partly walls, in, so the insides meet.
func TestShellJoinedShallowSquarePost(t *testing.T) {
	t.Parallel()
	tol := srf.Tol(0.001, 20)
	body := &srf.SShell{Tol: tol}
	body.MakePrism([]geom.Vector{v(-20, -20, 0), v(0, -20, 0), v(0, 20, 0), v(-20, 20, 0)}, v(0, 0, 0), v(0, 0, 10),
		core.NewRGBA(100, 100, 100, 255))
	post := &srf.SShell{Tol: tol}
	post.MakePrism([]geom.Vector{v(-1.5, -4, 0), v(6, -4, 0), v(6, 4, 0), v(-1.5, 4, 0)}, v(0, 0, 0), v(0, 0, 6),
		core.NewRGBA(100, 100, 100, 255))
	sh := &srf.SShell{Tol: tol}
	sh.MakeFromUnionOf(body, post)
	out, err := topo.ShellJoined(sh, []topo.ShellPart{{Shell: body}, {Shell: post}}, facesAt(t, sh, v(-10, 0, 10)), 1, 1000)
	if err != nil {
		t.Fatal(err)
	}
	// The block's inside to x = -1, the post's from -2.5 to 5, 6 wide,
	// 1 to 5 high.
	want := 8000.0 + 6*8*6 - (9*18*38 + 6*6*4)
	if r := soundness(out); !r.Sound() || math.Abs(r.Volume-want) > 0.002*want {
		t.Fatalf("a square post sunk shallow: %s, want %g", r.String(), want)
	}
}

// A quarter turn cut into a block's top, open at the block's bottom and
// at the cut's end in the plane y = 0: that end's plane is moved into the
// cutter by the thickness, so the inside runs through it.
func TestShellJoinedPartialRevolveOpenEnd(t *testing.T) {
	t.Parallel()
	tol := srf.Tol(0.001, 20)
	body := block(20, tol)
	cut, rev := turned(meridian(t, [][2]float64{{0, 15}, {5, 15}, {5, 25}, {0, 25}}, nil, tol), 0, math.Pi/2, tol)
	sh := &srf.SShell{Tol: tol}
	sh.MakeFromDifferenceOf(body, cut)
	parts := []topo.ShellPart{{Shell: body}, {Rev: &rev, Combine: srf.SurfaceDiff}}
	out, err := topo.ShellJoined(sh, parts, facesAt(t, sh, v(15, 15, 0), v(2, 0, 17)), 1, 1000)
	if err != nil {
		t.Fatal(err)
	}
	solid := 32000 - 25*math.Pi/4*5
	inside := 38*38*19 - 5*discArea(6, func(x, y float64) bool { return x >= -1 && y >= 1 })
	inCutter := 4 * discArea(5, func(x, y float64) bool { return x >= 0 && y <= 1 && y >= 0 })
	want := solid - (inside - inCutter)
	if r := soundness(out); !r.Sound() || math.Abs(r.Volume-want) > 0.002*want {
		t.Fatalf("a quarter turn open at an end: %s, want %g", r.String(), want)
	}
	// A half turn open at one end only is refused by name.
	cut, rev = turned(rev.Profile, 0, math.Pi, tol)
	sh = &srf.SShell{Tol: tol}
	sh.MakeFromDifferenceOf(body, cut)
	_, err = topo.ShellJoined(sh, []topo.ShellPart{{Shell: body}, {Rev: &rev, Combine: srf.SurfaceDiff}},
		facesAt(t, sh, v(15, 15, 0), v(2, 0, 17)), 1, 1000)
	if err == nil || !strings.Contains(err.Error(), "two ends of a half turn") {
		t.Errorf("a half turn open at one end: %v", err)
	}
}

// A block with a round pocket drafted wider going up cut into its top,
// open at the block's bottom: the drafted cutter is grown by the
// thickness, its cone moved out square to itself and its floor down, so a
// wall of the thickness stands round the pocket and under its floor.
func TestShellJoinedDraftedPocket(t *testing.T) {
	t.Parallel()
	tol := srf.Tol(0.001, 20)
	body := block(20, tol)
	loop := circle(6, 10, 0)
	alpha := -5 * math.Pi / 180
	angles := make([]float64, len(loop))
	for i := range angles {
		angles[i] = alpha
	}
	cutter, err := topo.DraftedPrismAngles([][]srf.SBezier{loop}, [][]float64{angles}, v(0, 0, 15), core.NewRGBA(100, 100, 100, 255), 0, tol)
	if err != nil {
		t.Fatal(err)
	}
	sh := &srf.SShell{Tol: tol}
	sh.MakeFromDifferenceOf(body, cutter)
	draft := topo.DraftedSolid{Sets: []topo.DraftedSet{{Loops: [][]srf.SBezier{loop}, Angles: [][]float64{angles}}},
		Dir: v(0, 0, 1), Lo: 0, Hi: 15}
	parts := []topo.ShellPart{{Shell: body}, {Shell: cutter, Draft: &draft, Combine: srf.SurfaceDiff}}
	out, err := topo.ShellJoined(sh, parts, facesAt(t, sh, v(15, 15, 0)), 1, 1000)
	if err != nil {
		t.Fatal(err)
	}
	tan, sec := math.Tan(-alpha), 1/math.Cos(alpha)
	frustum := func(h, r1, r2 float64) float64 { return math.Pi * h / 3 * (r1*r1 + r1*r2 + r2*r2) }
	solid := 32000 - frustum(10, 6, 6+10*tan)
	want := solid - (38*38*19 - frustum(10, 6+sec-tan, 6+sec+9*tan))
	if r := soundness(out); !r.Sound() || math.Abs(r.Volume-want) > 0.002*want {
		t.Fatalf("a drafted pocket: %s, want %g", r.String(), want)
	}
}
