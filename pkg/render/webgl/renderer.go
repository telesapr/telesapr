package webgl

import (
	"image"
	"math"
	"slices"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/render"
	"github.com/telesapr/telesapr/pkg/res"
	"github.com/telesapr/telesapr/pkg/srf"
)

// Renderer is a render.ViewportCanvas drawing with WebGL 2. It builds
// a stream of triangles in the state of the strokes and fills it is
// asked to draw with, and hands each run of the same state to its
// device in one call.
type Renderer struct {
	render.CanvasBase
	dev      device
	camera   render.Camera
	lighting render.Lighting

	// materials gives a face's material for Shaded lighting.
	materials func(face uint32) render.Material

	// The run being collected, the state it is drawn in and the scene
	// around it.
	batch batch
	pipe  pipe
	scene scene

	// lightOn is which lights have been set: as GL's, a light keeps
	// what it was last given when a frame gives it intensity 0.
	lightOn [2]bool
	// width and height are the frame's size in device pixels.
	width, height int
}

var (
	_ render.ViewportCanvas = (*Renderer)(nil)
	_ render.MaterialCanvas = (*Renderer)(nil)
)

// newRenderer is a renderer drawing with dev, in GL's initial state.
func newRenderer(dev device) *Renderer {
	r := &Renderer{dev: dev}
	r.scene.ambient = 0.2
	r.scene.view = [3]float32{0, 0, 1}
	r.scene.up = [3]float32{0, 1, 0}
	return r
}

// Camera returns the current camera.
func (r *Renderer) Camera() *render.Camera { return &r.camera }

// pixelSize is the camera's size in device pixels.
func (r *Renderer) pixelSize() (int, int) {
	pr := r.camera.PixelRatio
	if pr == 0 {
		pr = 1
	}
	return max(int(r.camera.Width*pr), 1), max(int(r.camera.Height*pr), 1)
}

// SetCamera sets the camera and the projection, resizes the frame to
// it, and clears the depth buffer.
func (r *Renderer) SetCamera(c render.Camera) {
	r.flushRun()
	r.camera = c
	r.width, r.height = r.pixelSize()
	r.dev.resize(r.width, r.height)

	r.scene.mvp = projectionMatrix(&c)
	view := c.ProjRight.Cross(c.ProjUp).WithMagnitude(1)
	up := c.ProjUp.WithMagnitude(1)
	r.scene.view = vec3(view)
	r.scene.up = vec3(up)
	r.dev.clear([4]float32{}, false)
}

// Resize sets the size of the frame in device pixels. The next
// SetCamera sets it from the camera again.
func (r *Renderer) Resize(width, height int) {
	r.flushRun()
	r.width, r.height = max(width, 1), max(height, 1)
	r.dev.resize(r.width, r.height)
}

// SetLighting sets the lighting for the next frame.
func (r *Renderer) SetLighting(l render.Lighting) { r.lighting = l }

// SetMaterials sets what gives each face's material when the lighting
// is Shaded.
func (r *Renderer) SetMaterials(of func(face uint32) render.Material) { r.materials = of }

// SetClipPlane makes what is drawn from now on keep only the half
// space h, or with nil, everything. A model point p is kept when
// h.D - h.N.p >= 0.
func (r *Renderer) SetClipPlane(h *srf.HalfSpace) {
	r.flushRun()
	if h == nil {
		r.scene.clipOn = false
		return
	}
	r.scene.clipOn = true
	r.scene.clip = [4]float32{float32(-h.N.X), float32(-h.N.Y), float32(-h.N.Z), float32(h.D)}
}

// StartFrame sets up the lights and clears the frame.
func (r *Renderer) StartFrame() {
	r.flushRun()
	r.SetClipPlane(nil)
	r.pipe = pipe{}

	l := &r.lighting
	for i := range r.scene.lightI {
		if l.LightIntensity[i] == 0.0 {
			continue
		}
		d := r.camera.VectorFromProjs(l.LightDirection[i]).WithMagnitude(1)
		r.scene.lightDir[i] = vec3(d)
		r.scene.lightI[i] = float32(l.LightIntensity[i])
		r.lightOn[i] = true
	}
	if l.AmbientIntensity != 0.0 {
		r.scene.ambient = float32(l.AmbientIntensity)
	}

	bg := l.BackgroundColor
	r.dev.clear([4]float32{bg.RedF(), bg.GreenF(), bg.BlueF(), bg.AlphaF()}, true)
}

// FlushFrame sends what is collected.
func (r *Renderer) FlushFrame() {
	r.flushRun()
	r.dev.flush()
}

