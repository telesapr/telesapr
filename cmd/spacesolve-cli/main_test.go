package main

import (
	"bytes"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spacesolve/spacesolve/pkg/srf"
)

func testdata(name string) string { return filepath.Join("..", "..", "testdata", name) }

func run(t *testing.T, args ...string) (bool, string) {
	t.Helper()
	var stderr bytes.Buffer
	srf.SetChordTolMm(0)
	ok := runCommand(append([]string{"spacesolve-cli"}, args...), &stderr)
	return ok, stderr.String()
}

// TestExportMeshVsCpp requires the STL that solvespace-cli export-mesh
// --chord-tol 0.1 wrote.
func TestExportMeshVsCpp(t *testing.T) {
	out := filepath.Join(t.TempDir(), "mesh.stl")
	ok, msg := run(t, "export-mesh", "--chord-tol", "0.1", "-o", out,
		testdata("group/boolean_knife_edge/normal.slvs"))
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
		t.Errorf("STL differs from C++'s")
	}
}

func TestOutputFor(t *testing.T) {
	for _, c := range []struct{ pattern, in, want string }{
		{"%-2d.png", "f/a.slvs", "f/a-2d.png"},
		{"%-2d.png", "a.slvs", "a-2d.png"},
		{"out/%.stl", "f/a.slvs", "out/f/a.stl"},
		{"x.stl", "f/a.slvs", "x.stl"},
	} {
		if got := outputFor(c.pattern, c.in); got != c.want {
			t.Errorf("outputFor(%q, %q) = %q, want %q", c.pattern, c.in, got, c.want)
		}
	}
}

func TestThumbnail(t *testing.T) {
	out := filepath.Join(t.TempDir(), "t.png")
	ok, msg := run(t, "thumbnail", "--size", "64x48", "--view", "isometric", "-o", out,
		testdata("group/boolean_knife_edge/normal.slvs"))
	if !ok {
		t.Fatalf("failed: %s", msg)
	}
	f, err := os.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, err := png.DecodeConfig(f)
	if err != nil || img.Width != 64 || img.Height != 48 {
		t.Errorf("png %v, %v", img, err)
	}
}

func TestBadCommandLines(t *testing.T) {
	in := testdata("request/line_segment/normal.slvs")
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
		{[]string{"export-mesh", "-o", "x.stl", "nothere.slvs"}, "Cannot load 'nothere.slvs'"},
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
	ok, msg := run(t, "export-mesh", "-o", out, testdata("request/line_segment/normal.slvs"))
	if ok || !strings.Contains(msg, "nothing to export") {
		t.Errorf("ok=%v: %s", ok, msg)
	}
}
