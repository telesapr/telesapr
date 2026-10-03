package topo

import (
	"math"
	"sort"

	"github.com/spacesolve/spacesolve/pkg/geom"
	"github.com/spacesolve/spacesolve/pkg/srf"
)

// Fillets of several edges at once, with one radius, as one edit: every
// face along a rounded edge, or at its end, is built again from its own
// loops with the corners there replaced, and the cylinders, and the
// spheres at corners, are put in. At a vertex, which must be a corner
// of three flat faces, the rounded edges meet in one of three ways:
//
//   - one edge: it ends on the third face, cut along the arc where the
//     cylinder meets it, as the end of a single fillet;
//   - two edges: their cylinders meet along an ellipse in the plane that
//     halves them, a mitre, when their tangent lines on the two other
//     faces reach the unrounded edge at one point;
//   - three edges: a sphere about the point r inside all three faces
//     (a face's tangent point is that centre moved r out along its normal),
//     which each cylinder meets along the arc where it ends.

// filletEdge is one edge to round and what follows from its faces.
type filletEdge struct {
	e      int
	v      [2]int // its vertices, from start to end
	f      [2]int // its faces
	n      [2]geom.Vector
	u      [2]geom.Vector // into each face, square to the edge
	dir    geom.Vector    // from v[0] to v[1]
	s, w   float64        // the tangent lines' distance from the edge, the arc's weight
	origin uint32
}

// filletCorner is how the rounded edges meet at a vertex.
type filletCorner struct {
	kind int // the number of rounded edges there
	// at is the point an edge's end moves to in a face.
	at map[[2]int]geom.Vector
	// For one edge: the face it ends on.
	end int
	// For three: the sphere's centre.
	centre geom.Vector
	// For two: the mitre's plane, through centre.
	mitreN geom.Vector
}

