package ui

import (
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/telesapr/telesapr/pkg/script/api"
)

// In the application a motion study saves a picture of each step, of
// the view the window shows (roadmap 16.3).
func TestMotionStudyPictures(t *testing.T) {
	t.Parallel()
	ss, _ := newTestApp(t)
	doc := api.NewDoc(ss.ScriptHost())
	if err := doc.Params.Set("w", "20"); err != nil {
		t.Fatal(err)
	}
	sides := doc.Sketch.Rect(api.V(0, 0, 0), api.V(20, 10, 0))
	if err := doc.Constrain.Length(sides[0], 20).SetExpr("w"); err != nil {
		t.Fatal(err)
	}
	doc.Features.Extrude(5, "")
	dir := t.TempDir()
	frames, err := doc.Assembly.MotionStudy("w", 10, 20, 2, filepath.Join(dir, "f%d.png"))
	if err != nil || len(frames) != 3 {
		t.Fatalf("%d frames, %v", len(frames), err)
	}
	for _, f := range frames {
		file, err := os.Open(f.File)
		if err != nil {
			t.Fatal(err)
		}
		img, err := png.Decode(file)
		_ = file.Close()
		if err != nil || img.Bounds().Dx() != 800 || math.Abs(f.Volume-f.Value*10*5) > 1e-6 {
			t.Errorf("frame %+v: %v", f, err)
		}
	}
}
