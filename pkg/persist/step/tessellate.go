package step

import (
	"errors"
	"fmt"
	"math"
	"sort"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/internal/cdt"
)

// Limits that keep a hostile or enormous file from exhausting memory.
const (
	maxEdgePoints = 4096
	maxGridPoints = 40000
)

// errTrim is a face whose bounds this tessellator could not make into
// a region of its surface's parameters.
var errTrim = errors.New("its bounds could not be laid out on its surface")

// polyline is the edge's points from start to end, its curve divided
// so that no chord is further than tol from it. It is computed once
// and shared by the faces on both sides, so they meet exactly.
func (e *Edge) polyline(tol float64) ([]geom.Vector, error) {
	e.once.Do(func() { e.poly, e.err = sampleEdge(e, tol) })
	return e.poly, e.err
}

func sampleEdge(e *Edge, tol float64) ([]geom.Vector, error) {
	c := e.Curve
	if _, ok := c.(*line); ok {
		if e.Closed {
			return nil, fmt.Errorf("edge #%d is a line that starts where it ends", e.ID)
		}
		return []geom.Vector{e.Start, e.End}, nil
	}
	t0, t1 := c.Param(e.Start), c.Param(e.End)
	lo, hi := c.Range()
	per := c.Period()
	switch {
	case per > 0 && e.Closed:
		if e.Sense {
			t1 = t0 + per
		} else {
			t1 = t0 - per
		}
	case per > 0:
		eps := 1e-12 * per
		if e.Sense {
			for t1 <= t0+eps {
				t1 += per
			}
			for t1-t0 > per {
				t1 -= per
			}
		} else {
			for t1 >= t0-eps {
				t1 -= per
			}
			for t0-t1 > per {
				t1 += per
			}
		}
	case e.Closed:
		if math.IsInf(lo, 0) || math.IsInf(hi, 0) {
			return nil, fmt.Errorf("edge #%d is an open curve that starts where it ends", e.ID)
		}
		t0, t1 = lo, hi
		if !e.Sense {
			t0, t1 = hi, lo
		}
	}
	if t0 == t1 {
		return nil, fmt.Errorf("edge #%d has no length along its curve", e.ID)
	}
	// Start from the knots, or from an eighth of a turn for conics.
	ts := []float64{t0}
	if b, ok := c.(breaker); ok && per == 0 {
		a, z := math.Min(t0, t1), math.Max(t0, t1)
		var in []float64
		for _, k := range b.Breaks() {
			if k > a && k < z {
				in = append(in, k)
			}
		}
		if t1 < t0 {
			for i, j := 0, len(in)-1; i < j; i, j = i+1, j-1 {
				in[i], in[j] = in[j], in[i]
			}
		}
		ts = append(ts, in...)
	} else {
		var n int
		if b, ok := c.(breaker); ok {
			// A closed B-spline: two pieces per knot span.
			n = 2 * len(b.Breaks())
		} else {
			n = int(math.Ceil(math.Abs(t1-t0) / (math.Pi / 8)))
		}
		n = max(1, min(n, 512))
		for i := 1; i < n; i++ {
			ts = append(ts, t0+(t1-t0)*float64(i)/float64(n))
		}
	}
	ts = append(ts, t1)
	pts := []geom.Vector{c.Eval(t0)}
	var refine func(ta, tb float64, pa, pb geom.Vector, depth int)
	refine = func(ta, tb float64, pa, pb geom.Vector, depth int) {
		tm := (ta + tb) / 2
		pm := c.Eval(tm)
		if depth < 14 && len(pts) < maxEdgePoints {
			sag := pm.DistanceToLine(pa, pb.Minus(pa))
			if pa.Equals(pb) {
				sag = pm.Minus(pa).Magnitude()
			}
			q1 := c.Eval((ta+tm)/2).DistanceToLine(pa, pb.Minus(pa))
			q3 := c.Eval((tm+tb)/2).DistanceToLine(pa, pb.Minus(pa))
			if sag > tol || q1 > tol || q3 > tol {
				refine(ta, tm, pa, pm, depth+1)
				refine(tm, tb, pm, pb, depth+1)
				return
			}
		}
		pts = append(pts, pb)
	}
	for i := 0; i+1 < len(ts); i++ {
		refine(ts[i], ts[i+1], c.Eval(ts[i]), c.Eval(ts[i+1]), 0)
	}
	// The ends are the vertices exactly, so faces meet there.
	pts[0], pts[len(pts)-1] = e.Start, e.End
	if len(pts) > maxEdgePoints {
		return nil, fmt.Errorf("edge #%d needs too many points", e.ID)
	}
	return pts, nil
}

// uvPoint is a point of a face's boundary: where it is in space, and
// its parameters on the face's surface. singU (singV) is set where the
// surface collapses so that u (v) is undefined, as at a cone's apex.
type uvPoint struct {
	p            geom.Vector
	u, v         float64
	singU, singV bool
}

// piece is one edge's points as a loop runs along it.
type piece struct {
	pts []uvPoint
	// seam is 1 when the edge is the line where u closes on itself, 2
	// where v does, else 0.
	seam int
}

