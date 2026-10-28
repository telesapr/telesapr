package api

import (
	"bufio"
	"errors"
	"fmt"
	"math"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/internal/fixtures"
)

// curvedBandOnCone is bandOnCone with the band's top a cylinder rising
// sag above the band's edges at height top (a hollow where sag is below
// 0): about an axis along y (kind "y"), its profile the band's in the
// plane y = 0 and the band two-sided 2y wide; or about an axis along x
// (kind "x"), its profile the band's end at x = 6, which the top meets
// in an arc. The band's side on the cone is filleted past its top onto
// that cylinder.
func curvedBandOnCone(d *Doc, kind string, y, top, sag float64) (SolidEdge, float64) {
	d.Features.SketchIn(d.Workplane("ZX"), "")
	axis := d.Sketch.Line(V(0, 0, -2), V(0, 0, 32))
	axis.SetConstruction(true)
	d.Sketch.Polyline(true, V(0, 0, 0), V(10, 0, 0), V(4, 0, 30), V(0, 0, 30))
	d.Features.Lathe(axis, "")
	var band Group
	if kind == "y" {
		d.Features.SketchIn(d.Workplane("ZX"), "")
		// The arc through (6, top), (20, top) and (13, top+sag).
		c := (49 - sag*sag) / (2 * sag)
		d.Sketch.Line(V(6, 0, top), V(6, 0, 0))
		d.Sketch.Line(V(6, 0, 0), V(20, 0, 0))
		d.Sketch.Line(V(20, 0, 0), V(20, 0, top))
		if sag > 0 {
			d.Sketch.Arc(V(13, 0, top-c), V(6, 0, top), V(20, 0, top))
		} else {
			d.Sketch.Arc(V(13, 0, top-c), V(20, 0, top), V(6, 0, top))
		}
		band = d.Features.Extrude(2*y, "band")
		band.SetTwoSided(true)
	} else {
		d.Features.Sketch3D("end")
		d.Features.SketchIn(d.Sketch.NewWorkplane(V(6, 0, 0), V(0, 1, 0), V(0, 0, 1)), "")
		// The arc through (-y, top), (y, top) and (0, top+sag).
		c := (y*y - sag*sag) / (2 * sag)
		d.Sketch.Line(V(6, -y, top), V(6, -y, 0))
		d.Sketch.Line(V(6, -y, 0), V(6, y, 0))
		d.Sketch.Line(V(6, y, 0), V(6, y, top))
		if sag > 0 {
			d.Sketch.Arc(V(6, 0, top-c), V(6, y, top), V(6, -y, top))
		} else {
			d.Sketch.Arc(V(6, 0, top-c), V(6, -y, top), V(6, y, top))
		}
		band = d.Features.Extrude(14, "band")
	}
	return band.Solid().EdgeAt(V(math.Sqrt(64-y*y), y, 10)), band.Analyze().Check().Volume
}

// runOffOracle is a line of testdata/topo/fillet-runoff-curved.txt:
// OpenCASCADE's volumes before and after the fillet, or refused.
type runOffOracle struct {
	kind           string
	y, top, sag, r float64
	before, after  float64
	refused        bool
}

