package srf

import (
	"math"

	"github.com/spacesolve/spacesolve/pkg/geom"
)

// This file ports solvespace/src/bsp.cpp: binary space partitioning
// trees, used to represent a volume in 3-space bounded by a triangle
// mesh. These are used to compute Boolean operations on meshes. These
// aren't used for anything relating to an SShell of ratpoly surfaces.

// BspClass is the side of a BSP node's plane something is on (C++
// BspClass).
type BspClass uint32

const (
	BspPos      BspClass = 100
	BspNeg      BspClass = 101
	BspCoplanar BspClass = 200
)

// SBsp3 is a node of a 3d BSP: the plane n . p = d of its triangle
// tri, the other triangles in that plane (more), and the edges of the
// triangles in that plane, as a 2d BSP (C++ SBsp3).
type SBsp3 struct {
	N geom.Vector
	D float64

	Tri STriangle
	Pos *SBsp3
	Neg *SBsp3

	More *SBsp3

	Edges *SBsp2
}

// SBsp3FromMesh builds the BSP of mesh m, inserting its triangles in
// the shuffled order C++ does (C++ SBsp3::FromMesh).
func SBsp3FromMesh(m *SMesh) *SBsp3 {
	mc := make([]STriangle, len(m.Triangles))
	copy(mc, m.Triangles)

	rnd := newGlibcRand(0) // Let's be deterministic, at least!
	n := len(mc)
	for n > 1 {
		k := rnd.rand() % n
		n--
		mc[k], mc[n] = mc[n], mc[k]
	}

	var bsp3 *SBsp3
	for i := range mc {
		bsp3 = SBsp3InsertOrCreate(bsp3, &mc[i], nil)
	}
	return bsp3
}

// IntersectionWith is where the segment from a to b, which must cross
// the node's plane, crosses it (C++ SBsp3::IntersectionWith).
func (b *SBsp3) IntersectionWith(a, bb geom.Vector) geom.Vector {
	da := a.Dot(b.N) - b.D
	db := bb.Dot(b.N) - b.D
	if da*db >= 0 {
		panic("Expected segment to intersect BSP node")
	}

	dab := db - da
	return a.ScaledBy(db / dab).Plus(bb.ScaledBy(-da / dab))
}

// InsertInPlane adds tr, which lies in the node's plane, to m if the
// Boolean keeps it: by whether a triangle in the plane covers its
// centroid, and on which side (C++ SBsp3::InsertInPlane).
func (b *SBsp3) InsertInPlane(pos2 bool, tr *STriangle, m *SMesh) {
	tc := tr.A.Plus(tr.B).Plus(tr.C).ScaledBy(1.0 / 3)

	onFace := false
	sameNormal := false
	maxNormalMag := -1.0

	trn := tr.Normal()

	for ll := b; ll != nil; ll = ll.More {
		if ll.Tri.ContainsPoint(tc) {
			onFace = true
			// If the mesh contains almost-zero-area triangles, and we're
			// just on the edge of one of those, then don't trust its normal.
			lln := ll.Tri.Normal()
			if lln.Magnitude() > maxNormalMag {
				sameNormal = trn.Dot(lln) > 0
				maxNormalMag = lln.Magnitude()
			}
		}
	}

	if (!onFace && ((m.KeepInsideOtherShell && !pos2) ||
		(!m.KeepInsideOtherShell && pos2))) ||
		(onFace && ((m.KeepCoplanar && m.FlipNormal && !sameNormal) ||
			(m.KeepCoplanar && !m.FlipNormal && sameNormal))) {
		// We have decided that we need to keep a triangle either inside,
		// outside or on the other shell. So add it and flip it if requested.
		if !m.FlipNormal {
			m.AddTriangle3(tr.Meta, tr.A, tr.B, tr.C)
		} else {
			m.AddTriangle3(tr.Meta, tr.C, tr.B, tr.A)
		}
	} else {
		m.AtLeastOneDiscarded = true
	}
}

