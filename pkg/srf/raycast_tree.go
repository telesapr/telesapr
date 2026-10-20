package srf

import (
	"sync/atomic"

	"github.com/telesapr/telesapr/pkg/geom"
)

// rayTree is the subdivision AllPointsIntersectingUntrimmed makes of a
// curved surface, kept between the rays of a Boolean loop. Every ray
// cast at a surface split it the same way, into halves by u and v in
// turn down to pieces flat within the chord tolerance, and projected
// the centre of each flat piece onto the surface for the Newton guess,
// which was a third of regenerating a thread; the pieces' boxes, which
// halves are flat and their guesses depend only on the surface, so
// the first ray to reach a piece now makes its node and the rest read
// it. A ray walks the tree in the order the recursion split, with the
// same box test and the same cap on pieces visited, so it finds what
// the recursion found, in the same order.
//
// A node keeps its piece's control net only until the piece is split
// or found flat: a flat piece keeps the centre its guess is made from.
type rayTree struct {
	// What the tree was made from, so that a copy of the surface
	// since changed falls back to the recursion.
	degM, degN int
	ctrl       [MaxDegree + 1][MaxDegree + 1]geom.Vector
	weight     [MaxDegree + 1][MaxDegree + 1]float64
	tol        Tolerance
	root       rayNode
}

// rayNode is a piece of the surface: its box, and either that it is
// flat, with the guess a ray refines from, or its two halves, made by
// the first ray to need them.
type rayNode struct {
	max, min geom.Vector
	flat     bool
	// centre is a flat piece's, the mean of its corners, which its
	// guess is the surface's closest point to.
	centre geom.Vector
	// guess is a flat piece's, made by the first ray to reach it;
	// guessState says whether it is made (rayGuessMade), or being
	// made by that ray, which the others don't wait for.
	guess      geom.Point2d
	guessState atomic.Int32
	// piece is the control net of a piece not yet split, nil once it
	// is, or for a flat piece.
	piece atomic.Pointer[rayPiece]
	depth int
	kids  atomic.Pointer[[2]rayNode]
}

// The states of a node's guess.
const (
	rayGuessNone int32 = iota
	rayGuessMaking
	rayGuessMade
)

// rayPiece is a piece's control net, as many points as its degrees
// take, in one array: at[4*(i*(degN+1)+j)] is the surface's Ctrl[i][j]
// (x, y, z) and its Weight[i][j].
type rayPiece struct {
	at []float64
}

// rayCell holds a surface's tree; the root is filled by the first ray.
type rayCell struct {
	tree atomic.Pointer[rayTree]
}

// rayTreeFor is the surface's tree, made now if it wasn't, or nil if
// the surface has no cell or a tree made from it as it was.
func (s *SSurface) rayTreeFor() *rayTree {
	c := s.rays
	if c == nil {
		return nil
	}
	t := c.tree.Load()
	if t == nil {
		t = newRayTree(s)
		if !c.tree.CompareAndSwap(nil, t) {
			t = c.tree.Load()
		}
	}
	if t.degM != s.DegM || t.degN != s.DegN || t.ctrl != s.Ctrl || t.weight != s.Weight || t.tol != s.Tol {
		return nil
	}
	return t
}

// newRayTree makes the tree of s with its root node.
func newRayTree(s *SSurface) *rayTree {
	t := &rayTree{degM: s.DegM, degN: s.DegN, ctrl: s.Ctrl, weight: s.Weight, tol: s.Tol}
	t.fill(&t.root, s)
	return t
}

// fill sets the node's box and, for a flat piece, its centre, else
// keeps the piece's control net, from the piece as the recursion would.
func (t *rayTree) fill(n *rayNode, piece *SSurface) {
	piece.AxisAlignedBounding(&n.max, &n.min)
	if piece.DepartureFromCoplanar() < 0.2*piece.Tol.ChordTolMm() {
		n.flat = true
		n.centre = piece.Ctrl[0][0].Plus(piece.Ctrl[0][piece.DegN]).Plus(piece.Ctrl[piece.DegM][0]).
			Plus(piece.Ctrl[piece.DegM][piece.DegN]).ScaledBy(0.25)
		return
	}
	p := &rayPiece{at: make([]float64, 4*(t.degM+1)*(t.degN+1))}
	for i := 0; i <= t.degM; i++ {
		for j := 0; j <= t.degN; j++ {
			c := piece.Ctrl[i][j]
			k := 4 * (i*(t.degN+1) + j)
			p.at[k], p.at[k+1], p.at[k+2], p.at[k+3] = c.X, c.Y, c.Z, piece.Weight[i][j]
		}
	}
	n.piece.Store(p)
}

// surface is the piece as a surface, for splitting.
func (t *rayTree) surface(p *rayPiece) SSurface {
	s := SSurface{DegM: t.degM, DegN: t.degN, Tol: t.tol}
	for i := 0; i <= t.degM; i++ {
		for j := 0; j <= t.degN; j++ {
			k := 4 * (i*(t.degN+1) + j)
			s.Ctrl[i][j] = geom.Vector{X: p.at[k], Y: p.at[k+1], Z: p.at[k+2]}
			s.Weight[i][j] = p.at[k+3]
		}
	}
	return s
}

// guessFor is the flat node's guess, made now if it wasn't: the
// surface's closest point to the centre of the piece's corners. A ray
// that finds it being made by another makes its own, the same, and
// keeps it to itself.
func (t *rayTree) guessFor(n *rayNode, sorig *SSurface) geom.Point2d {
	if n.guessState.Load() == rayGuessMade {
		return n.guess
	}
	var g geom.Point2d
	sorig.ClosestPointTo(n.centre, &g, false)
	if n.guessState.CompareAndSwap(rayGuessNone, rayGuessMaking) {
		n.guess = g
		n.guessState.Store(rayGuessMade)
	}
	return g
}

// halves is the node's two halves, made now if they weren't.
func (t *rayTree) halves(n *rayNode) *[2]rayNode {
	if k := n.kids.Load(); k != nil {
		return k
	}
	p := n.piece.Load()
	if p == nil {
		// Split meanwhile by another ray, which let the net go once
		// the halves were in place.
		return n.kids.Load()
	}
	s := t.surface(p)
	var a, b SSurface
	s.SplitInHalf(n.depth&1 == 0, &a, &b)
	k := &[2]rayNode{{depth: n.depth + 1}, {depth: n.depth + 1}}
	t.fill(&k[0], &a)
	t.fill(&k[1], &b)
	if !n.kids.CompareAndSwap(nil, k) {
		// Another ray made them first; both made the same.
		return n.kids.Load()
	}
	n.piece.Store(nil)
	return k
}

// walk is AllPointsIntersectingUntrimmed over the tree from node n,
// whose depth is the recursion's level.
func (t *rayTree) walk(n *rayNode, a, b geom.Vector, cnt *int, l *[]surfInter, asSegment bool, sorig *SSurface) {
	if lineOutsideBox(n.max, n.min, a, b, asSegment) {
		return
	}
	if *cnt > 2000 {
		return
	}
	*cnt++
	if n.flat {
		inter := surfInter{p: t.guessFor(n, sorig)}
		if sorig.PointIntersectingLine(a, b, &inter.p.U, &inter.p.V) {
			*l = append(*l, inter)
		}
		return
	}
	k := t.halves(n)
	t.walk(&k[0], a, b, cnt, l, asSegment, sorig)
	t.walk(&k[1], a, b, cnt, l, asSegment, sorig)
}
