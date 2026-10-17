package srf

import (
	"errors"
	"math"
	"slices"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/internal/cdt"
)

// The constrained Delaunay mesher (roadmap K4). The trim's points, in
// uv, are the mesh's boundary as they are, so the faces on either side
// of a trim share its vertices; inside, points come from a quadtree
// over the region, its cells split where the surface sags from them
// (twist across a cell's diagonals included) and, near the trim, until
// they are about as small as the trim's segments there; the points are
// triangulated Delaunay with the trim's segments kept as edges, in a
// chart where u and v are scaled to about millimetres, so the triangles
// are well shaped on the surface and not only in uv; and triangles that
// still stray from the surface get a point inserted, over a few rounds.

// errCDTMesh is why cdtTriangulateInto gave the region back to the ear
// clipper.
var errCDTMesh = errors.New("cdt mesh")

// cdtTriangulateInto triangulates the uv polygon p of srf, adding the
// triangles, in uv and turned as UvTriangulateInto turns them, to mesh.
// On an error nothing is added.
func (p *SPolygon) cdtTriangulateInto(mesh *SMesh, srf *SSurface) error {
	var bnd []geom.Vector
	var segs [][2]int
	for _, c := range p.Contours {
		pts := make([]geom.Vector, 0, len(c.Points))
		// Two curves' ends at a corner may differ in uv by a rounding:
		// as distinct points they would make a segment the length of
		// the rounding, which the triangulation can't keep. They are
		// one point if they are on the surface; in uv, LengthEps is a
		// fifth of a micrometre on a face a few hundred millimetres
		// across, and points of a short trim lie that close.
		same := func(a, b geom.Vector) bool {
			return a.EqualsExactly(b) || srf.PointAt(a.X, a.Y).Equals(srf.PointAt(b.X, b.Y)) && a.Minus(b).Magnitude() < 1e-6
		}
		for _, sp := range c.Points {
			if len(pts) > 0 && same(pts[len(pts)-1], sp.P) {
				continue
			}
			pts = append(pts, sp.P)
		}
		for len(pts) > 1 && same(pts[0], pts[len(pts)-1]) {
			pts = pts[:len(pts)-1]
		}
		if len(pts) < 3 {
			continue
		}
		base := len(bnd)
		bnd = append(bnd, pts...)
		for i := range pts {
			segs = append(segs, [2]int{base + i, base + (i+1)%len(pts)})
		}
	}
	if len(segs) < 3 {
		return errCDTMesh
	}
	ch := newUVChart(srf, bnd)
	if ch == nil {
		return errCDTMesh
	}
	pts := make([]cdt.Point, len(bnd))
	for i, q := range bnd {
		pts[i] = ch.to(q)
	}
	// A trim along a folded side is one point in a polar chart.
	kept := segs[:0]
	for _, sg := range segs {
		if pts[sg[0]] != pts[sg[1]] {
			kept = append(kept, sg)
		}
	}
	segs = kept
	if len(segs) < 3 {
		return errCDTMesh
	}
	r := newCDTRegion(pts, segs)
	if r.overlaps() {
		return errCDTMesh
	}
	tol := tolOf(srf).ChordTolMm()
	r.sags(srf, bnd, tol)
	// No cell or triangle is made smaller than a 2 MaxSegments'th of
	// the patch's parameters, about the old grid's finest, however
	// fine the tolerance: a twisted face at a tolerance of a micrometre
	// would otherwise take minutes.
	r.minX, r.minY = ch.minCell(tolOf(srf).MaxSegments())
	// A plane needs no points inside: the boundary's own triangulation
	// lies on it.
	var inner []cdt.Point
	if !srf.flat() {
		inner = r.quadPoints(srf, ch, tol)
	}
	all := append(slices.Clip(pts), inner...)
	var tris [][3]int
	var err error
	for round := range cdtRefineRounds + 1 {
		tris, err = cdt.Triangulate(all, segs)
		if err != nil {
			return err
		}
		if round == cdtRefineRounds || len(all) > cdtMaxPoints || srf.flat() {
			break
		}
		more := r.refinePoints(srf, ch, all, tris, tol)
		if len(more) == 0 {
			break
		}
		all = append(all, more...)
	}
	uv := func(i int) geom.Vector {
		if i < len(bnd) {
			return bnd[i]
		}
		return ch.from(all[i])
	}
	for _, t := range tris {
		// The ear clipper's triangles turn clockwise in uv; the caller
		// turns them over.
		if ch.flip {
			mesh.AddTriangle(STriangle{A: uv(t[0]), B: uv(t[1]), C: uv(t[2])})
		} else {
			mesh.AddTriangle(STriangle{A: uv(t[0]), B: uv(t[2]), C: uv(t[1])})
		}
	}
	return nil
}