// InsertHow inserts tr on the given side of the node, or, when instead
// is set and the tree ends there, decides whether the Boolean keeps it
// (C++ SBsp3::InsertHow).
func (b *SBsp3) InsertHow(how BspClass, tr *STriangle, instead *SMesh) {
	switch how {
	case BspPos:
		if instead != nil && b.Pos == nil {
			b.insertAlt(how, tr, instead)
			return
		}
		b.Pos = SBsp3InsertOrCreate(b.Pos, tr, instead)

	case BspNeg:
		if instead != nil && b.Neg == nil {
			b.insertAlt(how, tr, instead)
			return
		}
		b.Neg = SBsp3InsertOrCreate(b.Neg, tr, instead)

	case BspCoplanar:
		if instead != nil {
			b.insertAlt(how, tr, instead)
			return
		}
		m := &SBsp3{N: b.N, D: b.D, Tri: *tr, More: b.More}
		b.More = m
	}
}

// insertAlt is InsertHow's alt label.
func (b *SBsp3) insertAlt(how BspClass, tr *STriangle, instead *SMesh) {
	if (how == BspPos && !instead.KeepInsideOtherShell) ||
		(how == BspNeg && instead.KeepInsideOtherShell) {
		// We have decided that we need to keep a triangle (either inside or
		// outside the other shell. So add it and flip it if requested.
		if !instead.FlipNormal {
			instead.AddTriangle3(tr.Meta, tr.A, tr.B, tr.C)
		} else {
			instead.AddTriangle3(tr.Meta, tr.C, tr.B, tr.A)
		}
	} else if how == BspCoplanar {
		if b.Edges != nil {
			b.Edges.InsertTriangle(tr, instead, b)
		} else {
			// I suppose this actually is allowed to happen, if the coplanar
			// face is the leaf, and all of its neighbors are earlier in tree?
			b.InsertInPlane(false, tr, instead)
		}
	} else {
		instead.AtLeastOneDiscarded = true
	}
}

// bspUtil classifies and splits a triangle or convex polygon against a
// node's plane (C++ BspUtil).
type bspUtil struct {
	bsp *SBsp3

	onc, posc, negc    int
	isPos, isNeg, isOn []bool

	// triangle operations
	tr   *STriangle
	btri STriangle // also as alone
	ctri STriangle

	// convex operations
	on         [2]geom.Vector
	vpos, vneg []geom.Vector // vpos also as quad
}

func (u *bspUtil) classifyTriangle(tri *STriangle, node *SBsp3) {
	u.tr = tri
	u.bsp = node
	u.onc, u.posc, u.negc = 0, 0, 0

	u.isPos, u.isNeg, u.isOn = make([]bool, 3), make([]bool, 3), make([]bool, 3)

	dt := [3]float64{u.tr.A.Dot(u.bsp.N), u.tr.B.Dot(u.bsp.N), u.tr.C.Dot(u.bsp.N)}
	d := u.bsp.D
	// Count vertices in the plane
	for i := 0; i < 3; i++ {
		if dt[i] > d+geom.LengthEps {
			u.posc++
			u.isPos[i] = true
		} else if dt[i] < d-geom.LengthEps {
			u.negc++
			u.isNeg[i] = true
		} else {
			u.onc++
			u.isOn[i] = true
		}
	}
}

func (u *bspUtil) classifyConvex(vertex []geom.Vector, node *SBsp3, insertEdge bool) bool {
	u.bsp = node
	u.onc, u.posc, u.negc = 0, 0, 0

	cnt := len(vertex)
	u.isPos, u.isNeg, u.isOn = make([]bool, cnt), make([]bool, cnt), make([]bool, cnt)

	for i := 0; i < cnt; i++ {
		dt := u.bsp.N.Dot(vertex[i])
		if math.Abs(dt-u.bsp.D) < geom.LengthEps {
			u.isOn[i] = true
			if u.onc < 2 {
				u.on[u.onc] = vertex[i]
			}
			u.onc++
		} else if dt > u.bsp.D {
			u.isPos[i] = true
			u.posc++
		} else {
			u.isNeg[i] = true
			u.negc++
		}
	}

	if u.onc != 2 && u.onc != 1 && u.onc != 0 {
		return false
	}
	if u.onc == 2 {
		if insertEdge {
			e01 := vertex[1].Minus(vertex[0])
			e12 := vertex[2].Minus(vertex[1])
			out := e01.Cross(e12)
			se := SEdge{A: u.on[0], B: u.on[1]}
			u.bsp.Edges = SBsp2InsertOrCreateEdge(u.bsp.Edges, &se, u.bsp.N, out)
		}
	}
	return true
}

