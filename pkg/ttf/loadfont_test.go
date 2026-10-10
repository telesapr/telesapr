package ttf

import (
	"path/filepath"
	"testing"
)

// TestLoadFontReadsOnlyItsFile finds a font by its file's basename
// without reading the list's other files: one of them doesn't exist,
// and the list is never loaded whole.
func TestLoadFontReadsOnlyItsFile(t *testing.T) {
	t.Parallel()
	gentium := filepath.Join("..", "..", "testdata", "Gentium-R.ttf")
	fl := NewFontList([]string{filepath.Join(t.TempDir(), "Missing.ttf"), gentium})

	tf := fl.LoadFont("Gentium-R.ttf")
	if tf == nil || tf.face == nil || tf.FontFile != gentium {
		t.Fatalf("Gentium-R.ttf: %+v", tf)
	}
	if again := fl.LoadFont("Gentium-R.ttf"); again != tf {
		t.Error("the font was opened a second time")
	}
	if b := fl.LoadFont("BitstreamVeraSans-Roman-builtin.ttf"); b == nil || !b.IsResource() {
		t.Errorf("the builtin font: %+v", b)
	}
	for _, name := range []string{"Missing.ttf", "Nowhere.ttf"} {
		if tf := fl.LoadFont(name); tf != nil {
			t.Errorf("%s: %+v, want nil", name, tf)
		}
	}
	if fl.loaded {
		t.Error("LoadFont loaded every font")
	}
	if r := fl.AspectRatio("Gentium-R.ttf", "AB", false); r <= 0 {
		t.Errorf("AspectRatio = %g", r)
	}
}
