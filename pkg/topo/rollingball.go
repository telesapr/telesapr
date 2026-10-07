package topo

import (
	"errors"
	"math"
	"slices"
	"sort"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// Rolling-ball fillets (roadmap 22.2): the blend along an edge between
// two faces of any kind, which step 10's exact construction doesn't
// cover: a cylinder meeting a cylinder at an angle, a spout on a pot,
// spheres, tori, B-spline faces. A ball of the fillet's radius rolls
// along the edge touching both faces; the fillet is the surface it
// sweeps between the two curves where it touches them.
//
// The ball's centre at a point of the edge is found where the two faces
// moved by the radius meet (each face's offset, as 22.1 makes it, but
// found point by point): from the tangent planes at the faces' nearest
// points, again until it stays. Its section, the arc between the two
// points it touches about its centre, lies on the surface the ball
// sweeps, whatever plane picks the ball, since each face's normal there
// is square to the way the centre moves. The fillet is the surface
// through the sections (srf.SkinThrough), sections added until it lies
// within a tenth of the chord tolerance of the faces and the ball.
//
// The faces are cut back along the curves the ball touches, which are
// the fillet's own edges: the strip between the edge and that curve
// goes, with the pieces of the seams between a face's patches that
// cross it, each seam cut where the curve crosses it, and the fillet's
// patches go in, one between each two sections.
//
// The edge is a chain of edges between the two faces, across their
// seams: closed, as where a spout meets a pot or one pipe another, or
// open, ending at each end where it meets a flat face, as a band's side
// on a cone ends at the band's end and at the base (rollingend.go). A
// chamfer is made the same way, its sections straight from the points
// at its distance along each face, in the plane square to the edge.

// errNotRolling says that the rolling ball doesn't take the edge at
// all, so that the reason step 10's construction gave stands.
var errNotRolling = errors.New("topo: not an edge for a rolling ball")

// rbFoot is the point of a face (a patch) nearest a point: the patch,
// its parameters there, the point and the face's outward normal.
type rbFoot struct {
	f    int
	u, v float64
	p, n geom.Vector
}

// rbSection is the ball at one place along the edge: its centre and
// the points it touches on each side.
type rbSection struct {
	s    float64
	c    geom.Vector
	foot [2]rbFoot
	arc  srf.BSplineCurve
}

// rollingBall is a fillet's edge, its two sides, and the ball.
type rollingBall struct {
	t      *Topology
	r      float64
	flat   bool
	sigma  float64 // 1 for a convex edge, whose ball lies inside the solid
	pieces []ChainPiece
	verts  []int // the chain's vertices, verts[i] where piece i starts
	side   [2][]int
	closed bool
	// For an open chain, the flat faces it ends at, at its start and its
	// end.
	ends [2]*rbEnd
	// endS are the arc lengths where the points the ball touches each
	// face and its arc's middle lie in each end's flat face's plane;
	// target, where the ball's own sections take over from the conics.
	endS   [2][3]float64
	target [2]float64
	// uvLoops are faces' trims in their patches' (u, v).
	uvLoops map[int][][]geom.Vector
	// The edge, densely, on both faces, with the arc length along it.
	pts    []geom.Vector
	tan    []geom.Vector
	arcl   []float64
	length float64
	tol    float64
}

// smoothEdge reports whether edge x joins two faces that meet smoothly
// along it, as the patches of one face do at a seam.
func (t *Topology) smoothEdge(x int) bool {
	if t.Edges[x].Seam {
		return true
	}
	ed := &t.Edges[x]
	fs := t.EdgeFaces(x)
	if len(ed.CoEdges) != 2 || fs[0] < 0 || fs[1] < 0 || fs[0] == fs[1] {
		return false
	}
	a, b := t.Surface(fs[0]), t.Surface(fs[1])
	if a == nil || b == nil {
		return false
	}
	for _, p := range edgeSamples(ed.Points) {
		var ua, va, ub, vb float64
		a.ClosestPointToUV(p, &ua, &va, true)
		b.ClosestPointToUV(p, &ub, &vb, true)
		na, nb := a.NormalAt(ua, va), b.NormalAt(ub, vb)
		if na.Magnitude() == 0 || nb.Magnitude() == 0 {
			continue
		}
		if na.WithMagnitude(1).Dot(nb.WithMagnitude(1)) < seamCos {
			return false
		}
	}
	return true
}

// Smooth reports whether edge e is a seam where its two faces go on
// into each other without a crease, as the patches of a loft or of a
// file's face cut along its cells do: no fillet or chamfer rounds it.
func (t *Topology) Smooth(e int) bool { return t.smoothEdge(e) }

// blendNext is the edge that goes on from e at vertex v as one edge of
// the solid between the same two faces: the only edge there, other than
// e, that is not smooth; -1 when there is none.
func (t *Topology) blendNext(e, v int) int {
	next := -1
	for _, x := range t.Vertices[v].Edges {
		if x == e || t.smoothEdge(x) {
			continue
		}
		if next >= 0 {
			return -1
		}
		next = x
	}
	return next
}

// blendChain is the edge of the solid through edge e, across the seams
// of the faces at it, and whether it closes.
func (t *Topology) blendChain(e int) ([]ChainPiece, []int, bool) {
	pieces := []ChainPiece{{Edge: e}}
	verts := []int{t.Edges[e].V[0]}
	seen := map[int]bool{e: true}
	v := t.Edges[e].V[1]
	for cur := e; ; {
		if v == verts[0] {
			return pieces, verts, true
		}
		nx := t.blendNext(cur, v)
		if nx < 0 || seen[nx] {
			break
		}
		seen[nx] = true
		rev := t.Edges[nx].V[1] == v
		pieces = append(pieces, ChainPiece{Edge: nx, Reversed: rev})
		verts = append(verts, v)
		cur = nx
		if rev {
			v = t.Edges[nx].V[0]
		} else {
			v = t.Edges[nx].V[1]
		}
	}
	verts = append(verts, v)
	return pieces, verts, false
}

// blendChainBack goes on with an open chain before its first piece, as
// far as it goes as one edge between the same two faces.
func (t *Topology) blendChainBack(pieces []ChainPiece, verts []int) ([]ChainPiece, []int) {
	seen := map[int]bool{}
	for _, pc := range pieces {
		seen[pc.Edge] = true
	}
	cur, v := pieces[0].Edge, verts[0]
	for {
		nx := t.blendNext(cur, v)
		if nx < 0 || seen[nx] {
			return pieces, verts
		}
		seen[nx] = true
		// The piece runs into v from w.
		rev := t.Edges[nx].V[0] == v
		w := t.Edges[nx].V[0]
		if rev {
			w = t.Edges[nx].V[1]
		}
		pieces = append([]ChainPiece{{Edge: nx, Reversed: rev}}, pieces...)
		verts = append([]int{w}, verts...)
		cur, v = nx, w
	}
}

// joinedAt reports whether faces a and b are one face at vertex v, or
// two patches of one meeting smoothly there.
func (t *Topology) joinedAt(v, a, b int) bool {
	if a == b {
		return true
	}
	for _, x := range t.Vertices[v].Edges {
		fs := t.EdgeFaces(x)
		if (fs[0] == a && fs[1] == b || fs[0] == b && fs[1] == a) && t.smoothEdge(x) {
			return true
		}
	}
	return false
}

// generalBlendEdge reports whether edge e is curved, or straight with a
// curved face at it, a rolling ball's to round when step 10's
// construction doesn't take it.
func (t *Topology) generalBlendEdge(e int) bool {
	fs := t.EdgeFaces(e)
	if fs[0] < 0 || fs[1] < 0 || len(t.Edges[e].CoEdges) != 2 {
		return false
	}
	if !t.Straight(e) {
		return true
	}
	_, flat0 := t.planarNormal(fs[0])
	_, flat1 := t.planarNormal(fs[1])
	return !flat0 || !flat1
}

// newRollingBall works out the chain through edge e and its sides.
func (t *Topology) newRollingBall(e int, r float64, flat bool) (*rollingBall, error) {
	at := midpoint(t.Edges[e].Points)
	pieces, verts, closed := t.blendChain(e)
	if !closed {
		// Walk back from e's start too: the chain may go on before it.
		pieces, verts = t.blendChainBack(pieces, verts)
	}
	rb := &rollingBall{t: t, r: r, flat: flat, pieces: pieces, verts: verts, closed: closed}
	// Each piece's faces, side by side with the first's.
	fs := t.EdgeFaces(pieces[0].Edge)
	first := fs
	cur := fs
	seen := [2]map[int]bool{{}, {}}
	for i, pc := range pieces {
		g := t.EdgeFaces(pc.Edge)
		if i > 0 {
			v := verts[i]
			switch {
			case t.joinedAt(v, cur[0], g[0]) && t.joinedAt(v, cur[1], g[1]):
			case t.joinedAt(v, cur[0], g[1]) && t.joinedAt(v, cur[1], g[0]):
				g[0], g[1] = g[1], g[0]
			default:
				return nil, filletErr(t.Vertices[v].P, "the edge goes on between other faces here")
			}
		}
		cur = g
		for k := range 2 {
			if !seen[k][g[k]] {
				seen[k][g[k]] = true
				rb.side[k] = append(rb.side[k], g[k])
			}
		}
	}
	for k := range 2 {
		for _, f := range rb.side[k] {
			if seen[1-k][f] {
				return nil, filletErr(at, "the edge has one face on both its sides")
			}
		}
	}
	rb.tol = math.Max(srf.ChordTolMm()/10, 1e-6)
	if !closed {
		for k, at := range []struct {
			v     int
			faces [2]int
			edge  int
		}{{verts[0], first, pieces[0].Edge}, {verts[len(verts)-1], cur, pieces[len(pieces)-1].Edge}} {
			end, err := t.rbEndAt(at.v, at.faces, at.edge)
			if err != nil {
				return nil, err
			}
			rb.ends[k] = end
		}
	}
	if err := rb.trace(); err != nil {
		return nil, err
	}
	// Convex when face 0 runs away from face 1's outside, seen at the
	// middle of the edge, away from an open chain's ends.
	mi := len(rb.pts) / 2
	p := rb.pts[mi]
	a, b := rb.foot(0, p), rb.foot(1, p)
	d0 := rb.into(0, mi, a.n)
	rb.sigma = 1
	if d0.Dot(b.n) > 0 {
		rb.sigma = -1
	}
	return rb, nil
}

// foot is the point of side k's faces nearest x. Past an open chain's
// ends the ball runs on outside the solid, on the faces' surfaces
// carried on past their patches' sides: a patch that holds the point
// within its own (u, v) is taken first, else the one it lies least
// outside of.
func (rb *rollingBall) foot(k int, x geom.Vector) rbFoot {
	best := rbFoot{f: -1}
	bestD, bestOut := math.Inf(1), math.Inf(1)
	ext := 0.0
	if !rb.closed {
		ext = 1
	}
	inside := 0
	for _, f := range rb.side[k] {
		s := rb.t.Surface(f)
		u, v, p := nearestOnWithin(s, x, ext)
		out := math.Max(math.Max(-u, u-1), math.Max(-v, v-1))
		out = math.Max(0, out-1e-9)
		d := p.Minus(x).Magnitude()
		if out == 0 {
			inside++
		}
		if out < bestOut-1e-12 || out <= bestOut+1e-12 && d < bestD {
			bestD, bestOut = d, out
			best = rbFoot{f: f, u: u, v: v, p: p}
		}
	}
	if inside > 1 {
		// Faces that share a patch, trimmed apart, as a file's are cut
		// along its cells: the one whose trims hold the point.
		for _, f := range rb.side[k] {
			if f == best.f || rb.t.Surface(f) != rb.t.Surface(best.f) && !sameSurface(rb.t.Surface(f), rb.t.Surface(best.f)) {
				continue
			}
			if !rb.onFaceUV(best.f, best.u, best.v) && rb.onFaceUV(f, best.u, best.v) {
				best.f = f
				break
			}
		}
	}
	s := rb.t.Surface(best.f)
	n := s.NormalAt(best.u, best.v)
	if n.Magnitude() > 0 {
		best.n = n.WithMagnitude(1)
	}
	return best
}

// into is the direction from the edge into side k's face at dense point
// i, square to the edge, in the face's tangent plane, whose normal is n.
func (rb *rollingBall) into(k, i int, n geom.Vector) geom.Vector {
	d := rb.tan[i].Cross(n).WithMagnitude(1)
	p := rb.pts[i]
	step := math.Max(1e-4*rb.r, 1e-5)
	// The side whose point, put back on the face, is nearer the step out
	// than the other's: the face goes on there.
	for _, sg := range []float64{1, -1} {
		q := p.Plus(d.ScaledBy(sg * step))
		f := rb.foot(k, q)
		if rb.t.onFace(f.f, f.p) && f.p.Minus(q).Magnitude() < 0.1*step {
			return d.ScaledBy(sg)
		}
	}
	return d
}

// onFaceUV reports whether (u, v) of face f's patch lies inside the
// face's trims, their loops in (u, v) found once.
func (rb *rollingBall) onFaceUV(f int, u, v float64) bool {
	if rb.uvLoops == nil {
		rb.uvLoops = map[int][][]geom.Vector{}
	}
	loops, ok := rb.uvLoops[f]
	if !ok {
		s := rb.t.Surface(f)
		for _, l := range rb.t.Faces[f].Loops {
			loops = append(loops, rb.t.loopUV(s, rb.t.Loops[l].CoEdges))
		}
		rb.uvLoops[f] = loops
	}
	in := false
	for _, uv := range loops {
		if len(uv) > 2 && pointInPolygon(geom.Vector{X: u, Y: v}, uv) {
			in = !in
		}
	}
	return in
}

// sameSurface reports whether two patches are the same surface: of one
// degree, their control points and weights equal.
func sameSurface(a, b *srf.SSurface) bool {
	if a.DegM != b.DegM || a.DegN != b.DegN {
		return false
	}
	for i := 0; i <= a.DegM; i++ {
		for j := 0; j <= a.DegN; j++ {
			if !a.Ctrl[i][j].Equals(b.Ctrl[i][j]) || a.Weight[i][j] != b.Weight[i][j] {
				return false
			}
		}
	}
	return true
}

// onFace reports whether p, on face f's surface, lies inside the face.
func (t *Topology) onFace(f int, p geom.Vector) bool {
	s := t.Surface(f)
	var u, v float64
	s.ClosestPointToUV(p, &u, &v, true)
	in := false
	for _, l := range t.Faces[f].Loops {
		uv := t.loopUV(s, t.Loops[l].CoEdges)
		if len(uv) > 2 && pointInPolygon(geom.Vector{X: u, Y: v}, uv) {
			in = !in
		}
	}
	return in
}

// onEdge is the point of both sides' faces in the plane through x
// square to d, near x, with their normals there.
func (rb *rollingBall) onEdge(x, d geom.Vector) (geom.Vector, rbFoot, rbFoot, bool) {
	p := x
	var a, b rbFoot
	for range 60 {
		a, b = rb.foot(0, p), rb.foot(1, p)
		if math.Abs(a.n.Dot(b.n.Cross(d))) < 1e-9 {
			return p, a, b, false
		}
		q := meet3(a.n, a.n.Dot(a.p), b.n, b.n.Dot(b.p), d, d.Dot(x))
		moved := q.Minus(p).Magnitude()
		p = q
		if moved < 1e-12 {
			break
		}
	}
	return p, a, b, true
}

// trace finds the edge densely on both faces, and its tangents.
func (rb *rollingBall) trace() error {
	t := rb.t
	var raw []geom.Vector
	for _, pc := range rb.pieces {
		pts := t.Edges[pc.Edge].Points
		if pc.Reversed {
			pts = reversed(pts)
		}
		raw = append(raw, pts[:len(pts)-1]...)
	}
	segs := len(raw)
	if !rb.closed {
		last := rb.pieces[len(rb.pieces)-1]
		pts := t.Edges[last.Edge].Points
		if last.Reversed {
			pts = reversed(pts)
		}
		raw = append(raw, pts[len(pts)-1])
	}
	total := 0.0
	for i := range segs {
		total += raw[(i+1)%len(raw)].Minus(raw[i]).Magnitude()
	}
	// Pieces short against the radius and the edge, so that the edge's
	// tangent turns little from one to the next.
	h := math.Min(rb.r/4, total/256)
	for i := 0; i <= segs; i++ {
		a, b := raw[i%len(raw)], raw[(i+1)%len(raw)]
		n := int(math.Ceil(b.Minus(a).Magnitude() / h))
		if i == segs {
			if rb.closed {
				break
			}
			// An open edge's last point, its tangent the last piece's.
			a, b = raw[i-1], raw[i]
			n = 1
		}
		d := b.Minus(a).WithMagnitude(1)
		for j := 0; j < n; j++ {
			x := a.Plus(b.Minus(a).ScaledBy(float64(j) / float64(n)))
			if i == segs {
				x = b
			}
			p, fa, fb, ok := rb.onEdge(x, d)
			if !ok {
				return filletErr(x, "the faces meet tangent here: a fillet rounds an edge where they meet at an angle")
			}
			tn := fa.n.Cross(fb.n)
			if tn.Magnitude() < 1e-9 {
				return filletErr(x, "the faces meet tangent here: a fillet rounds an edge where they meet at an angle")
			}
			tn = tn.WithMagnitude(1)
			if tn.Dot(d) < 0 {
				tn = tn.ScaledBy(-1)
			}
			rb.pts = append(rb.pts, p)
			rb.tan = append(rb.tan, tn)
		}
	}
	rb.arcl = make([]float64, len(rb.pts)+1)
	for i := range rb.pts {
		rb.arcl[i+1] = rb.arcl[i] + rb.pts[(i+1)%len(rb.pts)].Minus(rb.pts[i]).Magnitude()
	}
	rb.length = rb.arcl[len(rb.pts)]
	if !rb.closed {
		// Open, the edge runs from its first point to its last.
		rb.length = rb.arcl[len(rb.pts)-1]
	}
	return nil
}

// along is the point and tangent of the dense edge at arc length s,
// taken round the closed edge.
func (rb *rollingBall) along(s float64) (geom.Vector, geom.Vector) {
	if !rb.closed {
		// Past an open edge's ends, on along the tangent there.
		n := len(rb.pts)
		switch {
		case s <= 0:
			return rb.pts[0].Plus(rb.tan[0].ScaledBy(s)), rb.tan[0]
		case s >= rb.length:
			return rb.pts[n-1].Plus(rb.tan[n-1].ScaledBy(s - rb.length)), rb.tan[n-1]
		}
	}
	s = math.Mod(s, rb.length)
	if s < 0 {
		s += rb.length
	}
	i := sort.SearchFloat64s(rb.arcl, s)
	if i > 0 {
		i--
	}
	if i >= len(rb.pts) {
		i = len(rb.pts) - 1
	}
	j := (i + 1) % len(rb.pts)
	f := (s - rb.arcl[i]) / (rb.arcl[i+1] - rb.arcl[i])
	p := rb.pts[i].Plus(rb.pts[j].Minus(rb.pts[i]).ScaledBy(f))
	d := rb.tan[i].ScaledBy(1 - f).Plus(rb.tan[j].ScaledBy(f)).WithMagnitude(1)
	return p, d
}

// section is the ball at arc length s along the edge: in the plane
// square to the edge there, touching both faces.
func (rb *rollingBall) section(s float64) (rbSection, error) {
	if rb.flat {
		return rb.chamferSection(s)
	}
	x, d := rb.along(s)
	p, a, b, ok := rb.onEdge(x, d)
	sec := rbSection{s: s}
	if !ok {
		return sec, filletErr(x, "the faces meet tangent here: a fillet rounds an edge where they meet at an angle")
	}
	sr := rb.sigma * rb.r
	c := meet3(a.n, a.n.Dot(p)-sr, b.n, b.n.Dot(p)-sr, d, d.Dot(x))
	converged := false
	for range 200 {
		a, b = rb.foot(0, c), rb.foot(1, c)
		if math.Abs(a.n.Dot(b.n.Cross(d))) < 1e-9 {
			break
		}
		cn := meet3(a.n, a.n.Dot(a.p)-sr, b.n, b.n.Dot(b.p)-sr, d, d.Dot(x))
		moved := cn.Minus(c).Magnitude()
		c = cn
		if moved < 1e-11*math.Max(1, rb.r) {
			converged = true
			break
		}
	}
	a, b = rb.foot(0, c), rb.foot(1, c)
	for k, f := range []rbFoot{a, b} {
		dist := c.Minus(f.p)
		eps := 1e-6 * math.Max(1, rb.r)
		if !converged || math.Abs(dist.Magnitude()-rb.r) > eps || math.Abs(dist.Dot(f.n)+sr) > eps {
			return sec, filletErr(f.p, "the ball the fillet rolls doesn't touch the face on side %d here: "+
				"the fillet runs off the face, or its radius is too large for the face's curve", k+1)
		}
	}
	sec.c, sec.foot = c, [2]rbFoot{a, b}
	sec.arc = rb.arcOf(c, a.p, b.p)
	return sec, nil
}

// chamferSection is a chamfer's section at arc length s along the edge:
// in the plane square to the edge there, the line between the points of
// each face at the chamfer's distance from the edge.
func (rb *rollingBall) chamferSection(s float64) (rbSection, error) {
	x, d := rb.along(s)
	p, a, b, ok := rb.onEdge(x, d)
	sec := rbSection{s: s}
	if !ok {
		return sec, filletErr(x, "the faces meet tangent here: a chamfer cuts an edge where they meet at an angle")
	}
	at := [2]rbFoot{a, b}
	for k := range 2 {
		dir := d.Cross(at[k].n)
		if dir.Magnitude() < 1e-12 {
			return sec, filletErr(x, "the faces meet tangent here: a chamfer cuts an edge where they meet at an angle")
		}
		dir = dir.WithMagnitude(1)
		// Into face k: away from the other face's outside at a convex
		// edge, towards it at a concave one.
		if rb.sigma*dir.Dot(at[1-k].n) > 0 {
			dir = dir.ScaledBy(-1)
		}
		q := p.Plus(dir.ScaledBy(rb.r))
		converged := false
		for range 100 {
			f := rb.foot(k, q)
			y := f.p.Minus(d.ScaledBy(d.Dot(f.p.Minus(p))))
			if y.Minus(p).Magnitude() < 1e-12 {
				break
			}
			qn := p.Plus(y.Minus(p).WithMagnitude(rb.r))
			moved := qn.Minus(q).Magnitude()
			q = qn
			if moved < 1e-11*math.Max(1, rb.r) {
				converged = true
				break
			}
		}
		f := rb.foot(k, q)
		if !converged || f.p.Minus(q).Magnitude() > 1e-6*math.Max(1, rb.r) {
			return sec, filletErr(q, "the chamfer's cut doesn't reach the face on side %d here: the chamfer runs off "+
				"the face, or its distance is too large for the face's curve", k+1)
		}
		sec.foot[k] = f
	}
	sec.c = sec.foot[0].p.Plus(sec.foot[1].p).ScaledBy(0.5)
	sec.arc = rb.arcOf(sec.c, sec.foot[0].p, sec.foot[1].p)
	return sec, nil
}

// arcOf is the section from f0 to f1 about c, as a B-spline curve: the
// arc of the circle, or for a chamfer the line.
func (rb *rollingBall) arcOf(c, f0, f1 geom.Vector) srf.BSplineCurve {
	if rb.flat {
		return srf.BSplineCurve{Deg: 2, Knots: []float64{0, 0, 0, 1, 1, 1},
			Ctrl: []geom.Vector{f0, f0.Plus(f1).ScaledBy(0.5), f1}, Weight: []float64{1, 1, 1}}
	}
	da, db := f0.Minus(c), f1.Minus(c)
	cosT := da.Dot(db) / (da.Magnitude() * db.Magnitude())
	theta := math.Acos(math.Max(-1, math.Min(1, cosT)))
	w := math.Cos(theta / 2)
	mid := c.Plus(da.Plus(db).WithMagnitude(rb.r / w))
	return srf.BSplineCurve{Deg: 2, Knots: []float64{0, 0, 0, 1, 1, 1},
		Ctrl: []geom.Vector{f0, mid, f1}, Weight: []float64{1, w, 1}}
}

// homog is a section's control points in homogeneous coordinates.
func homog(c srf.BSplineCurve) []geom.Vector4 {
	out := make([]geom.Vector4, len(c.Ctrl))
	for i := range c.Ctrl {
		out[i] = geom.Vector4FromWeighted(c.Weight[i], c.Ctrl[i])
	}
	return out
}

// derivative is the sections' homogeneous control points' derivative
// along the edge at s, per unit of arc length.
func (rb *rollingBall) derivative(s float64) ([]geom.Vector4, error) {
	h := 1e-4 * math.Min(rb.r, rb.length)
	a, err := rb.section(s - h)
	if err != nil {
		return nil, err
	}
	b, err := rb.section(s + h)
	if err != nil {
		return nil, err
	}
	ha, hb := homog(a.arc), homog(b.arc)
	out := make([]geom.Vector4, len(ha))
	for i := range ha {
		out[i] = hb[i].Minus(ha[i]).ScaledBy(1 / (2 * h))
	}
	return out, nil
}

// crossings are the arc lengths where the ball's point on side k moves
// from one patch to the next, at a seam, found from a scan.
func (rb *rollingBall) crossings(scan []rbSection) ([]float64, [][2]int, []int, error) {
	var at []float64
	var pairs [][2]int
	var sides []int
	n := len(scan)
	pairsN := n
	if !rb.closed {
		pairsN = n - 1
	}
	for k := range 2 {
		for i := 0; i < pairsN; i++ {
			a, b := scan[i], scan[(i+1)%n]
			fa, fb := a.foot[k].f, b.foot[k].f
			if fa == fb {
				continue
			}
			lo, hi := a.s, b.s
			if hi < lo {
				hi += rb.length
			}
			for it := 0; it < 60 && hi-lo > 1e-10*rb.length; it++ {
				m := (lo + hi) / 2
				sec, err := rb.section(m)
				if err != nil {
					return nil, nil, nil, err
				}
				if sec.foot[k].f == fa {
					lo = m
				} else {
					hi = m
				}
			}
			s := (lo + hi) / 2
			if rb.closed {
				s = math.Mod(s, rb.length)
			}
			at = append(at, s)
			pairs = append(pairs, [2]int{fa, fb})
			sides = append(sides, k)
		}
	}
	return at, pairs, sides, nil
}

// fit is the fillet's surface through the sections at arc lengths ss,
// rising from ss[0] to less than a turn round the edge past it, and
// round to ss[0] again: the first section is the last too, and the
// surface's derivative across is the same at both.
func (rb *rollingBall) fit(ss []float64) (*srf.BSplineSurface, []rbSection, []float64, error) {
	secs := make([]rbSection, 0, len(ss)+1)
	for _, s := range ss {
		sec, err := rb.section(s)
		if err != nil {
			return nil, nil, nil, err
		}
		sec.s = s
		secs = append(secs, sec)
	}
	last := secs[0]
	last.s = ss[0] + rb.length
	secs = append(secs, last)
	ts := make([]float64, len(secs))
	curves := make([]srf.BSplineCurve, len(secs))
	for i := range secs {
		ts[i] = (secs[i].s - ss[0]) / rb.length
		curves[i] = secs[i].arc
	}
	ts[len(ts)-1] = 1
	d, err := rb.derivative(ss[0])
	if err != nil {
		return nil, nil, nil, err
	}
	for i := range d {
		d[i] = d[i].ScaledBy(rb.length)
	}
	sf, err := srf.SkinThrough(curves, ts, d, d)
	if err != nil {
		return nil, nil, nil, err
	}
	return sf, secs, ts, nil
}

// misfit is how far the surface strays at t, between two of its
// sections, from the faces it should touch and from the ball (for a
// chamfer, the line) at arc length s, and where. With near, the ball is
// the one nearest the surface's middle point about s, as between an
// oblique end section and the next.
func (rb *rollingBall) misfit(sf *srf.BSplineSurface, t, s float64, near bool) (float64, geom.Vector) {
	p0, p1, pm := sf.PointAt(0, t), sf.PointAt(1, t), sf.PointAt(0.5, t)
	e := rb.foot(0, p0).p.Minus(p0).Magnitude()
	e = math.Max(e, rb.foot(1, p1).p.Minus(p1).Magnitude())
	where := pm
	if near && rb.flat {
		// Near the ends a chamfer's rulings run between the points its
		// oblique sections touch the faces: flat across.
		for _, u := range []float64{0.25, 0.5, 0.75} {
			p := sf.PointAt(u, t)
			if d := distanceToPolyline(p, []geom.Vector{p0, p1}); d > e {
				e, where = d, p
			}
		}
		return e, where
	}
	across := []float64{0.5}
	if near {
		// Between oblique sections the surface is checked across too,
		// against the conics' own fit to the balls, a few times the
		// tolerance.
		across = []float64{0.25, 0.5, 0.75}
	}
	for _, u := range across {
		p := sf.PointAt(u, t)
		su := s
		if near {
			su = rb.nearestSection(p, s)
		}
		sec, err := rb.section(su)
		if err != nil {
			return math.Inf(1), p
		}
		var d float64
		if rb.flat {
			d = distanceToPolyline(p, []geom.Vector{sec.foot[0].p, sec.foot[1].p})
		} else {
			d = math.Abs(p.Minus(sec.c).Magnitude() - rb.r)
		}
		if near {
			d /= 3
		}
		if d > e {
			e, where = d, sec.c
		}
	}
	return e, where
}

// nearestSection is the arc length, from s, of the section whose ball's
// centre (for a chamfer, its line) is nearest p: by parabolas through
// three.
func (rb *rollingBall) nearestSection(p geom.Vector, s float64) float64 {
	dist := func(s float64) float64 {
		sec, err := rb.section(s)
		if err != nil {
			return math.Inf(1)
		}
		if rb.flat {
			d := distanceToPolyline(p, []geom.Vector{sec.foot[0].p, sec.foot[1].p})
			return d * d
		}
		return sec.c.Minus(p).MagSquared()
	}
	h := 0.05 * rb.r
	for range 6 {
		a, b, c := dist(s-h), dist(s), dist(s+h)
		den := a - 2*b + c
		if math.IsInf(a+b+c, 0) || !(den > 0) {
			break
		}
		step := h * (a - c) / (2 * den)
		step = math.Max(-2*h, math.Min(2*h, step))
		s += step
		if math.Abs(step) < 1e-9*math.Max(1, rb.r) {
			break
		}
		h = math.Max(math.Abs(step), 1e-6*rb.r)
	}
	return s
}

// blend is the fillet's surface, its sections, and the arc lengths of
// the crossings of seams among them.
func (rb *rollingBall) blend() (*srf.BSplineSurface, []rbSection, []rbCrossing, error) {
	if !rb.closed {
		return rb.blendOpen()
	}
	n := int(math.Min(2000, math.Max(64, 4*rb.length/rb.r)))
	scan := make([]rbSection, n)
	for i := range scan {
		sec, err := rb.section(rb.length * float64(i) / float64(n))
		if err != nil {
			return nil, nil, nil, err
		}
		scan[i] = sec
	}
	at, pairs, sides, err := rb.crossings(scan)
	if err != nil {
		return nil, nil, nil, err
	}
	start := 0.0
	if len(at) > 0 {
		start = at[0]
		for _, s := range at {
			start = math.Min(start, s)
		}
	}
	unwrap := func(s float64) float64 {
		d := math.Mod(s-start, rb.length)
		if d < 0 {
			d += rb.length
		}
		return start + d
	}
	var cross []rbCrossing
	breaks := []float64{start}
	for i, s := range at {
		u := unwrap(s)
		cross = append(cross, rbCrossing{s: u, faces: pairs[i], side: sides[i]})
		if u != start {
			breaks = append(breaks, u)
		}
	}
	slices.Sort(breaks)
	breaks = append(breaks, start+rb.length)
	// Sections between the crossings, no further apart than a twelfth of
	// the edge or two radii.
	h := math.Min(rb.length/12, 2*rb.r)
	var ss []float64
	for i := 0; i+1 < len(breaks); i++ {
		a, b := breaks[i], breaks[i+1]
		if b-a < 1e-6*rb.length {
			// Crossings on the two sides at one place: one section for both.
			continue
		}
		m := int(math.Ceil((b - a) / h))
		for j := range m {
			ss = append(ss, a+(b-a)*float64(j)/float64(m))
		}
	}
	for round := 0; ; round++ {
		sf, secs, ts, err := rb.fit(ss)
		if err != nil {
			return nil, nil, nil, err
		}
		var more []float64
		worst, where := 0.0, geom.Vector{}
		for i := 0; i+1 < len(secs); i++ {
			sm := (secs[i].s + secs[i+1].s) / 2
			if _, err := rb.section(sm); err != nil {
				return nil, nil, nil, err
			}
			if e, at := rb.misfit(sf, (ts[i]+ts[i+1])/2, sm, false); e > rb.tol {
				more = append(more, sm)
				if e > worst {
					worst, where = e, at
				}
			}
		}
		if len(more) == 0 {
			return sf, secs, cross, nil
		}
		if round == 10 || len(ss)+len(more) > 600 {
			return nil, nil, nil, filletErr(where, "the fillet's surface can't be fitted to within %.3g mm here (%.3g)",
				rb.tol, worst)
		}
		ss = append(ss, more...)
		slices.Sort(ss)
	}
}

// rbCrossing is where the ball's point on one side crosses a seam from
// one patch of the face to the next.
type rbCrossing struct {
	s     float64
	faces [2]int
	side  int
}

// rollingBallFillet rounds the closed edge through edge e, between two
// curved faces, with radius r, as the package comment above says. The
// fillet's patches get origin as their origin and sheet.
func rollingBallFillet(sh *srf.SShell, t *Topology, e int, r float64, origin uint32, flat bool) error {
	at := midpoint(t.Edges[e].Points)
	rb, err := t.newRollingBall(e, r, flat)
	if err != nil {
		return err
	}
	sf, secs, cross, err := rb.blend()
	if err != nil {
		return err
	}
	all, err := sf.Patches()
	if err != nil {
		return filletErr(at, "the fillet's surface can't be cut into patches: %v", err)
	}
	if len(all) != 1 {
		return filletErr(at, "the fillet's sections are not single arcs")
	}
	row := all[0]
	m := len(row)
	if m != len(secs)-1 {
		return filletErr(at, "the fillet's surface has %d patches for %d sections", m, len(secs))
	}
	// The fillet's corner at section j on side k.
	corner := func(j, k int) geom.Vector {
		i := 0
		if k == 1 {
			i = row[0].DegM
		}
		if j == m {
			if !rb.closed {
				return row[m-1].Ctrl[i][row[m-1].DegN]
			}
			j = 0
		}
		return row[j].Ctrl[i][0]
	}
	onChain := map[int]bool{}
	for _, v := range rb.verts {
		onChain[v] = true
	}
	isChain := map[int]bool{}
	chainCurve := map[core.HSCurve]bool{}
	onChainLine := func(p geom.Vector) bool {
		for _, pc := range rb.pieces {
			if distanceToPolyline(p, t.Edges[pc.Edge].Points) < 10*geom.LengthEps {
				return true
			}
		}
		return false
	}
	for _, pc := range rb.pieces {
		isChain[pc.Edge] = true
		ed := &t.Edges[pc.Edge]
		chainCurve[ed.Curve] = true
		for _, ce := range ed.CoEdges {
			chainCurve[t.CoEdges[ce].Curve] = true
		}
	}

	// Each seam the ball's point crosses is cut there; the piece from
	// the edge to the cut goes.
	type pair struct{ a, b geom.Vector }
	var drop []pair
	for _, c := range cross {
		j, best := -1, 1e-5*rb.length
		for i := range secs {
			if rb.closed && i == m {
				continue
			}
			d := math.Abs(secs[i].s - c.s)
			if rb.closed {
				// Round the closed edge: a crossing just before the start's.
				d = math.Min(d, math.Abs(d-rb.length))
			}
			if d < best {
				j, best = i, d
			}
		}
		if j < 0 {
			return filletErr(at, "a seam's crossing is not among the fillet's sections")
		}
		q := corner(j, c.side)
		// The seam between the two patches from a vertex of the edge, the
		// one nearest the crossing: two patches may meet along two seams,
		// either side of a hole.
		// The seam may end at a vertex of the chain, or part way along one
		// of its edges, as a file's patches' seams do on a whole circle.
		x, v, near := -1, -1, math.Inf(1)
		for y := range t.Edges {
			fs := t.EdgeFaces(y)
			if isChain[y] || !(fs[0] == c.faces[0] && fs[1] == c.faces[1] || fs[0] == c.faces[1] && fs[1] == c.faces[0]) {
				continue
			}
			for _, cv := range t.Edges[y].V {
				if !onChain[cv] && !onChainLine(t.Vertices[cv].P) {
					continue
				}
				if d := distanceToPolyline(q, t.Edges[y].Points); d < near {
					x, v, near = y, cv, d
				}
			}
		}
		if x < 0 {
			return filletErr(q, "the fillet crosses a seam of a face that doesn't reach the edge: make it smaller")
		}
		// By the curve nearest the point: a curve may run along both
		// seams, and an earlier cut gives it new handles.
		if err := splitNearest(sh, q, t.Vertices[v].P); err != nil {
			return filletErr(q, "the fillet is wider than a patch of the face here: make it smaller")
		}
		drop = append(drop, pair{q, t.Vertices[v].P})
	}

	// An open chain's ends: each side's face cut back along the flat
	// face's edge to where the fillet's end meets it, and that piece of
	// the edge gone from the flat face too, which takes the fillet's end.
	var endCuts []rbEndCut
	if !rb.closed {
		for k, end := range rb.ends {
			sec := secs[0]
			if k == 1 {
				sec = secs[len(secs)-1]
			}
			cut, err := rb.cutEnd(sh, end, sec.arc)
			if err != nil {
				return err
			}
			for i := range 2 {
				if cut.goes[i] == nil && (cut.off == nil || cut.off.j != i) {
					drop = append(drop, pair{cut.q[i], t.Vertices[end.v].P})
				}
			}
			if ro := cut.off; ro != nil {
				if err := rb.runOffCuts(sh, row, k, ro); err != nil {
					return err
				}
				w := t.Vertices[ro.w].P
				drop = append(drop, pair{t.Vertices[end.v].P, w}, pair{w, ro.p2})
			}
			endCuts = append(endCuts, cut)
		}
	}
	// The fillet's patches an end that runs off cuts, as loops, and the
	// curves across them in the next face's plane.
	loops := map[int][]srf.SBezier{}
	var across [2][]srf.SBezier
	for k, cut := range endCuts {
		ro := cut.off
		if ro == nil {
			continue
		}
		for b := ro.first; b < ro.after; b++ {
			if _, ok := loops[b]; ok {
				return filletErr(at, "the fillet runs off faces at both its ends: make it smaller")
			}
			l, a, err := clipPatch(&row[b], ro, math.Max(srf.ChordTolMm()/10, 1e-6))
			if err != nil {
				return err
			}
			snapEnds(l, ro.p1At, ro.p1)
			snapEnds(a, ro.p1At, ro.p1)
			loops[b] = l
			across[k] = append(across[k], a...)
		}
	}

	// The faces at the edge, cut back along the fillet's edges.
	color := t.Surface(rb.side[0][0]).Color
	nextH := newCurveHandle(sh)
	for k := range 2 {
		pieces := map[int][]int{}
		for b := range m {
			sec, err := rb.section((secs[b].s + secs[b+1].s) / 2)
			if err != nil {
				return err
			}
			f := sec.foot[k].f
			pieces[f] = append(pieces[f], b)
		}
		for _, f := range rb.side[k] {
			s := sh.FindSurface(t.Faces[f].Surface)
			if s == nil {
				return filletErr(at, "a face at the edge is not in the shell")
			}
			fwd, found := rb.forward(sh, s, chainCurve)
			var kept []srf.STrimBy
			for _, tb := range s.Trim {
				if chainCurve[tb.Curve] {
					continue
				}
				gone := false
				for _, d := range drop {
					if tb.StartP.Equals(d.a) && tb.FinishP.Equals(d.b) || tb.StartP.Equals(d.b) && tb.FinishP.Equals(d.a) {
						gone = true
					}
				}
				if !gone {
					kept = append(kept, tb)
				}
			}
			if len(pieces[f]) > 0 && !found {
				return filletErr(at, "a face the fillet touches is not along the edge")
			}
			for _, b := range pieces[f] {
				sb, ok := rb.contactPiece(row, endCuts, k, b)
				if !ok {
					continue
				}
				var pts []geom.Vector
				sb.MakePwlIntoPoints(&pts, 0, 0)
				pts[0], pts[len(pts)-1] = sb.Start(), sb.Finish()
				nc := srf.SCurve{H: nextH, Exact: sb, IsExact: true, Deg: sb.Degree, SurfA: s.H}
				nextH.V++
				for _, p := range pts {
					nc.Points = append(nc.Points, srf.SCurvePt{Vertex: true, P: p})
				}
				tb := srf.STrimBy{Curve: nc.H, StartP: sb.Start(), FinishP: sb.Finish(), Start: 0, Finish: 1}
				if !fwd {
					tb.Backwards, tb.StartP, tb.FinishP = true, sb.Finish(), sb.Start()
				}
				kept = append(kept, tb)
				sh.Curve = append(sh.Curve, nc)
			}
			s.Trim = kept
			forget(s)
		}
	}
	for k, cut := range endCuts {
		if ro := cut.off; ro != nil {
			// The end section's part under the next face's plane, from side
			// 1-j's cut to p1.
			endP, endSide := 0, 0
			if k == 1 {
				endP, endSide = m-1, 2
			}
			sec := row[endP].EdgeCurve(endSide)
			tp := bezierRoot(&sec, ro.h, 0, 1)
			var part srf.SBezier
			if ro.j == 1 {
				part = sec.Sub(0, tp)
				part.Ctrl[0], part.Ctrl[part.Degree] = sec.Start(), ro.p1
			} else {
				part = sec.Sub(tp, 1)
				part.Ctrl[0], part.Ctrl[part.Degree] = ro.p1, sec.Finish()
			}
			if err := rb.closeRunOff(sh, cut, part, across[k], &nextH); err != nil {
				return err
			}
			continue
		}
		if err := rb.closeEnd(sh, cut, &nextH); err != nil {
			return err
		}
	}
	dropUnusedCurves(sh)

	// The fillet's patches.
	fa := NewFaceAdder(sh)
	for b := range m {
		p := row[b]
		loop := patchLoop(&p)
		if l, ok := loops[b]; ok {
			if len(l) == 0 {
				continue
			}
			loop = l
		}
		p.Color = color
		p.Origin = origin
		p.Sheet = origin
		if _, err := fa.Add(p, [][]srf.SBezier{loop}); err != nil {
			return filletErr(loop[0].Start(), "the fillet's face can't be put in: %v", err)
		}
	}
	t2, err := Build(sh)
	if err != nil {
		return filletErr(at, "%v", err)
	}
	if probs := t2.Validate(); len(probs) > 0 {
		return filletErr(probs[0].At, "the fillet leaves the solid open: %s", probs[0].Msg)
	}
	return nil
}

// forward reports whether surface s's face runs the edge along the way
// the chain does, from a trim of it along the edge, and whether it has
// one.
func (rb *rollingBall) forward(sh *srf.SShell, s *srf.SSurface, chainCurve map[core.HSCurve]bool) (bool, bool) {
	for _, tb := range s.Trim {
		if !chainCurve[tb.Curve] {
			continue
		}
		c := sh.FindCurve(tb.Curve)
		if c == nil || len(c.Points) < 2 {
			continue
		}
		i0, i1 := pointIndex(c, tb.StartP), pointIndex(c, tb.FinishP)
		if i0 < 0 {
			continue
		}
		step := 1
		if i1 >= 0 && i1 < i0 || i1 == i0 && tb.Backwards {
			step = -1
		}
		j := i0 + step
		if j < 0 || j >= len(c.Points) {
			continue
		}
		p, q := c.Points[i0].P, c.Points[j].P
		best, bestD := 0, math.Inf(1)
		for i, x := range rb.pts {
			if d := x.Minus(p).Magnitude(); d < bestD {
				best, bestD = i, d
			}
		}
		return q.Minus(p).Dot(rb.tan[best]) > 0, true
	}
	return false, false
}

// splitNearest splits the shell's curve nearest p, of those running
// through p and v, at p, with every trim along it; and any other curve
// along the same edge, as a Boolean leaves one for each face.
func splitNearest(sh *srf.SShell, p, v geom.Vector) error {
	bestD := math.Inf(1)
	type cand struct {
		h core.HSCurve
		d float64
	}
	var cands []cand
	for i := range sh.Curve {
		c := &sh.Curve[i]
		if pointIndex(c, v) < 0 {
			continue
		}
		pts := make([]geom.Vector, len(c.Points))
		for j := range c.Points {
			pts[j] = c.Points[j].P
		}
		d := distanceToPolyline(p, pts)
		cands = append(cands, cand{c.H, d})
		bestD = math.Min(bestD, d)
	}
	if len(cands) == 0 {
		return ErrNotOnEdge
	}
	for _, c := range cands {
		if c.d <= bestD+geom.LengthEps {
			if err := splitCurve(sh, c.h, p); err != nil {
				return err
			}
		}
	}
	return nil
}

// nearestOnWithin is the point of patch s nearest x, within the patch
// carried on by ext past each of its sides in (u, v): from the kernel's
// guess, by Gauss-Newton steps, which the kernel's own first-order
// steps leave short of for a point off the surface.
func nearestOnWithin(s *srf.SSurface, x geom.Vector, ext float64) (float64, float64, geom.Vector) {
	var u, v float64
	s.ClosestPointToUV(x, &u, &v, false)
	clamp := func(a float64) float64 { return math.Max(-ext, math.Min(1+ext, a)) }
	u, v = clamp(u), clamp(v)
	for range 80 {
		p := s.PointAt(u, v)
		var tu, tv geom.Vector
		s.TangentsAt(u, v, &tu, &tv, true)
		d := x.Minus(p)
		a11, a12, a22 := tu.Dot(tu), tu.Dot(tv), tv.Dot(tv)
		b1, b2 := tu.Dot(d), tv.Dot(d)
		det := a11*a22 - a12*a12
		if !(math.Abs(det) > 1e-300) {
			break
		}
		du, dv := (a22*b1-a12*b2)/det, (a11*b2-a12*b1)/det
		nu, nv := clamp(u+du), clamp(v+dv)
		moved := math.Abs(nu-u) + math.Abs(nv-v)
		u, v = nu, nv
		if moved < 1e-15 {
			break
		}
	}
	return u, v, s.PointAt(u, v)
}