func (u *bspUtil) classifyConvexVertices(vertex []geom.Vector, insertEdges bool) bool {
	var inter [2]geom.Vector
	inters := 0

	cnt := len(vertex)
	// Enlarge vertices list to consider two intersections
	u.vpos = make([]geom.Vector, 0, cnt+4)
	u.vneg = make([]geom.Vector, 0, cnt+4)

	for i := 0; i < cnt; i++ {
		ip := (i + 1) % cnt

		if u.isPos[i] {
			u.vpos = append(u.vpos, vertex[i])
		}
		if u.isNeg[i] {
			u.vneg = append(u.vneg, vertex[i])
		}
		if u.isOn[i] {
			u.vneg = append(u.vneg, vertex[i])
			u.vpos = append(u.vpos, vertex[i])
		}
		if (u.isPos[i] && u.isNeg[ip]) || (u.isNeg[i] && u.isPos[ip]) {
			vi := u.bsp.IntersectionWith(vertex[i], vertex[ip])
			u.vpos = append(u.vpos, vi)
			u.vneg = append(u.vneg, vi)

			if inters >= 2 {
				return false // triangulate: XXX shouldn't happen but does
			}
			inter[inters] = vi
			inters++
		}
	}
	if len(u.vpos) > cnt+1 || len(u.vneg) > cnt+1 {
		panic("Impossible")
	}

	if insertEdges {
		e01 := vertex[1].Minus(vertex[0])
		e12 := vertex[2].Minus(vertex[1])
		out := e01.Cross(e12)
		if inters == 2 {
			se := SEdge{A: inter[0], B: inter[1]}
			u.bsp.Edges = SBsp2InsertOrCreateEdge(u.bsp.Edges, &se, u.bsp.N, out)
		} else if inters == 1 && u.onc == 1 {
			se := SEdge{A: inter[0], B: u.on[0]}
			u.bsp.Edges = SBsp2InsertOrCreateEdge(u.bsp.Edges, &se, u.bsp.N, out)
		} else if inters == 0 && u.onc == 2 {
			// We already handled this on-plane existing edge
		} else {
			return false //triangulate;
		}
	}
	if len(u.vneg) < 3 || len(u.vpos) < 3 {
		return false // triangulate; // XXX
	}

	return true
}

func (u *bspUtil) processEdgeInsert() {
	if u.onc != 2 {
		panic("Impossible")
	}

	var a, b geom.Vector
	switch {
	case !u.isOn[0]:
		a, b = u.tr.B, u.tr.C
	case !u.isOn[1]:
		a, b = u.tr.C, u.tr.A
	case !u.isOn[2]:
		a, b = u.tr.A, u.tr.B
	default:
		panic("Impossible")
	}

	se := SEdge{A: a, B: b}
	u.bsp.Edges = SBsp2InsertOrCreateEdge(u.bsp.Edges, &se, u.bsp.N, u.tr.Normal())
}

func (u *bspUtil) splitIntoTwoTriangles(insertEdge bool) bool {
	if u.posc != 1 || u.negc != 1 || u.onc != 1 {
		panic("Impossible")
	}

	var bpos bool
	var a, b, c geom.Vector

	// Standardize so that a is on the plane
	switch {
	case u.isOn[0]:
		a, b, c = u.tr.A, u.tr.B, u.tr.C
		bpos = u.isPos[1]
	case u.isOn[1]:
		a, b, c = u.tr.B, u.tr.C, u.tr.A
		bpos = u.isPos[2]
	case u.isOn[2]:
		a, b, c = u.tr.C, u.tr.A, u.tr.B
		bpos = u.isPos[0]
	default:
		panic("Impossible")
	}

	bPc := u.bsp.IntersectionWith(b, c)
	u.btri = STriangleFrom(u.tr.Meta, a, b, bPc)
	u.ctri = STriangleFrom(u.tr.Meta, c, a, bPc)

	if insertEdge {
		se := SEdge{A: a, B: bPc}
		u.bsp.Edges = SBsp2InsertOrCreateEdge(u.bsp.Edges, &se, u.bsp.N, u.tr.Normal())
	}

	return bpos
}

