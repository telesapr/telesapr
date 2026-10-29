package topo

import (
	"errors"
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// Setback corners (roadmap 22.4). Where rounded edges of different radii
// meet at a corner of flat faces, or more than three rounded edges meet,
// or convex and concave ones that the stages can't round one kind at a
// time (a saddle's four), no sphere fits them all. Each fillet is cut instead at a plane square
// to its edge, set back from the corner past where the fillets'
// tangent lines cross on its faces, and the hole left is filled by
// patches tangent to everything round it.
//
// The hole has a side for each fillet, the fillet's section where it is
// cut, and one for each face between two fillets, a curve in the face
// from one fillet's tangent line to the other's, tangent to both: the
// parabola whose control point is where the lines cross. The hole is cut
// into quadrilaterals by curves across it, a star or a ladder
// (setbackPatches), each a quintic patch. Tangency is written into the patches' control
// points rather than fitted:
//
//   - along a face's curve the second row of control points lies in the
//     face's plane;
//   - along a fillet's section the second row is the first moved by
//     tau(t) times the cylinder's axis plus sigma(t) times the section's
//     own tangent, tau of the patch's degree and sigma linear, so the
//     derivative across is in the cylinder's tangent plane;
//   - across a curve between two patches the derivatives are opposite
//     but for beta(t) times the curve's tangent, beta linear, 1 at a
//     corner of the hole, where the curve leaves along the sum of the
//     legs either side, and at the far end -1 at another corner or 1 in
//     a middle point, where the curves' legs sum to nothing.
//
// What is left free is chosen to make the patches fair, least squares
// on their second differences. A section is a quintic within a few
// millionths of the radius of the circle, and the fillet's face is cut
// along it, so the patches and the fillets meet on one curve.

// setbackCorner is the kind of a corner filled by setback patches.
const setbackCorner = 5

// setbackShare is how far past where its faces' tangent lines cross a
// fillet is cut, as a share of the corner's largest radius.
const setbackShare = 0.5

// setbackTangentTol is how far, in radians, a setback patch's normal
// may turn from what lies beyond its side: more than the quintic
// sections stray, less than the seams the kernel takes as smooth.
const setbackTangentTol = 1e-3

// sideUV is the point a share t along side s (0 bottom, 1 right, 2 top,
// 3 left, each run the way the quad's loop runs it) of a patch.
func sideUV(s int, t float64) (float64, float64) {
	switch s {
	case 0:
		return t, 0
	case 1:
		return 1, t
	case 2:
		return 1 - t, 1
	}
	return 0, 1 - t
}

// patchDeg is the setback patches' degree both ways.
const patchDeg = 5

// setbackSide is a side of the hole: a quintic from one corner of the
// hole to the next, and what the patch must be tangent to along it, a
// cylinder of axis e or a plane of normal e.
type setbackSide struct {
	ctrl [patchDeg + 1]geom.Vector
	arc  bool
	e    geom.Vector
}

func (s setbackSide) bezier() srf.SBezier {
	var sb srf.SBezier
	sb.Degree = patchDeg
	for k := 0; k <= patchDeg; k++ {
		sb.Ctrl[k], sb.Weight[k] = s.ctrl[k], 1
	}
	return sb
}

// setbackRefusal reports whether err refuses a corner a setback can
// fill: fillets of different radii, more than three faces, or convex
// and concave fillets, as at a saddle's four edges, where no stage's
// fillet goes on round another's corner.
func setbackRefusal(err error) bool {
	var fx *FilletError
	if !errors.As(err, &fx) {
		return false
	}
	return strings.HasPrefix(fx.Msg, "fillets of radius") ||
		strings.HasPrefix(fx.Msg, "the edge ends where") ||
		strings.HasPrefix(fx.Msg, "a convex edge's fillet meets a concave one's")
}

// setbackCornerAt works out a setback corner at vertex v, where every
// edge is rounded, between flat faces. The edges may be of both kinds:
// each face's tangent lines lie in it whichever way its edges go, and
// the patches are tangent to the fillets and faces round the hole as
// they are at a corner of one kind.
func (t *Topology) setbackCornerAt(v int, edges []filletEdge, byEdge map[int]int) (*filletCorner, error) {
	p := t.Vertices[v].P
	var round []*filletEdge
	var radii []float64
	sharp := false
	for _, x := range t.Vertices[v].Edges {
		i, ok := byEdge[x]
		if !ok {
			sharp = true
			continue
		}
		r := edges[i].r
		known := false
		for _, y := range radii {
			known = known || math.Abs(y-r) <= geom.LengthEps
		}
		if !known {
			radii = append(radii, r)
		}
	}
	if sharp {
		slices.Sort(radii)
		names := make([]string, len(radii))
		for i, r := range radii {
			names[i] = strconv.FormatFloat(r, 'g', -1, 64)
		}
		if len(radii) == 1 {
			// Of one radius they are convex and concave, or meet at a
			// corner of more than three edges.
			return nil, filletErr(p, "fillets of radius %s meet here above an edge left sharp; round that edge too",
				names[0])
		}
		return nil, filletErr(p, "fillets of radius %s meet here above an edge left sharp; "+
			"round that edge too, or give the fillets one radius", strings.Join(names, " and "))
	}
	rmax := 0.0
	for _, x := range t.Vertices[v].Edges {
		i := byEdge[x]
		fe := &edges[i]
		if fe.circ {
			return nil, filletErr(p, "a circular edge's fillet meets other fillets of other radii here")
		}
		round = append(round, fe)
		rmax = math.Max(rmax, fe.r)
	}
	if len(round) < 3 || len(t.facesAt(v)) != len(round) {
		return nil, filletErr(p, "the corner's edges and faces don't go round it one after another")
	}
	normal := map[int]geom.Vector{}
	for _, f := range t.facesAt(v) {
		n, ok := t.planarNormal(f)
		if !ok {
			return nil, filletErr(p, "a face at the corner is curved")
		}
		normal[f] = n
	}
	away := func(fe *filletEdge) geom.Vector {
		if fe.v[1] == v {
			return fe.dir.ScaledBy(-1)
		}
		return fe.dir
	}
	kOf := func(fe *filletEdge, f int) int {
		if fe.f[0] == f {
			return 0
		}
		return 1
	}
	// The other rounded edge at v on face f.
	other := func(fe *filletEdge, f int) *filletEdge {
		for _, g := range round {
			if g != fe && (g.f[0] == f || g.f[1] == f) {
				return g
			}
		}
		return nil
	}
	// Where the tangent lines of two fillets cross on their face.
	cross := map[int]geom.Vector{}
	for _, f := range t.facesAt(v) {
		var two []*filletEdge
		for _, g := range round {
			if g.f[0] == f || g.f[1] == f {
				two = append(two, g)
			}
		}
		if len(two) != 2 {
			return nil, filletErr(p, "a face at the corner doesn't lie between two rounded edges")
		}
		a, b := two[0], two[1]
		pa := p.Plus(a.u[kOf(a, f)].ScaledBy(a.ss[kOf(a, f)]))
		pb := p.Plus(b.u[kOf(b, f)].ScaledBy(b.ss[kOf(b, f)]))
		x, ok := lineMeet(pa, away(a), pb, away(b))
		if !ok {
			return nil, filletErr(p, "two fillets at the corner run along one another")
		}
		cross[f] = x
	}
	// Each fillet is cut past where its tangent lines cross on both its
	// faces.
	setback := map[int]float64{}
	for _, fe := range round {
		d := 0.0
		for k := range 2 {
			base := p.Plus(fe.u[k].ScaledBy(fe.ss[k]))
			d = math.Max(d, cross[fe.f[k]].Minus(base).Dot(away(fe)))
		}
		setback[fe.e] = d + setbackShare*rmax
	}
	c := &filletCorner{kind: setbackCorner, at: map[[2]int]geom.Vector{}, long: -1, r: rmax,
		arcs: map[int]srf.SBezier{}, faceCurves: map[int]srf.SBezier{}}
	point := func(fe *filletEdge, f int) geom.Vector {
		k := kOf(fe, f)
		return p.Plus(away(fe).ScaledBy(setback[fe.e])).Plus(fe.u[k].ScaledBy(fe.ss[k]))
	}
	for _, fe := range round {
		for k := range 2 {
			c.at[[2]int{fe.e, fe.f[k]}] = point(fe, fe.f[k])
		}
		mid := p.Plus(away(fe).ScaledBy(setback[fe.e]))
		arc, err := quinticArc(fe.section(mid, false))
		if err != nil {
			return nil, filletErr(mid, "the fillet's section at the corner can't be made: %v", err)
		}
		c.arcs[fe.e] = arc
	}
	var facing geom.Vector
	for _, n := range normal {
		facing = facing.Plus(n)
	}
	// The hole's sides in order round it: a fillet's section from one of
	// its faces to the other, then that face's curve to the next fillet.
	// Each face's curve is tried as the parabola first, then, where the
	// patches fold, hugging the chord between its ends more
	// (setbackLegs).
	var firstErr error
	for _, leg := range setbackLegs {
		clear(c.faceCurves)
		fe := round[0]
		f := fe.f[0]
		var sides []setbackSide
		for range round {
			g := fe.f[1-kOf(fe, f)]
			arc := c.arcs[fe.e]
			if !arc.Start().Equals(point(fe, f)) {
				arc.Reverse()
			}
			sides = append(sides, setbackSide{ctrl: quinticCtrl(arc), arc: true, e: fe.dir})
			next := other(fe, g)
			if next == nil {
				return nil, filletErr(p, "the corner's edges don't go round it one after another")
			}
			fc, err := setbackFaceCurve(point(fe, g), cross[g], point(next, g), leg)
			if err != nil {
				return nil, filletErr(p, "the corner's curve on a face can't be made: %v", err)
			}
			c.faceCurves[g] = fc
			sides = append(sides, setbackSide{ctrl: quinticCtrl(fc), e: normal[g]})
			fe, f = next, g
		}
		if fe != round[0] {
			return nil, filletErr(p, "the corner's edges don't go round it one after another")
		}
		quads, err := setbackPatches(sides, facing)
		if err == nil {
			c.quads = quads
			return c, nil
		}
		if firstErr == nil {
			firstErr = err
		}
	}
	return nil, filletErr(p, "the corner can't be filled: %v", firstErr)
}

// setbackLegs are the shares of the way from each end to where the
// tangent lines cross that a face's curve round a setback corner leaves
// along, tried in turn (setbackFaceCurve): 0 for the parabola through
// the crossing. Beside a fillet five or six times the radius of the next
// the parabola's legs are as unequal, and the patches along it, slow at
// one end and fast at the other, fold beside it; legs a share of the
// shorter one's length keep the curve near the chord, and the patches
// fair.
var setbackLegs = []float64{0, 0.4, 0.2}

// setbackFaceCurve is the quintic on a face from a to b, tangent at each
// to the line to x, where the two fillets' tangent lines cross: the
// parabola with x its control point for leg 0, else the cubic whose legs
// are leg times the shorter of the two.
func setbackFaceCurve(a, x, b geom.Vector, leg float64) (srf.SBezier, error) {
	sb := srf.SBezierFrom3(a, x, b)
	if leg > 0 {
		l := leg * math.Min(x.Minus(a).Magnitude(), x.Minus(b).Magnitude())
		sb = srf.SBezierFrom4(a, a.Plus(x.Minus(a).WithMagnitude(l)), b.Plus(x.Minus(b).WithMagnitude(l)), b)
	}
	return sb.ElevatedTo(patchDeg)
}

func quinticCtrl(sb srf.SBezier) [patchDeg + 1]geom.Vector {
	var out [patchDeg + 1]geom.Vector
	for k := 0; k <= patchDeg; k++ {
		out[k] = sb.Ctrl[k]
	}
	return out
}

// quinticArc is a polynomial quintic close to the circular arc sb, from
// its start to its finish and tangent to it there: least squares on
// points spread evenly along the arc, the quintic's parameter the share
// of its length.
func quinticArc(sb srf.SBezier) (srf.SBezier, error) {
	const dense = 512
	pts := make([]geom.Vector, dense+1)
	lens := make([]float64, dense+1)
	for k := 0; k <= dense; k++ {
		pts[k] = sb.PointAt(float64(k) / dense)
		if k > 0 {
			lens[k] = lens[k-1] + pts[k].Minus(pts[k-1]).Magnitude()
		}
	}
	total := lens[dense]
	if total < 10*geom.LengthEps {
		return srf.SBezier{}, errNoArc
	}
	at := func(s float64) geom.Vector {
		k := 1
		for k < dense && lens[k] < s {
			k++
		}
		f := (s - lens[k-1]) / (lens[k] - lens[k-1])
		return pts[k-1].Plus(pts[k].Minus(pts[k-1]).ScaledBy(f))
	}
	p, q := sb.Start(), sb.Finish()
	tp := sb.TangentAt(0).WithMagnitude(1)
	tq := sb.TangentAt(1).WithMagnitude(1)
	// Unknowns: the legs' lengths at each end, and the two middle points.
	var a [][]float64
	var b []float64
	const samples = 64
	for s := 1; s < samples; s++ {
		u := float64(s) / samples
		x := at(u * total)
		var bb [6]float64
		for i := range bb {
			bb[i] = bernstein(patchDeg, i, u)
		}
		for d := range 3 {
			row := make([]float64, 8)
			row[0] = bb[1] * comp(tp, d)
			row[1] = -bb[4] * comp(tq, d)
			row[2+d] = bb[2]
			row[5+d] = bb[3]
			a = append(a, row)
			b = append(b, comp(x, d)-(bb[0]+bb[1])*comp(p, d)-(bb[4]+bb[5])*comp(q, d))
		}
	}
	x, ok := leastSquaresDense(a, b)
	if !ok {
		return srf.SBezier{}, errNoArc
	}
	var out srf.SBezier
	out.Degree = patchDeg
	for k := 0; k <= patchDeg; k++ {
		out.Weight[k] = 1
	}
	out.Ctrl[0], out.Ctrl[5] = p, q
	out.Ctrl[1] = p.Plus(tp.ScaledBy(x[0]))
	out.Ctrl[2] = geom.Vector{X: x[2], Y: x[3], Z: x[4]}
	out.Ctrl[3] = geom.Vector{X: x[5], Y: x[6], Z: x[7]}
	out.Ctrl[4] = q.Minus(tq.ScaledBy(x[1]))
	return out, nil
}

var errNoArc = filletErr(geom.Vector{}, "the section is too short")

func comp(v geom.Vector, d int) float64 {
	switch d {
	case 0:
		return v.X
	case 1:
		return v.Y
	}
	return v.Z
}

// bernsteinD is the m-th derivative, m at most 2, of the Bernstein
// polynomial i of degree n at t.
func bernsteinD(n, i int, t float64, m int) float64 { //nolint:unparam // n is the degree, as bernstein takes it; the patches are all of one degree
	b := func(k, j int) float64 {
		if j < 0 || j > k {
			return 0
		}
		return bernstein(k, j, t)
	}
	switch m {
	case 0:
		return b(n, i)
	case 1:
		return float64(n) * (b(n-1, i-1) - b(n-1, i))
	}
	return float64(n*(n-1)) * (b(n-2, i-2) - 2*b(n-2, i-1) + b(n-2, i))
}

// leastSquaresDense solves a x = b in the least-squares sense, by the
// normal equations and elimination with partial pivoting.
func leastSquaresDense(a [][]float64, b []float64) ([]float64, bool) {
	if len(a) == 0 {
		return nil, false
	}
	n := len(a[0])
	m := make([][]float64, n)
	r := make([]float64, n)
	for i := range m {
		m[i] = make([]float64, n)
	}
	for row := range a {
		for i, ai := range a[row] {
			if ai == 0 {
				continue
			}
			r[i] += ai * b[row]
			for j, aj := range a[row] {
				if aj != 0 {
					m[i][j] += ai * aj
				}
			}
		}
	}
	for i := range n {
		piv := i
		for k := i + 1; k < n; k++ {
			if math.Abs(m[k][i]) > math.Abs(m[piv][i]) {
				piv = k
			}
		}
		if math.Abs(m[piv][i]) < 1e-300 {
			return nil, false
		}
		m[i], m[piv] = m[piv], m[i]
		r[i], r[piv] = r[piv], r[i]
		for k := i + 1; k < n; k++ {
			f := m[k][i] / m[i][i]
			if f == 0 {
				continue
			}
			for j := i; j < n; j++ {
				m[k][j] -= f * m[i][j]
			}
			r[k] -= f * r[i]
		}
	}
	x := make([]float64, n)
	for i := n - 1; i >= 0; i-- {
		s := r[i]
		for j := i + 1; j < n; j++ {
			s -= m[i][j] * x[j]
		}
		x[i] = s / m[i][i]
	}
	return x, true
}

// setbackPatches fills the hole bounded by sides, each from one corner
// of the hole to the next, with quintic patches tangent to what lies
// beyond each side and to one another. The sides must be even in
// number, six at least.
func setbackPatches(sides []setbackSide, out geom.Vector) ([]srf.SSurface, error) {
	// Every corner of the hole to start from is tried, and the patches
	// whose normals turn least kept: a layout that suits one hole folds
	// another.
	var best []srf.SSurface
	bestQ := math.Inf(-1)
	var firstErr error
	ns := len(sides)
	for start := range ns {
		if (ns/2)%2 == 1 && start > 1 {
			break
		}
		rot := append(append([]setbackSide{}, sides[start:]...), sides[:start]...)
		ps, err := setbackLayout(rot)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if q := foldFree(ps, out); q > bestQ {
			best, bestQ = ps, q
		}
	}
	if best == nil {
		return nil, firstErr
	}
	if bestQ <= 0 {
		return nil, filletErr(sides[0].ctrl[0], "the patches that would fill it fold")
	}
	return best, nil
}

// foldFree is the least, over the patches, of the cosine between the
// normal and out, the way the corner faces, each patch's normal turned
// to face it at its middle: a patch with a normal that turns away
// folds.
func foldFree(ps []srf.SSurface, out geom.Vector) float64 {
	worst := 1.0
	const g = 24
	out = out.WithMagnitude(1)
	for i := range ps {
		sign := 1.0
		if ps[i].NormalAt(0.5, 0.5).Dot(out) < 0 {
			sign = -1
		}
		// The grid, and close beside each side, where a patch whose
		// derivative across nearly vanishes turns over.
		ts := []float64{1e-4, 1e-3, 1e-2}
		for a := 0; a <= g; a++ {
			ts = append(ts, float64(a)/g)
		}
		ts = append(ts, 1-1e-2, 1-1e-3, 1-1e-4)
		for _, u := range ts {
			for _, v := range ts {
				nm := ps[i].NormalAt(u, v)
				if nm.Magnitude() < 1e-12 {
					return -1
				}
				worst = math.Min(worst, sign*nm.WithMagnitude(1).Dot(out))
			}
		}
	}
	return worst
}

// setbackLayout is setbackPatches with the layout started at the
// sides' first corner.
func setbackLayout(sides []setbackSide) ([]srf.SSurface, error) {
	// Solved in coordinates about the hole's middle with its size 1: the
	// equations' weights are then alike at every size.
	var mid geom.Vector
	size := 0.0
	for _, sd := range sides {
		mid = mid.Plus(sd.ctrl[0])
	}
	mid = mid.ScaledBy(1 / float64(len(sides)))
	for _, sd := range sides {
		for _, p := range sd.ctrl {
			size = math.Max(size, p.Minus(mid).Magnitude())
		}
	}
	if size < 10*geom.LengthEps {
		return nil, filletErr(mid, "the corner's hole is too small")
	}
	unit := make([]setbackSide, len(sides))
	for i, sd := range sides {
		unit[i] = sd
		for k := range sd.ctrl {
			unit[i].ctrl[k] = sd.ctrl[k].Minus(mid).ScaledBy(1 / size)
		}
	}
	ps, err := setbackUnit(unit)
	if err != nil {
		if fx, ok := errors.AsType[*FilletError](err); ok {
			return nil, filletErr(fx.At.ScaledBy(size).Plus(mid), "%s", fx.Msg)
		}
		return nil, err
	}
	for q := range ps {
		for i := 0; i <= ps[q].DegM; i++ {
			for j := 0; j <= ps[q].DegN; j++ {
				ps[q].Ctrl[i][j] = ps[q].Ctrl[i][j].ScaledBy(size).Plus(mid)
			}
		}
	}
	// The sides' points exactly as given, not as scaled back: the faces
	// round the hole are cut along the same curves.
	var given []geom.Vector
	for _, sd := range sides {
		given = append(given, sd.ctrl[:]...)
	}
	for q := range ps {
		n := ps[q].DegM
		for i := 0; i <= n; i++ {
			for j := 0; j <= n; j++ {
				if i != 0 && i != n && j != 0 && j != n {
					continue
				}
				for _, g := range given {
					if g.Minus(ps[q].Ctrl[i][j]).Magnitude() < 1e-9*size {
						ps[q].Ctrl[i][j] = g
						break
					}
				}
			}
		}
	}
	return ps, nil
}

// setbackUnit is setbackLayout on sides about the origin of size 1.
func setbackUnit(sides []setbackSide) ([]srf.SSurface, error) {
	const n = patchDeg
	ns := len(sides)
	if ns < 6 || ns%2 != 0 {
		return nil, filletErr(sides[0].ctrl[0], "a corner's hole of %d sides", ns)
	}
	type ref struct {
		hole, diag int
		rev        bool
	}
	var quads [][4]ref
	// beta1 is beta at a curve's far end: -1 where it ends at a corner of
	// the hole, 1 where it ends in the middle.
	var beta1 []float64
	h := func(i int) ref { return ref{hole: (i + ns) % ns, diag: -1} }
	if k := ns / 2; k%2 == 1 {
		// An odd number of fillets: a star, curves from every other corner
		// of the hole to a middle point, a patch for each fillet, with its
		// section and the face's curve after it. Three curves or five meet
		// in the middle, where an odd number can be tangent all round.
		for m := range k {
			quads = append(quads, [4]ref{h(2 * m), h(2*m + 1),
				{hole: -1, diag: (m + 1) % k}, {hole: -1, diag: m, rev: true}})
			beta1 = append(beta1, 1)
		}
	} else {
		// An even number: a ladder of rungs across the hole, each from a
		// corner to a corner, its ends corners of two quads only. The
		// first quad has the three sides before the first rung, each
		// next one the rung before, a side on each hand and a new rung,
		// the last the rung before and three sides.
		nq := k - 1
		quads = append(quads, [4]ref{h(0), h(1), h(2), {hole: -1, diag: 0, rev: true}})
		beta1 = append(beta1, -1)
		ra, rb := 0, 3
		for q := 1; q < nq; q++ {
			prev := ref{hole: -1, diag: q - 1}
			if q == nq-1 {
				quads = append(quads, [4]ref{prev, h(rb), h(rb + 1), h(rb + 2)})
				break
			}
			na, nb := (ra-1+ns)%ns, rb+1
			quads = append(quads, [4]ref{prev, h(rb), {hole: -1, diag: q, rev: true}, h(na)})
			beta1 = append(beta1, -1)
			ra, rb = na, nb
		}
	}
	nq := len(quads)

	nPts := (n + 1) * (n + 1) * 3
	pv := func(q, i, j, d int) int { return q*nPts + (i*(n+1)+j)*3 + d }
	// Each fillet's side has n+1 values of tau and two of sigma.
	arcVar := map[int]int{}
	nv := nq * nPts
	for i, s := range sides {
		if s.arc {
			arcVar[i] = nv
			nv += n + 3
		}
	}
	var a [][]float64
	var b []float64
	scale := 0.0
	for _, s := range sides {
		scale = math.Max(scale, s.ctrl[n].Minus(s.ctrl[0]).Magnitude())
	}
	hard := 1e6
	eq := func(co map[int]float64, rhs, w float64) {
		row := make([]float64, nv)
		for k, x := range co {
			row[k] += x * w
		}
		a = append(a, row)
		b = append(b, rhs*w)
	}
	// slot is where along quad side s (0 bottom, 1 right, 2 top, 3 left)
	// its k-th control point in the side's own order lies, and the point
	// beside it, inside the patch.
	slot := func(side, k int) (int, int, int, int) {
		switch side {
		case 0:
			return k, 0, k, 1
		case 1:
			return n, k, n - 1, k
		case 2:
			return n - k, n, n - k, n - 1
		}
		return 0, n - k, 1, n - k
	}
	// The spokes: each one's points are those of the first quad
	// that has it, run its own way.
	diagOwner := map[int][3]int{} // quad, side, rev
	for q, sd := range quads {
		for s, r := range sd {
			if r.diag < 0 {
				continue
			}
			rv := 0
			if r.rev {
				rv = 1
			}
			if o, ok := diagOwner[r.diag]; ok {
				// Tie this quad's points along the curve to the owner's.
				for k := 0; k <= n; k++ {
					kk := k
					if (rv == 1) != (o[2] == 1) {
						kk = n - k
					}
					i1, j1, _, _ := slot(s, k)
					i0, j0, _, _ := slot(o[1], kk)
					for d := range 3 {
						eq(map[int]float64{pv(q, i1, j1, d): 1, pv(o[0], i0, j0, d): -1}, 0, hard)
					}
				}
				// Tangent across: in the curve's own order, the two rows
				// beside it opposite but for beta times its tangent.
				for k := 0; k <= n; k++ {
					ka, kb := k, k // indices in each quad's side order
					if o[2] == 1 {
						ka = n - k
					}
					if rv == 1 {
						kb = n - k
					}
					ia, ja, ia1, ja1 := slot(o[1], ka)
					ib, jb, ib1, jb1 := slot(s, kb)
					_ = ib
					_ = jb
					// The curve's points in its own order.
					dk := func(m int) (int, int) {
						mm := m
						if o[2] == 1 {
							mm = n - m
						}
						i, j, _, _ := slot(o[1], mm)
						return i, j
					}
					for d := range 3 {
						co := map[int]float64{}
						co[pv(o[0], ia1, ja1, d)]++
						co[pv(q, ib1, jb1, d)]++
						co[pv(o[0], ia, ja, d)] -= 2
						if k < n {
							f := float64(n-k) / n
							i2, j2 := dk(k + 1)
							co[pv(o[0], i2, j2, d)] -= f
							co[pv(o[0], ia, ja, d)] += f
						}
						if k > 0 {
							f := beta1[r.diag] * float64(k) / n
							i2, j2 := dk(k - 1)
							co[pv(o[0], ia, ja, d)] -= f
							co[pv(o[0], i2, j2, d)] += f
						}
						eq(co, 0, hard)
					}
				}
				continue
			}
			diagOwner[r.diag] = [3]int{q, s, rv}
		}
	}
	// The hole's sides.
	for q, sd := range quads {
		for s, r := range sd {
			if r.hole < 0 {
				continue
			}
			side := sides[r.hole]
			ctrl := side.ctrl
			// In the quad's order: sides 2 and 3 run backwards.
			for k := 0; k <= n; k++ {
				i, j, _, _ := slot(s, k)
				for d := range 3 {
					eq(map[int]float64{pv(q, i, j, d): 1}, comp(ctrl[k], d), hard)
				}
			}
			if !side.arc {
				for k := 0; k <= n; k++ {
					i, j, i1, j1 := slot(s, k)
					co := map[int]float64{}
					for d := range 3 {
						co[pv(q, i1, j1, d)] += comp(side.e, d)
						co[pv(q, i, j, d)] -= comp(side.e, d)
					}
					eq(co, 0, hard)
				}
				continue
			}
			base := arcVar[r.hole]
			for k := 0; k <= n; k++ {
				i, j, i1, j1 := slot(s, k)
				var db0, db1 geom.Vector
				if k < n {
					db0 = ctrl[k+1].Minus(ctrl[k])
				}
				if k > 0 {
					db1 = ctrl[k].Minus(ctrl[k-1])
				}
				for d := range 3 {
					co := map[int]float64{pv(q, i1, j1, d): 1, pv(q, i, j, d): -1}
					co[base+k] -= comp(side.e, d)
					co[base+n+1] -= float64(n-k) / n * comp(db0, d)
					co[base+n+2] -= float64(k) / n * comp(db1, d)
					eq(co, 0, hard)
				}
			}
		}
	}
	// Fairness.
	w := 1 / scale
	// And along each side of the hole, the row beside it leaning into the
	// hole, square to the side, by a share of its length: fairness alone
	// can leave it along the side, where the patch has no normal.
	var centre geom.Vector
	for _, sd := range sides {
		centre = centre.Plus(sd.ctrl[0])
	}
	centre = centre.ScaledBy(1 / float64(ns))
	for q, sd := range quads {
		for s, r := range sd {
			if r.hole < 0 {
				continue
			}
			side := sides[r.hole]
			sb := side.bezier()
			reach := side.ctrl[n].Minus(side.ctrl[0]).Magnitude() / n
			for k := 1; k < n; k++ {
				tk := float64(k) / n
				pt := sb.PointAt(tk)
				tan := sb.TangentAt(tk)
				var in geom.Vector
				if side.arc {
					in = side.e
				} else {
					in = side.e.Cross(tan)
				}
				if in.Magnitude() < 1e-12 {
					continue
				}
				in = in.WithMagnitude(1)
				if in.Dot(centre.Minus(pt)) < 0 {
					in = in.ScaledBy(-1)
				}
				if side.arc {
					sign := 1.0
					if side.e.Dot(in) < 0 {
						sign = -1
					}
					eq(map[int]float64{arcVar[r.hole] + k: 1}, sign*reach, w)
					continue
				}
				i, j, i1, j1 := slot(s, k)
				for d := range 3 {
					eq(map[int]float64{pv(q, i1, j1, d): 1, pv(q, i, j, d): -1}, comp(in, d)*reach, w)
				}
			}
		}
	}
	// The thin-plate energy of each patch, its second derivatives at
	// points spread over it.
	const g = 7
	for q := range quads {
		for gu := range g {
			for gv := range g {
				u, v := (float64(gu)+0.5)/g, (float64(gv)+0.5)/g
				for d := range 3 {
					uu, uv, vv := map[int]float64{}, map[int]float64{}, map[int]float64{}
					for i := 0; i <= n; i++ {
						for j := 0; j <= n; j++ {
							k := pv(q, i, j, d)
							uu[k] = bernsteinD(n, i, u, 2) * bernstein(n, j, v)
							uv[k] = math.Sqrt2 * bernsteinD(n, i, u, 1) * bernsteinD(n, j, v, 1)
							vv[k] = bernstein(n, i, u) * bernsteinD(n, j, v, 2)
						}
					}
					eq(uu, 0, w/g)
					eq(uv, 0, w/g)
					eq(vv, 0, w/g)
				}
			}
		}
	}
	x, ok := leastSquaresDense(a, b)
	if !ok {
		return nil, filletErr(sides[0].ctrl[0], "the patches' equations have no solution")
	}
	out := make([]srf.SSurface, nq)
	for q := range quads {
		s := &out[q]
		s.DegM, s.DegN = n, n
		for i := 0; i <= n; i++ {
			for j := 0; j <= n; j++ {
				s.Ctrl[i][j] = geom.Vector{X: x[pv(q, i, j, 0)], Y: x[pv(q, i, j, 1)], Z: x[pv(q, i, j, 2)]}
				s.Weight[i][j] = 1
			}
		}
	}
	// The sides exactly as given, and each spoke one curve.
	for q, sd := range quads {
		for s, r := range sd {
			if r.hole >= 0 {
				for k := 0; k <= n; k++ {
					i, j, _, _ := slot(s, k)
					out[q].Ctrl[i][j] = sides[r.hole].ctrl[k]
				}
			}
		}
	}
	for q, sd := range quads {
		for s, r := range sd {
			if r.diag < 0 {
				continue
			}
			o := diagOwner[r.diag]
			if o[0] == q {
				continue
			}
			rv := 0
			if r.rev {
				rv = 1
			}
			for k := 0; k <= n; k++ {
				kk := k
				if (rv == 1) != (o[2] == 1) {
					kk = n - k
				}
				i1, j1, _, _ := slot(s, k)
				i0, j0, _, _ := slot(o[1], kk)
				out[q].Ctrl[i1][j1] = out[o[0]].Ctrl[i0][j0]
			}
		}
	}
	// Each patch tangent to what lies beyond each of its sides, measured:
	// a patch can satisfy every equation and still turn its normal where
	// its derivative across nearly runs along the side.
	const samples = 16
	for q, sd := range quads {
		for si, r := range sd {
			for k := 0; k <= samples; k++ {
				tk := float64(k) / samples
				u, v := sideUV(si, tk)
				nm := out[q].NormalAt(u, v)
				if nm.Magnitude() < 1e-12 {
					return nil, filletErr(out[q].PointAt(u, v), "a patch that would fill it has no normal")
				}
				var want geom.Vector
				switch {
				case r.hole >= 0 && sides[r.hole].arc:
					sb := sides[r.hole].bezier()
					want = sides[r.hole].e.Cross(sb.TangentAt(tk))
				case r.hole >= 0:
					want = sides[r.hole].e
				default:
					// The other patch along the spoke, at the same point.
					o := -1
					for q2, sd2 := range quads {
						for _, r2 := range sd2 {
							if q2 != q && r2.hole < 0 && r2.diag == r.diag {
								o = q2
							}
						}
					}
					if o < 0 {
						continue
					}
					p := out[q].PointAt(u, v)
					var uv geom.Point2d
					out[o].ClosestPointTo(p, &uv, false)
					want = out[o].NormalAtUv(uv)
				}
				if want.Magnitude() < 1e-12 {
					continue
				}
				c := math.Abs(nm.WithMagnitude(1).Dot(want.WithMagnitude(1)))
				if math.Acos(math.Min(1, c)) > setbackTangentTol {
					return nil, filletErr(out[q].PointAt(u, v), "the patches that would fill it are not tangent to what is round them")
				}
				// And just inside: a patch whose derivative across nearly
				// vanishes is tangent on the side and turns sharply beside it.
				const in = 1e-3
				iu, iv := u, v
				switch si {
				case 0:
					iv += in
				case 1:
					iu -= in
				case 2:
					iv -= in
				default:
					iu += in
				}
				ni := out[q].NormalAt(iu, iv)
				if ni.Magnitude() < 1e-12 || math.Acos(math.Min(1, math.Abs(ni.WithMagnitude(1).Dot(want.WithMagnitude(1))))) > 10*setbackTangentTol {
					return nil, filletErr(out[q].PointAt(u, v), "the patches that would fill it fold")
				}
			}
		}
	}
	return out, nil
}