// FilletNetwork rounds the edges segs (each a whole straight edge
// between two flat faces that sticks out) with radius r. The cylinders
// get origins origin, origin+1, ..., and the spheres the origins after
// them.
func FilletNetwork(sh *srf.SShell, segs [][2]geom.Vector, r float64, origin uint32) error {
	if len(segs) == 0 {
		return filletErr(geom.Vector{}, "there are no edges to fillet")
	}
	if !(r > 0) || math.IsInf(r, 0) {
		return filletErr(segs[0][0], "a fillet's radius must be more than zero")
	}
	t, err := Build(sh)
	if err != nil {
		return filletErr(segs[0][0], "the solid's topology is broken: %v", err)
	}
	edges := make([]filletEdge, 0, len(segs))
	byEdge := map[int]int{}
	for i, sg := range segs {
		fe, err := t.filletEdgeOf(sg[0], sg[1], r)
		if err != nil {
			return err
		}
		if _, dup := byEdge[fe.e]; dup {
			return filletErr(sg[0], "an edge is given twice")
		}
		fe.origin = origin + uint32(i)
		byEdge[fe.e] = len(edges)
		edges = append(edges, fe)
	}

	// How the edges meet at each vertex they touch.
	corners := map[int]*filletCorner{}
	for _, fe := range edges {
		for _, v := range fe.v {
			if corners[v] != nil {
				continue
			}
			c, err := t.filletCornerAt(v, edges, byEdge, r)
			if err != nil {
				return err
			}
			corners[v] = c
		}
	}

	// The faces built again: those along the edges, and those the edges
	// end on.
	redo := map[int]bool{}
	for _, fe := range edges {
		redo[fe.f[0]], redo[fe.f[1]] = true, true
	}
	for _, c := range corners {
		if c.kind == 1 {
			redo[c.end] = true
		}
	}
	faces := make([]int, 0, len(redo))
	for f := range redo {
		faces = append(faces, f)
	}
	sort.Ints(faces)

	type rebuilt struct {
		s     srf.SSurface
		loops [][]srf.SBezier
	}
	var again []rebuilt
	for _, f := range faces {
		loops, err := t.filletLoops(f, edges, byEdge, corners)
		if err != nil {
			return err
		}
		again = append(again, rebuilt{*t.Surface(f), loops})
	}
	color := t.Surface(edges[0].f[0]).Color

	// The cylinders' loops and the spheres', while t still describes the
	// solid.
	type patch struct {
		s    srf.SSurface
		loop []srf.SBezier
		at   geom.Vector
	}
	var patches []patch
	for _, fe := range edges {
		a, b := t.Vertices[fe.v[0]].P, t.Vertices[fe.v[1]].P
		ca, cb := corners[fe.v[0]], corners[fe.v[1]]
		start := [2]geom.Vector{ca.at[[2]int{fe.e, fe.f[0]}], ca.at[[2]int{fe.e, fe.f[1]}]}
		end := [2]geom.Vector{cb.at[[2]int{fe.e, fe.f[0]}], cb.at[[2]int{fe.e, fe.f[1]}]}
		for k := 0; k < 2; k++ {
			if end[k].Minus(start[k]).Dot(fe.dir) < 10*geom.LengthEps {
				return filletErr(a.Plus(b).ScaledBy(0.5), "the edge is too short for a fillet this large")
			}
		}
		endArc := fe.connector(t, fe.v[1], cb, end[0], end[1])
		startArc := fe.connector(t, fe.v[0], ca, start[1], start[0])
		margin := fe.s + 2*r
		l := b.Minus(a).Magnitude()
		p0 := a.Minus(fe.dir.ScaledBy(margin))
		section := arc(p0.Plus(fe.u[0].ScaledBy(fe.s)), p0, p0.Plus(fe.u[1].ScaledBy(fe.s)), fe.w)
		cyl := srf.FromExtrusionOf(&section, geom.Vector{}, fe.dir.ScaledBy(l+2*margin))
		cyl.Color = color
		cyl.Origin = fe.origin
		patches = append(patches, patch{cyl, []srf.SBezier{
			srf.SBezierFrom2(start[0], end[0]), endArc,
			srf.SBezierFrom2(end[1], start[1]), startArc,
		}, a.Plus(b).ScaledBy(0.5)})
	}
	next := origin + uint32(len(edges))
	var vs []int
	for v := range corners {
		vs = append(vs, v)
	}
	sort.Ints(vs)
	for _, v := range vs {
		c := corners[v]
		if c.kind != 3 {
			continue
		}
		var loop []srf.SBezier
		for _, x := range t.Vertices[v].Edges {
			fe := &edges[byEdge[x]]
			loop = append(loop, fe.connector(t, v, c, c.at[[2]int{x, fe.f[0]}], c.at[[2]int{x, fe.f[1]}]))
		}
		loop, err := chainCurves(loop)
		if err != nil {
			return filletErr(t.Vertices[v].P, "the corner's arcs don't close: %v", err)
		}
		sphere := spherePatch(c.centre, r, loop)
		sphere.Color = color
		sphere.Origin = next
		next++
		patches = append(patches, patch{sphere, loop, t.Vertices[v].P})
	}

	RemoveFaces(sh, t, faces)
	for _, rb := range again {
		if _, err := AddFace(sh, rb.s, rb.loops); err != nil {
			return filletErr(rb.loops[0][0].Start(), "a face can't be built again: %v", err)
		}
	}
	for _, p := range patches {
		if _, err := AddFace(sh, p.s, [][]srf.SBezier{p.loop}); err != nil {
			return filletErr(p.at, "the fillet's face can't be put in: %v", err)
		}
	}
	if t, err = Build(sh); err != nil {
		return filletErr(segs[0][0], "%v", err)
	}
	if probs := t.Validate(); len(probs) > 0 {
		return filletErr(probs[0].At, "the fillet leaves the solid open: %s", probs[0].Msg)
	}
	return nil
}

// filletEdgeOf checks the edge from a to b and works out its fillet.
func (t *Topology) filletEdgeOf(a, b geom.Vector, r float64) (filletEdge, error) {
	var fe filletEdge
	m := a.Plus(b).ScaledBy(0.5)
	e := t.edgeAtPoint(m)
	if e < 0 {
		return fe, filletErr(m, "no edge of the solid runs there")
	}
	va, vb := t.vertexNear(a), t.vertexNear(b)
	ed := &t.Edges[e]
	whole := ed.V[0] == va && ed.V[1] == vb || ed.V[0] == vb && ed.V[1] == va
	if va < 0 || vb < 0 || !whole {
		return fe, filletErr(m, "the segment is not a whole edge of the solid")
	}
	if !t.Straight(e) {
		return fe, filletErr(m, "the edge is curved: a fillet here rounds straight edges")
	}
	fs := t.EdgeFaces(e)
	if fs[0] < 0 || fs[1] < 0 || len(ed.CoEdges) != 2 {
		return fe, filletErr(m, "the edge is not between two faces")
	}
	n1, ok1 := t.planarNormal(fs[0])
	n2, ok2 := t.planarNormal(fs[1])
	if !ok1 || !ok2 {
		return fe, filletErr(m, "a face at the edge is curved: a fillet here rounds edges between flat faces")
	}
	// The edge runs from its first vertex to its second.
	pa, pb := t.Vertices[ed.V[0]].P, t.Vertices[ed.V[1]].P
	u1 := t.intoFace(fs[0], pa, pb, n1)
	u2 := t.intoFace(fs[1], pa, pb, n2)
	if u1.Dot(n2) > -1e-9 || u2.Dot(n1) > -1e-9 {
		return fe, filletErr(m, "the edge is concave or flat: a fillet here rounds edges that stick out")
	}
	beta := math.Acos(math.Max(-1, math.Min(1, u1.Dot(u2))))
	fe = filletEdge{
		e: e, v: ed.V, f: fs, n: [2]geom.Vector{n1, n2}, u: [2]geom.Vector{u1, u2},
		dir: pb.Minus(pa).WithMagnitude(1), s: r / math.Tan(beta/2), w: math.Sin(beta / 2),
	}
	return fe, nil
}