// cdtRefineRounds is how many times cdtTriangulateInto inserts points
// where its triangles stray from the surface and triangulates again.
const cdtRefineRounds = 6

// cdtMaxPoints bounds the points of one region's mesh.
const cdtMaxPoints = 200000

// cdtMaxDepth bounds how many times a quadtree cell is halved.
const cdtMaxDepth = 12

// uvChart maps a surface's parameters to a plane where lengths are
// about those on the surface: u and v scaled by the surface's mean
// speed along each over the region; or, on a patch with a side folded
// to a pole, polar coordinates about the pole, the distance from the
// folded side as the radius and the parameter along it as the angle,
// so that the whole side is one point. In plain parameters the side is
// a line, and an edge from a point on it to the trim is a chord across
// the patch in uv that lies, on a cone, along the ruling the trim
// follows: a sliver over the trim.
type uvChart struct {
	lu, lv float64
	// pole is the folded side (0 to 3, u = 0, u = 1, v = 0, v = 1), or
	// -1 for plain parameters; alpha is the angle the patch turns
	// through about the pole, and ls its speed away from it.
	pole      int
	alpha, ls float64
	// flip is whether the chart turns the parameters over.
	flip bool
}

func newUVChart(srf *SSurface, bnd []geom.Vector) *uvChart {
	u0, u1, v0, v1 := math.Inf(1), math.Inf(-1), math.Inf(1), math.Inf(-1)
	for _, q := range bnd {
		u0, u1 = math.Min(u0, q.X), math.Max(u1, q.X)
		v0, v1 = math.Min(v0, q.Y), math.Max(v1, q.Y)
	}
	const ns = 6
	var lu, lv float64
	nu, nv := 0, 0
	for i := range ns + 1 {
		for j := range ns + 1 {
			u := u0 + (u1-u0)*(float64(i)+0.5)/(ns+1)
			v := v0 + (v1-v0)*(float64(j)+0.5)/(ns+1)
			var tu, tv geom.Vector
			srf.TangentsAt(u, v, &tu, &tv, false)
			if m := tu.Magnitude(); m > lengthEps {
				lu += m
				nu++
			}
			if m := tv.Magnitude(); m > lengthEps {
				lv += m
				nv++
			}
		}
	}
	if nu == 0 || nv == 0 {
		return nil
	}
	c := &uvChart{lu: lu / float64(nu), lv: lv / float64(nv), pole: -1}
	if !srf.flat() {
		var folds []int
		for side := range 4 {
			if srf.foldsSide(side) {
				folds = append(folds, side)
			}
		}
		if len(folds) == 1 {
			c.polarAbout(srf, folds[0])
		}
	}
	// The chart's turn against the parameters', at the region's middle.
	m := geom.Vector{X: (u0 + u1) / 2, Y: (v0 + v1) / 2}
	h := 1e-4
	a, b, d := c.to(m), c.to(m.Plus(geom.Vector{X: h})), c.to(m.Plus(geom.Vector{Y: h}))
	c.flip = (b.X-a.X)*(d.Y-a.Y)-(b.Y-a.Y)*(d.X-a.X) < 0
	return c
}