// FinishFrame sends what is collected and waits for the frame to be
// drawn; it is then in the canvas.
func (r *Renderer) FinishFrame() {
	r.flushRun()
	r.dev.finish()
}

// ReadFrame reads the frame drawn so far back, top row first and
// opaque.
func (r *Renderer) ReadFrame() *image.RGBA {
	r.flushRun()
	buf, w, h := r.dev.read()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	row := 4 * w
	for y := range h {
		copy(img.Pix[y*img.Stride:y*img.Stride+row], buf[(h-1-y)*row:(h-y)*row])
	}
	for i := 3; i < len(img.Pix); i += 4 {
		img.Pix[i] = 255
	}
	return img
}

// Ident is the browser's name for the GPU and the WebGL version.
func (r *Renderer) Ident() (vendor, renderer, version string) { return r.dev.ident() }

// Clear forgets the strokes and fills.
func (r *Renderer) Clear() {
	r.flushRun()
	r.CanvasBase.Clear()
	r.pipe = pipe{}
}

// CreateBatch returns nil: there is no batching.
func (r *Renderer) CreateBatch() render.BatchCanvas { return nil }

// Close frees what the renderer holds on the GPU. It draws no more.
func (r *Renderer) Close() {
	r.flushRun()
	r.dev.close()
}

// flushRun draws the run collected and starts another.
func (r *Renderer) flushRun() {
	if r.batch.vertices() == 0 {
		return
	}
	r.dev.draw(&drawCall{verts: r.batch.v, pipe: r.pipe, scene: r.scene})
	r.batch.reset()
}

// setPipe makes p the state of the vertices to come, ending the run if
// it is another.
func (r *Renderer) setPipe(p pipe) {
	if p != r.pipe {
		r.flushRun()
		r.pipe = p
	}
}

func colorF32(c core.RGBA) [4]float32 {
	return [4]float32{c.RedF(), c.GreenF(), c.BlueF(), c.AlphaF()}
}

func vec3(v geom.Vector) [3]float32 { return [3]float32{float32(v.X), float32(v.Y), float32(v.Z)} }

// selectStroke makes the stroke's colour and depth range current, with
// no pattern and no texture. Fat lines and points are quads, which a
// checkered fill would pattern, so a stroke always draws solid.
func (r *Renderer) selectStroke(hcs render.HStroke) *render.Stroke {
	s := r.Stroke(hcs)
	r.batch.color = colorF32(s.Color)
	r.setPipe(pipe{}.withLayer(s.Layer, s.ZIndex))
	return s
}

// selectFill makes the fill's colour, depth range and pattern current.
func (r *Renderer) selectFill(hcf render.HFill) {
	f := r.Fill(hcf)
	r.batch.color = colorF32(f.Color)
	p := pipe{}.withLayer(f.Layer, f.ZIndex)
	p.pattern = f.Pattern
	r.setPipe(p)
}

// lineBasis is a line's half width across the screen (abn) and along it
// (ab), each of length width/2, in the plane of the screen so that the
// line keeps its thickness as the view is turned. A line along the
// view direction has no width to draw.
func (r *Renderer) lineBasis(a, b geom.Vector, width float64) (ab, abn geom.Vector) {
	hw := width / 2
	gn := r.camera.ProjRight.Cross(r.camera.ProjUp)
	abn = b.Minus(a).Cross(gn).WithMagnitude(1)
	abn = abn.Minus(gn.ScaledBy(gn.Dot(abn))).WithMagnitude(hw)
	ab = gn.Cross(abn).WithMagnitude(hw)
	return ab, abn
}

// lineQuad adds a line of width mm as a quad.
func (r *Renderer) lineQuad(a, b geom.Vector, width float64) {
	_, abn := r.lineBasis(a, b, width)
	r.batch.quad(a.Plus(abn), b.Plus(abn), b.Minus(abn), a.Minus(abn))
}

// endcap adds the half circle at p spanned by u and v that rounds a fat
// line's end.
func (r *Renderer) endcap(p, u, v geom.Vector) {
	var pts [len(render.EndcapCircle)]geom.Vector
	for i, pc := range render.EndcapCircle {
		pts[i] = p.Plus(u.ScaledBy(pc[0])).Plus(v.ScaledBy(pc[1]))
	}
	r.batch.fan(pts[:])
}

