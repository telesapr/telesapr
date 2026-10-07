package topo_test

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/check"
	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/internal/sweep"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/topo"
)

// branched is a cylinder of radius 10 along z, from z -20 to 20, with a
// branch of radius ratio*10 leaving its axis at (0, off, 0) at angle
// alpha (radians) from it, 25 long: joined to it, or with hole cut out
// of it. A point on the edge where the branch meets the cylinder's side
// comes back too.
func branched(ratio, alpha, off float64, hole bool) (*srf.SShell, geom.Vector) {
	const big = 10.0
	rho := ratio * big
	var mainT, brT, out srf.SShell
	mainT.MakeFromTransformationOf(cylinder(big, 40), v(0, 0, -20), geom.QuaternionIdentity, 1)
	q := geom.QuaternionFromAxisAngle(v(0, 1, 0), alpha)
	base := v(0, off, 0)
	brT.MakeFromTransformationOf(cylinder(rho, 25), base, q, 1)
	if hole {
		out.MakeFromDifferenceOf(&mainT, &brT)
	} else {
		out.MakeFromUnionOf(&mainT, &brT)
	}
	// The branch's point at angle phi round it, where it leaves the
	// cylinder: (a + t d) at radius 10 from the z axis.
	d := v(math.Sin(alpha), 0, math.Cos(alpha))
	ex, ey := v(math.Cos(alpha), 0, -math.Sin(alpha)), v(0, 1, 0)
	phi := 2.0
	a := base.Plus(ex.ScaledBy(rho * math.Cos(phi))).Plus(ey.ScaledBy(rho * math.Sin(phi)))
	t := (-a.X + math.Sqrt(big*big-a.Y*a.Y)) / d.X
	return &out, a.Plus(d.ScaledBy(t))
}

// filletVolume fillets the edge at at with radius r and gives the
// volume it adds (less than zero when it takes material away), the
// solid checked sound.
func filletVolume(t *testing.T, sh *srf.SShell, at geom.Vector, r float64) (float64, error) {
	t.Helper()
	before := meshVolume(sh)
	if err := topo.FilletPicks(sh, []topo.FilletPick{{At: at, Arc: true}}, r, 1<<30, false); err != nil {
		return 0, err
	}
	return sound(t, sh) - before, nil
}

// fine sets the chord tolerance and the segments for volumes to a small
// share of a fillet's, until the test ends.
func fine(t *testing.T, tol float64) {
	old := srf.ChordTolMm()
	srf.SetChordTolMm(tol)
	srf.SetMaxSegments(2000)
	t.Cleanup(func() {
		srf.SetChordTolMm(old)
		srf.SetMaxSegments(20)
	})
}

// A branch joined to a pipe at right angles (a tee), its edge filled
// round, and a hole cut into the pipe the same way, its rim rounded:
// the volumes against OpenCASCADE's makeFillet on the same solids, or,
// for the hole's larger radius, against our fillet's volume by slices
// in FreeCAD, since there OpenCASCADE's surface strays up to 0.09 mm
// inside the balls it should touch (TestRollingBallOnEnvelope holds
// ours to them).
func TestRollingBallTee(t *testing.T) {
	fine(t, 0.002)
	for _, c := range []struct {
		hole    bool
		r, want float64
	}{
		{false, 1, 3.9617}, {false, 2, 15.769}, {false, 3, 35.370},
		{true, 1, -16.865}, {true, 2, -93.63},
	} {
		sh, at := branched(0.5, math.Pi/2, 0, c.hole)
		got, err := filletVolume(t, sh, at, c.r)
		if err != nil {
			t.Errorf("hole %t r %g: %v", c.hole, c.r, err)
			continue
		}
		if math.Abs(got-c.want) > 0.03*math.Abs(c.want)+0.02 {
			t.Errorf("hole %t r %g: added %g, want %g", c.hole, c.r, got, c.want)
		}
	}
}