// polarAbout makes the chart polar about the folded side, unless the
// patch turns nearly all the way round it.
func (c *uvChart) polarAbout(srf *SSurface, side int) {
	c.pole = side
	const d = 0.01
	pp := c.surfaceAt(srf, 0, 0.5)
	p0, pm, p1 := c.surfaceAt(srf, d, 0), c.surfaceAt(srf, d, 0.5), c.surfaceAt(srf, d, 1)
	angle := func(a, b geom.Vector) float64 {
		a, b = a.Minus(pp), b.Minus(pp)
		return math.Atan2(a.Cross(b).Magnitude(), a.Dot(b))
	}
	c.alpha = angle(p0, pm) + angle(pm, p1)
	c.ls = pm.Minus(pp).Magnitude() / d
	if !(c.alpha > 1e-6 && c.alpha < 1.8*math.Pi && c.ls > lengthEps) {
		c.pole = -1
	}
}

// polar is q's distance s from the folded side and place t along it,
// both in [0, 1].
func (c *uvChart) polar(q geom.Vector) (s, t float64) {
	switch c.pole {
	case 0:
		return q.X, q.Y
	case 1:
		return 1 - q.X, q.Y
	case 2:
		return q.Y, q.X
	default:
		return 1 - q.Y, q.X
	}
}

// unpolar is polar's inverse.
func (c *uvChart) unpolar(s, t float64) geom.Vector {
	switch c.pole {
	case 0:
		return geom.Vector{X: s, Y: t}
	case 1:
		return geom.Vector{X: 1 - s, Y: t}
	case 2:
		return geom.Vector{X: t, Y: s}
	default:
		return geom.Vector{X: t, Y: 1 - s}
	}
}

// surfaceAt is the surface's point at s from the folded side and t
// along it.
func (c *uvChart) surfaceAt(srf *SSurface, s, t float64) geom.Vector {
	q := c.unpolar(s, t)
	return srf.PointAt(q.X, q.Y)
}

// minCell is the size in the chart, along x and y, of a 2 n'th of the
// patch's parameters.
func (c *uvChart) minCell(n int) (float64, float64) {
	n = max(n, 1)
	if c.pole < 0 {
		return c.lu / float64(2*n), c.lv / float64(2*n)
	}
	m := c.ls / float64(2*n)
	return m, m
}

func (c *uvChart) to(q geom.Vector) cdt.Point {
	if c.pole < 0 {
		return cdt.Point{X: q.X * c.lu, Y: q.Y * c.lv}
	}
	s, t := c.polar(q)
	if s <= 0 {
		return cdt.Point{}
	}
	r, a := s*c.ls, t*c.alpha
	return cdt.Point{X: r * math.Cos(a), Y: r * math.Sin(a)}
}

func (c *uvChart) from(p cdt.Point) geom.Vector {
	if c.pole < 0 {
		return geom.Vector{X: p.X / c.lu, Y: p.Y / c.lv}
	}
	a := math.Atan2(p.Y, p.X)
	if a < -0.1*(2*math.Pi-c.alpha) {
		a += 2 * math.Pi
	}
	s := math.Hypot(p.X, p.Y) / c.ls
	t := math.Max(0, math.Min(1, a/c.alpha))
	return c.unpolar(math.Min(s, 1), t)
}

func (c *uvChart) point(srf *SSurface, p cdt.Point) geom.Vector {
	q := c.from(p)
	return srf.PointAt(q.X, q.Y)
}

// cdtRegion is a region's boundary in a chart, with its segments
// bucketed by cell for finding those near a point.
type cdtRegion struct {
	pts    []cdt.Point
	segs   [][2]int
	x0, y0 float64
	w, h   float64
	cell   float64
	nx, ny int
	bucket map[int][]int
	// segLen is each segment's length.
	segLen []float64
	// xyz is the boundary's points on the surface, sag how far from
	// each segment's chord in xyz a point must be kept, and maxSag the
	// largest.
	xyz    []geom.Vector
	sag    []float64
	maxSag float64
	// minX and minY are the smallest a cell is cut to.
	minX, minY float64
}

