package topo

import (
	"math"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// A circular edge between two straight picks of one kind and different
// radii, as the arc an earlier stage's fillet leaves in a ledge's top
// where its two top edges are rounded at 1 and 2, takes a radius that
// goes from one pick's to the other's along it: the cross section at
// each point is the one of that point's radius, so the rounded face
// stays tangent to both faces, and the radius's law is a cubic whose
// slope is nothing at both ends, so it meets each pick's cylinder along
// its cross section, tangent. Every face at such an edge is ruled in the
// half plane through the axis (a plane, a cylinder or a cone), so the
// cross section of radius ρ is that of radius 1 scaled by ρ about the
// edge's point, and the face is a rational patch of degree 5 along the
// arc (the arc's 2 and the law's 3) and the section's across.

// varyDeg is the degree along the arc of a rounded face whose radius
// varies.
const varyDeg = 5

// rAt is the edge's radius at its vertex v.
func (fe *filletEdge) rAt(v int) float64 {
	if fe.r1 != 0 && v == fe.v[1] {
		return fe.r1
	}
	return fe.r
}

// ssAt is the tangent curve's distance from the edge on face k at the
// edge's vertex v.
func (fe *filletEdge) ssAt(k, v int) float64 {
	if fe.r1 == 0 || v != fe.v[1] {
		// Not scaled, so a constant radius's last bits stay as they were.
		return fe.ss[k]
	}
	return fe.ss[k] * fe.r1 / fe.r
}

// varies reports whether the edge's radius goes from one to another
// along it.
func (fe *filletEdge) varies() bool {
	return fe.r1 != 0 && math.Abs(fe.r1-fe.r) > geom.LengthEps
}

// varyingArc is the circular edge fe, reached from the rounded edge from
// at fe's vertex v with from's radius, made to take the radius of the
// rounded edge of the same kind it meets smoothly at its other end, when
// that radius is another; false when there is none.
func (t *Topology) varyingArc(from *filletEdge, fe filletEdge, v int, edges []filletEdge, byEdge map[int]int, flat bool) (filletEdge, bool) {
	if !fe.circ || fe.v[0] == fe.v[1] {
		return fe, false
	}
	w := fe.v[0]
	if w == v {
		w = fe.v[1]
	}
	for _, y := range t.Vertices[w].Edges {
		i, ok := byEdge[y]
		if !ok || y == fe.e {
			continue
		}
		g := &edges[i]
		if g.circ || g.concave != fe.concave || math.Abs(g.r-from.r) <= geom.LengthEps {
			continue
		}
		at, err := t.filletEdgeOf(fe.e, g.r, flat)
		if err != nil || !t.smoothJoin(g, &at, w) {
			continue
		}
		// The radius at v[0] is the edge's own, at v[1] r1.
		out := fe
		if v == fe.v[1] {
			out = at
		}
		out.r1 = g.r
		if v == fe.v[1] {
			out.r1 = from.r
		}
		if out.fitsBoth() != nil {
			return fe, false
		}
		return out, true
	}
	return fe, false
}

// fitsBoth is fits at both of the edge's radii.
func (fe *filletEdge) fitsBoth() error {
	if err := fe.fits(fe.curve.Start()); err != nil {
		return err
	}
	at := *fe
	at.ss[0], at.ss[1] = fe.ssAt(0, fe.v[1]), fe.ssAt(1, fe.v[1])
	return at.fits(fe.curve.Finish())
}

// varyingRows are the homogeneous control points along the arc, of
// degree varyDeg, of each control point of the varying edge's cross
// section: the section's point on face 0, its control point on the edge
// (none for a chamfer's line) and its point on face 1; and the
// section's weights.
func (fe *filletEdge) varyingRows(flat bool) ([][varyDeg + 1]geom.Vector4, []float64) {
	// The arc, homogeneous, and the law of the radius.
	var arcH [3]geom.Vector4
	for j := range 3 {
		p, wj := fe.curve.Ctrl[j], fe.curve.Weight[j]
		arcH[j] = geom.Vector4{X: p.X * wj, Y: p.Y * wj, Z: p.Z * wj, W: wj}
	}
	law := [4]float64{1, 1, fe.r1 / fe.r, fe.r1 / fe.r}
	// The section of the edge's own radius r at a point, as directions
	// along the radius and the axis: each control point's.
	type ctl struct{ along, up float64 }
	var cs []ctl
	var weights []float64
	for k := range 2 {
		c := ctl{fe.ss[k] * fe.gen[k][0], fe.ss[k] * fe.gen[k][1]}
		if k == 1 && !flat {
			cs = append(cs, ctl{})
			weights = append(weights, fe.w)
		}
		cs = append(cs, c)
		weights = append(weights, 1)
	}
	rows := make([][varyDeg + 1]geom.Vector4, len(cs))
	for i, c := range cs {
		// The control point's offset from the edge, homogeneous: along the
		// radius (the arc's point less the centre, over the radius) and
		// along the axis; the law scales it.
		var off [3]geom.Vector4
		for j := range 3 {
			h := arcH[j]
			rad := geom.Vector{X: h.X, Y: h.Y, Z: h.Z}.Minus(fe.centre.ScaledBy(h.W)).ScaledBy(c.along / fe.radius)
			d := rad.Plus(fe.axis.ScaledBy(c.up * h.W))
			off[j] = geom.Vector4{X: d.X, Y: d.Y, Z: d.Z}
		}
		scaled := bernsteinTimes(law[:], off[:])
		edge := bernsteinTimes([]float64{1, 1, 1, 1}, arcH[:])
		for j := range varyDeg + 1 {
			rows[i][j] = edge[j].Plus(scaled[j])
		}
	}
	return rows, weights
}

// bernsteinTimes is the product of the scalar polynomial a and the
// homogeneous one b, each by its Bernstein coefficients.
func bernsteinTimes(a []float64, b []geom.Vector4) []geom.Vector4 {
	m, n := len(a)-1, len(b)-1
	out := make([]geom.Vector4, m+n+1)
	for i := range a {
		for j := range b {
			f := binomial(m, i) * binomial(n, j) / binomial(m+n, i+j)
			out[i+j] = out[i+j].Plus(b[j].ScaledBy(a[i] * f))
		}
	}
	return out
}

// fromH is the point and weight of a homogeneous control point.
func fromH(h geom.Vector4) (geom.Vector, float64) {
	return geom.Vector{X: h.X / h.W, Y: h.Y / h.W, Z: h.Z / h.W}, h.W
}

// varyingTangentCurve is the varying edge's tangent curve on face k,
// from p to q.
func (fe *filletEdge) varyingTangentCurve(k int, p, q geom.Vector) srf.SBezier {
	rows, _ := fe.varyingRows(true)
	var sb srf.SBezier
	sb.Degree = varyDeg
	for j := range varyDeg + 1 {
		sb.Ctrl[j], sb.Weight[j] = fromH(rows[k][j])
	}
	sb.Ctrl[0], sb.Ctrl[varyDeg] = p, q
	return sb
}

// varyingSurface is the varying edge's rounded face: across, the cross
// section; along, the arc from its start to its end.
func (fe *filletEdge) varyingSurface(flat bool) srf.SSurface {
	rows, weights := fe.varyingRows(flat)
	s := srf.SSurface{DegM: len(rows) - 1, DegN: varyDeg}
	for i := range rows {
		for j := range varyDeg + 1 {
			p, wj := fromH(rows[i][j])
			s.Ctrl[i][j], s.Weight[i][j] = p, wj*weights[i]
		}
	}
	return s
}