func (u *bspUtil) splitIntoTwoPieces(insertEdge bool) bool {
	var a, b, c geom.Vector
	if u.posc == 2 && u.negc == 1 {
		// Standardize so that a is on one side, and b and c are on the other.
		switch {
		case u.isNeg[0]:
			a, b, c = u.tr.A, u.tr.B, u.tr.C
		case u.isNeg[1]:
			a, b, c = u.tr.B, u.tr.C, u.tr.A
		case u.isNeg[2]:
			a, b, c = u.tr.C, u.tr.A, u.tr.B
		default:
			panic("Impossible")
		}
	} else if u.posc == 1 && u.negc == 2 {
		switch {
		case u.isPos[0]:
			a, b, c = u.tr.A, u.tr.B, u.tr.C
		case u.isPos[1]:
			a, b, c = u.tr.B, u.tr.C, u.tr.A
		case u.isPos[2]:
			a, b, c = u.tr.C, u.tr.A, u.tr.B
		default:
			panic("Impossible")
		}
	} else {
		panic("Impossible")
	}

	aPb := u.bsp.IntersectionWith(a, b)
	cPa := u.bsp.IntersectionWith(c, a)

	u.btri = STriangleFrom(u.tr.Meta, a, aPb, cPa)

	u.vpos = []geom.Vector{aPb, b, c, cPa}

	if insertEdge {
		se := SEdge{A: aPb, B: cPa}
		u.bsp.Edges = SBsp2InsertOrCreateEdge(u.bsp.Edges, &se, u.bsp.N, u.btri.Normal())
	}

	return u.posc == 2 && u.negc == 1
}

func bspTriangulate(bsp *SBsp3, meta STriMeta, vertex []geom.Vector, instead *SMesh) *SBsp3 {
	for i := 0; i < len(vertex)-2; i++ {
		tr := STriangleFrom(meta, vertex[0], vertex[i+1], vertex[i+2])
		bsp = SBsp3InsertOrCreate(bsp, &tr, instead)
	}
	return bsp
}

// InsertConvexHow inserts a convex polygon on the given side of the node
// (C++ SBsp3::InsertConvexHow).
func (b *SBsp3) InsertConvexHow(how BspClass, meta STriMeta, vertex []geom.Vector, instead *SMesh) {
	switch how {
	case BspPos:
		if b.Pos != nil {
			b.Pos = b.Pos.InsertConvex(meta, vertex, instead)
			return
		}
	case BspNeg:
		if b.Neg != nil {
			b.Neg = b.Neg.InsertConvex(meta, vertex, instead)
			return
		}
	default:
		panic("Unexpected BSP insert type")
	}

	for i := 0; i < len(vertex)-2; i++ {
		tr := STriangleFrom(meta, vertex[0], vertex[i+1], vertex[i+2])
		b.InsertHow(how, &tr, instead)
	}
}

// InsertConvex inserts a convex polygon, splitting it against the
// node's plane as a polygon where it can (C++ SBsp3::InsertConvex).
func (b *SBsp3) InsertConvex(meta STriMeta, vertex []geom.Vector, instead *SMesh) *SBsp3 {
	u := &bspUtil{}
	if u.classifyConvex(vertex, b, instead == nil) {
		if u.posc == 0 {
			b.InsertConvexHow(BspNeg, meta, vertex, instead)
			return b
		}
		if u.negc == 0 {
			b.InsertConvexHow(BspPos, meta, vertex, instead)
			return b
		}

		if u.classifyConvexVertices(vertex, instead == nil) {
			b.InsertConvexHow(BspNeg, meta, u.vneg, instead)
			b.InsertConvexHow(BspPos, meta, u.vpos, instead)
			return b
		}
	}

	// We don't handle the special case for this; do it as triangles
	return bspTriangulate(b, meta, vertex, instead)
}

