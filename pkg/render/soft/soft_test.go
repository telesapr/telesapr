package soft

import (
	"bytes"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/spacesolve/spacesolve/pkg/core"
	"github.com/spacesolve/spacesolve/pkg/geom"
	"github.com/spacesolve/spacesolve/pkg/render"
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

func testCamera(w, h float64) render.Camera {
	var c render.Camera
	c.LoadIdentity()
	c.Width, c.Height, c.PixelRatio = w, h, 1
	c.Scale = 5
	return c
}

func isoCamera(w, h float64) render.Camera {
	c := testCamera(w, h)
	c.ProjRight = geom.Vector{X: 0.7071, Y: -0.7071}
	c.ProjUp = geom.Vector{X: 0.4082, Y: 0.4082, Z: 0.8165}
	c.NormalizeProjectionVectors()
	return c
}

func defaultLighting() render.Lighting {
	return render.Lighting{
		BackgroundColor:  black,
		AmbientIntensity: 0.3,
		LightIntensity:   [2]float64{1.0, 0.5},
		LightDirection:   [2]geom.Vector{{X: -1, Y: 1}, {X: 1}},
	}
}

// checkGolden compares img with testdata/render/soft/<name>.png, or
// writes it when UPDATE_GOLDENS is set.
func checkGolden(t *testing.T, name string, img *image.RGBA) {
	t.Helper()
	path := filepath.Join("..", "..", "..", "testdata", "render", "soft", name+".png")
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	if os.Getenv("UPDATE_GOLDENS") != "" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("golden: %v (run with UPDATE_GOLDENS=1)", err)
	}
	defer func() { _ = f.Close() }()
	want, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	b := img.Bounds()
	if want.Bounds() != b {
		t.Fatalf("size %v, golden %v", b, want.Bounds())
	}
	diff := 0
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r1, g1, b1, a1 := img.At(x, y).RGBA()
			r2, g2, b2, a2 := want.At(x, y).RGBA()
			if r1 != r2 || g1 != g2 || b1 != b2 || a1 != a2 {
				diff++
			}
		}
	}
	if diff != 0 {
		t.Errorf("%d pixels differ from %s", diff, path)
	}
}

func frame(r *Renderer, cam render.Camera, draw func()) *image.RGBA {
	r.SetLighting(defaultLighting())
	r.SetCamera(cam)
	r.StartFrame()
	draw()
	r.FlushFrame()
	r.FinishFrame()
	return r.ReadFrame()
}

func v(x, y, z float64) geom.Vector { return geom.Vector{X: x, Y: y, Z: z} }

func TestSoftLayers(t *testing.T) {
	r := New()
	img := frame(r, testCamera(200, 160), func() {
		near := r.GetFill(render.Fill{Layer: render.LayerNormal, Color: red})
		far := r.GetFill(render.Fill{Layer: render.LayerNormal, Color: blue})
		back := r.GetFill(render.Fill{Layer: render.LayerBack, Color: core.NewRgbaColor(60, 60, 60, 255)})
		// Drawn last, but behind everything.
		r.DrawQuad(v(-18, -14, 0), v(-18, 14, 0), v(18, 14, 0), v(18, -14, 0), back)
		r.DrawQuad(v(-12, -8, 10), v(-12, 8, 10), v(6, 8, 10), v(6, -8, 10), near)
		// Behind the red quad (z is toward the viewer).
		r.DrawQuad(v(-4, -12, -10), v(-4, 4, -10), v(14, 4, -10), v(14, -12, -10), far)

		occluded := r.GetStroke(render.Stroke{Layer: render.LayerOccluded, Color: green, Width: 2, Unit: render.UnitPX,
			StipplePattern: core.StippleDash, StippleScale: 4})
		r.DrawLine(v(-16, 2, -20), v(16, 2, -20), occluded)
		front := r.GetStroke(render.Stroke{Layer: render.LayerFront, Color: white, Width: 1, Unit: render.UnitPX})
		r.DrawLine(v(-16, -2, -50), v(16, -2, -50), front)
		// An invisible quad that hides later lines in front of it.
		depthOnly := r.GetFill(render.Fill{Layer: render.LayerDepthOnly, Color: white})
		r.DrawQuad(v(8, 6, 20), v(8, 12, 20), v(16, 12, 20), v(16, 6, 20), depthOnly)
		normal := r.GetStroke(render.Stroke{Layer: render.LayerNormal, ZIndex: 1, Color: white, Width: 1.5, Unit: render.UnitPX})
		r.DrawLine(v(-16, 9, 15), v(16, 9, 15), normal)
		// Translucent, checkered.
		glass := r.GetFill(render.Fill{Layer: render.LayerNormal, ZIndex: 2, Color: core.NewRgbaColor(255, 255, 0, 128),
			Pattern: render.FillCheckeredA})
		r.DrawQuad(v(-16, -13, 30), v(-16, -9, 30), v(-8, -9, 30), v(-8, -13, 30), glass)
	})
	checkGolden(t, "layers", img)
}

