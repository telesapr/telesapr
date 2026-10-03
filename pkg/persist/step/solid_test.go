package step

import (
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/telesapr/telesapr/pkg/check"
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