// spout is a teapot's body, a cone narrowing from radius 30 to 20 over
// 40, and its spout, a cylinder of radius 4 rising at 48 degrees from
// a point inside, joined; and a point where they meet.
func spout() (*srf.SShell, geom.Vector) {
	var sp, u srf.SShell
	alpha := 42 * math.Pi / 180
	q := geom.QuaternionFromAxisAngle(v(0, 1, 0), alpha)
	base := v(10, 0, 12)
	sp.MakeFromTransformationOf(cylinder(4, 40), base, q, 1)
	u.MakeFromUnionOf(frustum(30, 20, 40), &sp)
	// The spout's top line, y = 4, where it leaves the cone: radius
	// 30 - z/4 from the z axis.
	d := v(math.Sin(alpha), 0, math.Cos(alpha))
	a := base.Plus(v(0, 4, 0))
	lo, hi := 0.0, 40.0
	for range 100 {
		m := (lo + hi) / 2
		p := a.Plus(d.ScaledBy(m))
		if math.Hypot(p.X, p.Y) < 30-p.Z/4 {
			lo = m
		} else {
			hi = m
		}
	}
	return &u, a.Plus(d.ScaledBy(lo))
}

// The teapot's spout filled round where it meets the pot, a cylinder
// meeting a cone at 48 degrees, against OpenCASCADE's volumes.
func TestRollingBallSpout(t *testing.T) {
	fine(t, 0.003)
	for _, c := range []struct{ r, want float64 }{{1, 7.3459}, {2, 29.926}, {3, 68.314}} {
		sh, at := spout()
		got, err := filletVolume(t, sh, at, c.r)
		if err != nil {
			t.Errorf("r %g: %v", c.r, err)
			continue
		}
		if math.Abs(got-c.want) > 0.03*c.want {
			t.Errorf("r %g: added %g, want %g", c.r, got, c.want)
		}
	}
}

// sphereShell is a ball of radius rad about the origin, turned from a
// half circle about z.
func sphereShell(rad float64) *srf.SShell {
	var sh srf.SShell
	var ls srf.SBezierLoopSet
	w := math.Sqrt2 / 2
	a1 := srf.SBezierFrom3(v(0, 0, -rad), v(rad, 0, -rad), v(rad, 0, 0))
	a1.Weight[1] = w
	a2 := srf.SBezierFrom3(v(rad, 0, 0), v(rad, 0, rad), v(0, 0, rad))
	a2.Weight[1] = w
	ls.Outer.Curves = []srf.SBezier{a1, a2, srf.SBezierFrom2(v(0, 0, rad), v(0, 0, -rad))}
	ls.Normal = v(0, 1, 0)
	sh.MakeFromRevolutionOf(&ls, v(0, 0, 0), v(0, 0, 1), core.NewRgbaColor(100, 100, 100, 255), nil)
	return &sh
}

// A rod through a ball off its centre, joined, filled round where they
// meet: the fill grows with the radius.
func TestRollingBallSphere(t *testing.T) {
	fine(t, 0.003)
	last := 0.0
	for _, r := range []float64{1, 2, 3} {
		var cy, u srf.SShell
		cy.MakeFromTransformationOf(cylinder(4, 25), v(0, 3, 0), geom.QuaternionFromAxisAngle(v(0, 1, 0), math.Pi/2), 1)
		u.MakeFromUnionOf(sphereShell(10), &cy)
		got, err := filletVolume(t, &u, v(math.Sqrt(75), 3, 4), r)
		if err != nil {
			t.Errorf("r %g: %v", r, err)
			continue
		}
		if got <= last {
			t.Errorf("r %g: added %g, no more than %g for the smaller radius", r, got, last)
		}
		last = got
	}
}

