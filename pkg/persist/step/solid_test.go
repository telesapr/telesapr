package step

import (
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/telesapr/telesapr/pkg/check"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
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
	old, oldSegs := srf.ChordTolMm(), srf.GetMaxSegments()
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
		sh, err := m.ShellOf(0, srf.OriginFor(9, 0))
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

// solidCorpusSound are the corpus files whose shapes read as sound
// solids (2026-10-04); the others fall back to meshes: NIST_MBE_PMI_5
// and _6 have an edge of one face along another face's seam, Schenkel
// and screw leave gaps or crossings in the mesh where edges miss their
// faces by more than the file's tolerance.
var solidCorpusSound = map[string]bool{
	"NIST_MBE_PMI_1.stp": true, "NIST_MBE_PMI_3.stp": true, "NIST_MBE_PMI_11.stp": true,
	"t20_data.step": true, "ap203e2_mapped_assembly.stp": true,
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
			sh, err := m.ShellOf(pl.Shape, srf.OriginFor(9, 0))
			if err != nil {
				why = err.Error()
				break
			}
			if r := solidReport(sh); !r.Sound() {
				why = r.String()
				break
			}
			sh2, _ := m.ShellOf(pl.Shape, srf.OriginFor(9, 0))
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
	for k := 0; k < 4; k++ {
		a0 := float64(k) * math.Pi / 2
		at := func(a, s float64) geom.Vector {
			return geom.Vector{X: c.X + r*s*math.Cos(a), Y: c.Y + r*s*math.Sin(a), Z: lo.Z - d.Z}
		}
		sb := srf.SBezierFrom3(at(a0, 1), at(a0+math.Pi/4, math.Sqrt2), at(a0+math.Pi/2, 1))
		sb.Weight[1] = math.Sqrt2 / 2
		ls.Outer.Curves = append(ls.Outer.Curves, sb)
	}
	ls.Normal = geom.Vector{Z: -1}
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
		if sh, err := m.ShellOf(0, srf.OriginFor(9, 0)); err == nil {
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

// solidBooleanMaxFail is the failures allowed: 6 of 28 since roadmap
// 24.1, all where the cylinder cuts: a cylinder through a cylinder
// (box_hole, sphere_bored), a cone through its apex (both cutters), a
// torus crossed by a cylinder, a sphere's pole inside the cutter. The
// four block cuts of a sphere and a vase that failed before had the
// curved patches join two points of the block's planar cut by an edge,
// which lay in the block's face and crossed its triangles
// (splitTrimDiagonals). It was 10 at the first measurement
// (2026-10-04).
const solidBooleanMaxFail = 6

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
	if _, err := m.ShellOf(0, 1); err == nil {
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
		if _, _, err := ReadModel(f).Solid(); err != nil {
			b.Fatal(err)
		}
	}
}