func newCDTRegion(pts []cdt.Point, segs [][2]int) *cdtRegion {
	r := &cdtRegion{pts: pts, segs: segs, bucket: map[int][]int{}}
	x1, y1 := math.Inf(-1), math.Inf(-1)
	r.x0, r.y0 = math.Inf(1), math.Inf(1)
	for _, p := range pts {
		r.x0, x1 = math.Min(r.x0, p.X), math.Max(x1, p.X)
		r.y0, y1 = math.Min(r.y0, p.Y), math.Max(y1, p.Y)
	}
	r.w, r.h = x1-r.x0, y1-r.y0
	r.segLen = make([]float64, len(segs))
	total := 0.0
	for k, s := range segs {
		a, b := pts[s[0]], pts[s[1]]
		r.segLen[k] = math.Hypot(b.X-a.X, b.Y-a.Y)
		total += r.segLen[k]
	}
	// About one segment per bucket.
	r.cell = math.Max(total/float64(len(segs)), 1e-9*math.Max(r.w, r.h))
	r.nx, r.ny = int(r.w/r.cell)+1, int(r.h/r.cell)+1
	for int64(r.nx)*int64(r.ny) > 4*int64(len(segs))+64 {
		r.cell *= 2
		r.nx, r.ny = int(r.w/r.cell)+1, int(r.h/r.cell)+1
	}
	for k, s := range segs {
		a, b := pts[s[0]], pts[s[1]]
		for x := r.ix(math.Min(a.X, b.X)); x <= r.ix(math.Max(a.X, b.X)); x++ {
			for y := r.iy(math.Min(a.Y, b.Y)); y <= r.iy(math.Max(a.Y, b.Y)); y++ {
				r.bucket[y*r.nx+x] = append(r.bucket[y*r.nx+x], k)
			}
		}
	}
	return r
}

func (r *cdtRegion) ix(x float64) int { return min(max(int((x-r.x0)/r.cell), 0), r.nx-1) }
func (r *cdtRegion) iy(y float64) int { return min(max(int((y-r.y0)/r.cell), 0), r.ny-1) }

// near calls f with each segment that may come within d of the box
// (xa, ya)-(xb, yb), once each; f returns false to stop.
func (r *cdtRegion) near(xa, ya, xb, yb, d float64, f func(k int) bool) {
	seen := map[int]bool{}
	for x := r.ix(xa - d); x <= r.ix(xb+d); x++ {
		for y := r.iy(ya - d); y <= r.iy(yb+d); y++ {
			for _, k := range r.bucket[y*r.nx+x] {
				if seen[k] {
					continue
				}
				seen[k] = true
				if !f(k) {
					return
				}
			}
		}
	}
}

// clearance is the distance from p to the nearest segment, looking no
// farther than d (d if none is nearer).
func (r *cdtRegion) clearance(p cdt.Point, d float64) float64 {
	best := d
	r.near(p.X, p.Y, p.X, p.Y, d, func(k int) bool {
		a, b := r.pts[r.segs[k][0]], r.pts[r.segs[k][1]]
		best = math.Min(best, segDistCDT(p, a, b))
		return true
	})
	return best
}

// sags sets each segment's sag: the gap between its chord in xyz and
// the trim curve it stands for, where a point inside the chords in uv
// can lie, on the surface, beyond the chord, under the face beyond.
// The curve runs on the surface, so the gap is about how far the
// surface at the segment's middle in uv lies from the chord's middle;
// twice that is kept, and at least the chord tolerance. A trim along a
// ruling sags not at all however long it is.
func (r *cdtRegion) sags(srf *SSurface, bnd []geom.Vector, tol float64) {
	r.xyz = make([]geom.Vector, len(bnd))
	for i, q := range bnd {
		r.xyz[i] = srf.PointAt(q.X, q.Y)
	}
	r.sag = make([]float64, len(r.segs))
	for k, sg := range r.segs {
		m := bnd[sg[0]].Plus(bnd[sg[1]]).ScaledBy(0.5)
		off := srf.PointAt(m.X, m.Y).Minus(r.xyz[sg[0]].Plus(r.xyz[sg[1]]).ScaledBy(0.5)).Magnitude()
		r.sag[k] = math.Max(tol, 2*off)
		r.maxSag = math.Max(r.maxSag, r.sag[k])
	}
}

