package webgl

import (
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/render"
	"github.com/telesapr/telesapr/pkg/res"
	"github.com/telesapr/telesapr/pkg/srf"
)

// fakeDevice records what the renderer asks of it.
type fakeDevice struct {
	w, h     int
	draws    []drawCall
	clears   []bool
	flushes  int
	finishes int
	invalid  []*res.Pixmap
	closed   bool
	frame    []byte
	frameW   int
	frameH   int
}

func (f *fakeDevice) resize(w, h int)            { f.w, f.h = w, h }
func (f *fakeDevice) clear(_ [4]float32, c bool) { f.clears = append(f.clears, c) }
func (f *fakeDevice) draw(c *drawCall) {
	cp := *c
	cp.verts = append([]float32(nil), c.verts...)
	f.draws = append(f.draws, cp)
}
func (f *fakeDevice) invalidate(pm *res.Pixmap) { f.invalid = append(f.invalid, pm) }
func (f *fakeDevice) read() ([]byte, int, int)  { return f.frame, f.frameW, f.frameH }
func (f *fakeDevice) ident() (string, string, string) {
	return "v", "r", "ver"
}
func (f *fakeDevice) flush()  { f.flushes++ }
func (f *fakeDevice) finish() { f.finishes++ }
func (f *fakeDevice) close()  { f.closed = true }

func testCamera() render.Camera {
	return render.Camera{
		Width: 200, Height: 100, PixelRatio: 1, Scale: 2,
		ProjRight: geom.Vector{X: 1}, ProjUp: geom.Vector{Y: 1},
	}
}

func newTest() (*Renderer, *fakeDevice) {
	f := &fakeDevice{}
	r := newRenderer(f)
	r.SetCamera(testCamera())
	return r, f
}

// vertex i of a draw's stream.
func vert(c *drawCall, i int) []float32 { return c.verts[i*vertexFloats : (i+1)*vertexFloats] }

func TestProjectionMatchesCamera(t *testing.T) {
	t.Parallel()
	for _, tangent := range []float64{0, 0.0004} {
		c := testCamera()
		c.ProjRight = geom.Vector{X: 0.8, Y: 0.6}
		c.ProjUp = geom.Vector{X: -0.6 * 0.8, Y: 0.8 * 0.8, Z: 0.6}.WithMagnitude(1)
		c.NormalizeProjectionVectors()
		c.Offset = geom.Vector{X: 3, Y: -4, Z: 5}
		c.Tangent = tangent
		m := projectionMatrix(&c)
		for _, p := range []geom.Vector{{}, {X: 10, Y: 20, Z: 30}, {X: -50, Y: 7, Z: -12}} {
			var clip [4]float64
			for row := range 4 {
				clip[row] = float64(m[row]*float32(p.X) + m[4+row]*float32(p.Y) + m[8+row]*float32(p.Z) + m[12+row])
			}
			want := c.ProjectPoint(p)
			gotX := clip[0] / clip[3] * c.Width / 2
			gotY := clip[1] / clip[3] * c.Height / 2
			if math.Abs(gotX-want.U) > 1e-3 || math.Abs(gotY-want.V) > 1e-3 {
				t.Errorf("tangent %v, %v: screen (%.4f, %.4f), camera says (%.4f, %.4f)", tangent, p, gotX, gotY, want.U, want.V)
			}
		}
	}
}

func TestSetCameraSizesTheFrame(t *testing.T) {
	t.Parallel()
	r, f := newTest()
	if f.w != 200 || f.h != 100 {
		t.Errorf("frame %dx%d, want 200x100", f.w, f.h)
	}
	c := testCamera()
	c.PixelRatio = 2
	r.SetCamera(c)
	if f.w != 400 || f.h != 200 {
		t.Errorf("frame %dx%d at pixel ratio 2, want 400x200", f.w, f.h)
	}
	if len(f.clears) != 2 || f.clears[0] || f.clears[1] {
		t.Errorf("SetCamera clears %v, want depth only twice", f.clears)
	}
}

