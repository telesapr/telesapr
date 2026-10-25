package slvs

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/telesapr/telesapr/pkg/sketch"
)

// TestImageAspectFromFile:, an image request takes its aspect ratio
// from the image file when it can be read, over the saved one.
func TestImageAspectFromFile(t *testing.T) {
	t.Parallel()
	_, thisFile, _, _ := runtime.Caller(0)
	src := filepath.Join(filepath.Dir(thisFile), "../../../testdata/request/image")
	data, err := os.ReadFile(filepath.Join(src, "normal.slvs"))
	if err != nil {
		t.Fatal(err)
	}
	png, err := os.ReadFile(filepath.Join(src, "drawing.png")) // 300 x 200
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	data = bytes.ReplaceAll(data, []byte("Request.aspectRatio=1.50000000000000000000"),
		[]byte("Request.aspectRatio=1.00000000000000000000"))
	for name, withPng := range map[string]bool{"with.slvs": true, "sub/without.slvs": false} {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
		if withPng {
			if err := os.WriteFile(filepath.Join(dir, "drawing.png"), png, 0o644); err != nil {
				t.Fatal(err)
			}
		}

		doc, err := LoadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		want := 1.0
		if withPng {
			want = 1.5
		}
		for _, r := range doc.Sketch.Requests {
			if r.Type == sketch.RequestImage && r.AspectRatio != want {
				t.Errorf("%s: aspect ratio %g, want %g", name, r.AspectRatio, want)
			}
		}
	}
}
