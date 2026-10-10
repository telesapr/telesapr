package export

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/persist/slvs"
	"github.com/telesapr/telesapr/pkg/persist/tlspr"
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/srf"
)

// fixture loads a SolveSpace model from testdata, regenerated.
func fixture(t *testing.T, path string) *sketch.Sketch {
	t.Helper()
	doc, err := slvs.LoadFile(filepath.Join("..", "..", "testdata", filepath.FromSlash(path)))
	if err != nil {
		t.Fatal(err)
	}
	sk := doc.Sketch
	sk.GenerateAll()
	return sk
}

// options are the export options a test writes with: the defaults of
// the configuration screen, every constraint shown and labelled by its
// value, and warnings kept.
func options(warnings *[]string) Options {
	return Options{
		Scale:          1,
		Tol:            srf.Tol(0.1, 64),
		CanvasSizeAuto: true,
		Margin:         Margin{Left: 5, Right: 5, Bottom: 5, Top: 5},
		Canvas:         Canvas{Width: 100, Height: 100, Dx: 5, Dy: 5},
		FixColors:      true,
		ViewScale:      5,
		GCode:          GCode{Depth: 10, SafeHeight: 5, Passes: 2, Feed: 10, PlungeFeed: 10},
		Lights:         [2]Light{{Dir: geom.Vector{X: -1, Y: 1}, Intensity: 1}, {Dir: geom.Vector{X: 1}, Intensity: 0.5}},
		Ambient:        0.3,
		Constraints:    labelled{},
		Warn:           func(msg string) { *warnings = append(*warnings, msg) },
	}
}

// labelled shows every constraint, labelled by its value.
type labelled struct{}

func (labelled) Visible(*core.Constraint) bool { return true }
func (labelled) Label(c *core.Constraint) string {
	if c.Type == core.ConstraintComment {
		return c.Comment
	}
	return strings.TrimRight(strings.TrimRight(formatFloat(c.ValA), "0"), ".")
}

func formatFloat(v float64) string { return fmt.Sprintf("%.3f", v) }

// curvesOf collects the visible entities' curves and edges, as the
// 2d view export does, projected onto the view's plane.
func curvesOf(sk *sketch.Sketch) (srf.SEdgeList, srf.SBezierList) {
	var edges srf.SEdgeList
	var beziers srf.SBezierList
	for i := range sk.Entities {
		e := &sk.Entities[i]
		if !sk.EntityIsVisible(e) {
			continue
		}
		sketch.GenerateEntityBeziers(e, &beziers, &sketch.Runtime{Sketch: sk})
	}
	return edges, beziers
}

// TestVectorFormats writes a sketch with constraints in every 2d
// format, through NewWriter, and checks each file's shape.
func TestVectorFormats(t *testing.T) {
	t.Parallel()
	sk := fixture(t, "constraint/diameter/normal.slvs")
	dir := t.TempDir()
	for _, tc := range []struct {
		ext   string
		wants []string
	}{
		{"svg", []string{"<svg", "</svg>", "stroke-width"}},
		{"eps", []string{"%!PS-Adobe-2.0", "grestore"}},
		{"pdf", []string{"%PDF-1.1", "%%EOF"}},
		{"plt", []string{"IN;", "PU", "PD"}},
		{"txt", []string{"G00", "G01"}},
		{"step", []string{"ISO-10303-21;", "END-ISO-10303-21;"}},
		{"dxf", []string{"SECTION", "ENTITIES", "EOF"}},
	} {
		t.Run(tc.ext, func(t *testing.T) {
			t.Parallel()
			var warnings []string
			path := filepath.Join(dir, "view."+tc.ext)
			out, err := NewWriter(path, sk, options(&warnings))
			if err != nil {
				t.Fatal(err)
			}
			edges, beziers := curvesOf(sk)
			u, v := geom.Vector{X: 1}, geom.Vector{Y: 1}
			out.SetModelviewProjection(u, v, u.Cross(v), geom.Vector{}, 0, 1)
			if out.OutputConstraints(sk.Constraints) != (tc.ext == "dxf") {
				t.Errorf("%s takes constraints", tc.ext)
			}
			var mesh srf.SMesh
			if err := WriteCurves(&edges, &beziers, &mesh, srf.Tol(0.1, 64), out); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			for _, w := range tc.wants {
				if !strings.Contains(string(data), w) {
					t.Errorf("%s lacks %q", tc.ext, w)
				}
			}
			if len(data) < 200 {
				t.Errorf("%s is %d bytes", tc.ext, len(data))
			}
			if out.HasCanvasSize() != (tc.ext == "svg" || tc.ext == "eps" || tc.ext == "pdf") {
				t.Errorf("%s has a canvas size: %v", tc.ext, out.HasCanvasSize())
			}
			if out.CanOutputMesh() != out.HasCanvasSize() {
				t.Errorf("%s can output a mesh: %v", tc.ext, out.CanOutputMesh())
			}
		})
	}
}

