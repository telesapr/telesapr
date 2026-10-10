package step

import (
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/telesapr/telesapr/pkg/check"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/topo"
)

// readModel parses a STEP file into its model.
func readModel(t *testing.T, path string) *Model {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	f, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	return ReadModel(f)
}

// fineMesh sets a fine chord tolerance for a test: a shell's trims take
// their points at the tolerance in force when it is built.
func fineMesh(t *testing.T, tol float64) {
	old, oldSegs := srf.ChordTolMm(), srf.MaxSegments()
	srf.SetChordTolMm(tol)
	srf.SetMaxSegments(100)
	t.Cleanup(func() { srf.SetChordTolMm(old); srf.SetMaxSegments(oldSegs) })
}

// solidReport meshes a shell and checks it.
func solidReport(sh *srf.SShell) check.Report {
	var m srf.SMesh
	sh.TriangulateInto(&m)
	return check.Solid(sh, &m)
}

// TestSolidShapes reads each shape of the analytic set as a solid: it
// is sound, and its volume within the mesh's tolerance of the exact one.
func TestSolidShapes(t *testing.T) {
	const tol = 0.005
	fineMesh(t, tol)
	for _, e := range readShapes(t) {
		m := readModel(t, filepath.Join(shapesDir, e.name+".step"))
		sh, err := m.ShellOf(0, srf.OriginFor(9, 0), srf.Tolerance{})
		if err != nil {
			t.Errorf("%s: %v", e.name, err)
			continue
		}
		r := solidReport(sh)
		allow := e.area*tol + 1e-9*e.volume
		if !r.Sound() || math.Abs(r.Volume-e.volume) > allow {
			t.Errorf("%s: %s, want volume %.6g within %.3g", e.name, r.String(), e.volume, allow)
			for _, p := range r.Topology {
				t.Logf("  %s at %v", p.Msg, p.At)
			}
		}
	}
}

// TestSolidHighDegree reads shapes whose curves and surfaces are of a
// degree above 5 as solids (package 27.2): a lid of degree 7 by 6 and a
// surface of revolution of a curve of degree 8, written at degree 5
// within lowDegreeTol. Each is sound and its volume the exact one
// within the mesh's tolerance; read as a mesh, the same.
func TestSolidHighDegree(t *testing.T) {
	const tol = 0.005
	fineMesh(t, tol)
	dir := filepath.Join("..", "..", "..", "testdata", "step", "degree")
	for _, e := range readExpected(t, dir) {
		path := filepath.Join(dir, e.name+".step")
		m := readModel(t, path)
		sh, err := m.ShellOf(0, srf.OriginFor(9, 0), srf.Tolerance{})
		if err != nil {
			t.Errorf("%s: %v", e.name, err)
			continue
		}
		r := solidReport(sh)
		allow := e.area*tol + 1e-9*e.volume
		if !r.Sound() || math.Abs(r.Volume-e.volume) > allow {
			t.Errorf("%s: %s, want volume %.6g within %.3g", e.name, r.String(), e.volume, allow)
		}
		res, err := ReadFile(path, Options{ChordTol: tol})
		if err != nil {
			t.Fatal(err)
		}
		mr := check.Solid(nil, &res.Mesh)
		if !mr.Sound() || math.Abs(mr.Volume-e.volume) > allow {
			t.Errorf("%s as a mesh: %s, want volume %.6g within %.3g", e.name, mr.String(), e.volume, allow)
		}
	}
}

