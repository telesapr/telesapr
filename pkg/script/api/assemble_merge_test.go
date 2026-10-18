package api

import (
	"math"
	"testing"
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
