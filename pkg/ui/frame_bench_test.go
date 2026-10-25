//go:build cgo && !nogl

package ui

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/persist/convert"
	"github.com/telesapr/telesapr/pkg/platform/headless"
	"github.com/telesapr/telesapr/pkg/render"
	"github.com/telesapr/telesapr/pkg/render/gl"
	"github.com/telesapr/telesapr/pkg/render/gl/glx"
	"github.com/telesapr/telesapr/pkg/render/soft"
)

// frameModels are the models BenchmarkFrame orbits, from small to the
// largest example there is.
var frameModels = []struct{ name, path string }{
	{"boolean", "testdata/tlspr/golden/group/boolean_tangent_fillet/normal.tlspr"},
	{"bolt", "testdata/thread/bolt_m8x30.tlspr"},
	{"ball_mill", "examples/designs/ball-mill/asm.slvs"},
}

const frameW, frameH = 1920, 1080

// BenchmarkFrame times one frame of the graphics window, with each
// renderer, as the view orbits a model: the view turns a little each
// frame, so nothing comes from the persistent layer's cache. A frame is
// drawn and finished (GL waits for the card with glFinish), but not
// shown: putting it on screen costs the window system, not the
// renderer. The GL renderer draws on an offscreen pixel buffer of the
// X server DISPLAY names, so no window opens; under Xvfb that GL is
// itself software (llvmpipe), so run it on a real display to time a
// graphics card:
//
//	go test ./pkg/ui -run '^$' -bench BenchmarkFrame -benchtime 50x
//
// Each result also reports the mesh's triangles.
func BenchmarkFrame(b *testing.B) {
	for _, m := range frameModels {
		b.Run(m.name, func(b *testing.B) {
			for _, renderer := range []string{"software", "opengl"} {
				b.Run(renderer, func(b *testing.B) { benchmarkFrame(b, m.path, renderer) })
			}
		})
	}
}

func benchmarkFrame(b *testing.B, path, renderer string) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	var canvas render.ViewportCanvas
	switch renderer {
	case "software":
		canvas = soft.New()
	case "opengl":
		if os.Getenv("DISPLAY") == "" {
			b.Skip("no X display")
		}
		ctx, err := glx.NewOffscreen(frameW, frameH)
		if err != nil {
			b.Skipf("no GL: %v", err)
		}
		defer ctx.Destroy()
		if canvas, err = gl.New(ctx); err != nil {
			b.Skipf("no GL: %v", err)
		}
	}

	p := headless.New()
	ss := New(p)
	ss.Init()
	file := filepath.Join("..", "..", path)
	if strings.HasSuffix(file, ".slvs") {
		// An example drawn in SolveSpace: imported beside a copy of its
		// folder, as File > Import would.
		dir := b.TempDir()
		if err := os.CopyFS(dir, os.DirFS(filepath.Dir(file))); err != nil {
			b.Fatal(err)
		}
		src := filepath.Join(dir, filepath.Base(file))
		file = convert.SiblingPath(src)
		if _, _, err := convert.File(src, file, true); err != nil {
			b.Fatal(err)
		}
	}
	if !loadModel(b, ss, file) {
		b.Fatal("load failed")
	}
	w := ss.GW.window.(*headless.Window)
	w.Width, w.Height = frameW, frameH
	w.NoFrame = true
	ss.GW.canvas = canvas
	ss.GW.ZoomToFit(false, false)
	w.Render()

	v := &ss.GW.view
	b.ResetTimer()
	for range b.N {
		v.ProjRight = v.ProjRight.RotatedAroundAxis(geom.Vector{Y: 1}, 0.01)
		v.ProjUp = v.ProjUp.RotatedAroundAxis(geom.Vector{Y: 1}, 0.01)
		ss.GW.Invalidate(false)
		w.Render()
	}
	b.StopTimer()
	// After the loop: ResetTimer drops metrics reported before it.
	g := ss.SK.Group(ss.GW.activeGroupH())
	b.ReportMetric(float64(len(g.DisplayMesh.Triangles)), "triangles")
}
