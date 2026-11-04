package sketch

import (
	"errors"
	"fmt"
	"math"
	"slices"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/topo"
)

// A sharp corner beside a cubic is no corner the kernel's fitted sweep
// mitres (topo.ErrPathCornerCurved): a cubic's skin would have to run on
// past the corner and be cut there, which its patches, untrimmed, can't
// be. The solid is still defined as a corner beside an arc too sharp to
// join is (sweep_corner_fill.go): each side swept up to the corner, and
// on the outside of the turn the overlap of the two sides carried on
// past it along their tangents there, which is the mitre two lines
// tangent to the pieces would make. Made of Booleans: the path cut at
// such corners into runs, each swept on its own (exactly where it is
// lines and arcs meeting tangent, else fitted, its own corners mitred or
// joined by the kernel as before), the profile turned at each cut corner
// about the corner's axis as a mitre turns it, and the corners' fills,
// all joined (joinFlush). Each run is carried a little past its corners
// along its tangent, and the fill stops short of that lead, so that the
// fill meets a run on a section of the lead, flat where the fill is,
// rather than along a cubic's skin, which the fill's faces would touch
// tangent along an edge. An open path with no twist, no frames held to a
// direction and no corner radius only: a twist or a held frame turns the
// sections otherwise than the corner does, and a corner radius rounds
// corners between lines and arcs, which a cubic's is not. Where the
// joins fail, the corner stays refused.

// ErrSweepJoined is FittedSweepPath's error for a sweep made of runs
// joined at corners beside a cubic, which no one path describes.
var ErrSweepJoined = errors.New("the sweep is runs joined at corners beside a cubic")

// cubicCornerLead is how far, as a fraction of the profile's reach,
// each run is carried along a line past a corner beside a cubic.
const cubicCornerLead = 0.05

// sweepRuns is a sweep's path cut at its corners beside a cubic: the
// runs, each a fitted path carried on along a tangent line of length
// lead[k] past a corner at its start (pre) and its end (post), and the
// turn at each corner between run k and run k+1.
type sweepRuns struct {
	runs  []*topo.SweepPath
	turns []srf.SweepStep
	// into and out are the path's unit tangents into and out of each
	// turn.
	into, out []geom.Vector
	// lead is how far each turn's sides are carried past it in the runs.
	lead []float64
	// chains are the runs' pieces, the leads in.
	chains [][]pathPiece
	// reach is how far the profile reaches from the path.
	reach float64
}

// pre says whether run k starts with a lead before a corner, and post
// whether it ends with one past a corner.
func (sr *sweepRuns) pre(k int) bool  { return k > 0 }
func (sr *sweepRuns) post(k int) bool { return k < len(sr.turns) }

// cornerEnd is how far along run k its own path ends, before the lead
// past the corner after it.
func (sr *sweepRuns) cornerEnd(k int) float64 {
	e := sr.runs[k].End()
	if sr.post(k) {
		e--
	}
	return e
}

// sweepStart is where the profile is drawn for run k's sweep, as the
// motion from where it is drawn for the whole: at the corner before it,
// moved back along the lead.
func (sr *sweepRuns) sweepStart(starts []srf.Rigid, k int) srf.Rigid {
	if !sr.pre(k) {
		return starts[k]
	}
	return starts[k].Then(srf.Rigid{Q: geom.QuaternionIdentity, T: sr.out[k-1].ScaledBy(-sr.lead[k-1])})
}

// starts are where each run takes the profile from at its corner, as
// the motion from where it is drawn, and the last the sweep's end.
func (sr *sweepRuns) starts() []srf.Rigid {
	out := []srf.Rigid{srf.RigidIdentity}
	for k, run := range sr.runs {
		at := sr.sweepStart(out, k).Then(run.Rigid(sr.cornerEnd(k)))
		if sr.post(k) {
			at = at.Then(sr.turns[k].Motion())
		}
		out = append(out, at)
	}
	return out
}