// fatLine adds a line wider than a few pixels as a quad with round
// caps.
func (r *Renderer) fatLine(a, b geom.Vector, width float64) {
	ab, abn := r.lineBasis(a, b, width)
	r.batch.quad(a.Plus(abn), b.Plus(abn), b.Minus(abn), a.Minus(abn))
	r.endcap(a, ab, abn)
	r.endcap(b, ab.ScaledBy(-1), abn)
}

// doLine draws a line with the stroke selected. WebGL has no lines
// of a width, so a thin one is a quad at least a pixel wide.
func (r *Renderer) doLine(a, b geom.Vector, s *render.Stroke) {
	if a.Equals(b) {
		return
	}
	wpx := s.WidthPx(&r.camera)
	if wpx <= 3.0 {
		r.lineQuad(a, b, max(wpx, 1)/r.camera.Scale)
	} else {
		r.fatLine(a, b, wpx/r.camera.Scale)
	}
}

// doPoint draws a dot d pixels across: a short line, or a disc.
func (r *Renderer) doPoint(p geom.Vector, d float64) {
	u := r.camera.ProjRight.WithMagnitude(d / 2.0 / r.camera.Scale)
	if d <= 3.0 {
		r.lineQuad(p.Minus(u), p.Plus(u), max(d, 1)/r.camera.Scale)
		return
	}
	v := r.camera.ProjUp.WithMagnitude(d / 2.0 / r.camera.Scale)
	r.endcap(p, u, v)
	r.endcap(p, u.ScaledBy(-1.0), v)
}

// doStippledLine draws a segment in the stroke's dash pattern, starting
// phase into it.
func (r *Renderer) doStippledLine(a, b geom.Vector, hcs render.HStroke, phase float64) {
	s := r.selectStroke(hcs)
	if pts := s.Wave(&r.camera, a, b, phase); pts != nil {
		for i := 1; i < len(pts); i++ {
			r.doLine(pts[i-1], pts[i], s)
		}
		return
	}
	scale := s.DashScaleMm(&r.camera)
	if scale == 0 {
		r.doLine(a, b, s)
		return
	}

	dashes := core.StipplePatternDashes(s.StipplePattern)
	length := core.StipplePatternLength(s.StipplePattern) * scale

	phase -= math.Floor(phase/length) * length

	curPhase := 0.0
	curDash := 0
	for curDash = 0; curDash < len(dashes); curDash++ { //nolint:intrange // curDash is read after the loop
		curPhase += dashes[curDash] * scale
		if phase < curPhase {
			break
		}
	}

	dir := b.Minus(a)
	length2 := dir.Magnitude()
	dir = dir.WithMagnitude(1.0)

	cur := 0.0
	curPos := a
	// A dot is as wide as the line, and doPoint takes it in pixels.
	wpx := s.WidthPx(&r.camera)

	curDashLen := (curPhase - phase) / scale
	for cur < length2 {
		next := math.Min(length2, cur+curDashLen*scale)
		nextPos := curPos.Plus(dir.ScaledBy(next - cur))
		if curDash%2 == 0 {
			if curDashLen <= geom.LengthEps {
				r.doPoint(curPos, wpx)
			} else {
				r.doLine(curPos, nextPos, s)
			}
		}
		cur = next
		curPos = nextPos
		curDash++
		curDashLen = dashes[curDash%len(dashes)]
	}
}

// DrawLine draws a stroked segment.
func (r *Renderer) DrawLine(a, b geom.Vector, hcs render.HStroke) {
	r.doStippledLine(a, b, hcs, 0)
}

// DrawEdges draws each edge, carrying the dash phase along.
func (r *Renderer) DrawEdges(el *srf.SEdgeList, hcs render.HStroke) {
	phase := 0.0
	for _, e := range el.Edges {
		r.doStippledLine(e.A, e.B, hcs, phase)
		phase += e.A.Minus(e.B).Magnitude()
	}
}

// DrawBeziers can't draw curves; the caller draws them as edges.
func (r *Renderer) DrawBeziers(*srf.SBezierList, render.HStroke) bool { return false }

// DrawOutlines draws the outlines drawAs picks.
func (r *Renderer) DrawOutlines(ol *srf.SOutlineList, hcs render.HStroke, drawAs render.DrawOutlinesAs) {
	projDir := r.camera.ProjRight.Cross(r.camera.ProjUp)
	phase := 0.0
	for i := range ol.Outlines {
		o := &ol.Outlines[i]
		var draw bool
		switch drawAs {
		case render.EmphasizedAndContour:
			draw = o.IsVisible(projDir) || o.Tag != 0
		case render.EmphasizedWithoutContour:
			draw = !o.IsVisible(projDir) && o.Tag != 0
		case render.ContourOnly:
			draw = o.IsVisible(projDir)
		}
		if draw {
			r.doStippledLine(o.A, o.B, hcs, phase)
		}
		phase += o.A.Minus(o.B).Magnitude()
	}
}

