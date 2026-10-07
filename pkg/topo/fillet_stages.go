package topo

import (
	"errors"
	"math"
	"sort"
	"strings"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// Corners where rounded edges of different radii, or convex and concave
// ones, meet (roadmap 22.4) are rounded in stages, the edges of one
// radius and kind at a time, so that a later stage's edges go on round
// the corners an earlier stage rounded. A box's vertical edges rounded
// first leave its top face's outline lines joined by arcs, which the
// top ring's fillet then follows, a cylinder along each line and a torus
// round each arc; a boss's rounded sides leave its foot's outline the
// same, which the concave fillet at the foot follows. Each surface is
// exact and each seam tangent.
//
// A stage doesn't round an edge of the solid no one picked: where a
// corner's third edge was left sharp, as where two top edges of
// different radii meet above a vertical edge, the later stage's chain
// would have to go on down it, and the corner is refused as before.

// mixedCorner reports whether err refuses a corner where rounded edges
// of different radii, or convex and concave ones, meet.
func mixedCorner(err error) bool {
	var fx *FilletError
	if !errors.As(err, &fx) {
		return false
	}
	return strings.HasPrefix(fx.Msg, "fillets of radius") ||
		strings.HasPrefix(fx.Msg, "a convex edge's fillet meets a concave one's")
}

// stageKey is what the edges rounded in one stage share.
type stageKey struct {
	r       float64
	concave bool
}

// stageOrigins is how far apart in origin the stages' faces are.
const stageOrigins = 0x1000

// filletInStages rounds the picks of orig in stages, and is the shell
// that results: the stages in the first order of their radii and kinds
// that works, the larger radii first and convex edges before concave
// ones of a radius tried first.
func filletInStages(orig *srf.SShell, picks []FilletPick, r float64, origin uint32, flat bool) (srf.SShell, error) {
	t, err := Build(orig)
	if err != nil {
		return srf.SShell{}, err
	}
	// Each pick's edge in the solid as it is, and its stage.
	edgeOf := make([]int, len(picks))
	keyOf := make([]int, len(picks))
	var keys []stageKey
	for i, p := range picks {
		var e int
		if p.Arc {
			e = t.curvedEdgeAt(p.At)
		} else if e, err = t.wholeEdge(p.A, p.B); err != nil {
			return srf.SShell{}, err
		}
		if e < 0 {
			return srf.SShell{}, filletErr(p.At, "no edge of the solid runs there")
		}
		pr := r
		if p.R != 0 {
			pr = p.R
		}
		fe, err := t.filletEdgeOf(e, pr, flat)
		if err != nil {
			return srf.SShell{}, err
		}
		k := stageKey{pr, fe.concave}
		keyOf[i] = -1
		for j, x := range keys {
			if math.Abs(x.r-k.r) <= geom.LengthEps && x.concave == k.concave {
				keyOf[i] = j
			}
		}
		if keyOf[i] < 0 {
			keyOf[i] = len(keys)
			keys = append(keys, k)
		}
		edgeOf[i] = e
	}
	if len(keys) < 2 || len(keys) > 4 {
		return srf.SShell{}, filletErr(geom.Vector{}, "the corners can't be rounded in stages")
	}
	// The edges no one picked that end where picks of more than one
	// stage meet: no stage rounds them.
	picked := map[int]int{}
	for i, e := range edgeOf {
		picked[e] = keyOf[i]
	}
	var sharp []int
	for v := range t.Vertices {
		seen := map[int]bool{}
		for _, x := range t.Vertices[v].Edges {
			if k, ok := picked[x]; ok {
				seen[k] = true
			}
		}
		if len(seen) < 2 {
			continue
		}
		for _, x := range t.Vertices[v].Edges {
			if _, ok := picked[x]; !ok {
				sharp = append(sharp, x)
			}
		}
	}
	tol := math.Max(10*geom.LengthEps, orig.Tol.ChordTolMm())
	// on reports whether edge x of the solid u lies along an edge of the
	// solid as it was, by its middle point.
	on := func(u *Topology, x int, of []int) bool {
		pts := u.Edges[x].Points
		if len(pts) < 2 {
			return false
		}
		m := midpoint(pts)
		for _, e := range of {
			if polylineDistance(t.Edges[e].Points, m) < tol {
				return true
			}
		}
		return false
	}

	// The reason given is the first order's, the one most likely meant.
	var firstErr, lastErr error
	for _, order := range stageOrders(keys) {
		if lastErr != nil && firstErr == nil {
			firstErr = lastErr
		}
		var sh srf.SShell
		sh.MakeFromCopyOf(orig)
		ok := true
		for n, k := range order {
			u, err := Build(&sh)
			if err != nil {
				lastErr, ok = err, false
				break
			}
			var stage []FilletPick
			var others []int
			for i, p := range picks {
				if keyOf[i] != k {
					others = append(others, edgeOf[i])
					continue
				}
				q := p
				q.R = 0
				if !p.Arc {
					a, b, found := u.edgeAlong(p.A, p.B, tol)
					if !found {
						lastErr = filletErr(p.A.Plus(p.B).ScaledBy(0.5), "an edge picked was rounded with another's radius")
						ok = false
						break
					}
					q.A, q.B = a, b
				}
				stage = append(stage, q)
			}
			if !ok {
				break
			}
			forbid := func(u *Topology, x int) bool { return on(u, x, sharp) || on(u, x, others) }
			if err := filletPicks(&sh, stage, keys[k].r, origin+uint32(n)*stageOrigins, flat, forbid); err != nil {
				lastErr, ok = err, false
				break
			}
		}
		if ok {
			return sh, nil
		}
	}
	if firstErr == nil {
		firstErr = lastErr
	}
	return srf.SShell{}, firstErr
}

// stageOrders are the orders to try the stages keys in: by radius, the
// largest first, and convex before concave at one radius; then every
// other order.
func stageOrders(keys []stageKey) [][]int {
	first := make([]int, len(keys))
	for i := range first {
		first[i] = i
	}
	sort.SliceStable(first, func(a, b int) bool {
		ka, kb := keys[first[a]], keys[first[b]]
		if math.Abs(ka.r-kb.r) > geom.LengthEps {
			return ka.r > kb.r
		}
		return !ka.concave && kb.concave
	})
	out := [][]int{first}
	var perm func(prefix []int, rest []int)
	perm = func(prefix, rest []int) {
		if len(rest) == 0 {
			if !equalInts(prefix, first) {
				out = append(out, append([]int(nil), prefix...))
			}
			return
		}
		for i := range rest {
			next := append(append([]int(nil), rest[:i]...), rest[i+1:]...)
			perm(append(prefix, rest[i]), next)
		}
	}
	perm(nil, first)
	return out
}

func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// edgeAlong is the straight edge of the solid that lies along the
// segment from a to b, the longest if there are several: what is left of
// an edge picked from a to b once an earlier stage cut its ends back.
func (t *Topology) edgeAlong(a, b geom.Vector, tol float64) (geom.Vector, geom.Vector, bool) {
	ab := b.Minus(a)
	l := ab.Magnitude()
	if l < geom.LengthEps {
		return a, b, false
	}
	d := ab.ScaledBy(1 / l)
	onSeg := func(p geom.Vector) bool {
		s := p.Minus(a).Dot(d)
		off := p.Minus(a).Minus(d.ScaledBy(s)).Magnitude()
		return off < tol && s > -tol && s < l+tol
	}
	best, bestL := -1, 0.0
	for e := range t.Edges {
		ed := &t.Edges[e]
		if !t.Straight(e) {
			continue
		}
		p, q := t.Vertices[ed.V[0]].P, t.Vertices[ed.V[1]].P
		if !onSeg(p) || !onSeg(q) {
			continue
		}
		if el := q.Minus(p).Magnitude(); el > bestL {
			best, bestL = e, el
		}
	}
	if best < 0 {
		return a, b, false
	}
	ed := &t.Edges[best]
	p, q := t.Vertices[ed.V[0]].P, t.Vertices[ed.V[1]].P
	if q.Minus(p).Dot(d) < 0 {
		p, q = q, p
	}
	return p, q, true
}

// polylineDistance is how far p lies from the polyline pts.
func polylineDistance(pts []geom.Vector, p geom.Vector) float64 {
	best := math.Inf(1)
	for i := 0; i+1 < len(pts); i++ {
		a, b := pts[i], pts[i+1]
		ab := b.Minus(a)
		l2 := ab.Dot(ab)
		s := 0.0
		if l2 > 0 {
			s = math.Max(0, math.Min(1, p.Minus(a).Dot(ab)/l2))
		}
		best = math.Min(best, p.Minus(a.Plus(ab.ScaledBy(s))).Magnitude())
	}
	if len(pts) == 1 {
		best = p.Minus(pts[0]).Magnitude()
	}
	return best
}