// end is where the sweep takes the profile.
func (sr *sweepRuns) end() srf.Rigid {
	s := sr.starts()
	return s[len(s)-1]
}

// pieces are the curves of every run, in order, the leads left out.
func (sr *sweepRuns) pieces() []srf.SBezier {
	var out []srf.SBezier
	for k, run := range sr.runs {
		p := run.Pieces()
		if sr.post(k) {
			p = p[:len(p)-1]
		}
		if sr.pre(k) {
			p = p[1:]
		}
		out = append(out, p...)
	}
	return out
}

// rigidSteps are run k's steps where it is lines and arcs meeting
// tangent, which move the profile rigidly and make exact surfaces, and
// nil where it is fitted: along a cubic, or turning at a corner of its
// own.
func (sr *sweepRuns) rigidSteps(k int) []srf.SweepStep {
	chain := sr.chains[k]
	var steps []srf.SweepStep
	for i, p := range chain {
		if p.free {
			return nil
		}
		if i > 0 && chain[i-1].tangentAt(true).Dot(p.tangentAt(false)) < sweepTangentCos {
			return nil
		}
		steps = append(steps, p.step())
	}
	return steps
}

// cubicCorners are the indices k of the chain where it turns sharply
// at a cubic, between chain[k-1] and chain[k].
func cubicCorners(chain []pathPiece) []int {
	var out []int
	for k := 1; k < len(chain); k++ {
		if !chain[k-1].free && !chain[k].free {
			continue
		}
		if chain[k-1].tangentAt(true).Dot(chain[k].tangentAt(false)) < sweepTangentCos {
			out = append(out, k)
		}
	}
	return out
}

// fitRuns cuts the chain at its corners beside a cubic (at) into runs,
// each carried on a little past its corners, and fits each, for a
// profile reaching reach from the path. A pathError where a corner can't
// be taken so.
func (sk *Sketch) fitRuns(g *Group, chain []pathPiece, at []int, closed bool, reach float64) (*sweepRuns, error) {
	x := chain[at[0]].start
	why := ""
	switch {
	case closed:
		why = "the path closes, and a closed sweep's frames are turned to meet round it"
	case g.ValC != 0:
		why = "the profile is twisted along the path"
	case g.SweepUp.Magnitude() > 0:
		why = "the frames are held to a direction"
	case g.ValB > 0:
		why = "a corner radius rounds corners between line segments and arcs only; leave it unset to keep this one sharp"
	}
	if why != "" {
		return nil, pathErr(&x, "the path has a sharp corner at (%g, %g, %g) next to a cubic, which is swept as a mitre "+
			"only on an open path with no twist, no frames held and no corner radius: %s", x.X, x.Y, x.Z, why)
	}
	sr := &sweepRuns{reach: reach}
	for _, k := range at {
		d1, d2 := chain[k-1].tangentAt(true), chain[k].tangentAt(false)
		sr.turns = append(sr.turns, srf.SweepStep{Corner: true, Center: chain[k].start,
			Axis: d1.Cross(d2).WithMagnitude(1), Angle: math.Acos(math.Max(-1, math.Min(1, d1.Dot(d2))))})
		sr.into, sr.out = append(sr.into, d1), append(sr.out, d2)
		sr.lead = append(sr.lead, cubicCornerLead*reach)
	}
	from := 0
	for n, k := range append(append([]int{}, at...), len(chain)) {
		run := append([]pathPiece(nil), chain[from:k]...)
		for i := range run {
			if run[i].curves != nil {
				continue
			}
			if run[i].arc {
				run[i].curves = arcCurves(run[i].center, run[i].axis, run[i].start, run[i].angle)
			} else {
				run[i].curves = []srf.SBezier{srf.SBezierFrom2(run[i].start, run[i].end)}
			}
		}
		if n > 0 {
			c, d := sr.turns[n-1].Center, sr.out[n-1]
			a := c.Minus(d.ScaledBy(sr.lead[n-1]))
			run = append([]pathPiece{{start: a, end: c, curves: []srf.SBezier{srf.SBezierFrom2(a, c)}}}, run...)
		}
		if n < len(at) {
			c, d := sr.turns[n].Center, sr.into[n]
			b := c.Plus(d.ScaledBy(sr.lead[n]))
			run = append(run, pathPiece{start: c, end: b, curves: []srf.SBezier{srf.SBezierFrom2(c, b)}})
		}
		sp, err := sweepPathOf(run, g, reach)
		if err != nil {
			if pe := fittedCornerError(err); pe != nil {
				return nil, pe
			}
			return nil, pathErr(&run[0].start, "%v", err)
		}
		sr.runs = append(sr.runs, sp)
		sr.chains = append(sr.chains, run)
		from = k
	}
	return sr, nil
}