// clearOfChords reports whether the surface's point p (in the chart, P
// on the surface) is farther from each segment's chord in xyz than its
// sag. The segments looked at are those within four of the largest
// sags in the chart, whose lengths are about the surface's.
func (r *cdtRegion) clearOfChords(p cdt.Point, pp geom.Vector) bool {
	ok := true
	r.near(p.X, p.Y, p.X, p.Y, 4*r.maxSag, func(k int) bool {
		if distanceToSegment(pp, r.xyz[r.segs[k][0]], r.xyz[r.segs[k][1]]) < r.sag[k] {
			ok = false
		}
		return ok
	})
	return ok
}

// overlaps reports whether a boundary point lies inside a segment it
// is not an end of: the trim runs back over itself, a spike of no width
// (a seam's edge walked there and back). The even-odd rule cancels the
// spike, where the faces beyond keep its edges; the ear clipper keeps
// them, as ears of no area.
func (r *cdtRegion) overlaps() bool {
	eps := 1e-9 * math.Max(r.w, r.h)
	found := false
	for k, sg := range r.segs {
		a, b := r.pts[sg[0]], r.pts[sg[1]]
		r.near(math.Min(a.X, b.X), math.Min(a.Y, b.Y), math.Max(a.X, b.X), math.Max(a.Y, b.Y), eps, func(j int) bool {
			if j == k {
				return true
			}
			for _, i := range r.segs[j] {
				p := r.pts[i]
				if p == a || p == b {
					continue
				}
				if segDistCDT(p, a, b) < eps {
					found = true
					return false
				}
			}
			return true
		})
		if found {
			return true
		}
	}
	return false
}

// inside reports whether p is inside the region, by the even-odd rule
// along a ray in +x.
func (r *cdtRegion) inside(p cdt.Point) bool {
	in := false
	for _, s := range r.segs {
		a, b := r.pts[s[0]], r.pts[s[1]]
		if (a.Y > p.Y) != (b.Y > p.Y) {
			if x := a.X + (p.Y-a.Y)*(b.X-a.X)/(b.Y-a.Y); x > p.X {
				in = !in
			}
		}
	}
	return in
}

// quadPoints is the centres of the leaves of a quadtree over the region
// that are inside it and clear of its boundary by half their size. A
// cell is halved across x where the surface sags by more than tol from
// the chords of its sides along x, across y likewise, and both ways
// where it sags from both diagonals at its middle, a twist, though its
// sides are straight; and, where trim segments cross it, both ways
// while it is more than twice as long as the shortest of them. A
// cylinder's cells are so cut only round it, as a grid's are.
func (r *cdtRegion) quadPoints(srf *SSurface, ch *uvChart, tol float64) []cdt.Point {
	if r.w <= 0 || r.h <= 0 {
		return nil
	}
	var out []cdt.Point
	var split func(x0, y0, x1, y1 float64, depth int)
	split = func(x0, y0, x1, y1 float64, depth int) {
		shortest := math.Inf(1)
		r.near(x0, y0, x1, y1, 0, func(k int) bool {
			a, b := r.pts[r.segs[k][0]], r.pts[r.segs[k][1]]
			if segBoxCross(a, b, x0, y0, x1, y1) {
				shortest = math.Min(shortest, r.segLen[k])
			}
			return true
		})
		crossed := !math.IsInf(shortest, 1)
		mid := cdt.Point{X: (x0 + x1) / 2, Y: (y0 + y1) / 2}
		if !crossed && !r.inside(mid) {
			return
		}
		var cutX, cutY bool
		if depth < cdtMaxDepth && len(out) < cdtMaxPoints {
			sx, sy, sd := cellSag(srf, ch, x0, y0, x1, y1)
			cutX, cutY = sx > tol, sy > tol
			if !cutX && !cutY && sd > tol {
				cutX, cutY = true, true
			}
			if crossed && math.Max(x1-x0, y1-y0) > 2*shortest {
				cutX = cutX || x1-x0 > 2*shortest
				cutY = cutY || y1-y0 > 2*shortest
			}
			if cellTurns(srf, ch, x0, y0, x1, y1) {
				cutX, cutY = true, true
			}
			cutX = cutX && x1-x0 > 2*r.minX
			cutY = cutY && y1-y0 > 2*r.minY
		}
		switch {
		case cutX && cutY:
			split(x0, y0, mid.X, mid.Y, depth+1)
			split(mid.X, y0, x1, mid.Y, depth+1)
			split(x0, mid.Y, mid.X, y1, depth+1)
			split(mid.X, mid.Y, x1, y1, depth+1)
			return
		case cutX:
			split(x0, y0, mid.X, y1, depth+1)
			split(mid.X, y0, x1, y1, depth+1)
			return
		case cutY:
			split(x0, y0, x1, mid.Y, depth+1)
			split(x0, mid.Y, x1, y1, depth+1)
			return
		}
		size := math.Min(x1-x0, y1-y0)
		if d := r.clearance(mid, size/2); d < size/2 || !r.inside(mid) || !r.clearOfChords(mid, ch.point(srf, mid)) {
			return
		}
		out = append(out, mid)
	}
	// Root cells about square.
	side := math.Min(r.w, r.h)
	nx := min(max(int(math.Round(r.w/side)), 1), 64)
	ny := min(max(int(math.Round(r.h/side)), 1), 64)
	for i := range nx {
		for j := range ny {
			split(r.x0+r.w*float64(i)/float64(nx), r.y0+r.h*float64(j)/float64(ny),
				r.x0+r.w*float64(i+1)/float64(nx), r.y0+r.h*float64(j+1)/float64(ny), 0)
		}
	}
	return out
}

