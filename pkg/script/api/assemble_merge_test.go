package api

import (
	"math"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/sketch"
)

// overlappingParts assembles a plate and two cheeks standing in it, the
// three lying on each other in their top faces and their sides.
func overlappingParts(t *testing.T) (*Doc, Entity) {
	t.Helper()
	d := New()
	d.Features.Sketch3D("datums")
	at := d.Sketch.Point(V(0, 0, 0))
	d.Constrain.Lock(at)
	d.Features.SketchAt(at, V(1, 0, 0), V(0, 1, 0), "")
	d.Sketch.Polyline(true, V(0, 0, 6), V(10, 0, 6), V(10, 12, 6), V(0, 12, 6))
	d.Features.Extrude(3, "plate").SetCombine(Assemble)
	for _, y := range []float64{0, 9} {
		d.Features.SketchAt(at, V(1, 0, 0), V(0, 0, -1), "")
		d.Sketch.Polyline(true, V(0, y, 0), V(10, y, 0), V(10, y, 9), V(0, y, 9))
		d.Sketch.Circle(V(5, y, 5), 1.8)
		d.Features.Extrude(3, "cheek").SetCombine(Assemble)
	}
	return d, at
}

// TestAssembleAfterSketch assembles parts that lie on each other. The
// sketch of each part after the first passes the solid so far through,
// and joining coincident faces there made faces of several parts,
// which did not mesh: the last cheek's volume was some -14000 mm³.
func TestAssembleAfterSketch(t *testing.T) {
	t.Parallel()
	d, _ := overlappingParts(t)
	r := d.Active().Analyze().Check()
	// The parts' volumes, each counted whole.
	const plate, cheek = 10 * 12 * 3, 10*9*3 - math.Pi*1.8*1.8*3
	if r.NakedEdges != 0 || r.Volume < plate+2*cheek-1 || r.Volume > plate+2*cheek+1 {
		t.Errorf("%d naked edges, volume %g, want %g", r.NakedEdges, r.Volume, plate+2*cheek)
	}
}

// TestUnionAfterAssembly joins a box apart from the assembled parts to
// them: the faces of the parts that lie on each other stay apart.
func TestUnionAfterAssembly(t *testing.T) {
	t.Parallel()
	d, at := overlappingParts(t)
	d.Features.SketchAt(at, V(1, 0, 0), V(0, 1, 0), "")
	d.Sketch.Polyline(true, V(30, 0, 0), V(35, 0, 0), V(35, 5, 0), V(30, 5, 0))
	g := d.Features.Extrude(5, "box")
	r := g.Analyze().Check()
	if r.NakedEdges != 0 || r.Volume < 360+2*240+125-2 {
		t.Errorf("%d naked edges, volume %g", r.NakedEdges, r.Volume)
	}
}

// TestJoinsAssembly tells a group whose solid so far holds assembled
// parts, and a failed Boolean of one says that the parts are the likely
// cause; one first part assembled onto nothing is a part alone.
func TestJoinsAssembly(t *testing.T) {
	t.Parallel()
	cutAfter := func(d *Doc, at Entity) Group {
		d.Features.SketchAt(at, V(1, 0, 0), V(0, 1, 0), "")
		d.Sketch.Circle(V(5, 5, 0), 1)
		g := d.Features.Extrude(20, "cut")
		g.SetCombine(Difference)
		return g
	}
	d, at := overlappingParts(t)
	cut := cutAfter(d, at)
	alone := New()
	alone.Features.Sketch3D("datums")
	at2 := alone.Sketch.Point(V(0, 0, 0))
	alone.Constrain.Lock(at2)
	alone.Features.SketchAt(at2, V(1, 0, 0), V(0, 1, 0), "")
	alone.Sketch.Polyline(true, V(0, 0, 0), V(10, 0, 0), V(10, 10, 0), V(0, 10, 0))
	alone.Features.Extrude(3, "plate").SetCombine(Assemble)
	cutAlone := cutAfter(alone, at2)
	for _, c := range []struct {
		g    Group
		want bool
	}{{cut, true}, {cutAlone, false}} {
		c.g.d.read(func(sk *sketch.Sketch) {
			group := c.g.get(sk)
			if got := group.JoinsAssembly(sk); got != c.want {
				t.Errorf("%v joins an assembly: %v", c.want, got)
			}
			group.BooleanFailed = true
			if msg := problemOf(sk, group); strings.Contains(msg, "assembled") != c.want {
				t.Errorf("problem %q", msg)
			}
			group.BooleanFailed = false
		})
	}
}

// TestCutAcrossTouchingParts cuts a slot across two blocks assembled
// face to face, through the face they share, as the rover's D-flat cut
// through a shaft and the gearbox it stands on. The cut of the two
// together failed, and the group's solid was unsound; each block is
// cut on its own then.
func TestCutAcrossTouchingParts(t *testing.T) {
	t.Parallel()
	d := New()
	d.Features.Sketch3D("datums")
	at := d.Sketch.Point(V(0, 0, 0))
	d.Constrain.Lock(at)
	up := d.Sketch.Point(V(0, 0, 10))
	d.Constrain.Lock(up)
	for _, o := range []Entity{at, up} {
		z := o.Pos().Z
		d.Features.SketchAt(o, V(1, 0, 0), V(0, 1, 0), "")
		d.Sketch.Polyline(true, V(0, 0, z), V(10, 0, z), V(10, 10, z), V(0, 10, z))
		d.Features.Extrude(10, "block").SetCombine(Assemble)
	}
	d.Features.SketchAt(at, V(1, 0, 0), V(0, 0, 1), "")
	d.Sketch.Polyline(true, V(2, -1, 5), V(4, -1, 5), V(4, -1, 15), V(2, -1, 15))
	cut := d.Features.Extrude(-12, "slot")
	cut.SetCombine(Difference)
	r := cut.Analyze().Check()
	if r.BooleanFailed || r.NakedEdges != 0 || math.Abs(r.Volume-(2000-2*10*10)) > 1e-6 || cut.Problem() != "" {
		t.Errorf("%s, problem %q", r.String(), cut.Problem())
	}
}
