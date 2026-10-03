package sketch_test

import (
	"archive/zip"
	"bytes"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/persist/tlspr"
	"github.com/telesapr/telesapr/pkg/script/api"
)

// A packed model (roadmap 17.5): an assembly that links a part, which
// links a STEP file in its own folder, and a mesh, saved as .tlsprz,
// holds them all, with the STEP file's tessellation; opened from
// another folder, with the originals gone, it is the same model; packed
// twice it is the same bytes.
func TestPackedModel(t *testing.T) {
	t.Setenv("TELESAPR_CACHE_DIR", t.TempDir())
	src := t.TempDir()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	// The part, in sub/, linking the STEP file beside it.
	stepFile, _ := filepath.Abs(filepath.Join("..", "..", "testdata", "step", "shapes", "cylinder.step"))
	data, err := os.ReadFile(stepFile)
	must(err)
	must(os.MkdirAll(filepath.Join(src, "sub"), 0o755))
	must(os.WriteFile(filepath.Join(src, "sub", "cylinder.step"), data, 0o644))
	part := api.New()
	part.Rect(api.V(0, 0, 0), api.V(10, 10, 0))
	part.Extrude(3, "")
	_, err = part.Link(filepath.Join(src, "sub", "cylinder.step"), "")
	must(err)
	must(part.Save(filepath.Join(src, "sub", "part.tlspr")))
	// A mesh.
	mesh := api.New()
	mesh.Rect(api.V(0, 0, 0), api.V(4, 4, 0))
	mesh.Extrude(4, "")
	must(mesh.Export(filepath.Join(src, "block.stl")))

	asm := api.New()
	_, err = asm.Link(filepath.Join(src, "sub", "part.tlspr"), "")
	must(err)
	_, err = asm.Link(filepath.Join(src, "block.stl"), "")
	must(err)
	want := asm.Active().Check()
	model := filepath.Join(src, "asm.tlspr")
	must(asm.Save(model))
	packed := filepath.Join(t.TempDir(), "asm.tlsprz")
	back, err := api.Open(model)
	must(err)
	must(back.Save(packed))
	first, _ := os.ReadFile(packed)
	must(back.Save(packed))
	if again, _ := os.ReadFile(packed); !bytes.Equal(first, again) {
		t.Error("packed twice, the bytes differ")
	}
	zr, err := zip.NewReader(bytes.NewReader(first), int64(len(first)))
	must(err)
	var names []string
	for _, f := range zr.File {
		names = append(names, f.Name)
	}
	sort.Strings(names)
	got := strings.Join(names, " ")
	for _, w := range []string{"model.tlspr", "parts/part.tlspr", "parts/cylinder.step", "parts/block.stl", "cache/step/"} {
		if !strings.Contains(got, w) {
			t.Errorf("the pack holds %s, not %s", got, w)
		}
	}

	// Elsewhere, with the originals gone.
	must(os.RemoveAll(src))
	moved := filepath.Join(t.TempDir(), "elsewhere.tlsprz")
	must(os.WriteFile(moved, first, 0o644))
	opened, err := api.Open(moved)
	must(err)
	r := opened.Active().Check()
	if math.Abs(r.Volume-want.Volume) > 1e-6*want.Volume || len(opened.Groups()) != len(asm.Groups()) {
		t.Errorf("opened: volume %g, want %g; %d groups, want %d", r.Volume, want.Volume, len(opened.Groups()), len(asm.Groups()))
	}
	// Saved packed again from there, the same model.
	repacked := filepath.Join(t.TempDir(), "again.tlsprz")
	must(opened.Save(repacked))
	if again, _ := os.ReadFile(repacked); !bytes.Equal(first, again) {
		t.Error("repacked from the opened copy, the bytes differ")
	}
}

// A pack whose file would be written outside its folder is refused.
func TestPackedModelOutside(t *testing.T) {
	var b bytes.Buffer
	zw := zip.NewWriter(&b)
	w, _ := zw.Create("../evil.tlspr")
	_, _ = w.Write([]byte("telesapr 2\n"))
	_ = zw.Close()
	path := filepath.Join(t.TempDir(), "evil.tlsprz")
	if err := os.WriteFile(path, b.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := tlspr.Unpack(path, t.TempDir()); err == nil || !strings.Contains(err.Error(), "outside") {
		t.Errorf("unpacked a file outside: %v", err)
	}
}
