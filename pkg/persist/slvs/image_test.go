package slvs

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/telesapr/telesapr/pkg/sketch"
)

// TestImageAspectFromFile: as in C++, an image request takes its
// aspect ratio from the image file when it can be read, over the saved
// one.
func TestImageAspectFromFile(t *testing.T) {
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
		os.MkdirAll(filepath.Dir(path), 0o755)
		os.WriteFile(path, data, 0o644)
		if withPng {
			os.WriteFile(filepath.Join(dir, "drawing.png"), png, 0o644)
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
