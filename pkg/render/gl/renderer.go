//go:build cgo && !nogl

package gl

import (
	"fmt"
	"image"
	"math"
	"sync"
	"unsafe"

	glapi "github.com/go-gl/gl/v2.1/gl"

	"github.com/spacesolve/spacesolve/pkg/core"
	"github.com/spacesolve/spacesolve/pkg/geom"
	"github.com/spacesolve/spacesolve/pkg/render"
	"github.com/spacesolve/spacesolve/pkg/res"
	"github.com/spacesolve/spacesolve/pkg/srf"
)

// Renderer is a render.ViewportCanvas drawing with OpenGL 1 (C++
// OpenGl1Renderer).
type Renderer struct {
	render.CanvasBase
	ctx      Context
	camera   render.Camera
	lighting render.Lighting

	// Cached GL state, to group consecutive draw calls.
	cur struct {
		drawing bool
		mode    uint32
		hcs     render.HStroke
		stroke  *render.Stroke
		hcf     render.HFill
		fill    *render.Fill
		texture *res.Pixmap
	}
}

var _ render.ViewportCanvas = (*Renderer)(nil)

var (
	initOnce sync.Once
	errInit  error
)

// New returns a renderer drawing with ctx, which it makes current to
// load GL's entry points the first time.
func New(ctx Context) (render.ViewportCanvas, error) {
	if err := ctx.MakeCurrent(); err != nil {
		return nil, err
	}
	initOnce.Do(func() { errInit = glapi.Init() })
	if errInit != nil {
		return nil, fmt.Errorf("gl: %w", errInit)
	}
	return &Renderer{ctx: ctx}, nil
}

func (r *Renderer) makeCurrent() {
	// A failure leaves no context current, and GL ignores the calls.
	_ = r.ctx.MakeCurrent()
}

// GetCamera returns the current camera.
func (r *Renderer) GetCamera() *render.Camera { return &r.camera }

// SetCamera sets the camera and the projection, and clears the depth
// buffer (C++ SetCamera).
func (r *Renderer) SetCamera(c render.Camera) {
	r.camera = c
	r.makeCurrent()
	r.updateProjection()
}

// SetLighting sets the lighting for the next frame.
func (r *Renderer) SetLighting(l render.Lighting) { r.lighting = l }

func (r *Renderer) pixelSize() (int, int) {
	pr := r.camera.PixelRatio
	if pr == 0 {
		pr = 1
	}
	return max(int(r.camera.Width*pr), 1), max(int(r.camera.Height*pr), 1)
}

// updateProjection is UpdateProjection: the viewport, then scale,
// perspective, rotation and translation in one matrix.
func (r *Renderer) updateProjection() {
	r.unSelectPrimitive()
	w, h := r.pixelSize()
	glapi.Viewport(0, 0, int32(w), int32(h))

	c := &r.camera
	sx := c.Scale * 2.0 / c.Width
	sy := c.Scale * 2.0 / c.Height
	sz := c.Scale * 1.0 / 30000
	clp := c.Tangent * c.Scale
	right, up := c.ProjRight, c.ProjUp
	n := up.Cross(right)
	off := c.Offset
	rows := [4][4]float64{
		{sx * right.X, sx * right.Y, sx * right.Z, sx * right.Dot(off)},
		{sy * up.X, sy * up.Y, sy * up.Z, sy * up.Dot(off)},
		{sz * n.X, sz * n.Y, sz * n.Z, sz * n.Dot(off)},
		{clp * n.X, clp * n.Y, clp * n.Z, clp*n.Dot(off) + 1},
	}
	var m [16]float64
	for row := range 4 {
		for col := range 4 {
			m[col*4+row] = rows[row][col]
		}
	}
	glapi.MatrixMode(glapi.PROJECTION)
	glapi.LoadMatrixd(&m[0])
	glapi.MatrixMode(glapi.MODELVIEW)
	glapi.LoadIdentity()

	glapi.ClearDepth(1.0)
	glapi.Clear(glapi.DEPTH_BUFFER_BIT)
}

