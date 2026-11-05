package sketch

import (
	"errors"
	"math"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/srf"
)

// A sharp corner beside an arc, of 150° say, has points of the profile
// whose paths along the two pieces don't cross: the line's wall, offset
// from the path, passes outside the arc's wall the same point turns
// along. The kernel joins a corner only through those crossings
// (srf.CheckSweepCorners), so it can't make it. The solid is still the
// same one: each piece swept up to the corner, and at the corner, on the
// outside of the turn, where the two pieces carried on past it overlap.
// Made of Booleans: the path cut at the corner into two runs each swept
// on its own, and the fill, the intersection of the profile carried on
// past the corner along the first piece and carried back before it
// along the second, joined to them. Round a closed path, started where
// it is smooth, the first run's start and the last's end meet flush
// where the profile is, and the union closes them, as stretches do
// (sweep_cross.go). Where the overlap runs on as far as the pieces are
// carried, the sides never meet and no joint closes the corner: that
// stays refused, with a corner radius to round it.

// cornerFillCarry is how far a piece is carried on past a corner for
// its fill: a quarter turn along an arc, as far as one surface of a turn
// holds, and along a line twice its length.
const cornerFillCarry = math.Pi / 2

// errCornerFillOpen is a corner whose fill grows with how far the
// pieces are carried: their outer sides never meet.
var errCornerFillOpen = errors.New("the pieces' outer sides never meet past it")

// cornerUnjoined says whether the corner at steps[k] beside an arc is
// one the kernel can't join because its pieces' sides don't meet, the
// profile set taken where the steps start.
func cornerUnjoined(set *srf.SBezierLoopSet, steps []srf.SweepStep, k int) bool {
	if k == 0 || k+1 >= len(steps) || !steps[k].Corner || !(steps[k-1].Arc || steps[k+1].Arc) {
		return false
	}
	at := srf.TransformedLoopSet(set, srf.SweepEnd(steps[:k-1]))
	return errors.Is(srf.CheckSweepCorners(at, steps[k-1:k+2], false), srf.ErrCornerNoJoint)
}

// unjoinedCorners are the corners of steps no set of the profile can
// be joined at by the kernel, the pieces' sides not meeting.
func unjoinedCorners(sbls *srf.SBezierLoopSetSet, steps []srf.SweepStep) []int {
	var out []int
	for k := range steps {
		for i := range sbls.Sets {
			if cornerUnjoined(&sbls.Sets[i], steps, k) {
				out = append(out, k)
				break
			}
		}
	}
	return out
}

// carried is the step piece st continued by the fraction f of its
// carry: an arc's turn on by a quarter turn, a line on by twice its
// length.
func carried(st srf.SweepStep, f float64) srf.SweepStep {
	if st.Arc {
		st.Angle = math.Copysign(cornerFillCarry*f, st.Angle)
		return st
	}
	st.Along = st.Along.ScaledBy(2 * f)
	return st
}

// backward is the motion that undoes st.
func backward(st srf.SweepStep) srf.Rigid {
	if st.Arc {
		st.Angle = -st.Angle
		return st.Motion()
	}
	st.Along = st.Along.ScaledBy(-1)
	return st.Motion()
}

// cornerFill is the solid filling the outside of the corner steps[k]
// for the profile set, taken where the steps start: the overlap of the
// piece before carried on past the corner and the piece after carried
// back before it, by the fraction f of their carry. Both hold the
// profile at the corner, so they always overlap: an empty overlap is a
// Boolean that failed.
func cornerFill(set *srf.SBezierLoopSet, steps []srf.SweepStep, k int, f float64, color core.RGBA, tol srf.Tolerance) (*srf.SShell, error) {
	a, b := carried(steps[k-1], f), carried(steps[k+1], f)
	endA := srf.SweepEnd(steps[:k])
	on := srf.SShell{Tol: tol}
	on.MakeFromSweepOf(srf.TransformedLoopSet(set, endA), []srf.SweepStep{a}, color, 0, 0)
	startB := srf.SweepEnd(steps[:k+1])
	back := srf.SShell{Tol: tol}
	back.MakeFromSweepOf(srf.TransformedLoopSet(set, startB.Then(backward(b))), []srf.SweepStep{b}, color, 0, 0)
	fill := srf.SShell{Tol: tol}
	fill.MakeFromIntersectionOf(&on, &back)
	if fill.BooleanFailed || len(fill.Surface) == 0 {
		return nil, errors.New("the corner's fill failed")
	}
	return &fill, nil
}

