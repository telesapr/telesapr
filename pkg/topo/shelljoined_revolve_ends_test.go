package topo_test

import (
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/topo"
)

// Rings revolved about the Z axis on a block's side (y = 0), an end
// standing on that side, so inside the block, open at the block's top:
// the end inside the block is moved out into it, an end in the air is a
// wall. Of more than half a turn, the wall walls off what lies within
// the thickness of the end itself; of a half turn whose ends move
// unlike, the plane of the ends steps, the thickness past the axis on
// the moved-out end's side, so the wall runs on round the axis. The
// ring is 5 to 10 out and 10 high, its inside 6 to 9 and 1 to 9 high;
// volumes by the inside's area in the ring's plan, in rows.
func TestShellJoinedRevolveEndsMovedUnlike(t *testing.T) {
	t.Parallel()
	tol := srf.Tol(0.001, 20)
	profile := meridian(t, [][2]float64{{5, 0}, {10, 0}, {10, 10}, {5, 10}}, nil, tol)
	annulus := func(in func(x, y float64) bool) float64 { return discArea(9, in) - discArea(6, in) }
	cases := []struct {
		name           string
		x0, x1, y0, y1 float64 // the block's extent, 10 high
		start, end     float64
		blockInner     float64 // the block's inside, open at its top
		in             func(x, y float64) bool
		open           []geom.Vector // faces taken away besides the block's top
	}{
		// Three quarters of a turn, the end at 3π/2 deep inside the
		// block too: both ends moved out, the inside all round but past
		// both planes moved out; where it lies in the block, the block's
		// inside holds it already.
		{"three quarters inside", -20, 20, -20, 0, 0, 3 * math.Pi / 2, 38 * 18 * 9,
			func(_, y float64) bool { return y >= -1 }, nil},
		// Three quarters of a turn, the end at 3π/2 in the air: a wall
		// 1 thick along it, the end on the block moved out past y = 0.
		{"three quarters, a wall", 2, 22, -20, 0, 0, 3 * math.Pi / 2, 18 * 18 * 9,
			func(x, y float64) bool { return !(y < -1 && x > 0) && !(math.Abs(x) <= 1 && y <= 0) }, nil},
		// The same, the wall the turn's start: from π/2 round to the
		// block's side at 2π.
		{"three quarters, a wall at the start", 2, 22, 0, 20, math.Pi / 2, 2 * math.Pi, 18 * 18 * 9,
			func(x, y float64) bool { return !(y > 1 && x > 0) && !(math.Abs(x) <= 1 && y >= 0) }, nil},
		// Turned the other way, from 0 down to -3π/2, the block above
		// y = 0.
		{"three quarters, a wall, turned back", 2, 22, 0, 20, 0, -3 * math.Pi / 2, 18 * 18 * 9,
			func(x, y float64) bool { return !(y > 1 && x > 0) && !(math.Abs(x) <= 1 && y >= 0) }, nil},
		{"half, a wall, turned back", 2, 22, 0, 20, 0, -math.Pi, 18 * 18 * 9,
			func(x, y float64) bool { return (x > 1 && y < 1) || (x <= 1 && y < -1) }, nil},
		// The end in the air taken away: the inside runs to its plane.
		{"three quarters, open", 2, 22, -20, 0, 0, 3 * math.Pi / 2, 18 * 18 * 9,
			func(x, y float64) bool { return !(y < -1 && x > 0) }, []geom.Vector{v(0, -7.5, 5)}},
		// A half turn, the end at π in the air: the step at x = 1.
		{"half, a wall", 2, 22, -20, 0, 0, math.Pi, 18 * 18 * 9,
			func(x, y float64) bool { return (x > 1 && y > -1) || (x <= 1 && y > 1) }, nil},
		// The same, that end taken away: the step at the axis.
		{"half, open", 2, 22, -20, 0, 0, math.Pi, 18 * 18 * 9,
			func(x, y float64) bool { return (x > 0 && y > -1) || (x <= 0 && y > 0) }, []geom.Vector{v(-7.5, 0, 5)}},
		// A half turn, the end at 0 in the air: the step at x = -1.
		{"half, a wall at the start", -22, -2, -20, 0, 0, math.Pi, 18 * 18 * 9,
			func(x, y float64) bool { return (x > -1 && y > 1) || (x <= -1 && y > -1) }, nil},
	}
	for _, c := range cases {
		body := &srf.SShell{Tol: tol}
		body.MakePrism([]geom.Vector{v(c.x0, c.y0, 0), v(c.x1, c.y0, 0), v(c.x1, c.y1, 0), v(c.x0, c.y1, 0)}, v(0, 0, 0), v(0, 0, 10),
			core.NewRGBA(100, 100, 100, 255))
		ring, rev := turned(profile, c.start, c.end, tol)
		sh := &srf.SShell{Tol: tol}
		sh.MakeFromUnionOf(body, ring)
		parts := []topo.ShellPart{{Shell: body}, {Rev: &rev}}
		top := v((c.x0+c.x1)/2, (c.y0+c.y1)/2, 10)
		if c.x0 < 0 && c.x1 > 0 {
			top = v(15, -15, 10)
		}
		out, err := topo.ShellJoined(sh, parts, facesAt(t, sh, append([]geom.Vector{top}, c.open...)...), 1, 1000)
		if err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		solid := (c.x1-c.x0)*(c.y1-c.y0)*10 + 75*math.Abs(c.end-c.start)/2*10
		if c.x0 < 0 && c.x1 > 0 {
			// The quarter from π to 3π/2 lies inside the block.
			solid = 8000 + 375*math.Pi
		}
		want := solid - (c.blockInner + 8*annulus(c.in))
		if r := soundness(out); !r.Sound() || math.Abs(r.Volume-want) > 0.002*want {
			t.Errorf("%s: %s, want %g", c.name, r.String(), want)
		}
	}
}
