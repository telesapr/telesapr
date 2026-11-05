package sketch

import (
	"math"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/topo"
)

// A profile is drawn across its path, and the sweep's solid passes
// through it where it is drawn. Along an open path the sweep runs from
// one end to the other, the profile moved along by the path's steps or
// frames from the end it starts at; a profile drawn across that end is
// moved as it is. One drawn across the path partway along it is first
// carried back along the path to the start, by the inverse of the
// motion that takes the start's section to where it is drawn, so that
// the whole path is swept and the section there is the profile; one
// drawn across the far end has the path taken from that end. A profile
// the path doesn't cross within its reach, drawn beside the path, is
// moved as it is from the end nearest its plane, as before.

// profileOnPath is where an open chain crosses the profile's plane
// nearest the profile, if the profile is drawn across it there: the
// piece, its curve and the curve's parameter, and the point. ok is false
// where the path doesn't cross the plane within the profile's reach of
// its middle.
func profileOnPath(chain []pathPiece, sbls *srf.SBezierLoopSetSet) (k, ci int, t float64, x geom.Vector, ok bool) {
	n, onPlane := sbls.Sets[0].Normal, sbls.Sets[0].Point
	near := loopsCentre(sbls)
	side := func(p geom.Vector) float64 { return p.Minus(onPlane).Dot(n) }
	best := math.Inf(1)
	for pk, p := range withCurves(chain) {
		for pci := range p.curves {
			planeCrossings(&p.curves[pci], side, func(pt float64, px geom.Vector) {
				if d := px.Minus(near).Magnitude(); d < best {
					best, k, ci, t, x = d, pk, pci, pt, px
				}
			})
		}
		// A piece ending on the plane meets it there, which the steps
		// along its curve may only touch.
		if math.Abs(side(p.end)) <= sweepTol {
			if d := p.end.Minus(near).Magnitude(); d < best {
				best, k, ci, t, x = d, pk, len(p.curves)-1, 1, p.end
			}
		}
		if pk == 0 && math.Abs(side(p.start)) <= sweepTol {
			if d := p.start.Minus(near).Magnitude(); d < best {
				best, k, ci, t, x = d, 0, 0, 0, p.start
			}
		}
	}
	if math.IsInf(best, 1) || best > profileReach(sbls, x) {
		return 0, 0, 0, geom.Vector{}, false
	}
	return k, ci, t, x, true
}

// withCurves is the chain with every piece's curves made: a circle's,
// and pieces a corner radius cut or made, have none yet.
func withCurves(chain []pathPiece) []pathPiece {
	out := append([]pathPiece(nil), chain...)
	for k := range out {
		switch p := &out[k]; {
		case p.curves != nil:
		case p.arc:
			p.curves = arcCurves(p.center, p.axis, p.start, p.angle)
		default:
			p.curves = []srf.SBezier{srf.SBezierFrom2(p.start, p.end)}
		}
	}
	return out
}

// reversedChain is the chain taken from its other end.
func reversedChain(chain []pathPiece) []pathPiece {
	out := make([]pathPiece, len(chain))
	for k, p := range chain {
		out[len(chain)-1-k] = p.reversed()
	}
	return out
}

// startAtProfile is the open chain taken from the end the profile is
// drawn across, and whether the profile is drawn partway along it
// instead, at neither end.
func startAtProfile(chain []pathPiece, sbls *srf.SBezierLoopSetSet) ([]pathPiece, bool) {
	_, _, _, x, ok := profileOnPath(chain, sbls)
	switch {
	case !ok, x.Minus(chain[0].start).Magnitude() <= sweepTol:
		return chain, false
	case x.Minus(chain[len(chain)-1].end).Magnitude() <= sweepTol:
		return reversedChain(chain), false
	}
	return chain, true
}

// rigidTo is the motion the steps of the open chain take the profile by
// from its start to where it crosses the profile's plane, x: every
// piece before it, the corners between, and the piece it is on up to
// it. A pathError where x is at a sharp corner, where the profile would
// be cut by the corner's mitre.
func rigidTo(chain []pathPiece, sbls *srf.SBezierLoopSetSet) (srf.Rigid, error) {
	k, ci, t, x, ok := profileOnPath(chain, sbls)
	if !ok {
		return srf.RigidIdentity, nil
	}
	chain = withCurves(chain)
	corner := func(i int) bool {
		return i > 0 && i < len(chain) && chain[i-1].tangentAt(true).Dot(chain[i].tangentAt(false)) < sweepTangentCos
	}
	if x.Minus(chain[k].end).Magnitude() <= sweepTol {
		// At the piece's end, which is the next one's start.
		k, t = k+1, 0
		if k < len(chain) {
			ci = 0
		}
	}
	if k < len(chain) && x.Minus(chain[k].start).Magnitude() <= sweepTol && corner(k) {
		return srf.RigidIdentity, pathErr(&x, "the profile is at a corner of the path, at (%g, %g, %g): "+
			"draw it across the path where it is smooth, or at an end", x.X, x.Y, x.Z)
	}
	r := srf.RigidIdentity
	for i := range min(k+1, len(chain)) {
		p := chain[i]
		if corner(i) {
			d1, d2 := chain[i-1].tangentAt(true), p.tangentAt(false)
			r = r.Then(srf.SweepStep{Corner: true, Center: p.start, Axis: d1.Cross(d2).WithMagnitude(1),
				Angle: math.Acos(math.Max(-1, math.Min(1, d1.Dot(d2))))}.Motion())
		}
		if i == k {
			if t > 0 {
				a, _ := splitPiece(p, ci, t, x)
				r = r.Then(a.step().Motion())
			}
			break
		}
		r = r.Then(p.step().Motion())
	}
	return r, nil
}