// cubicCornerCarry is how far each side of a corner is carried past it
// for its fill: past where the mitre of a profile reaching reach at a
// turn of angle reaches, and as far again.
func cubicCornerCarry(reach, angle float64) float64 {
	return 2 * reach * (1 + math.Tan(math.Min(angle, math.Pi*0.999)/2))
}

// cubicCornerFill is the solid filling the outside of turn k, short of
// the leads: the profile where run k reaches the corner carried on
// along the tangent there, from the end of its lead, overlapping the
// profile where run k+1 leaves the corner carried back along its
// tangent, up to the start of its lead, each by the fraction f of their
// carry.
func (sr *sweepRuns) cubicCornerFill(set *srf.SBezierLoopSet, starts []srf.Rigid, k int, f float64, color core.RGBA, tol srf.Tolerance) (*srf.SShell, error) {
	l := f * cubicCornerCarry(sr.reach, sr.turns[k].Angle)
	lead := sr.lead[k]
	t1, t2 := sr.into[k], sr.out[k]
	move := func(v geom.Vector) srf.Rigid { return srf.Rigid{Q: geom.QuaternionIdentity, T: v} }
	endA := sr.sweepStart(starts, k).Then(sr.runs[k].Rigid(sr.cornerEnd(k)))
	on := srf.SShell{Tol: tol}
	on.MakeFromSweepOf(srf.TransformedLoopSet(set, endA.Then(move(t1.ScaledBy(lead)))),
		[]srf.SweepStep{{Along: t1.ScaledBy(l - lead)}}, color, 0, 0)
	back := srf.SShell{Tol: tol}
	back.MakeFromSweepOf(srf.TransformedLoopSet(set, starts[k+1].Then(move(t2.ScaledBy(-l)))),
		[]srf.SweepStep{{Along: t2.ScaledBy(l - lead)}}, color, 0, 0)
	fill := srf.SShell{Tol: tol}
	fill.MakeFromIntersectionOf(&on, &back)
	if fill.BooleanFailed || len(fill.Surface) == 0 {
		return nil, errors.New("the corner's fill failed")
	}
	return &fill, nil
}

