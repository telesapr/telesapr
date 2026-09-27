package slvs

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestStyleDefaults: a style block fills the fields it leaves out with
// the default style's, and an entity naming a style the file lacks
// gets a new custom style when saved; both as C++ writes them (checked
// with a C++ harness).
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
		return "Style.h.v=00000100\n" +
			"Style.name=" + name + "\n" +
			"Style.width=" + width + "\n" +
			"Style.textHeight=11.50000000000000000000\n" +
			"Style.color=00ffffff\n" +
			"Style.fillColor=004c4c4c\n" +
			"Style.filled=0\n" +
			"Style.visible=1\n" +
			"Style.exportable=1\n" +
			"Style.stippleScale=15.00000000000000000000\n" +
			"AddStyle\n"
	}
	for _, tc := range []struct {
		name string
		data []byte
		want string
	}{
		{"missing", styled, block("new-custom-style", "1.50000000000000000000")},
		{"partial", partial, block("thin", "0.50000000000000000000")},
	} {
		path := filepath.Join(t.TempDir(), tc.name+".slvs")
		os.WriteFile(path, tc.data, 0o644)
		doc, err := LoadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if saved := string(doc.Save()); !strings.Contains(saved, tc.want) {
			t.Errorf("%s: saved style isn't C++'s:\n%s", tc.name, saved[strings.LastIndex(saved, "Style.h.v"):])
		}
	}
}