// TestSolidOffsetsConics reads shapes with offset surfaces, and with
// edges and extrusions of parabolas and hyperbolas, as solids (package
// 27.5): offsets of a cylinder and a sphere, read as the cylinder and
// sphere they are, and segments of a parabola and a hyperbola extruded,
// their curves exact Beziers. Each is sound and its volume the expected one
// within the mesh's tolerance.
func TestSolidOffsetsConics(t *testing.T) {
	const tol = 0.005
	fineMesh(t, tol)
	dir := filepath.Join("..", "..", "..", "testdata", "step", "offset")
	for _, e := range readExpected(t, dir) {
		m := readModel(t, filepath.Join(dir, e.name+".step"))
		sh, err := m.ShellOf(0, srf.OriginFor(9, 0), srf.Tolerance{})
		if err != nil {
			t.Errorf("%s: %v", e.name, err)
			continue
		}
		r := solidReport(sh)
		allow := e.area*tol + 1e-9*e.volume
		if !r.Sound() || math.Abs(r.Volume-e.volume) > allow {
			t.Errorf("%s: %s, want volume %.6g within %.3g", e.name, r.String(), e.volume, allow)
		}
	}
}

// TestOffsetCells lays an offset of a surface of no kind it can turn
// into, an extruded B-spline outline with a tight bend, out in patches
// of degree 5 (package 27.5): its lines refined where the base's are too
// far apart for one patch, each patch within lowDegreeTol of the offset
// at points between its own, and neighbours sharing their sides. A base
// that closes with a crease has an offset that doesn't.
func TestOffsetCells(t *testing.T) {
	pts := []geom.Vector{{X: 0, Y: 0}, {X: 5, Y: 3}, {X: 10, Y: 0}, {X: 12, Y: 6}, {X: 8, Y: 10}, {X: 2, Y: 9}, {X: 0, Y: 0}}
	knots := []float64{0, 0, 0, 0, 1, 2, 3, 4, 4, 4, 4}
	ws := make([]float64, len(pts))
	for i := range ws {
		ws[i] = 1
	}
	c, err := newBsplineCurve(3, knots, pts, ws)
	if err != nil {
		t.Fatal(err)
	}
	s := &offset{base: &extrusion{c: c, d: geom.Vector{Z: 1}}, d: 0.8}
	if pu, _ := s.Period(); pu != 0 {
		if bu, _ := s.base.Period(); bu > 0 {
			t.Errorf("an offset of an outline closed with a crease closes, period %g", pu)
		}
	}
	cells, err := offsetCells(s)
	if err != nil {
		t.Fatal(err)
	}
	us := append(append([]float64{0}, cells.breaksU(0, 4)...), 4)
	vs := append(append([]float64{0}, cells.breaksV(0, 5)...), 5)
	if len(us) <= 5 {
		t.Errorf("the base's lines not refined: %v", us)
	}
	if len(vs) != 2 {
		t.Errorf("lines along a straight extrusion: %v", vs)
	}
	var prev *srf.SSurface
	for i := 0; i+1 < len(us); i++ {
		p, err := cells.patch(us[i], us[i+1], vs[0], vs[1])
		if err != nil {
			t.Fatalf("cell %g..%g: %v", us[i], us[i+1], err)
		}
		for k := range 21 {
			for m := range 5 {
				x, y := float64(k)/20, float64(m)/4
				want := s.Eval(us[i]+(us[i+1]-us[i])*x, vs[0]+(vs[1]-vs[0])*y)
				if d := p.PointAt(x, y).Minus(want).Magnitude(); d > lowDegreeTol {
					t.Errorf("cell %g..%g: %g off at (%g, %g)", us[i], us[i+1], d, x, y)
				}
			}
		}
		if prev != nil {
			a, b := prev.EdgeCurve(1), p.EdgeCurve(3)
			for k := 0; k <= b.Degree; k++ {
				if !a.Ctrl[k].Equals(b.Ctrl[k]) {
					t.Errorf("cells at %g don't share their side", us[i])
					break
				}
			}
		}
		prev = &p
	}
}

