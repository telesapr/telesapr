package sketch

import (
	"bytes"
	"encoding/binary"
	"math"
	"slices"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/solver"
)

// A mechanism's plan (Kramer's incremental assembly) places its parts
// in closed form before the solve: the mates are read from its
// constraints (mechanism_mates.go), the parts ordered from the ground
// out, each joined to the bodies before it, and each placed by the
// mates that join it (mechanism_fragments.go), the loops closed by a
// search over the few freedoms the parts before leave
// (mechanism_loop.go). So the parts need no start near where they go: a
// four-bar drawn at full stretch, or every part where its link put it,
// assembles, and a motion study's step goes on from the one before.
//
// The plan is compiled once per mechanism, kept while its constraints
// and groups stay as they were (planKey), and run at each solve of the
// mechanism. Its placement is checked on the mechanism's own equations;
// where it holds them and the solve before, of the same plan, left no
// freedom and was solved, as at each step of a driven motion study,
// the solve's results stand and it is not run again. Anywhere else the
// solve runs as ever, from where the plan put the parts, or from where
// they were when the plan can't place them: a constraint the plan
// doesn't know, a loop its search can't close. The solve is what counts
// freedom, finds redundancy and reports, so none of that changes. Nor
// does a drag or a pull: their solve keeps the pulled part as near as
// it can, which the plan, placing each part from the ground out,
// doesn't.

// mechanismPlan is a mechanism's compiled plan.
type mechanismPlan struct {
	key []byte
	// covered is false for a mechanism the plan can't place, so that it
	// isn't compiled again until it changes.
	covered bool
	mates   []planMate
	// order is the bodies in the order they are placed; anchored those
	// that nothing placed before joins, which stay where they are.
	order    []int
	anchored []bool
	// placing is, of each body, the mates that join it to the bodies
	// before it.
	placing [][]int
	// solved is whether the last solve of the mechanism with this plan
	// was solved with no freedom left, so that a placement by the plan
	// that holds the equations is its solution too.
	solved bool
}

// planRun is one run of a plan.
type planRun struct {
	plan *mechanismPlan
	// start is each body's placement when the run began, pl where the
	// run puts it now; the ground's is none.
	start, pl []place
	mates     []liveMate
	bodies    []bodyRun
	// nv is the loop's unknowns; isTurn whether each is a turn.
	nv     int
	isTurn []bool
}

// bodyRun is what a run knows of a body.
type bodyRun struct {
	// anchorLocal is the centre of its placing mates' points in its own
	// frame, anchorWorld where it was at the start: what its move is
	// kept nearest.
	anchorLocal, anchorWorld geom.Vector
	// solvedTurn is a turn its own mates hold, searched at each
	// placement from theta, where the last left it; turnVar the loop's
	// unknown for a free turn, or -1.
	solvedTurn bool
	theta      float64
	turnVar    int
	// slides are its free moves, slideVar the first of their unknowns.
	slides   []geom.Vector
	slideVar int
	// g is what its placing mates miss by where it is placed.
	g []float64
}

// planKey is what a mechanism's plan is made from: its groups, its
// constraints and the entities they hold.
func (sk *Sketch) planKey(hgs []core.HGroup) []byte {
	var b []byte
	w := func(vs ...uint32) {
		for _, v := range vs {
			b = binary.LittleEndian.AppendUint32(b, v)
		}
	}
	flag := func(f bool) uint32 {
		if f {
			return 1
		}
		return 0
	}
	for _, hg := range hgs {
		g := sk.Group(hg)
		w(hg.V, flag(g.AllDimsReference), flag(g.RelaxConstraints))
	}
	ent := func(he core.HEntity) {
		if e := sk.Entity(he); e != nil {
			w(he.V, uint32(e.Type), e.Group.V)
			return
		}
		w(he.V, 0, 0)
	}
	for ci := range sk.Constraints {
		c := &sk.Constraints[ci]
		if !slices.Contains(hgs, c.Group) {
			continue
		}
		w(c.H.V, uint32(c.Type), c.Workplane.V, flag(c.Reference), uint32(c.Limit), flag(c.Other))
		for _, he := range []core.HEntity{c.PtA, c.PtB, c.EntityA, c.EntityB} {
			ent(he)
		}
	}
	return b
}

