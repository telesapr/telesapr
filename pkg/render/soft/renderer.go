// Package soft is a software renderer with the semantics of SolveSpace's
// OpenGL 1 renderer (render/rendergl1.cpp): a z-buffer with its depth
// ranges per layer and z-index, polygon stipples, antialiased thin
// lines and quad fat lines, fixed-function lighting with two
// directional lights, textured quads and alpha blending. It draws into
// an image.RGBA.
package soft

import (
	"image"
	"math"
	"runtime"
	"sync"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/render"
	"github.com/telesapr/telesapr/pkg/res"
	"github.com/telesapr/telesapr/pkg/srf"
)

// glLight is a light's state, which persists across frames as GL's
// does: StartFrame only changes a light whose intensity isn't zero.
type glLight struct {
	enabled   bool
	intensity float64
	dir       geom.Vector
}

// Renderer is a render.ViewportCanvas drawing in software.
type Renderer struct {
	render.CanvasBase
	camera   render.Camera
	lighting render.Lighting

	width, height int
	frame         *image.RGBA
	depth         []float32

	// The view transform (UpdateProjection): model space to clip space.
	projN          geom.Vector
	sx, sy, sz, sw float64

	cur state

	// Thin lines collected by batchLines.
	batching  bool
	lineBatch []lineSeg

	// Lighting state that outlives frames, as in GL.
	lights  [2]glLight
	ambient float64

	// materials gives a face's material for Shaded lighting.
	materials func(face uint32) render.Material

	// The user's clip plane in model space, and as a clip-space plane
	// for the current projection: userClip is its signed distance
	// function, nil with no plane, and userEq its coefficients of x,
	// y, z and w.
	clipPlane *srf.HalfSpace
	userEq    [4]float64
	userClip  func(v *cvert) float64
}

var _ render.ViewportCanvas = (*Renderer)(nil)

// New returns a renderer with GL's initial state.
func New() *Renderer {
	return &Renderer{ambient: 0.2}
}

// Camera returns the current camera.
func (r *Renderer) Camera() *render.Camera { return &r.camera }

// SetCamera sets the camera, resizes the frame to it and clears the
// depth buffer.
func (r *Renderer) SetCamera(c render.Camera) {
	r.camera = c
	r.updateProjection()
}

// SetMaterials sets what gives each face's material when the lighting
// is Shaded.
func (r *Renderer) SetMaterials(of func(face uint32) render.Material) { r.materials = of }

// SetLighting sets the lighting for the next frame.
func (r *Renderer) SetLighting(l render.Lighting) { r.lighting = l }

func (r *Renderer) updateProjection() {
	c := &r.camera
	pr := c.PixelRatio
	if pr == 0 {
		pr = 1
	}
	w := int(c.Width * pr)
	h := int(c.Height * pr)
	w, h = max(w, 1), max(h, 1)
	if r.frame == nil || r.width != w || r.height != h {
		r.width, r.height = w, h
		r.frame = image.NewRGBA(image.Rect(0, 0, w, h))
		r.depth = make([]float32, w*h)
	}

	// glScaled(scale*2/width, scale*2/height, scale/30000), then the
	// perspective (w' = tangent*scale*z + 1), the rotation into
	// (projRight, projUp, projUp x projRight) and the translation by
	// offset.
	r.projN = c.ProjUp.Cross(c.ProjRight)
	r.sx = c.Scale * 2.0 / c.Width
	r.sy = c.Scale * 2.0 / c.Height
	r.sz = c.Scale * 1.0 / 30000
	r.sw = c.Tangent * c.Scale
	r.updateUserClip()

	r.clearDepth()
}

// SetClipPlane makes what is drawn from now on keep only the half
// space h, or with nil, everything (GL's glClipPlane).
func (r *Renderer) SetClipPlane(h *srf.HalfSpace) {
	if h == nil {
		r.clipPlane = nil
	} else {
		cp := *h
		r.clipPlane = &cp
	}
	r.updateUserClip()
}

