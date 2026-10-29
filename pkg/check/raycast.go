package check

import (
	"errors"
	"fmt"
	"math"
	"slices"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/srf"
)

// The ray-cast oracle judges a solid without the kernel's surface
// intersection: it rebuilds the group tree from each group's own
// definition (a profile and an extrusion, a profile and an axis, a
// sweep's rigid steps or its path and frames, the motions that place a pattern's or a
// mirror's copies), finds
// where a ray may cross each leaf's boundary, and evaluates the tree
// between those places, point by point. Volumes, centroids and where a
// line passes through the solid then come from rays alone, and a
// disagreement with the B-rep's mesh says where one of them is wrong.

// ErrUnsupported is returned for a group tree with a leaf the oracle
// can't build: anything but extrusions, drafted or not, lathes,
// revolves, helices of a profile in a plane through their axis, sweeps
// (along lines and arcs, or fitted along cubics, twisted or held to a
// direction), and the step and repeats, patterns along a path and
// mirrors of those. Lofts and the solids made by editing others
// (fillets, chamfers, shells) are refused.
var ErrUnsupported = errors.New("the ray-cast oracle builds extrusions, lathes, revolves, helices, sweeps, and their copies only")

// profileTol is the chord tolerance the profiles' curves are cut to:
// fine enough that the oracle's own error is far below any mesh's.
const profileTol = 1e-5

// Interval is a stretch of a ray inside the solid, from From to To in
// the ray's parameter.
type Interval struct{ From, To float64 }

// node is a solid of the tree: a leaf or a Boolean of two.
type node interface {
	inside(x geom.Vector) bool
	// crossings appends the parameters s at which the ray o + s d may
	// cross the boundary: every crossing, and perhaps more.
	crossings(o, d geom.Vector, out []float64) []float64
	bounds(lo, hi *geom.Vector)
}

// boolean is two solids combined as a group combines its solid with
// the one before.
type boolean struct {
	a, b node
	how  sketch.CombineAs
}

func (n *boolean) inside(x geom.Vector) bool {
	a, b := n.a.inside(x), n.b.inside(x)
	switch n.how {
	case sketch.CombineDifference:
		return a && !b
	case sketch.CombineIntersection:
		return a && b
	default:
		return a || b
	}
}

func (n *boolean) crossings(o, d geom.Vector, out []float64) []float64 {
	return n.b.crossings(o, d, n.a.crossings(o, d, out))
}

func (n *boolean) bounds(lo, hi *geom.Vector) {
	n.a.bounds(lo, hi)
	if n.how != sketch.CombineDifference && n.how != sketch.CombineIntersection {
		n.b.bounds(lo, hi)
	}
}

// profile is a loop set cut to polygons in a plane, with 2D
// coordinates along eu and ev from origin.
type profile struct {
	origin, eu, ev, n geom.Vector
	polys             [][][2]float64
}

// newProfile cuts the loop set's curves into polygons.
func newProfile(sbls *srf.SBezierLoopSet, origin, eu, ev geom.Vector) profile {
	pr := profile{origin: origin, eu: eu, ev: ev, n: eu.Cross(ev)}
	loops := append([]srf.SBezierLoop{sbls.Outer}, sbls.Holes...)
	for _, l := range loops {
		var poly [][2]float64
		for i := range l.Curves {
			var pts []geom.Vector
			l.Curves[i].MakePwlIntoPoints(&pts, srf.Tol(profileTol, 100000), 0)
			for _, p := range pts[:len(pts)-1] {
				q := p.Minus(origin)
				poly = append(poly, [2]float64{q.Dot(eu), q.Dot(ev)})
			}
		}
		if len(poly) >= 3 {
			pr.polys = append(pr.polys, poly)
		}
	}
	return pr
}

// contains is the even-odd rule: (a, b) is inside when a ray from it
// crosses the polygons' edges an odd number of times.
func (pr *profile) contains(a, b float64) bool {
	in := false
	for _, poly := range pr.polys {
		for i := range poly {
			p, q := poly[i], poly[(i+1)%len(poly)]
			if (p[1] > b) != (q[1] > b) && a < p[0]+(b-p[1])*(q[0]-p[0])/(q[1]-p[1]) {
				in = !in
			}
		}
	}
	return in
}

// prism is an extrusion of a profile: its points are the profile's
// moved by tbot plus t times dir, t from 0 to 1.
type prism struct {
	pr       profile
	base     geom.Vector
	dir      geom.Vector
	dirDotN  float64
	cornerLo geom.Vector
	cornerHi geom.Vector
	// cuts, where set, end the prism in their place of t from 0 to 1:
	// a sweep's run mitred at a corner (newCutPrism).
	cuts []halfSpace
}