// TestVectorOptions writes with the options that are off by default:
// a fixed canvas, a background, shaded triangles, and a mesh in the
// formats that hold one.
func TestVectorOptions(t *testing.T) {
	t.Parallel()
	sk := fixture(t, "group/translate_nd/normal.slvs")
	g := sk.Group(sk.ActiveGroup())
	g.GenerateDisplayItems(sk)
	dir := t.TempDir()
	for _, ext := range []string{"svg", "eps", "pdf", "plt", "txt", "step", "dxf"} {
		var warnings []string
		o := options(&warnings)
		o.CanvasSizeAuto = false
		if ext == "pdf" {
			o.Canvas = Canvas{Width: 6000, Height: 6000}
		}
		bg := core.NewRGBA(10, 20, 30, 255)
		o.Background = &bg
		o.ShadedTriangles = true
		path := filepath.Join(dir, "shaded."+ext)
		out, err := NewWriter(path, sk, o)
		if err != nil {
			t.Fatal(err)
		}
		u, v := geom.Vector{X: 0.8, Y: -0.5, Z: 0.2}.WithMagnitude(1), geom.Vector{X: 0.3, Y: 0.6, Z: 0.75}.WithMagnitude(1)
		out.SetModelviewProjection(u, v, u.Cross(v), geom.Vector{}, 0, 1)
		edges, beziers := curvesOf(sk)
		mesh := g.DisplayMesh
		if err := WriteCurves(&edges, &beziers, &mesh, srf.Tol(0.1, 64), out); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if len(data) < 500 {
			t.Errorf("%s with the mesh is %d bytes", ext, len(data))
		}
		if ext == "pdf" && len(warnings) != 1 {
			t.Errorf("a 6 m page warned %q", warnings)
		}
	}
	// An unknown extension, and a file that can't be made.
	if _, err := NewWriter(filepath.Join(dir, "x.xyz"), sk, options(nil)); err == nil || !strings.Contains(err.Error(), "file type") {
		t.Errorf("unknown extension: %v", err)
	}
	if _, err := NewWriter(filepath.Join(dir, "none", "x.svg"), sk, options(nil)); err == nil || !strings.Contains(err.Error(), "write") {
		t.Errorf("unwritable: %v", err)
	}
}

// TestMeshFormats writes a solid's mesh as OBJ, three.js and VRML.
func TestMeshFormats(t *testing.T) {
	t.Parallel()
	sk := fixture(t, "group/translate_nd/normal.slvs")
	g := sk.Group(sk.ActiveGroup())
	g.GenerateDisplayItems(sk)
	m := &g.DisplayMesh
	if len(m.Triangles) == 0 {
		t.Fatal("no mesh")
	}
	o := options(nil)
	var obj, mtl bytes.Buffer
	WriteOBJ(&obj, &mtl, m, o)
	if !strings.Contains(obj.String(), "\nf ") || !strings.Contains(mtl.String(), "newmtl") {
		t.Errorf("obj %d bytes, mtl %d bytes", obj.Len(), mtl.Len())
	}
	view := sketch.DefaultView()
	viewp := &view
	for _, name := range []string{"model.js", "model.html"} {
		var buf bytes.Buffer
		WriteThreeJS(&buf, name, m, &g.DisplayOutlines, viewp, o)
		if buf.Len() < 1000 {
			t.Errorf("%s: %d bytes", name, buf.Len())
		}
		if strings.HasSuffix(name, ".html") != strings.Contains(buf.String(), "<html") {
			t.Errorf("%s has a page: %v", name, strings.Contains(buf.String(), "<html"))
		}
	}
	var wrl bytes.Buffer
	WriteVRML(&wrl, "model.wrl", m, o)
	if !strings.HasPrefix(wrl.String(), "#VRML") {
		t.Errorf("vrml starts %.20q", wrl.String())
	}
}

// TestMeshFormatsEmpty writes an empty mesh in each format without a
// panic; VRML gives a transform with no shapes.
func TestMeshFormatsEmpty(t *testing.T) {
	t.Parallel()
	var m srf.SMesh
	o := options(nil)
	var obj, mtl bytes.Buffer
	WriteOBJ(&obj, &mtl, &m, o)
	view := sketch.DefaultView()
	var js bytes.Buffer
	WriteThreeJS(&js, "model.js", &m, &srf.SOutlineList{}, &view, o)
	var wrl bytes.Buffer
	WriteVRML(&wrl, "model.wrl", &m, o)
	if s := wrl.String(); !strings.HasPrefix(s, "#VRML") || strings.Contains(s, "Shape") {
		t.Errorf("empty vrml:\n%s", s)
	}
}

