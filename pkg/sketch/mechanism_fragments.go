package sketch

import (
	"math"

	"github.com/telesapr/telesapr/pkg/geom"
)

// The plan places a part from the mates that join it to the bodies
// placed before it (mechanism_plan.go), in closed form, as Kramer's
// plan fragments do: the turn first, then the move.
//
// The turn: each mate that holds a direction of the part along a
// direction of a placed body is an alignment (a parallel; a point on a
// point with a second point of the part on a line through the first, a
// hinge's pin; two points on two points). The first alignment turns the
// part by the least turn that lays its direction along the other, the
// way it points now or the other way, whichever is nearer; a second, not
// parallel to the first, turns it about the first direction until it
// lies along it too, and the turn is fixed. With one alignment the part
// may still turn about that direction: its turn is free (a hinge, a pin
// in its hole), or held by another of its mates (a driven hinge's axis
// angle, a direction parallel seen in a workplane, or a mate that
// closes a loop), which a search over that one angle meets. With no
// alignment, a part held by points alone (a ball, a slot), its turn is
// three unknowns of the loop's search, from the turn it has.
//
// The move: with the turn held, what every mate misses by is affine in
// the part's move, so the move is a linear least squares in its three
// components, the one nearest where the part is (its mates' points kept
// where they were) along the directions the mates leave free: those are
// the part's slides. What the least squares leaves over, the part's
// mates can't all hold where the bodies before it are: a loop's closing
// mate, which the loop's search (mechanism_loop.go) takes up.

// align is a direction of a part, in its own frame, that a mate lays
// along a direction of a body placed before it, where that is now.
type align struct {
	local, world geom.Vector
}

// lineFeatTol is how near a point must lie to a line, relative to the
// distances involved, to be taken as on it when a hinge's pin is
// recognised.
const lineFeatTol = 1e-6

// turnFreeTol is how little a part's mates may change when it turns
// about its one alignment for the turn to be counted free.
const turnFreeTol = 1e-9

// alignments are the alignments of body b by the mates placing it,
// with the bodies before it where placements pl put them.
func (r *planRun) alignments(b int) []align {
	var out []align
	type pin struct{ local, world geom.Vector }
	var at []pin // the part's points held on points
	// A point on a line: the part's on a placed line (local), or a
	// placed one on the part's line.
	type onLine struct {
		local   bool
		p, o, d geom.Vector
	}
	var lines []onLine
	unit := func(v geom.Vector) geom.Vector { return v.WithMagnitude(1) }
	for _, mi := range r.plan.placing[b] {
		m := &r.mates[mi]
		switch m.kind {
		case mateParallel:
			a, o := m.d[0], m.d[1]
			if a.body != b {
				a, o = o, a
			}
			out = append(out, align{unit(a.local), unit(o.dir(r.pl))})
		case mateCoincident:
			p, k := m.p[0], m.p[1]
			if p.body != b {
				p, k = k, p
			}
			at = append(at, pin{p.local, k.point(r.pl)})
		case matePointOnLine:
			if m.p[0].body == b {
				lines = append(lines, onLine{local: true, p: m.p[0].local, o: m.o.point(r.pl), d: unit(m.d[0].dir(r.pl))})
			} else {
				lines = append(lines, onLine{p: m.p[0].point(r.pl), o: m.o.local, d: unit(m.d[0].local)})
			}
		}
	}
	off := func(p, o, d geom.Vector) float64 {
		v := p.Minus(o)
		return v.Minus(d.ScaledBy(v.Dot(d))).Magnitude() / (1 + v.Magnitude())
	}
	for i, c := range at {
		for _, l := range lines {
			if l.local {
				// The part's point on a line through the point it is
				// held on: the part's pin, from the one to the other,
				// along the line.
				if v := l.p.Minus(c.local); v.Magnitude() > lineFeatTol && off(c.world, l.o, l.d) < lineFeatTol {
					out = append(out, align{unit(v), l.d})
				}
			} else if v := l.p.Minus(c.world); v.Magnitude() > lineFeatTol && off(c.local, l.o, l.d) < lineFeatTol {
				// A placed point on the part's line through its point held:
				// the line, along from the one to the other.
				out = append(out, align{l.d, unit(v)})
			}
		}
		for _, c2 := range at[i+1:] {
			if v, w := c2.local.Minus(c.local), c2.world.Minus(c.world); v.Magnitude() > lineFeatTol && w.Magnitude() > lineFeatTol {
				out = append(out, align{unit(v), unit(w)})
			}
		}
	}
	return out
}

