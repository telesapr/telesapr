package soft

import (
	"math"

	"github.com/spacesolve/spacesolve/pkg/res"
)

// depthFunc is the depth comparison in use (GL_LEQUAL or GL_GREATER).
type depthFunc int

const (
	depthLEqual depthFunc = iota
	depthGreater
)

// fragMode is how a fragment's colour is found.
type fragMode int

const (
	fragSolid   fragMode = iota // the current colour
	fragColor                   // interpolated vertex colours
	fragTexture                 // the current texture, GL_REPLACE
)

// state is the GL state the fixed-function renderer sets per stroke or
// fill (ssglDepthRange, ssglFillPattern, glColor, the bound texture).
type state struct {
	depthFunc  depthFunc
	depthWrite bool
	colorWrite bool
	depthNear  float64
	depthFar   float64
	stipple    *[32]uint32
	color      [4]float64
	texture    *res.Pixmap
}

// cvert is a vertex in clip space with its attributes.
type cvert struct {
	x, y, z, w float64
	col        [4]float64
	s, t       float64
}

// rvert is a vertex in window space: x right and y down in pixels, z
// the depth in [0, 1], and 1/w for perspective-correct attributes.
type rvert struct {
	x, y, z float64
	invW    float64
	col     [4]float64
	s, t    float64
}

func lerpC(a, b *cvert, t float64) cvert {
	var v cvert
	v.x = a.x + (b.x-a.x)*t
	v.y = a.y + (b.y-a.y)*t
	v.z = a.z + (b.z-a.z)*t
	v.w = a.w + (b.w-a.w)*t
	for i := range v.col {
		v.col[i] = a.col[i] + (b.col[i]-a.col[i])*t
	}
	v.s = a.s + (b.s-a.s)*t
	v.t = a.t + (b.t-a.t)*t
	return v
}

// guardBand is how far outside the viewport, in viewport sizes,
// geometry is kept before clipping it in x and y; the rasterizer clamps
// the rest to the frame.
const guardBand = 8.0

// wEpsilon keeps geometry in front of the eye in perspective.
const wEpsilon = 1e-9

// clip planes: each returns the signed distance, inside when >= 0.
var clipPlanes = [...]func(v *cvert) float64{
	func(v *cvert) float64 { return v.w - wEpsilon },
	func(v *cvert) float64 { return v.w + v.z },
	func(v *cvert) float64 { return v.w - v.z },
	func(v *cvert) float64 { return guardBand*v.w + v.x },
	func(v *cvert) float64 { return guardBand*v.w - v.x },
	func(v *cvert) float64 { return guardBand*v.w + v.y },
	func(v *cvert) float64 { return guardBand*v.w - v.y },
}

// clipPolygon clips a convex polygon in clip space (Sutherland-Hodgman).
func clipPolygon(in []cvert) []cvert {
	cur := in
	for _, plane := range clipPlanes {
		if len(cur) == 0 {
			return cur
		}
		allIn := true
		for i := range cur {
			if plane(&cur[i]) < 0 {
				allIn = false
				break
			}
		}
		if allIn {
			continue
		}
		out := make([]cvert, 0, len(cur)+2)
		for i := range cur {
			a := &cur[i]
			b := &cur[(i+1)%len(cur)]
			da, db := plane(a), plane(b)
			if da >= 0 {
				out = append(out, *a)
			}
			if (da >= 0) != (db >= 0) {
				out = append(out, lerpC(a, b, da/(da-db)))
			}
		}
		cur = out
	}
	return cur
}

// clipSegment clips a segment in clip space; ok is false if nothing is
// left.
func clipSegment(a, b cvert) (cvert, cvert, bool) {
	for _, plane := range clipPlanes {
		da, db := plane(&a), plane(&b)
		if da < 0 && db < 0 {
			return a, b, false
		}
		if da < 0 {
			a = lerpC(&a, &b, da/(da-db))
		} else if db < 0 {
			b = lerpC(&a, &b, da/(da-db))
		}
	}
	return a, b, true
}

// toWindow maps a clip-space vertex to window space with the current
// depth range.
func (r *Renderer) toWindow(v *cvert) rvert {
	invW := 1 / v.w
	nx, ny, nz := v.x*invW, v.y*invW, v.z*invW
	return rvert{
		x:    (nx + 1) * 0.5 * float64(r.width),
		y:    float64(r.height) - (ny+1)*0.5*float64(r.height),
		z:    r.cur.depthNear + (r.cur.depthFar-r.cur.depthNear)*(nz+1)*0.5,
		invW: invW,
		col:  v.col,
		s:    v.s,
		t:    v.t,
	}
}