func TestQuadIsTwoTriangles(t *testing.T) {
	t.Parallel()
	r, f := newTest()
	hcf := r.InternFill(render.Fill{Color: core.RGBAFromFloat(1, 0.5, 0, 1)})
	r.DrawQuad(geom.Vector{}, geom.Vector{X: 1}, geom.Vector{X: 1, Y: 1}, geom.Vector{Y: 1}, hcf)
	r.FinishFrame()
	if len(f.draws) != 1 || f.finishes != 1 {
		t.Fatalf("%d draws, %d finishes", len(f.draws), f.finishes)
	}
	d := &f.draws[0]
	if n := len(d.verts) / vertexFloats; n != 6 {
		t.Fatalf("%d vertices, want 6", n)
	}
	if v := vert(d, 4); v[0] != 1 || v[1] != 1 || v[6] != 1 || math.Abs(float64(v[7])-0.5) > 0.01 {
		t.Errorf("vertex 4 = %v", v)
	}
	if d.pipe.mode != modeFlat || d.pipe.pattern != render.FillSolid {
		t.Errorf("pipe = %+v", d.pipe)
	}
}

func TestRunsEndWhereTheStateChanges(t *testing.T) {
	t.Parallel()
	r, f := newTest()
	a := r.InternFill(render.Fill{Color: core.RGBAFromFloat(1, 0, 0, 1)})
	b := r.InternFill(render.Fill{Color: core.RGBAFromFloat(0, 1, 0, 1)}) // same state, other colour
	c := r.InternFill(render.Fill{Layer: render.LayerOccluded})
	d := r.InternFill(render.Fill{Pattern: render.FillCheckeredA})
	q := func(h render.HFill) {
		r.DrawQuad(geom.Vector{}, geom.Vector{X: 1}, geom.Vector{X: 1, Y: 1}, geom.Vector{Y: 1}, h)
	}
	q(a)
	q(b)
	q(c)
	q(d)
	r.FlushFrame()
	if len(f.draws) != 3 {
		t.Fatalf("%d draws, want 3: colours share a run, a layer and a pattern do not", len(f.draws))
	}
	if n := len(f.draws[0].verts) / vertexFloats; n != 12 {
		t.Errorf("first run %d vertices, want 12", n)
	}
	if p := f.draws[1].pipe; p.test != depthGreater || p.depthWrite {
		t.Errorf("occluded pipe = %+v", p)
	}
	if f.draws[2].pipe.pattern != render.FillCheckeredA {
		t.Errorf("third pipe = %+v", f.draws[2].pipe)
	}
	if f.flushes != 1 {
		t.Errorf("%d flushes", f.flushes)
	}
}

func TestDepthRanges(t *testing.T) {
	t.Parallel()
	cases := []struct {
		layer     render.Layer
		z         int
		test      depthTest
		dw, cw    bool
		near, far float32
	}{
		{render.LayerNormal, 0, depthLEqual, true, true, 0.1, 1},
		{render.LayerDepthOnly, 0, depthLEqual, true, false, 0.1, 1},
		{render.LayerOccluded, 0, depthGreater, false, true, 0.1, 1},
		{render.LayerFront, 3, depthLEqual, true, true, 0, 0},
		{render.LayerBack, 3, depthLEqual, true, true, 1, 1},
	}
	for _, c := range cases {
		p := pipe{}.withLayer(c.layer, c.z)
		if p.test != c.test || p.depthWrite != c.dw || p.colorWrite != c.cw || p.near != c.near || p.far != c.far {
			t.Errorf("layer %v: %+v", c.layer, p)
		}
	}
	lo, hi := pipe{}.withLayer(render.LayerNormal, 0), pipe{}.withLayer(render.LayerNormal, 5)
	if !(hi.near < lo.near && hi.far < lo.far) {
		t.Errorf("a higher z-index must be nearer: %v %v", lo, hi)
	}
}

func TestThinAndFatLines(t *testing.T) {
	t.Parallel()
	r, f := newTest()
	thin := r.InternStroke(render.Stroke{Width: 2, Unit: render.UnitPX})
	fat := r.InternStroke(render.Stroke{Width: 8, Unit: render.UnitPX})
	r.DrawLine(geom.Vector{}, geom.Vector{X: 10}, thin)
	r.DrawLine(geom.Vector{}, geom.Vector{X: 10}, fat)
	r.DrawLine(geom.Vector{X: 1}, geom.Vector{X: 1}, fat) // no length, no line
	r.FlushFrame()
	if len(f.draws) != 1 {
		t.Fatalf("%d draws", len(f.draws))
	}
	d := &f.draws[0]
	// A thin line is a quad; a fat one a quad and two caps of nine
	// triangles.
	if n := len(d.verts) / vertexFloats; n != 6+6+2*9*3 {
		t.Fatalf("%d vertices", n)
	}
	// The thin quad is 2 pixels = 1 mm across, at scale 2.
	if y0, y1 := vert(d, 0)[1], vert(d, 2)[1]; math.Abs(float64(y0-y1)) != 1 {
		t.Errorf("thin line %v mm across, want 1", y0-y1)
	}
}