func newPrism(pr profile, tbot, ttop geom.Vector) *prism {
	p := &prism{pr: pr, base: pr.origin.Plus(tbot), dir: ttop.Minus(tbot)}
	p.dirDotN = p.dir.Dot(pr.n)
	p.cornerLo = geom.Vector{X: math.Inf(1), Y: math.Inf(1), Z: math.Inf(1)}
	p.cornerHi = p.cornerLo.ScaledBy(-1)
	for _, poly := range pr.polys {
		for _, v := range poly {
			q := p.base.Plus(pr.eu.ScaledBy(v[0])).Plus(pr.ev.ScaledBy(v[1]))
			q.MakeMaxMin(&p.cornerHi, &p.cornerLo)
			q.Plus(p.dir).MakeMaxMin(&p.cornerHi, &p.cornerLo)
		}
	}
	return p
}

// coords are x's place along the extrusion and in the profile.
func (p *prism) coords(x geom.Vector) (t, a, b float64) {
	r := x.Minus(p.base)
	t = r.Dot(p.pr.n) / p.dirDotN
	q := r.Minus(p.dir.ScaledBy(t))
	return t, q.Dot(p.pr.eu), q.Dot(p.pr.ev)
}

func (p *prism) inside(x geom.Vector) bool {
	t, a, b := p.coords(x)
	if p.cuts != nil {
		for _, h := range p.cuts {
			if !h.holds(x) {
				return false
			}
		}
		return p.pr.contains(a, b)
	}
	return t >= 0 && t <= 1 && p.pr.contains(a, b)
}

func (p *prism) crossings(o, d geom.Vector, out []float64) []float64 {
	t0, a0, b0 := p.coords(o)
	t1, a1, b1 := p.coords(o.Plus(d))
	dt, da, db := t1-t0, a1-a0, b1-b0
	switch {
	case p.cuts != nil:
		for _, h := range p.cuts {
			out = h.crossing(o, d, out)
		}
	case dt != 0:
		out = append(out, -t0/dt, (1-t0)/dt)
	}
	for _, poly := range p.pr.polys {
		for i := range poly {
			e0, e1 := poly[i], poly[(i+1)%len(poly)]
			// a0 + s da = e0 + m (e1 - e0), the same for b.
			ex, ey := e1[0]-e0[0], e1[1]-e0[1]
			den := da*ey - db*ex
			if den == 0 {
				continue
			}
			s := ((e0[0]-a0)*ey - (e0[1]-b0)*ex) / den
			// Where along the edge: kept only on it, give or take.
			m := ((e0[0]-a0)*db - (e0[1]-b0)*da) / den
			if m >= -1e-9 && m <= 1+1e-9 {
				out = append(out, s)
			}
		}
	}
	return out
}

func (p *prism) bounds(lo, hi *geom.Vector) {
	p.cornerLo.MakeMaxMin(hi, lo)
	p.cornerHi.MakeMaxMin(hi, lo)
}

// revolution is a profile turned about an axis through c along w: in
// (z, r), along the axis and from it, its points are the profile's; a
// revolve keeps to the angles from phi0 to phi1, measured about w from
// e, the side of the axis the profile lies on.
type revolution struct {
	pr         profile // eu is w, ev is e
	c, w, e, f geom.Vector
	full       bool
	phi0, phi1 float64
	zLo, zHi   float64
	rHi        float64
}

func newRevolution(sbls *srf.SBezierLoopSet, c, w geom.Vector, full bool, phi0, phi1 float64) *revolution {
	// The profile's side of the axis: from the axis to the profile's
	// point furthest from it.
	var e geom.Vector
	far := -1.0
	for _, l := range append([]srf.SBezierLoop{sbls.Outer}, sbls.Holes...) {
		for _, cv := range l.Curves {
			for k := 0; k <= cv.Degree; k++ {
				q := cv.Ctrl[k].Minus(c)
				rad := q.Minus(w.ScaledBy(q.Dot(w)))
				if m := rad.Magnitude(); m > far {
					far, e = m, rad
				}
			}
		}
	}
	e = e.WithMagnitude(1)
	rv := &revolution{pr: newProfile(sbls, c, w, e), c: c, w: w, e: e, f: w.Cross(e), full: full, phi0: phi0, phi1: phi1}
	rv.zLo, rv.zHi = math.Inf(1), math.Inf(-1)
	for _, poly := range rv.pr.polys {
		for _, v := range poly {
			rv.zLo, rv.zHi = math.Min(rv.zLo, v[0]), math.Max(rv.zHi, v[0])
			rv.rHi = math.Max(rv.rHi, math.Abs(v[1]))
		}
	}
	return rv
}

