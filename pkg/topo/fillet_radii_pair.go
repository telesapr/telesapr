package topo

import (
	"errors"
	"math"
	"strconv"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// Fillets of two radii meeting at a corner of three flat faces whose
// third edge is left sharp: a box's top edges at 1 and 2 above its
// upright, a ledge's two top edges above the concave upright, a rib's
// two foot edges below its convex one. The two cylinders are tangent
// to the face they share, top, so they cross where their tangent lines
// on it do, tt, and meet along a quartic with its double point there,
// not along the plane that halves a mitre of one radius. As at an
// uneven mitre (unevenMitre), one strip, A, reaches its side face's
// tangent line on the other's cylinder, at qa, before that line meets
// the third edge, and ends along the quartic from tt to qa; the other,
// B, goes on past it, cut by A's side face along a conic, to where its
// own tangent line meets the third edge. That is OpenCASCADE's corner:
// two cylinders and no patch.

// radiiPairTakes reports whether vertex v is a corner radiiPairAt
// makes: three flat faces and three straight edges, two of them rounded
// alike, convex or concave, at radii that differ, the third left sharp.
func (t *Topology) radiiPairTakes(v int, edges []filletEdge, byEdge map[int]int) bool {
	vt := &t.Vertices[v]
	if len(vt.Edges) != 3 || len(t.facesAt(v)) != 3 {
		return false
	}
	var round []*filletEdge
	for _, x := range vt.Edges {
		if !t.Straight(x) {
			return false
		}
		if i, ok := byEdge[x]; ok {
			round = append(round, &edges[i])
		}
	}
	if len(round) != 2 {
		return false
	}
	a, b := round[0], round[1]
	if a.circ || b.circ || a.varies() || b.varies() || a.concave != b.concave ||
		math.Abs(a.r-b.r) <= geom.LengthEps || sharedFace(a, b) < 0 {
		return false
	}
	for _, f := range t.facesAt(v) {
		if _, ok := t.planarNormal(f); !ok {
			return false
		}
	}
	return true
}

// radiiPairAt works out the corner at v (radiiPairTakes): of the two
// ways round, the one whose stopping strip's cut lies on the other's
// fillet.
func (t *Topology) radiiPairAt(v int, edges []filletEdge, byEdge map[int]int) (*filletCorner, error) {
	p := t.Vertices[v].P
	var round []*filletEdge
	z := -1
	for _, x := range t.Vertices[v].Edges {
		if i, ok := byEdge[x]; ok {
			round = append(round, &edges[i])
		} else {
			z = x
		}
	}
	for _, ab := range [][2]*filletEdge{{round[0], round[1]}, {round[1], round[0]}} {
		c, err := t.radiiPair(v, ab[0], ab[1], z)
		if err == nil {
			return c, nil
		}
		if !errors.Is(err, errRadiiPair) {
			return nil, err
		}
	}
	lo, hi := math.Min(round[0].r, round[1].r), math.Max(round[0].r, round[1].r)
	return nil, filletErr(p, "fillets of radius %s and %s meet here above an edge left sharp, and neither ends on the other's; "+
		"round that edge too, or give the fillets one radius",
		strconv.FormatFloat(lo, 'g', -1, 64), strconv.FormatFloat(hi, 'g', -1, 64))
}

// errRadiiPair refuses one way round a corner of two radii: A's tangent
// line on its side doesn't reach B's fillet before the third edge.
var errRadiiPair = errors.New("the fillet doesn't end on the other's")

// radiiPair is the corner at v with sa, the strip that stops, ending on
// sb's cylinder, and sb going on past it to the third edge z.
func (t *Topology) radiiPair(v int, sa, sb *filletEdge, z int) (*filletCorner, error) {
	p := t.Vertices[v].P
	sg := sa.sign()
	top := sharedFace(sa, sb)
	kA, kB := 1-faceK(sa, top), 1-faceK(sb, top)
	nt, _ := t.planarNormal(top)
	na, _ := t.planarNormal(sa.f[kA])
	nb, _ := t.planarNormal(sb.f[kB])
	awA, awB := awayFrom(sa, p, t), awayFrom(sb, p, t)
	tA := p.Plus(sa.u[1-kA].ScaledBy(sa.ss[1-kA]))
	tB := p.Plus(sb.u[1-kB].ScaledBy(sb.ss[1-kB]))
	sA := p.Plus(sa.u[kA].ScaledBy(sa.ss[kA]))
	axA := tA.Minus(nt.ScaledBy(sg * sa.r))
	axB := tB.Minus(nt.ScaledBy(sg * sb.r))
	tt, ok := lineMeet(tA, awA, tB, awB)
	if !ok {
		return nil, filletErr(p, "the corner's fillets run along one another")
	}
	// Where sa's tangent line on its side meets sb's fillet: of the line's
	// crossings with sb's cylinder, those between sb's tangent lines, the
	// first coming in along sa from its far end.
	qa, found := geom.Vector{}, false
	for _, s := range cylinderCrossings(sA, sa.dir, axB, sb.dir, sb.r) {
		x := sA.Plus(sa.dir.ScaledBy(s))
		if !onArc(x.Minus(axB), sb.dir, sg, nt, nb) {
			continue
		}
		if !found || x.Minus(p).Dot(awA) > qa.Minus(p).Dot(awA) {
			qa, found = x, true
		}
	}
	if !found {
		return nil, errRadiiPair
	}
	qA, err := t.pointAlong(z, v, sa.u[kA], sa.ss[kA], false)
	if err != nil {
		return nil, err
	}
	w, err := t.pointAlong(z, v, sb.u[kB], sb.ss[kB], false)
	if err != nil {
		return nil, err
	}
	// As at an uneven mitre: qa lies further along sa than the third
	// edge where that edge is of the fillets' kind, and past it, sa's side
	// face growing out to it, where it is of the other.
	inside := qa.Minus(qA).Dot(awA)
	if t.otherKind(z, sa) {
		inside = -inside
	}
	if inside < 10*geom.LengthEps {
		return nil, errRadiiPair
	}
	if qa.Minus(p).Dot(awA) > t.edgeLen(sa)-10*geom.LengthEps || tt.Minus(p).Dot(awA) > t.edgeLen(sa)-10*geom.LengthEps ||
		tt.Minus(p).Dot(awB) > t.edgeLen(sb)-10*geom.LengthEps {
		return nil, filletErr(p, "the edge is too short for a fillet this large")
	}
	chain, err := cylindersMeet(axA, sa.dir, sa.r, axB, sb.dir, sb.r, tt, qa)
	if err != nil {
		return nil, filletErr(p, "%v", err)
	}
	conic, err := cylinderConic(axB, sb.dir, sg*sb.r, na, nb, qa, w)
	if err != nil {
		return nil, filletErr(p, "%v", err)
	}
	c := &filletCorner{kind: 2, at: map[[2]int]geom.Vector{}, long: sb.e, r: sb.r}
	c.chain, c.conic, c.cutFace = chain, conic, sa.f[kA]
	c.at[[2]int{sa.e, top}], c.at[[2]int{sb.e, top}] = tt, tt
	c.at[[2]int{sa.e, sa.f[kA]}] = qa
	c.at[[2]int{sb.e, sb.f[kB]}] = w
	for _, f := range t.EdgeFaces(z) {
		c.at[[2]int{z, f}] = w
	}
	return c, nil
}

// cylinderCrossings are the parameters s at which the line through o
// along d meets the cylinder of radius r about the axis through a along
// e (a unit vector).
func cylinderCrossings(o, d, a, e geom.Vector, r float64) []float64 {
	w := o.Minus(a)
	w = w.Minus(e.ScaledBy(w.Dot(e)))
	dp := d.Minus(e.ScaledBy(d.Dot(e)))
	qa, qb, qc := dp.Dot(dp), 2*w.Dot(dp), w.Dot(w)-r*r
	if qa < 1e-18 {
		return nil
	}
	disc := qb*qb - 4*qa*qc
	if disc < 0 {
		if disc > -1e-12*qb*qb {
			disc = 0
		} else {
			return nil
		}
	}
	sq := math.Sqrt(disc)
	return []float64{(-qb - sq) / (2 * qa), (-qb + sq) / (2 * qa)}
}

// onArc reports whether a point at d from a fillet's axis along e lies
// on the fillet: between its tangent points on the faces of normals n1
// and n2, which the radial of a convex fillet (sg 1) points to and a
// concave one's away from.
func onArc(d, e geom.Vector, sg float64, n1, n2 geom.Vector) bool {
	d = d.Minus(e.ScaledBy(d.Dot(e))).ScaledBy(sg)
	c := n1.Dot(n2)
	den := 1 - c*c
	if den < 1e-12 {
		return false
	}
	x, y := d.Dot(n1), d.Dot(n2)
	al, be := (x-c*y)/den, (y-c*x)/den
	eps := 1e-9 * d.Magnitude()
	return al >= -eps && be >= -eps
}

// cylinderConic is the conic where the plane of normal na, holding q0,
// cuts the cylinder of radius |sr| about the axis through a along e,
// from q0 to q1: tangent at q1 to the face of normal nb, which the
// cylinder touches there: the end of the strip that goes on past a
// mitre (unevenMitre, radiiPair).
func cylinderConic(a, e geom.Vector, sr float64, na, nb, q0, q1 geom.Vector) (srf.SBezier, error) {
	radial := func(x geom.Vector) geom.Vector {
		d := x.Minus(a)
		return d.Minus(e.ScaledBy(d.Dot(e)))
	}
	t0 := na.Cross(radial(q0)).WithMagnitude(1)
	t1 := na.Cross(nb).WithMagnitude(1)
	ctrl, ok := lineMeet(q0, t0, q1, t1)
	if !ok {
		return srf.SBezier{}, errors.New("the corner's conic is degenerate")
	}
	// The weight from the conic's middle, on the line from the chord's
	// middle to the control point: the cylinder there.
	mc := q0.Plus(q1).ScaledBy(0.5)
	d0, d1 := radial(mc), radial(ctrl).Minus(radial(mc))
	qa, qb, qc := d1.Dot(d1), 2*d0.Dot(d1), d0.Dot(d0)-sr*sr
	disc := qb*qb - 4*qa*qc
	if qa < 1e-18 || disc < 0 {
		return srf.SBezier{}, errors.New("the corner's conic is degenerate")
	}
	k := -1.0
	for _, x := range []float64{(-qb - math.Sqrt(disc)) / (2 * qa), (-qb + math.Sqrt(disc)) / (2 * qa)} {
		if x > 1e-9 && x < 1-1e-9 {
			k = x
		}
	}
	if k < 0 {
		return srf.SBezier{}, errors.New("the corner's conic is degenerate")
	}
	return arc(q0, ctrl, q1, k/(1-k)), nil
}

// meetFit is how far the quintics standing for two cylinders' quartic may
// stray from it.
const meetFit = 0.1 * geom.LengthEps

// cylindersMeet is the curve where the cylinder of radius ra about the
// axis through a along ea meets the one of radius rb about b along eb,
// from p0 to p1, both on both: quintics, each within meetFit of the
// quartic. It is followed by the angle round the first cylinder, the
// point along its axis the crossing of that ruling with the second
// cylinder that p1's is.
func cylindersMeet(a, ea geom.Vector, ra float64, b, eb geom.Vector, rb float64, p0, p1 geom.Vector) ([]srf.SBezier, error) {
	radial := func(x geom.Vector) geom.Vector {
		d := x.Minus(a)
		return d.Minus(ea.ScaledBy(d.Dot(ea)))
	}
	e1 := radial(p0).WithMagnitude(1)
	r1 := radial(p1)
	e2 := r1.Minus(e1.ScaledBy(r1.Dot(e1)))
	if e2.Magnitude() < 1e-9*ra {
		return nil, errors.New("the fillets' curve is degenerate")
	}
	e2 = e2.WithMagnitude(1)
	th1 := math.Atan2(r1.Dot(e2), r1.Dot(e1))
	// The root of the ruling's two crossings that holds p1, kept all the
	// way: the two meet at p0, where the cylinders touch.
	root := 1
	ruling := func(th float64) geom.Vector {
		return a.Plus(e1.ScaledBy(ra * math.Cos(th))).Plus(e2.ScaledBy(ra * math.Sin(th)))
	}
	at := func(th float64) (geom.Vector, bool) {
		if th == 0 {
			// At the double point the two crossings are one, and the
			// discriminant's rounding moves it by its square root.
			return p0, true
		}
		y := ruling(th)
		s := cylinderCrossings(y, ea, b, eb, rb)
		if s == nil {
			// Beside the double point the discriminant may dip below
			// nothing by rounding: the ruling's nearest approach.
			w := y.Minus(b)
			w = w.Minus(eb.ScaledBy(w.Dot(eb)))
			dp := ea.Minus(eb.ScaledBy(ea.Dot(eb)))
			if dp.Dot(dp) < 1e-18 {
				return geom.Vector{}, false
			}
			return y.Plus(ea.ScaledBy(-w.Dot(dp) / dp.Dot(dp))), true
		}
		return y.Plus(ea.ScaledBy(s[root])), true
	}
	if s := cylinderCrossings(ruling(th1), ea, b, eb, rb); len(s) == 2 {
		x0, x1 := ruling(th1).Plus(ea.ScaledBy(s[0])), ruling(th1).Plus(ea.ScaledBy(s[1]))
		if x0.Minus(p1).Magnitude() < x1.Minus(p1).Magnitude() {
			root = 0
		}
	} else {
		return nil, errors.New("the fillets' curve is degenerate")
	}
	// Wide enough that the rounding of a crossing just beside the double
	// point, divided by it, stays below the steps' own error.
	lo, hi := math.Min(0, th1), math.Max(0, th1)
	h := 1e-3 * math.Abs(th1)
	pt := func(x float64) geom.Vector { q, _ := at(x); return q }
	// The first and second derivatives by differences: on one side, of
	// second order, where th is at an end, else central.
	derivs := func(th float64) (geom.Vector, geom.Vector) {
		sg := 0.0
		switch {
		case th-h < lo-1e-15:
			sg = 1
		case th+h > hi+1e-15:
			sg = -1
		}
		if sg == 0 {
			x0, x1, x2 := pt(th-h), pt(th), pt(th+h)
			return x2.Minus(x0).ScaledBy(1 / (2 * h)), x2.Minus(x1.ScaledBy(2)).Plus(x0).ScaledBy(1 / (h * h))
		}
		k := sg * h
		x0, x1, x2, x3 := pt(th), pt(th+k), pt(th+2*k), pt(th+3*k)
		d := x1.ScaledBy(4).Minus(x0.ScaledBy(3)).Minus(x2).ScaledBy(1 / (2 * k))
		dd := x0.ScaledBy(2).Minus(x1.ScaledBy(5)).Plus(x2.ScaledBy(4)).Minus(x3).ScaledBy(1 / (h * h))
		return d, dd
	}
	var out []srf.SBezier
	var fit func(t0, t1 float64, depth int) error
	fit = func(t0, t1 float64, depth int) error {
		x0, ok0 := at(t0)
		x1, ok1 := at(t1)
		if !ok0 || !ok1 {
			return errors.New("the fillets' curve is degenerate")
		}
		if t1 == th1 {
			x1 = p1
		}
		dt := t1 - t0
		// The quintic matching the curve's point, tangent and second
		// derivative at both ends.
		d0, dd0 := derivs(t0)
		d1, dd1 := derivs(t1)
		sb := srf.SBezier{Degree: 5}
		sb.Ctrl[0] = x0
		sb.Ctrl[1] = x0.Plus(d0.ScaledBy(dt / 5))
		sb.Ctrl[2] = x0.Plus(d0.ScaledBy(2 * dt / 5)).Plus(dd0.ScaledBy(dt * dt / 20))
		sb.Ctrl[3] = x1.Minus(d1.ScaledBy(2 * dt / 5)).Plus(dd1.ScaledBy(dt * dt / 20))
		sb.Ctrl[4] = x1.Minus(d1.ScaledBy(dt / 5))
		sb.Ctrl[5] = x1
		for i := range 6 {
			sb.Weight[i] = 1
		}
		worst := 0.0
		for i := 1; i < 16; i++ {
			u := float64(i) / 16
			x, _ := at(t0 + u*dt)
			worst = math.Max(worst, sb.PointAt(u).Minus(x).Magnitude())
		}
		if worst <= meetFit || depth >= 10 {
			if worst > geom.LengthEps {
				return errors.New("the fillets' curve can't be fitted")
			}
			out = append(out, sb)
			return nil
		}
		m := t0 + dt/2
		if err := fit(t0, m, depth+1); err != nil {
			return err
		}
		return fit(m, t1, depth+1)
	}
	if err := fit(0, th1, 0); err != nil {
		return nil, err
	}
	return out, nil
}

// orientChain is the chain of curves sbs, end to end, run from p: as it
// is, or reversed.
func orientChain(sbs []srf.SBezier, p geom.Vector) []srf.SBezier {
	out := make([]srf.SBezier, len(sbs))
	copy(out, sbs)
	if out[0].Start().Equals(p) {
		return out
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	for i := range out {
		out[i].Reverse()
	}
	return out
}
