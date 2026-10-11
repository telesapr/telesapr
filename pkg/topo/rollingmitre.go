package topo

import (
	"math"
	"slices"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// A mitred rolling-ball fillet: a chain that turns sharply at a vertex
// where the faces' third edge, a crease of one side's face, stays sharp,
// as the rim of an extruded profile with a corner does. The chain then
// runs from the crease round to it again, open at both ends at one
// vertex, and near the crease the ball rolling one way comes within its
// radius of the face beyond the crease: there is no section there. The
// fillet ends where the surfaces the ball sweeps either way meet, a
// mitre, which both ends share as their last section.
//
// Near each end the ball runs on along its own side's face carried on
// past the crease, never on the face across it (rbEnd.ban). Each end's
// surface, carried on, lies inside the other's in a band between the
// two curves where they meet, one in each plane that halves the turn;
// the balls in the band would stand out through the other side's face,
// and the fillet is cut at the band's edge in the plane through the
// crease. Past that edge, towards the crease, a point is outside the
// other end's swept surface: an end's height is a point's distance from
// the other end's centres less the radius. The end at the start is
// fitted as a curved end is (curvedEndSection), to where that height is
// zero, and the end at the finish takes the same curve.
//
// Where the rim turns at two or more creases, each run between two of
// them is a chain of its own, and an end meets the next chain's end
// (rollingchains.go): an end's mate is the end it meets, its own chain's
// other end or another chain's, and "the other end" above is the mate.

// mitreZone is how far along the chain from an end, in radii, the ball's
// centres are kept for the other end's height.
const mitreZone = 8

// mitreCrossing is the sine of the angle under which the two swept
// surfaces are taken to meet tangentially.
const mitreCrossing = 0.1

// mitreSamples is how many of the ball's centres are kept per radius.
const mitreSamples = 32

// mitred reports whether the chain's ends meet in mitres, each other or
// other chains' ends.
func (rb *rollingBall) mitred() bool {
	return !rb.closed && rb.ends[0] != nil && rb.ends[0].mitre && rb.ends[1].mitre
}

// rbNear is the end of a mitred chain a section is nearer (end), and
// whether past it; end is -1 on any other chain.
type rbNear struct {
	end  int
	past bool
}

// nearEnd is the end of a mitred chain that arc length s is nearer.
func (rb *rollingBall) nearEnd(s float64) rbNear {
	switch {
	case !rb.mitred():
		return rbNear{end: -1}
	case s < rb.length/2:
		return rbNear{end: 0, past: s < 0}
	}
	return rbNear{end: 1, past: s > rb.length}
}

// markMitre makes the ends of an open chain mitre ends when both are at
// one vertex and the face each found to end at is one of the chain's
// sides: the crease between the two is the third edge there. Each end
// then keeps the ball off that face.
func (rb *rollingBall) markMitre() {
	if rb.closed || rb.verts[0] != rb.verts[len(rb.verts)-1] {
		return
	}
	var ban [2][2]int
	for k, end := range rb.ends {
		ban[k] = [2]int{-1, -1}
		for j := range 2 {
			if slices.Contains(rb.side[j], end.f) {
				ban[k][j] = end.f
			}
		}
		if ban[k] == [2]int{-1, -1} {
			return
		}
	}
	for k, end := range rb.ends {
		end.mitre, end.curved, end.ban = true, true, ban[k]
		end.mate, end.mateK = rb, 1-k
	}
}

// linkMitres makes mitre ends of the open chains' ends that meet
// another chain's end at a vertex where the third edge, between the two
// chains' faces on one side, is a crease: the two chains share their
// other side's face, and each one's end face is the other's creased
// face. It reports whether every end of every chain is linked so, and
// to a chain of the same radius.
func linkMitres(rbs []*rollingBall) bool {
	for _, rb := range rbs {
		for k, end := range rb.ends {
			if rb.closed || end == nil {
				return false
			}
			for _, o := range rbs {
				if end.mitre {
					break
				}
				if o == rb || o.closed || o.r != rb.r || o.flat != rb.flat {
					continue
				}
				for ok, oe := range o.ends {
					if oe.v != end.v || oe.mitre {
						continue
					}
					ban, ok0 := mitreBan(end, oe, o)
					oban, ok1 := mitreBan(oe, end, rb)
					if !ok0 || !ok1 {
						continue
					}
					end.mitre, end.curved, end.ban, end.mate, end.mateK = true, true, ban, o, ok
					oe.mitre, oe.curved, oe.ban, oe.mate, oe.mateK = true, true, oban, rb, k
					break
				}
			}
			if !end.mitre {
				return false
			}
		}
	}
	return true
}

// mitreBan is end's ban where chain o's end oe meets it in a mitre: on
// the side whose edge with end's face is o's own, the face both share,
// none; on the other, the crease, end's face. It reports false where
// the two don't meet so.
func mitreBan(end, oe *rbEnd, o *rollingBall) ([2]int, bool) {
	ban := [2]int{-1, -1}
	if !slices.Contains(oe.face[:], end.f) {
		return ban, false
	}
	shared := 0
	for j := range 2 {
		if slices.ContainsFunc(o.pieces, func(pc ChainPiece) bool { return pc.Edge == end.a[j] }) {
			shared++
		} else {
			ban[j] = end.f
		}
	}
	return ban, shared == 1
}

// mitrePlane sets each mitre end's normal and plane: the plane that
// halves the turn at the crease from it to its mate, through the vertex,
// its normal pointing on past that end.
func (rb *rollingBall) mitrePlane() error {
	if !rb.mitred() {
		return nil
	}
	for k, end := range rb.ends {
		m := rb.beyond(k).Minus(end.mate.beyond(end.mateK))
		at := rb.t.Vertices[end.v].P
		if m.Magnitude() < 1e-6 {
			return filletErr(at, "the edge turns back on itself at the crease")
		}
		end.n = m.WithMagnitude(1)
		end.d = end.n.Dot(at)
	}
	return nil
}

// beyond is the way on past end k along the chain.
func (rb *rollingBall) beyond(k int) geom.Vector {
	if k == 0 {
		return rb.tan[0].ScaledBy(-1)
	}
	return rb.tan[len(rb.tan)-1]
}

// rbCentres are the ball's centres at arc lengths s near an end.
type rbCentres struct {
	s []float64
	c []geom.Vector
}

// centresNear are the ball's centres within mitreZone radii of end k,
// either side of it, worked out once.
func (rb *rollingBall) centresNear(k int) *rbCentres {
	if rb.mitreC[k] != nil {
		return rb.mitreC[k]
	}
	// In from the end, and on past it as far: the other end's mitre
	// lies where this end's surface, carried on, meets it.
	zone := math.Min(rb.length/2, mitreZone*rb.r)
	n := int(math.Ceil(2 * zone / rb.r * mitreSamples))
	cs := &rbCentres{}
	for i := 0; i <= n; i++ {
		s := -zone + 2*zone*float64(i)/float64(n)
		if k == 1 {
			s = rb.length - zone + 2*zone*float64(i)/float64(n)
		}
		sec, err := rb.section(s)
		if err != nil {
			continue
		}
		cs.s = append(cs.s, s)
		cs.c = append(cs.c, sec.c)
	}
	rb.mitreC[k] = cs
	return cs
}

// nearestCentre is the point of the ball's centres' curve near end k
// nearest p: the nearest sample, then the nearest point of the cubic
// through it and its neighbours.
func (rb *rollingBall) nearestCentre(k int, p geom.Vector) (geom.Vector, bool) {
	cs := rb.centresNear(k)
	if len(cs.c) < 4 {
		return geom.Vector{}, false
	}
	best, bd := 0, math.Inf(1)
	for i, c := range cs.c {
		if d := c.Minus(p).MagSquared(); d < bd {
			best, bd = i, d
		}
	}
	i := min(max(best, 1), len(cs.c)-3)
	// Catmull-Rom between samples i and i+1, and i-1 and i: the nearest
	// point on either span.
	at := func(j int, t float64) geom.Vector {
		p0, p1, p2, p3 := cs.c[j-1], cs.c[j], cs.c[j+1], cs.c[min(j+2, len(cs.c)-1)]
		t2, t3 := t*t, t*t*t
		return p0.ScaledBy(-0.5*t3 + t2 - 0.5*t).
			Plus(p1.ScaledBy(1.5*t3 - 2.5*t2 + 1)).
			Plus(p2.ScaledBy(-1.5*t3 + 2*t2 + 0.5*t)).
			Plus(p3.ScaledBy(0.5*t3 - 0.5*t2))
	}
	q, qd := cs.c[best], bd
	for _, j := range []int{i - 1, i} {
		if j < 1 {
			continue
		}
		lo, hi := 0.0, 1.0
		g := (math.Sqrt(5) - 1) / 2
		for range 60 {
			a, b := hi-g*(hi-lo), lo+g*(hi-lo)
			if at(j, a).Minus(p).MagSquared() < at(j, b).Minus(p).MagSquared() {
				hi = b
			} else {
				lo = a
			}
		}
		if c := at(j, (lo+hi)/2); c.Minus(p).MagSquared() < qd {
			q, qd = c, c.Minus(p).MagSquared()
		}
	}
	return q, true
}

// mateCentre is the point of the ball's centres' curve near end k's
// mate, the end it meets in the mitre, nearest p.
func (rb *rollingBall) mateCentre(k int, p geom.Vector) (geom.Vector, bool) {
	end := rb.ends[k]
	return end.mate.nearestCentre(end.mateK, p)
}

// mitreHeight is how far p lies outside the surface the ball sweeps near
// end k's mate: its distance from the centres less the radius.
func (rb *rollingBall) mitreHeight(k int, p geom.Vector) float64 {
	c, ok := rb.mateCentre(k, p)
	if !ok {
		return math.NaN()
	}
	return p.Minus(c).Magnitude() - rb.r
}

// mitreNormal is the way out of the mate's swept surface at q,
// the normal of end k's end there: from the nearest of its centres.
func (rb *rollingBall) mitreNormal(k int, q geom.Vector) geom.Vector {
	c, ok := rb.mateCentre(k, q)
	if !ok || q.Minus(c).Magnitude() == 0 {
		return geom.Vector{}
	}
	return q.Minus(c).WithMagnitude(1)
}

// mitreFoot is where the mitre meets side i's face near end k. On the
// creased side the other end's balls pass through this side's face near
// the crease, so the height changes sign where the point the ball
// touches the face crosses the crease: a root. On the face both sides
// share, both swept surfaces touch it, so they meet it tangentially,
// where the curves they touch it along cross, and the height only comes
// up to zero there: the arc length whose point comes nearest the other
// end's centres, at the radius. Where that falls short the two curves
// don't cross on the face, and the mitre would leave a gap: refused.
func (rb *rollingBall) mitreFoot(k, i int) (float64, rbSection, error) {
	at := rb.t.Vertices[rb.ends[k].v].P
	zone := math.Min(rb.length/2, mitreZone*rb.r)
	// gap is how far side i's point at s lies outside the other end's
	// swept surface.
	gap := func(s float64) (float64, rbSection, bool) {
		sec, err := rb.section(s)
		if err != nil {
			return math.NaN(), sec, false
		}
		c, ok := rb.mateCentre(k, sec.foot[i].p)
		if !ok {
			return math.NaN(), sec, false
		}
		return sec.foot[i].p.Minus(c).Magnitude() - rb.r, sec, true
	}
	// From past the crease in along the chain.
	from, step := -rb.r, rb.r/8
	arc := func(x float64) float64 {
		if k == 1 {
			return rb.length - x
		}
		return x
	}
	if rb.ends[k].ban[i] >= 0 {
		prev, prevG := math.NaN(), math.NaN()
		for x := from; x <= zone; x += step {
			g, _, ok := gap(arc(x))
			if !ok {
				continue
			}
			if !math.IsNaN(prevG) && prevG >= 0 && g < 0 {
				lo, hi := prev, x
				for range 80 {
					m := (lo + hi) / 2
					if gm, _, _ := gap(arc(m)); gm >= 0 {
						lo = m
					} else {
						hi = m
					}
				}
				s := arc((lo + hi) / 2)
				_, sec, _ := gap(s)
				return s, sec, nil
			}
			prev, prevG = x, g
		}
		return 0, rbSection{}, filletErr(at, "the fillet's mitre doesn't reach the crease here")
	}
	best, bestG := math.NaN(), math.Inf(1)
	for x := from; x <= zone; x += step {
		if g, _, ok := gap(arc(x)); ok && g < bestG {
			best, bestG = x, g
		}
	}
	if math.IsNaN(best) {
		return 0, rbSection{}, filletErr(at, "the fillet's mitre at the crease can't be found")
	}
	lo, hi := best-step, best+step
	gr := (math.Sqrt(5) - 1) / 2
	for range 80 {
		a, b := hi-gr*(hi-lo), lo+gr*(hi-lo)
		ga, _, oka := gap(arc(a))
		gb, _, okb := gap(arc(b))
		if oka && (!okb || ga < gb) {
			hi = b
		} else {
			lo = a
		}
	}
	s := arc((lo + hi) / 2)
	g, sec, ok := gap(s)
	if !ok || math.Abs(g) > rb.tol {
		return 0, sec, filletErr(at, "the fillet's two sides don't meet in a mitre on the face here (%.3g mm apart)", g)
	}
	return s, sec, nil
}

// mitreRoot is the arc length near end k where the point g measures
// crosses the other end's swept surface at the mitre, and its section.
// Along the chain from past the end the height falls, into the band
// where the surfaces lie inside each other, and rises again past the
// band's far edge, which is no mitre; towards the face both sides share
// the band narrows to nothing, under any fixed step. So the band's
// deepest point is found first, and the root between it and the end.
func (rb *rollingBall) mitreRoot(k int, g func(rbSection) float64) (float64, rbSection, error) {
	at := rb.t.Vertices[rb.ends[k].v].P
	zone := math.Min(rb.length/2, mitreZone*rb.r)
	arc := func(x float64) float64 {
		if k == 1 {
			return rb.length - x
		}
		return x
	}
	h := func(x float64) (float64, rbSection) {
		sec, err := rb.section(arc(x))
		if err != nil {
			return math.NaN(), sec
		}
		return g(sec), sec
	}
	lo, hi := -rb.r, zone
	if gl, _ := h(lo); !(gl > 0) {
		return 0, rbSection{}, filletErr(at, "the fillet's mitre at the crease can't be found")
	}
	// Down to where the band is narrowest, near the shared face.
	a, b := lo, hi
	gr := (math.Sqrt(5) - 1) / 2
	for b-a > mitreBandEps*rb.r {
		x1, x2 := b-gr*(b-a), a+gr*(b-a)
		g1, _ := h(x1)
		g2, _ := h(x2)
		if g1 < g2 || math.IsNaN(g2) {
			b = x2
		} else {
			a = x1
		}
	}
	deep := (a + b) / 2
	if gd, _ := h(deep); !(gd <= 0) {
		return 0, rbSection{}, filletErr(at, "the fillet's two sides don't meet in a mitre here")
	}
	in, out := deep, lo
	for in-out > 1e-12*math.Max(1, rb.length) {
		m := (in + out) / 2
		if gm, _ := h(m); gm > 0 {
			out = m
		} else {
			in = m
		}
	}
	x := (in + out) / 2
	_, sec := h(x)
	return arc(x), sec, nil
}

// mitreCut splits the crease, the edge between the creased side's faces
// at the chain's vertex, where the mitre arc meets it, and gives that
// point and the vertex: the piece of the crease between them goes from
// both faces. The face the sides share takes the fillet's edge round to
// the mitre from both ways, and the fillet's first and last patches
// share the mitre, so nothing else is cut.
func (rb *rollingBall) mitreCut(sh *srf.SShell, k int, arc srf.BSplineCurve) (geom.Vector, geom.Vector, error) {
	end := rb.ends[k]
	v := rb.t.Vertices[end.v].P
	q := arc.Ctrl[0]
	if end.ban[1] >= 0 {
		q = arc.Ctrl[len(arc.Ctrl)-1]
	}
	if err := splitNearest(sh, q, v); err != nil {
		return q, v, filletErr(q, "the fillet's mitre doesn't meet the crease here: make it smaller")
	}
	return q, v, nil
}

// mitreStep is the share of the way in at which the sections either side
// of a mitre end are taken for the derivative across it.
const mitreStep = 1e-3

// mitreBandEps is, in radii, how narrow the band where the swept
// surfaces lie inside each other is searched down to for its deepest
// point.
const mitreBandEps = 1e-7

// mitreGrid is how many steps along a section the mitre's arc lengths
// are found at, for the sections near a mitre end.
const mitreGrid = 16

// mitreRoots are the arc lengths near end k at which the ball's arcs'
// points at parameters j/mitreGrid lie on the mitre, worked out once:
// the ends the points the mitre touches the faces at (endSection's), the
// points between mitreRoot's.
func (rb *rollingBall) mitreRoots(k int) ([]float64, error) {
	if rb.mitreS[k] != nil {
		return rb.mitreS[k], nil
	}
	end := rb.ends[k]
	ss := make([]float64, mitreGrid+1)
	ss[0], ss[mitreGrid] = rb.endS[k][0], rb.endS[k][1]
	for j := 1; j < mitreGrid; j++ {
		tj := float64(j) / mitreGrid
		s, _, err := rb.mitreRoot(k, func(x rbSection) float64 { return rb.endHeight(end, x.arc.PointAt(tj)) })
		if err != nil {
			return nil, err
		}
		ss[j] = s
	}
	rb.mitreS[k] = ss
	return ss, nil
}

// mitreSection is the section lam of the way in from mitre end k: the
// curve on the ball's swept surface whose point at each parameter t of
// the ball's arcs lies at the arc length lam of the way from the mitre's
// there to the first of the ball's own sections. Every point is the
// swept surface's, and as lam grows each moves on along the chain, so
// the sections never cross, as planes through a mitre that bends would.
// Its arc length is the points' mean, and the innermost comes back too.
func (rb *rollingBall) mitreSection(k int, lam float64) (rbSection, float64, error) {
	return rb.mitreSectionWithin(k, lam, 1)
}

// mitreDerivative is mitreSection for the derivative across end k: the
// sections a step either side of the mitre are only differenced, and the
// surface they shape is checked against the swept surface; they are
// fitted within mitreDerivativeSlack times the sections' tolerance.
func (rb *rollingBall) mitreDerivative(k int, lam float64) (rbSection, float64, error) {
	return rb.mitreSectionWithin(k, lam, mitreDerivativeSlack)
}

// mitreDerivativeSlack is how much looser than a section's tolerance the
// sections either side of a mitre are fitted.
const mitreDerivativeSlack = 4

// mitreSectionWithin is mitreSection fitted within slack times the
// sections' tolerance.
func (rb *rollingBall) mitreSectionWithin(k int, lam, slack float64) (rbSection, float64, error) {
	ms, err := rb.mitreRoots(k)
	if err != nil {
		return rbSection{}, 0, err
	}
	ts := make([]float64, 0, mitreGrid-1)
	xs := make([]geom.Vector, 0, mitreGrid-1)
	var first, last rbSection
	sum, inner := 0.0, math.Inf(-1)
	if k == 1 {
		inner = math.Inf(1)
	}
	for j := range mitreGrid + 1 {
		s := (1-lam)*ms[j] + lam*rb.target[k]
		sec, err := rb.section(s)
		if err != nil {
			return rbSection{}, 0, err
		}
		sum += s
		if k == 0 {
			inner = math.Max(inner, s)
		} else {
			inner = math.Min(inner, s)
		}
		switch j {
		case 0:
			first = sec
		case mitreGrid:
			last = sec
		default:
			tj := float64(j) / mitreGrid
			ts = append(ts, tj)
			xs = append(xs, sec.arc.PointAt(tj))
		}
	}
	mean := sum / (mitreGrid + 1)
	ball, err := rb.section(mean)
	if err != nil {
		return rbSection{}, 0, err
	}
	c := rb.splitSection(raised(ball.arc))
	n := len(c.Ctrl) - 1
	c.Ctrl[0], c.Ctrl[n] = first.foot[0].p, last.foot[1].p
	// The ends turn anywhere in the faces' tangent planes: the swept
	// surface meets the faces tangentially there.
	var ways [2][]geom.Vector
	for i, f := range []rbFoot{first.foot[0], last.foot[1]} {
		ways[i] = []geom.Vector{f.n.NormalDir(0), f.n.NormalDir(1)}
	}
	fit, us, ok := fitThrough(c, c.Weight, ways, ts, xs)
	at := rb.t.Vertices[rb.ends[k].v].P
	if !ok {
		return rbSection{}, 0, filletErr(at, "the fillet's sections near its mitre can't be fitted here")
	}
	tol := slack * math.Max(rb.t.shell.Tol.ChordTolMm()/2, 1e-6)
	for j, p := range xs {
		if d := fit.PointAt(us[j]).Minus(p).Magnitude(); d > tol {
			return rbSection{}, 0, filletErr(p, "the fillet's sections near its mitre can't be fitted within %.3g mm here (%.3g): make it smaller", tol, d)
		}
	}
	sec := ball
	sec.s, sec.arc = mean, fit
	sec.foot = [2]rbFoot{first.foot[0], last.foot[1]}
	return sec, inner, nil
}

// mateShared is the mitre end k's mate has fitted, or nil.
func (rb *rollingBall) mateShared(k int) *rbSection {
	end := rb.ends[k]
	return end.mate.ends[end.mateK].shared
}

// fromMate is end k's last section sec with the mitre its mate fitted in
// its place: run the other way where the face both share is the other
// side of this chain than of the mate's.
func (rb *rollingBall) fromMate(k int, sec rbSection) rbSection {
	end := rb.ends[k]
	m := rb.mateShared(k)
	if end.mate == rb {
		sec.arc, sec.foot = m.arc, m.foot
		return sec
	}
	arc := m.arc
	if (end.ban[0] < 0) != (end.mate.ends[end.mateK].ban[0] < 0) {
		arc = reversedCurve(arc)
	}
	sec.arc = arc
	sec.foot[0].p, sec.foot[1].p = arc.Ctrl[0], arc.Ctrl[len(arc.Ctrl)-1]
	return sec
}

// reversedCurve is c run the other way.
func reversedCurve(c srf.BSplineCurve) srf.BSplineCurve {
	r := srf.BSplineCurve{Deg: c.Deg}
	r.Ctrl = slices.Clone(c.Ctrl)
	slices.Reverse(r.Ctrl)
	if c.Weight != nil {
		r.Weight = slices.Clone(c.Weight)
		slices.Reverse(r.Weight)
	}
	n := len(c.Knots)
	a, b := c.Knots[0], c.Knots[n-1]
	r.Knots = make([]float64, n)
	for i, x := range c.Knots {
		r.Knots[n-1-i] = a + b - x
	}
	return r
}
