//go:build cgo && !nogl

package gl

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/spacesolve/spacesolve/pkg/core"
	"github.com/spacesolve/spacesolve/pkg/geom"
	"github.com/spacesolve/spacesolve/pkg/render"
	"github.com/spacesolve/spacesolve/pkg/render/gl/glx"
	"github.com/spacesolve/spacesolve/pkg/render/soft"
	"github.com/spacesolve/spacesolve/pkg/res"
	"github.com/spacesolve/spacesolve/pkg/srf"
)

var (
	red   = core.NewRgbaColor(220, 40, 40, 255)
	green = core.NewRgbaColor(40, 200, 40, 255)
	blue  = core.NewRgbaColor(40, 80, 230, 255)
	white = core.NewRgbaColor(255, 255, 255, 255)
	black = core.NewRgbaColor(0, 0, 0, 255)
)

const width, height = 240, 180

// newGL makes a renderer on an offscreen context, or skips the test
// where there is no X display with GLX. The calling test's goroutine
// stays on its thread, as GL wants.
func newGL(t *testing.T) *Renderer {
	t.Helper()
	runtime.LockOSThread()
	t.Cleanup(runtime.UnlockOSThread)
	ctx, err := glx.NewOffscreen(width, height)
	if err != nil {
		t.Skipf("no GL: %v", err)
	}
	t.Cleanup(ctx.Destroy)
	r, err := New(ctx)
	if err != nil {
		t.Fatal(err)
	}
	return r.(*Renderer)
}

func isoCamera() render.Camera {
	var c render.Camera
	c.LoadIdentity()
	c.Width, c.Height, c.PixelRatio = width, height, 1
	c.Scale = 5
	c.ProjRight = geom.Vector{X: 0.7071, Y: -0.7071}
	c.ProjUp = geom.Vector{X: 0.4082, Y: 0.4082, Z: 0.8165}
	c.NormalizeProjectionVectors()
	return c
}

func lighting() render.Lighting {
	return render.Lighting{
		BackgroundColor:  black,
		AmbientIntensity: 0.3,
		LightIntensity:   [2]float64{1.0, 0.5},
		LightDirection:   [2]geom.Vector{{X: -1, Y: 1}, {X: 1}},
	}
}

// box is an axis-aligned box mesh, its faces outward and numbered.
func box(lo, hi geom.Vector, color core.RgbaColor) *srf.SMesh {
	var m srf.SMesh
	v := func(x, y, z int) geom.Vector {
		p := lo
		if x == 1 {
			p.X = hi.X
		}
		if y == 1 {
			p.Y = hi.Y
		}
		if z == 1 {
			p.Z = hi.Z
		}
		return p
	}
	quads := [][4][3]int{
		{{0, 0, 0}, {0, 1, 0}, {1, 1, 0}, {1, 0, 0}},
		{{0, 0, 1}, {1, 0, 1}, {1, 1, 1}, {0, 1, 1}},
		{{0, 0, 0}, {1, 0, 0}, {1, 0, 1}, {0, 0, 1}},
		{{0, 1, 0}, {0, 1, 1}, {1, 1, 1}, {1, 1, 0}},
		{{0, 0, 0}, {0, 0, 1}, {0, 1, 1}, {0, 1, 0}},
		{{1, 0, 0}, {1, 1, 0}, {1, 1, 1}, {1, 0, 1}},
	}
	for i, q := range quads {
		meta := srf.STriMeta{Face: srf.FaceID(i + 1), RgbaColor: color}
		a, b, c, d := v(q[0][0], q[0][1], q[0][2]), v(q[1][0], q[1][1], q[1][2]),
			v(q[2][0], q[2][1], q[2][2]), v(q[3][0], q[3][1], q[3][2])
		m.AddTriangle3(meta, a, b, c)
		m.AddTriangle3(meta, a, c, d)
	}
	return &m
}

// scene draws a bit of everything a canvas can draw.
func scene(c render.ViewportCanvas) {
	c.SetLighting(lighting())
	c.SetCamera(isoCamera())
	c.StartFrame()

	front := c.GetFill(render.Fill{Layer: render.LayerNormal, Color: core.RgbaColor{}})
	c.DrawMesh(box(geom.Vector{X: -8, Y: -8, Z: -8}, geom.Vector{X: 8, Y: 8, Z: 8}, blue), front, 0)

	thin := c.GetStroke(render.Stroke{Layer: render.LayerNormal, ZIndex: 1, Color: white, Width: 1, Unit: render.UnitPX})
	fat := c.GetStroke(render.Stroke{Layer: render.LayerFront, Color: red, Width: 6, Unit: render.UnitPX})
	dashed := c.GetStroke(render.Stroke{Layer: render.LayerFront, Color: green, Width: 2, Unit: render.UnitPX,
		StipplePattern: core.StippleDash, StippleScale: 4})
	c.DrawLine(geom.Vector{X: -8, Y: -8, Z: 8}, geom.Vector{X: 8, Y: -8, Z: 8}, thin)
	c.DrawLine(geom.Vector{X: -14, Y: 12}, geom.Vector{X: 14, Y: 12}, fat)
	c.DrawLine(geom.Vector{X: -14, Y: -14}, geom.Vector{X: 14, Y: -14}, dashed)

	var p srf.SPolygon
	p.Normal = geom.Vector{Z: 1}
	sq := func(r float64) {
		p.AddEmptyContour()
		sc := &p.Contours[len(p.Contours)-1]
		for _, xy := range [][2]float64{{-r, -r}, {r, -r}, {r, r}, {-r, r}, {-r, -r}} {
			sc.AddPoint(geom.Vector{X: xy[0] + 12, Y: xy[1] - 12, Z: 10})
		}
	}
	sq(4)
	sq(2)
	poly := c.GetFill(render.Fill{Layer: render.LayerFront, Color: green})
	c.DrawPolygon(&p, poly)
	checker := c.GetFill(render.Fill{Layer: render.LayerFront, Color: red, Pattern: render.FillCheckeredA})
	c.DrawQuad(geom.Vector{X: -16, Y: 6, Z: 10}, geom.Vector{X: -10, Y: 6, Z: 10},
		geom.Vector{X: -10, Y: 12, Z: 10}, geom.Vector{X: -16, Y: 12, Z: 10}, checker)
	c.FlushFrame()

	ui := render.CameraForPixels(*c.GetCamera())
	c.SetCamera(ui)
	u := &render.UiCanvas{Canvas: c}
	u.DrawRect(10, 60, 10, 30, white.WithAlpha(128), white, 0)
	u.DrawBitmapText("GL", 20, 30, red, 0)
	c.FlushFrame()
	c.FinishFrame()
}