// leastTurn is the least turn that takes unit vector a to unit vector
// b, which do not point apart.
func leastTurn(a, b geom.Vector) geom.Quaternion {
	x := a.Cross(b)
	s := x.Magnitude()
	if s < 1e-15 {
		return geom.Quaternion{W: 1}
	}
	return geom.QuaternionFromAxisAngle(x.ScaledBy(1/s), math.Atan2(s, a.Dot(b)))
}

// alignTurn is the turn of a part that turned qs before satisfies its
// alignments al, nearest qs; with turns, it may still turn about axis.
// With no alignment it is qs, which placeBody turns by the loop's
// unknowns.
func alignTurn(qs geom.Quaternion, al []align) (q geom.Quaternion, axis geom.Vector, turns bool) {
	if len(al) == 0 {
		return qs, geom.Vector{}, false
	}
	w1, t1 := qs.Rotate(al[0].local), al[0].world
	if w1.Dot(t1) < 0 {
		t1 = t1.ScaledBy(-1)
	}
	q = leastTurn(w1, t1).Times(qs).WithMagnitude(1)
	for _, a := range al[1:] {
		if a.local.Cross(al[0].local).Magnitude() < lineFeatTol {
			continue
		}
		w2 := q.Rotate(a.local)
		pw := w2.Minus(t1.ScaledBy(t1.Dot(w2)))
		pt := a.world.Minus(t1.ScaledBy(t1.Dot(a.world)))
		if pw.Magnitude() < lineFeatTol || pt.Magnitude() < lineFeatTol {
			continue
		}
		// Either way along the second direction, whichever turns less.
		turn := func(pt geom.Vector) float64 { return math.Atan2(pw.Cross(pt).Dot(t1), pw.Dot(pt)) }
		th := turn(pt)
		if o := turn(pt.ScaledBy(-1)); math.Abs(o) < math.Abs(th) {
			th = o
		}
		return geom.QuaternionFromAxisAngle(t1, th).Times(q).WithMagnitude(1), t1, false
	}
	return q, t1, true
}

// settleMove places body b turned q: its move the least squares of
// what its placing mates miss by, nearest where its anchor was. It
// returns what they miss by then and the directions in which the move
// is free, which change nothing they miss by.
func (r *planRun) settleMove(b int, q geom.Quaternion) (g []float64, free []geom.Vector) {
	br := &r.bodies[b]
	tb := br.anchorWorld.Minus(q.Rotate(br.anchorLocal))
	r.pl[b] = place{tb, q}
	g0 := r.bodyResidual(b, nil)
	var cols [3][]float64
	for k := range 3 {
		var e geom.Vector
		switch k {
		case 0:
			e.X = 1
		case 1:
			e.Y = 1
		default:
			e.Z = 1
		}
		r.pl[b].t = tb.Plus(e)
		cols[k] = r.bodyResidual(b, nil)
		for i := range cols[k] {
			cols[k][i] -= g0[i]
		}
	}
	var a [3][3]float64
	var y [3]float64
	for i := range 3 {
		for j := range 3 {
			for k := range g0 {
				a[i][j] += cols[i][k] * cols[j][k]
			}
		}
		for k := range g0 {
			y[i] += cols[i][k] * g0[k]
		}
	}
	vals, vecs := symEigen3(a)
	top := math.Max(vals[0], math.Max(vals[1], vals[2]))
	var step geom.Vector
	for i := range 3 {
		v := geom.Vector{X: vecs[0][i], Y: vecs[1][i], Z: vecs[2][i]}
		if vals[i] <= 1e-10*top || vals[i] <= 1e-20 {
			free = append(free, v)
			continue
		}
		step = step.Minus(v.ScaledBy((v.X*y[0] + v.Y*y[1] + v.Z*y[2]) / vals[i]))
	}
	r.pl[b].t = tb.Plus(step)
	return r.bodyResidual(b, nil), free
}

