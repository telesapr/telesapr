package topo

import (
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// chamferPairAt works out the corner where two chamfered edges a and b,
// sharing the face top, meet above a third edge z left sharp, between
// their side faces side. Each chamfer is a plane, so the corner is where
// planes meet, whatever the distances: the two chamfers meet along their
// mitre line, from tt on top to where it reaches a side face. q are
// where each chamfer's line on its side face meets z. Where they are one
// point the mitre runs to it. Else the chamfer that reaches further down
// z, the long one, goes on past the mitre along the line where it cuts
// the other's side face, to its own q: as at a box's corner of chamfers
// 1 and 2 deep, or at a rib's foot, where the side face grows out to it
// (planeUnder) as a fillet's does at an uneven mitre (unevenMitre).
func (t *Topology) chamferPairAt(c *filletCorner, v int, a, b *filletEdge, z, top int, side [2]int, q [2]geom.Vector) error {
	p := t.Vertices[v].P
	var pl [2]plane
	for i, fe := range []*filletEdge{a, b} {
		pl[i] = fe.chamferPlane(p)
	}
	nt, _ := t.planarNormal(top)
	tt, ok := meetPlanes(pl[0], pl[1], plane{nt, nt.Dot(p)})
	if !ok {
		return filletErr(p, "the two chamfers meet along a line that misses the face they share")
	}
	c.at[[2]int{a.e, top}], c.at[[2]int{b.e, top}] = tt, tt
	if q[0].Minus(q[1]).Magnitude() <= geom.LengthEps {
		c.at[[2]int{a.e, side[0]}], c.at[[2]int{z, side[0]}] = q[0], q[0]
		c.at[[2]int{b.e, side[1]}], c.at[[2]int{z, side[1]}] = q[0], q[0]
		return nil
	}
	// The long chamfer, il, reaches further from the vertex along z.
	il, is := 0, 1
	if q[1].Minus(p).Magnitude() > q[0].Minus(p).Magnitude() {
		il, is = 1, 0
	}
	fes := [2]*filletEdge{a, b}
	ns, _ := t.planarNormal(side[is])
	l, ok := meetPlanes(pl[0], pl[1], plane{ns, ns.Dot(p)})
	if !ok || l.Minus(tt).Magnitude() <= 10*geom.LengthEps || l.Minus(q[il]).Magnitude() <= 10*geom.LengthEps {
		return filletErr(p, "the mitre between the two chamfers is degenerate")
	}
	c.at[[2]int{fes[is].e, side[is]}] = l
	c.at[[2]int{fes[il].e, side[il]}] = q[il]
	c.at[[2]int{z, side[0]}], c.at[[2]int{z, side[1]}] = q[il], q[il]
	c.long, c.cutFace = fes[il].e, side[is]
	c.shared = srf.SBezierFrom2(tt, l)
	c.conic = srf.SBezierFrom2(l, q[il])
	return nil
}

// plane is the plane of points x with n·x = d.
type plane struct {
	n geom.Vector
	d float64
}

// chamferPlane is the plane of the straight edge's chamfer, through its
// lines on both faces; p is a point on the edge.
func (fe *filletEdge) chamferPlane(p geom.Vector) plane {
	a := p.Plus(fe.u[0].ScaledBy(fe.ss[0]))
	b := p.Plus(fe.u[1].ScaledBy(fe.ss[1]))
	n := fe.dir.Cross(b.Minus(a)).WithMagnitude(1)
	return plane{n, n.Dot(a)}
}

// meetPlanes is the point where three planes meet, false where two of
// them are near parallel.
func meetPlanes(a, b, c plane) (geom.Vector, bool) {
	if det := a.n.Dot(b.n.Cross(c.n)); det > -1e-9 && det < 1e-9 {
		return geom.Vector{}, false
	}
	return meet3(a.n, a.d, b.n, b.d, c.n, c.d), true
}