// planOf is the plan of mechanism hgs, compiled when it has changed;
// nil where the plan can't place it.
func (sk *Sketch) planOf(hgs []core.HGroup) *mechanismPlan {
	key := sk.planKey(hgs)
	p := sk.plans[hgs[0]]
	if p == nil || !bytes.Equal(p.key, key) {
		p = sk.compilePlan(hgs)
		p.key = key
		if sk.plans == nil || len(sk.plans) > 2*len(sk.Groups) {
			sk.plans = make(map[core.HGroup]*mechanismPlan)
		}
		sk.plans[hgs[0]] = p
		sk.PlanCompiles++
	}
	if !p.covered {
		return nil
	}
	return p
}

// compilePlan reads the mates of mechanism hgs and orders its bodies:
// from the ground, each next the one that the most mates join to those
// before it; one that none join stays where it is.
func (sk *Sketch) compilePlan(hgs []core.HGroup) *mechanismPlan {
	p := &mechanismPlan{}
	for _, hg := range hgs {
		if g := sk.Group(hg); g.AllDimsReference || g.RelaxConstraints {
			return p
		}
	}
	var cs []*core.Constraint
	for ci := range sk.Constraints {
		if c := &sk.Constraints[ci]; slices.Contains(hgs, c.Group) {
			cs = append(cs, c)
		}
	}
	slices.SortFunc(cs, func(a, b *core.Constraint) int { return int(a.H.V) - int(b.H.V) })
	for _, c := range cs {
		// A reference or a comment holds nothing; a bound holds only
		// where the solve meets it, which is checked after.
		if c.Reference || c.Type == core.ConstraintComment || c.Limit != core.LimitNone {
			continue
		}
		m, ok := sk.mateOf(hgs, c)
		if !ok {
			return p
		}
		p.mates = append(p.mates, m)
	}
	n := len(hgs) + 1
	placed := make([]bool, n)
	placed[0] = true
	p.anchored = make([]bool, n)
	p.placing = make([][]int, n)
	isPlaced := func(b int) bool { return placed[b] }
	for range n - 1 {
		best, most := -1, 0
		for b := 1; b < n; b++ {
			if placed[b] {
				continue
			}
			count := 0
			for i := range p.mates {
				if p.mates[i].joins(b, isPlaced) {
					count++
				}
			}
			if best < 0 || count > most {
				best, most = b, count
			}
		}
		if most == 0 {
			p.anchored[best] = true
		}
		for i := range p.mates {
			if p.mates[i].joins(best, isPlaced) {
				p.placing[best] = append(p.placing[best], i)
			}
		}
		placed[best] = true
		p.order = append(p.order, best)
	}
	p.covered = true
	return p
}

// placementOf is group hg's placement now, its turn made unit.
func (sk *Sketch) placementOf(hg core.HGroup) (place, bool) {
	val := func(i int) float64 { return sk.MustParam(hg.Param(i)).Val }
	q := geom.NewQuaternion(val(3), val(4), val(5), val(6))
	if m := q.Magnitude(); m < 1e-9 || math.IsNaN(m) {
		return place{}, false
	}
	return place{geom.Vector{X: val(0), Y: val(1), Z: val(2)}, q.WithMagnitude(1)}, true
}

// setPlacement writes placement pl into group hg's params, its
// quaternion on the side of near's, the same turn, so that the params
// change no more than the turn does.
func (sk *Sketch) setPlacement(hg core.HGroup, pl place, near geom.Quaternion) {
	q := pl.q
	if q.W*near.W+q.X*near.X+q.Y*near.Y+q.Z*near.Z < 0 {
		q = q.ScaledBy(-1)
	}
	for i, v := range []float64{pl.t.X, pl.t.Y, pl.t.Z, q.W, q.X, q.Y, q.Z} {
		sk.MustParam(hg.Param(i)).Val = v
	}
}

