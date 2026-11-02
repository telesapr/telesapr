package topo

import (
	"errors"
	"fmt"
	"math"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// How a closed loft's frame turns from one profile to the next. Most
// rings have their profiles less than a third of a turn apart, and the
// least turn taking one profile's normal onto the next's is the ring's
// turn between them. Past that the least turn stops saying which way
// round the ring goes: at half a turn it has no axis, and past half a
// turn it goes the short way back, which a ring with one long gap (three
// profiles bunched on one side) does not. Such a ring is turned about
// its own axis, the normal of the polygon of its profiles' centres, by
// the angle its centres go round the circle fitted through them, the
// profiles' normals' turn about the axis taken on that branch; what
// that leaves of the turn onto the next normal is a least turn after it.

// wideTurn is the most the least turn between two profiles may be for
// the ring to be turned by least turns (wideRing otherwise).
const wideTurn = 2 * math.Pi / 3

// ringGap is how a closed loft's frame turns from profile a to the next,
// b: by the least turn from na to nb, or on a wide ring about the ring's
// axis by angle and then by the least turn tiltAngle about tiltAxis.
type ringGap struct {
	na, nb    geom.Vector
	axis      geom.Vector
	angle     float64
	wide      bool
	tiltAxis  geom.Vector
	tiltAngle float64
}

// leastGap is the gap from na to nb turned by the least turn.
func leastGap(na, nb geom.Vector) ringGap {
	axis, angle := turnBetween(na, nb)
	return ringGap{na: na, nb: nb, axis: axis, angle: angle}
}

// turn is the frame f of the way from a to b, 0 to 1.
func (g ringGap) turn(f float64) func(geom.Vector) geom.Vector {
	spin := geom.QuaternionFromAxisAngle(g.axis, f*g.angle)
	if !g.wide || g.tiltAngle == 0 {
		return spin.Rotate
	}
	tilt := geom.QuaternionFromAxisAngle(g.tiltAxis, f*g.tiltAngle)
	return func(p geom.Vector) geom.Vector { return tilt.Rotate(spin.Rotate(p)) }
}

// forward takes a's frame onto b's.
func (g ringGap) forward() func(geom.Vector) geom.Vector {
	if !g.wide {
		return turnTo(g.na, g.nb)
	}
	return g.turn(1)
}

// back takes b's frame onto a's.
func (g ringGap) back() func(geom.Vector) geom.Vector {
	if !g.wide {
		return turnTo(g.nb, g.na)
	}
	spin := geom.QuaternionFromAxisAngle(g.axis, -g.angle)
	tilt := geom.QuaternionFromAxisAngle(g.tiltAxis, -g.tiltAngle)
	return func(p geom.Vector) geom.Vector { return spin.Rotate(tilt.Rotate(p)) }
}

// arcRatio is the length of the arc the ring's centres take over the gap
// to the chord between them: 1 on a ring of least turns, whose
// parameter goes by chords as it always has.
func (g ringGap) arcRatio() float64 {
	if !g.wide || g.angle < 1e-9 {
		return 1
	}
	return (g.angle / 2) / math.Sin(g.angle/2)
}

// ringGaps are the gaps between the profiles of loops, oriented as
// ClosedLoftSections orients them, and whether the ring is wide: some
// profile turns more than wideTurn onto the next, or the chord between
// two leaves one of them backwards. On a ring that isn't, every gap is a
// least turn.
func ringGaps(loops [][]srf.SBezier) ([]ringGap, bool) {
	k := len(loops)
	gaps := make([]ringGap, k)
	wide := false
	for i := range k {
		a, b := loops[i], loops[(i+1)%k]
		na, nb := newell(a).WithMagnitude(1), newell(b).WithMagnitude(1)
		gaps[i] = leastGap(na, nb)
		chord := centroid(b).Minus(centroid(a))
		if na.Dot(nb) < math.Cos(wideTurn)-1e-6 || na.Dot(chord) < 0 || nb.Dot(chord) < 0 {
			wide = true
		}
	}
	return gaps, wide
}

// errRingOrder is a ring whose profiles' centres don't go once round it
// in their order.
var errRingOrder = errors.New("the profiles don't go once round the ring in their order; draw them in turn round it")

// wideRing orients loops to face the way round their ring and turns
// each to start where the one before it does, seen in its frame, for a
// ring some of whose profiles are more than wideTurn apart: see the
// comment at the top of the file. The gaps are those between the
// oriented loops.
func wideRing(loops [][]srf.SBezier) ([][]srf.SBezier, []ringGap, error) {
	k := len(loops)
	c := make([]geom.Vector, k)
	for i, l := range loops {
		c[i] = centroid(l)
	}
	axis := newellOf(c)
	if axis.Magnitude() < geom.LengthEps*geom.LengthEps {
		// No area within: on a line, or crossing back over itself.
		d := c[1].Minus(c[0])
		for _, p := range c {
			if p.Minus(c[0]).Cross(d).Magnitude() > geom.LengthEps*d.Magnitude() {
				return nil, nil, errRingOrder
			}
		}
		return nil, nil, errors.New("the profiles' centres lie on a line; a closed loft goes round a ring")
	}
	axis = axis.WithMagnitude(1)
	centre, ok := ringCentre(c, axis)
	if !ok {
		return nil, nil, errRingOrder
	}
	// The angle of each centre about the ring's centre, and each loop
	// faced the way round there.
	u := axis.NormalDir(0)
	v := axis.Cross(u)
	phi := make([]float64, k)
	for i := range k {
		r := c[i].Minus(centre)
		phi[i] = math.Atan2(r.Dot(v), r.Dot(u))
		if newell(loops[i]).Dot(axis.Cross(r)) < 0 {
			loops[i] = reverseLoop(loops[i])
		}
	}
	gaps := make([]ringGap, k)
	total := 0.0
	for i := range k {
		b := (i + 1) % k
		dphi := math.Mod(phi[b]-phi[i]+4*math.Pi, 2*math.Pi)
		if dphi < 1e-6 {
			return nil, nil, errRingOrder
		}
		total += dphi
		na, nb := newell(loops[i]).WithMagnitude(1), newell(loops[b]).WithMagnitude(1)
		// The normals' turn about the axis, on the branch nearest the
		// centres' turn.
		pa, pb := na.Minus(axis.ScaledBy(na.Dot(axis))), nb.Minus(axis.ScaledBy(nb.Dot(axis)))
		if pa.Magnitude() < 1e-6 || pb.Magnitude() < 1e-6 {
			return nil, nil, fmt.Errorf("profile %d faces along the ring's axis, not round it", min(i, b)+1)
		}
		theta := math.Atan2(axis.Dot(pa.Cross(pb)), pa.Dot(pb))
		theta += 2 * math.Pi * math.Round((dphi-theta)/(2*math.Pi))
		if theta <= 0 || theta >= 2*math.Pi {
			return nil, nil, fmt.Errorf("profiles %d and %d face against the way round the ring", i+1, b+1)
		}
		g := ringGap{na: na, nb: nb, axis: axis, angle: theta, wide: true}
		g.tiltAxis, g.tiltAngle = turnBetween(geom.QuaternionFromAxisAngle(axis, theta).Rotate(na), nb)
		gaps[i] = g
	}
	if math.Abs(total-2*math.Pi) > 1e-6 {
		return nil, nil, errRingOrder
	}
	for i := 1; i < k; i++ {
		loops[i] = alignTurned(loops[i], loops[i-1], gaps[i-1].back())
	}
	return loops, gaps, nil
}

// ringCentre is the centre of the circle fitted through the points c,
// projected onto the plane through their mean square to axis, by least
// squares on x² + y² = 2ax + 2by + e.
func ringCentre(c []geom.Vector, axis geom.Vector) (geom.Vector, bool) {
	var mean geom.Vector
	for _, p := range c {
		mean = mean.Plus(p)
	}
	mean = mean.ScaledBy(1 / float64(len(c)))
	u := axis.NormalDir(0)
	v := axis.Cross(u)
	var m [3][3]float64
	var rhs [3]float64
	for _, p := range c {
		d := p.Minus(mean)
		x, y := d.Dot(u), d.Dot(v)
		row := [3]float64{2 * x, 2 * y, 1}
		for i := range 3 {
			for j := range 3 {
				m[i][j] += row[i] * row[j]
			}
			rhs[i] += row[i] * (x*x + y*y)
		}
	}
	sol, ok := solve3(m, rhs)
	if !ok {
		return geom.Vector{}, false
	}
	return mean.Plus(u.ScaledBy(sol[0])).Plus(v.ScaledBy(sol[1])), true
}

// solve3 solves m x = rhs by Cramer's rule, or reports m singular.
func solve3(m [3][3]float64, rhs [3]float64) ([3]float64, bool) {
	det := func(a [3][3]float64) float64 {
		return a[0][0]*(a[1][1]*a[2][2]-a[1][2]*a[2][1]) -
			a[0][1]*(a[1][0]*a[2][2]-a[1][2]*a[2][0]) +
			a[0][2]*(a[1][0]*a[2][1]-a[1][1]*a[2][0])
	}
	d := det(m)
	scale := 0.0
	for i := range 3 {
		for j := range 3 {
			scale = max(scale, math.Abs(m[i][j]))
		}
	}
	if math.Abs(d) <= 1e-12*scale*scale*scale {
		return [3]float64{}, false
	}
	var out [3]float64
	for col := range 3 {
		a := m
		for row := range 3 {
			a[row][col] = rhs[row]
		}
		out[col] = det(a) / d
	}
	return out, true
}

// alignTurned is alignTo with loop's joints first turned by back into
// prev's frame, about loop's centre.
func alignTurned(loop, prev []srf.SBezier, back func(geom.Vector) geom.Vector) []srf.SBezier {
	n := len(loop)
	cl, cp := centroid(loop), centroid(prev)
	best, bestK := math.Inf(1), 0
	for k := range n {
		d := 0.0
		for i := range n {
			a := back(loop[(i+k)%n].Start().Minus(cl))
			b := prev[i].Start().Minus(cp)
			d += a.Minus(b).MagSquared()
		}
		if d < best-geom.LengthEps {
			best, bestK = d, k
		}
	}
	return append(append([]srf.SBezier(nil), loop[bestK:]...), loop[:bestK]...)
}
