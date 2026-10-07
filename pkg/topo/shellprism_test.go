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

// roundedBox is a w x h x high box about the origin whose upright edges
// are rounded to radius r.
func roundedBox(w, h, r, high float64) *srf.SShell {
	l := roundedLoop(w, h, r)
	var sh srf.SShell
	var ls srf.SBezierLoopSet
	ls.Outer.Curves = l
	ls.Normal = v(0, 0, -1)
	sh.MakeFromExtrusionOf(&ls, v(0, 0, 0), v(0, 0, high), core.NewRGBA(100, 100, 100, 255))
	return &sh
}

// roundedArea is the area of a w x h rectangle with corners of radius r.
func roundedArea(w, h, r float64) float64 { return w*h - (4-math.Pi)*r*r }

// Prisms with curved sides, hollowed (roadmap 15.2): the inside is the
// end's outline offset by the thickness, arcs to smaller or larger
// radii, extruded between the ends moved in.
func TestShellCurvedSides(t *testing.T) {
	old := srf.ChordTolMm()
	defer srf.SetChordTolMm(old)
	srf.SetChordTolMm(0.001)
	top, bottom := v(0, 0, 1), v(0, 0, -1)
	cases := []struct {
		name  string
		sh    *srf.SShell
		open  []geom.Vector // a point on each open end, with its normal
		thick float64
		want  float64
		area  float64 // of the surfaces, for the chords' tolerance
	}{
		{"cup", cylinder(5, 10), []geom.Vector{v(0, 0, 10), top}, 1,
			math.Pi * (25*10 - 16*9), 2 * math.Pi * 5 * 20},
		{"tube", cylinder(5, 10), []geom.Vector{v(0, 0, 10), top, v(0, 0, 0), bottom}, 1.5,
			math.Pi * (25 - 3.5*3.5) * 10, 2 * math.Pi * 5 * 20},
		{"plate with a hole", plateWithHole(5, 10), []geom.Vector{v(19, 19, 10), top}, 1,
			(1600-25*math.Pi)*10 - (38*38-36*math.Pi)*9, 2 * math.Pi * 6 * 20},
		{"rounded box", roundedBox(20, 10, 3, 10), []geom.Vector{v(0, 0, 10), top}, 1,
			roundedArea(20, 10, 3)*10 - roundedArea(18, 8, 2)*9, 2 * math.Pi * 3 * 20},
		{"rounded box closed at the top", roundedBox(20, 10, 3, 10), []geom.Vector{v(0, 0, 0), bottom}, 2,
			roundedArea(20, 10, 3)*10 - roundedArea(16, 6, 1)*8, 2 * math.Pi * 3 * 20},
		// Walls thicker than the corners' radius: the inside's corners
		// are sharp.
		{"rounded box, thick walls", roundedBox(20, 10, 1, 10), []geom.Vector{v(0, 0, 10), top}, 2,
			roundedArea(20, 10, 1)*10 - 16*6*8, 2 * math.Pi * 1 * 20},
	}
	for _, c := range cases {
		tp := build(t, c.sh)
		var open []int
		for i := 0; i < len(c.open); i += 2 {
			open = append(open, faceAt(t, tp, c.open[i], c.open[i+1]))
		}
		out, err := topo.ShellSolid(c.sh, open, c.thick, 1<<30)
		if err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		if got := sound(t, out); math.Abs(got-c.want) > 0.001*c.area+1e-6 {
			t.Errorf("%s: volume %g, want %g", c.name, got, c.want)
		}
	}
}

func TestShellCurvedRefusals(t *testing.T) {
	cyl := cylinder(5, 10)
	ct := build(t, cyl)
	for name, c := range map[string]struct {
		sh    *srf.SShell
		open  func(*topo.Topology) []int
		thick float64
		want  string
	}{
		"walls past the axis": {cylinder(5, 10), func(tp *topo.Topology) []int {
			return []int{faceAt(t, tp, v(0, 0, 10), v(0, 0, 1))}
		}, 5, "radius"},
		"bottom meets top": {cylinder(5, 2), func(tp *topo.Topology) []int {
			return []int{faceAt(t, tp, v(0, 0, 0), v(0, 0, -1))}
		}, 2, "meet"},
		"a cone": {frustum(5, 3, 10), func(tp *topo.Topology) []int {
			return []int{faceAt(t, tp, v(0, 0, 10), v(0, 0, 1))}
		}, 1, "curved"},
	} {
		tp := build(t, c.sh)
		var se *topo.ShellError
		_, err := topo.ShellSolid(c.sh, c.open(tp), c.thick, 1<<30)
		if !errors.As(err, &se) || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: %v, want a shell error saying %q", name, err, c.want)
		}
	}
	// A side taken away: open only at the ends.
	sides := 0
	for f := range ct.Faces {
		_, err := topo.ShellSolid(cyl, []int{f}, 1, 1<<30)
		if err != nil && strings.Contains(err.Error(), "only at its ends") {
			sides++
		}
	}
	if sides != len(ct.Faces)-2 {
		t.Errorf("%d of the %d sides refused when taken away", sides, len(ct.Faces)-2)
	}
}

