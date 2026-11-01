package ui

import (
	"image"
	"os"
	"path/filepath"
	"testing"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/platform/headless"
	"github.com/telesapr/telesapr/pkg/script/api"
)

// rodShade renders a rod of 10 mm across, 30 long, threaded M10 12 mm
// of it as thread says ("" for none, "cosmetic", else modelled),
// and gives the mean brightness, 0 to 255, of the lit pixels of the
// window's middle, where the rod is.
func rodShade(t *testing.T, thread string) float64 {
	t.Helper()
	ss, _ := newTestApp(t)
	doc := api.NewDoc(ss.ScriptHost())
	doc.Sketch.Circle(api.V(0, 0, 0), 5)
	rod := doc.Features.Extrude(30, "rod")
	if thread != "" {
		doc.Features.ThreadOnFace(rod.Solid().FaceAt(api.V(5, 0, 15)),
			api.ThreadOptions{Designation: "M10", Length: 12, Cosmetic: thread == "cosmetic"})
	}
	w := ss.GW.window.(*headless.Window)
	w.Width, w.Height = 600, 600
	// From the side, a little above, as the guide's pictures show it.
	v := &ss.GW.view
	v.ProjRight = geom.Vector{X: 1}
	v.ProjUp = geom.Vector{Y: -0.5, Z: 1}
	ss.GW.NormalizeProjectionVectors()
	ss.GW.ZoomToFit(false, false)
	w.Render()
	if dir := os.Getenv("TELESAPR_RENDER_DIR"); dir != "" {
		writeRodPNG(t, filepath.Join(dir, "rod-"+thread+".png"), w.Frame)
	}
	var sum, n float64
	for y := 200; y < 400; y++ {
		for x := 250; x < 450; x++ {
			c := w.Frame.RGBAAt(x, y)
			if c.R == 0 && c.G == 0 && c.B == 0 {
				continue
			}
			sum += (float64(c.R) + float64(c.G) + float64(c.B)) / 3
			n++
		}
	}
	if n < 1000 {
		t.Fatalf("thread %q: only %g pixels of the rod drawn", thread, n)
	}
	return sum / n
}

func writeRodPNG(t *testing.T, path string, img *image.RGBA) {
	t.Helper()
	if err := writeFramePng(img, path); err != nil {
		t.Fatal(err)
	}
}

// TestCosmeticThreadShaded requires a rod with a cosmetic thread, its
// sketch the active group, shaded as the rod is without one: the solid
// is dimmed only for a sketch the user draws in.
func TestCosmeticThreadShaded(t *testing.T) {
	t.Parallel()
	plain := rodShade(t, "")
	cosmetic := rodShade(t, "cosmetic")
	t.Logf("plain %.1f, cosmetic %.1f", plain, cosmetic)
	if cosmetic < 0.85*plain {
		t.Errorf("the cosmetic thread's rod at %.1f, the plain rod at %.1f", cosmetic, plain)
	}
}
