package topo

import (
	"math"
	"slices"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// Chamfers at a corner whose other edges stay sharp. Every face there is
// flat and every edge straight, and the edges are all convex, so the
// solid about the vertex is a convex cone, or all concave, so the space
// outside it is. Each chamfer is a plane, and the corner is where those
// planes cut the cone: the solid's cone, less the side of each plane the
// vertex lies on (a chamfer of a convex edge), or the outside's (one of
// a concave edge, which fills). That is what a chamfer's wedge cuts, and
// what OpenCASCADE's makeChamfer makes at a pyramid's apex, of any
// distances and any edges picked; a chamfer that reaches its corner
// alone (filletCornerAt's kind 1) and two above a third edge left sharp
// (chamferPairAt) are the same at a corner of three.
//
// Each face at the vertex is cut down to the part of its wedge every
// plane keeps, and each chamfer to the part of its plane the cone and
// the other planes keep. Each face then ends at the vertex in a chain
// of lines between its two edges' new ends, and each chamfer in a chain
// across it from one of its faces to the other.

// coneCorner is the kind of a corner of chamfers cut as planes cut the
// cone of the solid's faces.
const coneCorner = 6

// chamferConeTakes reports whether chamferConeAt can work out the
// corner at vertex v: every face there flat, every edge straight and
// between two of them, as many faces as edges, the edges all convex or
// all concave, and one or more of them chamfered.
func (t *Topology) chamferConeTakes(v int, edges []filletEdge, byEdge map[int]int) bool {
	_, err := t.coneEdges(v, edges, byEdge)
	return err == nil
}

// coneEdges are the edges at vertex v that chamferConeTakes takes, the
// sharp ones worked out as chamfers that cut nothing, or why it can't.
func (t *Topology) coneEdges(v int, edges []filletEdge, byEdge map[int]int) ([]*filletEdge, error) {
	p := t.Vertices[v].P
	xs := t.Vertices[v].Edges
	if len(xs) < 3 || len(t.facesAt(v)) != len(xs) {
		return nil, filletErr(p, "the corner's faces don't go once round it")
	}
	var out []*filletEdge
	picked := 0
	for _, x := range xs {
		if i, ok := byEdge[x]; ok {
			if edges[i].circ {
				return nil, filletErr(p, "a circular edge's chamfer meets other edges here")
			}
			out = append(out, &edges[i])
			picked++
			continue
		}
		fe, err := t.filletEdgeOf(x, 0, true)
		if err != nil {
			return nil, err
		}
		if fe.circ {
			return nil, filletErr(p, "a curved edge meets the chamfers here")
		}
		out = append(out, &fe)
	}
	if picked == 0 {
		return nil, filletErr(p, "no edge at the corner is chamfered")
	}
	for _, fe := range out {
		if fe.concave != out[0].concave {
			return nil, filletErr(p, "a convex edge's chamfer meets a concave one's here")
		}
	}
	for _, f := range t.facesAt(v) {
		if _, ok := t.planarNormal(f); !ok {
			return nil, filletErr(p, curvedEndMsg)
		}
	}
	return out, nil
}

// coneGeom is a corner of chamfers being cut from its cone: the vertex,
// at p, its edges, each one's direction away from it, the reach of the
// polygons cut, and the planes that cut them.
type coneGeom struct {
	p     geom.Vector
	v     int
	edges []*filletEdge
	away  []geom.Vector
	reach float64
	// The chamfers' planes, by edge, each with the sign of the side it
	// keeps; and each face's plane, with the side of the cone.
	cut   map[int]keptPlane
	faces map[int]keptPlane
}

// keptPlane is a plane and the side of it kept: points x with
// side·(n·x - d) ≥ 0.
type keptPlane struct {
	plane
	side float64
}

// chamferConeAt works out the corner at vertex v, which
// chamferConeTakes takes.
func (t *Topology) chamferConeAt(v int, edges []filletEdge, byEdge map[int]int) (*filletCorner, error) {
	all, err := t.coneEdges(v, edges, byEdge)
	if err != nil {
		return nil, err
	}
	g := &coneGeom{p: t.Vertices[v].P, v: v, edges: all, reach: math.Inf(1),
		cut: map[int]keptPlane{}, faces: map[int]keptPlane{}}
	inside := geom.Vector{}
	for _, fe := range all {
		d, l := fe.dir, t.Vertices[fe.v[1]].P.Minus(g.p).Magnitude()
		if fe.v[1] == v {
			d, l = d.ScaledBy(-1), t.Vertices[fe.v[0]].P.Minus(g.p).Magnitude()
		}
		g.away = append(g.away, d)
		g.reach = math.Min(g.reach, 0.9*l)
		inside = inside.Plus(d)
	}
	// Inside the cone: the edges' directions summed.
	inside = g.p.Plus(inside.WithMagnitude(1))
	for _, f := range t.facesAt(v) {
		n, _ := t.planarNormal(f)
		pl := plane{n, n.Dot(g.p)}
		g.faces[f] = keptPlane{pl, math.Copysign(1, n.Dot(inside)-pl.d)}
	}
	for i, fe := range all {
		if _, ok := byEdge[fe.e]; !ok {
			continue
		}
		pl := fe.chamferPlane(g.p)
		g.cut[i] = keptPlane{pl, -math.Copysign(1, pl.n.Dot(g.p)-pl.d)}
	}

	c := &filletCorner{kind: coneCorner, at: map[[2]int]geom.Vector{}, long: -1,
		ends: map[int][]geom.Vector{}, chains: map[int][]geom.Vector{}}
	var pool []geom.Vector
	snap := func(q geom.Vector) geom.Vector {
		for _, o := range pool {
			if o.Minus(q).Magnitude() <= 10*geom.LengthEps {
				return o
			}
		}
		pool = append(pool, q)
		return q
	}
	// Each face's chain, from its first edge's new end to its other's.
	for _, f := range t.facesAt(v) {
		ia, ib := g.edgesOf(f)
		if ia < 0 {
			return nil, filletErr(g.p, "a face at the corner doesn't have two of its edges there")
		}
		chain, err := g.faceChain(f, ia, ib)
		if err != nil {
			return nil, err
		}
		for k := range chain {
			chain[k] = snap(chain[k])
		}
		for _, end := range []struct {
			i int
			q geom.Vector
		}{{ia, chain[0]}, {ib, chain[len(chain)-1]}} {
			key := [2]int{all[end.i].e, f}
			c.at[key] = end.q
		}
		c.chains[f] = chain
	}
	// A sharp edge's end is one point on both its faces.
	for _, fe := range all {
		if _, ok := byEdge[fe.e]; ok {
			continue
		}
		a, b := c.at[[2]int{fe.e, fe.f[0]}], c.at[[2]int{fe.e, fe.f[1]}]
		if !a.Equals(b) {
			return nil, filletErr(g.p, "the chamfers cut an edge left sharp at two points")
		}
	}
	// Each chamfer's chain across its end.
	for i, fe := range all {
		if _, ok := byEdge[fe.e]; !ok {
			continue
		}
		chain, err := g.stripChain(i)
		if err != nil {
			return nil, err
		}
		for k := range chain {
			chain[k] = snap(chain[k])
		}
		if !chain[0].Equals(c.at[[2]int{fe.e, fe.f[0]}]) || !chain[len(chain)-1].Equals(c.at[[2]int{fe.e, fe.f[1]}]) {
			return nil, filletErr(g.p, "a chamfer's end at the corner doesn't meet its faces' ends")
		}
		if len(chain) < 2 {
			return nil, filletErr(g.p, "a chamfer at the corner ends in a point")
		}
		// The chamfer's plane, fe.surface, reaches its margin past the
		// vertex along the edge.
		margin := math.Max(fe.ss[0], fe.ss[1]) + 2*fe.r
		for _, q := range chain {
			if q.Minus(g.p).Dot(g.away[i]) < -0.9*margin {
				return nil, filletErr(g.p, "a chamfer's end reaches too far back past the corner")
			}
		}
		c.ends[fe.e] = chain
	}
	// Where a point of one chain lies along a line of another, as where
	// four planes meet, the line is split there, so that the faces share
	// their vertices.
	for f, chain := range c.chains {
		c.chains[f] = splitAt(chain, pool)
	}
	for e, chain := range c.ends {
		c.ends[e] = splitAt(chain, pool)
	}
	return c, nil
}

// edgesOf are the indices into g.edges of face f's two edges at the
// vertex, or -1.
func (g *coneGeom) edgesOf(f int) (int, int) {
	var in []int
	for i, fe := range g.edges {
		if fe.f[0] == f || fe.f[1] == f {
			in = append(in, i)
		}
	}
	if len(in) != 2 {
		return -1, -1
	}
	return in[0], in[1]
}

// faceChain is face f's boundary at the vertex, from edge ia's new end
// to ib's: the wedge between the two edges, out to the reach, cut by
// every chamfer's plane.
func (g *coneGeom) faceChain(f, ia, ib int) ([]geom.Vector, error) {
	da, db := g.away[ia], g.away[ib]
	if da.Cross(db).Magnitude() < 1e-9 {
		return nil, filletErr(g.p, "a face at the corner has its two edges there in a line")
	}
	far := g.p.Plus(da.ScaledBy(g.reach))
	poly := []geom.Vector{g.p, far, g.p.Plus(db.ScaledBy(g.reach))}
	for _, k := range g.cut {
		poly = clipPolygon(poly, k)
	}
	// The far line, from the reach along a to the reach along b.
	farN := db.Minus(da).Cross(da.Cross(db))
	onFar := func(q geom.Vector) bool {
		return math.Abs(q.Minus(far).Dot(farN.WithMagnitude(1))) <= 10*geom.LengthEps
	}
	chain, ok := nearChain(poly, onFar)
	if !ok {
		return nil, filletErr(g.p, "the chamfers cut a face at the corner past the end of an edge")
	}
	// The wedge's far side runs from a's side to b's, so the rest of it
	// runs back from b's.
	slices.Reverse(chain)
	// The chain starts along a's line, or its chamfer's on f, and ends
	// along b's.
	if !g.alongEdge(ia, f, chain[0], chain[1]) || !g.alongEdge(ib, f, chain[len(chain)-1], chain[len(chain)-2]) {
		return nil, filletErr(g.p, "the chamfers cut an edge at the corner away")
	}
	return chain[1 : len(chain)-1], nil
}

// alongEdge reports whether the line from a to b runs along edge i on
// face f: along the edge itself when it is sharp, else along its
// chamfer's line on f.
func (g *coneGeom) alongEdge(i, f int, a, b geom.Vector) bool {
	fe := g.edges[i]
	o := g.p
	if _, ok := g.cut[i]; ok {
		k := 0
		if fe.f[1] == f {
			k = 1
		}
		o = o.Plus(fe.u[k].ScaledBy(fe.ss[k]))
	}
	d := g.away[i]
	return a.DistanceToLine(o, d) <= 10*geom.LengthEps && b.DistanceToLine(o, d) <= 10*geom.LengthEps
}

// stripChain is chamfer i's end at the vertex, from its line on its
// first face to its line on the other: the strip between the lines,
// from the reach along the edge to as far past the vertex, cut by the
// cone's faces and the other chamfers' planes.
func (g *coneGeom) stripChain(i int) ([]geom.Vector, error) {
	fe := g.edges[i]
	d := g.away[i]
	l0 := g.p.Plus(fe.u[0].ScaledBy(fe.ss[0]))
	l1 := g.p.Plus(fe.u[1].ScaledBy(fe.ss[1]))
	poly := []geom.Vector{
		l0.Plus(d.ScaledBy(g.reach)), l0.Minus(d.ScaledBy(g.reach)),
		l1.Minus(d.ScaledBy(g.reach)), l1.Plus(d.ScaledBy(g.reach)),
	}
	for f, k := range g.faces {
		if f != fe.f[0] && f != fe.f[1] {
			poly = clipPolygon(poly, k)
		}
	}
	for j, k := range g.cut {
		if j != i {
			poly = clipPolygon(poly, k)
		}
	}
	far := g.p.Plus(d.ScaledBy(g.reach))
	onFar := func(q geom.Vector) bool { return math.Abs(q.Minus(far).Dot(d)) <= 10*geom.LengthEps }
	for _, q := range poly {
		if math.Abs(q.Minus(g.p).Dot(d)+g.reach) <= 10*geom.LengthEps {
			return nil, filletErr(g.p, "a chamfer at the corner isn't closed off by the faces round it")
		}
	}
	chain, ok := nearChain(poly, onFar)
	if !ok || !g.alongEdge(i, fe.f[0], chain[0], chain[1]) || !g.alongEdge(i, fe.f[1], chain[len(chain)-1], chain[len(chain)-2]) {
		return nil, filletErr(g.p, "the chamfers at the corner cut one another past the end of an edge")
	}
	return chain[1 : len(chain)-1], nil
}

// nearChain is the convex polygon poly's boundary away from its far
// side: from the vertex of the far side that comes last in poly's order
// round to the one that comes first, the far side's two ends included.
// False unless the far side, the points onFar, is two vertices one after
// the other.
func nearChain(poly []geom.Vector, onFar func(geom.Vector) bool) ([]geom.Vector, bool) {
	n := len(poly)
	if n < 3 {
		return nil, false
	}
	start := -1
	count := 0
	for i := range n {
		if onFar(poly[i]) {
			count++
			if onFar(poly[(i+1)%n]) && !onFar(poly[(i+2)%n]) {
				start = (i + 1) % n
			}
		}
	}
	if count != 2 || start < 0 {
		return nil, false
	}
	var out []geom.Vector
	for k := range n {
		out = append(out, poly[(start+k)%n])
	}
	return out, true
}

// clipPolygon is the convex polygon poly cut down to the side of k it
// keeps.
func clipPolygon(poly []geom.Vector, k keptPlane) []geom.Vector {
	const eps = 1e-12
	h := func(q geom.Vector) float64 { return k.side * (k.n.Dot(q) - k.d) }
	var out []geom.Vector
	for i, a := range poly {
		b := poly[(i+1)%len(poly)]
		ha, hb := h(a), h(b)
		if ha >= -eps {
			out = append(out, a)
		}
		if ha > eps && hb < -eps || ha < -eps && hb > eps {
			out = append(out, a.Plus(b.Minus(a).ScaledBy(ha/(ha-hb))))
		}
	}
	return out
}

// splitAt is the polyline chain with each of its lines split at the
// points of pool that lie inside it.
func splitAt(chain []geom.Vector, pool []geom.Vector) []geom.Vector {
	out := []geom.Vector{chain[0]}
	for i := 1; i < len(chain); i++ {
		a, b := chain[i-1], chain[i]
		ab := b.Minus(a)
		l := ab.Magnitude()
		var mid []float64
		var at []geom.Vector
		for _, q := range pool {
			s := q.Minus(a).Dot(ab) / (l * l)
			if s*l > 10*geom.LengthEps && (1-s)*l > 10*geom.LengthEps && q.DistanceToLine(a, ab) <= 10*geom.LengthEps {
				mid, at = append(mid, s), append(at, q)
			}
		}
		for len(mid) > 0 {
			j := 0
			for k := range mid {
				if mid[k] < mid[j] {
					j = k
				}
			}
			out = append(out, at[j])
			mid = append(mid[:j], mid[j+1:]...)
			at = append(at[:j], at[j+1:]...)
		}
		out = append(out, b)
	}
	return out
}

// orientedLines are the polyline pts as lines, each from the end of the
// one before, run from its end at p.
func orientedLines(pts []geom.Vector, p geom.Vector) []srf.SBezier {
	if !pts[0].Equals(p) {
		pts = slices.Clone(pts)
		slices.Reverse(pts)
	}
	var out []srf.SBezier
	for i := 1; i < len(pts); i++ {
		out = append(out, srf.SBezierFrom2(pts[i-1], pts[i]))
	}
	return out
}