// contour is a closed loop in parameter space; its last point joins
// its first.
type contour struct {
	pts []uvPoint
	// wrap is the shift, in periods of u and v, between the loop's end
	// and its start, for a loop that goes round a closed surface.
	wrap [2]int
}

// faceSurface is the surface of a face with what the tessellator needs
// of it.
type faceSurface struct {
	Surface
	pu, pv         float64
	u0, u1, v0, v1 float64
	// scale is a length for telling where a derivative vanishes.
	scale float64
}

func (s *faceSurface) point(p geom.Vector, guess *[2]float64) uvPoint {
	u, v := s.Project(p, guess)
	_, su, sv := s.Derivs(u, v)
	eps := 1e-7 * s.scale
	return uvPoint{p: p, u: u, v: v, singU: su.Magnitude() < eps, singV: sv.Magnitude() < eps}
}

// face tessellates a face into triangles whose corners are in space,
// with the surface's normal at each, turned to the face's side.
func tessellateFace(f *Face, tol float64) ([]meshTri, error) {
	if pf, ok := polygonFace(f); ok {
		f = pf
	}
	s := &faceSurface{Surface: f.Surf}
	s.pu, s.pv = f.Surf.Period()
	s.u0, s.u1, s.v0, s.v1 = f.Surf.Domain()

	// Count the uses of each edge, to find the seams.
	uses := map[*Edge]int{}
	for _, b := range f.Bounds {
		for _, oe := range b.Edges {
			uses[oe.E]++
		}
	}
	// The edges' points, to size the face.
	var lo, hi geom.Vector
	first := true
	grow := func(p geom.Vector) {
		if first {
			lo, hi, first = p, p, false
		}
		growBox(p, &lo, &hi)
	}
	polys := map[*Edge][]geom.Vector{}
	for e := range uses {
		pts, err := e.polyline(tol)
		if err != nil {
			return nil, err
		}
		polys[e] = pts
		for _, p := range pts {
			grow(p)
		}
	}
	for _, b := range f.Bounds {
		for _, p := range b.Poly {
			grow(p)
		}
	}
	s.scale = hi.Minus(lo).Magnitude()
	if s.scale == 0 && !math.IsInf(s.u0, 0) && !math.IsInf(s.u1, 0) && !math.IsInf(s.v0, 0) && !math.IsInf(s.v1, 0) {
		// No edges, as on a whole sphere: size the surface itself.
		for i := 0; i <= 4; i++ {
			for j := 0; j <= 4; j++ {
				grow(s.Eval(s.u0+(s.u1-s.u0)*float64(i)/4, s.v0+(s.v1-s.v0)*float64(j)/4))
			}
		}
		s.scale = hi.Minus(lo).Magnitude()
	}
	if s.scale == 0 {
		s.scale = 1
	}

	var contours []contour
	for _, b := range f.Bounds {
		c, err := s.loop(b, polys, uses, f.Sense)
		if err != nil {
			return nil, err
		}
		if len(c.pts) >= 2 {
			contours = append(contours, c)
		}
	}
	if len(contours) == 0 {
		c, err := s.whole()
		if err != nil {
			return nil, err
		}
		contours = append(contours, c)
	}
	contours, err := s.closeWraps(contours, f.Sense)
	if err != nil {
		return nil, err
	}
	return s.triangulate(contours, tol, f.Sense)
}

// polygonFace is a face bounded only by polygons, as a faceted solid's
// are, on the plane of its outer polygon, its normal the way the
// polygon turns: faceted files often give every face one plane, or
// none that fits.
func polygonFace(f *Face) (*Face, bool) {
	if len(f.Bounds) == 0 {
		return nil, false
	}
	for _, b := range f.Bounds {
		if b.Poly == nil || len(b.Edges) > 0 {
			return nil, false
		}
	}
	pts := f.Bounds[0].Poly
	var n geom.Vector
	for i := range pts {
		a, b := pts[i], pts[(i+1)%len(pts)]
		n.X += (a.Y - b.Y) * (a.Z + b.Z)
		n.Y += (a.Z - b.Z) * (a.X + b.X)
		n.Z += (a.X - b.X) * (a.Y + b.Y)
	}
	if n.Magnitude() == 0 {
		return nil, false
	}
	g := *f
	g.Surf = &plane{f: buildAxes(pts[0], n, geom.Vector{}, false)}
	g.Sense = true
	return &g, true
}

