package slvs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spacesolve/spacesolve/pkg/internal/golden"
	"github.com/spacesolve/spacesolve/pkg/sketch"
	"github.com/spacesolve/spacesolve/pkg/srf"
)

// TestExportStl exports the mesh of every fixture that has one, at a
// 0.1 mm chord tolerance, and requires the golden STL file
// (testdata/export/stl, named after the fixture's path; first written
// by C++ solvespace-cli export-mesh --chord-tol 0.1).
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
			// As in a fresh C++ process.
			srf.SetChordTolMm(0)
			doc, err := LoadFile(fixture(src))
			if err != nil {
				t.Fatal(err)
			}
			sk := doc.Sketch
			// C++ AfterNewFile.
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
	if srf.ExportMode() {
		t.Errorf("export mode left on")
	}

	doc, err = LoadFile(fixture("group/boolean_knife_edge/normal.slvs"))
	if err != nil {
		t.Fatal(err)
	}
	doc.Sketch.GenerateAllTyped(sketch.GenerateAll)
	if err := doc.Sketch.ExportMeshTo(filepath.Join(t.TempDir(), "out.xyz"), es); err == nil ||
		!strings.Contains(err.Error(), "Can't identify output file type") {
		t.Errorf("unknown format: %v", err)
	}
}
