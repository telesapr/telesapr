package sketch

import (
	"errors"
	"math"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// ErrSweepCrossesItself is the problem of a sweep whose solid would pass
// through itself: its path comes back within the profile's reach of
// where it has been, as an arc turning back over the line before it, so
// the profile's two passes overlap. The kernel sweeps each piece of the
// path on its own and joins only neighbours at their corner, so nothing
// else would catch it. A sweep along lines and arcs, open or closed, is
// made in stretches each clear of itself, joined (sweepInStretches), and
// a fitted one in spans of its path the same way (fittedInStretches);
// this is the problem where that can't be done.
var ErrSweepCrossesItself = errors.New("the sweep passes through itself where its path comes back near where it has been")

// sweepCrossTurn is how far a path must turn between two of its
// points, within twice the profile's reach of each other, before its
// sweep is meshed to look for it crossing itself. Turning less between
// them, the path runs on, its tangent never more than this from where
// it was, so two of its points are at least cos(sweepCrossTurn/2) of
// the length between them apart: within the reach only near a corner,
// where the corner's mitre or joint meets them. A pipe bent round an
// elbow wider than its profile never comes so near itself, and costs
// nothing; a sharp corner of a quarter turn or more beside an arc does,
// its two sides near, and costs a mesh of the sweep and a search of it
// (2026-11-04: 1000 to 2000 triangles, tens of milliseconds).
const sweepCrossTurn = math.Pi / 2

// sweepCrossSamples is the most points a path is sampled at to look for
// it coming back; a longer path is sampled more coarsely, the reach
// widened by the spacing.
const sweepCrossSamples = 4000

// pathTrack is a path sampled for comesBack: its points, and how far
// it has turned up to each, either way.
type pathTrack struct {
	at   []geom.Vector
	turn []float64
}

func (pt *pathTrack) add(p geom.Vector, turn float64) {
	pt.at = append(pt.at, p)
	pt.turn = append(pt.turn, turn)
}

// comesBack reports whether two of the track's points come within
// twice reach of each other, widened by spacing, with the path turning
// at least sweepCrossTurn between them.
func (pt *pathTrack) comesBack(reach, spacing float64) bool {
	near := 2*reach + spacing
	for i := range pt.at {
		for j := i + 1; j < len(pt.at); j++ {
			if pt.turn[j]-pt.turn[i] >= sweepCrossTurn && pt.at[i].Minus(pt.at[j]).Magnitude() < near {
				return true
			}
		}
	}
	return false
}

// samplesFor is how many pieces a stretch of the given length is cut
// into at spacing, at least one.
func samplesFor(length, spacing float64) int {
	return max(1, int(math.Ceil(length/spacing)))
}

// profileCentre is the centre of the profile's curves' control points.
func profileCentre(sbls *srf.SBezierLoopSetSet) geom.Vector {
	var c geom.Vector
	n := 0
	for si := range sbls.Sets {
		for _, l := range sbls.Sets[si].Loops() {
			for ci := range l.Curves {
				for _, q := range l.Curves[ci].Ctrl[:l.Curves[ci].Degree+1] {
					c = c.Plus(q)
					n++
				}
			}
		}
	}
	if n == 0 {
		return c
	}
	return c.ScaledBy(1 / float64(n))
}

// stepsComeBack reports whether a rigid sweep's path, the profile's
// centre carried along its steps, comes back within the profile's reach
// of where it has been, turning far enough between to cross itself.
func stepsComeBack(sbls *srf.SBezierLoopSetSet, steps []srf.SweepStep) bool {
	q := profileCentre(sbls)
	reach := profileReach(sbls, q)
	if reach <= 0 {
		return false
	}
	length := 0.0
	for _, st := range steps {
		switch {
		case !st.Arc && !st.Corner:
			length += st.Along.Magnitude()
		default:
			length += math.Abs(st.Angle) * (q.DistanceToLine(st.Center, st.Axis) + reach)
		}
	}
	spacing := math.Max(reach/2, length/sweepCrossSamples)
	var pt pathTrack
	turn := 0.0
	pt.add(q, turn)
	for _, st := range steps {
		if !st.Arc && !st.Corner {
			n := samplesFor(st.Along.Magnitude(), spacing)
			for k := 1; k <= n; k++ {
				pt.add(q.Plus(st.Along.ScaledBy(float64(k)/float64(n))), turn)
			}
			q = q.Plus(st.Along)
			continue
		}
		axis := st.Axis.WithMagnitude(1)
		arc := math.Abs(st.Angle) * q.DistanceToLine(st.Center, axis)
		n := max(samplesFor(arc, spacing), int(math.Ceil(math.Abs(st.Angle)/(math.Pi/18))))
		for k := 1; k <= n; k++ {
			f := float64(k) / float64(n)
			rot := geom.QuaternionFromAxisAngle(axis, st.Angle*f)
			pt.add(st.Center.Plus(rot.Rotate(q.Minus(st.Center))), turn+math.Abs(st.Angle)*f)
		}
		rot := geom.QuaternionFromAxisAngle(axis, st.Angle)
		q = st.Center.Plus(rot.Rotate(q.Minus(st.Center)))
		turn += math.Abs(st.Angle)
	}
	return pt.comesBack(reach, spacing)
}

// curvesComeBack reports whether a fitted sweep's path comes back within
// the profile's reach, from the path's start, of where it has been,
// turning far enough between to cross itself.
func curvesComeBack(sbls *srf.SBezierLoopSetSet, pieces []srf.SBezier) bool {
	if len(pieces) == 0 {
		return false
	}
	start := pieces[0].Start()
	reach := profileReach(sbls, start)
	if reach <= 0 {
		return false
	}
	length := 0.0
	for i := range pieces {
		sb := &pieces[i]
		for k := 1; k <= sb.Degree; k++ {
			length += sb.Ctrl[k].Minus(sb.Ctrl[k-1]).Magnitude()
		}
	}
	spacing := math.Max(reach/2, length/sweepCrossSamples)
	var pt pathTrack
	turn := 0.0
	var last geom.Vector
	for i := range pieces {
		sb := &pieces[i]
		poly := 0.0
		for k := 1; k <= sb.Degree; k++ {
			poly += sb.Ctrl[k].Minus(sb.Ctrl[k-1]).Magnitude()
		}
		n := max(samplesFor(poly, spacing), 16)
		for k := range n + 1 {
			t := float64(k) / float64(n)
			if d := sb.TangentAt(t); d.Magnitude() > geom.LengthEps {
				d = d.WithMagnitude(1)
				if last.Magnitude() > 0 {
					turn += math.Acos(math.Max(-1, math.Min(1, d.Dot(last))))
				}
				last = d
			}
			pt.add(sb.PointAt(t), turn)
		}
	}
	return pt.comesBack(reach, spacing)
}

// refuseCrossing makes the sweep a problem, and no solid, where its own
// shell crosses itself; comesBack says whether its path comes near
// enough to where it has been to cross, and only then is it meshed.
// remake, where not nil, makes the solid another way when it does, the
// sweep in stretches each clear of itself, joined: nil where it can't.
func (g *Group) refuseCrossing(sk *Sketch, comesBack func() bool, remake func() *srf.SShell) {
	if g.Problem.Msg != "" || len(g.ThisShell.Surface) == 0 || !comesBack() {
		return
	}
	at, crosses := shellCrossing(&g.ThisShell, false)
	if !crosses {
		return
	}
	if remake != nil {
		if sh := remake(); sh != nil {
			g.ThisShell = *sh
			return
		}
	}
	g.ThisShell.Clear()
	g.ThisShell.Tol = sk.tol
	g.setProblem(&at, "%v: move it apart, or make the profile smaller", ErrSweepCrossesItself)
}

// sweepStretchDepth is how many times sweepInStretches cuts a stretch
// that still crosses itself before giving up, and sweepStretchSweeps the
// most stretches it sweeps in all, trying the cuts, so that a sweep that
// can't be made so fails in a bounded time.
const (
	sweepStretchDepth  = 6
	sweepStretchSweeps = 48
)

// shellCrossing is a point on an edge of the shell's mesh that passes
// through a triangle, or on one with no mate, and whether there is one;
// with naked, open edges count too.
func shellCrossing(sh *srf.SShell, naked bool) (geom.Vector, bool) {
	var m srf.SMesh
	sh.TriangulateInto(&m)
	if len(m.Triangles) == 0 {
		return geom.Vector{}, false
	}
	how := srf.EdgeSelfInter
	if naked {
		how = srf.EdgeNakedOrSelfInter
	}
	var bad srf.SEdgeList
	// Only edges through a triangle: the raw shell's caps and mitres
	// touch flat where the group's mesh, joined, does not.
	srf.SKdNodeFrom(&m).MakeCertainEdgesInto(&bad, how, false, nil, nil, 0)
	if len(bad.Edges) == 0 {
		return geom.Vector{}, false
	}
	e := bad.Edges[0]
	return e.A.Plus(e.B).ScaledBy(0.5), true
}

// splitStep cuts a line or an arc step at the fraction f of its length.
func splitStep(st srf.SweepStep, f float64) (srf.SweepStep, srf.SweepStep) {
	a, b := st, st
	if st.Arc {
		a.Angle, b.Angle = st.Angle*f, st.Angle*(1-f)
	} else {
		a.Along, b.Along = st.Along.ScaledBy(f), st.Along.ScaledBy(1-f)
	}
	return a, b
}

// stepLength is a step's share of a stretch's length, for where to cut
// it: a line's length, an arc's turn times reach; a corner none, as it
// is never cut.
func stepLength(st srf.SweepStep, reach float64) float64 {
	switch {
	case st.Corner:
		return 0
	case st.Arc:
		return math.Abs(st.Angle) * reach
	}
	return st.Along.Magnitude()
}

// sweepCuts are where, as fractions of its length, a stretch that
// crosses itself is cut in two, in turn until both halves can be made:
// a cut beside a corner can leave a piece too short for its joint.
var sweepCuts = []float64{0.5, 0.35, 0.65, 0.2, 0.8}

// cutEnd is how near, as a fraction of its length, a cut may come to a
// piece's end: one there leaves a piece of no length beside the corner,
// so the stretch would start or end at the corner with no piece on that
// side for its joint.
const cutEnd = 1e-6

// cutSteps cuts steps into two stretches at the fraction f of their
// length, inside a line or an arc and never at a corner, whose joint
// needs both its pieces: the stretches and whether there was a place.
// Two pieces alike about a corner put the half exactly on the corner.
func cutSteps(steps []srf.SweepStep, reach, f float64) ([]srf.SweepStep, []srf.SweepStep, bool) {
	total := 0.0
	for _, st := range steps {
		total += stepLength(st, reach)
	}
	half := total * f
	for i, st := range steps {
		l := stepLength(st, reach)
		if l <= 0 || half > l {
			half -= l
			continue
		}
		if t := half / l; t < cutEnd || t > 1-cutEnd {
			return nil, nil, false
		}
		a, b := splitStep(st, half/l)
		first := append(append([]srf.SweepStep{}, steps[:i]...), a)
		second := append([]srf.SweepStep{b}, steps[i+1:]...)
		return first, second, true
	}
	return nil, nil, false
}

// stretches are the shells of a profile swept along steps in stretches
// each clear of itself, the profile set where the stretch starts; false
// where a stretch still crosses itself after depth cuts, can't be cut
// where a corner's joint would fit, or the budget of sweeps runs out.
func stretches(set *srf.SBezierLoopSet, steps []srf.SweepStep, color core.RGBA, start, end srf.FaceID, tol srf.Tolerance, reach float64, depth int, budget *int) ([]*srf.SShell, bool) {
	if *budget <= 0 {
		return nil, false
	}
	*budget--
	if err := srf.CheckSweepCorners(set, steps, false); err != nil {
		return nil, false
	}
	sh := srf.SShell{Tol: tol}
	sh.MakeFromSweepOf(set, steps, color, start, end)
	if _, crosses := shellCrossing(&sh, true); !crosses {
		return []*srf.SShell{&sh}, true
	}
	if depth == 0 {
		return nil, false
	}
	for _, f := range sweepCuts {
		first, second, ok := cutSteps(steps, reach, f)
		if !ok {
			continue
		}
		a, ok := stretches(set, first, color, start, end, tol, reach, depth-1, budget)
		if !ok {
			continue
		}
		b, ok := stretches(srf.TransformedLoopSet(set, srf.SweepEnd(first)), second, color, start, end, tol, reach, depth-1, budget)
		if !ok {
			continue
		}
		return append(a, b...), true
	}
	return nil, false
}

// sweepInStretches is the solid a profile passes through along steps
// whose sweep crosses itself: the union of stretches each clear of
// itself, their ends meeting flush. Round a closed path, the steps from
// the profile round to it, the steps are cut at least once, so that the
// first stretch's start and the last's end meet flush where the profile
// is drawn. Nil where it can't be made so, or where the union is not
// sound.
func sweepInStretches(sbls *srf.SBezierLoopSetSet, steps []srf.SweepStep, closed bool, color core.RGBA, start, end srf.FaceID, tol srf.Tolerance) *srf.SShell {
	q := profileCentre(sbls)
	reach := profileReach(sbls, q)
	var parts []*srf.SShell
	for i := range sbls.Sets {
		set := &sbls.Sets[i]
		budget := sweepStretchSweeps
		var p []*srf.SShell
		ok := false
		if closed {
			p, ok = closedStretches(set, steps, color, start, end, tol, reach, &budget)
		} else {
			p, ok = stretches(set, steps, color, start, end, tol, reach, sweepStretchDepth, &budget)
		}
		if !ok {
			return nil
		}
		parts = append(parts, p...)
	}
	return joinStretches(parts, tol)
}

// closedStretches are stretches round a closed path, cut at least once:
// one stretch would end flush on its own start, its caps touching.
func closedStretches(set *srf.SBezierLoopSet, steps []srf.SweepStep, color core.RGBA, start, end srf.FaceID, tol srf.Tolerance, reach float64, budget *int) ([]*srf.SShell, bool) {
	for _, f := range sweepCuts {
		first, second, ok := cutSteps(steps, reach, f)
		if !ok {
			continue
		}
		a, ok := stretches(set, first, color, start, end, tol, reach, sweepStretchDepth-1, budget)
		if !ok {
			continue
		}
		b, ok := stretches(srf.TransformedLoopSet(set, srf.SweepEnd(first)), second, color, start, end, tol, reach, sweepStretchDepth-1, budget)
		if !ok {
			continue
		}
		return append(a, b...), true
	}
	return nil, false
}

// joinStretches is the union of a sweep's stretches, each clear of
// itself, or nil where a Boolean fails or the union is not sound.
func joinStretches(parts []*srf.SShell, tol srf.Tolerance) *srf.SShell {
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
	// Checked as the group will make its solid of it: its coincident
	// surfaces merged, which the joins across the stretches' flush ends
	// leave, and passed through the union with the solid so far, here
	// none, a Boolean of its own that can open what the joins closed.
	out.MergeCoincidentSurfaces()
	final := srf.SShell{Tol: tol}
	final.MakeFromUnionOf(&srf.SShell{Tol: tol}, out)
	final.MergeCoincidentSurfaces()
	if _, bad := shellCrossing(&final, true); bad || final.BooleanFailed {
		return nil
	}
	return out
}