// cellTurns reports whether the surface's normal turns by more than the
// angular tolerance across the cell, between its corners and middle.
// It is false when srf has no angular tolerance.
func cellTurns(srf *SSurface, ch *uvChart, x0, y0, x1, y1 float64) bool {
	c, on := srf.Tol.angleCos()
	if !on {
		return false
	}
	var ns []geom.Vector
	for _, p := range [...]cdt.Point{{X: x0, Y: y0}, {X: x1, Y: y0}, {X: x1, Y: y1}, {X: x0, Y: y1}, {X: (x0 + x1) / 2, Y: (y0 + y1) / 2}} {
		q := ch.from(p)
		n := srf.NormalAt(q.X, q.Y)
		if m := n.Magnitude(); m >= lengthEps {
			ns = append(ns, n.ScaledBy(1/m))
		}
	}
	for i := range ns {
		for j := i + 1; j < len(ns); j++ {
			if ns[i].Dot(ns[j]) < c {
				return true
			}
		}
	}
	return false
}

// cellSag is how far the surface strays from the cell (x0, y0)-(x1, y1)
// of the chart meshed flat: from the chords of its sides along x, at
// their middles; from those along y; and at its middle from the middle
// of either diagonal, less what the sides' sags account for.
func cellSag(srf *SSurface, ch *uvChart, x0, y0, x1, y1 float64) (sx, sy, sd float64) {
	at := func(x, y float64) geom.Vector { return ch.point(srf, cdt.Point{X: x, Y: y}) }
	a, b, c, d := at(x0, y0), at(x1, y0), at(x1, y1), at(x0, y1)
	xm, ym := (x0+x1)/2, (y0+y1)/2
	m := at(xm, ym)
	mid := func(p, q geom.Vector) geom.Vector { return p.Plus(q).ScaledBy(0.5) }
	sx = max(at(xm, y0).Minus(mid(a, b)).Magnitude(), at(xm, y1).Minus(mid(c, d)).Magnitude(), at(xm, ym).Minus(mid(at(x0, ym), at(x1, ym))).Magnitude())
	sy = max(at(x0, ym).Minus(mid(d, a)).Magnitude(), at(x1, ym).Minus(mid(b, c)).Magnitude(), at(xm, ym).Minus(mid(at(xm, y0), at(xm, y1))).Magnitude())
	sd = max(m.Minus(mid(a, c)).Magnitude(), m.Minus(mid(b, d)).Magnitude()) - max(sx, sy)
	return sx, sy, sd
}

