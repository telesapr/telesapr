package api

import (
	"fmt"
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/internal/fixtures"
	"github.com/telesapr/telesapr/pkg/internal/sweep"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// curvedEndPlateOnRod is a plate 2 wide (y from -1 to 1) standing on a
// rod of radius 5 along x from 0 to 40, its top an arc about an axis
// along y, 0.58 above the foot of the plate's side on the rod (z =
// sqrt(24)) at x = 9 and 17 and 2.5 at 13, its ends hollowed by
// cylinders about z of radius rc that reach x = 10 and 16 at y = 0. It
// gives the edge of the plate's side at y = 1 on the rod and the volume
// before: a blend of it that runs off the side past the top ends on a
// curved end and runs onto a curved top, which meet in a curve that is
// no line.
func curvedEndPlateOnRod(d *Doc, rc float64) (SolidEdge, float64) {
	foot := math.Sqrt(24)
	d.Features.SketchIn(d.Workplane("ZX"), "")
	cz, _ := curvedEndArc()
	d.Sketch.Line(V(9, 0, foot+0.58), V(9, 0, 3))
	d.Sketch.Line(V(9, 0, 3), V(17, 0, 3))
	d.Sketch.Line(V(17, 0, 3), V(17, 0, foot+0.58))
	d.Sketch.Arc(V(13, 0, cz), V(9, 0, foot+0.58), V(17, 0, foot+0.58))
	plate := d.Features.Extrude(2, "plate")
	plate.SetTwoSided(true)
	for _, s := range []float64{-1, 1} {
		d.Features.Sketch3D("hollow")
		d.Features.SketchIn(d.Sketch.NewWorkplane(V(0, 0, 0), V(1, 0, 0), V(0, 1, 0)), "")
		d.Sketch.Circle(V(13+s*(3+rc), 0, 0), rc)
		cut := d.Features.Extrude(20, "hollow")
		cut.SetCombine(sketch.CombineDifference)
	}
	d.Features.SketchIn(d.Workplane("YZ"), "")
	d.Sketch.Circle(V(0, 0, 0), 5)
	rod := d.Features.Extrude(40, "rod")
	return rod.Solid().EdgeAt(V(13, 1, foot)), rod.Analyze().Check().Volume
}

// curvedEndArc is the height of the centre of the arc of
// curvedEndPlateOnRod's top, and its radius.
func curvedEndArc() (float64, float64) {
	half, sag := 4.0, 2.5-0.58
	r := (half*half + sag*sag) / (2 * sag)
	return math.Sqrt(24) + 2.5 - r, r
}

// curvedEndAdded is what a blend of size r of curvedEndPlateOnRod's
// edge adds, integrated as gabledAdded is: across the edge its section
// is the same along it, and the plate's ends bound it in x by y alone,
// its top by x alone.
func curvedEndAdded(rc, r float64, chamfer bool) float64 {
	zA, R := curvedEndArc()
	upper, y1 := blendSectionTop(r, chamfer)
	const ny, nz = 1500, 1500
	sum := 0.0
	for j := range ny {
		y := 1 + (float64(j)+0.5)/ny*(y1-1)
		zr, zu := math.Sqrt(25-y*y), upper(y)
		if zu <= zr {
			continue
		}
		xa, xb := 13-3-rc+math.Sqrt(rc*rc-y*y), 13+3+rc-math.Sqrt(rc*rc-y*y)
		for i := range nz {
			z := zr + (float64(i)+0.5)/nz*(zu-zr)
			w := math.Sqrt(math.Max(0, R*R-(z-zA)*(z-zA)))
			if ln := math.Min(xb, 13+w) - math.Max(xa, 13-w); ln > 0 {
				sum += ln * (zu - zr) / nz
			}
		}
	}
	return sum * (y1 - 1) / ny
}

// Chamfers of the edge a plate stands on a rod by, which run off the
// plate's side past its curved top at ends hollowed by cylinders: the
// end and the top, both curved, share the curve where their surfaces
// meet, carried on past their edges with the side (topo's sharedCurve),
// which was refused before 2026-11-04 ("the face it ends at is curved
// too"). Sound, and adding what curvedEndAdded integrates within 0.2%
// and 0.02: the curved faces' meshes, the ends' hollow, add a little.
// The ordinary run takes the smallest chamfer of the ends hollowed at 3
// and the smallest and largest at 4, side by side; make test-sweeps all
// six.
func TestChamferRunsOffCurvedEndOntoCurvedTop(t *testing.T) {
	t.Parallel()
	cases := [][2]float64{{3, 1.6}, {3, 1.8}, {4, 1.8}, {4, 2}, {4, 2.2}, {4, 2.4}}
	switch {
	case sweep.Full():
	case testing.Short() || fixtures.RaceDetector:
		cases = cases[2:3]
	default:
		cases = [][2]float64{cases[0], cases[2], cases[5]}
	}
	for _, c := range cases {
		t.Run(fmt.Sprintf("ends hollowed at %g, chamfer of %g", c[0], c[1]), func(t *testing.T) {
			t.Parallel()
			d := New()
			d.Edit.SetChordTolerance(0.003)
			d.Edit.SetMaxSegments(200)
			edge, before := curvedEndPlateOnRod(d, c[0])
			g, err := blendOrRefusal(d, edge, c[1], true)
			if err != nil {
				t.Fatal(err)
			}
			rep := g.Analyze().Check()
			if !rep.Sound() {
				t.Fatal(rep.Summary())
			}
			added, want := rep.Volume-before, curvedEndAdded(c[0], c[1], true)
			if math.Abs(added-want) > 0.02+0.002*want {
				t.Errorf("added %g, %g integrated", added, want)
			}
		})
	}
}