// planParams are the params a run of a plan writes: the parts' moves
// and turns, and the params of the points on lines and the parallels.
func (sk *Sketch) planParams(hgs []core.HGroup) []core.HParam {
	var hs []core.HParam
	for _, hg := range hgs {
		for i := range 7 {
			hs = append(hs, hg.Param(i))
		}
	}
	for ci := range sk.Constraints {
		c := &sk.Constraints[ci]
		if slices.Contains(hgs, c.Group) && c.ValP != core.NoParam && sk.Param(c.ValP) != nil {
			hs = append(hs, c.ValP)
		}
	}
	return hs
}

// runPlan places mechanism hgs by plan p; false, with every param as it
// was, where it can't, or where what it placed doesn't hold the
// mechanism's equations.
func (sk *Sketch) runPlan(p *mechanismPlan, hgs []core.HGroup) bool {
	hs := sk.planParams(hgs)
	saved := make([]float64, len(hs))
	for i, h := range hs {
		saved[i] = sk.MustParam(h).Val
	}
	restore := func() bool {
		for i, h := range hs {
			sk.MustParam(h).Val = saved[i]
		}
		for _, hg := range hgs {
			sk.recalculateGroupEntities(hg)
		}
		return false
	}
	n := len(hgs) + 1
	r := &planRun{plan: p, start: make([]place, n), pl: make([]place, n), bodies: make([]bodyRun, n)}
	r.start[0] = place{q: geom.Quaternion{W: 1}}
	for i, hg := range hgs {
		pl, ok := sk.placementOf(hg)
		if !ok {
			return false
		}
		// The turn made unit first, so that the features read in each
		// part's frame are where the turn puts them.
		sk.setPlacement(hg, pl, pl.q)
		sk.recalculateGroupEntities(hg)
		r.start[i+1] = pl
	}
	copy(r.pl, r.start)
	if err := catchPanic(func() {
		for i := range p.mates {
			r.mates = append(r.mates, sk.liveMateOf(&p.mates[i], r.start))
		}
	}); err != nil {
		return restore()
	}
	for b := 1; b < n; b++ {
		br := &r.bodies[b]
		br.turnVar = -1
		var sum geom.Vector
		count := 0
		for _, mi := range p.placing[b] {
			m := &r.mates[mi]
			for _, f := range append(slices.Clone(m.p), m.o) {
				if f.body == b {
					sum = sum.Plus(f.local)
					count++
				}
			}
		}
		if count > 0 {
			br.anchorLocal = sum.ScaledBy(1 / float64(count))
		}
		br.anchorWorld = feat{b, br.anchorLocal}.point(r.start)
	}
	if !r.closeLoops() {
		return restore()
	}
	for i, hg := range hgs {
		sk.setPlacement(hg, r.pl[i+1], r.start[i+1].q)
		sk.recalculateGroupEntities(hg)
	}
	sk.settleMateParams(hgs)
	if res, ok := sk.blockResidual(hgs); !ok || res > solver.ConvergeTolerance {
		return restore()
	}
	return true
}

// settleMateParams gives the params of the points on lines and the
// parallels of mechanism hgs the values that hold where the parts are:
// how far along its line each point is, how long one direction is
// against the other.
func (sk *Sketch) settleMateParams(hgs []core.HGroup) {
	for ci := range sk.Constraints {
		c := &sk.Constraints[ci]
		if !slices.Contains(hgs, c.Group) || c.ValP == core.NoParam || sk.Param(c.ValP) == nil {
			continue
		}
		switch c.Type {
		case core.ConstraintPtOnLine:
			c.ModifyToSatisfy(sk)
		case core.ConstraintParallel:
			a := sk.MustEntity(c.EntityA).VectorGetNum(sk)
			b := sk.MustEntity(c.EntityB).VectorGetNum(sk)
			if bb := b.Dot(b); bb > 0 {
				sk.MustParam(c.ValP).Val = a.Dot(b) / bb
			}
		}
	}
}

