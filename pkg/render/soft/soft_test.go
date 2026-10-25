package soft

import (
	"bytes"
	"image"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/internal/golden"
	"github.com/telesapr/telesapr/pkg/render"
	"github.com/telesapr/telesapr/pkg/res"
	"github.com/telesapr/telesapr/pkg/srf"
)

var (
	red   = core.NewRGBA(220, 40, 40, 255)
	green = core.NewRGBA(40, 200, 40, 255)
	blue  = core.NewRGBA(40, 80, 230, 255)
	white = core.NewRGBA(255, 255, 255, 255)
	black = core.NewRGBA(0, 0, 0, 255)
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
// writes it when the goldens are regenerated (scripts/regen-golden.sh
// soft). The pixels are compared, not the PNG's bytes, which another
// encoder may write differently.
func checkGolden(t *testing.T, name string, img *image.RGBA) {
	t.Helper()
	path := filepath.Join("..", "..", "..", "testdata", "render", "soft", name+".png")
	if golden.Regen() {
		var buf bytes.Buffer
		if err := png.Encode(&buf, img); err != nil {
			t.Fatal(err)
		}
		golden.Check(t, path, buf.Bytes())
		return
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("golden: %v (record it with scripts/regen-golden.sh soft)", err)
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
	t.Parallel()
	r := New()
	img := frame(r, testCamera(200, 160), func() {
		near := r.InternFill(render.Fill{Layer: render.LayerNormal, Color: red})
		far := r.InternFill(render.Fill{Layer: render.LayerNormal, Color: blue})
		back := r.InternFill(render.Fill{Layer: render.LayerBack, Color: core.NewRGBA(60, 60, 60, 255)})
		// Drawn last, but behind everything.
		r.DrawQuad(v(-18, -14, 0), v(-18, 14, 0), v(18, 14, 0), v(18, -14, 0), back)
		r.DrawQuad(v(-12, -8, 10), v(-12, 8, 10), v(6, 8, 10), v(6, -8, 10), near)
		// Behind the red quad (z is toward the viewer).
		r.DrawQuad(v(-4, -12, -10), v(-4, 4, -10), v(14, 4, -10), v(14, -12, -10), far)

		occluded := r.InternStroke(render.Stroke{Layer: render.LayerOccluded, Color: green, Width: 2, Unit: render.UnitPX,
			StipplePattern: core.StippleDash, StippleScale: 4})
		r.DrawLine(v(-16, 2, -20), v(16, 2, -20), occluded)
		front := r.InternStroke(render.Stroke{Layer: render.LayerFront, Color: white, Width: 1, Unit: render.UnitPX})
		r.DrawLine(v(-16, -2, -50), v(16, -2, -50), front)
		// An invisible quad that hides later lines in front of it.
		depthOnly := r.InternFill(render.Fill{Layer: render.LayerDepthOnly, Color: white})
		r.DrawQuad(v(8, 6, 20), v(8, 12, 20), v(16, 12, 20), v(16, 6, 20), depthOnly)
		normal := r.InternStroke(render.Stroke{Layer: render.LayerNormal, ZIndex: 1, Color: white, Width: 1.5, Unit: render.UnitPX})
		r.DrawLine(v(-16, 9, 15), v(16, 9, 15), normal)
		// Translucent, checkered.
		glass := r.InternFill(render.Fill{Layer: render.LayerNormal, ZIndex: 2, Color: core.NewRGBA(255, 255, 0, 128),
			Pattern: render.FillCheckeredA})
		r.DrawQuad(v(-16, -13, 30), v(-16, -9, 30), v(-8, -9, 30), v(-8, -13, 30), glass)
	})
	checkGolden(t, "layers", img)
}

func TestSoftLines(t *testing.T) {
	t.Parallel()
	r := New()
	img := frame(r, testCamera(240, 200), func() {
		y := 18.0
		for _, w := range []float64{0.5, 1, 1.5, 2, 3, 5, 9} {
			hcs := r.InternStroke(render.Stroke{Color: white, Width: w, Unit: render.UnitPX})
			r.DrawLine(v(-22, y, 0), v(-2, y-3, 0), hcs)
			y -= 5
		}
		y = 18.0
		for p := core.StippleShortDash; p <= core.StippleFreehand; p++ {
			hcs := r.InternStroke(render.Stroke{Color: green, Width: 1.5, Unit: render.UnitPX, StipplePattern: p, StippleScale: 6})
			r.DrawLine(v(2, y, 0), v(22, y, 0), hcs)
			y -= 5
		}
		pt := r.InternStroke(render.Stroke{Color: red, Width: 7, Unit: render.UnitPX})
		r.DrawPoint(v(-15, -18, 0), pt)
		mm := r.InternStroke(render.Stroke{Color: blue, Width: 0.8, Unit: render.UnitMM})
		var el srf.SEdgeList
		el.AddEdge(v(-10, -18, 0), v(0, -12, 0), 0, 0, 0)
		el.AddEdge(v(0, -12, 0), v(10, -18, 0), 0, 0, 0)
		r.DrawEdges(&el, mm)
	})
	checkGolden(t, "lines", img)
}

func TestSoftText(t *testing.T) {
	t.Parallel()
	r := New()
	r.SetBitmapFont(sharedBitmapFont())
	cam := testCamera(260, 90)
	cam.GridFit = true
	img := frame(r, cam, func() {
		hcs := r.InternStroke(render.Stroke{Color: white, Width: 1, Unit: render.UnitPX})
		r.DrawVectorText("SolveSpace 3.2", 2.5, v(-24, 2, 0), v(1, 0, 0), v(0, 1, 0), hcs)

		ui := render.CameraForPixels(r.camera)
		r.SetCamera(ui)
		u := &render.UICanvas{Canvas: r, Flip: true}
		u.DrawRect(4, 120, 60, 84, core.NewRGBA(30, 30, 90, 255), white, 0)
		u.DrawBitmapText("Aa  日本", 8, 64, core.NewRGBA(255, 255, 0, 255), 1)
		u.DrawPixmap(res.LoadPng("icons/graphics-window/line.png"), 200, 50, 0)
	})
	checkGolden(t, "text", img)
}

func TestSoftPolygon(t *testing.T) {
	t.Parallel()
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
		hcf := r.InternFill(render.Fill{Color: core.NewRGBA(0, 180, 180, 160)})
		r.DrawPolygon(&p, hcf)
		chk := r.InternFill(render.Fill{ZIndex: 1, Color: red, Pattern: render.FillCheckeredB})
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

// benchmarkSoftLines draws 50k thin lines about length mm long (5
// pixels a mm) across a 1080p frame.
func benchmarkSoftLines(b *testing.B, length float64) {
	r := New()
	cam := testCamera(1920, 1080)
	var lines [][2]geom.Vector
	for i := range 50000 {
		// A spread of positions and directions, within the view.
		a := float64(i) * 0.61803398875
		x, y := 150*math.Sin(a*1.3), 90*math.Cos(a*0.7)
		dx, dy := length*math.Cos(a*5.1), length*math.Sin(a*5.1)
		lines = append(lines, [2]geom.Vector{v(x, y, 0), v(x+dx, y+dy, 0)})
	}
	for b.Loop() {
		frame(r, cam, func() {
			hs := r.InternStroke(render.Stroke{Layer: render.LayerNormal, Color: core.NewRGBA(255, 255, 255, 255),
				Width: 1, Unit: render.UnitPX})
			for _, l := range lines {
				r.DrawLine(l[0], l[1], hs)
			}
		})
	}
}

// BenchmarkSoftLines1080p: 50k lines of 10 pixels, as a detailed
// model's edges.
func BenchmarkSoftLines1080p(b *testing.B) { benchmarkSoftLines(b, 2) }

// BenchmarkSoftLongLines1080p: 50k lines of 100 pixels, about five
// million pixels drawn.
func BenchmarkSoftLongLines1080p(b *testing.B) { benchmarkSoftLines(b, 20) }

// BenchmarkSoftEdges1080p draws 50k edges of 10 pixels in one DrawEdges
// call, as a model's edges are drawn.
func BenchmarkSoftEdges1080p(b *testing.B) {
	r := New()
	cam := testCamera(1920, 1080)
	var el srf.SEdgeList
	for i := range 50000 {
		a := float64(i) * 0.61803398875
		x, y := 150*math.Sin(a*1.3), 90*math.Cos(a*0.7)
		dx, dy := 2*math.Cos(a*5.1), 2*math.Sin(a*5.1)
		el.AddEdge(v(x, y, 0), v(x+dx, y+dy, 0), 0, 0, 0)
	}
	for b.Loop() {
		frame(r, cam, func() {
			hs := r.InternStroke(render.Stroke{Layer: render.LayerNormal, Color: core.NewRGBA(255, 255, 255, 255),
				Width: 1, Unit: render.UnitPX})
			r.DrawEdges(&el, hs)
		})
	}
}

// The clip plane keeps one side of a plane, for quads, lines and
// planar polygons, in parallel and in perspective, and follows the
// camera it is drawn with.
func TestSoftClipPlane(t *testing.T) {
	t.Parallel()
	for _, tangent := range []float64{0, 0.003} {
		cam := testCamera(200, 160)
		cam.Tangent = tangent
		r := New()
		// Keep x + z <= 12: at z = 10, x <= 2.
		keep := srf.HalfSpace{N: v(1, 0, 1).WithMagnitude(1), D: 12 / math.Sqrt2}
		img := frame(r, testCamera(10, 10), func() {
			r.SetClipPlane(&keep)
			r.SetCamera(cam)
			fill := r.InternFill(render.Fill{Color: red})
			r.DrawQuad(v(-18, -14, 10), v(-18, 0, 10), v(18, 0, 10), v(18, -14, 10), fill)
			var p srf.SPolygon
			p.Normal = v(0, 0, 1)
			p.AddEmptyContour()
			for _, q := range []geom.Vector{v(-18, 2, 10), v(18, 2, 10), v(18, 10, 10), v(-18, 10, 10), v(-18, 2, 10)} {
				p.Contours[0].AddPoint(q)
			}
			r.DrawPolygon(&p, r.InternFill(render.Fill{Color: green}))
			fat := r.InternStroke(render.Stroke{Color: white, Width: 5, Unit: render.UnitPX})
			r.DrawLine(v(-18, 13, 10), v(18, 13, 10), fat)
			r.SetClipPlane(nil)
			r.DrawLine(v(-18, -1, 10), v(18, -1, 10), fat)
		})
		edge := cam.ProjectPoint(v(2, 0, 10))
		ex := int(math.Round(100 + edge.U))
		row := func(y float64) int { return int(math.Round(80 - cam.ProjectPoint(v(0, y, 10)).V)) }
		for _, c := range []struct {
			y    float64
			want core.RGBA
		}{{-7, red}, {6, green}, {13, white}} {
			y := row(c.y)
			in, out := img.RGBAAt(ex-4, y), img.RGBAAt(ex+4, y)
			if in.R != c.want.Red || in.G != c.want.Green || in.B != c.want.Blue {
				t.Errorf("tangent %v, y %v: kept pixel %v, want %v", tangent, c.y, in, c.want)
			}
			if out.R != 0 || out.G != 0 || out.B != 0 {
				t.Errorf("tangent %v, y %v: cut pixel %v, want the background", tangent, c.y, out)
			}
		}
		// Drawn after the plane was removed: not clipped.
		if px := img.RGBAAt(ex+4, row(-1)); px.R != 255 {
			t.Errorf("tangent %v: the line drawn without the plane is cut: %v", tangent, px)
		}
	}
}
