package sketch

import (
	"cmp"
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
// all joined (joinFlush). Each run ends at its corners, where its
// section lies flat on the fill's and on the next run's. A round
// profile's two runs touch tangent there, where the profile crosses the
// corner's axis: the Booleans are told those points
// (sweep_cubic_corner_touch.go). A twist, frames held to a direction
// and a closed path's frames turned to meet round it are made to agree
// at each corner (sweep_cubic_corner_frames.go); a corner radius rounds
// the path's other corners, between lines and arcs, and leaves this one
// sharp. Where the joins fail, the corner stays refused.

// ErrSweepJoined is FittedSweepPath's error for a sweep made of runs
// joined at corners beside a cubic, which no one path describes.
var ErrSweepJoined = errors.New("the sweep is runs joined at corners beside a cubic")

// sweepRuns is a sweep's path cut at its corners beside a cubic: the
// runs, each a fitted path from a corner or the path's start to the next
// corner or the path's end, and the turn at each corner between run k
// and run k+1. A twisted path is cut beside each corner too, where the
// twist is held still (sweep_cubic_corner_frames.go): there turn k is
// no corner but a smooth cut, which the profile passes unturned.
type sweepRuns struct {
	runs  []*topo.SweepPath
	turns []srf.SweepStep
	// smooth marks the turns that are cuts where the path goes on
	// tangent, no corner.
	smooth []bool
	// into and out are the path's unit tangents into and out of each
	// turn.
	into, out []geom.Vector
	// chains are the runs' pieces.
	chains [][]pathPiece
	// twists are the turn, in radians, each run's frames are twisted by
	// along it; nil for none.
	twists []float64
	// up is the direction the frames are held to, zero for none.
	up geom.Vector
	// reach is how far the profile reaches from the path.
	reach float64
	// closed is a path that ends where it starts, its last run meeting
	// its first where the profile is.
	closed bool
	// whole is the path's chain, kinds its cuts at the corners beside a
	// cubic (kinds[i] before piece i), and twist the twist, in radians,
	// spread over it with the frames' turn round a closed path taken out,
	// held still beside the corners over hold times holdLength.
	whole []pathPiece
	kinds []int
	twist float64
	hold  float64
}

// pre says whether run k starts at a turn, and post whether it ends at
// one; corner whether turn k is a corner.
func (sr *sweepRuns) pre(k int) bool    { return k > 0 }
func (sr *sweepRuns) post(k int) bool   { return k < len(sr.turns) }
func (sr *sweepRuns) corner(k int) bool { return !sr.smooth[k] }

// firstCorner is the index of the first turn that is a corner.
func (sr *sweepRuns) firstCorner() int { return max(0, slices.Index(sr.smooth, false)) }

// starts are where each run takes the profile from at its corner, as
// the motion from where it is drawn, and the last the sweep's end.
func (sr *sweepRuns) starts() []srf.Rigid {
	out := []srf.Rigid{srf.RigidIdentity}
	for k, run := range sr.runs {
		at := out[k].Then(run.Rigid(run.End()))
		if sr.post(k) && sr.corner(k) {
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

// pieces are the curves of every run, in order.
func (sr *sweepRuns) pieces() []srf.SBezier {
	var out []srf.SBezier
	for _, run := range sr.runs {
		out = append(out, run.Pieces()...)
	}
	return out
}

// rigidSteps are run k's steps where it is lines and arcs meeting
// tangent, which move the profile rigidly and make exact surfaces, and
// nil where it is fitted: along a cubic, turning at a corner of its
// own beside an arc, twisted, or with frames held to a direction the rigid motion
// doesn't keep (an arc about another axis).
func (sr *sweepRuns) rigidSteps(k int) []srf.SweepStep {
	if sr.twists != nil && sr.twists[k] != 0 {
		return nil
	}
	chain := sr.chains[k]
	var steps []srf.SweepStep
	for i, p := range chain {
		if p.free || p.arc && !heldAlong(sr.up, p.axis) {
			return nil
		}
		if i > 0 {
			d1, d2 := chain[i-1].tangentAt(true), p.tangentAt(false)
			if d1.Dot(d2) < sweepTangentCos {
				// A corner between two lines is mitred as a rigid
				// sweep's; one beside an arc needs its joint, which the
				// fitted sweep makes.
				if p.arc || chain[i-1].arc || !heldAgree(sr.up, d1, d2) {
					return nil
				}
				// A line its mitres could cross on, which the fitted
				// sweep measures and refuses.
				if sr.mitreReach(chain, i-1) >= chain[i-1].end.Minus(chain[i-1].start).Magnitude() ||
					sr.mitreReach(chain, i) >= p.end.Minus(p.start).Magnitude() {
					return nil
				}
				steps = append(steps, srf.SweepStep{Corner: true, Center: p.start,
					Axis: d1.Cross(d2).WithMagnitude(1), Angle: math.Acos(math.Max(-1, math.Min(1, d1.Dot(d2))))})
			}
		}
		steps = append(steps, p.step())
	}
	return steps
}

// mitreReach is how far along line i of a run its mitres reach, at
// most: a profile point within reach carried onto the plane halving a
// corner travels at most reach times the tangent of half its turn.
func (sr *sweepRuns) mitreReach(chain []pathPiece, i int) float64 {
	d := 0.0
	half := func(a, b pathPiece) float64 {
		c := a.tangentAt(true).Dot(b.tangentAt(false))
		if c >= sweepTangentCos {
			return 0
		}
		return sr.reach * math.Tan(math.Min(math.Acos(math.Max(-1, c)), math.Pi*0.999)/2)
	}
	if i > 0 {
		d += half(chain[i-1], chain[i])
	}
	if i+1 < len(chain) {
		d += half(chain[i], chain[i+1])
	}
	return d
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

// Kinds of a cut between two pieces of a chain cut into runs.
const (
	cutNone = iota
	cutCorner
	cutSmooth
)

// fitRuns cuts the chain at its corners beside a cubic (at) into runs,
// and fits each, for a profile reaching reach from the path. A corner
// radius rounds the path's other corners first, those between lines and
// arcs (roundCorners leaves the corners beside a cubic sharp). The runs'
// frames are then made to meet at each corner and, round a closed path,
// where it closes (sweep_cubic_corner_frames.go). A pathError where a
// corner can't be taken so.
func (sk *Sketch) fitRuns(g *Group, chain []pathPiece, at []int, closed bool, reach float64) (*sweepRuns, error) {
	if g.ValB > 0 && !closed {
		var err error
		if chain, err = sk.roundCorners(chain, g.ValB); err != nil {
			return nil, err
		}
		at = cubicCorners(chain)
	}
	chain = withCurves(chain)
	kinds := make([]int, len(chain)+1)
	for _, k := range at {
		kinds[k] = cutCorner
	}
	sr := &sweepRuns{reach: reach, closed: closed, up: g.SweepUp, whole: chain, kinds: kinds}
	sr.cut(chain, kinds)
	if err := sr.heldTurns(); err != nil {
		return nil, err
	}
	if err := sr.fit(); err != nil {
		return nil, err
	}
	twist := g.ValC * math.Pi / 180
	if closed {
		var err error
		if twist, err = sr.closingTwist(twist); err != nil {
			return nil, err
		}
	}
	if twist != 0 {
		sr.twist = twist
		var first error
		for _, hold := range holdScales {
			c, err := sr.heldBy(hold)
			if err == nil {
				sr, first = c, nil
				break
			}
			first = cmp.Or(first, err)
		}
		if first != nil {
			return nil, first
		}
	}
	if closed {
		if err := sr.closes(); err != nil {
			return nil, err
		}
	}
	return sr, nil
}

// cut cuts the chain into runs where kinds says, kinds[i] the cut
// before piece i, and makes each cut's turn.
func (sr *sweepRuns) cut(chain []pathPiece, kinds []int) {
	sr.turns, sr.smooth, sr.into, sr.out, sr.chains, sr.runs, sr.twists = nil, nil, nil, nil, nil, nil, nil
	from := 0
	for k := 1; k <= len(chain); k++ {
		if k < len(chain) && kinds[k] == cutNone {
			continue
		}
		sr.chains = append(sr.chains, chain[from:k])
		from = k
		if k == len(chain) {
			break
		}
		d1, d2 := chain[k-1].tangentAt(true), chain[k].tangentAt(false)
		turn := srf.SweepStep{Center: chain[k].start}
		if kinds[k] == cutCorner {
			turn = srf.SweepStep{Corner: true, Center: chain[k].start,
				Axis: d1.Cross(d2).WithMagnitude(1), Angle: math.Acos(math.Max(-1, math.Min(1, d1.Dot(d2))))}
		}
		sr.turns = append(sr.turns, turn)
		sr.smooth = append(sr.smooth, kinds[k] == cutSmooth)
		sr.into, sr.out = append(sr.into, d1), append(sr.out, d2)
	}
}

// fit fits each run's path, twisted by its share (twists).
func (sr *sweepRuns) fit() error {
	sr.runs = nil
	for k, run := range sr.chains {
		tw := 0.0
		if sr.twists != nil {
			tw = sr.twists[k]
		}
		sp, err := runPathOf(run, sr.up, sr.reach, tw)
		if err != nil {
			if pe := fittedCornerError(err); pe != nil {
				return pe
			}
			return pathErr(&run[0].start, "%v", err)
		}
		sr.runs = append(sr.runs, sp)
	}
	return nil
}

// cubicCornerCarry is how far each side of a corner is carried past it
// for its fill: past where the mitre of a profile reaching reach at a
// turn of angle reaches, and as far again.
func cubicCornerCarry(reach, angle float64) float64 {
	return 2 * reach * (1 + math.Tan(math.Min(angle, math.Pi*0.999)/2))
}

// atCorner is where run k takes the profile at its end, the corner of
// turn k, as the motion from where it is drawn.
func (sr *sweepRuns) atCorner(starts []srf.Rigid, k int) srf.Rigid {
	return starts[k].Then(sr.runs[k].Rigid(sr.runs[k].End()))
}

// cubicCornerFill is the solid filling the outside of turn k: the
// profile where run k reaches the corner carried on along the tangent
// there, overlapping the profile where run k+1 leaves the corner carried
// back along its tangent, each by the fraction f of their carry.
func (sr *sweepRuns) cubicCornerFill(set *srf.SBezierLoopSet, starts []srf.Rigid, k int, f float64, color core.RGBA, tol srf.Tolerance) (*srf.SShell, error) {
	l := f * cubicCornerCarry(sr.reach, sr.turns[k].Angle)
	t1, t2 := sr.into[k], sr.out[k]
	move := func(v geom.Vector) srf.Rigid { return srf.Rigid{Q: geom.QuaternionIdentity, T: v} }
	on := srf.SShell{Tol: tol}
	on.MakeFromSweepOf(srf.TransformedLoopSet(set, sr.atCorner(starts, k)),
		[]srf.SweepStep{{Along: t1.ScaledBy(l)}}, color, 0, 0)
	back := srf.SShell{Tol: tol}
	back.MakeFromSweepOf(srf.TransformedLoopSet(set, starts[k+1].Then(move(t2.ScaledBy(-l)))),
		[]srf.SweepStep{{Along: t2.ScaledBy(l)}}, color, 0, 0)
	fill := srf.SShell{Tol: tol}
	fill.MakeFromIntersectionOf(&on, &back)
	if fill.BooleanFailed || len(fill.Surface) == 0 {
		return nil, errors.New("the corner's fill failed")
	}
	return &fill, nil
}

// sweepRunsJoined is the solid of the profile swept along the runs, the
// caps' identities start and end and the patches' from origin on: each
// run swept, each corner's fill, all joined, told where the runs touch
// tangent (cornerTouches). A *srf.CornerError at a corner whose fill can't be made or
// grows with how far its sides are carried; a nil shell where a join
// fails.
func (sr *sweepRuns) sweepRunsJoined(sbls *srf.SBezierLoopSetSet, color core.RGBA, start, end, origin uint32, tol srf.Tolerance) (*srf.SShell, error) {
	starts := sr.starts()
	if err := sr.clearOfArcs(sbls, starts); err != nil {
		return nil, err
	}
	// The fills, the fitted runs and the exact ones, joined in that
	// order first, the other orders after it: where one order leaves
	// edges open, another may close.
	var fills, fitted, exact []*srf.SShell
	var touch []geom.Vector
	next := origin
	for i := range sbls.Sets {
		set := &sbls.Sets[i]
		for k, run := range sr.runs {
			moved := srf.TransformedLoopSet(set, starts[k])
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
			if !sr.post(k) || !sr.corner(k) {
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
			touch = append(touch, sr.cornerTouches(set, sr.atCorner(starts, k), k, tol)...)
		}
		touch = append(touch, sr.crossingTouches(set, starts, tol)...)
	}
	for _, sh := range slices.Concat(fills, fitted, exact) {
		sh.Touch = touch
	}
	for _, order := range [][3][]*srf.SShell{
		{fills, fitted, exact}, {fitted, fills, exact}, {fills, exact, fitted},
		{exact, fills, fitted}, {fitted, exact, fills}, {exact, fitted, fills},
	} {
		if sh := joinFlush(slices.Concat(order[0], order[1], order[2]), tol); sh != nil && topologySound(sh) {
			// The touches are the runs' own: a later Boolean of the
			// sweep with another solid must not end its curves there.
			sh.Touch = nil
			return sh, nil
		}
	}
	return nil, nil
}

// generateRunsSweepShell makes the sweep along its runs joined at
// corners beside a cubic (sweepRunsJoined): a problem on the group at
// the corner where it can't be made.
func (g *Group) generateRunsSweepShell(sk *Sketch, src *Group, start, end uint32) {
	prof := g.sweepProfile(src)
	sr := g.sweepRuns
	var sh *srf.SShell
	var err error
	// A twist held still further beside the corners, where the runs
	// held as far as a mitre would reach don't join.
	for _, c := range sr.holdsToTry() {
		for _, run := range c.runs {
			defer sk.lendPathPace(run)()
		}
		var e error
		if sh, e = c.sweepRunsJoined(prof, g.Color, start, end, srf.OriginFor(g.H.V, 0), sk.tol); sh != nil {
			sr, err = c, nil
			break
		}
		err = cmp.Or(err, e)
	}
	if err == nil && sh == nil {
		err = &srf.CornerError{At: sr.turns[sr.firstCorner()].Center, Err: srf.ErrCornerNoJoint}
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
		if pe, ok := errors.AsType[*pathError](err); ok {
			g.setProblem(pe.at, "%s", pe.msg)
			return
		}
		g.setProblem(nil, "%v", err)
		return
	}
	g.ThisShell = *sh
	g.ThisShell.Tol = sk.tol
	g.refuseCrossing(sk, func() bool { return curvesComeBack(prof, sr.pieces()) }, nil)
}

// topologySound reports whether the shell's topology is a closed solid
// whose holes its mesh counts the same, and whether the solid the group
// makes of it (united with nothing, as joinFlush checks it) has a mesh
// no edge passes through, counting faces lying flat on each other as
// Check Solid does: joinFlush checks the mesh's edges, but a join can
// leave the faces' loops wrong where the mesh closes, and only faces
// crossing where it looks. A run's end cap lying inside a tube the path
// comes back through (a cubic bent back across the line after its
// corner) was kept overlapping itself so, in a solid joinFlush passed.
// It looks at copies, so that the caches meshing and building the
// topology fill don't move the last bits of the solid's own mesh.
func topologySound(orig *srf.SShell) bool {
	c := orig.Clone()
	var m srf.SMesh
	c.TriangulateInto(&m)
	t, err := topo.Build(&c)
	if err != nil || len(t.Validate()) != 0 || t.Euler() != topo.MeshEuler(&m) {
		return false
	}
	c = orig.Clone()
	final := srf.SShell{Tol: orig.Tol}
	final.MakeFromUnionOf(&srf.SShell{Tol: orig.Tol}, &c)
	final.MergeCoincidentSurfaces()
	var fm srf.SMesh
	final.TriangulateInto(&fm)
	inter := false
	srf.SKdNodeFrom(&fm).MakeCertainEdgesInto(&srf.SEdgeList{}, srf.EdgeSelfInter, true, &inter, nil, 0)
	return !inter
}
