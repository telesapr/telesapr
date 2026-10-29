package topo

import (
	"math"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// A convex and a concave chamfer at a corner of three flat faces whose
// third edge stays sharp, as a ledge's top edge with the concave upright
// below it, or a rib's upright with one of its foot edges. The two
// strips share a face, mid, where their lines cross, at p0; each strip's
// line on its other face meets the sharp edge's line, one past the
// vertex, along the edge, the other behind it. The strip whose line
// meets the edge, the runner, ends across from p0 to that point, q; the
// other is cut square to its edge through p0, ending at a0 on its other
// face. That face, which the sharp edge bounds, takes a parabola from a0
// to q, leaving a0 along the strip's line and reaching q along the edge,
// through where the two lines meet behind the vertex, k; and the hole
// between the two ends and the parabola is filled by the hyperbolic
// paraboloid through the two ends' lines and the parabola. Where the
// walls meet at an obtuse angle k may lie level with a0 or past it, and
// the parabola, which becomes the line from a0 to q as k reaches a0, is
// that line, the hole a triangle.
//
// OpenCASCADE's makeChamfer has the same corners and fills the hole with
// a plate of degree 8, its side on the face leaving a0 and reaching q as
// the parabola does but bulging less (TestChamferCornerOracle). Before,
// the corner was made in stages, the convex chamfer first, its plane
// running on over the concave one's fill.

// chamferMixedPairTakes reports whether the corner at vertex v is one
// chamferMixedPairAt works out: three straight edges between three flat
// faces, two of them chamfered, one convex and one concave.
func (t *Topology) chamferMixedPairTakes(v int, edges []filletEdge, byEdge map[int]int) bool {
	xs := t.Vertices[v].Edges
	if len(xs) != 3 || len(t.facesAt(v)) != 3 {
		return false
	}
	var kinds [2]int
	for _, x := range xs {
		if j, ok := byEdge[x]; ok {
			if edges[j].circ {
				return false
			}
			kinds[b2i(edges[j].concave)]++
		} else if !t.Straight(x) {
			return false
		}
	}
	if kinds != [2]int{1, 1} {
		return false
	}
	for _, f := range t.facesAt(v) {
		if _, ok := t.planarNormal(f); !ok {
			return false
		}
	}
	return true
}

// chamferMixedPairAt works out the corner at vertex v, which
// chamferMixedPairTakes takes.
func (t *Topology) chamferMixedPairAt(v int, edges []filletEdge, byEdge map[int]int) (*filletCorner, error) {
	p := t.Vertices[v].P
	var round []*filletEdge
	z := -1
	for _, x := range t.Vertices[v].Edges {
		if j, ok := byEdge[x]; ok {
			round = append(round, &edges[j])
		} else {
			z = x
		}
	}
	mid := -1
	for _, f := range round[0].f {
		if f == round[1].f[0] || f == round[1].f[1] {
			mid = f
		}
	}
	if mid < 0 {
		return nil, filletErr(p, "the two chamfers meet here without a face between them")
	}
	g := &chamferGeom{p: p, v: v}
	// k is the index of fe's other face; line is its line on face f.
	other := func(fe *filletEdge) int {
		if fe.f[0] == mid {
			return 1
		}
		return 0
	}
	line := func(fe *filletEdge, f int) geom.Vector {
		return g.base(chamferStrip{fe: fe}, f)
	}
	p0, ok := lineMeet(line(round[0], mid), g.away(round[0]), line(round[1], mid), g.away(round[1]))
	if !ok {
		return nil, filletErr(p, "two chamfers at the corner run along one another")
	}
	for _, fe := range round {
		if p0.Minus(p).Dot(g.away(fe)) < 10*geom.LengthEps {
			return nil, filletErr(p, "the two chamfers' lines on the face they share cross outside it")
		}
	}
	// Each strip's line on its other face meets the sharp edge: past the
	// vertex for the runner, behind it for the one cut square.
	var meet [2]geom.Vector
	var past [2]bool
	for i, fe := range round {
		k := other(fe)
		q, err := t.pointAlong(z, v, fe.u[k], fe.ss[k], true)
		if err != nil {
			return nil, err
		}
		meet[i], past[i] = q, q.Minus(p).Dot(t.awayOn(z, v)) > 10*geom.LengthEps
	}
	if past[0] == past[1] {
		return nil, filletErr(p, "the chamfers' lines on the faces at the sharp edge both meet it on one side of the corner")
	}
	ir, is := 0, 1
	if past[1] {
		ir, is = 1, 0
	}
	run, sq := round[ir], round[is]
	q, kp := meet[ir], meet[is]
	fr, fs := run.f[other(run)], sq.f[other(sq)]
	a0 := line(sq, fs).Plus(g.away(sq).ScaledBy(p0.Minus(p).Dot(g.away(sq))))

	c := &filletCorner{kind: setbackCorner, at: map[[2]int]geom.Vector{}, long: -1,
		arcs: map[int]srf.SBezier{}, faceCurves: map[int]srf.SBezier{}}
	c.at[[2]int{run.e, mid}], c.at[[2]int{sq.e, mid}] = p0, p0
	c.at[[2]int{run.e, fr}], c.at[[2]int{z, fr}], c.at[[2]int{z, fs}] = q, q, q
	c.at[[2]int{sq.e, fs}] = a0
	c.arcs[run.e] = srf.SBezierFrom2(p0, q)
	c.arcs[sq.e] = srf.SBezierFrom2(a0, p0)
	if kp.Minus(a0).Dot(g.away(sq)) > -10*geom.LengthEps {
		// The lines meet level with a0 or past it, as where the walls
		// meet at an obtuse angle, and the parabola would leave a0 away
		// from the corner: it is the line from a0 to q, which it becomes
		// as they meet at a0, and the hole a triangle.
		c.faceCurves[fs] = srf.SBezierFrom2(q, a0)
		hole := []srf.SBezier{c.arcs[run.e], c.faceCurves[fs], c.arcs[sq.e]}
		plates, err := chamferPlates(hole)
		if err != nil {
			return nil, filletErr(p, "the corner can't be filled: %v", err)
		}
		c.plates = plates
		return c, nil
	}
	curve := srf.SBezierFrom3(q, kp, a0)
	c.faceCurves[fs] = curve
	nm := newellOf([]geom.Vector{p0, q, a0})
	if nm.Magnitude() < 1e-12 {
		return nil, filletErr(p, "the hole at the corner has no area")
	}
	s, err := threeSidedPatch(p0, a0, kp, q, nm.WithMagnitude(1))
	if err != nil {
		return nil, filletErr(p, "the corner can't be filled: %v", err)
	}
	c.plates = []cornerPlate{{s: s, loop: []srf.SBezier{c.arcs[run.e], curve, c.arcs[sq.e]}}}
	return c, nil
}

// awayOn is the direction along edge x away from its vertex v.
func (t *Topology) awayOn(x, v int) geom.Vector {
	ed := &t.Edges[x]
	a, b := t.Vertices[ed.V[0]].P, t.Vertices[ed.V[1]].P
	if ed.V[1] == v {
		a, b = b, a
	}
	return b.Minus(a).WithMagnitude(1)
}

// threeSidedPatch is the hyperbolic paraboloid through the lines from p0
// to a and to b and the parabola from a to b through k: the bilinear
// patch of corners p0, a, 2k - p0 and b, whose diagonal from a to b is
// that parabola. Over the triangle it fills its normal must keep the way
// nm faces, or the other way all over it.
func threeSidedPatch(p0, a, k, b, nm geom.Vector) (srf.SSurface, error) {
	pts := []geom.Vector{p0, a, k.ScaledBy(2).Minus(p0), b}
	sign := 0.0
	const g = 8
	for i := 0; i <= g; i++ {
		for j := 0; i+j <= g; j++ {
			s, t := float64(i)/g, float64(j)/g
			du := pts[1].Minus(pts[0]).ScaledBy(1 - t).Plus(pts[2].Minus(pts[3]).ScaledBy(t))
			dv := pts[3].Minus(pts[0]).ScaledBy(1 - s).Plus(pts[2].Minus(pts[1]).ScaledBy(s))
			d := du.Cross(dv).Dot(nm)
			if math.Abs(d) < 1e-12 || sign != 0 && d*sign < 0 {
				return srf.SSurface{}, filletErr(p0, "its patch folds")
			}
			sign = d
		}
	}
	return bilinearSurface(pts), nil
}