// solidCorpusSound are the corpus files whose shapes read as sound
// solids (2026-10-04); the others fall back to meshes: Schenkel's mesh
// and shell disagree on its Euler characteristic, and NIST_MBE_PMI_6,
// which closes since package 27.3 (TestSolidSeamEdges), has a topology
// problem at its poles. screw.step reads since roadmap 24.2: its cones'
// outlines ran across their patches to the apex. NIST_MBE_PMI_5 reads
// since the mesh checker measures an edge's crossing in the plane of
// the triangle it crosses (STriangle.edgeCrossesWithin): its crossings,
// where faces meet tangentially, were edges nearly in a sliver's plane
// taken to cross it far outside it.
var solidCorpusSound = map[string]bool{
	"NIST_MBE_PMI_1.stp": true, "NIST_MBE_PMI_3.stp": true, "NIST_MBE_PMI_11.stp": true,
	"t20_data.step": true, "ap203e2_mapped_assembly.stp": true, "screw.step": true,
	"NIST_MBE_PMI_5.stp": true,
}

// TestSolidCorpus reads the vendor files as solids: each shape of those
// listed sound built, sound, and the placed shapes' volume the
// reference's within the mesh's tolerance.
func TestSolidCorpus(t *testing.T) {
	if testing.Short() {
		t.Skip("the corpus")
	}
	dir := filepath.Join("..", "..", "..", "testdata", "step", "corpus")
	sound := 0
	for _, e := range readReference(t, filepath.Join(dir, "reference.txt")) {
		if e.expect != "ok" {
			continue
		}
		m := readModel(t, filepath.Join(dir, e.name))
		tol := math.Max(1e-3, math.Cbrt(e.volume)*5e-4)
		fineMesh(t, tol)
		vol := 0.0
		var why string
		for _, pl := range m.Placements {
			sh, err := m.ShellOf(pl.Shape, srf.OriginFor(9, 0), srf.Tolerance{})
			if err != nil {
				why = err.Error()
				break
			}
			if r := solidReport(sh); !r.Sound() {
				why = r.String()
				break
			}
			sh2, _ := m.ShellOf(pl.Shape, srf.OriginFor(9, 0), srf.Tolerance{})
			vol += solidReport(sh2).Volume
		}
		allow := e.area*tol + 1e-6*e.volume
		ok := why == "" && math.Abs(vol-e.volume) <= allow
		if ok {
			sound++
		}
		switch {
		case solidCorpusSound[e.name] && !ok:
			t.Errorf("%s: not read as a sound solid: %s; volume %.6g, want %.6g within %.3g", e.name, why, vol, e.volume, allow)
		case !solidCorpusSound[e.name] && ok:
			t.Logf("%s: now read as a sound solid; list it", e.name)
		}
	}
	t.Logf("%d sound", sound)
}

// TestSolidCorpusCDT reads the vendor files as solids meshed by the
// constrained Delaunay mesher (roadmap K4): all nine are sound, at the
// reference's volume within the mesh's tolerance. The ear clipper
// leaves NIST_MBE_PMI_6 with slivers at its poles, where the polar
// chart makes a pole one point, and Schenkel's mesh off its shell's
// Euler characteristic.
func TestSolidCorpusCDT(t *testing.T) {
	if testing.Short() {
		t.Skip("the corpus")
	}
	old := srf.DefaultTolerance
	t.Cleanup(func() { srf.DefaultTolerance = old })
	dir := filepath.Join("..", "..", "..", "testdata", "step", "corpus")
	for _, e := range readReference(t, filepath.Join(dir, "reference.txt")) {
		if e.expect != "ok" {
			continue
		}
		m := readModel(t, filepath.Join(dir, e.name))
		tol := math.Max(1e-3, math.Cbrt(e.volume)*5e-4)
		fineMesh(t, tol)
		srf.SetMesher(srf.MesherCDT)
		vol := 0.0
		var why string
		for _, pl := range m.Placements {
			sh, err := m.ShellOf(pl.Shape, srf.OriginFor(9, 0), srf.Tolerance{})
			if err != nil {
				why = err.Error()
				break
			}
			r := solidReport(sh)
			if !r.Sound() {
				why = r.String()
				break
			}
			vol += r.Volume
		}
		if allow := e.area*tol + 1e-6*e.volume; why != "" || math.Abs(vol-e.volume) > allow {
			t.Errorf("%s: not read as a sound solid: %s; volume %.6g, want %.6g within %.3g", e.name, why, vol, e.volume, allow)
		}
	}
}