// loop lays out a bound in parameter space.
func (s *faceSurface) loop(b *Bound, polys map[*Edge][]geom.Vector, uses map[*Edge]int, sense bool) (contour, error) {
	var pieces []piece
	if b.Poly != nil {
		var pc piece
		var guess *[2]float64
		for _, p := range append(append([]geom.Vector(nil), b.Poly...), b.Poly[0]) {
			q := s.point(p, guess)
			guess = &[2]float64{q.u, q.v}
			pc.pts = append(pc.pts, q)
		}
		pieces = append(pieces, pc)
	}
	for _, oe := range b.Edges {
		src := polys[oe.E]
		pts := make([]geom.Vector, len(src))
		for i := range src {
			if oe.Forward {
				pts[i] = src[i]
			} else {
				pts[i] = src[len(src)-1-i]
			}
		}
		var pc piece
		var guess *[2]float64
		for _, p := range pts {
			q := s.point(p, guess)
			guess = &[2]float64{q.u, q.v}
			pc.pts = append(pc.pts, q)
		}
		if uses[oe.E] >= 2 {
			pc.seam = s.seamOf(pc.pts)
		}
		pieces = append(pieces, pc)
	}
	if len(pieces) == 0 {
		return contour{}, nil
	}
	for i := range pieces {
		s.unwrapPiece(&pieces[i])
	}
	anchor := -1
	for i, pc := range pieces {
		if pc.seam == 0 {
			anchor = i
			break
		}
	}
	allSeams := anchor < 0
	if allSeams {
		// Only seams: the loop is the whole domain's edge, each seam on
		// the side its direction says, and along the domain.
		for i := range pieces {
			s.placeSeam(&pieces[i])
		}
		anchor = 0
	}
	n := len(pieces)
	end := pieces[anchor].pts[len(pieces[anchor].pts)-1]
	for k := 1; k < n && !allSeams; k++ {
		pc := &pieces[(anchor+k)%n]
		st := pc.pts[0]
		du, dv := 0.0, 0.0
		switch {
		case pc.seam == 1:
			for i := range pc.pts {
				pc.pts[i].u = end.u
			}
			if end.singU || st.singU {
				// Back down the seam on its other side, round the pole.
				du = s.acrossPole(end, pc.pts[0], sense)
			}
			dv = s.shift(end.v, st.v, s.pv)
		case pc.seam == 2:
			for i := range pc.pts {
				pc.pts[i].v = end.v
			}
			du = s.shift(end.u, st.u, s.pu)
		case s.pu > 0 && (end.singU || st.singU):
			// Across a pole the loop runs along the collapsed side, the
			// way that keeps the face on its left.
			du = s.acrossPole(end, st, sense)
			dv = s.shift(end.v, st.v, s.pv)
		default:
			if pc.seam != 1 {
				du = s.shift(end.u, st.u, s.pu)
			}
			if pc.seam != 2 {
				dv = s.shift(end.v, st.v, s.pv)
			}
		}
		for i := range pc.pts {
			pc.pts[i].u += du
			pc.pts[i].v += dv
		}
		end = pc.pts[len(pc.pts)-1]
	}
	// Join the pieces, dropping a joint's second copy.
	var c contour
	for k := 0; k < n; k++ {
		pc := pieces[(anchor+k)%n]
		for i, q := range pc.pts {
			if i == 0 && len(c.pts) > 0 && sameJoint(c.pts[len(c.pts)-1], q) {
				continue
			}
			c.pts = append(c.pts, q)
		}
	}
	if len(c.pts) < 2 {
		return c, nil
	}
	head, tail := c.pts[0], c.pts[len(c.pts)-1]
	if s.pu > 0 && !head.singU && !tail.singU {
		c.wrap[0] = int(math.Round((tail.u - head.u) / s.pu))
	}
	if s.pv > 0 {
		c.wrap[1] = int(math.Round((tail.v - head.v) / s.pv))
	}
	if c.wrap == [2]int{} && sameJoint(head, tail) {
		c.pts = c.pts[:len(c.pts)-1]
	} else if c.wrap != [2]int{} && head.p.Equals(tail.p) {
		// The end is the start one period on: kept implicit.
		c.pts = c.pts[:len(c.pts)-1]
	}
	return c, nil
}

// acrossPole is the shift, in whole periods of u, that puts the start
// of the next edge st on the side of the previous edge's end that keeps
// the face on the left: at the top pole (the larger v) the loop turns
// towards smaller u, at the bottom towards larger u, with the face's
// normal along Su x Sv; the other way round when it is reversed.
func (s *faceSurface) acrossPole(end, st uvPoint, sense bool) float64 {
	top := true
	vmax, okMax := s.Pole(poleVMax)
	vmin, okMin := s.Pole(poleVMin)
	switch {
	case okMax && okMin:
		top = math.Abs(end.v-vmax) < math.Abs(end.v-vmin)
	case okMin:
		top = false
	case !okMax:
		return s.shift(end.u, st.u, s.pu)
	}
	// The next start goes below end.u, in [end.u - pu, end.u), or
	// above, in (end.u, end.u + pu]: never on end.u, which would be
	// back down the same line.
	d := (end.u - st.u) / s.pu
	if top == sense {
		return (math.Ceil(d) - 1) * s.pu
	}
	return (math.Floor(d) + 1) * s.pu
}

// sameJoint tells whether the end of one edge and the start of the
// next are one point of the loop: the same vertex, unless the surface
// collapses there and the two have different parameters, as at a
// pole, where the loop runs along the collapsed side.
func sameJoint(a, b uvPoint) bool {
	if !a.p.EqualsExactly(b.p) {
		return false
	}
	if (a.singU || b.singU) && a.u != b.u {
		return false
	}
	if (a.singV || b.singV) && a.v != b.v {
		return false
	}
	return true
}

// shift is the whole number of periods to add to b to bring it nearest
// a.
func (s *faceSurface) shift(a, b, period float64) float64 {
	if period <= 0 {
		return 0
	}
	return math.Round((a-b)/period) * period
}

