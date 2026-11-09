package sketch

import (
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
)

// Newton solves a sketch from where it was, and goes to whichever of
// its solutions it comes to first: a dimension made two and a half
// times longer in one edit, or a point dragged half the sketch's width
// in one mouse move, can land it on a mirror image of what it was, a
// triangle turned over or an arm's elbow bent the other way. So an edit
// or a drag that the user makes (EditConstraintValue, SetConstraintText,
// MovePointTo) is remembered until the next solve of its group, and
// where it is large the solve follows it there in steps (continuation),
// each solved from the one before, then checks that no corner turned
// over (flip.go). A regeneration with nothing edited or dragged, as of
// a file read or a group made again, solves as before.

// followFraction is the most of a dimension's value, or of the sketch's
// size for a drag, that one step changes it by: a change larger than
// this is made in steps.
const followFraction = 0.25

// maxFollowSteps bounds the steps of one edit or drag, as pullSteps
// bounds a pull's.
const maxFollowSteps = pullSteps

// dimEdit is a dimension the user changed, and its value before.
type dimEdit struct {
	c    core.HConstraint
	from float64
}

// pointMove is a point the user moved, and where it was before.
type pointMove struct {
	p    core.HEntity
	from geom.Vector
}

// noteEdit remembers that the user is giving constraint c a new value,
// from from, for the next solve of its group. A dimension changed twice
// before that solve is followed from its first value.
func (sk *Sketch) noteEdit(c *core.Constraint, from float64) {
	if c.ValA == from || c.Type == core.ConstraintAxisAngle ||
		slices.ContainsFunc(sk.edits, func(e dimEdit) bool { return e.c == c.H }) {
		return
	}
	sk.edits = append(sk.edits, dimEdit{c: c.H, from: from})
}

// noteMove remembers that the user is moving point he, which is at from
// now, for the next solve of its group.
func (sk *Sketch) noteMove(he core.HEntity, from geom.Vector) {
	if slices.ContainsFunc(sk.moves, func(m pointMove) bool { return m.p == he }) {
		return
	}
	sk.moves = append(sk.moves, pointMove{p: he, from: from})
}

// forgetMotion forgets the edits and moves not yet followed, at the end
// of a regeneration, and at undo, which puts back a state they were
// not made in.
func (sk *Sketch) forgetMotion() {
	sk.edits, sk.moves = nil, nil
}

// motion is what the user edited and moved in one block of groups.
type motion struct {
	edits []dimEdit
	moves []pointMove
}

// takeMotion takes the edits and moves of the block of groups hgs out of
// those remembered; nil when there are none.
func (sk *Sketch) takeMotion(hgs []core.HGroup) *motion {
	if len(sk.edits) == 0 && len(sk.moves) == 0 || sk.NoFollow {
		return nil
	}
	m := &motion{}
	sk.edits = slices.DeleteFunc(sk.edits, func(e dimEdit) bool {
		c := sk.Constraint(e.c)
		if c == nil || !slices.Contains(hgs, c.Group) {
			return c == nil
		}
		m.edits = append(m.edits, e)
		return true
	})
	sk.moves = slices.DeleteFunc(sk.moves, func(mv pointMove) bool {
		e := sk.Entity(mv.p)
		if e == nil || !slices.Contains(hgs, e.Group) {
			return e == nil
		}
		m.moves = append(m.moves, mv)
		return true
	})
	if len(m.edits) == 0 && len(m.moves) == 0 {
		return nil
	}
	return m
}

// following is a solve following an edit or a drag: the block's order
// shape and its orientation at the last solution.
type following struct {
	shape  *orderShape
	last   orientation
	params []core.HParam
	// turned names what turned over and stayed so in a step.
	turned []string
}

