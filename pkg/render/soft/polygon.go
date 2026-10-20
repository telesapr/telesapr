package soft

import (
	"math"
	"slices"

	"github.com/telesapr/telesapr/pkg/srf"
)

// fillPlanarPolygon fills a planar polygon of any number of contours
// with the odd winding rule, one scanline at a time. Its depth is
// affine in window coordinates, since it is planar. A polygon partly
// behind the eye isn't drawn.
func (r *Renderer) fillPlanarPolygon(p *srf.SPolygon) {
	r.readyDepth()
	type wpt struct{ x, y, z float64 }
	var contours [][]wpt
	var all []wpt
	for _, sc := range p.Contours {
		var c []wpt
		for _, sp := range sc.Points {
			cv := r.clip(sp.P)
			if cv.w <= wEpsilon {
				return
			}
			pt := wpt{
				x: (cv.x/cv.w + 1) * 0.5 * float64(r.width),
				y: float64(r.height) - (cv.y/cv.w+1)*0.5*float64(r.height),
				z: cv.z / cv.w,
			}
			c = append(c, pt)
			all = append(all, pt)
		}
		if len(c) >= 3 {
			contours = append(contours, c)
		}
	}
	if len(contours) == 0 {
		return
	}

	// The plane nz = a*x + b*y + c through the three points spanning
	// the largest triangle.
	p0 := all[0]
	i1, best := -1, 0.0
	for i := range all {
		if d := math.Hypot(all[i].x-p0.x, all[i].y-p0.y); d > best {
			i1, best = i, d
		}
	}
	if i1 < 0 {
		return
	}
	p1 := all[i1]
	i2, bestA := -1, 0.0
	for i := range all {
		a := math.Abs((p1.x-p0.x)*(all[i].y-p0.y) - (p1.y-p0.y)*(all[i].x-p0.x))
		if a > bestA {
			i2, bestA = i, a
		}
	}
	if i2 < 0 {
		// Seen edge on.
		return
	}
	p2 := all[i2]
	det := (p1.x-p0.x)*(p2.y-p0.y) - (p1.y-p0.y)*(p2.x-p0.x)
	a := ((p1.z-p0.z)*(p2.y-p0.y) - (p2.z-p0.z)*(p1.y-p0.y)) / det
	b := ((p2.z-p0.z)*(p1.x-p0.x) - (p1.z-p0.z)*(p2.x-p0.x)) / det
	c := p0.z - a*p0.x - b*p0.y

	minY, maxY := math.Inf(1), math.Inf(-1)
	for _, pt := range all {
		minY = min(minY, pt.y)
		maxY = max(maxY, pt.y)
	}
	y0 := max(int(math.Floor(minY)), 0)
	y1 := min(int(math.Ceil(maxY)), r.height-1)
	var xs []float64
	for py := y0; py <= y1; py++ {
		yc := float64(py) + 0.5
		xs = xs[:0]
		for _, ct := range contours {
			for i := range ct {
				e0, e1 := ct[i], ct[(i+1)%len(ct)]
				if (e0.y <= yc) != (e1.y <= yc) {
					xs = append(xs, e0.x+(yc-e0.y)*(e1.x-e0.x)/(e1.y-e0.y))
				}
			}
		}
		slices.Sort(xs)
		for i := 0; i+1 < len(xs); i += 2 {
			// Pixels whose centres lie in [xa, xb).
			xa := max(int(math.Ceil(xs[i]-0.5)), 0)
			xb := min(int(math.Ceil(xs[i+1]-0.5))-1, r.width-1)
			for px := xa; px <= xb; px++ {
				nz := a*(float64(px)+0.5) + b*yc + c
				if nz < -1 || nz > 1 {
					continue
				}
				if r.userClip {
					// The plane's distance over w is affine in normalized
					// device coordinates, and w > 0 here.
					eq := &r.userEq
					nx := 2*(float64(px)+0.5)/float64(r.width) - 1
					ny := 1 - 2*yc/float64(r.height)
					if eq[0]*nx+eq[1]*ny+eq[2]*nz+eq[3] < 0 {
						continue
					}
				}
				z := r.cur.depthNear + (r.cur.depthFar-r.cur.depthNear)*(nz+1)*0.5
				r.fragment(px, py, z, r.cur.color)
			}
		}
	}
}
