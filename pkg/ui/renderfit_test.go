package ui

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/telesapr/telesapr/pkg/geom"
)

func decodePNG(t *testing.T, path string) image.Image {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	return img
}

// modelBounds is the box of the picture's pixels of the solid: its grey
// faces and its light blue edges, not the background, the axes or the
// dashed reference planes.
func modelBounds(t *testing.T, path string) (image.Rectangle, image.Rectangle) {
	t.Helper()
	img := decodePNG(t, path)
	b := img.Bounds()
	box := image.Rectangle{Min: b.Max, Max: b.Min}
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, _ := img.At(x, y).RGBA()
			r, g, bl = r>>8, g>>8, bl>>8
			grey := r == g && g == bl && r > 24
			edge := r > 60 && g > 100 && bl > 150
			if !grey && !edge {
				continue
			}
			box.Min.X, box.Min.Y = min(box.Min.X, x), min(box.Min.Y, y)
			box.Max.X, box.Max.Y = max(box.Max.X, x+1), max(box.Max.Y, y+1)
		}
	}
	return box, b
}

// A picture of the window right after Align View to Workplane shows the
// view where the turn ends, not where it was when the script asked, and
// the window is left there (roadmap 23.4).
func TestRenderWindowAfterAlign(t *testing.T) {
	ss, p := newTestApp(t)
	ss.AnimationSpeed = 300
	scriptConsole(t, ss, `doc.Sketch.Rect(V(0, 0, 0), V(60, 30, 0))`)
	ss.GW.SetProjection(geom.Vector{X: 0.707, Z: -0.707}, geom.Vector{X: -0.408, Y: 0.816, Z: -0.408})
	dir := t.TempDir()
	early, late := filepath.Join(dir, "early.png"), filepath.Join(dir, "late.png")
	out := scriptConsole(t, ss,
		`doc.Edit.RunCommand("Align View to Workplane")`,
		fmt.Sprintf(`doc.File.Render(%q, 320, 240, "window")`, early),
	)
	if out != "nil\nnil\n" {
		t.Fatalf("the script showed %q", out)
	}
	if n := ss.GW.view.ProjRight.Cross(ss.GW.view.ProjUp); n.Z < 0.999 {
		t.Errorf("after the picture the view looks along %v, not the workplane's normal", n)
	}
	time.Sleep(time.Duration(ss.AnimationSpeed+50) * time.Millisecond)
	p.RunDueTimers(100)
	if out := scriptConsole(t, ss, fmt.Sprintf(`doc.File.Render(%q, 320, 240, "window")`, late)); out != "nil\n" {
		t.Fatalf("the script showed %q", out)
	}
	a, err := os.ReadFile(early)
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(late)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a, b) {
		t.Error("the picture taken during the turn differs from the one after it")
	}
}

// A rendered picture holds the whole model, in its middle, filling much
// of it, from every named view, at every size, whatever the window
// shows (roadmap 23.4).
func TestRenderFits(t *testing.T) {
	ss, _ := newTestApp(t)
	// The window looks somewhere else and closer: a render must not
	// start from its offset or scale.
	ss.GW.view.Offset.X, ss.GW.view.Offset.Y = 70, -40
	ss.GW.view.Scale = 20
	dir := t.TempDir()
	if d := os.Getenv("TELESAPR_RENDER_DIR"); d != "" {
		dir = d
	}
	views := []string{"isometric", "front", "top", "right", "back"}
	sizes := [][2]int{{200, 150}, {640, 480}, {300, 600}, {1024, 256}}
	scriptConsole(t, ss,
		`doc.Sketch.Rect(V(0, 0, 0), V(60, 30, 0))`,
		`doc.Features.Extrude(20, "box")`,
	)
	for _, view := range views {
		for _, size := range sizes {
			name := fmt.Sprintf("%s-%dx%d.png", view, size[0], size[1])
			path := filepath.Join(dir, name)
			if out := scriptConsole(t, ss, fmt.Sprintf(`doc.File.Render(%q, %d, %d, %q)`, path, size[0], size[1], view)); out != "nil\n" {
				t.Fatalf("%s: %s", name, out)
			}
			box, all := modelBounds(t, path)
			if box.Empty() {
				t.Errorf("%s: nothing drawn", name)
				continue
			}
			w, h := float64(all.Dx()), float64(all.Dy())
			if box.Min.X <= 0 || box.Min.Y <= 0 || box.Max.X >= all.Max.X || box.Max.Y >= all.Max.Y {
				t.Errorf("%s: the model %v touches the edge of %v", name, box, all)
			}
			cx, cy := float64(box.Min.X+box.Max.X)/2, float64(box.Min.Y+box.Max.Y)/2
			if d := cx - w/2; d > 0.06*w || d < -0.06*w {
				t.Errorf("%s: the model %v is off the middle across by %.0f px", name, box, d)
			}
			if d := cy - h/2; d > 0.06*h || d < -0.06*h {
				t.Errorf("%s: the model %v is off the middle up and down by %.0f px", name, box, d)
			}
			if fx, fy := float64(box.Dx())/w, float64(box.Dy())/h; max(fx, fy) < 0.6 {
				t.Errorf("%s: the model %v fills only %.2f by %.2f of it", name, box, fx, fy)
			}
		}
	}
}