// followMotion steps the block of groups hgs, solved for params, from
// where m found it toward where m puts it, solving at each step by
// step, and leaves it at the step before the last, for the block's own
// solve to take the last. Each step's solution is checked against the
// one before (unflip). A step that fails to solve ends the stepping, at
// the solution before it.
func (sk *Sketch) followMotion(m *motion, hgs []core.HGroup, params []core.HParam, step func() core.SolveResult) *following {
	var moved []core.HEntity
	for _, mv := range m.moves {
		moved = append(moved, mv.p)
	}
	f := &following{shape: sk.orderShapeOf(hgs, params, moved), params: params}
	sk.Followed++

	// Where the edit or the drag goes: the dimensions' new values, the
	// points' new params.
	toVal := make([]float64, len(m.edits))
	for i, e := range m.edits {
		toVal[i] = sk.Constraint(e.c).ValA
	}
	toPos := make([]geom.Vector, len(m.moves))
	toParams := make([][]float64, len(m.moves))
	for i, mv := range m.moves {
		e := sk.MustEntity(mv.p)
		toPos[i] = e.PointGetNum(sk)
		toParams[i] = sk.pointParams(e)
	}
	at := func(t float64) {
		for i, e := range m.edits {
			sk.Constraint(e.c).ValA = between(e.from, toVal[i], t)
		}
		for i, mv := range m.moves {
			sk.MustEntity(mv.p).PointForceTo(sk, mv.from.Plus(toPos[i].Minus(mv.from).ScaledBy(t)))
		}
	}
	arrive := func() {
		for i, e := range m.edits {
			sk.Constraint(e.c).ValA = toVal[i]
		}
		for i, mv := range m.moves {
			sk.setPointParams(sk.MustEntity(mv.p), toParams[i])
		}
	}

	at(0)
	f.last = f.shape.orientation(sk)
	// Steps are taken only for an orientation to keep: a sketch with no
	// corner between held lengths that is not straight, and no arc that
	// is not a whole turn, as a line just drawn, is solved in one step as
	// before, since the path it took would only change where it rests.
	k := 1
	if f.last.oriented() {
		k = sk.followSteps(m, hgs, toVal, toPos)
	}
	for i := 1; i < k; i++ {
		saved := sk.paramValues(params)
		at(float64(i) / float64(k))
		if !solvedOkay(step()) {
			sk.setParamValues(params, saved)
			break
		}
		sk.FollowSteps++
		f.unflip(sk, step)
	}
	arrive()
	return f
}

// settle checks the block's own solve, which took the last step, by
// unflip, solving again by solve; it is the note for the group, "" when
// nothing turned over.
func (f *following) settle(sk *Sketch, solve func() core.SolveResult) string {
	f.unflip(sk, solve)
	if len(f.turned) == 0 {
		return ""
	}
	return fmt.Sprintf("the last edit or drag turned %s over, and no solution near it keeps the sketch's orientation",
		strings.Join(f.turned, ", "))
}

// unflip compares the block's orientation now, just solved, with the
// last; where a corner or an arc turned over, it reflects it back
// (orderShape.reflectTurned) and solves again by solve, keeping that
// solution if it solves and turns nothing over. Otherwise the first
// solution is put back, solved again so that what the solve reports is
// its own, and what turned over is named.
func (f *following) unflip(sk *Sketch, solve func() core.SolveResult) {
	now := f.shape.orientation(sk)
	corners, arcs := turnedOver(f.last, now)
	if len(corners) == 0 && len(arcs) == 0 {
		f.last.update(now)
		return
	}
	first := sk.paramValues(f.params)
	if f.shape.reflectTurned(sk, f.last, corners, arcs) {
		if solvedOkay(solve()) {
			again := f.shape.orientation(sk)
			if c, a := turnedOver(f.last, again); len(c) == 0 && len(a) == 0 {
				f.last.update(again)
				return
			}
		}
		sk.setParamValues(f.params, first)
		solve()
	}
	for _, n := range f.shape.turnedNames(sk, corners, arcs) {
		if !slices.Contains(f.turned, n) {
			f.turned = append(f.turned, n)
		}
	}
	f.last.update(now)
}