// seamOf tells whether an edge used twice by the face lies where the
// surface closes in u (1) or in v (2), or neither (0).
func (s *faceSurface) seamOf(pts []uvPoint) int {
	on := func(x, lo, period float64) bool {
		d := math.Mod(x-lo, period)
		if d < 0 {
			d += period
		}
		return d < 1e-6*period || period-d < 1e-6*period
	}
	check := func(get func(q uvPoint) (float64, bool), lo, period float64) bool {
		if period <= 0 {
			return false
		}
		any := false
		for _, q := range pts {
			x, sing := get(q)
			if sing {
				continue
			}
			if !on(x, lo, period) {
				return false
			}
			any = true
		}
		return any
	}
	if check(func(q uvPoint) (float64, bool) { return q.u, q.singU }, s.u0, s.pu) {
		return 1
	}
	if check(func(q uvPoint) (float64, bool) { return q.v, q.singV }, s.v0, s.pv) {
		return 2
	}
	return 0
}

// placeSeam puts a seam of a loop made only of seams on its side of
// the domain: going up in v, a u seam is the domain's right side;
// going down, its left; a v seam going along u is its bottom, going
// back its top.
func (s *faceSurface) placeSeam(pc *piece) {
	a, z := pc.pts[0], pc.pts[len(pc.pts)-1]
	switch pc.seam {
	case 1:
		u := s.u0
		if z.v > a.v {
			u = s.u0 + s.pu
		}
		lo := math.Inf(1)
		for i := range pc.pts {
			pc.pts[i].u = u
			lo = math.Min(lo, pc.pts[i].v)
		}
		dv := s.alongDomain(lo, s.v0, s.pv)
		for i := range pc.pts {
			pc.pts[i].v += dv
		}
	case 2:
		v := s.v0 + s.pv
		if z.u > a.u {
			v = s.v0
		}
		lo := math.Inf(1)
		for i := range pc.pts {
			pc.pts[i].v = v
			lo = math.Min(lo, pc.pts[i].u)
		}
		du := s.alongDomain(lo, s.u0, s.pu)
		for i := range pc.pts {
			pc.pts[i].u += du
		}
	}
}

// alongDomain is the whole number of periods that brings lo, the least
// parameter of a seam, into the domain's first period.
func (s *faceSurface) alongDomain(lo, start, period float64) float64 {
	if period <= 0 {
		return 0
	}
	return -math.Floor((lo-start)/period+1e-9) * period
}

// unwrapPiece makes a piece's parameters continuous along it where
// they repeat, and gives a point where one is undefined its
// neighbour's.
func (s *faceSurface) unwrapPiece(pc *piece) {
	pts := pc.pts
	fill := func(get func(q *uvPoint) (*float64, bool)) {
		known := -1
		for i := range pts {
			if _, sing := get(&pts[i]); !sing {
				known = i
				break
			}
		}
		if known < 0 {
			return
		}
		for i := range pts {
			x, sing := get(&pts[i])
			if !sing {
				known = i
				continue
			}
			k, _ := get(&pts[known])
			*x = *k
		}
	}
	fill(func(q *uvPoint) (*float64, bool) { return &q.u, q.singU })
	fill(func(q *uvPoint) (*float64, bool) { return &q.v, q.singV })
	for i := 1; i < len(pts); i++ {
		pts[i].u += s.shift(pts[i-1].u, pts[i].u, s.pu)
		pts[i].v += s.shift(pts[i-1].v, pts[i].v, s.pv)
	}
}

// whole is the edge of the whole domain, for a face with no bounds on
// a closed surface such as a sphere.
func (s *faceSurface) whole() (contour, error) {
	if math.IsInf(s.u0, 0) || math.IsInf(s.u1, 0) || math.IsInf(s.v0, 0) || math.IsInf(s.v1, 0) {
		return contour{}, fmt.Errorf("it has no bounds on an unbounded surface")
	}
	var c contour
	side := func(ua, va, ub, vb float64) {
		const n = 16
		a, b := s.Eval(ua, va), s.Eval(ub, vb)
		mid := s.Eval((ua+ub)/2, (va+vb)/2)
		if a.Minus(b).Magnitude() < 1e-9*s.scale && a.Minus(mid).Magnitude() < 1e-9*s.scale {
			// A side collapsed to a pole: one point, at both ends.
			c.pts = append(c.pts, uvPoint{p: a, u: ua, v: va, singU: va == vb, singV: ua == ub})
			return
		}
		for i := 0; i < n; i++ {
			u := ua + (ub-ua)*float64(i)/n
			v := va + (vb-va)*float64(i)/n
			c.pts = append(c.pts, uvPoint{p: s.Eval(u, v), u: u, v: v})
		}
	}
	side(s.u0, s.v0, s.u1, s.v0)
	side(s.u1, s.v0, s.u1, s.v1)
	side(s.u1, s.v1, s.u0, s.v1)
	side(s.u0, s.v1, s.u0, s.v0)
	// Where the surface closes, the far side is the near side.
	for i := range c.pts {
		q := &c.pts[i]
		u, v := q.u, q.v
		if s.pu > 0 && u == s.u1 {
			u = s.u0
		}
		if s.pv > 0 && v == s.v1 {
			v = s.v0
		}
		if u != q.u || v != q.v {
			q.p = s.Eval(u, v)
		}
	}
	// The point where a collapsed side ends is the pole too.
	for i := range c.pts {
		j := (i + 1) % len(c.pts)
		if c.pts[i].p.Minus(c.pts[j].p).Magnitude() < 1e-9*s.scale {
			c.pts[j].p = c.pts[i].p
		}
	}
	return c, nil
}

