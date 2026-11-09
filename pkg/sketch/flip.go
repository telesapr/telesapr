package sketch

import (
	"bytes"
	"math"
	"slices"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
)

// A dimension edit or a drag moves a sketch to another solution of its
// equations, and of the several a sketch has (a triangle given its three
// sides has two, mirror images) Newton takes whichever it comes to. The
// one meant is taken to be the one with the sketch's orientation: each
// corner turned the way it was. So after a solve that follows an edit
// or a drag, the corners and arcs are compared with those before it;
// a corner that turned over is put back by reflecting its point across
// the line of its two neighbours, the other root, and solving from there.

// minFlipSine is the sine of a corner's angle, about 3°, below which a
// corner is taken as straight, or folded flat, and so on neither side: a
// corner that passes through straight, as an arm's elbow does at full
// stretch, may come out either way.
const minFlipSine = 0.05

// maxCornerDegree bounds the edges at one point whose pairs are checked,
// so that a point with a hundred dimensions to it costs no more than one
// with this many.
const maxCornerDegree = 16

// orderShape is what a block's orientation is read from: its points,
// the coincident ones taken as one, the corners between two edges that
// hold their length, and its arcs.
type orderShape struct {
	// classes are the points that are one point, by coincidence
	// constraints; the first is the one read.
	classes [][]core.HEntity
	// movable is whether a class can be reflected: every point of it is
	// an unknown of the block, and none is being moved by the user.
	movable []bool
	// degree is how many edges a class has.
	degree  []int
	corners []orderCorner
	arcs    []orderArc
	// neighbours are, for each class, the classes joined to it by an
	// edge.
	neighbours [][]int
}

// orderCorner is the corner at b between the edges to a and to c, its
// sine read in the plane of normal n.
type orderCorner struct {
	a, b, c int
	n       geom.Vector
}

// orderArc is an arc and the classes of its centre, start and end.
type orderArc struct {
	h                    core.HEntity
	center, start, end   int
	startMoves, endMoves bool
}

// orientation is a block's orientation at one moment: the sine of each
// corner's angle, signed by its turn, and the sweep of each arc.
type orientation struct {
	sines  []float64
	sweeps []float64
}

// orderShapeOf is the order shape of the block of groups hgs, solved for
// params, whose points moved are those the user moves. A drag asks for
// it at every frame, and the shape changes only when what it is read
// from does (orderInputs.key), so the last one built is kept.
func (sk *Sketch) orderShapeOf(hgs []core.HGroup, params []core.HParam, moved []core.HEntity) *orderShape {
	in := sk.orderInputsOf(hgs)
	// Most of a drag's frames are of a sketch with nothing to keep, and
	// cost no more than the scan of its constraints and entities.
	if len(in.edges) == 0 && len(in.arcs) == 0 {
		return &orderShape{}
	}
	key := in.key(sk, hgs, params, moved)
	if c := sk.orderCache; c != nil && bytes.Equal(c.key, key) {
		return c.shape
	}
	o := sk.buildOrderShape(in, params, moved)
	sk.orderCache = &orderShapeCache{key: key, shape: o}
	return o
}