// TestSolidSeamEdges reads the two NIST files whose faces meet along
// another face's seam or a line of its grid (package 27.3): an edge
// along a sphere-like face's knot line, within the file's tolerance of
// it but not on it, and two halves of such a face bounded by meridians
// through its poles. They close now: no naked edges, every edge between
// two faces running it opposite ways, and the volume FreeCAD gives
// within the mesh's tolerance, and no mesh edge crossing a triangle.
func TestSolidSeamEdges(t *testing.T) {
	if testing.Short() {
		t.Skip("the corpus")
	}
	dir := filepath.Join("..", "..", "..", "testdata", "step", "corpus")
	for _, e := range readReference(t, filepath.Join(dir, "reference.txt")) {
		if e.name != "NIST_MBE_PMI_5.stp" && e.name != "NIST_MBE_PMI_6.stp" {
			continue
		}
		m := readModel(t, filepath.Join(dir, e.name))
		tol := math.Max(1e-3, math.Cbrt(e.volume)*5e-4)
		fineMesh(t, tol)
		sh, err := m.ShellOf(m.Placements[0].Shape, srf.OriginFor(9, 0), srf.Tolerance{})
		if err != nil {
			t.Errorf("%s: %v", e.name, err)
			continue
		}
		var mesh srf.SMesh
		sh.TriangulateInto(&mesh)
		r := check.Solid(sh, &mesh)
		allow := e.area*tol + 1e-6*e.volume
		// NIST_MBE_PMI_6's mesh has slivers in the meridian plane at
		// two of its three poles, where a trim runs up a meridian into
		// the pole: the mesher, out of other ears, joins the trim's
		// points to the pole, as the face beyond does, so the slivers
		// lie on each other (step 24, what is left). Since the mesher
		// passes over such ears where it has others, the third pole is
		// clean: the mesh's Euler characteristic is off by 6, was 8.
		// Neither mesh crosses itself: the two crossings each had were
		// the checker's (STriangle.edgeCrossesWithin).
		topos, euler := 0, 0
		if e.name == "NIST_MBE_PMI_6.stp" {
			topos, euler = 1, 6
		}
		if d := topo.MeshEuler(&mesh) - r.Euler; r.NakedEdges != 0 || r.SelfIntersecting != 0 || r.TopologyProblems > topos || d > euler || d < -euler ||
			math.Abs(r.Volume-e.volume) > allow {
			t.Errorf("%s: %s, mesh Euler off by %d; want volume %.6g within %.3g", e.name, r.String(), d, e.volume, allow)
		}
	}
}

// TestSolidSchenkelClosed reads Schenkel as a solid: its mesh closes and
// doesn't cross itself, at FreeCAD's volume within the mesh's tolerance.
// A wedge of a sphere from a circle of latitude up to its pole was
// turned over, its pole's parameter along the folded side being any,
// which left 40 naked edges round it. The shell's Euler characteristic
// still differs from its mesh's (-34 against -6), cause not found, so
// it is not counted sound (roadmap 27.4).
func TestSolidSchenkelClosed(t *testing.T) {
	if testing.Short() {
		t.Skip("the corpus")
	}
	dir := filepath.Join("..", "..", "..", "testdata", "step", "corpus")
	for _, e := range readReference(t, filepath.Join(dir, "reference.txt")) {
		if e.name != "Schenkel.stp" {
			continue
		}
		m := readModel(t, filepath.Join(dir, e.name))
		tol := math.Max(1e-3, math.Cbrt(e.volume)*5e-4)
		fineMesh(t, tol)
		sh, err := m.ShellOf(m.Placements[0].Shape, srf.OriginFor(9, 0), srf.Tolerance{})
		if err != nil {
			t.Fatal(err)
		}
		r := solidReport(sh)
		allow := e.area*tol + 1e-6*e.volume
		if r.NakedEdges != 0 || r.SelfIntersecting != 0 || math.Abs(r.Volume-e.volume) > allow {
			t.Errorf("%s: %s; want volume %.6g within %.3g", e.name, r.String(), e.volume, allow)
		}
	}
}

