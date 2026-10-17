package api

import (
	"os"
	"path/filepath"
	"testing"
)

// TestExportOptions reads the options, sets them and has them refused
// when out of range; the mesh export on a bare sketch follows the chord
// tolerance and segment limit.
func TestExportOptions(t *testing.T) {
	t.Parallel()
	d := New()
	o := d.File.ExportOptions()
	if o.Scale != 1 || o.MaxSegments != 64 || !o.CanvasAuto || o.GCode.Passes != 1 || o.Margin.Left != 5 {
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
}
