package api

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/sketch"
)

// copyTestdata copies the files of a testdata directory into a new
// temporary directory, and returns it.
func copyTestdata(t *testing.T, dir string) string {
	t.Helper()
	out := t.TempDir()
	from := filepath.Join("..", "..", "..", "testdata", filepath.FromSlash(dir))
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
		if err := os.WriteFile(filepath.Join(out, e.Name()), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return out
}

// TestImport converts SolveSpace models, alone and with what they link,
// and opens the result.
func TestImport(t *testing.T) {
	dir := copyTestdata(t, "group/translate_nd")
	d, err := Import(filepath.Join(dir, "normal.slvs"), filepath.Join(dir, "part.tlspr"))
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Groups()) < 3 || d.Groups()[len(d.Groups())-1].Analyze().Volume() <= 0 {
		t.Errorf("imported %v", d.Groups())
	}

	// Saved elsewhere and loaded again, it is the same model.
	other := filepath.Join(t.TempDir(), "again.tlspr")
	if err := d.File.Save(other); err != nil {
		t.Fatal(err)
	}
	if err := d.File.Load(other); err != nil {
		t.Fatal(err)
	}

	asm := copyTestdata(t, "group/link")
	d, err = Import(filepath.Join(asm, "normal.slvs"), filepath.Join(asm, "assembly.tlspr"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(asm, "rect_v20.tlspr")); err != nil {
		t.Errorf("the linked model wasn't converted: %v", err)
	}
	linked := false
	for _, g := range d.Groups() {
		linked = linked || g.Type() == sketch.GroupLinked
	}
	if !linked {
		t.Errorf("no link group in %v", d.Groups())
	}

	if err := os.Remove(filepath.Join(asm, "rect_v20.slvs")); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(asm, "rect_v20.tlspr")); err != nil {
		t.Fatal(err)
	}
	if _, err := Import(filepath.Join(asm, "normal.slvs"), filepath.Join(asm, "again.tlspr")); err == nil ||
		!strings.Contains(err.Error(), "missing linked files") || !strings.Contains(err.Error(), "rect_v20.slvs") {
		t.Errorf("missing link: %v", err)
	}
}

// TestFileMisuse: what can't be opened, saved or imported returns an
// error that says why.
func TestFileMisuse(t *testing.T) {
	dir := t.TempDir()
	fixture := filepath.Join("..", "..", "..", "testdata", "group", "translate_nd", "normal.slvs")
	d := New()

	for path, want := range map[string]string{
		filepath.Join(dir, "a.slvs"):        "doesn't write SolveSpace models",
		filepath.Join(dir, "a.txt"):         "a sketch is saved as a .tlspr file",
		filepath.Join(dir, "no", "a.tlspr"): "no such file",
	} {
		if err := d.File.Save(path); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("Save(%s): %v, want %q", path, err, want)
		}
	}

	if _, err := Open(fixture); err == nil || !strings.Contains(err.Error(), "convert it with Import first") {
		t.Errorf("Open of a .slvs file: %v", err)
	}
	if err := d.File.Load(fixture); err == nil || !strings.Contains(err.Error(), "convert it with Import first") {
		t.Errorf("Load of a .slvs file: %v", err)
	}
	if _, err := Open(filepath.Join(dir, "none.tlspr")); err == nil {
		t.Error("opened a missing file")
	}

	future := filepath.Join(dir, "future.tlspr")
	if err := d.File.Save(future); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(future)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(future, append(data, "\nhologram {\n}\n"...), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(future); err == nil || !strings.Contains(err.Error(), "a save would lose") ||
		!strings.Contains(err.Error(), `unknown statement "hologram"`) {
		t.Errorf("Open of a file with unknown parts: %v", err)
	}

	for _, c := range []struct{ src, dst, want string }{
		{fixture, filepath.Join(dir, "x.txt"), "is not a TeleSAPR file"},
		{future, filepath.Join(dir, "x.tlspr"), "is not a SolveSpace model"},
		{filepath.Join(dir, "none.slvs"), filepath.Join(dir, "x.tlspr"), "no such file"},
		{fixture, filepath.Join(dir, "no", "x.tlspr"), "no such file"},
	} {
		if _, err := Import(c.src, c.dst); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("Import(%s, %s): %v, want %q", c.src, c.dst, err, c.want)
		}
	}
}