// facesAt are the faces round vertex v, each once.
func (t *Topology) facesAt(v int) []int {
	var out []int
	seen := map[int]bool{}
	for _, x := range t.Vertices[v].Edges {
		for _, f := range t.EdgeFaces(x) {
			if f >= 0 && !seen[f] {
				seen[f] = true
				out = append(out, f)
			}
		}
	}
	return out
}

// filletCornerAt works out how the rounded edges meet at vertex v.
func (t *Topology) filletCornerAt(v int, edges []filletEdge, byEdge map[int]int, r float64) (*filletCorner, error) {
	p := t.Vertices[v].P
	if len(t.Vertices[v].Edges) != 3 || len(t.facesAt(v)) != 3 {
		return nil, filletErr(p, "the edge ends where %d edges meet; a fillet's end needs a corner of three faces", len(t.Vertices[v].Edges))
	}
	for _, f := range t.facesAt(v) {
		if _, ok := t.planarNormal(f); !ok {
			return nil, filletErr(p, "a face at the edge's end is curved")
		}
	}
	var round, plain []int
	for _, x := range t.Vertices[v].Edges {
		if _, ok := byEdge[x]; ok {
			round = append(round, x)
		} else {
			plain = append(plain, x)
		}
	}
	c := &filletCorner{kind: len(round), at: map[[2]int]geom.Vector{}}
	switch c.kind {
	case 1:
		fe := &edges[byEdge[round[0]]]
		along, end, err := t.endEdges(fe.e, v, fe.f)
		if err != nil {
			return nil, err
		}
		c.end = end
		for k := 0; k < 2; k++ {
			q, err := t.pointAlong(along[k], v, fe.u[k], fe.s)
			if err != nil {
				return nil, err
			}
			c.at[[2]int{fe.e, fe.f[k]}] = q
			c.at[[2]int{along[k], fe.f[k]}] = q
			c.at[[2]int{along[k], end}] = q
		}
	case 3:
		fs := t.facesAt(v)
		var n [3]geom.Vector
		var d [3]float64
		for i, f := range fs {
			n[i], _ = t.planarNormal(f)
			d[i] = n[i].Dot(p) - r
		}
		if math.Abs(n[0].Dot(n[1].Cross(n[2]))) < 1e-9 {
			return nil, filletErr(p, "the faces at the corner don't meet in a point")
		}
		c.centre = meet3(n[0], d[0], n[1], d[1], n[2], d[2])
		for i, f := range fs {
			tp := c.centre.Plus(n[i].ScaledBy(r))
			for _, x := range t.Vertices[v].Edges {
				efs := t.EdgeFaces(x)
				if efs[0] == f || efs[1] == f {
					c.at[[2]int{x, f}] = tp
				}
			}
		}
	case 2:
		a, b := &edges[byEdge[round[0]]], &edges[byEdge[round[1]]]
		z := plain[0]
		top := -1
		for _, f := range a.f {
			if f == b.f[0] || f == b.f[1] {
				top = f
			}
		}
		if top < 0 {
			return nil, filletErr(p, "two rounded edges meet here without a face between them")
		}
		// The tangent line of each edge on its other face reaches z.
		var q [2]geom.Vector
		var side [2]int
		for i, fe := range []*filletEdge{a, b} {
			k := 0
			if fe.f[0] == top {
				k = 1
			}
			side[i] = fe.f[k]
			var err error
			if q[i], err = t.pointAlong(z, v, fe.u[k], fe.s); err != nil {
				return nil, err
			}
		}
		if !q[0].Equals(q[1]) {
			return nil, filletErr(p, "two fillets meet at a corner whose faces don't lie alike about them; a mitre needs them to")
		}
		nt, _ := t.planarNormal(top)
		n1, _ := t.planarNormal(side[0])
		n2, _ := t.planarNormal(side[1])
		c.centre = meet3(nt, nt.Dot(p)-r, n1, n1.Dot(p)-r, n2, n2.Dot(p)-r)
		tt := c.centre.Plus(nt.ScaledBy(r))
		// The plane through the centre that halves the two cylinders and
		// holds the mitre's ends.
		m := q[0].Minus(c.centre).Cross(tt.Minus(c.centre))
		if m.Magnitude() < 1e-12 {
			return nil, filletErr(p, "the mitre between the two fillets is degenerate")
		}
		c.mitreN = m.WithMagnitude(1)
		c.at[[2]int{a.e, top}], c.at[[2]int{b.e, top}] = tt, tt
		c.at[[2]int{a.e, side[0]}], c.at[[2]int{z, side[0]}] = q[0], q[0]
		c.at[[2]int{b.e, side[1]}], c.at[[2]int{z, side[1]}] = q[1], q[1]
	}
	return c, nil
}