func TestSoftLines(t *testing.T) {
	r := New()
	img := frame(r, testCamera(240, 200), func() {
		y := 18.0
		for _, w := range []float64{0.5, 1, 1.5, 2, 3, 5, 9} {
			hcs := r.GetStroke(render.Stroke{Color: white, Width: w, Unit: render.UnitPX})
			r.DrawLine(v(-22, y, 0), v(-2, y-3, 0), hcs)
			y -= 5
		}
		y = 18.0
		for p := core.StippleShortDash; p <= core.StippleFreehand; p++ {
			hcs := r.GetStroke(render.Stroke{Color: green, Width: 1.5, Unit: render.UnitPX, StipplePattern: p, StippleScale: 6})
			r.DrawLine(v(2, y, 0), v(22, y, 0), hcs)
			y -= 5
		}
		pt := r.GetStroke(render.Stroke{Color: red, Width: 7, Unit: render.UnitPX})
		r.DrawPoint(v(-15, -18, 0), pt)
		mm := r.GetStroke(render.Stroke{Color: blue, Width: 0.8, Unit: render.UnitMM})
		var el srf.SEdgeList
		el.AddEdge(v(-10, -18, 0), v(0, -12, 0), 0, 0, 0)
		el.AddEdge(v(0, -12, 0), v(10, -18, 0), 0, 0, 0)
		r.DrawEdges(&el, mm)
	})
	checkGolden(t, "lines", img)
}

func TestSoftText(t *testing.T) {
	r := New()
	r.SetBitmapFont(sharedBitmapFont())
	cam := testCamera(260, 90)
	cam.GridFit = true
	img := frame(r, cam, func() {
		hcs := r.GetStroke(render.Stroke{Color: white, Width: 1, Unit: render.UnitPX})
		r.DrawVectorText("SolveSpace 3.2", 2.5, v(-24, 2, 0), v(1, 0, 0), v(0, 1, 0), hcs)

		ui := render.CameraForPixels(r.camera)
		r.SetCamera(ui)
		u := &render.UiCanvas{Canvas: r, Flip: true}
		u.DrawRect(4, 120, 60, 84, core.NewRgbaColor(30, 30, 90, 255), white, 0)
		u.DrawBitmapText("Aa  日本", 8, 64, core.NewRgbaColor(255, 255, 0, 255), 1)
		u.DrawPixmap(res.LoadPng("icons/graphics-window/line.png"), 200, 50, 0)
	})
	checkGolden(t, "text", img)
}

func TestSoftPolygon(t *testing.T) {
	r := New()
	img := frame(r, isoCamera(200, 160), func() {
		var p srf.SPolygon
		p.Normal = v(0, 0, 1)
		outer := srf.SContour{}
		for _, q := range []geom.Vector{v(-15, -15, 0), v(15, -15, 0), v(15, 15, 0), v(-15, 15, 0)} {
			outer.Points = append(outer.Points, srf.SPoint{P: q})
		}
		hole := srf.SContour{}
		for _, q := range []geom.Vector{v(-5, -5, 0), v(5, -5, 0), v(5, 5, 0), v(-5, 5, 0)} {
			hole.Points = append(hole.Points, srf.SPoint{P: q})
		}
		p.Contours = []srf.SContour{outer, hole}
		hcf := r.GetFill(render.Fill{Color: core.NewRgbaColor(0, 180, 180, 160)})
		r.DrawPolygon(&p, hcf)
		chk := r.GetFill(render.Fill{ZIndex: 1, Color: red, Pattern: render.FillCheckeredB})
		r.DrawQuad(v(-4, -4, 0), v(4, -4, 0), v(4, 4, 0), v(-4, 4, 0), chk)
	})
	checkGolden(t, "polygon", img)
}

var bitmapFont *res.BitmapFont

func sharedBitmapFont() *res.BitmapFont {
	if bitmapFont == nil {
		bitmapFont = res.NewBitmapFont()
	}
	return bitmapFont
}