func (rv *revolution) inside(x geom.Vector) bool {
	q := x.Minus(rv.c)
	z := q.Dot(rv.w)
	rad := q.Minus(rv.w.ScaledBy(z))
	r := rad.Magnitude()
	if !rv.full {
		phi := math.Atan2(rad.Dot(rv.f), rad.Dot(rv.e))
		// Into the turn's range, which may start below zero.
		for phi < rv.phi0 {
			phi += 2 * math.Pi
		}
		for phi > rv.phi0+2*math.Pi {
			phi -= 2 * math.Pi
		}
		if phi > rv.phi1 {
			return false
		}
	}
	return rv.pr.contains(z, r)
}

func (rv *revolution) crossings(o, d geom.Vector, out []float64) []float64 {
	q := o.Minus(rv.c)
	z0, zd := q.Dot(rv.w), d.Dot(rv.w)
	// The radial offset is linear in s, its square quadratic.
	r0, rd := q.Minus(rv.w.ScaledBy(z0)), d.Minus(rv.w.ScaledBy(zd))
	qa, qb, qc := rd.Dot(rd), 2*r0.Dot(rd), r0.Dot(r0)
	for _, poly := range rv.pr.polys {
		for i := range poly {
			e0, e1 := poly[i], poly[(i+1)%len(poly)]
			// The edge's line: nz z + nr r = k.
			nz, nr := -(e1[1] - e0[1]), e1[0]-e0[0]
			k := nz*e0[0] + nr*e0[1]
			if nr == 0 {
				// Square to the axis: a plane.
				if zd != 0 {
					out = append(out, (k/nz-z0)/zd)
				}
				continue
			}
			// (k - nz z)² = nr² r², each side a quadratic in s; both
			// signs of r, which is a superset of the crossings.
			l0, l1 := k-nz*z0, -nz*zd
			a := l1*l1 - nr*nr*qa
			b := 2*l0*l1 - nr*nr*qb
			cc := l0*l0 - nr*nr*qc
			// Kept only where the root is on the edge in z, give or
			// take: the edge isn't square to the axis here.
			zlo, zhi := math.Min(e0[0], e1[0]), math.Max(e0[0], e1[0])
			slack := 1e-9 * (1 + zhi - zlo)
			n := len(out)
			out = appendRoots(out, a, b, cc)
			kept := out[:n]
			for _, s := range out[n:] {
				if z := z0 + s*zd; z >= zlo-slack && z <= zhi+slack {
					kept = append(kept, s)
				}
			}
			out = kept
		}
	}
	if !rv.full {
		for _, phi := range []float64{rv.phi0, rv.phi1} {
			// The half-plane at phi: its normal turns with it.
			n := rv.f.ScaledBy(math.Cos(phi)).Minus(rv.e.ScaledBy(math.Sin(phi)))
			if dn := d.Dot(n); dn != 0 {
				out = append(out, -q.Dot(n)/dn)
			}
		}
	}
	return out
}

// appendRoots appends the real roots of a s² + b s + c.
func appendRoots(out []float64, a, b, c float64) []float64 {
	scale := math.Max(math.Abs(a), math.Max(math.Abs(b), math.Abs(c)))
	if scale == 0 {
		return out
	}
	if math.Abs(a) <= 1e-14*scale {
		if b != 0 {
			out = append(out, -c/b)
		}
		return out
	}
	disc := b*b - 4*a*c
	if disc < 0 {
		if disc > -1e-12*b*b {
			return append(out, -b/(2*a))
		}
		return out
	}
	sq := math.Sqrt(disc)
	// The stable pair: no cancellation in either root.
	qq := -0.5 * (b + math.Copysign(sq, b))
	if qq == 0 {
		return append(out, 0)
	}
	return append(out, qq/a, c/qq)
}

func (rv *revolution) bounds(lo, hi *geom.Vector) {
	// A box round the cylinder that holds the turn.
	for _, z := range []float64{rv.zLo, rv.zHi} {
		for _, s := range []float64{-1, 1} {
			for _, t := range []float64{-1, 1} {
				p := rv.c.Plus(rv.w.ScaledBy(z)).Plus(rv.e.ScaledBy(s * rv.rHi)).Plus(rv.f.ScaledBy(t * rv.rHi))
				p.MakeMaxMin(hi, lo)
			}
		}
	}
}

// union is several leaves taken together, as a group's loop sets are.
type union []node

