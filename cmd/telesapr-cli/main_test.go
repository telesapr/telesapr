package main

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/persist/convert"
	"github.com/telesapr/telesapr/pkg/srf"
)

func testdata(name string) string { return filepath.Join("..", "..", "testdata", name) }

// model converts a fixture, and the files beside it, into a new
// temporary directory and returns the .tlspr file.
func model(t *testing.T, fixture string) string {
	t.Helper()
	dir := t.TempDir()
	from := testdata(filepath.Dir(fixture))
	entries, err := os.ReadDir(from)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(from, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, e.Name()), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	src := filepath.Join(dir, filepath.Base(fixture))
	dst := convert.SiblingPath(src)
	srf.SetChordTolMm(0)
	if _, _, err := convert.File(src, dst, true); err != nil {
		t.Fatal(err)
	}
	return dst
}

func run(t *testing.T, args ...string) (bool, string) {
	t.Helper()
	var stderr bytes.Buffer
	srf.SetChordTolMm(0)
	ok := runCommand(append([]string{"telesapr-cli"}, args...), &stderr)
	return ok, stderr.String()
}

// TestExportMesh requires the golden STL of export-mesh --chord-tol 0.1
// (first written by C++ solvespace-cli; TestExportStl in
// pkg/persist/slvs regenerates it).
func TestExportMesh(t *testing.T) {
	out := filepath.Join(t.TempDir(), "mesh.stl")
	ok, msg := run(t, "export-mesh", "--chord-tol", "0.1", "-o", out,
		model(t, "group/boolean_knife_edge/normal.slvs"))
	if !ok {
		t.Fatalf("failed: %s", msg)
	}
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(testdata("export/stl/group_boolean_knife_edge_normal.stl"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("STL differs from the golden")
	}
}

// TestExportMesh3mf exports the same model as 3MF (roadmap 16.5): a zip
// whose model has as many triangles as the STL.
func TestExportMesh3mf(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"mesh.stl", "mesh.3mf"} {
		if ok, msg := run(t, "export-mesh", "--chord-tol", "0.1", "-o", filepath.Join(dir, name),
			model(t, "group/boolean_knife_edge/normal.slvs")); !ok {
			t.Fatalf("%s: %s", name, msg)
		}
	}
	stl, err := os.ReadFile(filepath.Join(dir, "mesh.stl"))
	if err != nil {
		t.Fatal(err)
	}
	zr, err := zip.OpenReader(filepath.Join(dir, "mesh.3mf"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = zr.Close() }()
	var model []byte
	for _, f := range zr.File {
		if f.Name == "3D/3dmodel.model" {
			r, _ := f.Open()
			model, _ = io.ReadAll(r)
			_ = r.Close()
		}
	}
	if got, want := bytes.Count(model, []byte("<triangle ")), int(binary.LittleEndian.Uint32(stl[80:84])); got != want {
		t.Errorf("%d triangles in the 3MF, %d in the STL", got, want)
	}
}

// TestExportDrawing writes the default drawing of a model: an SVG on an
// A3 sheet (roadmap 18.1).
func TestExportDrawing(t *testing.T) {
	out := filepath.Join(t.TempDir(), "sheet.svg")
	if ok, msg := run(t, "export-drawing", "-o", out, model(t, "group/boolean_knife_edge/normal.slvs")); !ok {
		t.Fatalf("failed: %s", msg)
	}
	data, err := os.ReadFile(out)
	if err != nil || !bytes.Contains(data, []byte("width='420.000mm' height='297.000mm'")) {
		t.Errorf("not an A3 sheet (%v): %.200s", err, data)
	}
}

func TestOutputFor(t *testing.T) {
	for _, c := range []struct{ pattern, in, want string }{
		{"%-2d.png", "f/a.tlspr", "f/a-2d.png"},
		{"%-2d.png", "a.tlspr", "a-2d.png"},
		{"out/%.stl", "f/a.tlspr", "out/f/a.stl"},
		{"x.stl", "f/a.tlspr", "x.stl"},
		{"%.tlspr", "f/a.slvs", "f/a.tlspr"},
	} {
		if got := outputFor(c.pattern, c.in); got != c.want {
			t.Errorf("outputFor(%q, %q) = %q, want %q", c.pattern, c.in, got, c.want)
		}
	}
}

func TestThumbnail(t *testing.T) {
	out := filepath.Join(t.TempDir(), "t.png")
	ok, msg := run(t, "thumbnail", "--size", "64x48", "--view", "isometric", "-o", out,
		model(t, "group/boolean_knife_edge/normal.slvs"))
	if !ok {
		t.Fatalf("failed: %s", msg)
	}
	f, err := os.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	img, err := png.DecodeConfig(f)
	if err != nil || img.Width != 64 || img.Height != 48 {
		t.Errorf("png %v, %v", img, err)
	}
}

func TestBadCommandLines(t *testing.T) {
	in := model(t, "request/line_segment/normal.slvs")
	for _, c := range []struct {
		args []string
		msg  string
	}{
		{[]string{"frobnicate"}, "Unrecognized command 'frobnicate'"},
		{[]string{"export-mesh", in}, "An output pattern must be specified"},
		{[]string{"export-mesh", "-o", "x.stl"}, "At least one input file"},
		{[]string{"export-mesh", "-o", "x.stl", in, in}, "must include a %"},
		{[]string{"export-mesh", "--view", "top", "-o", "x.stl", in}, "Unrecognized option '--view'"},
		{[]string{"export-view", "-o", "x.svg", in}, "View direction must be specified"},
		{[]string{"thumbnail", "-v", "top", "-o", "x.png", in}, "viewport size"},
		{[]string{"export-mesh", "-o", "x.stl", "nothere.tlspr"}, "Cannot load 'nothere.tlspr'"},
		{[]string{"export-mesh", "-o", "x.stl", testdata("request/line_segment/normal.slvs")}, "telesapr-cli convert"},
		{[]string{"convert", "--view", "top", in}, "Unrecognized option '--view'"},
		{[]string{"convert"}, "At least one input file"},
		{[]string{"convert", "-o", "x.tlspr", in, in}, "must include a %"},
	} {
		ok, out := run(t, c.args...)
		if ok || !strings.Contains(out, c.msg) {
			t.Errorf("%v: ok=%v, output %q lacks %q", c.args, ok, out, c.msg)
		}
	}
}

// TestExportErrorFails requires a failing export, such as a mesh of a
// sketch without solids, to fail.
func TestExportErrorFails(t *testing.T) {
	out := filepath.Join(t.TempDir(), "x.stl")
	ok, msg := run(t, "export-mesh", "-o", out, model(t, "request/line_segment/normal.slvs"))
	if ok || !strings.Contains(msg, "nothing to export") {
		t.Errorf("ok=%v: %s", ok, msg)
	}
}

// TestRegenerateWritesTlspr regenerates a model into its own file, at
// the absolute chord tolerance, so the shell it holds for linking files
// is finer than the default.
func TestRegenerateWritesTlspr(t *testing.T) {
	in := model(t, "group/translate_nd/normal.slvs")
	if ok, msg := run(t, "regenerate", "--chord-tol", "0.05", in); !ok || !strings.Contains(msg, "Written '"+in+"'") {
		t.Fatalf("ok=%v: %s", ok, msg)
	}
	data, err := os.ReadFile(in)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(data), "telesapr 2\n") {
		t.Errorf("regenerated file starts %.30q", data)
	}
	out := filepath.Join(t.TempDir(), "x.stl")
	if ok, msg := run(t, "export-mesh", "-o", out, in); !ok {
		t.Errorf("the regenerated model doesn't export: %s", msg)
	}
}