// bodyResidual appends to out what the mates placing body b miss by.
func (r *planRun) bodyResidual(b int, out []float64) []float64 {
	for _, mi := range r.plan.placing[b] {
		out = r.mates[mi].residual(r.pl, out)
	}
	return out
}

// placeBody places body b, after the bodies before it, with the loop
// search's unknowns vars: its free turn and its slides, from where
// the plan puts it. The first placement of a run (structure) finds
// which turns are free and what slides there are, and gives each an
// unknown.
func (r *planRun) placeBody(b int, vars []float64, structure bool) {
	br := &r.bodies[b]
	al := r.alignments(b)
	q0, axis, turns := alignTurn(r.start[b].q, al)
	// A part held by points alone, a ball or a slot, has no direction to
	// lay along another: its turn is three unknowns of the loop, a
	// rotation vector, zero where it is turned now.
	if len(al) == 0 && len(r.plan.placing[b]) > 0 {
		if structure {
			br.ballVar = r.nv
			r.nv += 3
			r.isTurn = append(r.isTurn, true, true, true)
		}
		if br.ballVar >= 0 {
			v := geom.Vector{X: varAt(vars, br.ballVar), Y: varAt(vars, br.ballVar+1), Z: varAt(vars, br.ballVar+2)}
			if a := v.Magnitude(); a > 0 {
				q0 = geom.QuaternionFromAxisAngle(v.ScaledBy(1/a), a).Times(q0).WithMagnitude(1)
			}
		}
	}
	var free []geom.Vector
	at := func(th float64) []float64 {
		q := q0
		if th != 0 {
			q = geom.QuaternionFromAxisAngle(axis, th).Times(q0).WithMagnitude(1)
		}
		var g []float64
		g, free = r.settleMove(b, q)
		return g
	}
	th := 0.0
	if turns {
		if structure {
			br.turnVar = -1
			br.solvedTurn = differs(at(0), at(2), turnFreeTol)
			if !br.solvedTurn {
				br.turnVar = r.nv
				r.isTurn = append(r.isTurn, true)
				r.nv++
			}
		}
		switch {
		case br.solvedTurn:
			th = r.searchTurn(at, br.theta, structure)
			br.theta = th
		case br.turnVar >= 0:
			th = varAt(vars, br.turnVar)
		}
	}
	br.g = at(th)
	if structure {
		br.slides = free
		br.slideVar = r.nv
		r.nv += len(free)
		for range free {
			r.isTurn = append(r.isTurn, false)
		}
	}
	// The slides kept pointing as they first did, so that an unknown
	// means the same move at every step.
	for k, s := range br.slides {
		if k >= len(free) {
			break
		}
		d := free[k]
		if d.Dot(s) < 0 {
			d = d.ScaledBy(-1)
		}
		r.pl[b].t = r.pl[b].t.Plus(d.ScaledBy(varAt(vars, br.slideVar+k)))
	}
}

// varAt is the unknown i of vars, 0 before there are any.
func varAt(vars []float64, i int) float64 {
	if i < len(vars) {
		return vars[i]
	}
	return 0
}

// differs reports whether a and b differ by more than tol anywhere.
func differs(a, b []float64, tol float64) bool {
	for i := range a {
		if math.Abs(a[i]-b[i]) > tol {
			return true
		}
	}
	return false
}