// updateUserClip expresses the clip plane in clip space. A model point
// p is kept when D - N.p >= 0. With q = p + offset rotated into the
// view's axes, x = sx*q.right, y = sy*q.up, z = sz*q.n and w = sw*q.n
// + 1, so that 1 = w - (sw/sz)*z, the distance is linear in x, y, z
// and w.
func (r *Renderer) updateUserClip() {
	h := r.clipPlane
	if h == nil || r.sx == 0 || r.sy == 0 || r.sz == 0 {
		r.userClip = nil
		return
	}
	c := &r.camera
	c0 := h.D + h.N.Dot(c.Offset)
	r.userEq = [4]float64{
		-h.N.Dot(c.ProjRight) / r.sx,
		-h.N.Dot(c.ProjUp) / r.sy,
		-h.N.Dot(r.projN)/r.sz - c0*r.sw/r.sz,
		c0,
	}
	eq := r.userEq
	r.userClip = func(v *cvert) float64 { return eq[0]*v.x + eq[1]*v.y + eq[2]*v.z + eq[3]*v.w }
}

// clip transforms a model-space point to clip space.
func (r *Renderer) clip(p geom.Vector) cvert {
	c := &r.camera
	q := p.Plus(c.Offset)
	x := q.Dot(c.ProjRight)
	y := q.Dot(c.ProjUp)
	z := q.Dot(r.projN)
	return cvert{x: x * r.sx, y: y * r.sy, z: z * r.sz, w: r.sw*z + 1}
}

// StartFrame sets up lighting and clears the frame to the background
// colour.
func (r *Renderer) StartFrame() {
	l := &r.lighting
	for i := range r.lights {
		if l.LightIntensity[i] != 0.0 {
			r.lights[i] = glLight{
				enabled:   true,
				intensity: float64(float32(l.LightIntensity[i])),
				dir:       r.camera.VectorFromProjs(l.LightDirection[i]),
			}
		}
	}
	if l.AmbientIntensity != 0.0 {
		r.ambient = float64(float32(l.AmbientIntensity))
	}

	r.SetClipPlane(nil)

	bg := l.BackgroundColor
	// The frame is read back as RGB, as the GL renderer does.
	fill(r.frame.Pix, []byte{bg.Red, bg.Green, bg.Blue, 255})
	r.clearDepth()
}

// fill repeats pattern over b, whose length is a multiple of it.
func fill(b, pattern []byte) {
	if len(b) == 0 {
		return
	}
	n := copy(b, pattern)
	for n < len(b) {
		n += copy(b[n:], b[:n])
	}
}

func (r *Renderer) clearDepth() {
	if len(r.depth) == 0 {
		return
	}
	r.depth[0] = 1
	for n := 1; n < len(r.depth); n *= 2 {
		copy(r.depth[n:], r.depth[:n])
	}
}

// FlushFrame does nothing; drawing is immediate.
func (r *Renderer) FlushFrame() {}

// FinishFrame does nothing; drawing is immediate.
func (r *Renderer) FinishFrame() {}

// ReadFrame is the frame drawn so far, top row first and opaque. The
// renderer keeps drawing into it.
func (r *Renderer) ReadFrame() *image.RGBA { return r.frame }

// Ident names the renderer.
func (r *Renderer) Ident() (vendor, renderer, version string) {
	return "TeleSAPR", "software (OpenGL 1 semantics)", "1"
}

// Clear forgets the strokes and fills.
func (r *Renderer) Clear() {
	r.CanvasBase.Clear()
	r.cur = state{}
}

func colorF(c core.RGBA) [4]float64 {
	return [4]float64{float64(c.RedF()), float64(c.GreenF()), float64(c.BlueF()), float64(c.AlphaF())}
}

// setDepthRange is ssglDepthRange.
func (r *Renderer) setDepthRange(layer render.Layer, zIndex int) {
	switch layer {
	case render.LayerNormal, render.LayerFront, render.LayerBack:
		r.cur.depthFunc, r.cur.depthWrite, r.cur.colorWrite = depthLEqual, true, true
	case render.LayerDepthOnly:
		r.cur.depthFunc, r.cur.depthWrite, r.cur.colorWrite = depthLEqual, true, false
	case render.LayerOccluded:
		r.cur.depthFunc, r.cur.depthWrite, r.cur.colorWrite = depthGreater, false, true
	}
	switch layer {
	case render.LayerFront:
		r.cur.depthNear, r.cur.depthFar = 0, 0
	case render.LayerBack:
		r.cur.depthNear, r.cur.depthFar = 1, 1
	default:
		// The size of this step depends on the resolution of the Z
		// buffer; for a 16-bit buffer, this should be fine.
		offset := 1.0 / (65535 * 0.8) * float64(zIndex)
		r.cur.depthNear = clamp01(0.1 - offset)
		r.cur.depthFar = clamp01(1.0 - offset)
	}
}