// closeWraps turns loops that go round a closed surface into regions:
// two that go round in opposite senses bound a band, joined by a cut;
// one alone closes at the pole its interior side reaches.
func (s *faceSurface) closeWraps(cs []contour, sense bool) ([]contour, error) {
	var out []contour
	var wraps []contour
	for _, c := range cs {
		if c.wrap == [2]int{} {
			out = append(out, c)
		} else {
			wraps = append(wraps, c)
		}
	}
	switch len(wraps) {
	case 0:
	case 1:
		c, err := s.closeAtPole(wraps[0], sense)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	case 2:
		a, b := wraps[0], wraps[1]
		if a.wrap[0]+b.wrap[0] != 0 || a.wrap[1]+b.wrap[1] != 0 ||
			(a.wrap[0] != 0) == (a.wrap[1] != 0) || abs(a.wrap[0]+a.wrap[1]) != 1 {
			return nil, errTrim
		}
		out = append(out, s.bridge(a, b))
	default:
		return nil, errTrim
	}
	s.align(out)
	return out, nil
}

// align moves each loop by whole periods to lie within the loop that
// spans most, as the holes of a face on a cylinder must lie within its
// outer loop whatever period their points were found in.
func (s *faceSurface) align(cs []contour) {
	if len(cs) < 2 || s.pu == 0 && s.pv == 0 {
		return
	}
	type box struct{ u0, u1, v0, v1 float64 }
	boxOf := func(c contour) box {
		b := box{math.Inf(1), math.Inf(-1), math.Inf(1), math.Inf(-1)}
		for _, q := range c.pts {
			b.u0, b.u1 = math.Min(b.u0, q.u), math.Max(b.u1, q.u)
			b.v0, b.v1 = math.Min(b.v0, q.v), math.Max(b.v1, q.v)
		}
		return b
	}
	ref := 0
	refBox := boxOf(cs[0])
	for i := 1; i < len(cs); i++ {
		b := boxOf(cs[i])
		if (b.u1-b.u0)+(b.v1-b.v0) > (refBox.u1-refBox.u0)+(refBox.v1-refBox.v0) {
			ref, refBox = i, b
		}
	}
	for i := range cs {
		if i == ref {
			continue
		}
		b := boxOf(cs[i])
		du := s.shift((refBox.u0+refBox.u1)/2, (b.u0+b.u1)/2, s.pu)
		dv := s.shift((refBox.v0+refBox.v1)/2, (b.v0+b.v1)/2, s.pv)
		for k := range cs[i].pts {
			cs[i].pts[k].u += du
			cs[i].pts[k].v += dv
		}
	}
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// bridge joins two loops that go round a closed surface in opposite
// senses into one contour, along a cut from a's start to the nearest
// point of b and back one period on.
func (s *faceSurface) bridge(a, b contour) contour {
	dir := 0 // which parameter the loops go round
	period := s.pu
	if a.wrap[1] != 0 {
		dir, period = 1, s.pv
	}
	get := func(q uvPoint) float64 {
		if dir == 0 {
			return q.u
		}
		return q.v
	}
	other := func(q uvPoint) float64 {
		if dir == 0 {
			return q.v
		}
		return q.u
	}
	move := func(q uvPoint, d float64) uvPoint {
		if dir == 0 {
			q.u += d
		} else {
			q.v += d
		}
		return q
	}
	sa := float64(a.wrap[dir]) * period // a's travel, plus or minus a period
	a0 := a.pts[0]
	// The point of b nearest a's start one period on.
	target := get(a0) + sa
	best, bestD := 0, math.Inf(1)
	for j, q := range b.pts {
		d := math.Abs(get(q) + s.shift(target, get(q), period) - target)
		d = math.Hypot(d, other(q)-other(a0))
		if d < bestD {
			best, bestD = j, d
		}
	}
	bj := b.pts[best]
	shiftB := s.shift(target, get(bj), period)
	var c contour
	c.pts = append(c.pts, a.pts...)
	c.pts = append(c.pts, move(a0, sa))
	// b from best round to best again, its travel being minus a's.
	sb := float64(b.wrap[dir]) * period
	for k := 0; k < len(b.pts); k++ {
		j := best + k
		wraps := 0.0
		if j >= len(b.pts) {
			j -= len(b.pts)
			wraps = sb
		}
		c.pts = append(c.pts, move(b.pts[j], shiftB+wraps))
	}
	c.pts = append(c.pts, move(bj, shiftB+sb))
	return c
}

// closeAtPole closes a loop that goes once round a closed surface
// through the side of the domain its interior is on, which should be a
// pole; where it is not, the domain's edge closes it.
func (s *faceSurface) closeAtPole(c contour, sense bool) (contour, error) {
	dir := 0
	if c.wrap[1] != 0 {
		dir = 1
	}
	if abs(c.wrap[dir]) != 1 || c.wrap[1-dir] != 0 {
		return contour{}, errTrim
	}
	travel := float64(c.wrap[dir])
	// The interior is on the left, looking along the face's normal.
	left := travel > 0
	if !sense {
		left = !left
	}
	var side int
	if dir == 0 {
		// Going along +u, left is +v.
		side = poleVMin
		if left {
			side = poleVMax
		}
	} else {
		// Going along +v, left is -u.
		side = poleUMax
		if left {
			side = poleUMin
		}
	}
	at, ok := s.Pole(side)
	period := s.pu
	if dir == 1 {
		period = s.pv
	}
	a0 := c.pts[0]
	endPt := a0
	if dir == 0 {
		endPt.u += travel * period
	} else {
		endPt.v += travel * period
	}
	c.pts = append(c.pts, endPt)
	if !ok {
		// No pole: close straight across the parameters, back to the
		// start, as OpenCASCADE closes such a loop.
		const n = 32
		for i := 1; i < n; i++ {
			q := uvPoint{u: endPt.u + (a0.u-endPt.u)*float64(i)/n, v: endPt.v + (a0.v-endPt.v)*float64(i)/n}
			q.p = s.Eval(q.u, q.v)
			c.pts = append(c.pts, q)
		}
		c.wrap = [2]int{}
		return c, nil
	}
	// Along the pole, which is one point, back over the start.
	for _, t := range []float64{endPt.u, a0.u} {
		q := uvPoint{u: t, v: at, singU: true}
		if dir == 1 {
			q = uvPoint{u: at, v: endPt.v, singV: true}
			if t == a0.u {
				q.v = a0.v
			}
		}
		q.p = s.Eval(q.u, q.v)
		c.pts = append(c.pts, q)
	}
	c.pts[len(c.pts)-1].p = c.pts[len(c.pts)-2].p
	c.wrap = [2]int{}
	return c, nil
}

// meshTri is a triangle of the tessellation with a normal at each
// corner.
type meshTri struct {
	p [3]geom.Vector
	n [3]geom.Vector
}

// triangulate fills the contours, in parameter space with each
// parameter scaled to about a millimetre, adding a grid of points
// inside as dense as the surface's curvature needs.
func (s *faceSurface) triangulate(cs []contour, tol float64, sense bool) ([]meshTri, error) {
	umin, umax := math.Inf(1), math.Inf(-1)
	vmin, vmax := math.Inf(1), math.Inf(-1)
	for _, c := range cs {
		for _, q := range c.pts {
			umin, umax = math.Min(umin, q.u), math.Max(umax, q.u)
			vmin, vmax = math.Min(vmin, q.v), math.Max(vmax, q.v)
		}
	}
	if !(umax > umin) || !(vmax > vmin) {
		return nil, nil
	}
	// The metric and curvature, sampled over the region.
	const ns = 6
	var lu, lv, kuu, kvv, kuv float64
	nlu, nlv := 0, 0
	hu, hv := (umax-umin)*1e-3, (vmax-vmin)*1e-3
	for i := 0; i <= ns; i++ {
		for j := 0; j <= ns; j++ {
			u := umin + (umax-umin)*(float64(i)+0.5)/(ns+1)
			v := vmin + (vmax-vmin)*(float64(j)+0.5)/(ns+1)
			p, su, sv := s.Derivs(u, v)
			if m := su.Magnitude(); m > 1e-9*s.scale {
				lu += m
				nlu++
			}
			if m := sv.Magnitude(); m > 1e-9*s.scale {
				lv += m
				nlv++
			}
			suu := s.Eval(u+hu, v).Plus(s.Eval(u-hu, v)).Minus(p.ScaledBy(2)).ScaledBy(1 / (hu * hu))
			svv := s.Eval(u, v+hv).Plus(s.Eval(u, v-hv)).Minus(p.ScaledBy(2)).ScaledBy(1 / (hv * hv))
			suv := s.Eval(u+hu, v+hv).Minus(s.Eval(u+hu, v-hv)).Minus(s.Eval(u-hu, v+hv)).
				Plus(s.Eval(u-hu, v-hv)).ScaledBy(1 / (4 * hu * hv))
			// Only the part along the normal bends the surface away from
			// a flat triangle.
			n := su.Cross(sv)
			if m := n.Magnitude(); m > 0 {
				n = n.ScaledBy(1 / m)
				kuu = math.Max(kuu, math.Abs(suu.Dot(n)))
				kvv = math.Max(kvv, math.Abs(svv.Dot(n)))
				kuv = math.Max(kuv, math.Abs(suv.Dot(n)))
			}
		}
	}
	if nlu == 0 || nlv == 0 {
		return nil, errTrim
	}
	lu /= float64(nlu)
	lv /= float64(nlv)
	// Scaled parameters (x, y) = (u lu, v lv) are about millimetres.
	kxx, kyy, kxy := kuu/(lu*lu), kvv/(lv*lv), kuv/(lu*lv)
	spacing := func(k float64) float64 {
		if k <= 1e-12 {
			return math.Inf(1)
		}
		return math.Sqrt(8 * tol / k)
	}
	hx, hy := spacing(kxx+kxy), spacing(kyy+kxy)
	// Not too long and thin, so the triangles between the grid and the
	// edges stay near the surface too.
	const aspect = 6
	if hx < math.Inf(1) || hy < math.Inf(1) {
		hx = math.Min(hx, aspect*hy)
		hy = math.Min(hy, aspect*hx)
	}
	ch := s.chartFor(cs, umin, vmin, lu, lv)
	var pts []cdt.Point
	var segs [][2]int
	var bnd []uvPoint
	for _, c := range cs {
		base := len(pts)
		for _, q := range c.pts {
			pts = append(pts, ch.to(q.u, q.v))
			bnd = append(bnd, q)
		}
		for i := range c.pts {
			segs = append(segs, [2]int{base + i, base + (i+1)%len(c.pts)})
		}
	}
	nb := len(pts)
	x0, x1 := math.Inf(1), math.Inf(-1)
	y0, y1 := math.Inf(1), math.Inf(-1)
	for _, p := range pts {
		x0, x1 = math.Min(x0, p.X), math.Max(x1, p.X)
		y0, y1 = math.Min(y0, p.Y), math.Max(y1, p.Y)
	}
	if ch.polar {
		// The chart is about isotropic.
		hx = math.Min(hx, hy)
		hy = hx
	}
	w, h := x1-x0, y1-y0
	if hx < w/2 || hy < h/2 {
		hx, hy = math.Min(hx, w/2), math.Min(hy, h/2)
		if n := (w / hx) * (h / hy); n > maxGridPoints {
			f := math.Sqrt(n / maxGridPoints)
			hx, hy = hx*f, hy*f
		}
		pts = append(pts, gridPoints(pts[:nb], segs, x0, y0, w, h, hx, hy)...)
	}
	tris, err := cdt.Triangulate(pts, segs)
	if err != nil && !errors.Is(err, cdt.ErrCrossing) {
		return nil, err
	}
	if ch.flip {
		for i := range tris {
			tris[i][1], tris[i][2] = tris[i][2], tris[i][1]
		}
	}
	at := func(i int) (geom.Vector, geom.Vector) {
		var u, v float64
		var p geom.Vector
		if i < nb {
			u, v, p = bnd[i].u, bnd[i].v, bnd[i].p
		} else {
			u, v = ch.from(pts[i])
			p = s.Eval(u, v)
		}
		_, su, sv := s.Derivs(u, v)
		n := su.Cross(sv)
		if m := n.Magnitude(); m > 1e-12*s.scale*s.scale {
			n = n.ScaledBy(1 / m)
		} else {
			n = geom.Vector{}
		}
		if !sense {
			n = n.ScaledBy(-1)
		}
		return p, n
	}
	cache := map[int][2]geom.Vector{}
	out := make([]meshTri, 0, len(tris))
	for _, t := range tris {
		var mt meshTri
		for k := 0; k < 3; k++ {
			pn, ok := cache[t[k]]
			if !ok {
				p, n := at(t[k])
				pn = [2]geom.Vector{p, n}
				cache[t[k]] = pn
			}
			mt.p[k], mt.n[k] = pn[0], pn[1]
		}
		if mt.p[0].EqualsExactly(mt.p[1]) || mt.p[1].EqualsExactly(mt.p[2]) || mt.p[2].EqualsExactly(mt.p[0]) {
			continue
		}
		if !sense {
			mt.p[1], mt.p[2] = mt.p[2], mt.p[1]
			mt.n[1], mt.n[2] = mt.n[2], mt.n[1]
		}
		fn := mt.p[1].Minus(mt.p[0]).Cross(mt.p[2].Minus(mt.p[0]))
		if m := fn.Magnitude(); m > 0 {
			fn = fn.ScaledBy(1 / m)
		}
		for k := 0; k < 3; k++ {
			if mt.n[k].MagSquared() == 0 {
				mt.n[k] = fn
			}
		}
		out = append(out, mt)
	}
	if err != nil {
		return out, errTrim
	}
	return out, nil
}

// gridPoints is the points of a staggered grid of spacing hx by hy
// over the box at (x0, y0), w by h, in a chart of the face, that lie
// inside the contours (segs between pts) and not near their edges.
func gridPoints(pts []cdt.Point, segIdx [][2]int, x0, y0, w, h, hx, hy float64) []cdt.Point {
	type seg struct{ a, b cdt.Point }
	segs := make([]seg, len(segIdx))
	for k, sg := range segIdx {
		a, b := pts[sg[0]], pts[sg[1]]
		segs[k] = seg{cdt.Point{X: a.X - x0, Y: a.Y - y0}, cdt.Point{X: b.X - x0, Y: b.Y - y0}}
	}
	keep := 0.45 * math.Min(hx, hy)
	// Buckets of segments, to find those near a point.
	cell := math.Max(hx, hy)
	nx, ny := int(w/cell)+1, int(h/cell)+1
	buckets := make(map[int][]int)
	for k, sg := range segs {
		xa := int(math.Floor((math.Min(sg.a.X, sg.b.X) - keep) / cell))
		xb := int(math.Floor((math.Max(sg.a.X, sg.b.X) + keep) / cell))
		ya := int(math.Floor((math.Min(sg.a.Y, sg.b.Y) - keep) / cell))
		yb := int(math.Floor((math.Max(sg.a.Y, sg.b.Y) + keep) / cell))
		for x := max(xa, 0); x <= min(xb, nx); x++ {
			for y := max(ya, 0); y <= min(yb, ny); y++ {
				buckets[y*(nx+1)+x] = append(buckets[y*(nx+1)+x], k)
			}
		}
	}
	var out []cdt.Point
	rows := int(h / hy)
	for j := 0; j <= rows; j++ {
		y := (float64(j) + 0.5) * h / float64(rows+1)
		// Where the row crosses the contours, for the even-odd test.
		var xs []float64
		for _, sg := range segs {
			if (sg.a.Y > y) != (sg.b.Y > y) {
				xs = append(xs, sg.a.X+(y-sg.a.Y)*(sg.b.X-sg.a.X)/(sg.b.Y-sg.a.Y))
			}
		}
		sort.Float64s(xs)
		cols := int(w / hx)
		off := 0.5
		if j%2 == 1 {
			off = 0
		}
		for i := 0; i <= cols; i++ {
			x := (float64(i) + off) * w / float64(cols+1)
			if x <= 0 || x >= w {
				continue
			}
			if sort.SearchFloat64s(xs, x)%2 == 0 {
				continue
			}
			p := cdt.Point{X: x, Y: y}
			near := false
			for _, k := range buckets[int(y/cell)*(nx+1)+int(x/cell)] {
				if segDist(p, segs[k].a, segs[k].b) < keep {
					near = true
					break
				}
			}
			if !near {
				out = append(out, cdt.Point{X: x + x0, Y: y + y0})
			}
		}
	}
	return out
}

// chart maps a face's parameters to a plane where distances are about
// those on the surface, for triangulating.
type chart struct {
	to   func(u, v float64) cdt.Point
	from func(p cdt.Point) (u, v float64)
	// flip is set when the chart turns the parameters' orientation
	// over.
	flip  bool
	polar bool
}

// chartFor is the parameters scaled to about millimetres, or, for a
// face that reaches one pole of a surface closed in u, a polar chart
// about the pole: there the parameters put points that are near
// together far apart and turn the order of points round the pole, so
// triangles made in them overlap on the surface.
func (s *faceSurface) chartFor(cs []contour, umin, vmin, lu, lv float64) chart {
	plain := chart{
		to: func(u, v float64) cdt.Point { return cdt.Point{X: (u - umin) * lu, Y: (v - vmin) * lv} },
		from: func(p cdt.Point) (u, v float64) {
			return p.X/lu + umin, p.Y/lv + vmin
		},
	}
	if s.pu <= 0 {
		return plain
	}
	lo, okLo := s.Pole(poleVMin)
	hi, okHi := s.Pole(poleVMax)
	touchLo, touchHi := false, false
	uref, vmid := math.Inf(1), 0.0
	n := 0
	for _, c := range cs {
		for _, q := range c.pts {
			if q.singU {
				if okLo && math.Abs(q.v-lo) < math.Abs(q.v-hi) || !okHi {
					touchLo = touchLo || okLo
				} else {
					touchHi = true
				}
				continue
			}
			uref = math.Min(uref, q.u)
			vmid += q.v
			n++
		}
	}
	if touchLo == touchHi || n == 0 || math.IsInf(uref, 0) {
		return plain
	}
	vmid /= float64(n)
	vp := lo
	if touchHi {
		vp = hi
	}
	side := 1.0 // v grows away from the pole
	if vmid < vp {
		side = -1
	}
	// How much of a full turn the surface turns about the pole: one for
	// a sphere, sin of the half angle for a cone, whose chart is then
	// its development.
	d := 1e-4 * math.Max(math.Abs(vmid-vp), 1e-6)
	k, kn := 0.0, 0
	for i := 0; i < 8; i++ {
		u := uref + s.pu*float64(i)/8
		_, su, sv := s.Derivs(u, vp+side*d)
		if m := sv.Magnitude(); m > 0 {
			k += su.Magnitude() / (m * d)
			kn++
		}
	}
	if kn == 0 {
		return plain
	}
	k = math.Max(0.02, math.Min(0.999, k/float64(kn)))
	turn := 2 * math.Pi * k / s.pu
	ch := chart{polar: true, flip: side > 0}
	ch.to = func(u, v float64) cdt.Point {
		r := side * (v - vp) * lv
		if r <= 0 {
			return cdt.Point{}
		}
		t := (u - uref) * turn
		return cdt.Point{X: r * math.Cos(t), Y: r * math.Sin(t)}
	}
	ch.from = func(p cdt.Point) (u, v float64) {
		t := math.Atan2(p.Y, p.X)
		if t < 0 {
			t += 2 * math.Pi
		}
		return uref + t/turn, vp + side*math.Hypot(p.X, p.Y)/lv
	}
	return ch
}

func segDist(p, a, b cdt.Point) float64 {
	dx, dy := b.X-a.X, b.Y-a.Y
	l := dx*dx + dy*dy
	t := 0.0
	if l > 0 {
		t = math.Max(0, math.Min(1, ((p.X-a.X)*dx+(p.Y-a.Y)*dy)/l))
	}
	return math.Hypot(p.X-a.X-t*dx, p.Y-a.Y-t*dy)
}
