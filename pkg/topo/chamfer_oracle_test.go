package topo_test

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/check"
	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/internal/sweep"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/topo"
)

// faceIDAt is the identity of sh's flat face through q with normal n,
// either way round.
func faceIDAt(sh *srf.SShell, q, n geom.Vector) srf.FaceID {
	for _, s := range sh.Surface {
		m := s.NormalAt(0, 0).WithMagnitude(1)
		if math.Abs(math.Abs(m.Dot(n))-1) < 1e-9 && math.Abs(q.Minus(s.Ctrl[0][0]).Dot(m)) < 1e-9 {
			return s.Face
		}
	}
	return 0
}

// cornerNotch is the 20 cube with the cube of 10 at its far corner taken
// away, at the tolerance tol: three concave edges meet at (10, 10, 10).
func cornerNotch(tol srf.Tolerance) *srf.SShell {
	var a, b srf.SShell
	a.Tol, b.Tol = tol, tol
	a.MakePrism([]geom.Vector{v(0, 0, 0), v(20, 0, 0), v(20, 20, 0), v(0, 20, 0)}, v(0, 0, 0), v(0, 0, 20), core.NewRGBA(100, 100, 100, 255))
	b.MakePrism([]geom.Vector{v(10, 10, 10), v(30, 10, 10), v(30, 30, 10), v(10, 30, 10)}, v(0, 0, 0), v(0, 0, 20), core.NewRGBA(100, 100, 100, 255))
	out := &srf.SShell{Tol: tol}
	out.MakeFromDifferenceOf(&a, &b)
	for i := range out.Surface {
		out.Surface[i].Face = srf.FaceID(i + 1)
	}
	return out
}

// chamferOracleCase builds the shape a line of
// testdata/topo/chamfer-corners.txt names and chamfers its corner as
// the line says, at the tolerance tol.
func chamferOracleCase(shape string, d []float64, tol srf.Tolerance) (*srf.SShell, error) {
	var sh *srf.SShell
	var picks []topo.FilletPick
	switch shape {
	case "box", "uneven":
		sh = box(tol)
		c := v(10, 20, 30)
		z := topo.FilletPick{A: c, B: v(10, 20, 0), R: d[0]}
		if shape == "uneven" {
			// The first distance on the face square to x.
			z.D2, z.First = d[1], faceIDAt(sh, c, v(1, 0, 0))
			d = d[1:]
		}
		picks = []topo.FilletPick{z, {A: c, B: v(0, 20, 30), R: d[1]}, {A: c, B: v(10, 0, 30), R: d[2]}}
	case "apex":
		apex := v(10, 10, 10)
		var err error
		if sh, _, err = pyramid([]geom.Vector{v(0, 0, 0), v(0, 20, 0), v(20, 20, 0), v(20, 0, 0)}, apex, tol); err != nil {
			return nil, err
		}
		for i, b := range []geom.Vector{v(0, 0, 0), v(20, 0, 0), v(20, 20, 0), v(0, 20, 0)} {
			picks = append(picks, topo.FilletPick{A: apex, B: b, R: d[i]})
		}
	case "prism":
		th := d[0] * math.Pi / 180
		sh = trianglePrism(th, tol)
		picks = []topo.FilletPick{
			{A: v(0, 0, 0), B: v(20, 0, 0), R: d[1]},
			{A: v(0, 0, 0), B: v(20*math.Cos(th), 20*math.Sin(th), 0), R: d[2]},
			{A: v(0, 0, 0), B: v(0, 0, 20), R: d[3]}}
	case "notch":
		sh = cornerNotch(tol)
		c := v(10, 10, 10)
		picks = []topo.FilletPick{{A: c, B: v(20, 10, 10), R: d[0]}, {A: c, B: v(10, 20, 10), R: d[1]}, {A: c, B: v(10, 10, 20), R: d[2]}}
	}
	return sh, topo.FilletPicks(sh, picks, 1, 1<<30, true)
}

// TestChamferCornerSweep chamfers a box's corner of three distances, a
// pyramid's apex of d1, d2, d3, d2, and the foot of a prism's upright
// edge at angles from 45 to 135 degrees, over distances no more than 4
// to 1 apart, and the box's corner with two distances on its upright
// edge: each made, sound and closed. The rate allowed, 0, was fixed
// before the sweep first ran.
func TestChamferCornerSweep(t *testing.T) {
	t.Parallel()
	grid := sweep.Grid{
		Parallel: true,
		Axes: []sweep.Axis{
			sweep.Strings("corner", "box", "uneven", "apex", "prism45", "prism75", "prism105", "prism135"),
			sweep.Floats("d1", 0.5, 1.25, 2),
			sweep.Floats("d2", 0.5, 1.25, 2),
			sweep.Floats("d3", 0.5, 2),
		},
		MaxFailRate: 0,
		Sample:      24,
	}
	grid.Run(func(p sweep.Point) error {
		d1, d2, d3 := p.Float("d1"), p.Float("d2"), p.Float("d3")
		shape, d := p.String("corner"), []float64{d1, d2, d3}
		switch {
		case shape == "uneven":
			d = []float64{d1, d2, d3, d1}
		case shape == "apex":
			d = []float64{d1, d2, d3, d2}
		case strings.HasPrefix(shape, "prism"):
			deg, _ := strconv.ParseFloat(strings.TrimPrefix(shape, "prism"), 64)
			shape, d = "prism", []float64{deg, d1, d2, d3}
		}
		sh, err := chamferOracleCase(shape, d, srf.Tol(0.002, 20))
		if err != nil {
			return err
		}
		var m srf.SMesh
		sh.TriangulateInto(&m)
		if rep := check.Solid(sh, &m); !rep.Sound() {
			return fmt.Errorf("unsound: %s", rep.String())
		}
		return nil
	}).Require(t)
}