// StartFrame sets up the state, the lights, and clears the frame
// (C++ StartFrame).
func (r *Renderer) StartFrame() {
	r.makeCurrent()
	r.cur.hcs, r.cur.stroke, r.cur.hcf, r.cur.fill, r.cur.texture = 0, nil, 0, nil, nil

	glapi.Enable(glapi.NORMALIZE)
	glapi.BlendFunc(glapi.SRC_ALPHA, glapi.ONE_MINUS_SRC_ALPHA)
	glapi.Enable(glapi.BLEND)
	glapi.Enable(glapi.LINE_SMOOTH)
	glapi.Hint(glapi.LINE_SMOOTH_HINT, glapi.NICEST)
	glapi.DepthFunc(glapi.LEQUAL)
	glapi.Enable(glapi.DEPTH_TEST)

	l := &r.lighting
	lights := [2]uint32{glapi.LIGHT0, glapi.LIGHT1}
	for i, light := range lights {
		if l.LightIntensity[i] == 0.0 {
			continue
		}
		glapi.Enable(light)
		f := float32(l.LightIntensity[i])
		li := [4]float32{f, f, f, 1}
		glapi.Lightfv(light, glapi.DIFFUSE, &li[0])
		glapi.Lightfv(light, glapi.SPECULAR, &li[0])
		ld := r.camera.VectorFromProjs(l.LightDirection[i])
		pos := [4]float32{float32(ld.X), float32(ld.Y), float32(ld.Z), 0}
		glapi.Lightfv(light, glapi.POSITION, &pos[0])
	}
	if l.AmbientIntensity != 0.0 {
		a := float32(l.AmbientIntensity)
		amb := [4]float32{a, a, a, 1}
		glapi.LightModelfv(glapi.LIGHT_MODEL_AMBIENT, &amb[0])
	}

	bg := l.BackgroundColor
	glapi.ClearColor(bg.RedF(), bg.GreenF(), bg.BlueF(), bg.AlphaF())
	glapi.Clear(glapi.COLOR_BUFFER_BIT)
	glapi.ClearDepth(1.0)
	glapi.Clear(glapi.DEPTH_BUFFER_BIT)
}

// FlushFrame sends what is batched.
func (r *Renderer) FlushFrame() {
	r.unSelectPrimitive()
	glapi.Flush()
}

// FinishFrame waits for the frame to be drawn.
func (r *Renderer) FinishFrame() {
	r.unSelectPrimitive()
	glapi.Finish()
	// An error here is the driver's, and drawing goes on as in C++,
	// which only logs it; clear it so the next frame's check is fresh.
	glapi.GetError()
}

// ReadFrame reads the frame drawn so far back, top row first and
// opaque.
func (r *Renderer) ReadFrame() *image.RGBA {
	r.unSelectPrimitive()
	w, h := r.pixelSize()
	if r.ctx.DoubleBuffered() {
		glapi.ReadBuffer(glapi.BACK)
	} else {
		glapi.ReadBuffer(glapi.FRONT)
	}
	glapi.PixelStorei(glapi.PACK_ALIGNMENT, 1)
	buf := make([]byte, 4*w*h)
	glapi.ReadPixels(0, 0, int32(w), int32(h), glapi.RGBA, glapi.UNSIGNED_BYTE, unsafe.Pointer(&buf[0]))
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		copy(img.Pix[y*img.Stride:y*img.Stride+4*w], buf[(h-1-y)*4*w:(h-y)*4*w])
	}
	for i := 3; i < len(img.Pix); i += 4 {
		img.Pix[i] = 255
	}
	return img
}

// GetIdent is GL's vendor, renderer and version strings.
func (r *Renderer) GetIdent() (vendor, renderer, version string) {
	r.makeCurrent()
	str := func(name uint32) string {
		p := glapi.GetString(name)
		if p == nil {
			return ""
		}
		return glapi.GoStr(p)
	}
	return str(glapi.VENDOR), str(glapi.RENDERER), str(glapi.VERSION)
}

