package topo

import (
	"math"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// Crease ends: a fillet or chamfer that ends at a corner of four flat
// faces where it is the only edge rounded, as one of a saddle's four
// edges at its middle. Its two faces each go on there, past an edge
// left sharp (along), into a face across the corner, and the two faces
// across meet along the fourth edge, the crease, left sharp too.
//
// The fillet ends where its surface carried on meets those two faces'
// planes, as it ends in an end face's plane at a corner of three
// (filletCornerAt's kind 1): its cross section at the vertex projected
// along the edge onto them, the part on the one side of the crease
// onto the one face, the rest onto the other. Projecting along the edge
// is affine, so each part, a piece of the section's arc, stays a
// rational quadratic of the arc's own weights, lying on the fillet's
// surface and in its face's plane exactly: the two faces' new edges.
// The section's ends land on the along edges, where the fillet's
// tangent lines meet them, and the point between the two parts on the
// crease, which is cut back to it. OpenCASCADE's makeFillet ends one
// rounded edge of a saddle so (scripts/fillet-saddle-sharp-fixtures.py).

// creaseCorner is the kind of a crease end.
const creaseCorner = 7

// creaseEndOf is the one rounded edge at v, its along edges by face, the
// faces across and the crease, where v is such a corner.
func (t *Topology) creaseEndOf(v int, edges []filletEdge, byEdge map[int]int) (fe *filletEdge, along, across [2]int, crease int, ok bool) {
	vt := &t.Vertices[v]
	if len(vt.Edges) != 4 || len(t.facesAt(v)) != 4 {
		return nil, along, across, -1, false
	}
	for _, x := range vt.Edges {
		if !t.Straight(x) {
			return nil, along, across, -1, false
		}
		if i, ok := byEdge[x]; ok {
			if fe != nil {
				return nil, along, across, -1, false
			}
			fe = &edges[i]
		}
	}
	if fe == nil || fe.circ || fe.varies() {
		return nil, along, across, -1, false
	}
	for _, f := range t.facesAt(v) {
		if _, ok := t.planarNormal(f); !ok {
			return nil, along, across, -1, false
		}
	}
	along, across, crease = [2]int{-1, -1}, [2]int{-1, -1}, -1
	for _, x := range vt.Edges {
		if x == fe.e {
			continue
		}
		efs := t.EdgeFaces(x)
		placed := false
		for k := range 2 {
			if efs[0] == fe.f[k] || efs[1] == fe.f[k] {
				if along[k] >= 0 {
					return nil, along, across, -1, false
				}
				along[k] = x
				across[k] = efs[0] + efs[1] - fe.f[k]
				placed = true
			}
		}
		if !placed {
			crease = x
		}
	}
	if along[0] < 0 || along[1] < 0 || crease < 0 || across[0] == across[1] {
		return nil, along, across, -1, false
	}
	cf := t.EdgeFaces(crease)
	if !(cf[0] == across[0] && cf[1] == across[1]) && !(cf[0] == across[1] && cf[1] == across[0]) {
		return nil, along, across, -1, false
	}
	return fe, along, across, crease, true
}

// creaseEndAt works out the crease end at v (creaseEndOf), or with flat
// the chamfer's.
func (t *Topology) creaseEndAt(v int, edges []filletEdge, byEdge map[int]int, flat bool) (*filletCorner, error) {
	fe, along, across, crease, ok := t.creaseEndOf(v, edges, byEdge)
	if !ok {
		return nil, filletErr(t.Vertices[v].P, "the corner is not a crease end")
	}
	p := t.Vertices[v].P
	cw := t.edgeAway(crease, v)
	var n [2]geom.Vector
	var side [2]float64
	for k := range 2 {
		n[k], _ = t.planarNormal(across[k])
		if math.Abs(n[k].Dot(fe.dir)) < 1e-6 {
			return nil, filletErr(p, "the fillet runs along a face at its end")
		}
		// The face across lies on the side of the crease its along edge
		// runs on.
		side[k] = cw.Cross(t.edgeAway(along[k], v)).Dot(n[k])
		if math.Abs(side[k]) < 1e-9 {
			return nil, filletErr(p, "the faces at the corner fold flat")
		}
	}
	// onto is q projected along the edge onto face k's plane; in says
	// how far onto the face's side of the crease it falls.
	onto := func(k int, q geom.Vector) geom.Vector {
		return q.Plus(fe.dir.ScaledBy(n[k].Dot(p.Minus(q)) / n[k].Dot(fe.dir)))
	}
	in := func(k int, q geom.Vector) float64 {
		return cw.Cross(onto(k, q).Minus(p)).Dot(n[k]) * math.Copysign(1, side[k])
	}
	sec := fe.section(p, flat)
	if in(0, sec.Start()) <= 0 || in(1, sec.Finish()) <= 0 || in(0, sec.Finish()) >= 0 {
		return nil, filletErr(p, "the fillet's end doesn't cross the edge left sharp between the faces it ends on")
	}
	lo, hi := 0.0, 1.0
	for range 80 {
		mid := (lo + hi) / 2
		if in(0, sec.PointAt(mid)) > 0 {
			lo = mid
		} else {
			hi = mid
		}
	}
	u := (lo + hi) / 2
	for _, s := range []float64{0.25, 0.5, 0.75} {
		// Each part on its own side, so each is one face's.
		if in(0, sec.PointAt(u*s)) <= 0 || in(1, sec.PointAt(u+(1-u)*s)) <= 0 {
			return nil, filletErr(p, "the fillet's end crosses the edge left sharp more than once")
		}
	}
	a, b := sec.Sub(0, u), sec.Sub(u, 1)
	for i := 0; i <= a.Degree; i++ {
		a.Ctrl[i] = onto(0, a.Ctrl[i])
		b.Ctrl[i] = onto(1, b.Ctrl[i])
	}
	m := a.Finish().Plus(b.Start()).ScaledBy(0.5)
	a.Ctrl[a.Degree], b.Ctrl[0] = m, m
	margin := math.Max(fe.ss[0], fe.ss[1]) + 2*fe.r
	for _, q := range []geom.Vector{a.Start(), m, b.Finish()} {
		if math.Abs(q.Minus(p).Dot(fe.dir)) > margin-geom.LengthEps {
			return nil, filletErr(p, "the fillet's end runs too far past the corner")
		}
	}
	for _, x := range []struct {
		e int
		q geom.Vector
	}{{along[0], a.Start()}, {along[1], b.Finish()}, {crease, m}} {
		reach := x.q.Minus(p).Dot(t.edgeAway(x.e, v))
		if x.e == crease {
			// The crease may run on back past the vertex, where the
			// fillet's end takes the corner away: the faces across grow
			// over it up to the crease carried on.
			reach = math.Abs(reach)
		}
		if reach < 10*geom.LengthEps || reach > t.straightLen(x.e)-10*geom.LengthEps {
			return nil, filletErr(p, "a radius this large is wider than a face at the edge's end")
		}
	}
	c := &filletCorner{kind: creaseCorner, at: map[[2]int]geom.Vector{}, long: -1, r: fe.r,
		faceCurves: map[int]srf.SBezier{across[0]: a, across[1]: b}, chain: []srf.SBezier{a, b}}
	for k, q := range [2]geom.Vector{a.Start(), b.Finish()} {
		c.at[[2]int{fe.e, fe.f[k]}] = q
		c.at[[2]int{along[k], fe.f[k]}] = q
		c.at[[2]int{along[k], across[k]}] = q
	}
	c.at[[2]int{crease, across[0]}] = m
	c.at[[2]int{crease, across[1]}] = m
	return c, nil
}

// Two opposite edges of such a corner rounded, the other two left sharp,
// as a saddle's two ridges: the two fillets meet in a mitre in the plane
// through the two sharp edges, each its surface carried on to that
// plane, as a crease end runs to the faces across. Where the fillets'
// tangent lines reach the sharp edges at one point each, both ends,
// their sections projected along their edges onto the plane, are one
// curve, which they share; where not, the mitre is no plane, and the
// corner is refused as before.

// creasePairOf is the two opposite rounded edges at v, each one's faces
// in the order of the sharp edges, sharp, they go on into the other's
// across.
func (t *Topology) creasePairOf(v int, edges []filletEdge, byEdge map[int]int) (fes [2]*filletEdge, sharp [2]int, ok bool) {
	vt := &t.Vertices[v]
	if len(vt.Edges) != 4 || len(t.facesAt(v)) != 4 {
		return fes, sharp, false
	}
	var round, plain []int
	for _, x := range vt.Edges {
		if !t.Straight(x) {
			return fes, sharp, false
		}
		if _, ok := byEdge[x]; ok {
			round = append(round, x)
		} else {
			plain = append(plain, x)
		}
	}
	if len(round) != 2 {
		return fes, sharp, false
	}
	for i, x := range round {
		fes[i] = &edges[byEdge[x]]
		if fes[i].circ || fes[i].varies() {
			return fes, sharp, false
		}
	}
	if sharedFace(fes[0], fes[1]) >= 0 {
		// Side by side, as a ridge and a valley: a vertex blend, not this.
		return fes, sharp, false
	}
	for _, f := range t.facesAt(v) {
		if _, ok := t.planarNormal(f); !ok {
			return fes, sharp, false
		}
	}
	sharp = [2]int{plain[0], plain[1]}
	for _, s := range sharp {
		// Each sharp edge between a face of one and a face of the other.
		sf := t.EdgeFaces(s)
		on := func(fe *filletEdge) bool {
			return sf[0] == fe.f[0] || sf[0] == fe.f[1] || sf[1] == fe.f[0] || sf[1] == fe.f[1]
		}
		if !on(fes[0]) || !on(fes[1]) {
			return fes, sharp, false
		}
	}
	return fes, sharp, true
}

// creasePairAt works out the mitre of the two opposite rounded edges at
// v (creasePairOf), or with flat the chamfers'.
func (t *Topology) creasePairAt(v int, edges []filletEdge, byEdge map[int]int, flat bool) (*filletCorner, error) {
	fes, sharp, ok := t.creasePairOf(v, edges, byEdge)
	if !ok {
		return nil, filletErr(t.Vertices[v].P, "the corner is not a pair of opposite edges rounded")
	}
	p := t.Vertices[v].P
	a, b := t.edgeAway(sharp[0], v), t.edgeAway(sharp[1], v)
	n := a.Cross(b)
	if n.Magnitude() < 1e-9 {
		return nil, filletErr(p, "the edges left sharp at the corner run on into one another")
	}
	n = n.WithMagnitude(1)
	var ends [2]srf.SBezier
	for i, fe := range fes {
		if math.Abs(n.Dot(fe.dir)) < 1e-6 {
			return nil, filletErr(p, "a fillet runs along the plane its mitre lies in")
		}
		sec := fe.section(p, flat)
		for j := 0; j <= sec.Degree; j++ {
			q := sec.Ctrl[j]
			sec.Ctrl[j] = q.Plus(fe.dir.ScaledBy(n.Dot(p.Minus(q)) / n.Dot(fe.dir)))
		}
		ends[i] = sec
	}
	// The two ends one curve, else the fillets' surfaces don't meet in
	// the plane.
	other := ends[1]
	if !other.Start().Equals(ends[0].Start()) {
		other.Reverse()
	}
	for _, u := range []float64{0, 0.25, 0.5, 0.75, 1} {
		if ends[0].PointAt(u).Minus(other.PointAt(u)).Magnitude() > geom.LengthEps {
			return nil, filletErr(p, "the two fillets' ends don't meet in one curve at the corner; round the other two edges too")
		}
	}
	c := &filletCorner{kind: creaseCorner, at: map[[2]int]geom.Vector{}, long: -1, r: fes[0].r,
		chain: []srf.SBezier{ends[0]}}
	for _, fe := range fes {
		margin := math.Max(fe.ss[0], fe.ss[1]) + 2*fe.r
		for k := range 2 {
			q := ends[0].Start()
			if fe.f[k] != t.faceHolding(q, fe.f) {
				q = ends[0].Finish()
			}
			if math.Abs(q.Minus(p).Dot(fe.dir)) > margin-geom.LengthEps {
				return nil, filletErr(p, "the fillet's end runs too far past the corner")
			}
			c.at[[2]int{fe.e, fe.f[k]}] = q
		}
	}
	for _, s := range sharp {
		var q geom.Vector
		var found bool
		for _, e := range []geom.Vector{ends[0].Start(), ends[0].Finish()} {
			off := e.Minus(p)
			w := t.edgeAway(s, v)
			if off.Minus(w.ScaledBy(off.Dot(w))).Magnitude() < 10*geom.LengthEps {
				q, found = e, true
			}
		}
		reach := q.Minus(p).Dot(t.edgeAway(s, v))
		if !found || reach < 10*geom.LengthEps || reach > t.straightLen(s)-10*geom.LengthEps {
			return nil, filletErr(p, "a radius this large is wider than a face at the edge's end")
		}
		for _, f := range t.EdgeFaces(s) {
			c.at[[2]int{s, f}] = q
		}
	}
	return c, nil
}

// faceHolding is the one of fs whose plane q lies in, of two flat faces.
func (t *Topology) faceHolding(q geom.Vector, fs [2]int) int {
	best, bd := fs[0], math.Inf(1)
	for _, f := range fs {
		n, _ := t.planarNormal(f)
		pts := t.Edges[t.CoEdges[t.Loops[t.Faces[f].Loops[0]].CoEdges[0]].Edge].Points
		if d := math.Abs(n.Dot(q.Minus(pts[0]))); d < bd {
			best, bd = f, d
		}
	}
	return best
}
