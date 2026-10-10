package check

import (
	"errors"
	"fmt"
	"math"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// moved is a solid placed as a copy: scaled by scale about the origin,
// then turned by q and moved by t. The map is affine, so a ray taken
// back into the solid's own place keeps its parameter: a crossing at s
// there is one at s here.
type moved struct {
	of    node
	t     geom.Vector
	q, qi geom.Quaternion
	scale float64
}

func newMoved(of node, pl sketch.CopyPlacement) *moved {
	return &moved{of: of, t: pl.Move.T, q: pl.Move.Q, qi: pl.Move.Q.Inverse(), scale: pl.Scale}
}

// back takes a point of the copy to the solid's own place.
func (m *moved) back(x geom.Vector) geom.Vector {
	return m.qi.Rotate(x.Minus(m.t)).ScaledBy(1 / m.scale)
}

func (m *moved) inside(x geom.Vector) bool { return m.of.inside(m.back(x)) }

func (m *moved) crossings(o, d geom.Vector, out []float64) []float64 {
	return m.of.crossings(m.back(o), m.qi.Rotate(d).ScaledBy(1/m.scale), out)
}

func (m *moved) bounds(lo, hi *geom.Vector) {
	// The copy of the solid's box holds the copy.
	blo := geom.Vector{X: math.Inf(1), Y: math.Inf(1), Z: math.Inf(1)}
	bhi := blo.ScaledBy(-1)
	m.of.bounds(&blo, &bhi)
	if blo.X > bhi.X {
		return
	}
	for k := range 8 {
		c := blo
		if k&1 != 0 {
			c.X = bhi.X
		}
		if k&2 != 0 {
			c.Y = bhi.Y
		}
		if k&4 != 0 {
			c.Z = bhi.Z
		}
		m.q.Rotate(c.ScaledBy(m.scale)).Plus(m.t).MakeMaxMin(hi, lo)
	}
}

// isCopies reports whether g makes its solid from copies of another's.
func isCopies(g *sketch.Group) bool {
	switch g.Type {
	case sketch.GroupTranslate, sketch.GroupRotate, sketch.GroupPathPattern, sketch.GroupMirror:
		return true
	}
	return false
}

// copiesNode is the solid a step and repeat, a pattern along a path or a
// mirror makes alone: its source's copies, united; and whether it is the
// whole solid, a mirror of the solid so far, rather than one to combine
// with what came before.
func copiesNode(sk *sketch.Sketch, g *sketch.Group) (n node, whole bool, err error) {
	places, ofSolid, err := g.CopyPlacements(sk)
	if err != nil {
		return nil, false, fmt.Errorf("%s: %w", g.DescriptionString(), err)
	}
	src := sk.Group(g.OpA)
	if src == nil {
		return nil, false, fmt.Errorf("%s: %w", g.DescriptionString(), errors.New("the group copied is gone"))
	}
	var of node
	if ofSolid {
		of, err = oracleNode(sk, src)
	} else {
		of, err = thisNode(sk, src)
	}
	if err != nil || of == nil {
		return nil, ofSolid, err
	}
	var copies union
	for _, pl := range places {
		copies = append(copies, newMoved(of, pl))
	}
	if len(copies) == 1 {
		return copies[0], ofSolid, nil
	}
	return copies, ofSolid, nil
}

// thisNode is the solid group g makes alone, before it is combined with
// the solid so far: what the kernel holds as its ThisShell.
func thisNode(sk *sketch.Sketch, g *sketch.Group) (node, error) {
	if len(g.ThisShell.Surface) == 0 && g.ThisMesh.IsEmpty() {
		return nil, nil
	}
	if isCopies(g) {
		n, _, err := copiesNode(sk, g)
		return n, err
	}
	return oracleLeaf(sk, g)
}