// SBsp3InsertOrCreate inserts tr into the tree at where, making a node
// of it if the tree ends there; with instead, the triangle instead goes
// into (or is discarded from) that mesh (C++ SBsp3::InsertOrCreate).
func SBsp3InsertOrCreate(where *SBsp3, tr *STriangle, instead *SMesh) *SBsp3 {
	if where == nil {
		if instead != nil {
			// ruevs: I do not think this code is reachable, but in
			// principle should we use instead->keepInsideOtherShell
			// in place of instead->flipNormal ?
			if instead.FlipNormal {
				instead.AtLeastOneDiscarded = true
			} else {
				instead.AddTriangle3(tr.Meta, tr.A, tr.B, tr.C)
			}
			return nil
		}

		// Brand new node; so allocate for it, and fill us in.
		r := &SBsp3{}
		r.N = tr.Normal().WithMagnitude(1)
		r.D = tr.A.Dot(r.N)
		r.Tri = *tr
		return r
	}
	where.Insert(tr, instead)
	return where
}

// Insert inserts tr below this node, splitting it against the node's
// plane (C++ SBsp3::Insert).
func (b *SBsp3) Insert(tr *STriangle, instead *SMesh) {
	u := &bspUtil{}
	u.classifyTriangle(tr, b)

	// All vertices in-plane
	if u.onc == 3 {
		b.InsertHow(BspCoplanar, tr, instead)
		return
	}

	// No split required
	if u.posc == 0 || u.negc == 0 {
		if instead == nil && u.onc == 2 {
			u.processEdgeInsert()
		}

		if u.posc > 0 {
			b.InsertHow(BspPos, tr, instead)
		} else {
			b.InsertHow(BspNeg, tr, instead)
		}
		return
	}

	// The polygon must be split into two triangles, one above, one below.
	if u.posc == 1 && u.negc == 1 && u.onc == 1 {
		if u.splitIntoTwoTriangles(instead == nil) {
			b.InsertHow(BspPos, &u.btri, instead)
			b.InsertHow(BspNeg, &u.ctri, instead)
		} else {
			b.InsertHow(BspPos, &u.ctri, instead)
			b.InsertHow(BspNeg, &u.btri, instead)
		}
		return
	}

	// The polygon must be split into two pieces: a triangle and a quad.
	if u.splitIntoTwoPieces(instead == nil) {
		b.InsertConvexHow(BspPos, tr.Meta, u.vpos, instead)
		b.InsertHow(BspNeg, &u.btri, instead)
	} else {
		b.InsertConvexHow(BspNeg, tr.Meta, u.vpos, instead)
		b.InsertHow(BspPos, &u.btri, instead)
	}
}

// GenerateInPaintOrder lists the tree's triangles back to front along z
// (C++ SBsp3::GenerateInPaintOrder).
func (b *SBsp3) GenerateInPaintOrder(m *SMesh) {
	// Doesn't matter which branch we take if the normal has zero z
	// component, so don't need a separate case for that.
	if b.N.Z < 0 {
		if b.Pos != nil {
			b.Pos.GenerateInPaintOrder(m)
		}
	} else {
		if b.Neg != nil {
			b.Neg.GenerateInPaintOrder(m)
		}
	}

	for flip := b; flip != nil; flip = flip.More {
		m.AddTriangle(flip.Tri)
	}

	if b.N.Z < 0 {
		if b.Neg != nil {
			b.Neg.GenerateInPaintOrder(m)
		}
	} else {
		if b.Pos != nil {
			b.Pos.GenerateInPaintOrder(m)
		}
	}
}

// SBsp2 is a 2d BSP over the edges of the triangles in a 3d node's
// plane: the line through edge, in the plane with normal np, with
// outward normal no, at no . p = d (C++ SBsp2).
type SBsp2 struct {
	NP   geom.Vector // normal to the plane
	NO   geom.Vector // outer normal to the edge
	D    float64
	Edge SEdge

	Pos *SBsp2
	Neg *SBsp2

	More *SBsp2
}