func (u union) inside(x geom.Vector) bool {
	for _, n := range u {
		if n.inside(x) {
			return true
		}
	}
	return false
}

func (u union) crossings(o, d geom.Vector, out []float64) []float64 {
	for _, n := range u {
		out = n.crossings(o, d, out)
	}
	return out
}

func (u union) bounds(lo, hi *geom.Vector) {
	for _, n := range u {
		n.bounds(lo, hi)
	}
}

// Oracle is a group's solid as the ray-cast oracle builds it.
type Oracle struct {
	root     node
	Min, Max geom.Vector
}

// NewOracle builds the solid group hg's tree makes, from the groups'
// definitions: ErrUnsupported, wrapped with the group's name, for a
// group the oracle can't build, and nil and no error where the tree
// holds no solid.
func NewOracle(sk *sketch.Sketch, hg core.HGroup) (*Oracle, error) {
	root, err := oracleNode(sk, sk.Group(hg))
	if err != nil || root == nil {
		return nil, err
	}
	o := &Oracle{root: root}
	o.Min = geom.Vector{X: math.Inf(1), Y: math.Inf(1), Z: math.Inf(1)}
	o.Max = o.Min.ScaledBy(-1)
	root.bounds(&o.Min, &o.Max)
	return o, nil
}

// oracleNode is the solid the groups up to g make together.
func oracleNode(sk *sketch.Sketch, g *sketch.Group) (node, error) {
	if g == nil {
		return nil, nil
	}
	if isCopies(g) && !g.Suppress && (len(g.ThisShell.Surface) > 0 || !g.ThisMesh.IsEmpty()) {
		// Copies are combined with what came before their source, as
		// their source combines (GenerateShellAndMesh); a mirror of the
		// solid so far is the whole.
		copies, whole, err := copiesNode(sk, g)
		if err != nil || whole {
			return copies, err
		}
		src := sk.Group(g.OpA)
		base, err := oracleNode(sk, src.RunningMeshGroup(sk))
		if err != nil {
			return nil, err
		}
		return combine(base, copies, src.MeshCombine), nil
	}
	base, err := oracleNode(sk, g.RunningMeshGroup(sk))
	if err != nil {
		return nil, err
	}
	if g.Suppress || len(g.ThisShell.Surface) == 0 && g.ThisMesh.IsEmpty() {
		return base, nil
	}
	leaf, err := oracleLeaf(sk, g)
	if err != nil {
		return nil, err
	}
	return combine(base, leaf, g.MeshCombine), nil
}

// combine is leaf combined with the solid so far, base, as how says;
// with nothing so far, a difference or an intersection is nothing.
func combine(base, leaf node, how sketch.CombineAs) node {
	switch {
	case leaf == nil:
		return base
	case base != nil:
		return &boolean{a: base, b: leaf, how: how}
	case how == sketch.CombineDifference || how == sketch.CombineIntersection:
		return nil
	}
	return leaf
}