// cornerPoints are the corners of the faces of tp whose every vertex
// lies within reach of c: the faces that fill a chamfered corner, as
// scripts/chamfer-setback-fixtures.py finds OpenCASCADE's.
func cornerPoints(tp *topo.Topology, c geom.Vector, reach float64) []geom.Vector {
	var out []geom.Vector
	for f := range tp.Faces {
		var vs []geom.Vector
		near := true
		for _, x := range tp.FaceEdges(f) {
			for _, w := range tp.Edges[x].V {
				q := tp.Vertices[w].P
				near = near && q.Minus(c).Magnitude() < reach
				vs = append(vs, q)
			}
		}
		if !near {
			continue
		}
		for _, q := range vs {
			if !slices.ContainsFunc(out, func(o geom.Vector) bool { return o.Minus(q).Magnitude() < 1e-5 }) {
				out = append(out, q)
			}
		}
	}
	return out
}

// samePoints reports whether a and b hold the same points, within 1e-5.
func samePoints(a, b []geom.Vector) bool {
	if len(a) != len(b) {
		return false
	}
	for _, p := range a {
		if !slices.ContainsFunc(b, func(q geom.Vector) bool { return q.Minus(p).Magnitude() < 1e-5 }) {
			return false
		}
	}
	return true
}

// chamferOracleCorner is the corner each shape of
// testdata/topo/chamfer-corners.txt chamfers.
var chamferOracleCorner = map[string]geom.Vector{
	"box": v(10, 20, 30), "uneven": v(10, 20, 30), "apex": v(10, 10, 10), "prism": v(0, 0, 0), "notch": v(10, 10, 10),
}

// TestChamferCornerOracle chamfers the corners of
// testdata/topo/chamfer-corners.txt, written by
// scripts/chamfer-setback-fixtures.py from OpenCASCADE's makeChamfer:
// a box's corner of three distances, two on one edge, a pyramid's apex
// of four, the foot of a prism's upright edge whose faces don't meet
// square, and a concave corner. Each must be sound and of OpenCASCADE's
// count of faces. Where the face that fills the corner has
// OpenCASCADE's corners, the volume must be its, within what the
// bilinear patches' meshes stray at this chord tolerance.
//
// OpenCASCADE parts from that in two ways, counted here and held to
// what they were when the fixtures were first compared, 2026-10-28. At
// a corner of three it makes the triangle of the points where the
// strips' lines cross wherever one of them lies close enough to the
// line between the others (DistDebFin < 0.3 DistMin in
// ChFi3d_ChBuilder_C3.cxx, whose comment calls it a threshold effect),
// where the corner here keeps its pivot's cuts: 9 of the 104 lines,
// held to within half a cubic millimetre. At a corner of four it fills the hole
// with a plate that bulges out of the plane of its corners, where here
// it is that plane, or the bilinear patch: the same corners, a little
// less volume.
func TestChamferCornerOracle(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "topo", "chamfer-corners.txt"))
	if err != nil {
		t.Fatal(err)
	}
	triangles, plates := 0, 0
	for line := range strings.SplitSeq(strings.TrimSpace(string(data)), "\n") {
		head, corners, _ := strings.Cut(line, "|")
		f := strings.Fields(head)
		var d []float64
		for _, x := range f[1 : len(f)-2] {
			y, err := strconv.ParseFloat(x, 64)
			if err != nil {
				t.Fatalf("%q: %v", line, err)
			}
			d = append(d, y)
		}
		want, _ := strconv.ParseFloat(f[len(f)-2], 64)
		faces, _ := strconv.Atoi(f[len(f)-1])
		var occ []geom.Vector
		for c := range strings.FieldsSeq(corners) {
			var p geom.Vector
			if _, err := fmt.Sscanf(c, "%g,%g,%g", &p.X, &p.Y, &p.Z); err != nil {
				t.Fatalf("%q: %v", line, err)
			}
			occ = append(occ, p)
		}
		sh, err := chamferOracleCase(f[0], d, srf.Tol(0.002, 20))
		if err != nil {
			t.Errorf("%s: %v", head, err)
			continue
		}
		got := sound(t, sh)
		tp := build(t, sh)
		if len(tp.Faces) != faces {
			t.Errorf("%s: %d faces, OpenCASCADE's %d", head, len(tp.Faces), faces)
		}
		dmax := slices.Max(d)
		if f[0] == "prism" {
			dmax = slices.Max(d[1:])
		}
		ours := cornerPoints(tp, chamferOracleCorner[f[0]], 3*dmax+0.5)
		switch {
		case f[0] == "apex":
			if !samePoints(ours, occ) || got > want || got < want-0.003*want {
				t.Errorf("%s: corners %v, volume %.6f, %+.3g: OpenCASCADE's plate has the same corners and a little more", head, ours, got, got-want)
			}
			plates++
		case math.Abs(got-want) <= 0.005:
			// OpenCASCADE's volume; where the corners differ, two strips of
			// the largest distance could each be the pivot, mirror images.
		case len(occ) == 3 && !samePoints(ours, occ) && math.Abs(got-want) <= 0.5:
			t.Logf("%s: OpenCASCADE's triangle, %+.3g", head, got-want)
			triangles++
		default:
			t.Errorf("%s: corners %v, OpenCASCADE's %v; volume %.6f, %+.3g", head, ours, occ, got, got-want)
		}
	}
	if triangles > 9 || plates != 7 {
		t.Errorf("%d of OpenCASCADE's triangles, 9 when this was written; %d apexes, 7", triangles, plates)
	}
}