func newSoft() *soft.Renderer {
	r := soft.New()
	return r
}

// diff counts the pixels where a channel differs by more than tol.
func diff(a, b *image.RGBA, tol int) int {
	n := 0
	for i := 0; i+3 < len(a.Pix); i += 4 {
		for k := range 3 {
			d := int(a.Pix[i+k]) - int(b.Pix[i+k])
			if d > tol || d < -tol {
				n++
				break
			}
		}
	}
	return n
}

func dump(t *testing.T, name string, img *image.RGBA) {
	dir := os.Getenv("SPACESOLVE_GL_DUMP")
	if dir == "" {
		return
	}
	f, err := os.Create(filepath.Join(dir, name+".png"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}

// The GL renderer follows the software one: the same scene differs
// only in a few pixels, at antialiased line edges and triangle seams.
func TestMatchesSoftware(t *testing.T) {
	g := newGL(t)
	scene(g)
	got := g.ReadFrame()
	s := newSoft()
	scene(s)
	want := s.ReadFrame()
	if got.Bounds() != want.Bounds() {
		t.Fatalf("bounds %v, software %v", got.Bounds(), want.Bounds())
	}
	dump(t, "gl", got)
	dump(t, "soft", want)
	n := diff(got, want, 24)
	t.Logf("%d of %d pixels differ", n, width*height)
	if n > width*height/50 {
		t.Errorf("%d of %d pixels differ from the software renderer", n, width*height)
	}
}

func TestFaceColour(t *testing.T) {
	g := newGL(t)
	g.SetLighting(render.Lighting{BackgroundColor: black, AmbientIntensity: 1})
	c := isoCamera()
	c.ProjRight, c.ProjUp = geom.Vector{X: 1}, geom.Vector{Y: 1}
	g.SetCamera(c)
	g.StartFrame()
	hcf := g.GetFill(render.Fill{Color: core.RgbaColor{}})
	g.DrawMesh(box(geom.Vector{X: -8, Y: -8, Z: -8}, geom.Vector{X: 8, Y: 8, Z: 8}, red), hcf, 0)
	g.FinishFrame()
	img := g.ReadFrame()
	px := img.RGBAAt(width/2, height/2)
	// Ambient light only, of intensity one: the face shows its colour.
	if d := int(px.R) - int(red.Red); d < -2 || d > 2 || px.G > red.Green+2 || px.B > red.Blue+2 {
		t.Errorf("centre pixel %v, want the face colour %v", px, red)
	}
	if bg := img.RGBAAt(2, 2); bg.R != 0 || bg.G != 0 || bg.B != 0 || bg.A != 255 {
		t.Errorf("corner pixel %v, want the opaque background", bg)
	}
	if v, r, ver := g.GetIdent(); v == "" || r == "" || ver == "" {
		t.Errorf("ident %q %q %q", v, r, ver)
	}
}

// A pixmap drawn, changed and invalidated is uploaded again.
func TestInvalidatePixmap(t *testing.T) {
	g := newGL(t)
	g.SetLighting(lighting())
	g.SetCamera(render.CameraForPixels(isoCamera()))
	pm := res.NewPixmap(res.FormatRGBA, 4, 4)
	set := func(c core.RgbaColor) {
		for i := 0; i+3 < len(pm.Data); i += 4 {
			pm.Data[i], pm.Data[i+1], pm.Data[i+2], pm.Data[i+3] = c.Red, c.Green, c.Blue, 255
		}
	}
	u := &render.UiCanvas{Canvas: g}
	for _, c := range []core.RgbaColor{red, green} {
		set(c)
		g.InvalidatePixmap(pm)
		g.StartFrame()
		u.DrawPixmap(pm, 0, 0, 0)
		g.FinishFrame()
		px := g.ReadFrame().RGBAAt(1, height-2)
		if px.R != c.Red || px.G != c.Green || px.B != c.Blue {
			t.Errorf("pixmap pixel %v, want %v", px, c)
		}
	}
}