// blockResidual is the most any equation of mechanism hgs misses by;
// ok false where they can't be written.
func (sk *Sketch) blockResidual(hgs []core.HGroup) (res float64, ok bool) {
	var l []core.Equation
	err := catchPanic(func() {
		for _, hg := range hgs {
			sk.WriteEquationsExceptFor(hg, core.NoConstraint, &l)
		}
		for _, e := range l {
			v := core.EvalExpr(sk, e.Expr)
			if math.IsNaN(v) {
				res = math.Inf(1)
				return
			}
			res = math.Max(res, math.Abs(v))
		}
	})
	return res, err == nil
}

// solveByPlan places mechanism hgs by its plan, before its solve, and
// reports whether that solved it: the plan's placement holds its
// equations, and the solve before, of the same plan, was solved with no
// freedom left, no bound met, as the solve's results then stand. It
// does nothing while a part is dragged or pulled.
func (sk *Sketch) solveByPlan(hgs []core.HGroup) bool {
	if sk.NoMechanismPlan || !movesInMechanism(sk.Group(hgs[0])) || len(sk.pulled) > 0 ||
		len(sk.hooks().DraggedParams()) > 0 || sk.movedIn(hgs) {
		return false
	}
	p := sk.planOf(hgs)
	if p == nil {
		return false
	}
	sk.seedAxisAngles(hgs)
	res, ok := sk.blockResidual(hgs)
	if !ok {
		return false
	}
	if res > solver.ConvergeTolerance {
		if !sk.runPlan(p, hgs) {
			return false
		}
		sk.PlanPlaced++
	}
	if !p.solved || sk.editedIn(hgs) || !sk.boundsClear(hgs) {
		return false
	}
	for _, hg := range hgs {
		if !sk.Group(hg).IsSolvedOkay() {
			return false
		}
		sk.recalculateGroupEntities(hg)
	}
	sk.PlanSolved++
	return true
}

// notePlanSolved records, after a solve of mechanism hgs, whether it
// left the mechanism solved with no freedom, so that the next run of
// its plan may stand for the solve.
func (sk *Sketch) notePlanSolved(hgs []core.HGroup) {
	p := sk.plans[hgs[0]]
	if p == nil {
		return
	}
	p.solved = false
	// The cheap tests first: a pull's or a drag's solves come here many
	// times a frame, of a mechanism with freedom left.
	for _, hg := range hgs {
		g := sk.Group(hg)
		if !g.IsSolvedOkay() || g.Solved.Dof != 0 || g.Solved.Timeout || g.SketchDiagnostic != nil {
			return
		}
	}
	p.solved = sk.boundsClear(hgs) && bytes.Equal(p.key, sk.planKey(hgs))
}

// boundsClear reports whether no bound of mechanism hgs is held, met or
// crossed where its parts are.
func (sk *Sketch) boundsClear(hgs []core.HGroup) bool {
	for ci := range sk.Constraints {
		c := &sk.Constraints[ci]
		if !slices.Contains(hgs, c.Group) || !c.IsInequality() || c.Reference {
			continue
		}
		if c.Bound != core.BoundFree {
			return false
		}
		if by, _ := c.Crossed(sk); core.IsCrossed(by) || core.IsAtBound(by) {
			return false
		}
	}
	return true
}

// movedIn reports whether a point of mechanism hgs is being moved, for
// its solve to follow (continuation.go).
func (sk *Sketch) movedIn(hgs []core.HGroup) bool {
	return slices.ContainsFunc(sk.moves, func(m pointMove) bool {
		e := sk.Entity(m.p)
		return e != nil && slices.Contains(hgs, e.Group)
	})
}

// editedIn reports whether a dimension of mechanism hgs was changed,
// for its solve to follow.
func (sk *Sketch) editedIn(hgs []core.HGroup) bool {
	return slices.ContainsFunc(sk.edits, func(e dimEdit) bool {
		c := sk.Constraint(e.c)
		return c != nil && slices.Contains(hgs, c.Group)
	})
}
