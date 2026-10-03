package ui

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/telesapr/telesapr/pkg/script/api"
)

// The window shades by appearance (roadmap 19.1, 19.2): rendered with
// shading on, a metal block looks other than a matte one; with it off,
// the appearance changes nothing.
func TestShadedAppearance(t *testing.T) {
	ss, _ := newTestApp(t)
	doc := api.NewDoc(ss.ScriptHost())
	doc.Rect(api.V(0, 0, 0), api.V(20, 10, 0))
	block := doc.Extrude(10, "")
	dir := t.TempDir()
	render := func(shaded bool, appearance string) []byte {
		ss.Shaded = shaded
		block.SetAppearance(appearance)
		path := filepath.Join(dir, "view.png")
		if err := doc.Render(path, 200, 150, "isometric"); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	if bytes.Equal(render(true, "matte"), render(true, "metal")) {
		t.Error("shaded, matte and metal look the same")
	}
	if !bytes.Equal(render(false, "matte"), render(false, "glossy")) {
		t.Error("unshaded, the appearance changed the picture")
	}
	if bytes.Equal(render(false, "plastic"), render(true, "plastic")) {
		t.Error("shading changed nothing")
	}
}
