package topo_test

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/spacesolve/spacesolve/pkg/check"
	"github.com/spacesolve/spacesolve/pkg/core"
	"github.com/spacesolve/spacesolve/pkg/geom"
	"github.com/spacesolve/spacesolve/pkg/internal/sweep"
	"github.com/spacesolve/spacesolve/pkg/srf"
	"github.com/spacesolve/spacesolve/pkg/topo"
)

// cylinder is a cylinder of radius rad and height h on the XY plane,
// about the z axis. The extrusion takes an outer loop running clockwise
// about the loop set's normal.
func cylinder(rad, h float64) *srf.SShell {
	var sh srf.SShell
	var ls srf.SBezierLoopSet
	ls.Outer.Curves = circle(rad, 0, 0)
	ls.Normal = v(0, 0, -1)
	sh.MakeFromExtrusionOf(&ls, v(0, 0, 0), v(0, 0, h), core.NewRgbaColor(100, 100, 100, 255))
	return &sh
}

// plateWithHole is a 40 x 40 x h plate with a hole of radius rad
// through its middle.
func plateWithHole(rad, h float64) *srf.SShell {
	var plate, out srf.SShell
	plate.MakePrism([]geom.Vector{v(-20, -20, 0), v(20, -20, 0), v(20, 20, 0), v(-20, 20, 0)}, v(0, 0, 0), v(0, 0, h),
		core.NewRgbaColor(100, 100, 100, 255))
	hole := cylinder(rad, h+2)
	var moved srf.SShell
	moved.MakeFromTransformationOf(hole, v(0, 0, -1), geom.QuaternionIdentity, 1)
	out.MakeFromDifferenceOf(&plate, &moved)
	return &out
}

// spandrel is the area a fillet of radius r takes from a square corner,
// and the distance of its centroid from each face.
func spandrel(r float64) (area, c float64) {
	return r * r * (1 - math.Pi/4), r * (10 - 3*math.Pi) / (12 - 3*math.Pi)
}

// sound meshes sh and checks it, returning its volume.
func sound(t *testing.T, sh *srf.SShell) float64 {
	t.Helper()
	tp := build(t, sh)
	var m srf.SMesh
	sh.TriangulateInto(&m)
	r := check.Solid(sh, &m)
	if !r.Sound() {
		t.Errorf("unsound: %s", r.String())
	}
	if e, me := tp.Euler(), topo.MeshEuler(&m); e != me {
		t.Errorf("Euler %d, the mesh's %d", e, me)
	}
	return r.Volume
}

// A cylinder's top edge, picked by one point, filleted all round (the
// four arcs of its circle): a torus. By Pappus, the volume taken is the
// spandrel's area times the length of the circle its centroid follows.
func TestFilletCylinderTop(t *testing.T) {
	old := srf.ChordTolMm()
	defer srf.SetChordTolMm(old)
	for _, c := range []struct{ rad, r float64 }{{5, 1}, {5, 4}, {20, 3}} {
		srf.SetChordTolMm(0.001 * c.r)
		sh := cylinder(c.rad, 10)
		at := v(c.rad*math.Cos(0.3), c.rad*math.Sin(0.3), 10)
		if err := topo.FilletPicks(sh, []topo.FilletPick{{At: at, Arc: true}}, c.r, 1<<30, false); err != nil {
			t.Fatalf("%+v: %v", c, err)
		}
		a, cd := spandrel(c.r)
		want := math.Pi*c.rad*c.rad*10 - a*2*math.Pi*(c.rad-cd)
		got := sound(t, sh)
		// The chords cut inside the cylinder and the torus by up to the
		// tolerance over their area.
		allow := 0.001 * c.r * 2 * math.Pi * c.rad * (10 + math.Pi/2*c.r)
		if math.Abs(got-want) > allow {
			t.Errorf("%+v: volume %g, want %g within %g", c, got, want, allow)
		}
	}
}

// The same edge chamfered: a cone, taking a right triangle of side d.
func TestChamferCylinderTop(t *testing.T) {
	old := srf.ChordTolMm()
	defer srf.SetChordTolMm(old)
	srf.SetChordTolMm(0.001)
	for _, d := range []float64{1, 3} {
		sh := cylinder(5, 10)
		if err := topo.FilletPicks(sh, []topo.FilletPick{{At: v(5, 0, 10), Arc: true}}, d, 1<<30, true); err != nil {
			t.Fatal(err)
		}
		want := math.Pi*25*10 - d*d/2*2*math.Pi*(5-d/3)
		// The chords cut inside the cylinder and the cone by up to the
		// tolerance over their area.
		if got := sound(t, sh); math.Abs(got-want) > 0.001*2*math.Pi*5*(10+d*math.Sqrt2) {
			t.Errorf("chamfer %g: volume %g, want %g", d, got, want)
		}
	}
}