// searchTurn is the angle, of a part's one free turn about its
// alignment, at which g, what its mates miss by, is least, nearest
// from: Newton's method on the one angle from there; and at a run's
// first placement (sweep), or where Newton's doesn't settle, the least
// of the minima of a sweep round the whole turn, of them the nearest
// from. It is the closed form of a point on a circle meeting a line or
// a plane, found numerically, so that one search serves every mate.
func (r *planRun) searchTurn(g func(float64) []float64, from float64, sweep bool) float64 {
	th, s, settled := newtonTurn(g, from)
	if s <= planTol*planTol || settled && !sweep {
		return th
	}
	const n = 36
	sq := make([]float64, n)
	for k := range n {
		sq[k] = sumSq(g(from + 2*math.Pi*float64(k)/n))
	}
	best, bestS := th, s
	if !settled {
		bestS = math.Inf(1)
	}
	for k := range n {
		if sq[k] > sq[(k+1)%n] || sq[k] > sq[(k+n-1)%n] {
			continue
		}
		t, ts, _ := newtonTurn(g, from+2*math.Pi*float64(k)/n)
		// The least, and of those as least, the nearest.
		near := math.Abs(math.Remainder(t-from, 2*math.Pi)) < math.Abs(math.Remainder(best-from, 2*math.Pi))
		if ts < bestS*(1-1e-9)-1e-24 || ts <= bestS*(1+1e-9)+1e-24 && near {
			best, bestS = t, ts
		}
	}
	return from + math.Remainder(best-from, 2*math.Pi)
}

// newtonTurn is Gauss–Newton on one angle from th for the least of
// g's sum of squares, that sum there, and whether it settled: its last
// step came to nothing, a minimum, zero or not.
func newtonTurn(g func(float64) []float64, th float64) (float64, float64, bool) {
	const h = 1e-7
	v := g(th)
	s := sumSq(v)
	for range 16 {
		if s <= planTol*planTol*1e-4 {
			return th, s, true
		}
		gp, gm := g(th+h), g(th-h)
		var jj, jg float64
		for i := range v {
			j := (gp[i] - gm[i]) / (2 * h)
			jj += j * j
			jg += j * v[i]
		}
		if jj < 1e-30 {
			return th, s, false
		}
		step := jg / jj
		step = math.Max(-0.5, math.Min(0.5, step))
		if math.Abs(step) < 1e-12 {
			return th, s, true
		}
		nt := th - step
		nv := g(nt)
		ns := sumSq(nv)
		if ns >= s {
			// Halved once, then given up: a minimum that isn't zero.
			nt = th - step/2
			nv = g(nt)
			if ns = sumSq(nv); ns >= s {
				return th, s, math.Abs(step) < 1e-9
			}
		}
		th, v, s = nt, nv, ns
	}
	return th, s, false
}

// sumSq is the sum of the squares of v.
func sumSq(v []float64) float64 {
	s := 0.0
	for _, x := range v {
		s += x * x
	}
	return s
}

// symEigen3 is the eigenvalues of the symmetric matrix a and its
// eigenvectors, the columns of vecs, by Jacobi's rotations.
func symEigen3(a [3][3]float64) (vals [3]float64, vecs [3][3]float64) {
	for i := range 3 {
		vecs[i][i] = 1
	}
	for range 50 {
		off := math.Abs(a[0][1]) + math.Abs(a[0][2]) + math.Abs(a[1][2])
		if off < 1e-300 {
			break
		}
		for p := range 2 {
			for q := p + 1; q < 3; q++ {
				if a[p][q] == 0 {
					continue
				}
				theta := (a[q][q] - a[p][p]) / (2 * a[p][q])
				t := math.Copysign(1, theta) / (math.Abs(theta) + math.Sqrt(theta*theta+1))
				c := 1 / math.Sqrt(t*t+1)
				s := t * c
				for k := range 3 {
					akp, akq := a[k][p], a[k][q]
					a[k][p], a[k][q] = c*akp-s*akq, s*akp+c*akq
				}
				for k := range 3 {
					apk, aqk := a[p][k], a[q][k]
					a[p][k], a[q][k] = c*apk-s*aqk, s*apk+c*aqk
				}
				for k := range 3 {
					vkp, vkq := vecs[k][p], vecs[k][q]
					vecs[k][p], vecs[k][q] = c*vkp-s*vkq, s*vkp+c*vkq
				}
			}
		}
	}
	for i := range 3 {
		vals[i] = a[i][i]
	}
	return vals, vecs
}
