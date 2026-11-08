package topo

import (
	"math"

	"github.com/telesapr/telesapr/pkg/geom"
)

// Mitres at an inside corner. Where two pivot ends (rollingpivot.go)
// meet the crease at one point, as round a pocket in a flat face, the
// ball rolling either way, carried on past the crease along its own
// wall's plane, stays inside the solid, and the fillet is the outside of
// the two swept surfaces: a mitre where they cross, from the crease up
// to where both touch the face they share, a radius past the vertex.
// That is what the balls take away; a corner swung round the crease
// would leave the overlap and come to a cusp on it. Where the two feet
// lie at different depths the walls would have to grow past the crease
// between them, and the ends pivot instead.
//
// Past the crease an end's point on its own wall lies inside the mate's
// swept surface and in along the chain outside it, the other way round
// from a mitre at a convex crease: on the creased side the root is where
// the gap rises through zero, at the crease, and in the band where the
// two surfaces lie inside each other the mitre is its edge in along the
// chain.

// pivotSameFoot is how near two pivot ends' feet on the crease are taken
// as one at least.
const pivotSameFoot = 10 * geom.LengthEps

// insideMitres makes a mitre of each pair of pivot ends of rbs, traced,
// whose feet on the crease are one point.
func insideMitres(rbs []*rollingBall) error {
	for _, rb := range rbs {
		if rb.closed {
			continue
		}
		for k, end := range rb.ends {
			if end == nil || !end.pivot || end.mate == nil {
				continue
			}
			mate := end.mate
			qa, err := rb.creaseFoot(k)
			if err != nil {
				return err
			}
			qb, err := mate.creaseFoot(end.mateK)
			if err != nil {
				return err
			}
			if qa.Minus(qb).Magnitude() > math.Max(rb.tol, pivotSameFoot) {
				continue
			}
			oe := mate.ends[end.mateK]
			for _, x := range []*rbEnd{end, oe} {
				x.pivot, x.mitre, x.curved, x.concave = false, true, true, true
			}
		}
	}
	return nil
}

// creaseFoot is where the ball's section at pivot end k touches the
// crease, its creased side's foot: worked out with the end face's normal
// turned on past the end, as blendOpen turns it.
func (rb *rollingBall) creaseFoot(k int) (geom.Vector, error) {
	end := rb.ends[k]
	n, d, sign := end.n, end.d, end.sign
	if end.n.Dot(rb.beyond(k)) < 0 {
		end.n, end.d, end.sign = end.n.ScaledBy(-1), -end.d, -end.sign
	}
	s, err := rb.pivotS(k)
	end.n, end.d, end.sign = n, d, sign
	if err != nil {
		return geom.Vector{}, err
	}
	sec, err := rb.section(s)
	if err != nil {
		return geom.Vector{}, err
	}
	return sec.foot[rb.creaseSide(k)].p, nil
}

// insideMitreFoot is mitreFoot at a concave crease: on the creased side
// the root where the gap rises through zero, from three radii past the
// vertex; on the face both share the tangency, which lies about a radius
// past it, further where that face curves away.
func (rb *rollingBall) insideMitreFoot(k, i int, gap func(float64) (float64, rbSection, bool),
	arc func(float64) float64, zone, step float64) (float64, rbSection, error) {
	at := rb.t.Vertices[rb.ends[k].v].P
	from := -3 * rb.r
	if rb.ends[k].ban[i] >= 0 || rb.flat {
		prev, prevG := math.NaN(), math.NaN()
		for x := from; x <= zone; x += step {
			g, _, ok := gap(arc(x))
			if !ok {
				continue
			}
			if !math.IsNaN(prevG) && prevG < 0 && g >= 0 {
				lo, hi := prev, x
				for range 80 {
					m := (lo + hi) / 2
					if gm, _, _ := gap(arc(m)); gm < 0 {
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
		return 0, rbSection{}, filletErr(at, "the fillet's mitre at the inside corner doesn't reach the crease here")
	}
	best, bestG := math.NaN(), math.Inf(1)
	for x := from; x <= zone; x += step {
		if g, _, ok := gap(arc(x)); ok && g < bestG {
			best, bestG = x, g
		}
	}
	if math.IsNaN(best) {
		return 0, rbSection{}, filletErr(at, "the fillet's mitre at the inside corner can't be found")
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

// insideMitreRoot is mitreRoot at a concave crease: a point three radii
// past the vertex lies outside the mate's swept surface, falls into the
// band about the plane that halves the turn, and comes out again on the
// chain's own side of it, where the fillet is kept; the root is the
// band's edge in along the chain.
func (rb *rollingBall) insideMitreRoot(k int, h func(float64) (float64, rbSection),
	arc func(float64) float64, zone float64) (float64, rbSection, error) {
	at := rb.t.Vertices[rb.ends[k].v].P
	lo, hi := -3*rb.r, zone
	gl, _ := h(lo)
	gh, _ := h(hi)
	if !(gl > 0) || !(gh > 0) {
		return 0, rbSection{}, filletErr(at, "the fillet's mitre at the inside corner can't be found")
	}
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
	in, out := deep, hi
	for out-in > 1e-12*math.Max(1, rb.length) {
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
