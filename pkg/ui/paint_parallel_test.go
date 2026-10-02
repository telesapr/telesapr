package ui

import (
	"bytes"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/spacesolve/spacesolve/pkg/platform/headless"
)

// paintFrame renders a fixture at 1920x1080 and returns a copy of the
// frame.
func paintFrame(t *testing.T, fixture string) []byte {
	p := headless.New()
	ss := New(p)
	ss.Init()
	if !loadSlvs(t, ss, filepath.Join("..", "..", "testdata", fixture)) {
		t.Fatal("load failed")
	}
	w := ss.GW.window.(*headless.Window)
	w.Width, w.Height = 1920, 1080
	ss.GW.ZoomToFit(false, false)
	w.Render()
	return bytes.Clone(w.Frame.Pix)
}

// TestPaintParallelSameAsSerial checks that meshes rasterised in bands
// at once give the same frame as one thread.
func TestPaintParallelSameAsSerial(t *testing.T) {
	if runtime.GOMAXPROCS(0) < 2 {
		t.Skip("needs two threads")
	}
	for _, f := range []string{
		"group/translate_nd/normal.slvs",
		"request/ttf_text/extrude.slvs",
		"group/boolean_tangent_fillet/normal.slvs",
	} {
		par := paintFrame(t, f)
		old := runtime.GOMAXPROCS(1)
		ser := paintFrame(t, f)
		runtime.GOMAXPROCS(old)
		if !bytes.Equal(par, ser) {
			t.Errorf("%s: parallel frame differs from serial", f)
		}
	}
}