// fragment runs the stipple, depth and blend stages for pixel x, y of
// the frame (y down) with depth z and straight-alpha colour col.
func (r *Renderer) fragment(x, y int, z float64, col [4]float64) {
	if st := r.cur.stipple; st != nil {
		// The pattern is in GL window coordinates, y up.
		gy := r.height - 1 - y
		if st[gy&31]&(0x80000000>>uint(x&31)) == 0 {
			return
		}
	}
	idx := y*r.width + x
	zf := float32(z)
	d := r.depth[idx]
	switch r.cur.depthFunc {
	case depthLEqual:
		if !(zf <= d) {
			return
		}
	case depthGreater:
		if !(zf > d) {
			return
		}
	}
	if r.cur.depthWrite {
		r.depth[idx] = zf
	}
	if !r.cur.colorWrite {
		return
	}
	a := clamp01(col[3])
	p := r.frame.Pix[idx*4 : idx*4+4 : idx*4+4]
	if a == 1 {
		// src*1 + dst*0 is src, exactly.
		for i := 0; i < 3; i++ {
			p[i] = uint8(math.Round(clamp01(col[i]) * 255))
		}
		return
	}
	for i := 0; i < 3; i++ {
		src := clamp01(col[i]) * 255
		p[i] = uint8(math.Round(src*a + float64(p[i])*(1-a)))
	}
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// texel samples the current texture at s, t with nearest filtering and
// GL_CLAMP, combined with the fragment colour as GL_REPLACE does.
func (r *Renderer) texel(s, t float64, frag [4]float64) [4]float64 {
	pm := r.cur.texture
	i := int(math.Floor(s * float64(pm.Width)))
	j := int(math.Floor(t * float64(pm.Height)))
	i = min(max(i, 0), pm.Width-1)
	j = min(max(j, 0), pm.Height-1)
	c := pm.GetPixel(i, j)
	switch pm.Format {
	case res.FormatA:
		return [4]float64{frag[0], frag[1], frag[2], float64(c.Alpha) / 255}
	case res.FormatRGB, res.FormatBGR:
		return [4]float64{float64(c.Red) / 255, float64(c.Green) / 255, float64(c.Blue) / 255, frag[3]}
	default:
		return [4]float64{float64(c.Red) / 255, float64(c.Green) / 255, float64(c.Blue) / 255, float64(c.Alpha) / 255}
	}
}

// edgeIncludes breaks ties for pixels exactly on an edge so that two
// triangles sharing it don't both draw them: of the two directions an
// edge is walked in, exactly one qualifies.
func edgeIncludes(dx, dy float64) bool {
	return dy > 0 || (dy == 0 && dx > 0)
}

// rasterTriangle fills a triangle in window space; mode picks where
// each fragment's colour comes from.
func (r *Renderer) rasterTriangle(a, b, c *rvert, mode fragMode) {
	r.rasterTriangleRows(a, b, c, mode, 0, r.height)
}

// rasterTriangleRows is rasterTriangle for the rows from y0 up to y1
// only, so that bands of the frame can be drawn at once.
func (r *Renderer) rasterTriangleRows(a, b, c *rvert, mode fragMode, y0, y1 int) {
	area := (b.x-a.x)*(c.y-a.y) - (b.y-a.y)*(c.x-a.x)
	if area == 0 || math.IsNaN(area) {
		return
	}
	if area < 0 {
		b, c = c, b
		area = -area
	}
	minX := int(math.Floor(min(a.x, b.x, c.x)))
	maxX := int(math.Ceil(max(a.x, b.x, c.x)))
	minY := int(math.Floor(min(a.y, b.y, c.y)))
	maxY := int(math.Ceil(max(a.y, b.y, c.y)))
	minX = max(minX, 0)
	minY = max(minY, 0, y0)
	maxX = min(maxX, r.width-1)
	maxY = min(maxY, r.height-1, y1-1)
	if minX > maxX || minY > maxY {
		return
	}

	// Edge e_i is opposite vertex i; it is positive inside.
	type edge struct{ ox, oy, dx, dy float64 }
	edges := [3]edge{
		{b.x, b.y, c.x - b.x, c.y - b.y},
		{c.x, c.y, a.x - c.x, a.y - c.y},
		{a.x, a.y, b.x - a.x, b.y - a.y},
	}
	var incl [3]bool
	for i, e := range edges {
		incl[i] = edgeIncludes(e.dx, e.dy)
	}
	invArea := 1 / area
	for py := minY; py <= maxY; py++ {
		fy := float64(py) + 0.5
		// Each edge bounds x on this row: w_i = k_i - dy_i*fx >= 0.
		lo, hi := float64(minX), float64(maxX)+1
		empty := false
		var k [3]float64
		for i, e := range edges {
			k[i] = e.dx*(fy-e.oy) + e.dy*e.ox
			switch {
			case e.dy > 0:
				hi = min(hi, k[i]/e.dy)
			case e.dy < 0:
				lo = max(lo, k[i]/e.dy)
			default:
				if k[i] < 0 || (k[i] == 0 && !incl[i]) {
					empty = true
				}
			}
		}
		if empty || lo > hi {
			continue
		}
		// Pixels whose centres are within [lo, hi], one either side to
		// be safe from rounding; the exact test decides.
		x0 := max(int(math.Ceil(lo-0.5))-1, minX)
		x1 := min(int(math.Floor(hi-0.5))+1, maxX)
		for px := x0; px <= x1; px++ {
			fx := float64(px) + 0.5
			var w [3]float64
			inside := true
			for i, e := range edges {
				w[i] = e.dx*(fy-e.oy) - e.dy*(fx-e.ox)
				if w[i] < 0 || (w[i] == 0 && !incl[i]) {
					inside = false
					break
				}
			}
			if !inside {
				continue
			}
			l0, l1, l2 := w[0]*invArea, w[1]*invArea, w[2]*invArea
			z := l0*a.z + l1*b.z + l2*c.z
			var col [4]float64
			switch mode {
			case fragSolid:
				col = r.cur.color
			case fragColor:
				q0, q1, q2 := l0*a.invW, l1*b.invW, l2*c.invW
				qs := 1 / (q0 + q1 + q2)
				for k := range col {
					col[k] = (q0*a.col[k] + q1*b.col[k] + q2*c.col[k]) * qs
				}
			case fragTexture:
				q0, q1, q2 := l0*a.invW, l1*b.invW, l2*c.invW
				qs := 1 / (q0 + q1 + q2)
				s := (q0*a.s + q1*b.s + q2*c.s) * qs
				t := (q0*a.t + q1*b.t + q2*c.t) * qs
				col = r.texel(s, t, r.cur.color)
			}
			r.fragment(px, py, z, col)
		}
	}
}

// rasterLine draws an antialiased line of the given width in pixels,
// as GL does with GL_LINE_SMOOTH: each pixel's coverage of the line's
// rectangle scales its alpha.
func (r *Renderer) rasterLine(a, b *rvert, width float64) {
	r.rasterLineRows(a, b, width, 0, r.height)
}

// rasterLineRows is rasterLine for the rows from y0 up to y1 only.
func (r *Renderer) rasterLineRows(a, b *rvert, width float64, y0, y1 int) {
	dx, dy := b.x-a.x, b.y-a.y
	length := math.Hypot(dx, dy)
	if length == 0 || math.IsNaN(length) {
		return
	}
	ux, uy := dx/length, dy/length
	hw := width / 2
	maxCov := min(width, 1)
	ext := hw + 1
	minX := int(math.Floor(min(a.x, b.x) - ext))
	maxX := int(math.Ceil(max(a.x, b.x) + ext))
	minY := int(math.Floor(min(a.y, b.y) - ext))
	maxY := int(math.Ceil(max(a.y, b.y) + ext))
	minX = max(minX, 0)
	minY = max(minY, 0, y0)
	maxX = min(maxX, r.width-1)
	maxY = min(maxY, r.height-1, y1-1)
	// Only the pixels near the segment can be covered: in each row, those
	// within hw+0.5 of its line and within 0.5 of its ends. Bound them,
	// with a margin far wider than the rounding of either this bound or
	// the test below, and test just those; the test decides each pixel
	// as before.
	reach := hw + 0.5
	for py := minY; py <= maxY; py++ {
		fy := float64(py) + 0.5 - a.y
		lo, hi := minX, maxX
		if uy != 0 {
			c := fy * ux
			lo, hi = narrowColumns(lo, hi, (c-reach)/uy, (c+reach)/uy, a.x, 1e-9/math.Abs(uy))
		}
		if ux != 0 {
			d := fy * uy
			lo, hi = narrowColumns(lo, hi, (-0.5-d)/ux, (length+0.5-d)/ux, a.x,
				1e-9*(1+length)/math.Abs(ux))
		}
		for px := lo; px <= hi; px++ {
			fx := float64(px) + 0.5 - a.x
			along := fx*ux + fy*uy
			perp := math.Abs(fx*uy - fy*ux)
			cp := min(max(hw+0.5-perp, 0), maxCov)
			if cp <= 0 {
				continue
			}
			ca := min(max(min(along, length-along)+0.5, 0), 1)
			cov := cp * ca
			if cov <= 0 {
				continue
			}
			t := min(max(along/length, 0), 1)
			z := a.z + (b.z-a.z)*t
			col := r.cur.color
			col[3] *= cov
			r.fragment(px, py, z, col)
		}
	}
}

// narrowColumns narrows the columns lo to hi to those whose centres
// fall between f1 and f2 (in either order) relative to x0, give or take
// margin.
func narrowColumns(lo, hi int, f1, f2, x0, margin float64) (int, int) {
	if f1 > f2 {
		f1, f2 = f2, f1
	}
	l := math.Floor(f1 + x0 - 0.5 - margin)
	h := math.Ceil(f2 + x0 - 0.5 + margin)
	if l > float64(lo) {
		lo = int(min(l, float64(hi)+1))
	}
	if h < float64(hi) {
		hi = int(max(h, float64(lo)-1))
	}
	return lo, hi
}
