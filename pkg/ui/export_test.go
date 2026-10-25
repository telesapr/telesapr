package ui

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/internal/golden"
	"github.com/telesapr/telesapr/pkg/platform"
	"github.com/telesapr/telesapr/pkg/platform/headless"
)

// exportFixtures are the fixtures TestExport exports.
var exportFixtures = []string{
	"constraint/angle/normal.tlspr",
	"constraint/comment/normal.tlspr",
	"constraint/diameter/normal.tlspr",
	"constraint/diameter/reference.tlspr",
	"constraint/pt_face_distance/normal.tlspr",
	"constraint/pt_line_distance/normal.tlspr",
	"constraint/pt_pt_distance/normal.tlspr",
	"group/boolean_knife_edge/normal.tlspr",
	"group/link_stl/normal.tlspr",
	"group/translate_nd/normal.tlspr",
	"request/arc_of_circle/normal.tlspr",
	"request/circle/normal.tlspr",
	"request/cubic/normal.tlspr",
	"request/cubic_periodic/normal.tlspr",
	"request/ttf_text/normal.tlspr",
}

// TestExport exports each fixture in every format and compares the
// files with the goldens (testdata/export), byte for byte.
func TestExport(t *testing.T) {
	t.Parallel()
	root := filepath.Join("..", "..", "testdata")
	for _, f := range exportFixtures {
		t.Run(f, func(t *testing.T) {
			t.Parallel()
			gotDir := t.TempDir()
			exportAllFor(t, filepath.Join(root, "tlspr", "golden", f), gotDir)
			golden.CheckDir(t, filepath.Join(root, "export", strings.TrimSuffix(f, ".tlspr")), gotDir)
		})
	}
}

// exportAllFor exports a model in every format into dir.
func exportAllFor(t testing.TB, path, dir string) {
	p := headless.New()
	// Exports can report what didn't export; answer them all.
	for range 100 {
		p.DialogResponses = append(p.DialogResponses, platform.ResponseOK)
	}
	ss := New(p)
	ss.LoadSettings()
	if err := ss.Load(path); err != nil {
		t.Fatal(err)
	}
	// The goldens are drawn by a camera of 297x210.
	w := ss.GW.window.(*headless.Window)
	w.Width, w.Height = 297, 210

	v := &ss.GW.view
	v.Scale = 5
	v.Offset = geom.Vector{}
	v.ProjRight = geom.Vector{X: 0.8, Y: -0.5, Z: 0.2}
	v.ProjUp = geom.Vector{X: 0.3, Y: 0.6, Z: 0.75}
	ss.GW.NormalizeProjectionVectors()

	// A fixture without a solid has nothing for the section, mesh and
	// surface writers, which say so; any other error is a failure.
	tolerate := func(err error) {
		t.Helper()
		if err == nil {
			return
		}
		for _, empty := range []string{"no solid model", "mesh is empty", "does not contain any surfaces",
			"bad selection for export section"} {
			if strings.Contains(err.Error(), empty) {
				return
			}
		}
		t.Fatal(err)
	}
	for _, ext := range []string{"svg", "eps", "pdf", "plt", "txt", "step", "dxf"} {
		if err := ss.ExportViewOrWireframeTo(filepath.Join(dir, "view."+ext), false); err != nil {
			t.Fatal(err)
		}
	}
	for _, ext := range []string{"dxf", "step"} {
		if err := ss.ExportViewOrWireframeTo(filepath.Join(dir, "wireframe."+ext), true); err != nil {
			t.Fatal(err)
		}
	}
	// Straight on, so circles stay circles, with diameters shown as
	// radii.
	v.ProjRight = geom.Vector{X: 1}
	v.ProjUp = geom.Vector{Y: 1}
	for i := range ss.SK.Constraints {
		if c := &ss.SK.Constraints[i]; c.Type == core.ConstraintDiameter {
			c.Other = true
		}
	}
	for _, ext := range []string{"svg", "eps", "pdf", "dxf"} {
		if err := ss.ExportViewOrWireframeTo(filepath.Join(dir, "front."+ext), false); err != nil {
			t.Fatal(err)
		}
	}
	tolerate(ss.ExportSectionTo(filepath.Join(dir, "section.svg")))
	// With the export options that are off by default.
	ss.ExportShadedTriangles = true
	ss.ExportBackgroundColor = true
	ss.ExportCanvasSizeAuto = false
	for _, ext := range []string{"svg", "eps", "pdf"} {
		if err := ss.ExportViewOrWireframeTo(filepath.Join(dir, "options."+ext), false); err != nil {
			t.Fatal(err)
		}
	}
	ss.ExportOffset = 0.5
	if err := ss.ExportViewOrWireframeTo(filepath.Join(dir, "offset.txt"), false); err != nil {
		t.Fatal(err)
	}
	ss.ExportOffset = 0
	for _, ext := range []string{"stl", "glb", "obj", "js", "wrl"} {
		tolerate(ss.ExportMeshTo(filepath.Join(dir, "mesh."+ext)))
	}
	tolerate(ss.ExportSurfacesTo(filepath.Join(dir, "surfaces.step")))
}
