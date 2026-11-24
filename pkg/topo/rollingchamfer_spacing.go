package topo

import (
	"math"
	"slices"

	"github.com/telesapr/telesapr/pkg/geom"
)

// A chamfer is flat across whatever its distance, so how far apart its
// first sections need be is set by how its edge and faces bend, not by
// the distance as a fillet's are (two radii): a chamfer of 0.2 along a
// gently curved edge would otherwise take a section every 0.4 mm, and
// most of a model's patches. Where the spacing below isn't close enough,
// the misfit between sections adds more.
const (
	// chamferTurn is how far, in radians, the edge or either face's
	// normal along it may turn between two sections.
	chamferTurn = math.Pi / 8
	// chamferTurnChange is how far the turn over a span may differ from
	// the turn at its start's rate: where the edge's bending changes at
	// once, as where a sketch's line goes on into an arc, the skin
	// through the sections, smooth across them, rings on either side.
	chamferTurnChange = math.Pi / 64
)

// firstSections are the arc lengths of the first sections between a and
// b along the edge, a the first of them and b left for the next: spaced
// evenly at most h apart for a fillet; for a chamfer (flat) by how the
// edge and its faces bend (chamferSpacing).
func (rb *rollingBall) firstSections(a, b, h float64) []float64 {
	var sp sectionDensity
	if rb.flat {
		sp = rb.chamferSpacing(h)
	}
	if !rb.flat || sp.even {
		// A chamfer's edge too short, or bending too fast all along, for
		// its sections to lie further apart than h: as a fillet's, to the
		// bit, since a blend's later steps (a run-off's dip under a
		// floor) may turn on the last bits of where its sections lie.
		m := int(math.Ceil((b - a) / h))
		ss := make([]float64, m)
		for j := range m {
			ss[j] = a + (b-a)*float64(j)/float64(m)
		}
		return ss
	}
	da, db := sp.at(a), sp.at(b)
	m := max(1, int(math.Ceil(db-da-1e-9)))
	ss := make([]float64, m)
	ss[0] = a
	for j := 1; j < m; j++ {
		ss[j] = sp.inverse(da+(db-da)*float64(j)/float64(m), a, b)
	}
	return ss
}

// sectionDensity is how many of a chamfer's first sections lie along
// the edge up to each of its dense points (rollingBall.pts): the
// integral of one over the spacing there.
type sectionDensity struct {
	arcl []float64
	cum  []float64
	// first and last are the density before the edge's start and past
	// its end, an open edge's ends carried on.
	first, last float64
	length      float64
	closed      bool
	// even is that the spacing is h all along.
	even bool
}

// chamferSpacing is the density of a chamfer's first sections along the
// edge. At each piece of the dense edge the spacing is the length along
// which the edge's tangent and the faces' normals turn by chamferTurn,
// and no more than 2 chamferTurnChange over how much their rate of turn
// changes within a twelfth of the edge either side; kept between h and
// a twelfth of the edge.
func (rb *rollingBall) chamferSpacing(h float64) sectionDensity {
	hmax := math.Max(h, rb.length/12)
	n := len(rb.pts)
	segs := n - 1
	if rb.closed {
		segs = n
	}
	angle := func(p, q geom.Vector) float64 {
		if p.Magnitude() == 0 || q.Magnitude() == 0 {
			return 0
		}
		return math.Atan2(p.Cross(q).Magnitude(), p.Dot(q))
	}
	rate := make([]float64, segs)
	ds := make([]float64, segs)
	for i := range segs {
		j := (i + 1) % n
		ds[i] = rb.arcl[i+1] - rb.arcl[i]
		turn := angle(rb.tan[i], rb.tan[j])
		if i < len(rb.nrm) && j < len(rb.nrm) {
			for k := range 2 {
				turn = math.Max(turn, angle(rb.nrm[i][k], rb.nrm[j][k]))
			}
		}
		if ds[i] > 0 {
			rate[i] = turn / ds[i]
		}
	}
	// next is the piece after i one way (by +1 or -1), or -1 past an
	// open edge's end.
	next := func(i, by int) int {
		i += by
		if rb.closed {
			return (i + segs) % segs
		}
		if i < 0 || i >= segs {
			return -1
		}
		return i
	}
	sd := sectionDensity{arcl: rb.arcl, cum: make([]float64, segs+1), length: rb.length, closed: rb.closed, even: true}
	for i := range segs {
		lo, hi := rate[i], rate[i]
		for _, by := range []int{-1, 1} {
			run := 0.0
			for j := next(i, by); j >= 0 && j != i && run < hmax; j = next(j, by) {
				lo, hi = math.Min(lo, rate[j]), math.Max(hi, rate[j])
				run += ds[j]
			}
		}
		step := hmax
		if rate[i] > 0 {
			step = math.Min(step, chamferTurn/rate[i])
		}
		if hi > lo {
			step = math.Min(step, 2*chamferTurnChange/(hi-lo))
		}
		step = math.Max(h, step)
		sd.even = sd.even && step == h
		rho := 1 / step
		sd.cum[i+1] = sd.cum[i] + rho*ds[i]
		if i == 0 {
			sd.first = rho
		}
		sd.last = rho
	}
	return sd
}

// at is how many sections lie from the edge's start to arc length s.
func (sd sectionDensity) at(s float64) float64 {
	segs := len(sd.cum) - 1
	if sd.closed {
		q := math.Floor(s / sd.length)
		return q*sd.cum[segs] + sd.within(s-q*sd.length)
	}
	switch {
	case s <= 0:
		return s * sd.first
	case s >= sd.length:
		return sd.cum[segs] + (s-sd.length)*sd.last
	}
	return sd.within(s)
}

// within is at for s along the dense edge itself.
func (sd sectionDensity) within(s float64) float64 {
	segs := len(sd.cum) - 1
	i, _ := slices.BinarySearch(sd.arcl[:segs+1], s)
	i = min(max(i-1, 0), segs-1)
	ds := sd.arcl[i+1] - sd.arcl[i]
	if ds <= 0 {
		return sd.cum[i]
	}
	f := math.Max(0, math.Min(1, (s-sd.arcl[i])/ds))
	return sd.cum[i] + f*(sd.cum[i+1]-sd.cum[i])
}

// inverse is the arc length between a and b where at is v.
func (sd sectionDensity) inverse(v, a, b float64) float64 {
	for range 100 {
		m := (a + b) / 2
		if sd.at(m) < v {
			a = m
		} else {
			b = m
		}
		if b-a < 1e-12*math.Max(1, sd.length) {
			break
		}
	}
	return (a + b) / 2
}