// TestTurnedLoopsAtPole: a bound through a pole of a closed surface is
// taken as the file runs it, whichever way round its layout goes.
func TestTurnedLoopsAtPole(t *testing.T) {
	half := math.Pi / 2
	// Up a meridian at u = pi to the pole, down another at u = 0: laid
	// out clockwise when the second comes out at 2 pi.
	var l []uvSample
	for k := range 5 {
		l = append(l, uvSample{u: math.Pi, v: -half + float64(k)*half/2})
	}
	for k := range 5 {
		l = append(l, uvSample{u: 2 * math.Pi, v: half - float64(k)*half/2})
	}
	pole := func(q uvSample) bool { return math.Abs(math.Abs(q.v)-half) < 1e-12 }
	none := func(uvSample) bool { return false }
	if !turnedLoops([][]uvSample{l}, 2*math.Pi, 0, none) {
		t.Error("a clockwise bound not taken as turned")
	}
	if turnedLoops([][]uvSample{l}, 2*math.Pi, 0, pole) {
		t.Error("a bound through a pole taken as turned")
	}
}

// cutters are a block through the middle of a box and a cylinder down
// it, a little off the middle: the analytic shapes are symmetric about
// their middles, where their patches meet, and a cut along a patch's
// edge is coincident contact, which the kernel doesn't take in general
// (roadmap F5).
func cutters(lo, hi geom.Vector) []*srf.SShell {
	col := DefaultColor
	d := hi.Minus(lo)
	c := lo.Plus(hi).ScaledBy(0.5).Plus(geom.Vector{X: 0.0731 * d.X, Y: 0.0419 * d.Y})
	var block srf.SShell
	x0, x1 := c.X, hi.X+d.X
	y0, y1 := lo.Y-d.Y, hi.Y+d.Y
	block.MakePrism([]geom.Vector{{X: x0, Y: y0, Z: lo.Z - d.Z}, {X: x1, Y: y0, Z: lo.Z - d.Z},
		{X: x1, Y: y1, Z: lo.Z - d.Z}, {X: x0, Y: y1, Z: lo.Z - d.Z}}, geom.Vector{}, geom.Vector{Z: 3 * d.Z}, col)
	r := 0.15 * math.Min(d.X, d.Y)
	var ls srf.SBezierLoopSet
	for k := range 4 {
		a0 := float64(k) * math.Pi / 2
		at := func(a, s float64) geom.Vector {
			return geom.Vector{X: c.X + r*s*math.Cos(a), Y: c.Y + r*s*math.Sin(a), Z: lo.Z - d.Z}
		}
		sb := srf.SBezierFrom3(at(a0, 1), at(a0+math.Pi/4, math.Sqrt2), at(a0+math.Pi/2, 1))
		sb.Weight[1] = math.Sqrt2 / 2
		ls.Outer.Curves = append(ls.Outer.Curves, sb)
	}
	// The caps lie in the plane through Point: at the loop, below the
	// shape, not at z = 0 where they would lie in a face of it.
	ls.Normal, ls.Point = geom.Vector{Z: -1}, ls.Outer.Curves[0].Start()
	var cyl srf.SShell
	cyl.MakeFromExtrusionOf(&ls, geom.Vector{}, geom.Vector{Z: 3 * d.Z}, col)
	return []*srf.SShell{&block, &cyl}
}