// Clear forgets the strokes and fills.
func (r *Renderer) Clear() {
	r.unSelectPrimitive()
	r.CanvasBase.Clear()
	r.cur.hcs, r.cur.stroke, r.cur.hcf, r.cur.fill = 0, nil, 0, nil
}

// CreateBatch returns nil, as the GL 1 renderer does.
func (r *Renderer) CreateBatch() render.BatchCanvas { return nil }

func (r *Renderer) selectPrimitive(mode uint32) {
	if r.cur.drawing && r.cur.mode == mode {
		return
	} else if r.cur.drawing {
		glapi.End()
	}
	glapi.Begin(mode)
	r.cur.drawing = true
	r.cur.mode = mode
}

func (r *Renderer) unSelectPrimitive() {
	if !r.cur.drawing {
		return
	}
	glapi.End()
	r.cur.drawing = false
}

func vertex(v geom.Vector) { glapi.Vertex3d(v.X, v.Y, v.Z) }
func normal(n geom.Vector) { glapi.Normal3d(n.X, n.Y, n.Z) }

func colorRGBA(c core.RgbaColor) {
	glapi.Color4d(float64(c.RedF()), float64(c.GreenF()), float64(c.BlueF()), float64(c.AlphaF()))
}

func materialRGBA(side uint32, c core.RgbaColor) {
	m := [4]float32{c.RedF(), c.GreenF(), c.BlueF(), c.AlphaF()}
	glapi.Materialfv(side, glapi.AMBIENT_AND_DIFFUSE, &m[0])
}

// depthRange is ssglDepthRange.
func depthRange(layer render.Layer, zIndex int) {
	switch layer {
	case render.LayerNormal, render.LayerFront, render.LayerBack:
		glapi.DepthFunc(glapi.LEQUAL)
		glapi.DepthMask(true)
		glapi.ColorMask(true, true, true, true)
	case render.LayerDepthOnly:
		glapi.DepthFunc(glapi.LEQUAL)
		glapi.DepthMask(true)
		glapi.ColorMask(false, false, false, false)
	case render.LayerOccluded:
		glapi.DepthFunc(glapi.GREATER)
		glapi.DepthMask(false)
		glapi.ColorMask(true, true, true, true)
	}
	switch layer {
	case render.LayerFront:
		glapi.DepthRange(0.0, 0.0)
	case render.LayerBack:
		glapi.DepthRange(1.0, 1.0)
	default:
		// The size of this step depends on the resolution of the Z
		// buffer; for a 16-bit buffer, this should be fine.
		offset := 1.0 / (65535 * 0.8) * float64(zIndex)
		glapi.DepthRange(0.1-offset, 1.0-offset)
	}
}

// The polygon stipple masks of the two checkered fills.
var maskA, maskB = func() (a, b [32 * 32 / 8]uint8) {
	for x := range 32 {
		for y := range 32 {
			i, bit := y*4+x/8, x%8
			ym, xm := y%4, x%4
			for k := range 2 {
				if xm >= 1 && xm <= 2 && ym >= 1 && ym <= 2 {
					if k == 0 {
						b[i] |= 0x80 >> bit
					} else {
						a[i] |= 0x80 >> bit
					}
				}
				ym, xm = (ym+2)%4, (xm+2)%4
			}
		}
	}
	return
}()

func fillPattern(p render.FillPattern) {
	switch p {
	case render.FillSolid:
		glapi.Disable(glapi.POLYGON_STIPPLE)
	case render.FillCheckeredA:
		glapi.Enable(glapi.POLYGON_STIPPLE)
		glapi.PolygonStipple(&maskA[0])
	case render.FillCheckeredB:
		glapi.Enable(glapi.POLYGON_STIPPLE)
		glapi.PolygonStipple(&maskB[0])
	}
}