// followSteps is how many steps an edit or a drag is made in: enough
// that no step changes a dimension by more than followFraction of its
// value, nor moves a point by more than followFraction of the block's
// size; 1, a single solve, for a small one.
func (sk *Sketch) followSteps(m *motion, hgs []core.HGroup, toVal []float64, toPos []geom.Vector) int {
	k := 1
	for i, e := range m.edits {
		k = max(k, dimSteps(e.from, toVal[i]))
	}
	if len(m.moves) > 0 {
		if size := sk.blockSize(hgs); size > 0 {
			for i, mv := range m.moves {
				d := toPos[i].Minus(mv.from).Magnitude()
				k = max(k, int(math.Ceil(d/(followFraction*size)-1e-9)))
			}
		}
	}
	return min(k, maxFollowSteps)
}

// dimSteps is how many steps a dimension's change from a to b is made
// in: in proportion where both are of one sign, a step multiplying it by
// at most 1 + followFraction, and otherwise in even steps of at most
// followFraction of the larger.
func dimSteps(a, b float64) int {
	if math.Abs(b-a) <= followFraction*math.Abs(a) {
		return 1
	}
	if a*b > 0 {
		return int(math.Ceil(math.Abs(math.Log(b/a))/math.Log1p(followFraction) - 1e-9))
	}
	return int(math.Ceil(math.Abs(b-a)/(followFraction*max(math.Abs(a), math.Abs(b))) - 1e-9))
}

// between is the value a fraction t of the way from a to b, in
// proportion where both are of one sign (dimSteps).
func between(a, b, t float64) float64 {
	switch {
	case t >= 1:
		return b
	case a*b > 0:
		return a * math.Pow(b/a, t)
	}
	return a + (b-a)*t
}

// blockSize is the diagonal of the box round the points of the block of
// groups hgs. A drag asks for it at every frame, so the points are read
// under one guard, and only a sketch with a point that can't be read
// reads them one by one, leaving that point out.
func (sk *Sketch) blockSize(hgs []core.HGroup) float64 {
	var size float64
	if catchPanic(func() {
		size = sk.blockSizeOf(hgs, func(e *core.Entity) (geom.Vector, bool) { return e.PointGetNum(sk), true })
	}) == nil {
		return size
	}
	return sk.blockSizeOf(hgs, func(e *core.Entity) (p geom.Vector, ok bool) {
		ok = catchPanic(func() { p = e.PointGetNum(sk) }) == nil
		return p, ok
	})
}

// blockSizeOf is blockSize with each point read by at, which says
// whether it could be.
func (sk *Sketch) blockSizeOf(hgs []core.HGroup, at func(*core.Entity) (geom.Vector, bool)) float64 {
	lo := geom.Vector{X: math.Inf(1), Y: math.Inf(1), Z: math.Inf(1)}
	hi := lo.ScaledBy(-1)
	for i := range sk.Entities {
		e := &sk.Entities[i]
		if !e.IsPoint() || !slices.Contains(hgs, e.Group) {
			continue
		}
		p, ok := at(e)
		if !ok {
			continue
		}
		lo = geom.Vector{X: min(lo.X, p.X), Y: min(lo.Y, p.Y), Z: min(lo.Z, p.Z)}
		hi = geom.Vector{X: max(hi.X, p.X), Y: max(hi.Y, p.Y), Z: max(hi.Z, p.Z)}
	}
	if lo.X > hi.X {
		return 0
	}
	return hi.Minus(lo).Magnitude()
}

// solvedOkay reports whether a solve found a solution.
func solvedOkay(how core.SolveResult) bool {
	return how == core.SolveOkay || how == core.SolveRedundantOkay
}

// paramValues are the values of params.
func (sk *Sketch) paramValues(params []core.HParam) []float64 {
	v := make([]float64, len(params))
	for i, h := range params {
		if p := sk.Param(h); p != nil {
			v[i] = p.Val
		}
	}
	return v
}

// setParamValues gives params the values v (paramValues).
func (sk *Sketch) setParamValues(params []core.HParam, v []float64) {
	for i, h := range params {
		if p := sk.Param(h); p != nil {
			p.Val = v[i]
		}
	}
}

// pointParams are the values of point e's params.
func (sk *Sketch) pointParams(e *core.Entity) []float64 {
	return sk.paramValues(e.Param[:])
}

// setPointParams gives point e's params the values v (pointParams).
func (sk *Sketch) setPointParams(e *core.Entity, v []float64) {
	sk.setParamValues(e.Param[:], v)
}