func TestDashedLine(t *testing.T) {
	t.Parallel()
	r, f := newTest()
	h := r.InternStroke(render.Stroke{Width: 1, Unit: render.UnitPX, StipplePattern: core.StippleDash, StippleScale: 1, Color: core.RGBAFromFloat(1, 1, 1, 1)})
	r.DrawLine(geom.Vector{}, geom.Vector{X: 100}, h)
	r.FlushFrame()
	if len(f.draws) != 1 || len(f.draws[0].verts)/vertexFloats <= 6 {
		t.Errorf("a dashed line is several quads: %d draws", len(f.draws))
	}
}

func TestClipPlane(t *testing.T) {
	t.Parallel()
	r, f := newTest()
	h := r.InternFill(render.Fill{})
	q := func() { r.DrawQuad(geom.Vector{}, geom.Vector{X: 1}, geom.Vector{X: 1, Y: 1}, geom.Vector{Y: 1}, h) }
	q()
	r.SetClipPlane(&srf.HalfSpace{N: geom.Vector{X: 1}, D: 4})
	q()
	r.SetClipPlane(nil)
	q()
	r.FlushFrame()
	if len(f.draws) != 3 {
		t.Fatalf("%d draws, want 3", len(f.draws))
	}
	if f.draws[0].scene.clipOn || !f.draws[1].scene.clipOn || f.draws[2].scene.clipOn {
		t.Error("clip on only for the middle run")
	}
	// A point is kept where D - N.p >= 0: clip . (p, 1) >= 0.
	if c := f.draws[1].scene.clip; c != [4]float32{-1, 0, 0, 4} {
		t.Errorf("clip = %v", c)
	}
	r.SetClipPlane(&srf.HalfSpace{})
	r.StartFrame()
	if r.scene.clipOn {
		t.Error("StartFrame must lift the clip plane")
	}
}

func triMesh() *srf.SMesh {
	m := &srf.SMesh{}
	for face := range 2 {
		tr := srf.STriangleFrom(srf.STriMeta{Face: srf.FaceID(face + 1), RGBA: core.RGBAFromFloat(0, 0, 1, 1)},
			geom.Vector{}, geom.Vector{X: 1}, geom.Vector{Y: 1})
		m.Triangles = append(m.Triangles, tr)
	}
	m.Triangles[1].NA = geom.Vector{Z: 1}
	m.Triangles[1].NB = geom.Vector{Z: 1}
	m.Triangles[1].NC = geom.Vector{Z: 1}
	return m
}

func TestMeshLit(t *testing.T) {
	t.Parallel()
	r, f := newTest()
	front := r.InternFill(render.Fill{})
	back := r.InternFill(render.Fill{Color: core.RGBAFromFloat(1, 0.1, 0.1, 1)})
	r.DrawMesh(triMesh(), front, back)
	r.FlushFrame()
	if len(f.draws) != 1 {
		t.Fatalf("%d draws", len(f.draws))
	}
	d := &f.draws[0]
	if d.pipe.mode != modeLit || !d.pipe.twoSide || !d.pipe.useBack || d.pipe.backCol[0] != 1 {
		t.Errorf("pipe = %+v", d.pipe)
	}
	// The triangle with no vertex normals takes the unit face normal,
	// and each triangle its own colour.
	if v := vert(d, 0); v[5] != 1 || v[8] != 1 {
		t.Errorf("flat vertex = %v", v)
	}
}

func TestMeshShadedBatchesByMaterial(t *testing.T) {
	t.Parallel()
	r, f := newTest()
	r.SetMaterials(func(face uint32) render.Material {
		if face == 1 {
			return render.Materials["metal"]
		}
		return render.Materials["matte"]
	})
	r.SetLighting(render.Lighting{Shaded: true, AmbientIntensity: 0.3, LightIntensity: [2]float64{0.7, 0},
		LightDirection: [2]geom.Vector{{X: 0, Y: 0, Z: 1}}})
	r.StartFrame()
	h := r.InternFill(render.Fill{})
	r.DrawMesh(triMesh(), h, h)
	r.FlushFrame()
	if len(f.draws) != 2 {
		t.Fatalf("%d draws, want a run for each of two materials", len(f.draws))
	}
	if f.draws[0].pipe.mode != modeShaded || f.draws[0].pipe.mat != render.Materials["metal"] ||
		f.draws[1].pipe.mat != render.Materials["matte"] {
		t.Errorf("pipes %+v %+v", f.draws[0].pipe, f.draws[1].pipe)
	}
	if f.draws[0].pipe.useBack {
		t.Error("a back fill of no colour must not override the colour")
	}
	s := f.draws[0].scene
	if s.ambient != 0.3 || s.lightI[0] != 0.7 || s.lightDir[0] != [3]float32{0, 0, 1} || s.view != [3]float32{0, 0, 1} {
		t.Errorf("scene = %+v", s)
	}
}