func (r *Renderer) selectStroke(hcs render.HStroke) *render.Stroke {
	if r.cur.hcs == hcs && r.cur.stroke != nil {
		return r.cur.stroke
	}
	s := r.Stroke(hcs)
	r.unSelectPrimitive()
	colorRGBA(s.Color)
	depthRange(s.Layer, s.ZIndex)
	glapi.LineWidth(float32(s.WidthPx(&r.camera)))
	// Fat lines and points are quads affected by glPolygonStipple, so
	// make sure they are displayed correctly.
	fillPattern(render.FillSolid)
	glapi.Disable(glapi.TEXTURE_2D)

	r.cur.hcs, r.cur.stroke = hcs, s
	r.cur.hcf, r.cur.fill = 0, nil
	r.cur.texture = nil
	return s
}

func (r *Renderer) selectFill(hcf render.HFill) *render.Fill {
	if r.cur.hcf == hcf && r.cur.fill != nil {
		return r.cur.fill
	}
	f := r.Fill(hcf)
	r.unSelectPrimitive()
	colorRGBA(f.Color)
	depthRange(f.Layer, f.ZIndex)
	fillPattern(f.Pattern)
	glapi.Disable(glapi.TEXTURE_2D)

	r.cur.hcs, r.cur.stroke = 0, nil
	r.cur.hcf, r.cur.fill = hcf, f
	r.cur.texture = nil
	return f
}

func (r *Renderer) selectTexture(pm *res.Pixmap) {
	if r.cur.texture == pm {
		return
	}
	glapi.BindTexture(glapi.TEXTURE_2D, 1)
	glapi.TexParameteri(glapi.TEXTURE_2D, glapi.TEXTURE_MAG_FILTER, glapi.NEAREST)
	glapi.TexParameteri(glapi.TEXTURE_2D, glapi.TEXTURE_MIN_FILTER, glapi.NEAREST)
	glapi.TexParameteri(glapi.TEXTURE_2D, glapi.TEXTURE_WRAP_S, glapi.CLAMP)
	glapi.TexParameteri(glapi.TEXTURE_2D, glapi.TEXTURE_WRAP_T, glapi.CLAMP)
	glapi.TexEnvf(glapi.TEXTURE_ENV, glapi.TEXTURE_ENV_MODE, glapi.REPLACE)

	var format uint32
	switch pm.Format {
	case res.FormatRGBA:
		format = glapi.RGBA
	case res.FormatRGB:
		format = glapi.RGB
	case res.FormatA:
		format = glapi.ALPHA
	default:
		panic("gl: unexpected pixmap format")
	}
	// Rows are padded to four bytes, GL's default unpack alignment.
	glapi.TexImage2D(glapi.TEXTURE_2D, 0, int32(format), int32(pm.Width), int32(pm.Height), 0,
		format, glapi.UNSIGNED_BYTE, unsafe.Pointer(&pm.Data[0]))
	glapi.Enable(glapi.TEXTURE_2D)
	r.cur.texture = pm
}

// endcapCircle is cos and sin of (pi*i/10 + pi/2), for i from 0 to 10.
var endcapCircle = [11][2]float64{
	{0.0000, 1.0000},
	{-0.3090, 0.9511},
	{-0.5878, 0.8090},
	{-0.8090, 0.5878},
	{-0.9511, 0.3090},
	{-1.0000, 0.0000},
	{-0.9511, -0.3090},
	{-0.8090, -0.5878},
	{-0.5878, -0.8090},
	{-0.3090, -0.9511},
	{0.0000, -1.0000},
}

func (r *Renderer) doFatLineEndcap(p, u, v geom.Vector) {
	r.selectPrimitive(glapi.TRIANGLE_FAN)
	for _, pc := range endcapCircle {
		vertex(p.Plus(u.ScaledBy(pc[0])).Plus(v.ScaledBy(pc[1])))
	}
	r.unSelectPrimitive()
}

