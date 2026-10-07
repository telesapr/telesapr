package sketch_test

import (
	"archive/zip"
	"bytes"
	"math"
	"os"
	"path/filepath"
	"slices"
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
	slices.Sort(names)
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

// A pack whose file declares more than MaxPackedFile is refused before
// anything is read, and one that inflates to more than it declares is
// refused by the zip reader when it is.
func TestPackedModelTooLarge(t *testing.T) {
	path := filepath.Join(t.TempDir(), "big.tlsprz")
	var b bytes.Buffer
	zw := zip.NewWriter(&b)
	w, _ := zw.CreateHeader(&zip.FileHeader{Name: "model.tlspr", Method: zip.Deflate})
	_, _ = w.Write([]byte("telesapr 2\n"))
	_ = zw.Close()
	data := b.Bytes()
	// The central directory's record of the uncompressed size is the
	// last of the three sizes after the file's name there.
	cd := bytes.LastIndex(data, []byte("model.tlspr"))
	cd = bytes.LastIndex(data[:cd], []byte{0x50, 0x4b, 0x01, 0x02})
	if cd < 0 {
		t.Fatal("no central directory")
	}
	declared := data[cd+24 : cd+28]
	copy(declared, []byte{0, 0, 0, 0x40}) // 1 GiB
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := tlspr.Unpack(path, t.TempDir()); err == nil || !strings.Contains(err.Error(), "more than") {
		t.Errorf("unpacked a file declared too large: %v", err)
	}
	copy(declared, []byte{1, 0, 0, 0}) // one byte, less than it holds
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := tlspr.Unpack(path, t.TempDir()); err == nil {
		t.Error("unpacked a file larger than declared")
	}
}

// A pack's tessellations go into the STEP cache only when the pack
// holds a model, and only by a plain name.
func TestPackedModelCacheNeedsModel(t *testing.T) {
	cache := t.TempDir()
	t.Setenv("TELESAPR_CACHE_DIR", cache)
	pack := func(names ...string) string {
		var b bytes.Buffer
		zw := zip.NewWriter(&b)
		for _, n := range names {
			w, _ := zw.Create(n)
			_, _ = w.Write([]byte("telesapr 2\n"))
		}
		_ = zw.Close()
		path := filepath.Join(t.TempDir(), "p.tlsprz")
		if err := os.WriteFile(path, b.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	if _, err := tlspr.Unpack(pack("cache/step/orphan.mesh"), t.TempDir()); err == nil {
		t.Error("unpacked a pack without a model")
	}
	if _, err := os.Stat(filepath.Join(cache, "step", "orphan.mesh")); err == nil {
		t.Error("a pack without a model wrote into the STEP cache")
	}
	if _, err := tlspr.Unpack(pack("model.tlspr", "cache/step/kept.mesh", "cache/step/sub/deep.mesh"), t.TempDir()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(cache, "step", "kept.mesh")); err != nil {
		t.Error("the pack's tessellation was not kept:", err)
	}
	if _, err := os.Stat(filepath.Join(cache, "step", "sub", "deep.mesh")); err == nil {
		t.Error("a tessellation in a folder was written")
	}
}