// D3.3: shapes read as solids, cut by a block and a cylinder: the part
// cut away and the part kept are sound and make up the whole. Rate:
// see solidBooleanMaxFail.
func TestSolidBooleans(t *testing.T) {
	if testing.Short() {
		t.Skip("a sweep")
	}
	const tol = 0.01
	fineMesh(t, tol)
	type part struct {
		name string
		sh   *srf.SShell
	}
	var parts []part
	for _, e := range readShapes(t) {
		m := readModel(t, filepath.Join(shapesDir, e.name+".step"))
		if sh, err := m.ShellOf(0, srf.OriginFor(9, 0), srf.Tolerance{}); err == nil {
			parts = append(parts, part{e.name, sh})
		}
	}
	points, failed := 0, 0
	for _, p := range parts {
		whole := solidReport(p.sh)
		var lo, hi geom.Vector
		var mm srf.SMesh
		p.sh.TriangulateInto(&mm)
		for i, tr := range mm.Triangles {
			for k, q := range []geom.Vector{tr.A, tr.B, tr.C} {
				if i == 0 && k == 0 {
					lo, hi = q, q
				}
				growBox(q, &lo, &hi)
			}
		}
		for ci, cut := range cutters(lo, hi) {
			points++
			var kept, gone srf.SShell
			kept.MakeFromDifferenceOf(p.sh, cut)
			gone.MakeFromIntersectionOf(p.sh, cut)
			rk, rg := solidReport(&kept), solidReport(&gone)
			sum := rk.Volume + rg.Volume
			allow := 2e-3 * whole.Volume
			if !rk.Sound() || !rg.Sound() || math.Abs(sum-whole.Volume) > allow {
				failed++
				t.Logf("%s, cutter %d: kept %s; cut %s; %.6g + %.6g against %.6g",
					p.name, ci, rk.String(), rg.String(), rk.Volume, rg.Volume, whole.Volume)
			}
		}
	}
	t.Logf("%d points, %d failed", points, failed)
	if failed > solidBooleanMaxFail {
		t.Errorf("%d of %d failed, more than %d", failed, points, solidBooleanMaxFail)
	}
}

// solidBooleanMaxFail is the failures allowed: none since roadmap 24.1's
// rest, where a half sphere's patch beside the cylinder's hole was
// clipped by a chord grazing the hole's rim (SContour.grazed); 1 of 28
// from 24.2 to then. At the first
// measurement (2026-10-04) 10 failed: four block cuts of spheres and a
// vase had the curved patches join two points of the block's planar
// cut by an edge lying in the block's face (splitTrimDiagonals, 24.1);
// three were this test's, the cylinder's caps lying at z = 0 in the
// shapes' faces; the cone cut through its apex had its patches'
// outlines run across them to the apex (openPoles, 24.2).
const solidBooleanMaxFail = 0

// Tessellated shapes (AP242, roadmap 21.6) read as meshes: a cube of
// triangulated faces in a tessellated solid, a tetrahedron of strips
// and fans; as solids they are refused, having no surfaces.
func TestTessellated(t *testing.T) {
	path := filepath.Join("..", "..", "..", "testdata", "step", "tessellated.step")
	res, err := ReadFile(path, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if p := res.Report.Problem(); p != "" || res.Report.Shapes != 2 {
		t.Fatalf("%d shapes, problem %q", res.Report.Shapes, p)
	}
	r := check.Solid(nil, &res.Mesh)
	if want := 1000.0 + 36; r.NakedEdges != 0 || math.Abs(r.Volume-want) > 1e-9 || len(res.Mesh.Triangles) != 16 {
		t.Errorf("%d triangles, %d naked edges, volume %g, want %g", len(res.Mesh.Triangles), r.NakedEdges, r.Volume, want)
	}
	m := readModel(t, path)
	if _, err := m.ShellOf(0, 1, srf.Tolerance{}); err == nil {
		t.Error("a tessellated shape read as a solid")
	}
}

func BenchmarkSolidNIST3(b *testing.B) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "step", "corpus", "NIST_MBE_PMI_3.stp"))
	if err != nil {
		b.Fatal(err)
	}
	for b.Loop() {
		f, err := Parse(data)
		if err != nil {
			b.Fatal(err)
		}
		if _, _, err := ReadModel(f).Solid(srf.Tolerance{}); err != nil {
			b.Fatal(err)
		}
	}
}