// doFatLine draws a line wider than GL_LINES can as a quad with round
// caps, of constant thickness on screen.
func (r *Renderer) doFatLine(a, b geom.Vector, width float64) {
	hw := width / 2
	ab := b.Minus(a)
	gn := r.camera.ProjRight.Cross(r.camera.ProjUp)
	abn := ab.Cross(gn).WithMagnitude(1)
	abn = abn.Minus(gn.ScaledBy(gn.Dot(abn)))
	abn = abn.WithMagnitude(hw)
	ab = gn.Cross(abn)
	ab = ab.WithMagnitude(hw)

	r.selectPrimitive(glapi.QUADS)
	vertex(a.Plus(abn))
	vertex(b.Plus(abn))
	vertex(b.Minus(abn))
	vertex(a.Minus(abn))
	r.doFatLineEndcap(a, ab, abn)
	r.doFatLineEndcap(b, ab.ScaledBy(-1), abn)
}

func (r *Renderer) doLine(a, b geom.Vector, hcs render.HStroke) {
	if a.Equals(b) {
		return
	}
	s := r.selectStroke(hcs)
	if wpx := s.WidthPx(&r.camera); wpx <= 3.0 {
		r.selectPrimitive(glapi.LINES)
		vertex(a)
		vertex(b)
	} else {
		r.doFatLine(a, b, wpx/r.camera.Scale)
	}
}

func (r *Renderer) doPoint(p geom.Vector, d float64) {
	u := r.camera.ProjRight.WithMagnitude(d / 2.0 / r.camera.Scale)
	if d <= 3.0 {
		r.selectPrimitive(glapi.LINES)
		vertex(p.Minus(u))
		vertex(p.Plus(u))
		return
	}
	v := r.camera.ProjUp.WithMagnitude(d / 2.0 / r.camera.Scale)
	r.doFatLineEndcap(p, u, v)
	r.doFatLineEndcap(p, u.ScaledBy(-1.0), v)
}

