package topo

import (
	"fmt"
	"math"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// A path may turn sharply beside an arc too. Along an arc the frames
// that turn least turn about its axis, and along a line not at all, so
// with no twist there (it is held still, pathsweep_hold.go) the sweep of
// a run of lines and arcs about such a corner is a rigid sweep's, whose
// joints srf makes exactly where each profile point's paths along the
// two pieces cross (srf.CheckSweepCorners). The run is swept so, the
// fitted pieces either side as before, each closed by caps on the
// sections between them, and the solids joined by taking those caps out
// (srf.SShell.JoinedAtCaps): their outlines are the one section.

// joinCapBase numbers the caps made between runs to be taken out again;
// no face of a sketch's entities comes near it.
const joinCapBase uint32 = 0xfff00000

// arcOf is sb's circle when it is an arc, a rational quadratic: its
// centre, its axis by the right-hand rule along sb, and the angle it
// turns through.
func arcOf(sb *srf.SBezier) (center, axis geom.Vector, angle float64, ok bool) {
	if sb.Degree != 2 || sb.Weight[0] <= 0 || math.Abs(sb.Weight[2]-sb.Weight[0]) > 1e-12*sb.Weight[0] {
		return center, axis, 0, false
	}
	p0, p1, p2 := sb.Ctrl[0], sb.Ctrl[1], sb.Ctrl[2]
	a, b := p0.Minus(p1), p2.Minus(p1)
	la, lb := a.Magnitude(), b.Magnitude()
	if la == 0 || math.Abs(la-lb) > 1e-9*la {
		return center, axis, 0, false
	}
	phi := math.Acos(math.Max(-1, math.Min(1, a.Dot(b)/(la*lb))))
	angle = math.Pi - phi
	if angle < 1e-9 || math.Abs(sb.Weight[1]/sb.Weight[0]-math.Cos(angle/2)) > 1e-9 {
		return center, axis, 0, false
	}
	t := a.ScaledBy(-1 / la)
	axis = t.Cross(b).WithMagnitude(1)
	center = p0.Plus(axis.Cross(t).ScaledBy(la / math.Tan(angle/2)))
	return center, axis, angle, true
}

// sameCircle reports whether arcs a and b lie on one circle, turning the
// same way.
func sameCircle(a, b *srf.SBezier) bool {
	ca, xa, _, oka := arcOf(a)
	cb, xb, _, okb := arcOf(b)
	return oka && okb && ca.Minus(cb).Magnitude() < geom.LengthEps && xa.Dot(xb) > 1-1e-9
}

// growRigid widens each run marked rigid over the pieces it meets at a
// corner, which a rigid sweep must join too, and along its arcs' own
// circles, so that a joint has the whole arc beside it.
func (sp *SweepPath) growRigid() {
	n := len(sp.pieces)
	for grown := true; grown; {
		grown = false
		for i := range n {
			if !sp.rigid[i] {
				continue
			}
			mark := func(j int) {
				if j >= 0 && j < n && !sp.rigid[j] {
					sp.rigid[j], grown = true, true
				}
			}
			if sp.corner[i] || i > 0 && sameCircle(&sp.pieces[i-1], &sp.pieces[i]) {
				mark(i - 1)
			}
			if i+1 < n && (sp.corner[i+1] || sameCircle(&sp.pieces[i], &sp.pieces[i+1])) {
				mark(i + 1)
			}
		}
	}
	for i := range n {
		sp.prism[i] = sp.prism[i] && !sp.rigid[i]
	}
}

// rigidRuns are the runs of pieces swept rigidly, [lo, hi) each.
func (sp *SweepPath) rigidRuns() [][2]int {
	var out [][2]int
	for i := 0; i < len(sp.pieces); {
		if !sp.rigid[i] {
			i++
			continue
		}
		j := i
		for j < len(sp.pieces) && sp.rigid[j] {
			j++
		}
		out = append(out, [2]int{i, j})
		i = j
	}
	return out
}

// runSteps are the rigid sweep's steps along pieces lo to hi, with the
// corners between them.
func (sp *SweepPath) runSteps(lo, hi int) []srf.SweepStep {
	var steps []srf.SweepStep
	for i := lo; i < hi; i++ {
		if i > lo && sp.corner[i] {
			steps = append(steps, sp.cornerStep(i))
		}
		steps = append(steps, sp.pieceStep(i))
	}
	return steps
}

// cornerStep is the turn at the corner where piece i starts.
func (sp *SweepPath) cornerStep(i int) srf.SweepStep {
	d1, d2 := sp.tangentInto(i-1), sp.tangent(float64(i))
	return srf.SweepStep{Corner: true, Center: sp.pieces[i].Start(),
		Axis: d1.Cross(d2).WithMagnitude(1), Angle: math.Acos(math.Max(-1, math.Min(1, d1.Dot(d2))))}
}

// pieceStep is piece i, a line or an arc, as a rigid sweep's step.
func (sp *SweepPath) pieceStep(i int) srf.SweepStep {
	pc := &sp.pieces[i]
	if c, axis, angle, ok := arcOf(pc); ok {
		return srf.SweepStep{Arc: true, Center: c, Axis: axis, Angle: angle}
	}
	return srf.SweepStep{Along: pc.Finish().Minus(pc.Start())}
}

// checkRigid refuses a run whose frames turn otherwise than its rigid
// steps turn the profile: frames held to a direction other than the
// arcs' and corners' axis. They are compared at each piece's start and
// middle, and the run's end.
func (sp *SweepPath) checkRigid() error {
	for _, r := range sp.rigidRuns() {
		f0 := sp.untwistedAt(float64(r[0]))
		off := func(s float64, q geom.Quaternion) bool {
			f := sp.untwistedAt(s)
			return q.Rotate(f0.r).Minus(f.r).Magnitude() > 1e-7 || q.Rotate(f0.t).Minus(f.t).Magnitude() > 1e-7
		}
		q := geom.QuaternionIdentity
		for i := r[0]; i < r[1]; i++ {
			if i > r[0] && sp.corner[i] {
				q = sp.cornerStep(i).Motion().Q.Times(q)
			}
			st := sp.pieceStep(i)
			half := q
			if st.Arc {
				half = geom.QuaternionFromAxisAngle(st.Axis, st.Angle/2).Times(q)
			}
			if off(float64(i), q) || off(float64(i)+0.5, half) {
				return &srf.CornerError{At: sp.pieces[i].Start(), Err: ErrPathCornerHeld}
			}
			q = st.Motion().Q.Times(q)
		}
		if off(float64(r[1])-1e-9, q) {
			return &srf.CornerError{At: sp.pieces[r[1]-1].Finish(), Err: ErrPathCornerHeld}
		}
	}
	return nil
}

// profileSet is the profile's loops as a loop set at the path's start,
// the outer loop clockwise about its normal, as a sketch's sets run,
// and the holes the other way.
func profileSet(loops [][]srf.SBezier) *srf.SBezierLoopSet {
	n := newell(loops[0]).WithMagnitude(-1)
	set := &srf.SBezierLoopSet{Outer: srf.SBezierLoop{Curves: loops[0]}, Normal: n, Point: loops[0][0].Start()}
	for _, h := range loops[1:] {
		if newell(h).Dot(n) < 0 {
			h = reverseLoop(h)
		}
		set.Holes = append(set.Holes, srf.SBezierLoop{Curves: h})
	}
	return set
}

// makeJointed is MakePathSweep along a path with runs swept rigidly: each
// run and each stretch of fitted pieces between them a solid of its own,
// closed by caps between them that are then taken out.
func (sp *SweepPath) makeJointed(loops [][]srf.SBezier, color core.RGBA, startFace, endFace, origin uint32) (*srf.SShell, error) {
	surfs, _, err := sp.SweptSurfaces(loops)
	if err != nil {
		return nil, err
	}
	set := profileSet(loops)
	// The runs and the stretches between them, in order.
	type part struct {
		lo, hi int
		rigid  bool
	}
	var parts []part
	at := 0
	for _, r := range sp.rigidRuns() {
		if r[0] > at {
			parts = append(parts, part{at, r[0], false})
		}
		parts = append(parts, part{r[0], r[1], true})
		at = r[1]
	}
	if at < len(sp.pieces) {
		parts = append(parts, part{at, len(sp.pieces), false})
	}
	closed := sp.opts.Closed
	capAt := func(j int, end bool) uint32 {
		switch {
		case !closed && j == 0 && !end:
			return startFace
		case !closed && j == len(parts)-1 && end:
			return endFace
		case end:
			return joinCapBase + uint32(2*j+1)
		}
		return joinCapBase + uint32(2*j)
	}
	var whole *srf.SShell
	id := origin
	for j, pt := range parts {
		c0, c1 := capAt(j, false), capAt(j, true)
		var sh *srf.SShell
		if pt.rigid {
			steps := sp.runSteps(pt.lo, pt.hi)
			moved := srf.TransformedLoopSet(set, sp.Rigid(float64(pt.lo)))
			if err := srf.CheckSweepCorners(moved, steps, false); err != nil {
				return nil, err
			}
			sh = &srf.SShell{Tol: sp.tol}
			sh.MakeFromSweepOf(moved, steps, color, srf.FaceID(c0), srf.FaceID(c1))
			if len(sh.Surface) == 0 {
				return nil, fmt.Errorf("the sweep's run from %v made no solid", sp.pieces[pt.lo].Start())
			}
			// Identities as the fitted pieces' are: the caps' and the
			// patches' origins, no faces.
			for i := range sh.Surface {
				ss := &sh.Surface[i]
				switch ss.Face {
				case srf.FaceID(c0), srf.FaceID(c1):
					ss.Origin = uint32(ss.Face)
				default:
					ss.Origin = id
					id++
				}
				ss.Face = 0
			}
		} else {
			chains := make([][]*srf.BSplineSurface, len(surfs))
			for li := range surfs {
				chains[li] = surfs[li][pt.lo:pt.hi]
			}
			if sh, err = closeSkins(chains, color, c0, c1, id, sp.tol); err != nil {
				return nil, err
			}
			id += uint32(len(sh.Surface) - 2)
		}
		if whole == nil {
			whole = sh
			continue
		}
		var w srf.SShell
		w.MakeFromAssemblyOf(whole, sh)
		whole = &w
	}
	for j := 0; j+1 < len(parts); j++ {
		if whole, err = whole.JoinedAtCaps(capAt(j, true), capAt(j+1, false)); err != nil {
			return nil, err
		}
	}
	if closed {
		if whole, err = whole.JoinedAtCaps(capAt(len(parts)-1, true), capAt(0, false)); err != nil {
			return nil, err
		}
	}
	t, err := Build(whole)
	if err != nil {
		return nil, err
	}
	if probs := t.Validate(); len(probs) > 0 {
		return nil, fmt.Errorf("the surface is not closed: %s", probs[0].Msg)
	}
	return whole, nil
}
