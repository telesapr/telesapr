package sketch

import (
	"encoding/binary"
	"math"
	"slices"

	"github.com/telesapr/telesapr/pkg/core"
)

// A drag solves its block at every mouse move, and each solve that
// follows it reads the block's order shape (flip.go): built afresh, its
// maps and lists cost a sketch of 150 dimensioned segments a third of
// the drag's solve. Between two frames of a drag only the params' values
// change, so the shape is built from what this file gathers and is kept
// while that stays the same.

// entityPair is two points: an edge that holds its length, or two
// points made one.
type entityPair struct{ a, b core.HEntity }

// orderInputs is what a block's order shape is built from: its edges,
// its points made one by coincidence, its arcs and its points locked
// where they were dragged.
type orderInputs struct {
	edges, same []entityPair
	arcs        []*core.Entity
	locked      []core.HEntity
}

// orderShapeCache is the order shape last built, and the key of what it
// was built from (orderInputs.key).
type orderShapeCache struct {
	key   []byte
	shape *orderShape
}

// orderInputsOf gathers the order inputs of the block of groups hgs.
func (sk *Sketch) orderInputsOf(hgs []core.HGroup) *orderInputs {
	in := &orderInputs{}
	inBlock := func(hg core.HGroup) bool { return slices.Contains(hgs, hg) }
	isPoint := func(h core.HEntity) bool {
		e := sk.Entity(h)
		return e != nil && e.IsPoint()
	}
	segment := func(h core.HEntity) {
		if e := sk.Entity(h); e != nil && e.Type == core.EntityLineSegment {
			in.edges = append(in.edges, entityPair{e.Point[0], e.Point[1]})
		}
	}
	for ci := range sk.Constraints {
		c := &sk.Constraints[ci]
		// A locked point is held where its params are when the solve
		// starts, so moving it would move the lock.
		if c.Type == core.ConstraintWhereDragged {
			in.locked = append(in.locked, c.PtA)
		}
		if !inBlock(c.Group) || c.Reference {
			continue
		}
		switch c.Type {
		case core.ConstraintPointsCoincident:
			if isPoint(c.PtA) && isPoint(c.PtB) {
				in.same = append(in.same, entityPair{c.PtA, c.PtB})
			}
		case core.ConstraintPtPtDistance:
			if isPoint(c.PtA) && isPoint(c.PtB) {
				in.edges = append(in.edges, entityPair{c.PtA, c.PtB})
			}
		case core.ConstraintEqualLengthLines, core.ConstraintLengthRatio, core.ConstraintLengthDifference,
			core.ConstraintAngle, core.ConstraintPerpendicular:
			segment(c.EntityA)
			segment(c.EntityB)
		}
	}
	for ei := range sk.Entities {
		e := &sk.Entities[ei]
		if e.Type == core.EntityArcOfCircle && inBlock(e.Group) && !e.ForceHidden {
			in.arcs = append(in.arcs, e)
		}
	}
	return in
}

// key is everything the order shape of in, solved for params with the
// points moved, is built from, written out whole, so that two keys are
// equal only when the shapes would be: the block, its unknowns, the
// points moved and locked, the edges, coincidences and arcs, each
// point's kind, workplane and params, and each workplane's normal, to
// the bit, since a workplane solved in the block may turn.
func (in *orderInputs) key(sk *Sketch, hgs []core.HGroup, params []core.HParam, moved []core.HEntity) []byte {
	b := make([]byte, 0, 64*(len(in.edges)+len(in.same)+len(in.arcs))+8*len(params))
	u := func(v uint32) { b = binary.LittleEndian.AppendUint32(b, v) }
	f := func(v float64) { b = binary.LittleEndian.AppendUint64(b, math.Float64bits(v)) }
	list := func(n int, at func(i int) uint32) {
		u(uint32(n))
		for i := range n {
			u(at(i))
		}
	}
	list(len(hgs), func(i int) uint32 { return hgs[i].V })
	list(len(params), func(i int) uint32 { return params[i].V })
	list(len(moved), func(i int) uint32 { return moved[i].V })
	list(len(in.locked), func(i int) uint32 { return in.locked[i].V })
	var workplanes []core.HEntity
	point := func(h core.HEntity) {
		u(h.V)
		e := sk.Entity(h)
		if e == nil {
			u(0)
			return
		}
		u(uint32(e.Type))
		u(e.Workplane.V)
		for _, p := range e.Param[:3] {
			u(p.V)
		}
		if e.Type == core.EntityPointIn2D && !slices.Contains(workplanes, e.Workplane) {
			workplanes = append(workplanes, e.Workplane)
		}
	}
	pairs := func(ps []entityPair) {
		u(uint32(len(ps)))
		for _, p := range ps {
			point(p.a)
			point(p.b)
		}
	}
	pairs(in.edges)
	pairs(in.same)
	u(uint32(len(in.arcs)))
	for _, a := range in.arcs {
		u(a.H.V)
		for _, p := range a.Point[:3] {
			point(p)
		}
	}
	for _, wp := range workplanes {
		u(wp.V)
		if e := sk.Entity(wp); e != nil {
			n := e.NormalEntity(sk).NormalN(sk)
			f(n.X)
			f(n.Y)
			f(n.Z)
		}
	}
	return b
}