// fittedTo is the motion a fitted path takes the profile by from its
// start to where it crosses the profile's plane nearest the profile.
func fittedTo(sp *topo.SweepPath, sbls *srf.SBezierLoopSetSet) srf.Rigid {
	s, ok := crossingParam(sp.Pieces(), sbls)
	if !ok {
		return srf.RigidIdentity
	}
	return sp.Rigid(s)
}

// runsTo is fittedTo for a sweep in runs joined at corners beside a
// cubic: along the run the crossing is on, from where that run takes
// the profile.
func runsTo(sr *sweepRuns, sbls *srf.SBezierLoopSetSet) srf.Rigid {
	near := loopsCentre(sbls)
	starts := sr.starts()
	best, out := math.Inf(1), srf.RigidIdentity
	for k, run := range sr.runs {
		s, ok := crossingParam(run.Pieces(), sbls)
		if !ok {
			continue
		}
		if d := runPoint(run, s).Minus(near).Magnitude(); d < best {
			best, out = d, starts[k].Then(run.Rigid(s))
		}
	}
	return out
}

// runPoint is a fitted path's point at s.
func runPoint(sp *topo.SweepPath, s float64) geom.Vector {
	pieces := sp.Pieces()
	i := max(0, min(int(math.Floor(s)), len(pieces)-1))
	return pieces[i].PointAt(s - float64(i))
}

// crossingParam is the parameter, on a fitted path made of curves, of
// its crossing of the profile's plane nearest the profile.
func crossingParam(curves []srf.SBezier, sbls *srf.SBezierLoopSetSet) (float64, bool) {
	n, onPlane := sbls.Sets[0].Normal, sbls.Sets[0].Point
	near := loopsCentre(sbls)
	side := func(p geom.Vector) float64 { return p.Minus(onPlane).Dot(n) }
	best, at := math.Inf(1), 0.0
	for i := range curves {
		planeCrossings(&curves[i], side, func(t float64, x geom.Vector) {
			if d := x.Minus(near).Magnitude(); d < best {
				best, at = d, float64(i)+t
			}
		})
	}
	return at, !math.IsInf(best, 1)
}

// inverseRigid undoes r.
func inverseRigid(r srf.Rigid) srf.Rigid {
	q := r.Q.Inverse()
	return srf.Rigid{Q: q, T: q.Rotate(r.T).ScaledBy(-1)}
}

// sweepProfile is the profile's loops as the sweep moves them from its
// path's start: where it is drawn, or, drawn partway along an open
// path, carried back along it to the start.
func (g *Group) sweepProfile(src *Group) *srf.SBezierLoopSetSet {
	if g.sweepFrom == nil {
		return &src.BezierLoops
	}
	back := inverseRigid(*g.sweepFrom)
	out := &srf.SBezierLoopSetSet{}
	for i := range src.BezierLoops.Sets {
		out.Sets = append(out.Sets, *srf.TransformedLoopSet(&src.BezierLoops.Sets[i], back))
	}
	return out
}

// SweepProfile is the profile's loops as the sweep moves them along the
// path from its start, by SweepSteps or FittedSweepPath: where the
// profile is drawn, or, drawn across an open path partway along it,
// carried back along the path to its start.
func (g *Group) SweepProfile(sk *Sketch) (*srf.SBezierLoopSetSet, error) {
	if _, err := g.sweepPath(sk); err != nil {
		return nil, err
	}
	return g.sweepProfile(sk.Group(g.OpA)), nil
}

// sweepStartMotion is the motion that takes the profile where it is
// drawn to the path's start: identity unless it is drawn partway.
func (g *Group) sweepStartMotion() srf.Rigid {
	if g.sweepFrom == nil {
		return srf.RigidIdentity
	}
	return inverseRigid(*g.sweepFrom)
}
