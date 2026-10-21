package srf

import (
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
)

// Profiles swept round closed paths of lines and arcs: a turn about an
// axis, a stadium of two lines and two half turns, and a frame of four
// lines mitred at its corners, started halfway along a side. No caps;
// watertight, of the volume the centroid theorem gives, the profile's
// area times the length of the path through its centre.
func TestClosedSweep(t *testing.T) {
	t.Parallel()
	y := geom.Vector{Y: 1}
	corner := func(at geom.Vector, d1, d2 geom.Vector) SweepStep {
		return SweepStep{Corner: true, Center: at, Axis: d1.Cross(d2).WithMagnitude(1), Angle: math.Pi / 2}
	}
	z, x := geom.Vector{Z: 1}, geom.Vector{X: 1}
	for _, c := range []struct {
		name  string
		steps []SweepStep
		want  float64
	}{
		{"ring", []SweepStep{{Arc: true, Center: geom.Vector{X: 10}, Axis: y, Angle: 2 * math.Pi}}, 4 * 10 * 2 * math.Pi},
		{"stadium", []SweepStep{
			{Along: geom.Vector{Z: 10}},
			{Arc: true, Center: geom.Vector{X: 8, Z: 10}, Axis: y, Angle: math.Pi},
			{Along: geom.Vector{Z: -10}},
			{Arc: true, Center: geom.Vector{X: 8}, Axis: y, Angle: math.Pi},
		}, 4 * (20 + 2*8*math.Pi)},
		{"frame", []SweepStep{
			{Along: geom.Vector{Z: 5}},
			corner(geom.Vector{Z: 5}, z, x),
			{Along: geom.Vector{X: 20}},
			corner(geom.Vector{X: 20, Z: 5}, x, z.ScaledBy(-1)),
			{Along: geom.Vector{Z: -10}},
			corner(geom.Vector{X: 20, Z: -5}, z.ScaledBy(-1), x.ScaledBy(-1)),
			{Along: geom.Vector{X: -20}},
			corner(geom.Vector{Z: -5}, x.ScaledBy(-1), z),
			{Along: geom.Vector{Z: 5}},
		}, 4 * 60},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if end := SweepEnd(c.steps); end.Apply(geom.Vector{X: 1, Y: 1}).Minus(geom.Vector{X: 1, Y: 1}).Magnitude() > 1e-9 {
				t.Fatalf("the path doesn't close: %v", end)
			}
			for _, flip := range []bool{false, true} {
				sq := squareAt(0, 0, 2)
				if flip {
					// The profile facing the other way: swept from the other end.
					sq = PolygonLoopSet([]geom.Vector{{X: -1, Y: -1}, {X: -1, Y: 1}, {X: 1, Y: 1}, {X: 1, Y: -1}}, geom.Vector{Z: -1})
				}
				var s SShell
				s.SetTolerance(Tol(1e-3, MaxSegments()))
				s.MakeFromClosedSweepOf(sq, c.steps, core.RGBA{})
				var open SShell
				open.MakeFromSweepOf(sq, c.steps, core.RGBA{}, 0, 0)
				if len(s.Surface) != len(open.Surface)-2 {
					t.Errorf("flip %v: %d surfaces, want the open sweep's %d less its caps", flip, len(s.Surface), len(open.Surface))
				}
				if !meshIsSound(&s) {
					t.Errorf("flip %v: the sweep is not watertight", flip)
				}
				if v := shellVolume(&s); math.Abs(v-c.want) > 2e-3*c.want {
					t.Errorf("flip %v: volume %g, want %g", flip, v, c.want)
				}
			}
		})
	}
}