func (r *Renderer) setFillPattern(p render.FillPattern) {
	switch p {
	case render.FillCheckeredA:
		r.cur.stipple = &stippleA
	case render.FillCheckeredB:
		r.cur.stipple = &stippleB
	default:
		r.cur.stipple = nil
	}
}

// selectStroke is SelectStroke: the stroke's colour and depth range,
// no stipple, no texture.
func (r *Renderer) selectStroke(hcs render.HStroke) *render.Stroke {
	s := r.Stroke(hcs)
	r.cur.color = colorF(s.Color)
	r.setDepthRange(s.Layer, s.ZIndex)
	// Fat lines and points are quads affected by glPolygonStipple, so
	// make sure they are displayed correctly.
	r.setFillPattern(render.FillSolid)
	r.cur.texture = nil
	return s
}

// selectFill is SelectFill.
func (r *Renderer) selectFill(hcf render.HFill) *render.Fill {
	f := r.Fill(hcf)
	r.cur.color = colorF(f.Color)
	r.setDepthRange(f.Layer, f.ZIndex)
	r.setFillPattern(f.Pattern)
	r.cur.texture = nil
	return f
}

// drawPolygonClip draws a convex polygon given in clip space.
func (r *Renderer) drawPolygonClip(vs []cvert, mode fragMode) {
	vs = clipPolygon(vs, r.userClip)
	if len(vs) < 3 {
		return
	}
	w := make([]rvert, len(vs))
	for i := range vs {
		w[i] = r.toWindow(&vs[i])
	}
	for i := 1; i+1 < len(w); i++ {
		r.rasterTriangle(&w[0], &w[i], &w[i+1], mode)
	}
}

func (r *Renderer) quad(a, b, c, d geom.Vector) {
	r.drawPolygonClip([]cvert{r.clip(a), r.clip(b), r.clip(c), r.clip(d)}, fragSolid)
}

// line draws a GL_LINES segment of the current stroke's pixel width.
func (r *Renderer) line(a, b geom.Vector, widthPx float64) {
	ca, cb, ok := clipSegment(r.clip(a), r.clip(b), r.userClip)
	if !ok {
		return
	}
	wa, wb := r.toWindow(&ca), r.toWindow(&cb)
	if r.batching {
		r.lineBatch = append(r.lineBatch, lineSeg{wa, wb, widthPx})
		return
	}
	r.rasterLine(&wa, &wb, widthPx)
}

// lineSeg is a thin line in window coordinates, waiting to be drawn.
type lineSeg struct {
	a, b  rvert
	width float64
}

// batchLines runs draw, which draws with one stroke, collecting its thin
// lines, then rasterises them in order; many of them are drawn in
// horizontal bands at once, as rasterTriangles does. A fat line is a
// quad and caps, drawn at once, but a stroke's lines are all thin or
// all fat.
func (r *Renderer) batchLines(draw func()) {
	r.batching = true
	r.lineBatch = r.lineBatch[:0]
	draw()
	r.batching = false
	segs := r.lineBatch
	workers := min(runtime.GOMAXPROCS(0), r.height/16)
	if len(segs) < 256 || workers < 2 {
		for i := range segs {
			r.rasterLine(&segs[i].a, &segs[i].b, segs[i].width)
		}
		return
	}
	band := (r.height + workers - 1) / workers
	var wg sync.WaitGroup
	for y0 := 0; y0 < r.height; y0 += band {
		y1 := min(y0+band, r.height)
		wg.Go(func() {
			for i := range segs {
				r.rasterLineRows(&segs[i].a, &segs[i].b, segs[i].width, y0, y1)
			}
		})
	}
	wg.Wait()
}

// doFatLineEndcap is a semicircle fan.
func (r *Renderer) doFatLineEndcap(p, u, v geom.Vector) {
	vs := make([]cvert, 0, len(render.EndcapCircle))
	for _, pc := range render.EndcapCircle {
		vs = append(vs, r.clip(p.Plus(u.ScaledBy(pc[0])).Plus(v.ScaledBy(pc[1]))))
	}
	r.drawPolygonClip(vs, fragSolid)
}