// IntersectionWith is where the segment from a to b, which must cross
// the node's line, crosses it (C++ SBsp2::IntersectionWith).
func (b *SBsp2) IntersectionWith(a, bb geom.Vector) geom.Vector {
	da := a.Dot(b.NO) - b.D
	db := bb.Dot(b.NO) - b.D
	if da*db >= 0 {
		panic("Expected segment to intersect BSP node")
	}

	dab := db - da
	return a.ScaledBy(db / dab).Plus(bb.ScaledBy(-da / dab))
}

// newSBsp2 is the node for nedge, in the plane with normal nnp, with
// its outward normal on the side of out.
func newSBsp2(nedge *SEdge, nnp, out geom.Vector) *SBsp2 {
	r := &SBsp2{}
	r.NP = nnp
	r.NO = r.NP.Cross(nedge.B.Minus(nedge.A)).WithMagnitude(1)
	if out.Dot(r.NO) < 0 {
		r.NO = r.NO.ScaledBy(-1)
	}
	r.D = nedge.A.Dot(r.NO)
	r.Edge = *nedge
	return r
}

// SBsp2InsertOrCreateEdge inserts nedge into the tree at where, making
// a node of it if the tree ends there (C++ SBsp2::InsertOrCreateEdge).
func SBsp2InsertOrCreateEdge(where *SBsp2, nedge *SEdge, nnp, out geom.Vector) *SBsp2 {
	if where == nil {
		// Brand new node; so allocate for it, and fill us in.
		return newSBsp2(nedge, nnp, out)
	}
	where.InsertEdge(nedge, nnp, out)
	return where
}

// InsertEdge inserts nedge below this node, splitting it against the
// node's line (C++ SBsp2::InsertEdge).
func (b *SBsp2) InsertEdge(nedge *SEdge, nnp, out geom.Vector) {
	dt := [2]float64{nedge.A.Dot(b.NO), nedge.B.Dot(b.NO)}

	var isPos, isNeg, isOn [2]bool
	for i := 0; i < 2; i++ {
		if math.Abs(dt[i]-b.D) < geom.LengthEps {
			isOn[i] = true
		} else if dt[i] > b.D {
			isPos[i] = true
		} else {
			isNeg[i] = true
		}
	}

	if (isPos[0] && isPos[1]) || (isPos[0] && isOn[1]) || (isOn[0] && isPos[1]) {
		b.Pos = SBsp2InsertOrCreateEdge(b.Pos, nedge, nnp, out)
		return
	}
	if (isNeg[0] && isNeg[1]) || (isNeg[0] && isOn[1]) || (isOn[0] && isNeg[1]) {
		b.Neg = SBsp2InsertOrCreateEdge(b.Neg, nedge, nnp, out)
		return
	}
	if isOn[0] && isOn[1] {
		m := newSBsp2(nedge, nnp, out)
		m.More = b.More
		b.More = m
		return
	}
	if (isPos[0] && isNeg[1]) || (isNeg[0] && isPos[1]) {
		aPb := b.IntersectionWith(nedge.A, nedge.B)

		ea := SEdge{A: nedge.A, B: aPb}
		eb := SEdge{A: aPb, B: nedge.B}

		if isPos[0] {
			b.Pos = SBsp2InsertOrCreateEdge(b.Pos, &ea, nnp, out)
			b.Neg = SBsp2InsertOrCreateEdge(b.Neg, &eb, nnp, out)
		} else {
			b.Neg = SBsp2InsertOrCreateEdge(b.Neg, &ea, nnp, out)
			b.Pos = SBsp2InsertOrCreateEdge(b.Pos, &eb, nnp, out)
		}
		return
	}
	panic("Impossible")
}

// InsertTriangleHow passes tr down the given side, or, where the tree
// ends, decides it against the 3d node's plane (C++
// SBsp2::InsertTriangleHow).
func (b *SBsp2) InsertTriangleHow(how BspClass, tr *STriangle, m *SMesh, bsp3 *SBsp3) {
	switch how {
	case BspPos:
		if b.Pos != nil {
			b.Pos.InsertTriangle(tr, m, bsp3)
		} else {
			bsp3.InsertInPlane(true, tr, m)
		}
	case BspNeg:
		if b.Neg != nil {
			b.Neg.InsertTriangle(tr, m, bsp3)
		} else {
			bsp3.InsertInPlane(false, tr, m)
		}
	default:
		panic("Unexpected BSP insert type")
	}
}