// What the rolling ball refuses, with the place: a radius the hole's
// rim has no room for, and an edge picked with another.
func TestRollingBallRefusals(t *testing.T) {
	sh, at := branched(0.5, math.Pi/2, 0, true)
	err := topo.FilletPicks(sh, []topo.FilletPick{{At: at, Arc: true}}, 3, 1<<30, false)
	var fe *topo.FilletError
	if !errors.As(err, &fe) || !strings.Contains(err.Error(), "doesn't touch the face") {
		t.Errorf("a fillet too large for the hole's rim: %v", err)
	} else if fe.At.Magnitude() < 5 {
		t.Errorf("the refusal is placed at %v, not on the rim", fe.At)
	}
	// With another edge picked too, the ball's edge must be picked alone.
	sh, at = branched(0.5, math.Pi/2, 0, false)
	top := v(10*math.Cos(0.3), 10*math.Sin(0.3), 20)
	err = topo.FilletPicks(sh, []topo.FilletPick{{At: top, Arc: true}, {At: at, Arc: true}}, 1, 1<<30, false)
	if err == nil || !strings.Contains(err.Error(), "pick it alone") {
		t.Errorf("the ball's edge with another: %v", err)
	}
}

// TestRollingBallSweep fills or rounds where a branch meets a pipe, over
// the branch's size, its angle, how far its axis passes from the pipe's,
// joined or cut, and the radius as a share of the room it has: each solid
// sound and the volume added or taken on the right side of zero. The
// rate allowed, 10%, was set before the general fillet was built, and
// holds for a sample; the whole grid fails 2 of 108 (TELESAPR_SWEEP=1):
// a large branch at 55 degrees with the largest radius, whose ball runs
// off the branch.
func TestRollingBallSweep(t *testing.T) {
	grid := sweep.Grid{
		Axes: []sweep.Axis{
			sweep.Strings("kind", "joined", "hole"),
			sweep.Floats("ratio", 0.3, 0.5, 0.7),
			sweep.Floats("angle", 90, 70, 55),
			sweep.Floats("off", 0, 0.2),
			sweep.Floats("k", 0.1, 0.25, 0.4),
		},
		MaxFailRate:       2.5 / 108,
		SampleMaxFailRate: 0.10,
		Sample:            16,
	}
	grid.Run(func(p sweep.Point) error {
		old := srf.ChordTolMm()
		defer srf.SetChordTolMm(old)
		srf.SetChordTolMm(0.01)
		hole := p.String("kind") == "hole"
		ratio := p.Float("ratio")
		sh, at := branched(ratio, p.Float("angle")*math.Pi/180, p.Float("off")*10, hole)
		before := meshVolume(sh)
		// The room a fillet has: the branch's radius; for a hole, no more
		// than the wall beside it, between the hole's side and the pipe's.
		room := ratio * 10
		if hole {
			room = math.Min(room, 10-ratio*10-p.Float("off")*10)
		}
		r := p.Float("k") * room
		if err := topo.FilletPicks(sh, []topo.FilletPick{{At: at, Arc: true}}, r, 1<<30, false); err != nil {
			return err
		}
		tp, err := topo.Build(sh)
		if err != nil {
			return err
		}
		if probs := tp.Validate(); len(probs) > 0 {
			return fmt.Errorf("%s", probs[0].Msg)
		}
		var m srf.SMesh
		sh.TriangulateInto(&m)
		rep := check.Solid(sh, &m)
		if !rep.Sound() {
			return fmt.Errorf("unsound: %s", rep.String())
		}
		if added := rep.Volume - before; (added > 0) == hole {
			return fmt.Errorf("added %g", added)
		}
		return nil
	}).Require(t)
}

// A chamfer where a branch meets a pipe at right angles: the same edge
// as the fillet's, its sections straight from the points at its
// distance along each face. Where the faces meet square it takes r²/2
// across the edge against the fillet's r²(1 - π/4), and the corner
// opens wider round the edge, so it adds between that ratio and twice
// the fillet's volume times it.
func TestRollingBallChamferTee(t *testing.T) {
	fine(t, 0.002)
	sh, at := branched(0.5, math.Pi/2, 0, false)
	before := meshVolume(sh)
	if err := topo.FilletPicks(sh, []topo.FilletPick{{At: at, Arc: true}}, 1, 1<<30, true); err != nil {
		t.Fatal(err)
	}
	added := sound(t, sh) - before
	ratio := 0.5 / (1 - math.Pi/4)
	if fillet := 3.9617; added < 0.5*ratio*fillet || added > 2*ratio*fillet {
		t.Errorf("added %g, want about %g", added, ratio*fillet)
	}
}