// meshVolume is the volume the shell's mesh encloses.
func meshVolume(sh *srf.SShell) float64 {
	var m srf.SMesh
	sh.TriangulateInto(&m)
	return m.CalculateVolume()
}

// sweepCornersFilled is the solid of a profile swept along steps whose
// corners at the indices unjoined the kernel can't join: the runs
// between them each swept, and each such corner's fill, joined. A
// *srf.CornerError at the corner where the fill can't be made, or
// where the pieces' outer sides never meet; nil error with a nil shell
// where the joins fail.
func sweepCornersFilled(sbls *srf.SBezierLoopSetSet, steps []srf.SweepStep, unjoined []int, color core.RGBA, start, end srf.FaceID, tol srf.Tolerance) (*srf.SShell, error) {
	var parts []*srf.SShell
	for i := range sbls.Sets {
		set := &sbls.Sets[i]
		from := 0
		for n, k := range append(append([]int{}, unjoined...), len(steps)) {
			run := steps[from:k]
			at := srf.TransformedLoopSet(set, srf.SweepEnd(steps[:from]))
			if err := srf.CheckSweepCorners(at, run, false); err != nil {
				return nil, err
			}
			s, e := srf.FaceID(0), srf.FaceID(0)
			if from == 0 {
				s = start
			}
			if k == len(steps) {
				e = end
			}
			sh := &srf.SShell{Tol: tol}
			sh.MakeFromSweepOf(at, run, color, s, e)
			parts = append(parts, sh)
			if n == len(unjoined) {
				break
			}
			corner := &srf.CornerError{At: steps[k].Center, Err: srf.ErrCornerNoJoint}
			fill, err := cornerFill(set, steps, k, 1, color, tol)
			if err != nil {
				return nil, corner
			}
			// Carried half as far, a closed corner's fill is the same:
			// where it grows, the sides never meet.
			half, err := cornerFill(set, steps, k, 0.5, color, tol)
			if err != nil {
				return nil, corner
			}
			if v, vh := meshVolume(fill), meshVolume(half); math.Abs(v-vh) > 1e-3*v+tol.ChordTolMm() {
				return nil, &srf.CornerError{At: steps[k].Center, Err: errCornerFillOpen}
			}
			parts = append(parts, fill)
			from = k + 1
		}
	}
	return joinFlush(parts, tol), nil
}

// joinFlush is the union of parts that meet flush, checked as the group
// will make its solid of it; nil where a join fails or the result is not
// sound.
func joinFlush(parts []*srf.SShell, tol srf.Tolerance) *srf.SShell {
	var out *srf.SShell
	for _, p := range parts {
		p.Tol = tol
		if out == nil {
			out = p
			continue
		}
		var joined srf.SShell
		joined.Tol = tol
		joined.MakeFromUnionOf(out, p)
		if joined.BooleanFailed {
			return nil
		}
		out = &joined
	}
	if out == nil {
		return nil
	}
	out.MergeCoincidentSurfaces()
	final := srf.SShell{Tol: tol}
	final.MakeFromUnionOf(&srf.SShell{Tol: tol}, out)
	final.MergeCoincidentSurfaces()
	if _, bad := shellCrossing(&final, true); bad || final.BooleanFailed {
		return nil
	}
	return out
}

// generateFilledSweepShell makes the sweep whose corners at unjoined
// the kernel can't join, filling them (sweepCornersFilled): a problem
// on the group, with the corner radius to set, where it can't be made.
func (g *Group) generateFilledSweepShell(sk *Sketch, src *Group, steps []srf.SweepStep, unjoined []int, start, end srf.FaceID) {
	prof := g.sweepProfile(src)
	sh, err := sweepCornersFilled(prof, steps, unjoined, g.Color, start, end, sk.tol)
	if err == nil && sh == nil {
		err = &srf.CornerError{At: steps[unjoined[0]].Center, Err: srf.ErrCornerNoJoint}
	}
	if err != nil {
		pe := cornerJointError(err)
		if p, ok := errors.AsType[*pathError](pe); ok {
			g.setProblem(p.at, "%s", p.msg)
		} else {
			g.setProblem(nil, "%v", pe)
		}
		return
	}
	g.ThisShell = *sh
	g.ThisShell.Tol = sk.tol
	g.refuseCrossing(sk, func() bool { return stepsComeBack(prof, steps) }, nil)
}