// InsertTriangle classifies tr, which lies in the 3d node's plane,
// against the edges in that plane, splitting it as needed (C++
// SBsp2::InsertTriangle).
func (b *SBsp2) InsertTriangle(tr *STriangle, m *SMesh, bsp3 *SBsp3) {
	dt := [3]float64{tr.A.Dot(b.NO), tr.B.Dot(b.NO), tr.C.Dot(b.NO)}

	var isPos, isNeg, isOn [3]bool
	inc, posc, negc := 0, 0, 0
	for i := 0; i < 3; i++ {
		if math.Abs(dt[i]-b.D) < geom.LengthEps {
			isOn[i] = true
			inc++
		} else if dt[i] > b.D {
			isPos[i] = true
			posc++
		} else {
			isNeg[i] = true
			negc++
		}
	}

	if inc == 3 {
		// All vertices on-line; so it's a degenerate triangle, to ignore.
		return
	}

	// No split required
	if posc == 0 || negc == 0 {
		if posc > 0 {
			b.InsertTriangleHow(BspPos, tr, m, bsp3)
		} else {
			b.InsertTriangleHow(BspNeg, tr, m, bsp3)
		}
		return
	}

	// The polygon must be split into two pieces, one above, one below.
	var a, bv, c geom.Vector

	if posc == 1 && negc == 1 && inc == 1 {
		var bpos bool
		// Standardize so that a is on the plane
		switch {
		case isOn[0]:
			a, bv, c = tr.A, tr.B, tr.C
			bpos = isPos[1]
		case isOn[1]:
			a, bv, c = tr.B, tr.C, tr.A
			bpos = isPos[2]
		case isOn[2]:
			a, bv, c = tr.C, tr.A, tr.B
			bpos = isPos[0]
		default:
			panic("Impossible")
		}

		bPc := b.IntersectionWith(bv, c)
		btri := STriangleFrom(tr.Meta, a, bv, bPc)
		ctri := STriangleFrom(tr.Meta, c, a, bPc)

		if bpos {
			b.InsertTriangleHow(BspPos, &btri, m, bsp3)
			b.InsertTriangleHow(BspNeg, &ctri, m, bsp3)
		} else {
			b.InsertTriangleHow(BspPos, &ctri, m, bsp3)
			b.InsertTriangleHow(BspNeg, &btri, m, bsp3)
		}

		return
	}

	if posc == 2 && negc == 1 {
		// Standardize so that a is on one side, and b and c are on the other.
		switch {
		case isNeg[0]:
			a, bv, c = tr.A, tr.B, tr.C
		case isNeg[1]:
			a, bv, c = tr.B, tr.C, tr.A
		case isNeg[2]:
			a, bv, c = tr.C, tr.A, tr.B
		default:
			panic("Impossible")
		}
	} else if posc == 1 && negc == 2 {
		switch {
		case isPos[0]:
			a, bv, c = tr.A, tr.B, tr.C
		case isPos[1]:
			a, bv, c = tr.B, tr.C, tr.A
		case isPos[2]:
			a, bv, c = tr.C, tr.A, tr.B
		default:
			panic("Impossible")
		}
	} else {
		panic("Impossible")
	}

	aPb := b.IntersectionWith(a, bv)
	cPa := b.IntersectionWith(c, a)

	alone := STriangleFrom(tr.Meta, a, aPb, cPa)
	quad1 := STriangleFrom(tr.Meta, aPb, bv, c)
	quad2 := STriangleFrom(tr.Meta, aPb, c, cPa)

	if posc == 2 && negc == 1 {
		b.InsertTriangleHow(BspPos, &quad1, m, bsp3)
		b.InsertTriangleHow(BspPos, &quad2, m, bsp3)
		b.InsertTriangleHow(BspNeg, &alone, m, bsp3)
	} else {
		b.InsertTriangleHow(BspNeg, &quad1, m, bsp3)
		b.InsertTriangleHow(BspNeg, &quad2, m, bsp3)
		b.InsertTriangleHow(BspPos, &alone, m, bsp3)
	}
}
