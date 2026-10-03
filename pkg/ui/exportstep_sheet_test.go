package ui

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/check"
	"github.com/telesapr/telesapr/pkg/persist/step"
	"github.com/telesapr/telesapr/pkg/topo"
)

// TestExportStepSheets writes solids whose faces are several patches
// to STEP, a face as one B-spline surface (step 11.4), and reads them
// back with the STEP importer: one face per face of the solid, none
// left out or with bounds that cross, a closed mesh, and the solid's
// volume. TELESAPR_STEP_OUT=dir keeps the files, written both ways,
// for checking in another reader, as FreeCAD.
func TestExportStepSheets(t *testing.T) {
	for _, model := range []string{
		"topo/boss.tlspr", "topo/frustum.tlspr", "solid/fillet-circle.tlspr", "solid/sweep-pipe.tlspr",
		"solid/fillet-cone.tlspr", "solid/chamfer-circle-and-edge.tlspr",
		"solid/fillet-boss-foot.tlspr", "solid/sweep-ring-down.tlspr",
	} {
		t.Run(model, func(t *testing.T) {
			r := newUIScriptRunner(t)
			r.step("file $testdata/" + model)
			r.step("menu File|Open...")
			ss := r.ss
			g := ss.SK.GetGroup(ss.GW.activeGroupH())
			tp, err := topo.Build(&g.RunningShell)
			if err != nil {
				t.Fatal(err)
			}
			multi := 0
			for s := range tp.Sheets {
				if len(tp.Sheets[s].Faces) > 1 {
					multi++
				}
			}
			if multi == 0 {
				t.Fatal("no face of several patches")
			}
			rep := check.Group(ss.SK, g)
			path := filepath.Join(t.TempDir(), "out.step")
			read := func() (*step.Result, string) {
				ss.ExportSurfacesTo(path)
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				res, err := step.Read(data, step.Options{ChordTol: 0.002})
				if err != nil {
					t.Fatal(err)
				}
				return res, string(data)
			}
			res, data := read()
			if d := os.Getenv("TELESAPR_STEP_OUT"); d != "" {
				if err := os.WriteFile(filepath.Join(d, strings.ReplaceAll(model, "/", "_")+".faces.step"), []byte(data), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if n := strings.Count(data, "ADVANCED_FACE("); n != len(tp.Sheets) {
				t.Errorf("%d advanced faces for %d faces of %d patches", n, len(tp.Sheets), len(tp.Faces))
			}
			if p := res.Report.Problem(); p != "" || res.Report.Faces != len(tp.Sheets) {
				t.Errorf("read back: %d faces, %q", res.Report.Faces, p)
			}
			back := check.Solid(nil, &res.Mesh)
			// The same solid written a patch at a time, as before step
			// 11.4, read back by the same reader at the same tolerance;
			// and the solid's own mesh, at its coarser one. (The reader
			// meshes the frustum written a patch at a time 7e-4 under its
			// volume, and as one face per face 2e-4 under.)
			stepOneSurfacePerFace = false
			res2, data2 := read()
			stepOneSurfacePerFace = true
			if d := os.Getenv("TELESAPR_STEP_OUT"); d != "" {
				if err := os.WriteFile(filepath.Join(d, strings.ReplaceAll(model, "/", "_")+".patches.step"), []byte(data2), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			was := check.Solid(nil, &res2.Mesh)
			if !back.Sound() || math.Abs(back.Volume-was.Volume) > 1e-3*was.Volume ||
				math.Abs(back.Volume-rep.Volume) > 1e-2*rep.Volume {
				t.Errorf("read back: %s; a patch at a time %g, the solid's mesh %g", back.String(), was.Volume, rep.Volume)
			}
		})
	}
}
