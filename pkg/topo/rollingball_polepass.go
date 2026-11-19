package topo

import (
	"math"
	"slices"

	"github.com/telesapr/telesapr/pkg/geom"
)

// rbPoleSnap is a section whose foot on side k is put on the pole at
// vertex v: the ball's point passes within a speck of it.
type rbPoleSnap struct {
	s float64
	k int
	v int
}

// section is the ball at arc length s along the edge (ballSection), its
// foot put on a pole it passes within a speck of (nearPoles).
func (rb *rollingBall) section(s float64) (rbSection, error) {
	sec, err := rb.ballSection(s)
	if err != nil {
		return sec, err
	}
	for _, sn := range rb.poleSnaps {
		if sn.s == s {
			snapFoot(&sec, sn.k, rb.t.Vertices[sn.v].P)
		}
	}
	return sec, nil
}

// snapFoot moves side k's foot of sec to p, and the section's curve
// with it, the more the nearer that foot: its other end stays.
func snapFoot(sec *rbSection, k int, p geom.Vector) {
	arc := &sec.arc
	n := len(arc.Ctrl) - 1
	if n < 1 {
		return
	}
	end := 0
	if arc.Ctrl[n].Minus(sec.foot[k].p).Magnitude() < arc.Ctrl[0].Minus(sec.foot[k].p).Magnitude() {
		end = n
	}
	d := p.Minus(arc.Ctrl[end])
	ctrl := slices.Clone(arc.Ctrl)
	for i := range ctrl {
		share := 1 - float64(i)/float64(n)
		if end == n {
			share = float64(i) / float64(n)
		}
		ctrl[i] = ctrl[i].Plus(d.ScaledBy(share))
	}
	arc.Ctrl = ctrl
	sec.foot[k].p = p
}

// poleSpeck is how near a pole the ball's point passes for it to go
// through the pole (nearPoles): twice the tolerance the fillet's surface
// is fitted to. Between the section put on the pole and its neighbours
// the surface strays from the ball by about a fifth of the distance it
// was moved, within the fit's tolerance up to four times it (refused
// "can't be fitted" past that). Seams cut further from the pole than
// this leave a sliver of a patch that still closes: at a chord tolerance
// of 0.01 mm, those cut 0.0014 mm off and more did, those 0.00085 mm
// off and less came out open (TestFilletNearPole). The seams a single
// crossing cuts this near a pole are refused (cutBack).
func (rb *rollingBall) poleSpeck() float64 { return math.Max(2*rb.tol, 10*geom.LengthEps) }

// nearPoles takes the crossings of seams the ball's point makes within
// a speck of a pole of a face (a sphere's top, a cone's apex), where
// its patches' seams meet, as its passing through the pole: of a patch
// crossed for no more than a speck there, two crossings become one
// from the patch before to the patch after, and the foot of the section
// there is put on the pole. Cut a speck from the pole, the seams would
// leave a sliver of a patch no mesh closes round. Only for an open
// chain, whose crossings' arc lengths are its sections'.
func (rb *rollingBall) nearPoles(at []float64, pairs [][2]int, sides []int) ([]float64, [][2]int, []int) {
	if rb.closed || len(at) == 0 {
		return at, pairs, sides
	}
	t := rb.t
	speck := rb.poleSpeck()
	foot := func(s float64, k int) (geom.Vector, bool) {
		sec, err := rb.section(s)
		return sec.foot[k].p, err == nil
	}
	var oat []float64
	var opairs [][2]int
	var osides []int
	var snaps []rbPoleSnap
	for i := 0; i < len(at); i++ {
		k := sides[i]
		p, ok := foot(at[i], k)
		if !ok {
			oat, opairs, osides = append(oat, at[i]), append(opairs, pairs[i]), append(osides, k)
			continue
		}
		if i+1 < len(at) && sides[i+1] == k && pairs[i][1] == pairs[i+1][0] && pairs[i][0] != pairs[i+1][1] {
			if q, ok := foot(at[i+1], k); ok {
				if v, ok := rb.poleNear(speck, pairs[i][1], p, q); ok {
					s := rb.nearestToPole(at[i], at[i+1], k, t.Vertices[v].P)
					oat, opairs, osides = append(oat, s), append(opairs, [2]int{pairs[i][0], pairs[i+1][1]}), append(osides, k)
					snaps = append(snaps, rbPoleSnap{s: s, k: k, v: v})
					i++
					continue
				}
			}
		}
		if !t.shareSeam(pairs[i]) {
			// Crossed from one patch to one that meets it only at a pole:
			// through the pole within the search's steps.
			if v, ok := rb.poleNear(speck, pairs[i][0], p); ok && collapsedAt(t.Surface(pairs[i][1]), t.Vertices[v].P) {
				snaps = append(snaps, rbPoleSnap{s: at[i], k: k, v: v})
			}
		}
		oat, opairs, osides = append(oat, at[i]), append(opairs, pairs[i]), append(osides, k)
	}
	rb.poleSnaps = snaps
	return oat, opairs, osides
}