// buildOrderShape is the order shape of in, solved for params, whose
// points moved are those the user moves.
func (sk *Sketch) buildOrderShape(in *orderInputs, params []core.HParam, moved []core.HEntity) *orderShape {
	o := &orderShape{}
	index := make(map[core.HEntity]int)
	var parent []int
	find := func(i int) int {
		for parent[i] != i {
			parent[i] = parent[parent[i]]
			i = parent[i]
		}
		return i
	}
	class := func(h core.HEntity) int {
		if i, ok := index[h]; ok {
			return i
		}
		i := len(parent)
		index[h] = i
		parent = append(parent, i)
		return i
	}
	edges, same, arcs, locked := in.edges, in.same, in.arcs, in.locked
	for _, sm := range same {
		a, b := find(class(sm.a)), find(class(sm.b))
		parent[max(a, b)] = min(a, b)
	}
	for _, e := range arcs {
		o.arcs = append(o.arcs, orderArc{h: e.H, center: class(e.Point[0]), start: class(e.Point[1]), end: class(e.Point[2])})
	}
	for _, ed := range edges {
		class(ed.a)
		class(ed.b)
	}
	// The classes, numbered by their roots in the order first met.
	root := make([]int, len(parent))
	byRoot := make(map[int]int)
	handles := make([]core.HEntity, len(parent))
	for h, i := range index {
		handles[i] = h
	}
	for i := range parent {
		r := find(i)
		k, ok := byRoot[r]
		if !ok {
			k = len(o.classes)
			byRoot[r] = k
			o.classes = append(o.classes, nil)
		}
		root[i] = k
		o.classes[k] = append(o.classes[k], handles[i])
	}
	for k := range o.classes {
		slices.SortFunc(o.classes[k], func(a, b core.HEntity) int { return int(a.V) - int(b.V) })
	}
	for i := range o.arcs {
		a := &o.arcs[i]
		a.center, a.start, a.end = root[a.center], root[a.start], root[a.end]
	}

	unknown := make(map[core.HParam]bool, len(params))
	for _, h := range params {
		unknown[h] = true
	}
	o.movable = make([]bool, len(o.classes))
	for k, members := range o.classes {
		o.movable[k] = true
		for _, h := range members {
			if slices.Contains(moved, h) || slices.Contains(locked, h) || !pointUnknown(sk.Entity(h), unknown) {
				o.movable[k] = false
				break
			}
		}
	}
	for i := range o.arcs {
		a := &o.arcs[i]
		a.startMoves, a.endMoves = o.movable[a.start], o.movable[a.end]
	}

	o.neighbours = make([][]int, len(o.classes))
	for _, ed := range edges {
		a, b := root[index[ed.a]], root[index[ed.b]]
		if a == b {
			continue
		}
		if !slices.Contains(o.neighbours[a], b) {
			o.neighbours[a] = append(o.neighbours[a], b)
			o.neighbours[b] = append(o.neighbours[b], a)
		}
	}
	o.degree = make([]int, len(o.classes))
	for b, ns := range o.neighbours {
		o.degree[b] = len(ns)
		n, ok := sk.pointNormal(o.classes[b][0])
		if !ok {
			continue
		}
		ns = ns[:min(len(ns), maxCornerDegree)]
		for i := range ns {
			for j := i + 1; j < len(ns); j++ {
				o.corners = append(o.corners, orderCorner{a: ns[i], b: b, c: ns[j], n: n})
			}
		}
	}
	return o
}

// pointUnknown reports whether every param of point e is one of the
// unknowns: a point drawn in a workplane or in 3d, solved by the block.
func pointUnknown(e *core.Entity, unknown map[core.HParam]bool) bool {
	if e == nil {
		return false
	}
	n := 0
	switch e.Type {
	case core.EntityPointIn2D:
		n = 2
	case core.EntityPointIn3D:
		n = 3
	default:
		return false
	}
	for _, h := range e.Param[:n] {
		if !unknown[h] {
			return false
		}
	}
	return true
}

// pointNormal is the normal of the workplane point he is drawn in; a
// point in 3d has no plane to turn in.
func (sk *Sketch) pointNormal(he core.HEntity) (geom.Vector, bool) {
	e := sk.Entity(he)
	if e == nil || e.Type != core.EntityPointIn2D {
		return geom.Vector{}, false
	}
	wp := sk.Entity(e.Workplane)
	if wp == nil {
		return geom.Vector{}, false
	}
	return wp.NormalEntity(sk).NormalN(sk), true
}

// pos is where class k is.
func (o *orderShape) pos(sk *Sketch, k int) geom.Vector {
	return sk.MustEntity(o.classes[k][0]).PointGetNum(sk)
}

// orientation reads the block's orientation where it is now.
func (o *orderShape) orientation(sk *Sketch) orientation {
	or := orientation{sines: make([]float64, len(o.corners)), sweeps: make([]float64, len(o.arcs))}
	for i, c := range o.corners {
		b := o.pos(sk, c.b)
		u, v := o.pos(sk, c.a).Minus(b), o.pos(sk, c.c).Minus(b)
		if l := u.Magnitude() * v.Magnitude(); l > 0 {
			or.sines[i] = u.Cross(v).Dot(c.n) / l
		}
	}
	for i, a := range o.arcs {
		_ = catchPanic(func() { _, _, or.sweeps[i] = sk.MustEntity(a.h).ArcGetAngles(sk) })
	}
	return or
}

// update takes the orientation now as the one to compare the next with,
// except for a corner that is straight now (minFlipSine), which keeps the
// side it was last on: a corner that passes through straight, as an
// arm's elbow folded flat by a drag across its hinge, is to come out on
// the side it went in. An arc shrunk to nothing keeps its last sweep
// likewise.
func (or *orientation) update(now orientation) {
	for i, s := range now.sines {
		if math.Abs(s) > minFlipSine {
			or.sines[i] = s
		}
	}
	for i, s := range now.sweeps {
		if sweepOriented(s) {
			or.sweeps[i] = s
		}
	}
}