// DrawVectorText draws text in the builtin vector font, grid-fitted.
func (r *Renderer) DrawVectorText(text string, height float64, o, u, v geom.Vector, hcs render.HStroke) {
	res.BuiltinVectorFont().TraceAligned(height, o, u, v, text, func(a, b geom.Vector) {
		r.doStippledLine(a, b, hcs, 0)
	}, &r.camera)
}

// DrawQuad fills a quad.
func (r *Renderer) DrawQuad(a, b, c, d geom.Vector, hcf render.HFill) {
	r.selectFill(hcf)
	r.batch.quad(a, b, c, d)
}

// DrawPoint draws a point as a square of the stroke's width.
func (r *Renderer) DrawPoint(o geom.Vector, hcs render.HStroke) {
	s := r.Stroke(hcs)
	hcf := r.InternFill(render.Fill{Layer: s.Layer, ZIndex: s.ZIndex, Color: s.Color})
	rr := r.camera.ProjRight.ScaledBy(s.Width / 2.0 / r.camera.Scale)
	u := r.camera.ProjUp.ScaledBy(s.Width / 2.0 / r.camera.Scale)
	r.DrawQuad(o.Plus(rr).Plus(u), o.Plus(rr).Minus(u), o.Minus(rr).Minus(u), o.Minus(rr).Plus(u), hcf)
}

// DrawPolygon fills a planar polygon with the odd winding rule, as the
// other renderers do, through the Go triangulator.
func (r *Renderer) DrawPolygon(p *srf.SPolygon, hcf render.HFill) {
	r.selectFill(hcf)
	m := triangulate(p)
	r.batch.setNormal(p.Normal)
	for i := range m.Triangles {
		tr := &m.Triangles[i]
		r.batch.tri(tr.A, tr.B, tr.C)
	}
}

// triangulate triangulates p, which may hold open contours.
func triangulate(p *srf.SPolygon) *srf.SMesh {
	m := &srf.SMesh{}
	var cp srf.SPolygon
	cp.Normal = p.Normal
	for i := range p.Contours {
		sc := &p.Contours[i]
		var c srf.SContour
		c.Points = append(c.Points, sc.Points...)
		// The triangulator wants closed contours.
		if n := len(c.Points); n > 0 && !c.Points[0].P.Equals(c.Points[n-1].P) {
			c.Points = append(c.Points, c.Points[0])
		}
		if len(c.Points) >= 4 {
			cp.Contours = append(cp.Contours, c)
		}
	}
	if len(cp.Contours) > 0 {
		cp.TriangulateInto(m)
	}
	return m
}

// DrawFaces fills the triangles of the given faces, unlit.
func (r *Renderer) DrawFaces(m *srf.SMesh, faces []uint32, hcf render.HFill) {
	r.selectFill(hcf)
	for i := range m.Triangles {
		tr := &m.Triangles[i]
		if slices.Contains(faces, uint32(tr.Meta.Face)) {
			r.batch.tri(tr.A, tr.B, tr.C)
		}
	}
}

// DrawPixmap draws a textured quad o, o+u, o+u+v, o+v with texture
// coordinates from ta to tb.
func (r *Renderer) DrawPixmap(pm *res.Pixmap, o, u, v geom.Vector, ta, tb geom.Point2d, hcf render.HFill) {
	r.selectFill(hcf)
	p := r.pipe
	p.tex = pm
	r.setPipe(p)

	// A texture coordinate goes with each corner, so the quad's two
	// triangles are added corner by corner.
	p0, p1, p2, p3 := o, o.Plus(v), o.Plus(u).Plus(v), o.Plus(u)
	corner := func(pt geom.Vector, s, t float64) {
		r.batch.setTex(s, t)
		r.batch.vertex(pt)
	}
	corner(p0, ta.U, ta.V)
	corner(p1, ta.U, tb.V)
	corner(p2, tb.U, tb.V)
	corner(p0, ta.U, ta.V)
	corner(p2, tb.U, tb.V)
	corner(p3, tb.U, ta.V)
	r.batch.setTex(0, 0)
}

// InvalidatePixmap makes the next draw of pm upload it again.
func (r *Renderer) InvalidatePixmap(pm *res.Pixmap) {
	r.flushRun()
	r.dev.invalidate(pm)
}