// copyTestdata copies the files of a testdata directory into a new
// temporary directory, and returns it.
func copyTestdata(t *testing.T, dir string) string {
	t.Helper()
	out := t.TempDir()
	entries, err := os.ReadDir(testdata(dir))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		data, err := os.ReadFile(filepath.Join(testdata(dir), e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(out, e.Name()), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return out
}

// TestConvert converts an assembly, and checks what is reused,
// overwritten, and refused.
func TestConvert(t *testing.T) {
	dir := copyTestdata(t, "group/link")
	src := filepath.Join(dir, "normal.slvs")
	linked := filepath.Join(dir, "rect_v20.tlspr")

	ok, msg := run(t, "convert", src)
	if !ok || !strings.Contains(msg, "Written '"+linked+"'") || !strings.Contains(msg, "Written '"+filepath.Join(dir, "normal.tlspr")+"'") {
		t.Fatalf("ok=%v: %s", ok, msg)
	}
	ok, msg = run(t, "convert", "-o", "%-copy.tlspr", src)
	if !ok || !strings.Contains(msg, "Kept '"+linked+"'") || !strings.Contains(msg, "normal-copy.tlspr") {
		t.Errorf("ok=%v: %s", ok, msg)
	}
	ok, msg = run(t, "convert", "--overwrite", src)
	if !ok || !strings.Contains(msg, "Written '"+linked+"'") {
		t.Errorf("ok=%v: %s", ok, msg)
	}

	// What it wrote is a model the other commands take.
	if ok, msg := run(t, "regenerate", filepath.Join(dir, "normal.tlspr")); !ok {
		t.Errorf("regenerating the converted model: %s", msg)
	}

	if err := os.Remove(filepath.Join(dir, "rect_v20.slvs")); err != nil {
		t.Fatal(err)
	}
	ok, msg = run(t, "convert", "-o", "%-missing.tlspr", src)
	if ok || !strings.Contains(msg, "the linked file '"+filepath.Join(dir, "rect_v20.slvs")+"' can't be read") {
		t.Errorf("ok=%v: %s", ok, msg)
	}
	if _, err := os.Stat(filepath.Join(dir, "normal-missing.tlspr")); err == nil {
		t.Error("a model with a missing link was written")
	}

	ok, msg = run(t, "convert", "-o", filepath.Join(dir, "x.txt"), src)
	if ok || !strings.Contains(msg, "is not a TeleSAPR file") {
		t.Errorf("ok=%v: %s", ok, msg)
	}
	ok, msg = run(t, "convert", "-o", filepath.Join(dir, "none", "x.tlspr"), filepath.Join(dir, "normal.slvs"))
	if ok || !strings.Contains(msg, "Cannot convert") {
		t.Errorf("unwritable output: ok=%v: %s", ok, msg)
	}
}