// sweepOriented reports whether an arc of sweep s is on a side of its
// start: not shrunk to nothing, nor a whole turn, as an arc just drawn
// with its end still on its start.
func sweepOriented(s float64) bool {
	return s > minFlipSine && s < 2*math.Pi-minFlipSine
}

// oriented reports whether any corner of the orientation is on a side,
// not straight, or any arc's sweep less than a whole turn: whether there
// is an orientation to keep.
func (or orientation) oriented() bool {
	return slices.ContainsFunc(or.sines, func(s float64) bool { return math.Abs(s) > minFlipSine }) ||
		slices.ContainsFunc(or.sweeps, sweepOriented)
}

// turnedOver are the corners and arcs of after that turned over from
// before: a corner whose sine changed sign, both well away from
// straight, and an arc whose sweep went past its start, more than half
// a turn in one solve, from a sweep that was not a whole turn.
func turnedOver(before, after orientation) (corners, arcs []int) {
	for i, s := range before.sines {
		t := after.sines[i]
		if s*t < 0 && math.Abs(s) > minFlipSine && math.Abs(t) > minFlipSine {
			corners = append(corners, i)
		}
	}
	for i, s := range before.sweeps {
		if sweepOriented(s) && math.Abs(after.sweeps[i]-s) > math.Pi {
			arcs = append(arcs, i)
		}
	}
	return corners, arcs
}

// reflectTurned puts back, where the block is now, a point of each
// corner and arc that turned over from last (turnedOver). A corner's own
// point is reflected across the line of the two it is held to, the other
// root of a point that two lengths place: the one with fewest edges
// first, and no point joined by an edge to one already reflected, whose
// own corner the reflection moved. An arc's end, or its start where the
// end cannot move, is turned about the centre to the sweep it had, so
// that an end dragged past the start takes the start along. It reports
// whether it moved any.
func (o *orderShape) reflectTurned(sk *Sketch, last orientation, corners, arcs []int) bool {
	type move struct {
		k  int
		to geom.Vector
	}
	var moves []move
	done := make(map[int]bool)
	byDegree := slices.Clone(corners)
	slices.SortStableFunc(byDegree, func(i, j int) int { return o.degree[o.corners[i].b] - o.degree[o.corners[j].b] })
	for _, i := range byDegree {
		c := o.corners[i]
		if !o.movable[c.b] || done[c.b] || slices.ContainsFunc(o.neighbours[c.b], func(n int) bool { return done[n] }) {
			continue
		}
		done[c.b] = true
		moves = append(moves, move{c.b, reflectAcross(o.pos(sk, c.b), o.pos(sk, c.a), o.pos(sk, c.c), c.n)})
	}
	for _, i := range arcs {
		a := o.arcs[i]
		n := sk.MustEntity(a.h).NormalEntity(sk).NormalN(sk)
		p, other, turn := a.end, a.start, last.sweeps[i]
		if !a.endMoves {
			p, other, turn = a.start, a.end, -turn
		}
		if !o.movable[p] || done[p] {
			continue
		}
		done[p] = true
		moves = append(moves, move{p, turnAbout(o.pos(sk, other), o.pos(sk, a.center), n, turn)})
	}
	for _, m := range moves {
		for _, h := range o.classes[m.k] {
			sk.MustEntity(h).PointForceTo(sk, m.to)
		}
	}
	return len(moves) > 0
}

// turnAbout is p turned by angle about the axis through c along the unit
// vector n, anticlockwise looking down n.
func turnAbout(p, c, n geom.Vector, angle float64) geom.Vector {
	v := p.Minus(c)
	cos, sin := math.Cos(angle), math.Sin(angle)
	v = v.ScaledBy(cos).Plus(n.Cross(v).ScaledBy(sin)).Plus(n.ScaledBy(n.Dot(v) * (1 - cos)))
	return c.Plus(v)
}

// reflectAcross is p reflected across the line through a and b, in the
// plane of normal n.
func reflectAcross(p, a, b, n geom.Vector) geom.Vector {
	m := n.Cross(b.Minus(a))
	if m.Magnitude() == 0 {
		return p
	}
	m = m.WithMagnitude(1)
	return p.Minus(m.ScaledBy(2 * p.Minus(a).Dot(m)))
}

// turnedNames names the points of the corners and arcs that turned
// over, for the note on the group.
func (o *orderShape) turnedNames(sk *Sketch, corners, arcs []int) []string {
	var names []string
	add := func(n string) {
		if !slices.Contains(names, n) {
			names = append(names, n)
		}
	}
	for _, i := range corners {
		add("the corner at " + sk.EntityName(o.classes[o.corners[i].b][0]))
	}
	for _, i := range arcs {
		add(sk.EntityName(o.arcs[i].h))
	}
	return names
}