func readRunOffOracle(t *testing.T) []runOffOracle {
	t.Helper()
	f, err := os.Open("../../../testdata/topo/fillet-runoff-curved.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	var out []runOffOracle
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fs := strings.Fields(sc.Text())
		if len(fs) < 6 {
			continue
		}
		o := runOffOracle{kind: fs[0], refused: fs[5] == "refused"}
		var x [6]float64
		for i, s := range fs[1:] {
			if i < 4 || !o.refused {
				if x[i], err = strconv.ParseFloat(s, 64); err != nil {
					t.Fatal(err)
				}
			}
		}
		o.y, o.top, o.sag, o.r, o.before, o.after = x[0], x[1], x[2], x[3], x[4], x[5]
		out = append(out, o)
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// blendOrRefusal fillets or chamfers edge with size r, giving the group
// or the error that refused it.
func blendOrRefusal(d *Doc, edge SolidEdge, r float64, chamfer bool) (g Group, err error) {
	defer func() {
		if p := recover(); p != nil {
			e, ok := p.(error)
			if !ok {
				panic(p)
			}
			err = e
		}
	}()
	if chamfer {
		g = d.Features.ChamferSolid([]SolidEdge{edge}, r, "")
	} else {
		g = d.Features.FilletSolid([]SolidEdge{edge}, r, "")
	}
	if p := g.Problem(); p != "" {
		return g, errors.New(p)
	}
	return g, nil
}

// runOffRefused are the cases of TestFilletRunsOffOntoCylinder that are
// refused, and what the refusal says: chamfers ending on the narrowest
// band's top, the cone going on past it, whose end's sections lie too
// far off the chamfer's surface to fit; and fillets whose end only just
// runs off onto the x band's top, the end section crossing it within
// the chord tolerance of the band's side, where the top would grow by a
// sliver its mesh leaves open; and the x band's widest chamfers, which
// run off both the band's side and the cone, as on a flat top.
var runOffRefused = map[string]string{
	"chamfer, y band 4 wide, top 18 sag 1 r 1.5": "can't be fitted",
	"chamfer, y band 4 wide, top 18 sag 1 r 2":   "can't be fitted",
	"chamfer, y band 4 wide, top 18 sag 1 r 2.5": "can't be fitted",
	"x band 6 wide, top 20 sag -1 r 1.5":         "only just runs off",
	"x band 6 wide, top 20 sag 1 r 1.5":          "only just runs off",
	"x band 6 wide, top 21 sag 1 r 2":            "only just runs off",
	"chamfer, x band 6 wide, top 20 sag 1 r 2.5": "runs off the faces on both sides",
	"chamfer, x band 6 wide, top 21 sag 1 r 2.5": "runs off the faces on both sides",
}

// Fillets and chamfers of the band's edge on the cone whose end meets
// the band's top, a cylinder (roadmap: run-offs onto a curved face). On
// the bands 2.5 and 3 wide at height 22, and on the x bands, the end
// runs off the band's side past the top: the top's surface is carried on
// past its edge with the side, it grows over the end, and the blend is
// cut where it meets that surface; the end face and the top share the
// line where the top's surface meets the end's plane, a ruling of the y
// bands' top, an arc of the x bands'. On the band 2 wide at 18 the edge
// ends on the top itself, with the cone going on past it: the top grows
// the same way, up to where the cone meets its surface carried on.
// Every blend not in runOffRefused is sound; every fillet adds what
// OpenCASCADE's makeFillet adds within 1%, at 200 segments a curve, as
// for the flat top (TestChamferRunsOffBand), and where it refuses (every
// run-off but the y bands' at 18) ours adds no less than at the smaller
// radius. Chamfers are made at one sag each.
func TestFilletRunsOffOntoCylinder(t *testing.T) {
	t.Parallel()
	cases := readRunOffOracle(t)
	if testing.Short() || fixtures.RaceDetector {
		// One case that runs off onto the cylinder: the grid takes half a
		// minute, ten times that under the race detector.
		cases = slices.DeleteFunc(cases, func(o runOffOracle) bool {
			return o.kind != "y" || o.y != 2.5 || o.sag != 1 || o.r != 2
		})
	}
	prev := map[string]float64{}
	for _, o := range cases {
		for _, chamfer := range []bool{false, true} {
			if chamfer && o.sag != 1 {
				continue
			}
			series := fmt.Sprintf("%s band %g wide, top %g sag %g", o.kind, 2*o.y, o.top, o.sag)
			name := fmt.Sprintf("%s r %g", series, o.r)
			if chamfer {
				name, series = "chamfer, "+name, "chamfer, "+series
			}
			d := New()
			d.Edit.SetChordTolerance(0.003)
			d.Edit.SetMaxSegments(200)
			edge, before := curvedBandOnCone(d, o.kind, o.y, o.top, o.sag)
			g, err := blendOrRefusal(d, edge, o.r, chamfer)
			if want, ok := runOffRefused[name]; ok {
				if err == nil || !strings.Contains(err.Error(), want) {
					t.Errorf("%s: %v, want a refusal saying %q", name, err, want)
				}
				continue
			}
			if err != nil {
				t.Errorf("%s: %v", name, err)
				continue
			}
			rep := g.Analyze().Check()
			if !rep.Sound() {
				t.Errorf("%s: %s", name, rep.Summary())
				continue
			}
			added := rep.Volume - before
			if added <= prev[series] {
				t.Errorf("%s added %g, no more than at a smaller size, %g", name, added, prev[series])
			}
			prev[series] = added
			if chamfer || o.refused {
				continue
			}
			if occ := o.after - o.before; math.Abs(added-occ) > 0.01*occ {
				t.Errorf("%s added %g, OpenCASCADE's %g", name, added, occ)
			}
		}
	}
}