// doFatLine draws a line wider than GL_LINES can as a quad with round
// caps, of constant thickness on screen.
func (r *Renderer) doFatLine(a, b geom.Vector, width float64) {
	hw := width / 2
	ab := b.Minus(a)
	gn := r.camera.ProjRight.Cross(r.camera.ProjUp)
	abn := ab.Cross(gn).WithMagnitude(1)
	abn = abn.Minus(gn.ScaledBy(gn.Dot(abn)))
	// So now abn is normal to the projection of ab into the screen, so
	// the line will always have constant thickness as the view is
	// rotated.
	abn = abn.WithMagnitude(hw)
	ab = gn.Cross(abn)
	ab = ab.WithMagnitude(hw)

	r.quad(a.Plus(abn), b.Plus(abn), b.Minus(abn), a.Minus(abn))
	r.doFatLineEndcap(a, ab, abn)
	r.doFatLineEndcap(b, ab.ScaledBy(-1), abn)
}

// doLine is DoLine with the stroke already selected.
func (r *Renderer) doLine(a, b geom.Vector, s *render.Stroke) {
	if a.Equals(b) {
		return
	}
	wpx := s.WidthPx(&r.camera)
	if wpx <= 3.0 {
		r.line(a, b, wpx)
	} else {
		r.doFatLine(a, b, wpx/r.camera.Scale)
	}
}

// doPoint is DoPoint: a short line, or a disc.
func (r *Renderer) doPoint(p geom.Vector, d, lineWidthPx float64) {
	if d <= 3.0 {
		u := r.camera.ProjRight.WithMagnitude(d / 2.0 / r.camera.Scale)
		r.line(p.Minus(u), p.Plus(u), lineWidthPx)
	} else {
		u := r.camera.ProjRight.WithMagnitude(d / 2.0 / r.camera.Scale)
		v := r.camera.ProjUp.WithMagnitude(d / 2.0 / r.camera.Scale)
		r.doFatLineEndcap(p, u, v)
		r.doFatLineEndcap(p, u.ScaledBy(-1.0), v)
	}
}