// shareSeam reports whether faces fs meet along an edge.
func (t *Topology) shareSeam(fs [2]int) bool {
	for y := range t.Edges {
		if sameFaces(t.EdgeFaces(y), fs) {
			return true
		}
	}
	return false
}

// poleNear is a vertex within d of every point of ps at which face f is
// collapsed: a pole of the patch.
func (rb *rollingBall) poleNear(d float64, f int, ps ...geom.Vector) (int, bool) {
	t := rb.t
	s := t.Surface(f)
	for v := range t.Vertices {
		vp := t.Vertices[v].P
		if len(t.Vertices[v].Edges) < 3 || slices.ContainsFunc(ps, func(p geom.Vector) bool { return p.Minus(vp).Magnitude() > d }) {
			continue
		}
		if collapsedAt(s, vp) {
			return v, true
		}
	}
	return -1, false
}

// nearestToPole is the arc length between a and b at which side k's
// foot passes nearest p, of a few: the ball's point there is put on the
// pole.
func (rb *rollingBall) nearestToPole(a, b float64, k int, p geom.Vector) float64 {
	best, near := (a+b)/2, math.Inf(1)
	const steps = 8
	for i := range steps + 1 {
		s := a + (b-a)*float64(i)/steps
		sec, err := rb.ballSection(s)
		if err != nil {
			continue
		}
		if d := sec.foot[k].p.Minus(p).Magnitude(); d < near {
			best, near = s, d
		}
	}
	return best
}

// poleSnapAt is the pole crossing c was put through (nearPoles).
func (rb *rollingBall) poleSnapAt(c rbCrossing) (rbPoleSnap, bool) {
	for _, sn := range rb.poleSnaps {
		if sn.s == c.s && sn.k == c.side {
			return sn, true
		}
	}
	return rbPoleSnap{}, false
}

// passPole is the pole at vertex v that crossing c passes through: its
// seams that run to the edge go, with the patches between them that are
// neither of c's (the ball's point passes over them at the pole alone);
// its other seams stay, the patches beyond them untouched.
func (rb *rollingBall) passPole(v int, c rbCrossing, isChain map[int]bool, reaches func(int) bool) (rbPole, bool) {
	t := rb.t
	p := rbPole{v: v}
	for _, e := range t.Vertices[v].Edges {
		ed := &t.Edges[e]
		w := ed.V[0]
		if w == v {
			w = ed.V[1]
		}
		if isChain[e] || w == v {
			return rbPole{}, false
		}
		if !reaches(w) {
			continue
		}
		p.to = append(p.to, w)
		for _, f := range t.EdgeFaces(e) {
			if f != c.faces[0] && f != c.faces[1] && !slices.Contains(p.swallowed, f) {
				p.swallowed = append(p.swallowed, f)
			}
		}
	}
	return p, len(p.to) > 0
}