// sweepRunsJoined is the solid of the profile swept along the runs, the
// caps' identities start and end and the patches' from origin on: each
// run swept, carried a little past its corners, each corner's fill, all
// joined. A *srf.CornerError at a corner whose fill can't be made or
// grows with how far its sides are carried; a nil shell where a join
// fails.
func (sr *sweepRuns) sweepRunsJoined(sbls *srf.SBezierLoopSetSet, color core.RGBA, start, end, origin uint32, tol srf.Tolerance) (*srf.SShell, error) {
	starts := sr.starts()
	// The fills, the fitted runs and the exact ones, joined in that
	// order first: a fill joined to the cubic's run, then the line's,
	// closes where other orders leave edges open along the leads. The
	// other orders are tried after it.
	var fills, fitted, exact []*srf.SShell
	next := origin
	for i := range sbls.Sets {
		set := &sbls.Sets[i]
		for k, run := range sr.runs {
			moved := srf.TransformedLoopSet(set, sr.sweepStart(starts, k))
			s, e := uint32(0), uint32(0)
			if !sr.pre(k) {
				s = start
			}
			if !sr.post(k) {
				e = end
			}
			var sh *srf.SShell
			if steps := sr.rigidSteps(k); steps != nil {
				sh = &srf.SShell{Tol: tol}
				sh.MakeFromSweepOf(moved, steps, color, srf.FaceID(s), srf.FaceID(e))
				exact = append(exact, sh)
			} else {
				loops := [][]srf.SBezier{moved.Outer.Curves}
				for _, h := range moved.Holes {
					loops = append(loops, h.Curves)
				}
				var err error
				if sh, err = topo.MakePathSweep(run, loops, color, s, e, next, tol); err != nil {
					return nil, err
				}
				next += uint32(len(sh.Surface))
				sh.FlattenPlanarPatches()
				fitted = append(fitted, sh)
			}
			if !sr.post(k) {
				continue
			}
			corner := &srf.CornerError{At: sr.turns[k].Center, Err: srf.ErrCornerNoJoint}
			fill, err := sr.cubicCornerFill(set, starts, k, 1, color, tol)
			if err != nil {
				return nil, corner
			}
			half, err := sr.cubicCornerFill(set, starts, k, 0.5, color, tol)
			if err != nil {
				return nil, corner
			}
			if v, vh := meshVolume(fill), meshVolume(half); math.Abs(v-vh) > 1e-3*v+tol.ChordTolMm() {
				return nil, &srf.CornerError{At: sr.turns[k].Center, Err: errCornerFillOpen}
			}
			fills = append(fills, fill)
		}
	}
	for _, order := range [][3][]*srf.SShell{
		{fills, fitted, exact}, {fitted, fills, exact}, {fills, exact, fitted},
		{exact, fills, fitted}, {fitted, exact, fills}, {exact, fitted, fills},
	} {
		if sh := joinFlush(slices.Concat(order[0], order[1], order[2]), tol); sh != nil && topologySound(sh) {
			return sh, nil
		}
	}
	return nil, nil
}

// generateRunsSweepShell makes the sweep along its runs joined at
// corners beside a cubic (sweepRunsJoined): a problem on the group at
// the corner where it can't be made.
func (g *Group) generateRunsSweepShell(sk *Sketch, src *Group, start, end uint32) {
	sr := g.sweepRuns
	sh, err := sr.sweepRunsJoined(&src.BezierLoops, g.Color, start, end, srf.OriginFor(g.H.V, 0), sk.tol)
	if err == nil && sh == nil {
		err = &srf.CornerError{At: sr.turns[0].Center, Err: srf.ErrCornerNoJoint}
	}
	if err != nil {
		if ce, ok := errors.AsType[*srf.CornerError](err); ok {
			x := ce.At
			why := "the pieces either side, swept and joined, don't make a closed solid there"
			if errors.Is(err, errCornerFillOpen) {
				why = errCornerFillOpen.Error()
			}
			g.setProblem(&x, "%s", fmt.Sprintf("the path's sharp corner at (%g, %g, %g) next to a cubic can't be "+
				"joined: %s; round it in the path with an arc or a tangent cubic", x.X, x.Y, x.Z, why))
			return
		}
		g.setProblem(nil, "%v", err)
		return
	}
	g.ThisShell = *sh
	g.ThisShell.Tol = sk.tol
	g.refuseCrossing(sk, func() bool { return curvesComeBack(&src.BezierLoops, sr.pieces()) }, nil)
}

// topologySound reports whether the shell's topology is a closed solid
// whose holes its mesh counts the same: joinFlush checks the mesh's
// edges, but a join can leave the faces' loops wrong where the mesh
// closes. It looks at a copy, so that the caches meshing and building
// the topology fill don't move the last bits of the solid's own mesh.
func topologySound(orig *srf.SShell) bool {
	c := orig.Clone()
	var m srf.SMesh
	c.TriangulateInto(&m)
	t, err := topo.Build(&c)
	return err == nil && len(t.Validate()) == 0 && t.Euler() == topo.MeshEuler(&m)
}