// doStippledLine is DoStippledLine: the stroke's dash pattern, starting
// phase into it.
func (r *Renderer) doStippledLine(a, b geom.Vector, hcs render.HStroke, phase float64) {
	s := r.selectStroke(hcs)
	if s.StipplePattern == core.StippleContinuous {
		r.doLine(a, b, s)
		return
	}

	scale := s.StippleScaleMm(&r.camera)
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
	width := s.WidthMm(&r.camera)
	wpx := s.WidthPx(&r.camera)

	curDashLen := (curPhase - phase) / scale
	for cur < length2 {
		next := math.Min(length2, cur+curDashLen*scale)
		nextPos := curPos.Plus(dir.ScaledBy(next - cur))
		if curDash%2 == 0 {
			if curDashLen <= geom.LengthEps {
				r.doPoint(curPos, width, wpx)
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
	r.batchLines(func() {
		phase := 0.0
		for _, e := range el.Edges {
			r.doStippledLine(e.A, e.B, hcs, phase)
			phase += e.A.Minus(e.B).Magnitude()
		}
	})
}

// DrawBeziers can't draw curves; the caller draws them as edges.
func (r *Renderer) DrawBeziers(*srf.SBezierList, render.HStroke) bool { return false }

// DrawOutlines draws the outlines drawAs picks.
func (r *Renderer) DrawOutlines(ol *srf.SOutlineList, hcs render.HStroke, drawAs render.DrawOutlinesAs) {
	r.batchLines(func() { r.drawOutlines(ol, hcs, drawAs) })
}

func (r *Renderer) drawOutlines(ol *srf.SOutlineList, hcs render.HStroke, drawAs render.DrawOutlinesAs) {
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
	r.quad(a, b, c, d)
}

// DrawPoint draws a point as a square of the stroke's width.
func (r *Renderer) DrawPoint(o geom.Vector, hcs render.HStroke) {
	s := r.selectStroke(hcs)
	hcf := r.InternFill(render.Fill{Layer: s.Layer, ZIndex: s.ZIndex, Color: s.Color})
	rr := r.camera.ProjRight.ScaledBy(s.Width / 2.0 / r.camera.Scale)
	u := r.camera.ProjUp.ScaledBy(s.Width / 2.0 / r.camera.Scale)
	a := o.Plus(rr).Plus(u)
	b := o.Plus(rr).Minus(u)
	c := o.Minus(rr).Minus(u)
	d := o.Minus(rr).Plus(u)
	r.DrawQuad(a, b, c, d, hcf)
}

// DrawPolygon fills a planar polygon with the odd winding rule, as
// GLU's tessellator does for the GL renderer.
func (r *Renderer) DrawPolygon(p *srf.SPolygon, hcf render.HFill) {
	r.selectFill(hcf)
	r.fillPlanarPolygon(p)
}

// DrawMesh draws a lit mesh: with a back fill, its back faces in that
// colour (two-sided lighting); an empty fill colour means each
// triangle's own.
func (r *Renderer) DrawMesh(m *srf.SMesh, hcfFront, hcfBack render.HFill) {
	front := r.selectFill(hcfFront)
	frontColor := front.Color
	var backColor core.RGBA
	twoSide := hcfBack != 0
	if twoSide {
		back := r.Fill(hcfBack)
		backColor = back.Color
		if front.Layer != back.Layer || front.ZIndex != back.ZIndex {
			panic("soft: frontFill and backFill should belong to the same depth range")
		}
		if front.Pattern != back.Pattern {
			panic("soft: frontFill and backFill should have the same pattern")
		}
	}

	matFront, matBack := colorF(frontColor), colorF(backColor)
	var tris []rvert
	for i := range m.Triangles {
		tr := &m.Triangles[i]
		if frontColor.IsEmpty() {
			matFront = colorF(tr.Meta.RGBA)
		}
		if backColor.IsEmpty() {
			matBack = colorF(tr.Meta.RGBA)
		}

		var na, nb, nc geom.Vector
		if tr.NA.EqualsExactly(geom.Vector{}) {
			n := tr.Normal()
			na, nb, nc = n, n, n
		} else {
			na, nb, nc = tr.NA, tr.NB, tr.NC
		}

		vs := []cvert{r.clip(tr.A), r.clip(tr.B), r.clip(tr.C)}
		frontFacing := r.isFrontFacing(vs)
		mat, flip := matFront, false
		if twoSide && !frontFacing {
			mat, flip = matBack, true
		}
		if r.lighting.Shaded {
			m := render.Materials["plastic"]
			if r.materials != nil {
				m = r.materials(uint32(tr.Meta.Face))
			}
			vs[0].col = r.shade(na, mat, flip, m)
			vs[1].col = r.shade(nb, mat, flip, m)
			vs[2].col = r.shade(nc, mat, flip, m)
		} else {
			vs[0].col = r.light(na, mat, flip)
			vs[1].col = r.light(nb, mat, flip)
			vs[2].col = r.light(nc, mat, flip)
		}
		tris = r.appendPolygonTriangles(tris, vs)
	}
	r.rasterTriangles(tris, fragColor)
}

// appendPolygonTriangles clips a convex polygon given in clip space and
// appends its fan of triangles in window space.
func (r *Renderer) appendPolygonTriangles(tris []rvert, vs []cvert) []rvert {
	vs = clipPolygon(vs, r.userClip)
	if len(vs) < 3 {
		return tris
	}
	w0 := r.toWindow(&vs[0])
	prev := r.toWindow(&vs[1])
	for i := 2; i < len(vs); i++ {
		next := r.toWindow(&vs[i])
		tris = append(tris, w0, prev, next)
		prev = next
	}
	return tris
}

// rasterTriangles draws triangles in order. A big batch is drawn in
// horizontal bands at once: each pixel still meets its triangles in
// order, so the frame is the same.
func (r *Renderer) rasterTriangles(tris []rvert, mode fragMode) {
	n := len(tris) / 3
	workers := min(runtime.GOMAXPROCS(0), r.height/16)
	if n < 64 || workers < 2 {
		for i := range n {
			r.rasterTriangle(&tris[3*i], &tris[3*i+1], &tris[3*i+2], mode)
		}
		return
	}
	band := (r.height + workers - 1) / workers
	var wg sync.WaitGroup
	for y0 := 0; y0 < r.height; y0 += band {
		y1 := min(y0+band, r.height)
		wg.Go(func() {
			for i := range n {
				r.rasterTriangleRows(&tris[3*i], &tris[3*i+1], &tris[3*i+2], mode, y0, y1)
			}
		})
	}
	wg.Wait()
}

// isFrontFacing reports whether the triangle is counterclockwise in GL
// window coordinates (GL_CCW front faces).
func (r *Renderer) isFrontFacing(vs []cvert) bool {
	var x, y [3]float64
	for i := range vs {
		x[i] = vs[i].x / vs[i].w
		y[i] = vs[i].y / vs[i].w
	}
	area := (x[1]-x[0])*(y[2]-y[0]) - (x[2]-x[0])*(y[1]-y[0])
	sign := 1.0
	for i := range vs {
		if vs[i].w < 0 {
			sign = -sign
		}
	}
	return area*sign > 0
}

// light is GL's fixed-function lighting for a vertex: the model
// ambient plus each enabled directional light's diffuse term; the
// material's specular is GL's default of zero. With flip, the back
// face is lit, with the normal reversed.
func (r *Renderer) light(n geom.Vector, mat [4]float64, flip bool) [4]float64 {
	n = n.WithMagnitude(1)
	if flip {
		n = n.ScaledBy(-1)
	}
	var c [4]float64
	for k := range 3 {
		c[k] = r.ambient * mat[k]
	}
	for _, l := range r.lights {
		if !l.enabled {
			continue
		}
		d := n.Dot(l.dir.WithMagnitude(1))
		if d <= 0 {
			continue
		}
		for k := range 3 {
			c[k] += d * l.intensity * mat[k]
		}
	}
	for k := range 3 {
		c[k] = clamp01(c[k])
	}
	c[3] = mat[3]
	return c
}

// shade is a vertex's Shaded lighting (render.Shade), with the back
// face lit with the normal reversed when flip is set.
func (r *Renderer) shade(n geom.Vector, mat [4]float64, flip bool, m render.Material) [4]float64 {
	if flip {
		n = n.ScaledBy(-1)
	}
	var lights []geom.Vector
	var intensity []float64
	for _, l := range r.lights {
		if l.enabled {
			lights = append(lights, l.dir.WithMagnitude(1))
			intensity = append(intensity, l.intensity)
		}
	}
	view := r.camera.ProjRight.Cross(r.camera.ProjUp).WithMagnitude(1)
	c := render.Shade(n, [3]float64{mat[0], mat[1], mat[2]}, r.ambient, lights, intensity, view,
		r.camera.ProjUp.WithMagnitude(1), m)
	return [4]float64{c[0], c[1], c[2], mat[3]}
}

// DrawFaces fills the triangles of the given faces, unlit.
func (r *Renderer) DrawFaces(m *srf.SMesh, faces []uint32, hcf render.HFill) {
	r.selectFill(hcf)
	var tris []rvert
	for i := range m.Triangles {
		tr := &m.Triangles[i]
		face := uint32(tr.Meta.Face)
		for _, f := range faces {
			if f != face {
				continue
			}
			tris = r.appendPolygonTriangles(tris, []cvert{r.clip(tr.A), r.clip(tr.B), r.clip(tr.C)})
			break
		}
	}
	r.rasterTriangles(tris, fragSolid)
}

// DrawPixmap draws a textured quad o, o+u, o+u+v, o+v with texture
// coordinates from ta to tb.
func (r *Renderer) DrawPixmap(pm *res.Pixmap, o, u, v geom.Vector, ta, tb geom.Point2d, hcf render.HFill) {
	r.selectFill(hcf)
	r.cur.texture = pm
	a, b, c, d := r.clip(o), r.clip(o.Plus(v)), r.clip(o.Plus(u).Plus(v)), r.clip(o.Plus(u))
	a.s, a.t = ta.U, ta.V
	b.s, b.t = ta.U, tb.V
	c.s, c.t = tb.U, tb.V
	d.s, d.t = tb.U, ta.V
	r.drawPolygonClip([]cvert{a, b, c, d}, fragTexture)
	r.cur.texture = nil
}

// InvalidatePixmap does nothing: textures are sampled from the pixmap
// as it is when drawn.
func (r *Renderer) InvalidatePixmap(*res.Pixmap) {}

// CreateBatch returns nil, as the GL 1 renderer does.
func (r *Renderer) CreateBatch() render.BatchCanvas { return nil }