// oracleLeaf is the solid group g makes alone.
func oracleLeaf(sk *sketch.Sketch, g *sketch.Group) (node, error) {
	src := sk.Group(g.OpA)
	param := func(i int) float64 {
		if p := sk.Param(g.H.Param(i)); p != nil {
			return p.Val
		}
		return 0
	}
	var leaves union
	switch {
	case g.Type == sketch.GroupExtrude && src != nil:
		translate := geom.Vector{X: param(0), Y: param(1), Z: param(2)}
		tbot, ttop := translate.ScaledBy(-1), translate
		if g.Subtype == sketch.GroupOneSided || g.Subtype == sketch.GroupOneSkewed {
			tbot, ttop = geom.Vector{}, translate.ScaledBy(2)
		}
		// A draft the kernel refused leaves the extrusion upright,
		// with a problem on the group.
		drafted := g.HasDraft() && g.Problem.Msg == ""
		for i := range src.BezierLoops.Sets {
			sbls := &src.BezierLoops.Sets[i]
			n := sbls.Normal.WithMagnitude(1)
			if drafted {
				dir := ttop.Minus(tbot).WithMagnitude(1)
				dp, ok := newDraftedPrism(g, sbls, dir, tbot.Dot(dir), ttop.Dot(dir), g.ValC*math.Pi/180)
				if !ok {
					return nil, fmt.Errorf("%s: %w", g.DescriptionString(), ErrUnsupported)
				}
				leaves = append(leaves, dp)
				continue
			}
			eu := n.NormalDir(0)
			leaves = append(leaves, newPrism(newProfile(sbls, sbls.Point, eu, n.Cross(eu)), tbot, ttop))
		}
	case (g.Type == sketch.GroupLathe || g.Type == sketch.GroupRevolve) && src != nil:
		pt := sk.MustEntity(g.Predef.Origin).PointGetNum(sk)
		axis := sk.MustEntity(g.Predef.EntityB).VectorGetNum(sk).WithMagnitude(1)
		full, phi0, phi1 := true, 0.0, 0.0
		if g.Type == sketch.GroupRevolve {
			phi1 = param(3) * 4
			if g.Subtype != sketch.GroupOneSided {
				phi1 *= 0.5
				phi0 = -phi1
			}
			full = math.Abs(phi1-phi0) >= 2*math.Pi
		}
		for i := range src.BezierLoops.Sets {
			leaves = append(leaves, newRevolution(&src.BezierLoops.Sets[i], pt, axis, full, phi0, phi1))
		}
	case g.Type == sketch.GroupHelix && src != nil:
		pt := sk.MustEntity(g.Predef.Origin).PointGetNum(sk)
		axis := sk.MustEntity(g.Predef.EntityB).VectorGetNum(sk).WithMagnitude(1)
		// As the kernel reads them: the angle four times its parameter,
		// the distance twice, both about the drawing for two sides.
		th0, th1, d0, d1 := 0.0, param(3)*4, 0.0, param(7)*2
		if g.Subtype != sketch.GroupOneSided {
			th1, d1 = th1/2, d1/2
			th0, d0 = -th1, -d1
		}
		for i := range src.BezierLoops.Sets {
			hx, ok := newHelix(&src.BezierLoops.Sets[i], pt, axis, th0, th1, d0, d1)
			if !ok {
				return nil, fmt.Errorf("%s: its profile is not in a plane through the axis: %w", g.DescriptionString(), ErrUnsupported)
			}
			leaves = append(leaves, hx)
		}
	case g.Type == sketch.GroupSweep && src != nil:
		sw, err := sweepLeaves(sk, g, src)
		if err != nil {
			return nil, err
		}
		leaves = append(leaves, sw...)
	default:
		return nil, fmt.Errorf("%s: %w", g.DescriptionString(), ErrUnsupported)
	}
	if len(leaves) == 1 {
		return leaves[0], nil
	}
	return leaves, nil
}

// Inside reports whether x is inside the solid.
func (o *Oracle) Inside(x geom.Vector) bool { return o.root.inside(x) }

// Probe is where the line through from along dir passes through the
// solid, in the line's parameter (dir's length to a unit), from where
// it enters the solid's box to where it leaves.
func (o *Oracle) Probe(from, dir geom.Vector) []Interval {
	// The box, a little grown, bounds the parameters searched.
	lo, hi := math.Inf(-1), math.Inf(1)
	for k := range 3 {
		a, b := from.Element(k), dir.Element(k)
		mn, mx := o.Min.Element(k)-1, o.Max.Element(k)+1
		if b == 0 {
			if a < mn || a > mx {
				return nil
			}
			continue
		}
		s0, s1 := (mn-a)/b, (mx-a)/b
		lo, hi = math.Max(lo, math.Min(s0, s1)), math.Min(hi, math.Max(s0, s1))
	}
	if !(lo < hi) {
		return nil
	}
	cuts := o.root.crossings(from, dir, []float64{lo, hi})
	cuts = slices.DeleteFunc(cuts, func(s float64) bool { return !(s >= lo && s <= hi) })
	slices.Sort(cuts)
	var out []Interval
	for i := 1; i < len(cuts); i++ {
		a, b := cuts[i-1], cuts[i]
		if b-a <= 1e-12*(1+math.Abs(a)) {
			continue
		}
		if !o.root.inside(from.Plus(dir.ScaledBy((a + b) / 2))) {
			continue
		}
		if n := len(out); n > 0 && a-out[n-1].To <= 1e-12*(1+math.Abs(a)) {
			out[n-1].To = b
		} else {
			out = append(out, Interval{a, b})
		}
	}
	return out
}

// RayMeasure is what three grids of rays find of a solid: its volume
// and centroid by each grid's direction, and their mean.
type RayMeasure struct {
	// Volume and CenterOfMass are the three grids' mean.
	Volume       float64
	CenterOfMass geom.Vector
	// Volumes are the grids' along x, y and z.
	Volumes [3]float64
	// Inertia is the inertia tensor about the centre of mass, for a
	// density of one (mm⁵ for mm): on the diagonal the moments about
	// the axes through it along x, y and z, the integrals of y²+z²,
	// z²+x² and x²+y²; off it the products, less the integrals of xy,
	// yz and zx. The three grids' mean.
	Inertia [3][3]float64
	// Rays is how many rays each grid cast, along one side.
	Rays int
}