// refinePoints is the points to insert into the triangulation tris of
// pts where it strays from the surface by
// more than tol: a triangle's middle, where the surface leaves the
// triangle's own middle by more than tol, and the middle of an edge
// that is not the boundary's, where it leaves the edge's. A point
// nearer the boundary than a fifth of its triangle's shortest side is
// left out, so that no sliver is made against a trim segment, which
// can't be split: the face beyond it shares it.
func (r *cdtRegion) refinePoints(srf *SSurface, ch *uvChart, pts []cdt.Point, tris [][3]int, tol float64) []cdt.Point {
	xyz := make(map[int]geom.Vector, len(pts))
	at := func(i int) geom.Vector {
		if p, ok := xyz[i]; ok {
			return p
		}
		p := ch.point(srf, pts[i])
		xyz[i] = p
		return p
	}
	boundary := make(map[[2]int]bool, len(r.segs))
	for _, s := range r.segs {
		boundary[[2]int{min(s[0], s[1]), max(s[0], s[1])}] = true
	}
	done := map[[2]int]bool{}
	var out []cdt.Point
	add := func(p cdt.Point, shortest float64) {
		if d := r.clearance(p, shortest/5); d < shortest/5 || !r.inside(p) || !r.clearOfChords(p, ch.point(srf, p)) {
			return
		}
		out = append(out, p)
	}
	for _, t := range tris {
		var side [3]float64
		for k := range 3 {
			a, b := pts[t[k]], pts[t[(k+1)%3]]
			side[k] = math.Hypot(b.X-a.X, b.Y-a.Y)
		}
		shortest := min(side[0], side[1], side[2])
		if max(side[0], side[1], side[2]) < 2*math.Min(r.minX, r.minY) {
			continue
		}
		pa, pb, pc := at(t[0]), at(t[1]), at(t[2])
		cm := cdt.Point{X: (pts[t[0]].X + pts[t[1]].X + pts[t[2]].X) / 3, Y: (pts[t[0]].Y + pts[t[1]].Y + pts[t[2]].Y) / 3}
		if ch.point(srf, cm).Minus(pa.Plus(pb).Plus(pc).ScaledBy(1.0/3)).Magnitude() > tol {
			add(cm, shortest)
			continue
		}
		for k := range 3 {
			i, j := t[k], t[(k+1)%3]
			key := [2]int{min(i, j), max(i, j)}
			if boundary[key] || done[key] {
				continue
			}
			done[key] = true
			em := cdt.Point{X: (pts[i].X + pts[j].X) / 2, Y: (pts[i].Y + pts[j].Y) / 2}
			if ch.point(srf, em).Minus(at(i).Plus(at(j)).ScaledBy(0.5)).Magnitude() > tol {
				add(em, shortest)
			}
		}
	}
	return out
}

// segBoxCross reports whether the segment a-b meets the box.
func segBoxCross(a, b cdt.Point, x0, y0, x1, y1 float64) bool {
	// Clip the segment's parameter range to the box (Liang and Barsky).
	t0, t1 := 0.0, 1.0
	dx, dy := b.X-a.X, b.Y-a.Y
	for _, c := range [4][2]float64{{-dx, a.X - x0}, {dx, x1 - a.X}, {-dy, a.Y - y0}, {dy, y1 - a.Y}} {
		p, q := c[0], c[1]
		if p == 0 {
			if q < 0 {
				return false
			}
			continue
		}
		t := q / p
		if p < 0 {
			t0 = math.Max(t0, t)
		} else {
			t1 = math.Min(t1, t)
		}
		if t0 > t1 {
			return false
		}
	}
	return true
}

// segDistCDT is the distance from p to the segment a-b.
func segDistCDT(p, a, b cdt.Point) float64 {
	dx, dy := b.X-a.X, b.Y-a.Y
	l2 := dx*dx + dy*dy
	t := 0.0
	if l2 > 0 {
		t = math.Max(0, math.Min(1, ((p.X-a.X)*dx+(p.Y-a.Y)*dy)/l2))
	}
	return math.Hypot(p.X-a.X-t*dx, p.Y-a.Y-t*dy)
}