func (r *Renderer) doStippledLine(a, b geom.Vector, hcs render.HStroke, phase float64) {
	s := r.selectStroke(hcs)
	if s.StipplePattern == core.StippleContinuous {
		r.doLine(a, b, hcs)
		return
	}

	scale := s.StippleScaleMm(&r.camera)
	dashes := core.StipplePatternDashes(s.StipplePattern)
	length := core.StipplePatternLength(s.StipplePattern) * scale

	phase -= math.Floor(phase/length) * length

	curPhase := 0.0
	curDash := 0
	for curDash = 0; curDash < len(dashes); curDash++ {
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

	curDashLen := (curPhase - phase) / scale
	for cur < length2 {
		next := math.Min(length2, cur+curDashLen*scale)
		nextPos := curPos.Plus(dir.ScaledBy(next - cur))
		if curDash%2 == 0 {
			if curDashLen <= geom.LengthEps {
				r.doPoint(curPos, width)
			} else {
				r.doLine(curPos, nextPos, hcs)
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
	r.selectPrimitive(glapi.QUADS)
	vertex(a)
	vertex(b)
	vertex(c)
	vertex(d)
}

// DrawPoint draws a point as a square of the stroke's width.
func (r *Renderer) DrawPoint(o geom.Vector, hcs render.HStroke) {
	s := r.selectStroke(hcs)
	hcf := r.GetFill(render.Fill{Layer: s.Layer, ZIndex: s.ZIndex, Color: s.Color})
	rr := r.camera.ProjRight.ScaledBy(s.Width / 2.0 / r.camera.Scale)
	u := r.camera.ProjUp.ScaledBy(s.Width / 2.0 / r.camera.Scale)
	r.DrawQuad(o.Plus(rr).Plus(u), o.Plus(rr).Minus(u), o.Minus(rr).Minus(u), o.Minus(rr).Plus(u), hcf)
}

// DrawPolygon fills a planar polygon with the odd winding rule, as
// the software renderer does, through the Go triangulator.
func (r *Renderer) DrawPolygon(p *srf.SPolygon, hcf render.HFill) {
	r.unSelectPrimitive()
	r.selectFill(hcf)

	m := triangulate(p)
	normal(p.Normal)
	r.selectPrimitive(glapi.TRIANGLES)
	for i := range m.Triangles {
		tr := &m.Triangles[i]
		vertex(tr.A)
		vertex(tr.B)
		vertex(tr.C)
	}
	r.unSelectPrimitive()
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

// DrawMesh draws a lit mesh: with a back fill, its back faces in that
// colour (two-sided lighting); an empty fill colour means each
// triangle's own.
func (r *Renderer) DrawMesh(m *srf.SMesh, hcfFront, hcfBack render.HFill) {
	r.unSelectPrimitive()

	front := r.selectFill(hcfFront)
	frontColor := front.Color
	var backColor core.RgbaColor

	materialRGBA(glapi.FRONT, frontColor)
	if hcfBack != 0 {
		back := r.Fill(hcfBack)
		backColor = back.Color
		if front.Layer != back.Layer || front.ZIndex != back.ZIndex {
			panic("gl: frontFill and backFill should belong to the same depth range")
		}
		if front.Pattern != back.Pattern {
			panic("gl: frontFill and backFill should have the same pattern")
		}
		glapi.LightModeli(glapi.LIGHT_MODEL_TWO_SIDE, 1)
		materialRGBA(glapi.BACK, backColor)
	} else {
		glapi.LightModeli(glapi.LIGHT_MODEL_TWO_SIDE, 0)
	}

	var triangleColor core.RgbaColor
	glapi.Enable(glapi.LIGHTING)
	glapi.Begin(glapi.TRIANGLES)
	for i := range m.Triangles {
		tr := &m.Triangles[i]
		if frontColor.IsEmpty() || backColor.IsEmpty() {
			if triangleColor.IsEmpty() || !triangleColor.Equals(tr.Meta.RgbaColor) {
				triangleColor = tr.Meta.RgbaColor
				if frontColor.IsEmpty() {
					materialRGBA(glapi.FRONT, triangleColor)
				}
				if backColor.IsEmpty() {
					materialRGBA(glapi.BACK, triangleColor)
				}
			}
		}
		if tr.NA.EqualsExactly(geom.Vector{}) {
			normal(tr.Normal())
			vertex(tr.A)
			vertex(tr.B)
			vertex(tr.C)
		} else {
			normal(tr.NA)
			vertex(tr.A)
			normal(tr.NB)
			vertex(tr.B)
			normal(tr.NC)
			vertex(tr.C)
		}
	}
	glapi.End()
	glapi.Disable(glapi.LIGHTING)
}

// DrawFaces fills the triangles of the given faces, unlit.
func (r *Renderer) DrawFaces(m *srf.SMesh, faces []uint32, hcf render.HFill) {
	r.selectFill(hcf)
	r.selectPrimitive(glapi.TRIANGLES)
	for i := range m.Triangles {
		tr := &m.Triangles[i]
		face := uint32(tr.Meta.Face)
		for _, f := range faces {
			if f != face {
				continue
			}
			vertex(tr.A)
			vertex(tr.B)
			vertex(tr.C)
			break
		}
	}
}

// DrawPixmap draws a textured quad o, o+u, o+u+v, o+v with texture
// coordinates from ta to tb.
func (r *Renderer) DrawPixmap(pm *res.Pixmap, o, u, v geom.Vector, ta, tb geom.Point2d, hcf render.HFill) {
	r.unSelectPrimitive()
	r.selectFill(hcf)
	r.selectTexture(pm)
	r.selectPrimitive(glapi.QUADS)
	glapi.TexCoord2d(ta.U, ta.V)
	vertex(o)
	glapi.TexCoord2d(ta.U, tb.V)
	vertex(o.Plus(v))
	glapi.TexCoord2d(tb.U, tb.V)
	vertex(o.Plus(u).Plus(v))
	glapi.TexCoord2d(tb.U, ta.V)
	vertex(o.Plus(u))
}

// InvalidatePixmap makes the next draw of pm upload it again.
func (r *Renderer) InvalidatePixmap(pm *res.Pixmap) {
	if r.cur.texture == pm {
		r.cur.texture = nil
	}
}