// Prisms with curved sides, of several shapes and heights, hollowed to
// walls of several thicknesses, open at one end or both.
func TestShellCurvedSweep(t *testing.T) {
	old := srf.ChordTolMm()
	defer srf.SetChordTolMm(old)
	srf.SetChordTolMm(0.001)
	grid := sweep.Grid{
		Axes: []sweep.Axis{
			sweep.Strings("shape", "cylinder", "rounded box", "sharp inside", "plate with a hole"),
			sweep.Strings("open", "top", "bottom", "both ends"),
			sweep.Floats("k", 0.05, 0.2, 0.4),
			sweep.Floats("height", 3, 15, 60),
		},
		MaxFailRate: 0,
		Sample:      20,
	}
	grid.Run(func(p sweep.Point) error {
		h := p.Float("height")
		// The thickness as a part of the narrowest the shape gets: 10
		// across the rounded boxes and the cylinder, 15 between the
		// hole and the plate's side, and the height.
		thick := p.Float("k") * math.Min(10, h)
		var sh *srf.SShell
		var area func(d float64) float64 // of the inside's outline
		var full, round float64          // round: the length of the arcs, inside and out
		switch p.String("shape") {
		case "cylinder":
			sh = cylinder(5, h)
			full = 25 * math.Pi
			area = func(d float64) float64 { return (5 - d) * (5 - d) * math.Pi }
			round = 2 * math.Pi * 10
		case "rounded box":
			sh = roundedBox(20, 10, 4, h)
			full = roundedArea(20, 10, 4)
			area = func(d float64) float64 { return roundedArea(20-2*d, 10-2*d, 4-d) }
			round = 2 * math.Pi * 8
		case "sharp inside":
			sh = roundedBox(20, 10, 0.5, h)
			full = roundedArea(20, 10, 0.5)
			area = func(d float64) float64 { return roundedArea(20-2*d, 10-2*d, math.Max(0, 0.5-d)) }
			round = 2 * math.Pi
		default:
			sh = plateWithHole(5, h)
			full = 1600 - 25*math.Pi
			area = func(d float64) float64 { return (40-2*d)*(40-2*d) - (5+d)*(5+d)*math.Pi }
			round = 2 * math.Pi * 12
		}
		tp, err := topo.Build(sh)
		if err != nil {
			return err
		}
		var open []int
		inside := h
		switch p.String("open") {
		case "top":
			open, inside = []int{faceOn(tp, h)}, h-thick
		case "bottom":
			open, inside = []int{faceOn(tp, 0)}, h-thick
		default:
			open = []int{faceOn(tp, h), faceOn(tp, 0)}
		}
		out, err := topo.ShellSolid(sh, open, thick, 1<<30)
		if err != nil {
			return err
		}
		var m srf.SMesh
		out.TriangulateInto(&m)
		r := check.Solid(out, &m)
		want := full*h - area(thick)*inside
		if !r.Sound() {
			return fmt.Errorf("unsound: %s", r.String())
		}
		// The chords cut inside the round walls by up to the tolerance.
		if math.Abs(r.Volume-want) > 0.001*round*h+1e-6*want {
			return fmt.Errorf("volume %g, want %g", r.Volume, want)
		}
		return nil
	}).Require(t)
}

// faceOn is the face of a prism along z, its end at height z.
func faceOn(tp *topo.Topology, z float64) int {
	for f := range tp.Faces {
		ok := true
		for _, ce := range tp.Loops[tp.Faces[f].Loops[0]].CoEdges {
			for _, p := range tp.Edges[tp.CoEdges[ce].Edge].Points {
				ok = ok && math.Abs(p.Z-z) < 1e-9
			}
		}
		if ok {
			return f
		}
	}
	return -1
}
