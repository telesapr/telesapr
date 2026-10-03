package ui

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/internal/golden"
	"github.com/telesapr/telesapr/pkg/persist/slvs"
	"github.com/telesapr/telesapr/pkg/platform"
	"github.com/telesapr/telesapr/pkg/platform/headless"
	"github.com/telesapr/telesapr/pkg/ttf"
)

// exportFixtures are the fixtures TestExport exports.
var exportFixtures = []string{
	"constraint/angle/normal.slvs",
	"constraint/comment/normal.slvs",
	"constraint/diameter/normal.slvs",
	"constraint/diameter/reference.slvs",
	"constraint/pt_face_distance/normal.slvs",
	"constraint/pt_line_distance/normal.slvs",
	"constraint/pt_pt_distance/normal.slvs",
	"group/boolean_knife_edge/normal.slvs",
	"group/link_stl/normal.slvs",
	"group/translate_nd/normal.slvs",
	"request/arc_of_circle/normal.slvs",
	"request/circle/normal.slvs",
	"request/cubic/normal.slvs",
	"request/cubic_periodic/normal.slvs",
	"request/ttf_text/normal.slvs",
}

// TestExport exports each fixture in every format and compares the
// files with the goldens (testdata/export, first recorded from C++),
// byte for byte.
func TestExport(t *testing.T) {
	root := filepath.Join("..", "..", "testdata")
	ttf.SetFontFiles([]string{filepath.Join(root, "Gentium-R.ttf")})
	for _, f := range exportFixtures {
		t.Run(f, func(t *testing.T) {
			gotDir := t.TempDir()
			exportAllFor(t, filepath.Join(root, f), gotDir)
			golden.CheckDir(t, filepath.Join(root, "export", strings.TrimSuffix(f, ".slvs")), gotDir)
		})
	}
}

// exportAllFor runs the harness's exports on a fixture.
func exportAllFor(t testing.TB, path, dir string) {
	p := headless.New()
	// Exports can report what didn't export; answer them all.
	for range 100 {
		p.DialogResponses = append(p.DialogResponses, platform.ResponseOK)
	}
	ss := New(p)
	ss.LoadSettings()
	ss.NewFile()
	ss.AfterNewFile()
	doc, err := slvs.LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	ss.LoadSketchForTest(doc.Sketch, path)
	// C++'s headless build has no window: its camera is 297x210.
	w := ss.GW.window.(*headless.Window)
	w.Width, w.Height = 297, 210

	v := &ss.GW.view
	v.Scale = 5
	v.Offset = geom.Vector{}
	v.ProjRight = geom.Vector{X: 0.8, Y: -0.5, Z: 0.2}
	v.ProjUp = geom.Vector{X: 0.3, Y: 0.6, Z: 0.75}
	ss.GW.NormalizeProjectionVectors()

	for _, ext := range []string{"svg", "eps", "pdf", "plt", "txt", "step", "dxf"} {
		ss.ExportViewOrWireframeTo(filepath.Join(dir, "view."+ext), false)
	}
	for _, ext := range []string{"dxf", "step"} {
		ss.ExportViewOrWireframeTo(filepath.Join(dir, "wireframe."+ext), true)
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
		ss.ExportViewOrWireframeTo(filepath.Join(dir, "front."+ext), false)
	}
	ss.ExportSectionTo(filepath.Join(dir, "section.svg"))
	// With the export options that are off by default.
	ss.ExportShadedTriangles = true
	ss.ExportBackgroundColor = true
	ss.ExportCanvasSizeAuto = false
	for _, ext := range []string{"svg", "eps", "pdf"} {
		ss.ExportViewOrWireframeTo(filepath.Join(dir, "options."+ext), false)
	}
	ss.ExportOffset = 0.5
	ss.ExportViewOrWireframeTo(filepath.Join(dir, "offset.txt"), false)
	ss.ExportOffset = 0
	for _, ext := range []string{"stl", "obj", "js", "wrl"} {
		ss.ExportMeshTo(filepath.Join(dir, "mesh."+ext))
	}
	ss.ExportSurfacesTo(filepath.Join(dir, "surfaces.step"))
}
