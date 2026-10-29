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

// mitreAt reports whether end k of the open chain meets a mitre.
func (rb *rollingBall) mitreAt(k int) bool {
	return !rb.closed && rb.ends[k] != nil && rb.ends[k].mitre
}

// anyMitre reports whether either end of the chain meets a mitre: a
// chain whose other end ends at a face, flat or curved, as an edge
// picked with one it meets at a corner (linkMitresAt, partial) does.
func (rb *rollingBall) anyMitre() bool {
	return rb.mitreAt(0) || rb.mitreAt(1)
}

// rbNear is the end of a mitred chain a section is nearer (end), and
// whether past it; end is -1 on any other chain, and away from an end
// that isn't a mitre.
type rbNear struct {
	end  int
	past bool
}

// nearEnd is the end of a mitred chain that arc length s is nearer.
func (rb *rollingBall) nearEnd(s float64) rbNear {
	switch {
	case !rb.anyMitre():
		return rbNear{end: -1}
	case s < rb.length/2:
		if !rb.mitreAt(0) {
			return rbNear{end: -1}
		}
		return rbNear{end: 0, past: s < 0}
	}
	if !rb.mitreAt(1) {
		return rbNear{end: -1}
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

// linkMitresAt makes mitre ends of the open chains' ends that meet
// another chain's end at a vertex where the third edge, between the two
// chains' faces on one side, is a crease: the two chains share their
// other side's face, and each one's end face is the other's creased
// face. It reports whether every end of every chain is linked so, and
// for chamfers to one of the same distance (fillets of two radii are
// mitred too, rollingkink.go). With partial, an end at a vertex no
// other chain's end shares may stay an end at a face, flat or curved,
// and it reports whether every end that shares its vertex with another
// chain's is linked, and at least one is.
func linkMitresAt(rbs []*rollingBall, partial bool) bool {
	linked := false
	for _, rb := range rbs {
		for k, end := range rb.ends {
			if rb.closed || end == nil {
				return false
			}
			for _, o := range rbs {
				if end.mitre {
					break
				}
				if o == rb || o.closed || o.flat != rb.flat {
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
			if end.mitre {
				linked = true
				continue
			}
			if !partial || sharesVertex(rbs, rb, end.v) {
				return false
			}
		}
	}
	return !partial || linked
}

// sharesVertex reports whether a chain of rbs other than rb has an end
// or a vertex at v.
func sharesVertex(rbs []*rollingBall, rb *rollingBall, v int) bool {
	for _, o := range rbs {
		if o != rb && slices.Contains(o.verts, v) {
			return true
		}
	}
	return false
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
	if !rb.anyMitre() {
		return nil
	}
	for k, end := range rb.ends {
		if !end.mitre {
			continue
		}
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

// rbCentres are the ball's centres at arc lengths s near an end; for a
// chamfer the middles of its lines, and u the lines' normals in their
// sections, pointing to the edge.
type rbCentres struct {
	s []float64
	c []geom.Vector
	u []geom.Vector
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
		if rb.flat {
			x, d := rb.along(s)
			u := d.Cross(sec.foot[1].p.Minus(sec.foot[0].p))
			if u.Dot(x.Minus(sec.c)) < 0 {
				u = u.ScaledBy(-1)
			}
			cs.u = append(cs.u, u.WithMagnitude(1))
		}
	}
	rb.mitreC[k] = cs
	return cs
}

// nearestCentre is the point of the ball's centres' curve near end k
// nearest p: the nearest sample, then the nearest point of the cubic
// through it and its neighbours.
func (rb *rollingBall) nearestCentre(k int, p geom.Vector) (geom.Vector, bool) {
	cs := rb.centresNear(k)
	j, t, ok := cs.nearest(p)
	if !ok {
		return geom.Vector{}, false
	}
	return catmullRom(cs.c, j, t), true
}

// nearest is the span j and parameter t on it of the centres' curve's
// point nearest p, the curve the Catmull-Rom spline through them: the
// nearest sample, then the nearest point of the spans either side.
func (cs *rbCentres) nearest(p geom.Vector) (int, float64, bool) {
	if len(cs.c) < 4 {
		return 0, 0, false
	}
	best, bd := 0, math.Inf(1)
	for i, c := range cs.c {
		if d := c.Minus(p).MagSquared(); d < bd {
			best, bd = i, d
		}
	}
	i := min(max(best, 1), len(cs.c)-3)
	bj, bt := best, 0.0
	for _, j := range []int{i - 1, i} {
		if j < 1 {
			continue
		}
		lo, hi := 0.0, 1.0
		g := (math.Sqrt(5) - 1) / 2
		for range 60 {
			a, b := hi-g*(hi-lo), lo+g*(hi-lo)
			if catmullRom(cs.c, j, a).Minus(p).MagSquared() < catmullRom(cs.c, j, b).Minus(p).MagSquared() {
				hi = b
			} else {
				lo = a
			}
		}
		t := (lo + hi) / 2
		if d := catmullRom(cs.c, j, t).Minus(p).MagSquared(); d < bd {
			bj, bt, bd = j, t, d
		}
	}
	return bj, bt, true
}

// catmullRom is the point t of the way along the Catmull-Rom span
// between ps[j] and ps[j+1]; at t 0, ps[j].
func catmullRom(ps []geom.Vector, j int, t float64) geom.Vector {
	if t == 0 {
		return ps[j]
	}
	p0, p1, p2, p3 := ps[j-1], ps[j], ps[j+1], ps[min(j+2, len(ps)-1)]
	t2, t3 := t*t, t*t*t
	return p0.ScaledBy(-0.5*t3 + t2 - 0.5*t).
		Plus(p1.ScaledBy(1.5*t3 - 2.5*t2 + 1)).
		Plus(p2.ScaledBy(-1.5*t3 + 2*t2 + 0.5*t)).
		Plus(p3.ScaledBy(0.5*t3 - 0.5*t2))
}

// mitreHeight is how far p lies outside the surface the ball sweeps near
// end k's mate: its distance from the centres less the mate's radius.
// For a chamfer it is p's height over the mate's line in the section
// through p, towards the edge.
func (rb *rollingBall) mitreHeight(k int, p geom.Vector) float64 {
	h, _, ok := rb.mateSurface(k, p)
	if !ok {
		return math.NaN()
	}
	return h
}

// mitreNormal is the way out of the mate's swept surface at q,
// the normal of end k's end there: from the nearest of its centres.
func (rb *rollingBall) mitreNormal(k int, q geom.Vector) geom.Vector {
	_, n, ok := rb.mateSurface(k, q)
	if !ok {
		return geom.Vector{}
	}
	return n
}

// mateSurface is p's height over the surface end k's mate sweeps
// (mitreHeight) and the way out of that surface nearest p.
func (rb *rollingBall) mateSurface(k int, p geom.Vector) (float64, geom.Vector, bool) {
	end := rb.ends[k]
	mate := end.mate
	cs := mate.centresNear(end.mateK)
	j, t, ok := cs.nearest(p)
	if !ok {
		return 0, geom.Vector{}, false
	}
	c := catmullRom(cs.c, j, t)
	if mate.flat {
		u := catmullRom(cs.u, j, t)
		if u.Magnitude() == 0 {
			return 0, geom.Vector{}, false
		}
		u = u.WithMagnitude(1)
		return p.Minus(c).Dot(u), u, true
	}
	d := p.Minus(c)
	if d.Magnitude() == 0 {
		return -mate.r, geom.Vector{}, true
	}
	return d.Magnitude() - mate.r, d.WithMagnitude(1), true
}

// mitreFoot is where the mitre meets side i's face near end k. On the
// creased side the other end's balls pass through this side's face near
// the crease, so the height changes sign where the point the ball
// touches the face crosses the crease: a root. On the face both sides
// share, both swept surfaces touch it, so they meet it tangentially,
// where the curves they touch it along cross, and the height only comes
// up to zero there: the arc length whose point comes nearest the other
// end's centres, at the radius. Where that falls short the two curves
// don't cross on the face, and the mitre would leave a gap: refused. A
// chamfer's surfaces cut the face both share, and the height changes
// sign there as on the creased side.
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
		h, _, ok := rb.mateSurface(k, sec.foot[i].p)
		return h, sec, ok
	}
	// From past the crease in along the chain.
	from, step := -rb.r, rb.r/8
	arc := func(x float64) float64 {
		if k == 1 {
			return rb.length - x
		}
		return x
	}
	if rb.ends[k].ban[i] >= 0 || rb.flat {
		// A chamfer's surfaces cross the face both share, not touching
		// it: a root there too.
		// A chamfer's mitre with a smaller one, at a wide corner, can
		// meet the face further past the crease: from twice as far after.
		for _, start := range []float64{from, 2 * from} {
			if start != from && !rb.flat {
				break
			}
			prev, prevG := math.NaN(), math.NaN()
			for x := start; x <= zone; x += step {
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
	gl, _ := h(lo)
	for range 2 {
		if gl > 0 || !rb.flat {
			break
		}
		// A chamfer's plane past the end of a smaller one's, at a wide
		// corner, can lie under the larger's line a radius past its end.
		lo *= 2
		gl, _ = h(lo)
	}
	if !(gl > 0) {
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
	ts := rb.mitreTs(k)
	ss := make([]float64, mitreGrid+1)
	ss[0], ss[mitreGrid] = rb.endS[k][0], rb.endS[k][1]
	for j := 1; j < mitreGrid; j++ {
		tj := ts[j]
		var s float64
		var err error
		if rb.kinkedEnd(k) {
			s, _, err = rb.kinkRoot(k, tj)
		} else {
			s, _, err = rb.mitreRoot(k, func(x rbSection) float64 { return rb.endHeight(end, x.arc.PointAt(tj)) })
		}
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
	grid := rb.mitreTs(k)
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
			ts = append(ts, grid[j])
			xs = append(xs, sec.arc.PointAt(grid[j]))
		}
	}
	mean := sum / (mitreGrid + 1)
	ball, err := rb.section(mean)
	if err != nil {
		return rbSection{}, 0, err
	}
	at := rb.t.Vertices[rb.ends[k].v].P
	if rb.kinkedEnd(k) {
		// All grid points, the ends and the kink among them, in order.
		kts := append(append([]float64{0}, ts...), 1)
		kxs := append(append([]geom.Vector{first.foot[0].p}, xs...), last.foot[1].p)
		fit, us, fxs, ok := rb.kinkedFit(k, ball.arc, first.foot[0], last.foot[1], kts, kxs)
		if !ok {
			return rbSection{}, 0, filletErr(at, "the fillet's sections near its mitre of two radii can't be fitted here")
		}
		return rb.mitreFitted(ball, mean, inner, fit, us, fxs, first, last, slack)
	}
	c := rb.splitSection(raised(ball.arc))
	n := len(c.Ctrl) - 1
	c.Ctrl[0], c.Ctrl[n] = first.foot[0].p, last.foot[1].p
	// The ends turn anywhere in the faces' tangent planes: the swept
	// surface meets the faces tangentially there. A chamfer's meets them
	// at an angle, and its ends turn any way.
	var ways [2][]geom.Vector
	for i, f := range []rbFoot{first.foot[0], last.foot[1]} {
		ways[i] = []geom.Vector{f.n.NormalDir(0), f.n.NormalDir(1)}
		if rb.flat {
			ways[i] = []geom.Vector{{X: 1}, {Y: 1}, {Z: 1}}
		}
	}
	fit, us, ok := fitThrough(c, c.Weight, ways, ts, xs)
	if !ok {
		return rbSection{}, 0, filletErr(at, "the fillet's sections near its mitre can't be fitted here")
	}
	return rb.mitreFitted(ball, mean, inner, fit, us, xs, first, last, slack)
}

// mitreFitted is the section near a mitre whose curve is fit, checked
// against the points xs at its parameters us within slack times the
// sections' tolerance: the ball's at arc length mean otherwise, its feet
// first's and last's, with inner the innermost arc length.
func (rb *rollingBall) mitreFitted(ball rbSection, mean, inner float64, fit srf.BSplineCurve, us []float64, xs []geom.Vector,
	first, last rbSection, slack float64) (rbSection, float64, error) {
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
	if rb.mateKinked(k) {
		// The kinked mitre's first span, which runs to the face both share.
		arc = rb.mateSpan(k)
	}
	if (end.ban[0] < 0) != (end.mate.ends[end.mateK].ban[0] < 0) {
		arc = reversedCurve(arc)
	}
	if rb.mateKinked(k) {
		arc = rb.splitSection(rb.matchParams(k, arc))
	} else if len(rb.kinks) > 0 {
		// A mate of one radius split at the same kinks, which run the
		// other way came back a rounding off: this chain's own.
		arc.Knots = slices.Clone(arc.Knots)
		for i, x := range arc.Knots {
			if j := slices.IndexFunc(rb.kinks, func(t float64) bool { return math.Abs(t-x) < kinkMerge }); j >= 0 {
				arc.Knots[i] = rb.kinks[j]
			}
		}
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
