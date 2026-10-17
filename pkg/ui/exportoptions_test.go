package ui

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/telesapr/telesapr/pkg/platform"
	"github.com/telesapr/telesapr/pkg/script/api"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// TestScriptExportOptions sets the export options and the segment limit
// from a script, and requires that the window's export writes the same
// file as when the configuration screen's fields hold the same values.
func TestScriptExportOptions(t *testing.T) {
	ss, _ := newTestApp(t)
	doc := api.NewDoc(ss.ScriptHost())
	doc.Sketch.Rect(api.V(0, 0, 0), api.V(40, 20, 0))
	doc.Sketch.Circle(api.V(20, 10, 0), 6)
	dir := t.TempDir()
	write := func(name string) string {
		path := filepath.Join(dir, name)
		if err := ss.ExportViewOrWireframeTo(path, false); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	before := write("before.svg")

	o := doc.File.ExportOptions()
	if o.Scale != ss.ExportScale || o.MaxSegments != ss.ExportMaxSegments || o.GCode.Feed != ss.GCode.Feed {
		t.Errorf("the options %+v are not the window's settings", o)
	}
	o.Scale = 2
	o.PwlCurves = true
	o.CanvasAuto = false
	o.Canvas.Width = 150
	o.Canvas.Height = 90
	o.ChordTol = 0.05
	o.MaxSegments = 100
	o.AngleTol = 15
	o.StrictMesh = true
	o.GCode.Passes = 4
	o.GCode.Depth = 3
	if err := doc.File.SetExportOptions(o); err != nil {
		t.Fatal(err)
	}
	doc.Edit.SetMaxSegments(30)

	if ss.ExportScale != 2 || !ss.ExportPwlCurves || ss.ExportCanvasSizeAuto || ss.ExportCanvas.Width != 150 ||
		ss.ExportChordTol != 0.05 || ss.ExportMaxSegments != 100 || ss.ExportAngleTol != 15 || !ss.ExportStrictMesh || ss.GCode.Passes != 4 || ss.GCode.Depth != 3 {
		t.Errorf("the window's settings did not follow: %+v", ss.config)
	}
	if ss.MaxSegments != 30 || ss.SK.MaxSegments != 30 || doc.Edit.MaxSegments() != 30 {
		t.Errorf("segments: window %d, sketch %d", ss.MaxSegments, ss.SK.MaxSegments)
	}
	if got := ss.exportOptions(); got.Scale != 2 || got.GCode.Passes != 4 || got.CanvasSizeAuto || got.Canvas.Width != 150 {
		t.Errorf("the writers are told %+v", got)
	}
	if got := doc.File.ExportOptions(); got != o {
		t.Errorf("read back %+v, set %+v", got, o)
	}
	if after := write("after.svg"); after == before {
		t.Error("the export did not change with the options")
	}

	// A refused set changes nothing.
	bad := o
	bad.Scale = 0
	if err := doc.File.SetExportOptions(bad); err == nil || ss.ExportScale != 2 {
		t.Errorf("a zero scale: %v, scale %g", err, ss.ExportScale)
	}
}

// TestScriptExportMeshTolerance has the window's mesh export follow the
// export chord tolerance and segment limit a script sets: the file is
// not the display mesh the sketch cached.
func TestScriptExportMeshTolerance(t *testing.T) {
	ss, _ := newTestApp(t)
	doc := api.NewDoc(ss.ScriptHost())
	doc.Sketch.Circle(api.V(0, 0, 0), 10)
	doc.Features.Extrude(5, "")
	dir := t.TempDir()
	size := func(name string, tol float64, segments int) int64 {
		o := doc.File.ExportOptions()
		o.ChordTol, o.MaxSegments = tol, segments
		if err := doc.File.SetExportOptions(o); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(dir, name)
		if err := doc.File.ExportAs(api.ExportMesh, path); err != nil {
			t.Fatal(err)
		}
		fi, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		return fi.Size()
	}
	coarse := size("coarse.stl", 0.5, 1000)
	fine := size("fine.stl", 0.001, 1000)
	capped := size("capped.stl", 0.001, 7)
	if !(fine > coarse) || !(capped < fine) {
		t.Errorf("STL sizes: coarse %d, fine %d, capped %d", coarse, fine, capped)
	}
}

// TestExportMeshAngleTolerance has the window's mesh export follow the
// angular tolerance in its settings: the STL has more facets.
func TestExportMeshAngleTolerance(t *testing.T) {
	ss, _ := newTestApp(t)
	doc := api.NewDoc(ss.ScriptHost())
	doc.Sketch.Circle(api.V(0, 0, 0), 10)
	doc.Features.Extrude(5, "")
	dir := t.TempDir()
	size := func(name string, angle float64) int64 {
		ss.ExportAngleTol = angle
		path := filepath.Join(dir, name)
		if err := ss.ExportMeshTo(path); err != nil {
			t.Fatal(err)
		}
		fi, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		return fi.Size()
	}
	ss.ExportChordTol, ss.ExportMaxSegments = 0.5, 1000
	plain, angled := size("plain.stl", 0), size("angled.stl", 10)
	if angled <= plain {
		t.Errorf("STL sizes: %d at an angle of 10, %d at 0", angled, plain)
	}
}

// TestExportMeshStrict has the window refuse to export a mesh that
// crosses itself when its settings say strict. (Without it the export
// reports the mesh in a dialog, which the headless platform can't answer.)
func TestExportMeshStrict(t *testing.T) {
	ss, _ := newTestApp(t)
	doc := api.NewDoc(ss.ScriptHost())
	doc.Sketch.Rect(api.V(0, 0, 0), api.V(10, 10, 0))
	doc.Features.Extrude(10, "")
	doc.Features.SketchIn(doc.Workplane("XY"), "")
	doc.Sketch.Rect(api.V(8, 0, 0), api.V(18, 10, 0))
	doc.Features.Extrude(10, "second").SetCombine(api.Assemble)
	dir := t.TempDir()

	ss.ExportStrictMesh = true
	strict := filepath.Join(dir, "strict.stl")
	if err := ss.ExportMeshTo(strict); !errors.Is(err, sketch.ErrMeshNotSound) {
		t.Fatalf("strict: %v", err)
	}
	if _, err := os.Stat(strict); err == nil {
		t.Error("strict: the file was created")
	}
}

// TestExportQualitySettingsRoundTrip freezes the angular tolerance and
// the strict flag and thaws them back.
func TestExportQualitySettingsRoundTrip(t *testing.T) {
	t.Parallel()
	s := platform.NewMemorySettings()
	var in config
	in.ExportAngleTol, in.ExportStrictMesh = 10, true
	in.Freeze(s)
	var out config
	out.Thaw(s)
	if out.ExportAngleTol != 10 || !out.ExportStrictMesh {
		t.Errorf("thawed %g, %t", out.ExportAngleTol, out.ExportStrictMesh)
	}
	var fresh config
	fresh.Thaw(platform.NewMemorySettings())
	if fresh.ExportAngleTol != 0 || fresh.ExportStrictMesh {
		t.Errorf("defaults %g, %t", fresh.ExportAngleTol, fresh.ExportStrictMesh)
	}
}