// A hole's rim in a plate, filleted: the torus turns about the hole, so
// the spandrel's centroid lies outside the circle.
func TestFilletHoleRim(t *testing.T) {
	old := srf.ChordTolMm()
	defer srf.SetChordTolMm(old)
	srf.SetChordTolMm(0.002)
	for _, r := range []float64{1, 3} {
		sh := plateWithHole(6, 10)
		before := sound(t, sh)
		if err := topo.FilletPicks(sh, []topo.FilletPick{{At: v(0, 6, 10), Arc: true}}, r, 1<<30, false); err != nil {
			t.Fatal(err)
		}
		a, cd := spandrel(r)
		want := before - a*2*math.Pi*(6+cd)
		allow := 0.002 * (math.Pi / 2 * r) * 2 * math.Pi * (6 + r) * 2
		if got := sound(t, sh); math.Abs(got-want) > allow {
			t.Errorf("rim at %g: volume %g, want %g within %g", r, got, want, allow)
		}
	}
}

func TestFilletCircleRefused(t *testing.T) {
	// A boss on a plate.
	var plate, boss, sh srf.SShell
	plate.MakePrism([]geom.Vector{v(-20, -20, 0), v(20, -20, 0), v(20, 20, 0), v(-20, 20, 0)}, v(0, 0, 0), v(0, 0, 5),
		core.NewRgbaColor(100, 100, 100, 255))
	boss.MakeFromTransformationOf(cylinder(5, 10), v(0, 0, 5), geom.QuaternionIdentity, 1)
	sh.MakeFromUnionOf(&plate, &boss)
	for _, c := range []struct {
		at   geom.Vector
		r    float64
		flat bool
		want string
	}{
		{v(5, 0, 15), 6, false, "wider than the cylinder"},
		{v(20, 0, 5), 1, true, "straight ones are cut by a Boolean"},
	} {
		var cp srf.SShell
		cp.MakeFromCopyOf(&sh)
		var picks []topo.FilletPick
		if c.flat && c.at.X == 20 {
			picks = []topo.FilletPick{{A: v(20, -20, 5), B: v(20, 20, 5)}}
		} else {
			picks = []topo.FilletPick{{At: c.at, Arc: true}}
		}
		err := topo.FilletPicks(&cp, picks, c.r, 1<<30, c.flat)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%v: %v, want %q", c.at, err, c.want)
		}
	}
	// The boss's top, on the union, rounds.
	var cp srf.SShell
	cp.MakeFromCopyOf(&sh)
	if err := topo.FilletPicks(&cp, []topo.FilletPick{{At: v(0, 5, 15), Arc: true}}, 1, 1<<30, false); err != nil {
		t.Errorf("the boss's top: %v", err)
	}
}

// TestFilletCircleSweep rounds or chamfers a cylinder's top circle or a
// hole's rim over the circle's radius, the height and the size of the
// cut, against Pappus's volumes; the rate allowed, 0, was set before.
func TestFilletCircleSweep(t *testing.T) {
	old := srf.ChordTolMm()
	defer srf.SetChordTolMm(old)
	grid := sweep.Grid{
		Axes: []sweep.Axis{
			sweep.Strings("edge", "boss", "hole"),
			sweep.Strings("cut", "fillet", "chamfer"),
			sweep.Floats("radius", 2, 5, 15),
			sweep.Floats("height", 3, 10),
			sweep.Floats("k", 0.1, 0.5, 0.9),
		},
		MaxFailRate: 0,
		Sample:      24,
	}
	grid.Run(func(p sweep.Point) error {
		rad, h := p.Float("radius"), p.Float("height")
		hole, flat := p.String("edge") == "hole", p.String("cut") == "chamfer"
		// A boss's cut fits in its radius; a hole's in the plate round it,
		// 20 - 15 wide at the least; both in the height.
		room := math.Min(rad, h)
		if hole {
			room = math.Min(20-rad, h)
		}
		r := p.Float("k") * room
		tol := 0.002 * r
		srf.SetChordTolMm(tol)
		var sh *srf.SShell
		if hole {
			sh = plateWithHole(rad, h)
		} else {
			sh = cylinder(rad, h)
		}
		var m0 srf.SMesh
		sh.TriangulateInto(&m0)
		before := m0.CalculateVolume()
		at := v(rad*math.Cos(1), rad*math.Sin(1), h)
		if err := topo.FilletPicks(sh, []topo.FilletPick{{At: at, Arc: true}}, r, 1<<30, flat); err != nil {
			return err
		}
		var m srf.SMesh
		sh.TriangulateInto(&m)
		rep := check.Solid(sh, &m)
		var want float64
		switch {
		case flat && hole:
			want = r * r / 2 * 2 * math.Pi * (rad + r/3)
		case flat:
			want = r * r / 2 * 2 * math.Pi * (rad - r/3)
		default:
			a, c := spandrel(r)
			if hole {
				want = a * 2 * math.Pi * (rad + c)
			} else {
				want = a * 2 * math.Pi * (rad - c)
			}
		}
		// The chords cut inside the cut face and the cylinder, by up to
		// the tolerance over their area.
		allow := tol * 2 * math.Pi * (rad + r) * (h + 2*r)
		switch {
		case !rep.Sound():
			return fmt.Errorf("unsound: %s", rep.String())
		case math.Abs(before-rep.Volume-want) > allow:
			return fmt.Errorf("took %g, want %g within %g", before-rep.Volume, want, allow)
		}
		return nil
	}).Require(t)
}