func TestLightsKeepTheirLastState(t *testing.T) {
	t.Parallel()
	r, _ := newTest()
	r.SetLighting(render.Lighting{LightIntensity: [2]float64{0.5, 0.25}, LightDirection: [2]geom.Vector{{X: 1}, {Y: 1}}})
	r.StartFrame()
	r.SetLighting(render.Lighting{LightIntensity: [2]float64{0.5, 0}})
	r.StartFrame()
	if r.scene.lightI != [2]float32{0.5, 0.25} {
		t.Errorf("lights = %v: a light given intensity 0 keeps its state, as GL's does", r.scene.lightI)
	}
}

func TestPixmapQuad(t *testing.T) {
	t.Parallel()
	r, f := newTest()
	pm := res.NewPixmap(res.FormatRGBA, 4, 4)
	h := r.InternFill(render.Fill{})
	r.DrawPixmap(pm, geom.Vector{}, geom.Vector{X: 1}, geom.Vector{Y: 1}, geom.Point2d{U: 0, V: 0}, geom.Point2d{U: 1, V: 1}, h)
	r.DrawQuad(geom.Vector{}, geom.Vector{X: 1}, geom.Vector{X: 1, Y: 1}, geom.Vector{Y: 1}, h)
	r.InvalidatePixmap(pm)
	r.FlushFrame()
	if len(f.draws) != 2 || f.draws[0].pipe.tex != pm || f.draws[1].pipe.tex != nil {
		t.Fatalf("draws = %d", len(f.draws))
	}
	d := &f.draws[0]
	// Corner o+u+v has texture coordinate (1, 1); o+u has (1, 0).
	if v := vert(d, 2); v[10] != 1 || v[11] != 1 {
		t.Errorf("vertex 2 = %v", v)
	}
	if v := vert(d, 5); v[0] != 1 || v[1] != 0 || v[10] != 1 || v[11] != 0 {
		t.Errorf("vertex 5 = %v", v)
	}
	if len(f.invalid) != 1 || f.invalid[0] != pm {
		t.Error("InvalidatePixmap must reach the device")
	}
}

func TestDrawFacesAndPolygon(t *testing.T) {
	t.Parallel()
	r, f := newTest()
	h := r.InternFill(render.Fill{})
	r.DrawFaces(triMesh(), []uint32{2}, h)
	var p srf.SPolygon
	p.Normal = geom.Vector{Z: 1}
	p.AddEmptyContour()
	for _, v := range []geom.Vector{{}, {X: 2}, {X: 2, Y: 2}, {Y: 2}} {
		p.Contours[0].AddPoint(v)
	}
	r.DrawPolygon(&p, h)
	r.FlushFrame()
	if len(f.draws) != 1 {
		t.Fatalf("%d draws", len(f.draws))
	}
	if n := len(f.draws[0].verts) / vertexFloats; n != 3+6 {
		t.Errorf("%d vertices, want one triangle of face 2 and two of the square", n)
	}
}

func TestReadFrameFlipsRows(t *testing.T) {
	t.Parallel()
	r, f := newTest()
	f.frameW, f.frameH = 2, 2
	// Bottom row first: red, green, then blue, white on top.
	f.frame = []byte{255, 0, 0, 0, 0, 255, 0, 0, 0, 0, 255, 0, 255, 255, 255, 0}
	img := r.ReadFrame()
	if img.Pix[0] != 0 || img.Pix[2] != 255 || img.Pix[8] != 255 || img.Pix[9] != 0 {
		t.Errorf("pix = %v", img.Pix)
	}
	for i := 3; i < len(img.Pix); i += 4 {
		if img.Pix[i] != 255 {
			t.Fatal("the frame must be opaque")
		}
	}
}

func TestCloseAndIdent(t *testing.T) {
	t.Parallel()
	r, f := newTest()
	if v, rr, ver := r.Ident(); v != "v" || rr != "r" || ver != "ver" {
		t.Error("Ident must be the device's")
	}
	r.Resize(640, 480)
	if f.w != 640 || f.h != 480 {
		t.Errorf("Resize gave %dx%d", f.w, f.h)
	}
	r.Close()
	if !f.closed {
		t.Error("Close must reach the device")
	}
}