// connector is the curve across the edge's cylinder at vertex v, from
// p to q: the arc it ends along.
func (fe *filletEdge) connector(t *Topology, v int, c *filletCorner, p, q geom.Vector) srf.SBezier {
	vp := t.Vertices[v].P
	// The cross section's control point is where the section's plane
	// meets the edge; across, the arc between the tangent points.
	switch c.kind {
	case 1:
		// Projected along the edge onto the face it ends on, its control
		// point the vertex.
		return arc(p, vp, q, fe.w)
	case 3:
		x := vp.Plus(fe.dir.ScaledBy(c.centre.Minus(vp).Dot(fe.dir)))
		return arc(p, x, q, fe.w)
	}
	// A mitre: the section at the centre, projected along the edge onto
	// the mitre's plane.
	x := vp.Plus(fe.dir.ScaledBy(c.centre.Minus(vp).Dot(fe.dir)))
	onto := func(a geom.Vector) geom.Vector {
		k := c.centre.Minus(a).Dot(c.mitreN) / fe.dir.Dot(c.mitreN)
		return a.Plus(fe.dir.ScaledBy(k))
	}
	sb := arc(p, onto(x), q, fe.w)
	sb.Ctrl[0], sb.Ctrl[2] = p, q
	return sb
}

// filletLoops are face f's loops with the corners the fillets reach
// moved: each co-edge runs between the points its ends move to, a
// rounded edge along its tangent line, and a face a fillet ends on
// takes the arc between the two points it has at that vertex.
func (t *Topology) filletLoops(f int, edges []filletEdge, byEdge map[int]int, corners map[int]*filletCorner) ([][]srf.SBezier, error) {
	point := func(x, v int) geom.Vector {
		if c := corners[v]; c != nil {
			if q, ok := c.at[[2]int{x, f}]; ok {
				return q
			}
		}
		return t.Vertices[v].P
	}
	var loops [][]srf.SBezier
	for _, l := range t.outerFirst(f) {
		ces := t.Loops[l].CoEdges
		var loop []srf.SBezier
		for i, ce := range ces {
			x := t.CoEdges[ce].Edge
			va, vb := t.Start(ce), t.End(ce)
			a, b := point(x, va), point(x, vb)
			moved := !a.Equals(t.Vertices[va].P) || !b.Equals(t.Vertices[vb].P)
			_, round := byEdge[x]
			switch {
			case round || t.Straight(x):
				loop = append(loop, srf.SBezierFrom2(a, b))
			case moved:
				return nil, filletErr(a, "a curved edge of a face at the fillet would have to be cut")
			default:
				sb, err := t.edgeCurve(ce)
				if err != nil {
					return nil, err
				}
				loop = append(loop, sb)
			}
			// At the vertex the co-edge ends at, an arc when the face is
			// one a fillet ends on.
			nx := t.CoEdges[ces[(i+1)%len(ces)]].Edge
			if c := corners[vb]; c != nil && c.kind == 1 && c.end == f {
				p, q := point(x, vb), point(nx, vb)
				for _, fe := range edges {
					if fe.v[0] == vb || fe.v[1] == vb {
						loop = append(loop, fe.connector(t, vb, c, p, q))
						break
					}
				}
			}
		}
		loops = append(loops, loop)
	}
	return loops, nil
}