// PrincipalMoments are the inertia tensor's eigenvalues, the moments
// about the principal axes through the centre of mass, least first.
func (m RayMeasure) PrincipalMoments() [3]float64 {
	return symmetricEigenvalues(m.Inertia)
}

// Spread is the largest difference between two grids' volumes, as a
// fraction of the volume.
func (m RayMeasure) Spread() float64 {
	if m.Volume == 0 {
		return 0
	}
	lo := math.Min(m.Volumes[0], math.Min(m.Volumes[1], m.Volumes[2]))
	hi := math.Max(m.Volumes[0], math.Max(m.Volumes[1], m.Volumes[2]))
	return (hi - lo) / m.Volume
}

// Measure integrates the solid along three grids of rays, one along
// each axis, starting at 16 rays a side and doubling them up to most
// until the three volumes agree within rel, and with the volume of the
// grids of half as many rays. Each ray's inside length, times its
// cell's area, is the volume of the cell's column; each grid errs only
// at the solid's edges, differently, so their agreement bounds the
// error. Coarse grids whose cells all fall alike on the solid's faces
// (four spokes 4 wide on a grid of 2.5) agree with one another and
// miss alike; the coarser grids' volume catches that.
func (o *Oracle) Measure(rel float64, most int) RayMeasure {
	var m RayMeasure
	prev := math.NaN()
	for n := 16; ; n *= 2 {
		m = o.measureAt(n)
		settled := m.Volume == 0 || math.Abs(m.Volume-prev) <= rel*math.Abs(m.Volume)
		if m.Spread() <= rel && settled || n*2 > most {
			return m
		}
		prev = m.Volume
	}
}

func (o *Oracle) measureAt(n int) RayMeasure {
	m := RayMeasure{Rays: n}
	var moment [3]geom.Vector
	// The second moments are taken about the box's middle, p0, and moved
	// to the centre of mass at the end: about the origin, a solid far
	// from it would lose them to the difference of two large sums.
	p0 := o.Min.Plus(o.Max).ScaledBy(0.5)
	var second [3][3]float64
	// add sums into the upper triangle, which is all inertiaAbout reads.
	add := func(a, b int, x float64) { second[min(a, b)][max(a, b)] += x }
	for k := range 3 {
		i, j := (k+1)%3, (k+2)%3
		hi, hj := (o.Max.Element(i)-o.Min.Element(i))/float64(n), (o.Max.Element(j)-o.Min.Element(j))/float64(n)
		var dir geom.Vector
		setElement(&dir, k, 1)
		for a := range n {
			for b := range n {
				var from geom.Vector
				setElement(&from, i, o.Min.Element(i)+(float64(a)+0.5)*hi)
				setElement(&from, j, o.Min.Element(j)+(float64(b)+0.5)*hj)
				for _, iv := range o.Probe(from, dir) {
					l := (iv.To - iv.From) * hi * hj
					m.Volumes[k] += l
					var c geom.Vector
					setElement(&c, i, from.Element(i)*l)
					setElement(&c, j, from.Element(j)*l)
					setElement(&c, k, (iv.From+iv.To)/2*l)
					moment[k] = moment[k].Plus(c)
					// Along the ray, x_k runs from s0 to s1; the others stay.
					xi, xj := from.Element(i)-p0.Element(i), from.Element(j)-p0.Element(j)
					s0, s1 := iv.From-p0.Element(k), iv.To-p0.Element(k)
					area := hi * hj
					first := (s1*s1 - s0*s0) / 2 * area
					sq := (s1*s1*s1 - s0*s0*s0) / 3 * area
					// A ray stands for its cell, which spreads h²/12 about
					// it in each direction across the ray.
					add(i, i, (xi*xi+hi*hi/12)*l)
					add(j, j, (xj*xj+hj*hj/12)*l)
					add(k, k, sq)
					add(i, j, xi*xj*l)
					add(i, k, xi*first)
					add(j, k, xj*first)
				}
			}
		}
	}
	m.Volume = (m.Volumes[0] + m.Volumes[1] + m.Volumes[2]) / 3
	if m.Volume != 0 {
		sum := moment[0].Plus(moment[1]).Plus(moment[2])
		m.CenterOfMass = sum.ScaledBy(1 / (3 * m.Volume))
		m.Inertia = inertiaAbout(second, m.Volume, m.CenterOfMass.Minus(p0))
	}
	return m
}

