//go:build cgo && !nogl

package tkgui

import (
	"runtime"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/platform"
	"github.com/telesapr/telesapr/pkg/render"
	"github.com/telesapr/telesapr/pkg/render/gl"
	"github.com/telesapr/telesapr/pkg/render/soft"
	"github.com/telesapr/telesapr/pkg/srf"
)

// square is a mesh of one square face, facing +z, of half-size h.
func square(h float64, color core.RGBA) *srf.SMesh {
	var m srf.SMesh
	meta := srf.STriMeta{Face: 1, RGBA: color}
	a, b := geom.Vector{X: -h, Y: -h}, geom.Vector{X: h, Y: -h}
	c, d := geom.Vector{X: h, Y: h}, geom.Vector{X: -h, Y: h}
	m.AddTriangle3(meta, a, b, c)
	m.AddTriangle3(meta, a, c, d)
	return &m
}

// drawFace draws the square, lit by ambient light alone so that it
// shows its own colour, centred in a frame of the window's size.
func drawFace(w *Window, r render.ViewportCanvas, color core.RGBA) {
	width, height := w.ContentSize()
	var c render.Camera
	c.LoadIdentity()
	c.Width, c.Height, c.PixelRatio, c.Scale = width, height, 1, 5
	r.SetLighting(render.Lighting{BackgroundColor: core.NewRGBA(0, 0, 0, 255), AmbientIntensity: 1})
	r.SetCamera(c)
	r.StartFrame()
	r.DrawMesh(square(10, color), r.InternFill(render.Fill{}), 0)
	r.FlushFrame()
	r.FinishFrame()
}

// A window drawing with GL renders frames, and a pixel read back from
// inside a face has the face's colour.
func TestGLWindowRendersFace(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	p := requireGUI(t)
	w := p.CreateWindow(platform.WindowToplevel, nil).(*Window)
	w.SetMinContentSize(320, 240)
	red := core.NewRGBA(200, 30, 40, 255)

	var r render.ViewportCanvas
	renders := 0
	var centre [3]byte
	w.Events().OnRender = func() {
		if r == nil {
			r = w.CreateRenderer(true)
		}
		drawFace(w, r, red)
		frame := r.ReadFrame()
		b := frame.Bounds()
		px := frame.RGBAAt(b.Dx()/2, b.Dy()/2)
		centre = [3]byte{px.R, px.G, px.B}
		w.PresentFrame(r)
		renders++
	}
	w.SetVisible(true)
	run(p,
		func() { w.Invalidate() },
		func() {
			// An expose over a GL frame makes it render again.
			w.canvas.blit()
		},
		func() {},
	)
	if _, ok := r.(*gl.Renderer); !ok {
		t.Fatalf("renderer %T, want OpenGL", r)
	}
	if renders < 2 {
		t.Errorf("%d renders, want one more after the expose", renders)
	}
	if centre != [3]byte{red.Red, red.Green, red.Blue} {
		t.Errorf("centre pixel %v, want the face colour %v", centre, red)
	}
	if vendor, _, _ := r.Ident(); vendor == "" {
		t.Error("no GL vendor")
	}
}

// Switching renderers drops the GL context and makes it again.
func TestGLWindowSwitchesRenderer(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	p := requireGUI(t)
	w := p.CreateWindow(platform.WindowToplevel, nil).(*Window)
	w.SetMinContentSize(200, 150)
	w.SetVisible(true)
	var kinds []string
	green := core.NewRGBA(30, 200, 40, 255)
	run(p,
		func() {
			for _, useGL := range []bool{true, false, true} {
				r := w.CreateRenderer(useGL)
				switch r.(type) {
				case *gl.Renderer:
					kinds = append(kinds, "gl")
				case *soft.Renderer:
					kinds = append(kinds, "soft")
					if w.canvas.glCtx != nil {
						t.Error("software renderer kept the GL context")
					}
				}
				drawFace(w, r, green)
				w.PresentFrame(r)
			}
		},
	)
	if len(kinds) != 3 || kinds[0] != "gl" || kinds[1] != "soft" || kinds[2] != "gl" {
		t.Errorf("renderers %v, want gl soft gl", kinds)
	}
}
