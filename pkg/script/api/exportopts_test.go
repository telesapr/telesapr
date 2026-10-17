package api

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/telesapr/telesapr/pkg/sketch"
)

// TestExportOptions reads the options, sets them and has them refused
// when out of range; the mesh export on a bare sketch follows the chord
// tolerance and segment limit.
func TestExportOptions(t *testing.T) {
	t.Parallel()
	d := New()
	o := d.File.ExportOptions()
	if o.Scale != 1 || o.MaxSegments != 64 || o.AngleTol != 0 || o.StrictMesh || o.Quality() != MeshStandard || !o.CanvasAuto || o.GCode.Passes != 1 || o.Margin.Left != 5 {
		t.Errorf("defaults: %+v", o)
	}
	o.Scale = 2.5
	o.PwlCurves = true
	o.GCode.Passes = 3
	o.Canvas.Width = 210
	if err := d.File.SetExportOptions(o); err != nil {
		t.Fatal(err)
	}
	if got := d.File.ExportOptions(); got != o {
		t.Errorf("got %+v, set %+v", got, o)
	}

	for name, mod := range map[string]func(*ExportOptions){
		"zero scale":       func(o *ExportOptions) { o.Scale = 0 },
		"negative offset":  func(o *ExportOptions) { o.CutterOffset = -1 },
		"negative tol":     func(o *ExportOptions) { o.ChordTol = -1 },
		"negative angle":   func(o *ExportOptions) { o.AngleTol = -1 },
		"angle over 180":   func(o *ExportOptions) { o.AngleTol = 181 },
		"few segments":     func(o *ExportOptions) { o.MaxSegments = 3 },
		"many segments":    func(o *ExportOptions) { o.MaxSegments = 5000 },
		"no passes":        func(o *ExportOptions) { o.GCode.Passes = 0 },
		"not a number":     func(o *ExportOptions) { o.Margin.Top /= zero() },
		"unreasonable len": func(o *ExportOptions) { o.GCode.Feed = 1e12 },
	} {
		bad := d.File.ExportOptions()
		mod(&bad)
		if err := d.File.SetExportOptions(bad); err == nil {
			t.Errorf("%s: set", name)
		}
	}
	if got := d.File.ExportOptions(); got != o {
		t.Errorf("a refused set changed the options: %+v", got)
	}
}

func zero() float64 { return 0 }

// TestSetMaxSegments sets the display limit and has the mesh follow it.
func TestSetMaxSegments(t *testing.T) {
	t.Parallel()
	d := New()
	if n := d.Edit.MaxSegments(); n != 20 {
		t.Errorf("default %d segments", n)
	}
	d.Sketch.Circle(V(0, 0, 0), 10)
	d.Features.Extrude(5, "")
	count := func() int {
		sk := d.Host().Sketch()
		g := sk.Group(sk.ActiveGroup())
		g.GenerateDisplayItems(sk)
		return len(g.DisplayMesh.Triangles)
	}
	d.Edit.SetMaxSegments(7)
	few := d.Host().Sketch().SegmentLimit()
	if few != 7 || d.Edit.MaxSegments() != 7 {
		t.Errorf("limit %d, want 7", few)
	}
	fewTris := count()
	d.Edit.SetMaxSegments(200)
	if count() <= fewTris {
		t.Errorf("%d triangles at 200 segments, %d at 7", count(), fewTris)
	}
	for _, n := range []int{6, 1001, 0, -1} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("%d segments accepted", n)
				}
			}()
			d.Edit.SetMaxSegments(n)
		}()
	}
}

// TestExportOptionsMesh has the STL export follow the export chord
// tolerance and the segment limit of the options.
func TestExportOptionsMesh(t *testing.T) {
	t.Parallel()
	d := New()
	d.Sketch.Circle(V(0, 0, 0), 10)
	d.Features.Extrude(5, "")
	dir := t.TempDir()
	size := func(name string) int64 {
		path := filepath.Join(dir, name)
		if err := d.File.ExportAs(ExportMesh, path); err != nil {
			t.Fatal(err)
		}
		fi, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		return fi.Size()
	}
	o := d.File.ExportOptions()
	o.ChordTol = 0.5
	o.MaxSegments = 1000
	if err := d.File.SetExportOptions(o); err != nil {
		t.Fatal(err)
	}
	coarse := size("coarse.stl")
	o.ChordTol = 0.001
	if err := d.File.SetExportOptions(o); err != nil {
		t.Fatal(err)
	}
	fine := size("fine.stl")
	o.MaxSegments = 7
	if err := d.File.SetExportOptions(o); err != nil {
		t.Fatal(err)
	}
	capped := size("capped.stl")
	if !(fine > coarse) || !(capped < fine) {
		t.Errorf("STL sizes: coarse %d, fine %d, capped to 7 segments %d", coarse, fine, capped)
	}

	// The presets: fine writes a bigger file than standard, and Quality
	// reads the preset back until a field is edited by hand.
	for _, q := range []MeshQuality{MeshDraft, MeshFine, MeshStandard} {
		if err := d.File.SetMeshQuality(q); err != nil {
			t.Fatal(err)
		}
		if got := d.File.ExportOptions().Quality(); got != q {
			t.Errorf("set %v, read %v", q, got)
		}
	}
	if err := d.File.SetMeshQuality(MeshStandard); err != nil {
		t.Fatal(err)
	}
	std := size("standard.stl")
	if err := d.File.SetMeshQuality(MeshFine); err != nil {
		t.Fatal(err)
	}
	if f := size("fine-preset.stl"); f <= std {
		t.Errorf("fine preset STL %d bytes, standard %d", f, std)
	}
	o = d.File.ExportOptions()
	o.ChordTol = 0.03
	if err := d.File.SetExportOptions(o); err != nil {
		t.Fatal(err)
	}
	if q := d.File.ExportOptions().Quality(); q != MeshCustom {
		t.Errorf("after a hand edit: %v", q)
	}
	if err := d.File.SetMeshQuality(MeshCustom); err == nil {
		t.Error("custom accepted as a preset")
	}
}

// TestExportStrictMesh has a strict mesh export refuse a model whose
// shells overlap, leaving no file, and the same export without strict
// write it.
func TestExportStrictMesh(t *testing.T) {
	t.Parallel()
	d := New()
	d.Sketch.Rect(V(0, 0, 0), V(10, 10, 0))
	d.Features.Extrude(10, "first")
	d.Features.SketchIn(d.Workplane("XY"), "")
	d.Sketch.Rect(V(8, 0, 0), V(18, 10, 0))
	d.Features.Extrude(10, "second").SetCombine(Assemble)
	dir := t.TempDir()

	o := d.File.ExportOptions()
	o.StrictMesh = true
	if err := d.File.SetExportOptions(o); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "strict.stl")
	if err := d.File.ExportAs(ExportMesh, path); !errors.Is(err, sketch.ErrMeshNotSound) {
		t.Fatalf("strict export of overlapping shells: %v", err)
	}
	if _, err := os.Stat(path); err == nil {
		t.Error("a refused export left a file")
	}
	o.StrictMesh = false
	if err := d.File.SetExportOptions(o); err != nil {
		t.Fatal(err)
	}
	if err := d.File.ExportAs(ExportMesh, path); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Error(err)
	}
}
