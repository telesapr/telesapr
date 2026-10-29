package topo

import (
	"errors"
	"fmt"
	"math"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// Blend corners: a convex fillet and a concave one meeting at a corner
// of three flat faces whose third edge is left sharp, as a ledge's top
// edge above the concave upright below it, or a rib's upright above one
// of its foot's edges. The two fillets touch only at a point, where
// their tangent lines cross on the face they share, and the sharp edge
// has to end somewhere on the blend: no sphere, mitre or stage rounds
// it, and the hole between them is filled by one patch, as OpenCASCADE
// fills it.
//
// Of the two fillets, the one whose tangent line on the face it shares
// with the sharp edge meets that edge past the corner is cut obliquely
// (o): from where the two fillets' tangent lines cross on their shared
// face, Q, up its rounded face to that point on the sharp edge, P. The
// other (m) is cut through Q too, square to its edge where the two edges
// are square. The hole is then three sided: m's end from its tangent
// line on the sharp edge's face, S, to Q; o's cut from Q to P; and a
// curve on the face m shares with the sharp edge from P back to S. The
// patch has its fourth side folded to P, where the sharp edge ends:
// tangent to the face along one side of P and to o's rounded face along
// the other, its normal turning between them across the fold.
//
// Both cuts are where a plane cuts the cylinder, ellipses the quintics
// follow, and each leaves Q along the other fillet's axis; the face's
// curve reaches S along m's tangent line. Where they don't, the patch's
// twist at Q or S can't be tangent both ways and its normal strays from
// the fillet's beside the corner.
//
// Tangency is written into the patch's control points as for setback
// corners (setbackUnit): along each fillet's end the second row leans
// along the cylinder's axis and the curve's own tangent, which keeps it
// in the cylinder's tangent plane whether or not the curve is a section;
// along the face's curve it lies in the face. At P the row beside o's
// cut is P itself, which holds the lean there at nothing.

// blendCornerTakes reports whether vertex v is a corner a blend fills:
// three flat faces and three straight edges, one rounded convex, one
// rounded concave, the third left sharp.
func (t *Topology) blendCornerTakes(v int, edges []filletEdge, byEdge map[int]int) bool {
	vt := &t.Vertices[v]
	if len(vt.Edges) != 3 || len(t.facesAt(v)) != 3 {
		return false
	}
	var kinds [2]int
	for _, x := range vt.Edges {
		i, ok := byEdge[x]
		if !ok {
			if !t.Straight(x) {
				return false
			}
			continue
		}
		if edges[i].circ {
			return false
		}
		kinds[b2i(edges[i].concave)]++
	}
	if kinds[0] != 1 || kinds[1] != 1 {
		return false
	}
	for _, f := range t.facesAt(v) {
		if _, ok := t.planarNormal(f); !ok {
			return false
		}
	}
	return true
}

// blendGeom is what a blend corner is worked out from.
type blendGeom struct {
	p    geom.Vector
	m, o *filletEdge
	// sharp is the edge left sharp, away along it from the corner.
	sharp int
	away  geom.Vector
	// fmo is the face m and o share, fms the one m shares with the sharp
	// edge, fos the one o does.
	fmo, fms, fos int
}

// blendCornerAt works out the blend corner at v (blendCornerTakes): of
// the two ways round, the one whose cut ends on the sharp edge past the
// corner.
func (t *Topology) blendCornerAt(v int, edges []filletEdge, byEdge map[int]int) (*filletCorner, error) {
	p := t.Vertices[v].P
	var round [2]*filletEdge
	sharp := -1
	for _, x := range t.Vertices[v].Edges {
		i, ok := byEdge[x]
		if !ok {
			sharp = x
			continue
		}
		round[b2i(edges[i].concave)] = &edges[i]
	}
	away := t.edgeAway(sharp, v)
	firstErr := error(nil)
	for _, mo := range [][2]*filletEdge{{round[0], round[1]}, {round[1], round[0]}} {
		g := blendGeom{p: p, m: mo[0], o: mo[1], sharp: sharp, away: away}
		g.fmo, g.fms, g.fos = sharedFace(g.m, g.o), otherFace(g.m, g.o), otherFace(g.o, g.m)
		if g.fmo < 0 {
			return nil, filletErr(p, "the corner's fillets don't share a face")
		}
		c, err := t.blendCorner(&g)
		if err == nil {
			return c, nil
		}
		// The way round whose fillets' lines meet past the corner says
		// why better than the one whose don't.
		if firstErr == nil || errors.Is(firstErr, errBlendLines) {
			firstErr = err
		}
	}
	if errors.Is(firstErr, errBlendLines) {
		return nil, filletErr(p, "%v", firstErr)
	}
	return nil, firstErr
}

// errBlendLines refuses a way round a blend corner whose fillets'
// tangent lines don't meet past it.
var errBlendLines = errors.New("the fillets' tangent lines don't meet past the corner")

// edgeAway is the direction along straight edge x away from vertex v.
func (t *Topology) edgeAway(x, v int) geom.Vector {
	pts := t.Edges[x].Points
	a, b := pts[0], pts[len(pts)-1]
	if a.Equals(t.Vertices[v].P) {
		return b.Minus(a).WithMagnitude(1)
	}
	return a.Minus(b).WithMagnitude(1)
}

// sharedFace is the face a and b both lie along, or -1.
func sharedFace(a, b *filletEdge) int {
	for _, f := range a.f {
		if f == b.f[0] || f == b.f[1] {
			return f
		}
	}
	return -1
}

// otherFace is a's face that b doesn't lie along.
func otherFace(a, b *filletEdge) int {
	if a.f[0] == b.f[0] || a.f[0] == b.f[1] {
		return a.f[1]
	}
	return a.f[0]
}

// faceK is which of fe's faces f is.
func faceK(fe *filletEdge, f int) int {
	if fe.f[0] == f {
		return 0
	}
	return 1
}

// awayFrom is the direction along fe's edge away from the corner p.
func awayFrom(fe *filletEdge, p geom.Vector, t *Topology) geom.Vector {
	if t.Vertices[fe.v[1]].P.Equals(p) {
		return fe.dir.ScaledBy(-1)
	}
	return fe.dir
}

// blendCorner is the blend corner of one way round, g.
func (t *Topology) blendCorner(g *blendGeom) (*filletCorner, error) {
	p, m, o := g.p, g.m, g.o
	am, ao := awayFrom(m, p, t), awayFrom(o, p, t)
	line := func(fe *filletEdge, f int) geom.Vector {
		k := faceK(fe, f)
		return p.Plus(fe.u[k].ScaledBy(fe.ss[k]))
	}
	q, ok := lineMeet(line(m, g.fmo), am, line(o, g.fmo), ao)
	if !ok {
		return nil, filletErr(p, "the corner's fillets run along one another")
	}
	pp, ok := lineMeet(line(o, g.fos), ao, p, g.away)
	if !ok {
		return nil, filletErr(p, "the fillet runs along the edge left sharp")
	}
	sm, so := q.Minus(p).Dot(am), q.Minus(p).Dot(ao)
	reach := pp.Minus(p).Dot(g.away)
	if sm < 10*geom.LengthEps || so < -geom.LengthEps || reach < 10*geom.LengthEps ||
		reach > t.straightLen(g.sharp)-10*geom.LengthEps || pp.Minus(p).Dot(ao) < -geom.LengthEps {
		return nil, errBlendLines
	}
	if so > t.edgeLen(o)-10*geom.LengthEps || sm > t.edgeLen(m)-10*geom.LengthEps {
		return nil, filletErr(p, "the edge is too short for a fillet this large")
	}
	// Each fillet's end is where a plane through q cuts its rounded face,
	// leaving q along the other's axis: there the patch's twist can be
	// tangent to both. m's plane holds o's axis and the shared face's
	// normal, and is square to m's edge where the two edges are; o's
	// holds m's axis and reaches pp.
	nmo, _ := t.planarNormal(g.fmo)
	marc, err := planeCut(m, g.fms, q, o.dir.Cross(nmo), p)
	if err != nil {
		return nil, filletErr(q, "the fillet's end at the corner can't be made: %v", err)
	}
	s := marc.Start()
	if s.Minus(p).Dot(am) < 10*geom.LengthEps {
		return nil, errBlendLines
	}
	ocut, err := planeCut(o, g.fmo, q, m.dir.Cross(pp.Minus(q)), p)
	if err != nil {
		return nil, filletErr(q, "the fillet's end at the corner can't be made: %v", err)
	}
	if !ocut.Finish().Equals(pp) {
		return nil, filletErr(pp, "the fillet's end at the corner misses the edge left sharp")
	}
	marc.Ctrl[patchDeg], ocut.Ctrl[0], ocut.Ctrl[patchDeg] = q, q, pp
	nms, _ := t.planarNormal(g.fms)
	fc, patch, err := blendFill(marc, ocut, blendFaceCurves(s, am, pp, g.away), am, ao, nms)
	if _, at := errors.AsType[*FilletError](err); err != nil && !at {
		return nil, filletErr(p, "%v", err)
	} else if err != nil {
		return nil, err
	}
	c := &filletCorner{kind: setbackCorner, at: map[[2]int]geom.Vector{}, long: -1, r: math.Max(m.r, o.r),
		arcs: map[int]srf.SBezier{m.e: marc, o.e: ocut}, faceCurves: map[int]srf.SBezier{g.fms: fc},
		quads: []srf.SSurface{patch}}
	c.at[[2]int{m.e, g.fms}] = s
	c.at[[2]int{m.e, g.fmo}] = q
	c.at[[2]int{o.e, g.fmo}] = q
	c.at[[2]int{o.e, g.fos}] = pp
	c.at[[2]int{g.sharp, g.fms}] = pp
	c.at[[2]int{g.sharp, g.fos}] = pp
	return c, nil
}

// blendFaceCurves are the curves tried for a blend's side on the face m
// shares with the sharp edge, from pp on it back to s, each for a leg
// (setbackFaceCurve): leaving pp along the sharp edge and reaching s
// along m's tangent line am, on which the twist at s depends. The
// parabola through where the two lines cross, where they cross between
// them; then cubics leaving pp back along the edge, towards s, or
// between, for in a corner more open than a right angle the edge's way
// back passes the vertex, and the patch folds there.
func blendFaceCurves(s, am, pp, away geom.Vector) []func(leg float64) (srf.SBezier, error) {
	var tries []func(leg float64) (srf.SBezier, error)
	if x, ok := lineMeet(s, am, pp, away); ok && x.Minus(s).Dot(am) < -10*geom.LengthEps &&
		pp.Minus(x).Dot(away) > 10*geom.LengthEps {
		tries = append(tries, func(leg float64) (srf.SBezier, error) { return setbackFaceCurve(pp, x, s, leg) })
	}
	chord := s.Minus(pp).WithMagnitude(1)
	for _, d := range []geom.Vector{away.ScaledBy(-1), chord.Minus(away).WithMagnitude(1), chord} {
		tries = append(tries, func(leg float64) (srf.SBezier, error) {
			l := math.Max(leg, 0.2) * pp.Minus(s).Magnitude()
			sb := srf.SBezierFrom4(pp, pp.Plus(d.ScaledBy(l)), s.Minus(am.ScaledBy(l)), s)
			return sb.ElevatedTo(patchDeg)
		})
	}
	return tries
}

// blendLegs are the face curve's legs tried in turn: nearer the chord
// first, as OpenCASCADE's is (the parabola takes up to 0.17 mm³ more
// than its fill on a ledge at radius 2, 0.4 up to 0.15 less), then the
// parabola.
var blendLegs = []float64{0.4, 0.2, 0}

// blendFill is a blend's face curve and patch: m's end marc and o's cut
// ocut, the hole along m's and o's axes against am and ao, the face's
// curve from those tries in a plane of normal nms. The curves and legs
// are tried in turn, and with each the patch leaning into the hole by
// less, then more, until one doesn't fold.
func blendFill(marc, ocut srf.SBezier, tries []func(leg float64) (srf.SBezier, error), am, ao, nms geom.Vector) (srf.SBezier, srf.SSurface, error) {
	var first error
	for _, curve := range tries {
		for _, leg := range blendLegs {
			fc, err := curve(leg)
			if err != nil {
				return srf.SBezier{}, srf.SSurface{}, fmt.Errorf("the corner's curve on a face can't be made: %w", err)
			}
			for _, lw := range blendLean {
				patch, err := blendPatch(
					setbackSide{ctrl: quinticCtrl(marc), arc: true, e: am.ScaledBy(-1)},
					setbackSide{ctrl: quinticCtrl(ocut), arc: true, e: ao.ScaledBy(-1)},
					setbackSide{ctrl: quinticCtrl(fc), e: nms}, lw)
				if err == nil {
					return fc, patch, nil
				}
				if first == nil {
					first = err
				}
			}
		}
	}
	return srf.SBezier{}, srf.SSurface{}, first
}

// planeCut is where the plane through at of normal n cuts fe's rounded
// face, from its tangent line on face from to the other, as a quintic:
// the section at p with each control point moved along the edge onto
// the plane, which moves the rational arc onto the ellipse exactly.
func planeCut(fe *filletEdge, from int, at, n, p geom.Vector) (srf.SBezier, error) {
	if math.Abs(fe.dir.Dot(n)) < 1e-6*n.Magnitude() {
		return srf.SBezier{}, errNoArc
	}
	sb := fe.section(p, false)
	if faceK(fe, from) == 1 {
		sb.Reverse()
	}
	for i := 0; i <= sb.Degree; i++ {
		sb.Ctrl[i] = sb.Ctrl[i].Plus(fe.dir.ScaledBy(at.Minus(sb.Ctrl[i]).Dot(n) / fe.dir.Dot(n)))
	}
	return quinticArc(sb)
}

// straightLen is the length of straight edge x.
func (t *Topology) straightLen(x int) float64 {
	pts := t.Edges[x].Points
	return pts[len(pts)-1].Minus(pts[0]).Magnitude()
}

// edgeLen is the length of fe's edge.
func (t *Topology) edgeLen(fe *filletEdge) float64 {
	return t.Vertices[fe.v[1]].P.Minus(t.Vertices[fe.v[0]].P).Magnitude()
}

// blendLean are the weights, against fairness, of the patch's rows
// leaning into the hole, tried in turn.
var blendLean = []float64{0.1, 0.01, 0.3, 1}

// blendPatch is the patch of a blend corner: bottom from s to q, a
// fillet's section; right from q to p, a cut along a fillet; its top
// folded to p; left from p back to s, a curve on a face. The fillets'
// sides' e is their axis, pointing into the hole; w is the weight of the
// rows leaning into it.
func blendPatch(bottom, right, left setbackSide, w float64) (srf.SSurface, error) {
	const n = patchDeg
	p := right.ctrl[n]
	var mid geom.Vector
	for _, sd := range []setbackSide{bottom, right, left} {
		mid = mid.Plus(sd.ctrl[0])
	}
	mid = mid.ScaledBy(1.0 / 3)
	size := 0.0
	for _, sd := range []setbackSide{bottom, right, left} {
		for _, c := range sd.ctrl {
			size = math.Max(size, c.Minus(mid).Magnitude())
		}
	}
	if size < 10*geom.LengthEps {
		return srf.SSurface{}, filletErr(mid, "the corner's hole is too small")
	}
	unit := func(sd setbackSide) setbackSide {
		for k := range sd.ctrl {
			sd.ctrl[k] = sd.ctrl[k].Minus(mid).ScaledBy(1 / size)
		}
		return sd
	}
	bu, ru, lu := unit(bottom), unit(right), unit(left)
	pu := ru.ctrl[n]
	// The left side runs from s up to p.
	var up [n + 1]geom.Vector
	for k := range n + 1 {
		up[k] = lu.ctrl[n-k]
	}

	nPts := (n + 1) * (n + 1) * 3
	pv := func(i, j, d int) int { return (i*(n+1)+j)*3 + d }
	bv, rv := nPts, nPts+n+3
	nv := nPts + 2*(n+3)
	var a [][]float64
	var b []float64
	eq := func(co map[int]float64, rhs, w float64) {
		row := make([]float64, nv)
		for k, x := range co {
			row[k] += x * w
		}
		a = append(a, row)
		b = append(b, rhs*w)
	}
	const hard = 1e6
	fix := func(i, j int, at geom.Vector) {
		for d := range 3 {
			eq(map[int]float64{pv(i, j, d): 1}, comp(at, d), hard)
		}
	}
	for k := range n + 1 {
		fix(k, 0, bu.ctrl[k])
		fix(n, k, ru.ctrl[k])
		fix(k, n, pu)
		fix(0, k, up[k])
	}
	// Tangent to a cylinder of axis e along the curve ctrl: the row
	// beside it (in, at k) is the curve's point moved by tau along the
	// axis and sigma, linear, along the curve's own legs.
	cyl := func(ctrl [n + 1]geom.Vector, e geom.Vector, vars int, at, in func(k int) (int, int), last int) {
		for k := 0; k <= last; k++ {
			i, j := at(k)
			i1, j1 := in(k)
			var db0, db1 geom.Vector
			if k < n {
				db0 = ctrl[k+1].Minus(ctrl[k])
			}
			if k > 0 {
				db1 = ctrl[k].Minus(ctrl[k-1])
			}
			for d := range 3 {
				co := map[int]float64{pv(i1, j1, d): 1, pv(i, j, d): -1}
				co[vars+k] -= comp(e, d)
				co[vars+n+1] -= float64(n-k) / n * comp(db0, d)
				co[vars+n+2] -= float64(k) / n * comp(db1, d)
				eq(co, 0, hard)
			}
		}
	}
	cyl(bu.ctrl, bu.e, bv, func(k int) (int, int) { return k, 0 }, func(k int) (int, int) { return k, 1 }, n)
	// At p the row beside the cut is p itself, which holds tau and sigma
	// there at nothing: else the derivative across, short of p, would
	// lean off the cylinder by what they leave.
	cyl(ru.ctrl, ru.e, rv, func(k int) (int, int) { return n, k }, func(k int) (int, int) { return n - 1, k }, n)
	for k := range n {
		co := map[int]float64{}
		for d := range 3 {
			co[pv(1, k, d)] += comp(lu.e, d)
			co[pv(0, k, d)] -= comp(lu.e, d)
		}
		eq(co, 0, hard)
	}
	// Each side's inner row leaning into the hole by a share of its
	// length, as setbackUnit does: fairness alone can leave it along the
	// side. The hole lies along each fillet's axis e as given, and beside
	// the face's curve on the side the section leaves s into.
	lu.ctrl = up
	luSb := lu.bezier()
	bSb := bu.bezier()
	inFace := lu.e.Cross(luSb.TangentAt(0))
	if inFace.Dot(bSb.TangentAt(0)) < 0 {
		inFace = inFace.ScaledBy(-1)
	}
	for _, sd := range []struct {
		side setbackSide
		vars int
	}{{bu, bv}, {ru, rv}} {
		reach := sd.side.ctrl[n].Minus(sd.side.ctrl[0]).Magnitude() / n
		for k := 1; k < n; k++ {
			eq(map[int]float64{sd.vars + k: 1}, reach, w)
		}
	}
	reach := lu.ctrl[n].Minus(lu.ctrl[0]).Magnitude() / n
	for k := 1; k < n; k++ {
		dir := lu.e.Cross(luSb.TangentAt(float64(k) / n))
		if dir.Magnitude() < 1e-12 {
			continue
		}
		dir = dir.WithMagnitude(1)
		if dir.Dot(inFace) < 0 {
			dir = dir.ScaledBy(-1)
		}
		for d := range 3 {
			eq(map[int]float64{pv(1, k, d): 1, pv(0, k, d): -1}, comp(dir, d)*reach, w)
		}
	}
	// The thin-plate energy.
	const grid = 7
	for gu := range grid {
		for gv := range grid {
			u, v := (float64(gu)+0.5)/grid, (float64(gv)+0.5)/grid
			for d := range 3 {
				uu, uv, vv := map[int]float64{}, map[int]float64{}, map[int]float64{}
				for i := 0; i <= n; i++ {
					for j := 0; j <= n; j++ {
						k := pv(i, j, d)
						uu[k] = bernsteinD(n, i, u, 2) * bernstein(n, j, v)
						uv[k] = math.Sqrt2 * bernsteinD(n, i, u, 1) * bernsteinD(n, j, v, 1)
						vv[k] = bernstein(n, i, u) * bernsteinD(n, j, v, 2)
					}
				}
				eq(uu, 0, 1.0/grid)
				eq(uv, 0, 1.0/grid)
				eq(vv, 0, 1.0/grid)
			}
		}
	}
	x, ok := leastSquaresDense(a, b)
	if !ok {
		return srf.SSurface{}, filletErr(p, "the patch's equations have no solution")
	}
	var s srf.SSurface
	s.DegM, s.DegN = n, n
	for i := 0; i <= n; i++ {
		for j := 0; j <= n; j++ {
			s.Ctrl[i][j] = geom.Vector{X: x[pv(i, j, 0)], Y: x[pv(i, j, 1)], Z: x[pv(i, j, 2)]}.ScaledBy(size).Plus(mid)
			s.Weight[i][j] = 1
		}
	}
	for k := range n + 1 {
		s.Ctrl[k][0], s.Ctrl[n][k], s.Ctrl[k][n], s.Ctrl[0][k] = bottom.ctrl[k], right.ctrl[k], p, left.ctrl[n-k]
	}
	if err := blendPatchCheck(&s, bottom, right, left); err != nil {
		return srf.SSurface{}, err
	}
	return s, nil
}

// blendPatchCheck measures that the blend's patch is tangent to what
// lies beyond each side, on one hand of it all along, and doesn't turn
// over inside: its normal turns by less than a right angle from one
// point of a grid to the next.
func blendPatchCheck(s *srf.SSurface, bottom, right, left setbackSide) error {
	const samples = 16
	bb, rb := bottom.bezier(), right.bezier()
	for si, sd := range []struct {
		uv   func(t float64) (float64, float64)
		want func(t float64) geom.Vector
	}{
		{func(t float64) (float64, float64) { return t, 0 }, func(t float64) geom.Vector { return bottom.e.Cross(bb.TangentAt(t)) }},
		{func(t float64) (float64, float64) { return 1, t }, func(t float64) geom.Vector { return right.e.Cross(rb.TangentAt(t)) }},
		{func(t float64) (float64, float64) { return 0, t }, func(float64) geom.Vector { return left.e }},
	} {
		hand := 0.0
		for k := 0; k <= samples; k++ {
			// Short of the folded side, where the patch has no normal.
			tk := float64(k) / samples
			if si > 0 {
				tk = math.Min(tk, 1-1e-4)
			}
			u, v := sd.uv(tk)
			nm, want := s.NormalAt(u, v), sd.want(tk)
			if nm.Magnitude() < 1e-12 || want.Magnitude() < 1e-12 {
				return filletErr(s.PointAt(u, v), "the patch that would fill the corner has no normal")
			}
			c := nm.WithMagnitude(1).Dot(want.WithMagnitude(1))
			if math.Acos(math.Min(1, math.Abs(c))) > setbackTangentTol {
				return filletErr(s.PointAt(u, v), "the patch that would fill the corner is not tangent to what is round it")
			}
			if hand == 0 {
				hand = c
			} else if hand*c < 0 {
				return filletErr(s.PointAt(u, v), "the patch that would fill the corner folds")
			}
		}
	}
	const g = 48
	prev := make([]geom.Vector, g+1)
	for a := 0; a <= g; a++ {
		var below geom.Vector
		for b := 0; b <= g; b++ {
			u, v := float64(a)/g, math.Min(float64(b)/g, 1-1e-4)
			nm := s.NormalAt(u, v)
			if nm.Magnitude() < 1e-12 {
				return filletErr(s.PointAt(u, v), "the patch that would fill the corner has no normal")
			}
			nm = nm.WithMagnitude(1)
			if b > 0 && nm.Dot(below) <= 0 || a > 0 && nm.Dot(prev[b]) <= 0 {
				return filletErr(s.PointAt(u, v), "the patch that would fill the corner folds")
			}
			below, prev[b] = nm, nm
		}
	}
	return nil
}