// inertiaAbout is the inertia tensor about the centre of mass, c from
// the point the second moments were taken about, from three grids'
// second moments summed (second[a][b] for a <= b) and the volume v.
func inertiaAbout(second [3][3]float64, v float64, c geom.Vector) [3][3]float64 {
	var cov [3][3]float64
	for a := range 3 {
		for b := a; b < 3; b++ {
			x := second[a][b]/3 - v*c.Element(a)*c.Element(b)
			cov[a][b], cov[b][a] = x, x
		}
	}
	tr := cov[0][0] + cov[1][1] + cov[2][2]
	var in [3][3]float64
	for a := range 3 {
		for b := range 3 {
			in[a][b] = -cov[a][b]
			if a == b {
				in[a][b] += tr
			}
		}
	}
	return in
}

// symmetricEigenvalues are the eigenvalues of the symmetric matrix a,
// least first, by the closed form for three by three.
func symmetricEigenvalues(a [3][3]float64) [3]float64 {
	p1 := a[0][1]*a[0][1] + a[0][2]*a[0][2] + a[1][2]*a[1][2]
	q := (a[0][0] + a[1][1] + a[2][2]) / 3
	if p1 == 0 {
		e := [3]float64{a[0][0], a[1][1], a[2][2]}
		slices.Sort(e[:])
		return e
	}
	p2 := (a[0][0]-q)*(a[0][0]-q) + (a[1][1]-q)*(a[1][1]-q) + (a[2][2]-q)*(a[2][2]-q) + 2*p1
	p := math.Sqrt(p2 / 6)
	var b [3][3]float64
	for i := range 3 {
		for j := range 3 {
			b[i][j] = a[i][j] / p
			if i == j {
				b[i][j] -= q / p
			}
		}
	}
	det := b[0][0]*(b[1][1]*b[2][2]-b[1][2]*b[2][1]) - b[0][1]*(b[1][0]*b[2][2]-b[1][2]*b[2][0]) +
		b[0][2]*(b[1][0]*b[2][1]-b[1][1]*b[2][0])
	r := math.Max(-1, math.Min(1, det/2))
	phi := math.Acos(r) / 3
	hi := q + 2*p*math.Cos(phi)
	lo := q + 2*p*math.Cos(phi+2*math.Pi/3)
	return [3]float64{lo, 3*q - hi - lo, hi}
}

// MeshProbe is where the line through from along dir passes through a
// closed mesh: its hits with the triangles, sorted, taken in pairs.
func MeshProbe(mesh *srf.SMesh, from, dir geom.Vector) []Interval {
	var hits []float64
	for i := range mesh.Triangles {
		tr := &mesh.Triangles[i]
		if s, ok := rayTriangle(from, dir, tr.A, tr.B, tr.C); ok {
			hits = append(hits, s)
		}
	}
	slices.Sort(hits)
	// A ray through an edge or a vertex hits both triangles there.
	hits = slices.CompactFunc(hits, func(a, b float64) bool { return math.Abs(b-a) <= 1e-9*(1+math.Abs(a)) })
	var out []Interval
	for i := 0; i+1 < len(hits); i += 2 {
		out = append(out, Interval{hits[i], hits[i+1]})
	}
	return out
}

// rayTriangle is where o + s d meets triangle abc (Möller-Trumbore).
func rayTriangle(o, d, a, b, c geom.Vector) (float64, bool) {
	e1, e2 := b.Minus(a), c.Minus(a)
	p := d.Cross(e2)
	det := e1.Dot(p)
	if math.Abs(det) < 1e-300 {
		return 0, false
	}
	t := o.Minus(a)
	u := t.Dot(p) / det
	if u < 0 || u > 1 {
		return 0, false
	}
	q := t.Cross(e1)
	v := d.Dot(q) / det
	if v < 0 || u+v > 1 {
		return 0, false
	}
	return e2.Dot(q) / det, true
}

// RayDiff is a ray along which the oracle and a mesh disagree.
type RayDiff struct {
	From, Dir    geom.Vector
	Oracle, Mesh []Interval
	// By is the length of the ray inside one of them and not the other.
	By float64
}

