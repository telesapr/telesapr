package slvs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/internal/golden"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// TestExportStl exports the mesh of every fixture that has one, at a
// 0.1 mm chord tolerance, and requires the golden STL file
// (testdata/export/stl, named after the fixture's path; first written
// by SolveSpace solvespace-cli export-mesh --chord-tol 0.1).
func TestExportStl(t *testing.T) {
	goldens, err := filepath.Glob(fixture("export/stl/*.stl"))
	if err != nil || len(goldens) == 0 {
		t.Fatalf("no goldens: %v", err)
	}
	dir := t.TempDir()
	for _, goldenPath := range goldens {
		name := strings.TrimSuffix(filepath.Base(goldenPath), ".stl")
		// constraint_pt_on_face_normal -> constraint/pt_on_face/normal.slvs
		i := strings.IndexByte(name, '_')
		j := strings.LastIndexByte(name, '_')
		src := name[:i] + "/" + name[i+1:j] + "/" + name[j+1:] + ".slvs"
		t.Run(src, func(t *testing.T) {
			// As in a fresh SolveSpace process.
			doc, err := LoadFile(fixture(src))
			if err != nil {
				t.Fatal(err)
			}
			sk := doc.Sketch
			// SolveSpace AfterNewFile.
			sk.GenerateAllTyped(sketch.GenerateAll)

			out := filepath.Join(dir, name+".stl")
			es := sketch.ExportSettings{ChordTol: 0.1, MaxSegments: 64, Scale: 1}
			if err := sk.ExportMeshTo(out, es); err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(out)
			if err != nil {
				t.Fatal(err)
			}
			golden.Check(t, goldenPath, got)
		})
	}
}

// TestExportStlAngleTolerance exports a curved fixture with and without
// an angular tolerance: the angle adds facets, and 0 adds none.
func TestExportStlAngleTolerance(t *testing.T) {
	doc, err := LoadFile(fixture("constraint/pt_on_face/normal.slvs"))
	if err != nil {
		t.Fatal(err)
	}
	sk := doc.Sketch
	sk.GenerateAllTyped(sketch.GenerateAll)
	dir := t.TempDir()
	count := func(angle float64) int {
		out := filepath.Join(dir, "angle.stl")
		es := sketch.ExportSettings{ChordTol: 0.1, MaxSegments: 64, Scale: 1, AngleTol: angle}
		if err := sk.ExportMeshTo(out, es); err != nil {
			t.Fatal(err)
		}
		b, err := os.ReadFile(out)
		if err != nil {
			t.Fatal(err)
		}
		return (len(b) - 84) / 50
	}
	plain, angled := count(0), count(10)
	if angled <= plain {
		t.Errorf("%d triangles at an angle of 10, %d at 0", angled, plain)
	}
	if again := count(0); again != plain {
		t.Errorf("%d triangles at 0 after the angle, %d before", again, plain)
	}
}

// TestExportMeshToErrors checks the errors of a sketch without a mesh
// and of a format that isn't STL.
func TestExportMeshToErrors(t *testing.T) {
	doc, err := LoadFile(fixture("request/line_segment/normal.slvs"))
	if err != nil {
		t.Fatal(err)
	}
	es := sketch.ExportSettings{ChordTol: 0.1, MaxSegments: 64, Scale: 1}
	out := filepath.Join(t.TempDir(), "out.stl")
	if err := doc.Sketch.ExportMeshTo(out, es); err == nil ||
		!strings.Contains(err.Error(), "nothing to export") {
		t.Errorf("empty mesh: %v", err)
	}
	if doc.Sketch.Exporting.IsSet() {
		t.Errorf("export mode left on")
	}

	doc, err = LoadFile(fixture("group/boolean_knife_edge/normal.slvs"))
	if err != nil {
		t.Fatal(err)
	}
	doc.Sketch.GenerateAllTyped(sketch.GenerateAll)
	if err := doc.Sketch.ExportMeshTo(filepath.Join(t.TempDir(), "out.xyz"), es); err == nil ||
		!strings.Contains(err.Error(), "can't identify output file type") {
		t.Errorf("unknown format: %v", err)
	}
}