// TestStep writes a solid as STEP, and refuses a group without one.
func TestStep(t *testing.T) {
	t.Parallel()
	sk := fixture(t, "group/translate_nd/normal.slvs")
	g := sk.Group(sk.ActiveGroup())
	var out bytes.Buffer
	for _, one := range []bool{true, false} {
		out.Reset()
		o := options(nil)
		o.StepOneSurfacePerFace = one
		if err := WriteStep(&out, sk, g, o); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out.String(), "MANIFOLD_SOLID_BREP") || !strings.Contains(out.String(), "END-ISO-10303-21;") {
			t.Errorf("one surface per face %v: %d bytes", one, out.Len())
		}
	}
	flat := fixture(t, "constraint/diameter/normal.slvs")
	if err := WriteStep(&out, flat, flat.Group(flat.ActiveGroup()), options(nil)); err == nil || !strings.Contains(err.Error(), "surfaces") {
		t.Errorf("a sketch without a solid: %v", err)
	}
}

// TestDxfDimensions writes the constraint fixtures as DXF: each kind
// of dimension, and a comment as text.
func TestDxfDimensions(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	for _, name := range []string{"pt_pt_distance", "pt_line_distance", "diameter", "angle", "comment"} {
		sk := fixture(t, "constraint/"+name+"/normal.slvs")
		if len(sk.Constraints) == 0 {
			t.Fatalf("%s has no constraints", name)
		}
		var warnings []string
		path := filepath.Join(dir, name+".dxf")
		out, err := NewWriter(path, sk, options(&warnings))
		if err != nil {
			t.Fatal(err)
		}
		u, v := geom.Vector{X: 1}, geom.Vector{Y: 1}
		out.SetModelviewProjection(u, v, u.Cross(v), geom.Vector{}, 0, 1)
		out.OutputConstraints(sk.Constraints)
		edges, beziers := curvesOf(sk)
		if err := WriteCurves(&edges, &beziers, nil, srf.Tol(0.1, 64), out); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		want := "DIMENSION"
		if name == "comment" {
			want = "TEXT"
		}
		if !strings.Contains(string(data), want) {
			t.Errorf("%s: no %s in the file", name, want)
		}
	}
	// A diameter shown as a radius.
	sk := fixture(t, "constraint/diameter/normal.slvs")
	sk.Constraints[0].Other = true
	path := filepath.Join(dir, "radius.dxf")
	out, err := NewWriter(path, sk, options(nil))
	if err != nil {
		t.Fatal(err)
	}
	out.SetModelviewProjection(geom.Vector{X: 1}, geom.Vector{Y: 1}, geom.Vector{Z: 1}, geom.Vector{}, 0, 1)
	out.OutputConstraints(sk.Constraints)
	edges, beziers := curvesOf(sk)
	if err := WriteCurves(&edges, &beziers, nil, srf.Tol(0.1, 64), out); err != nil {
		t.Fatal(err)
	}
}

// TestStepSheets writes a solid whose faces are several patches, as
// one surface per face and as the patches, and an assembly of linked
// parts.
func TestStepSheets(t *testing.T) {
	t.Parallel()
	for _, model := range []string{"topo/boss.tlspr", "topo/frustum.tlspr", "solid/fillet-circle.tlspr"} {
		doc, err := tlspr.LoadFile(filepath.Join("..", "..", "testdata", filepath.FromSlash(model)))
		if err != nil {
			t.Fatal(err)
		}
		sk := doc.Sketch
		sk.GenerateAll()
		g := sk.Group(sk.ActiveGroup())
		var one, patches bytes.Buffer
		o := options(nil)
		o.StepOneSurfacePerFace = true
		if err := WriteStep(&one, sk, g, o); err != nil {
			t.Fatal(err)
		}
		o.StepOneSurfacePerFace = false
		if err := WriteStep(&patches, sk, g, o); err != nil {
			t.Fatal(err)
		}
		a, b := strings.Count(one.String(), "ADVANCED_FACE("), strings.Count(patches.String(), "ADVANCED_FACE(")
		if a == 0 || a >= b {
			t.Errorf("%s: %d faces as surfaces, %d as patches", model, a, b)
		}
	}
	doc, err := tlspr.LoadFile(filepath.Join("..", "..", "testdata", "tlspr", "v1", "assembly", "assembly.tlspr"))
	if err != nil {
		t.Fatal(err)
	}
	sk := doc.Sketch
	sk.GenerateAll()
	g := sk.Group(sk.ActiveGroup())
	if parts := sk.AssemblyParts(g.H); len(parts) < 2 {
		t.Skipf("%d parts", len(parts))
	}
	var out bytes.Buffer
	if err := WriteStep(&out, sk, g, options(nil)); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "NEXT_ASSEMBLY_USAGE_OCCURRENCE") {
		t.Error("no assembly in the file")
	}
}

func TestDxfLineType(t *testing.T) {
	t.Parallel()
	if DxfLineType(core.StippleDash) != "DASHED" || DxfLineType(core.StippleZigzag) != "CONTINUOUS" {
		t.Error("line types")
	}
}