// edgeCurve is the exact curve of co-edge ce, run its way: the shell's
// curve, when it spans the edge.
func (t *Topology) edgeCurve(ce int) (srf.SBezier, error) {
	ed := &t.Edges[t.CoEdges[ce].Edge]
	a, b := t.Vertices[t.Start(ce)].P, t.Vertices[t.End(ce)].P
	c := t.shell.FindCurve(ed.Curve)
	if c == nil || !c.IsExact {
		return srf.SBezier{}, filletErr(a, "a curved edge of a face at the fillet has no exact curve")
	}
	sb := c.Exact
	switch {
	case sb.Start().Equals(a) && sb.Finish().Equals(b):
	case sb.Start().Equals(b) && sb.Finish().Equals(a):
		sb.Reverse()
	default:
		return srf.SBezier{}, filletErr(a, "a curved edge of a face at the fillet is part of a longer curve")
	}
	return sb, nil
}

// chainCurves orders curves into a closed loop, reversing those that
// run the other way.
func chainCurves(cs []srf.SBezier) ([]srf.SBezier, error) {
	out := []srf.SBezier{cs[0]}
	used := make([]bool, len(cs))
	used[0] = true
	for len(out) < len(cs) {
		end := out[len(out)-1].Finish()
		found := false
		for i, c := range cs {
			if used[i] {
				continue
			}
			if c.Start().Equals(end) {
				out, used[i], found = append(out, c), true, true
				break
			}
			if c.Finish().Equals(end) {
				c.Reverse()
				out, used[i], found = append(out, c), true, true
				break
			}
		}
		if !found {
			return nil, filletErr(end, "no arc goes on from here")
		}
	}
	if !out[len(out)-1].Finish().Equals(out[0].Start()) {
		return nil, filletErr(out[0].Start(), "the arcs don't close")
	}
	return out, nil
}

// spherePatch is a patch of the sphere about c of radius r over the
// spherical triangle the loop bounds: a surface of revolution about an
// axis through c square to the triangle's middle, so that neither pole
// lies on it, covering the triangle with a margin.
func spherePatch(c geom.Vector, r float64, loop []srf.SBezier) srf.SSurface {
	var pts []geom.Vector
	for _, sb := range loop {
		for k := 0; k < 32; k++ {
			pts = append(pts, sb.PointAt(float64(k)/32).Minus(c).WithMagnitude(1))
		}
	}
	var mid geom.Vector
	for _, p := range pts {
		mid = mid.Plus(p)
	}
	mid = mid.WithMagnitude(1)
	axis := mid.NormalDir(0).WithMagnitude(1)
	e0 := mid.Minus(axis.ScaledBy(mid.Dot(axis))).WithMagnitude(1)
	e1 := axis.Cross(e0)
	// The triangle's range of angle from the axis and about it.
	th0, th1 := math.Inf(1), math.Inf(-1)
	ph0, ph1 := math.Inf(1), math.Inf(-1)
	for _, p := range pts {
		th := math.Acos(math.Max(-1, math.Min(1, p.Dot(axis))))
		ph := math.Atan2(p.Dot(e1), p.Dot(e0))
		th0, th1 = math.Min(th0, th), math.Max(th1, th)
		ph0, ph1 = math.Min(ph0, ph), math.Max(ph1, ph)
	}
	const margin = 0.05
	th0, th1 = math.Max(th0-margin, 1e-3), math.Min(th1+margin, math.Pi-1e-3)
	ph0, ph1 = ph0-margin, ph1+margin
	// The meridian at angle 0 about the axis, from th0 to th1.
	at := func(th float64) geom.Vector {
		return c.Plus(axis.ScaledBy(r * math.Cos(th))).Plus(e0.ScaledBy(r * math.Sin(th)))
	}
	half := (th1 - th0) / 2
	ctrl := c.Plus(axis.ScaledBy(r * math.Cos(th0+half) / math.Cos(half))).
		Plus(e0.ScaledBy(r * math.Sin(th0+half) / math.Cos(half)))
	meridian := arc(at(th0), ctrl, at(th1), math.Cos(half))
	// RotatedAbout turns e0 towards e1 for a positive angle about axis.
	sign := 1.0
	if e0.RotatedAroundAxis(axis, 0.1).Dot(e1) < 0 {
		sign = -1
	}
	return srf.FromRevolutionOf(&meridian, c, axis, sign*ph0, sign*ph1, 0, 0)
}
