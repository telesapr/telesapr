package slvs

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spacesolve/spacesolve/pkg/persist/spcs"
)

// TestStyleDefaults: a style block fills the fields it leaves out with
// the default style's, and an entity naming a style the file lacks
// gets a new custom style when saved; both as C++ wrote them (checked
// with a C++ harness while the C++ comparison ran).
func TestStyleDefaults(t *testing.T) {
	data, err := os.ReadFile(fixture("request/line_segment/normal.slvs"))
	if err != nil {
		t.Fatal(err)
	}
	i := bytes.Index(data, []byte("Request.h.v=00000004"))
	j := i + bytes.Index(data[i:], []byte("AddRequest"))
	styled := append(append(append([]byte{}, data[:j]...), "Request.style=00000100\n"...), data[j:]...)
	partial := append(bytes.TrimRight(append([]byte{}, styled...), "\n"),
		"\n\nStyle.h.v=00000100\nStyle.name=thin\nStyle.width=0.50000000000000000000\nAddStyle\n\n"...)

	block := func(name, width string) string {
		return "style 0x100 \"" + name + "\" {\n" +
			"  width " + width + "\n" +
			"  text-height 11.5\n" +
			"  color 255 255 255 255\n" +
			"  fill-color 76 76 76 255\n" +
			"  visible\n" +
			"  exportable\n" +
			"  stipple-scale 15\n" +
			"}\n"
	}
	for _, tc := range []struct {
		name string
		data []byte
		want string
	}{
		{"missing", styled, block("new-custom-style", "1.5")},
		{"partial", partial, block("thin", "0.5")},
	} {
		path := filepath.Join(t.TempDir(), tc.name+".slvs")
		os.WriteFile(path, tc.data, 0o644)
		doc, err := LoadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if saved := string(spcs.Encode(doc.Sketch, spcs.Options{})); !strings.Contains(saved, tc.want) {
			t.Errorf("%s: saved style isn't the one C++ wrote:\n%s", tc.name, saved[strings.Index(saved, "\nstyle "):])
		}
	}
}