// Diff casts a grid of n by n rays along each axis through the oracle
// and the mesh, and lists those along which the two disagree: where
// one has the ray inside and the other not, over a stretch whose
// middle lies further than tol from the mesh's triangles. A mesh's
// chords miss a curved face by up to the chord tolerance, and a ray
// that grazes the face runs a long way through that sliver; the
// sliver's middle is never further from the mesh than its width, so
// those stretches don't count, and a face missing or misplaced does.
func (o *Oracle) Diff(mesh *srf.SMesh, n int, tol float64) []RayDiff {
	var out []RayDiff
	for k := range 3 {
		i, j := (k+1)%3, (k+2)%3
		hi, hj := (o.Max.Element(i)-o.Min.Element(i))/float64(n), (o.Max.Element(j)-o.Min.Element(j))/float64(n)
		var dir geom.Vector
		setElement(&dir, k, 1)
		for a := range n {
			for b := range n {
				var from geom.Vector
				setElement(&from, i, o.Min.Element(i)+(float64(a)+0.37)*hi)
				setElement(&from, j, o.Min.Element(j)+(float64(b)+0.61)*hj)
				or, me := o.Probe(from, dir), MeshProbe(mesh, from, dir)
				by := 0.0
				for _, iv := range symmetricDifference(or, me) {
					mid := from.Plus(dir.ScaledBy((iv.From + iv.To) / 2))
					if distanceToMesh(mesh, mid, tol) > tol {
						by += iv.To - iv.From
					}
				}
				if by > 0 {
					out = append(out, RayDiff{From: from, Dir: dir, Oracle: or, Mesh: me, By: by})
				}
			}
		}
	}
	return out
}

// symmetricDifference is the stretches inside one list of intervals
// and not the other.
func symmetricDifference(a, b []Interval) []Interval {
	var cuts []float64
	for _, l := range [][]Interval{a, b} {
		for _, iv := range l {
			cuts = append(cuts, iv.From, iv.To)
		}
	}
	slices.Sort(cuts)
	in := func(l []Interval, s float64) bool {
		for _, iv := range l {
			if s > iv.From && s < iv.To {
				return true
			}
		}
		return false
	}
	var out []Interval
	for i := 1; i < len(cuts); i++ {
		lo, hi := cuts[i-1], cuts[i]
		if hi <= lo {
			continue
		}
		if mid := (lo + hi) / 2; in(a, mid) != in(b, mid) {
			if n := len(out); n > 0 && out[n-1].To == lo {
				out[n-1].To = hi
			} else {
				out = append(out, Interval{lo, hi})
			}
		}
	}
	return out
}

// distanceToMesh is how far p is from the mesh's nearest triangle, or
// a number past limit once no triangle can be nearer than limit.
func distanceToMesh(mesh *srf.SMesh, p geom.Vector, limit float64) float64 {
	best := math.Inf(1)
	for i := range mesh.Triangles {
		tr := &mesh.Triangles[i]
		// A triangle whose box is further than the best so far can't
		// be nearer.
		lo, hi := tr.A, tr.A
		tr.B.MakeMaxMin(&hi, &lo)
		tr.C.MakeMaxMin(&hi, &lo)
		out := geom.Vector{
			X: math.Max(0, math.Max(lo.X-p.X, p.X-hi.X)),
			Y: math.Max(0, math.Max(lo.Y-p.Y, p.Y-hi.Y)),
			Z: math.Max(0, math.Max(lo.Z-p.Z, p.Z-hi.Z)),
		}
		if out.Magnitude() >= best {
			continue
		}
		best = math.Min(best, pointTriangle(p, tr.A, tr.B, tr.C))
		if best <= limit {
			return best
		}
	}
	return best
}

// pointTriangle is the distance from p to triangle abc.
func pointTriangle(p, a, b, c geom.Vector) float64 {
	n := b.Minus(a).Cross(c.Minus(a))
	if nn := n.Magnitude(); nn > 0 {
		n = n.ScaledBy(1 / nn)
		q := p.Minus(n.ScaledBy(p.Minus(a).Dot(n)))
		// Inside when q is on the inner side of each edge.
		if b.Minus(a).Cross(q.Minus(a)).Dot(n) >= 0 && c.Minus(b).Cross(q.Minus(b)).Dot(n) >= 0 &&
			a.Minus(c).Cross(q.Minus(c)).Dot(n) >= 0 {
			return math.Abs(p.Minus(a).Dot(n))
		}
	}
	seg := func(u, v geom.Vector) float64 {
		d := v.Minus(u)
		t := 0.0
		if l := d.Dot(d); l > 0 {
			t = math.Max(0, math.Min(1, p.Minus(u).Dot(d)/l))
		}
		return p.Minus(u.Plus(d.ScaledBy(t))).Magnitude()
	}
	return math.Min(seg(a, b), math.Min(seg(b, c), seg(c, a)))
}

// setElement sets v's component i: x, y or z.
func setElement(v *geom.Vector, i int, x float64) {
	switch i {
	case 0:
		v.X = x
	case 1:
		v.Y = x
	default:
		v.Z = x
	}
}
