package srf

import (
	"math"

	"github.com/spacesolve/spacesolve/pkg/geom"
)

// This file ports the SKdNode part of solvespace/src/mesh.cpp: a
// kd-tree over a mesh's triangles, used to find the edges of the mesh
// that have no mate (naked), that cross other triangles, or that
// outline it.

// kdTreeEps is how close to a node's splitting plane a triangle must
// be to go on both sides (C++ KDTREE_EPS).
const kdTreeEps = 20 * lengthEps

// SKdNode is a node of the kd-tree: an interior node splits space at
// coordinate Which == C, a leaf holds a list of triangles (C++ SKdNode).
type SKdNode struct {
	Which int // whether C is x, y, or z
	C     float64

	Gt, Lt *SKdNode

	Tris []*STriangle
}

// glibcRand reproduces glibc's rand() (the TYPE_3 additive feedback
// generator of random_r) after srand(seed), so that the kd-tree C++
// builds, and so the order in which it finds edges, comes out the same.
type glibcRand struct {
	r []int32
	i int
}

func newGlibcRand(seed uint32) *glibcRand {
	if seed == 0 {
		seed = 1
	}
	g := &glibcRand{r: make([]int32, 344)}
	g.r[0] = int32(seed)
	for i := 1; i < 31; i++ {
		// r[i] = (16807 * r[i-1]) % 2147483647, without overflow
		hi := g.r[i-1] / 127773
		lo := g.r[i-1] % 127773
		word := 16807*lo - 2836*hi
		if word < 0 {
			word += 2147483647
		}
		g.r[i] = word
	}
	for i := 31; i < 34; i++ {
		g.r[i] = g.r[i-31]
	}
	for i := 34; i < 344; i++ {
		g.r[i] = int32(uint32(g.r[i-31]) + uint32(g.r[i-3]))
	}
	g.i = 344
	return g
}

func (g *glibcRand) rand() int {
	v := int32(uint32(g.r[g.i-31]) + uint32(g.r[g.i-3]))
	g.r = append(g.r, v)
	g.i++
	return int(uint32(v) >> 1)
}

// SKdNodeFrom builds the kd-tree of mesh m, over its triangles in the
// shuffled order C++ gives them (C++ SKdNode::From(SMesh *)). The tree
// refers to m's triangles, whose tags FindEdgeOn uses.
func SKdNodeFrom(m *SMesh) *SKdNode {
	tra := make([]*STriangle, len(m.Triangles))
	for i := range m.Triangles {
		tra[i] = &m.Triangles[i]
	}

	rnd := newGlibcRand(0)
	n := len(tra)
	for n > 1 {
		k := rnd.rand() % n
		n--
		tra[k], tra[n] = tra[n], tra[k]
	}

	// C++ builds a linked list by prepending, which reverses the order.
	tll := make([]*STriangle, len(tra))
	for i, tr := range tra {
		tll[len(tra)-1-i] = tr
	}
	return skdNodeFromList(tll)
}

// skdNodeFromList builds the kd-tree over tll (C++
// SKdNode::From(STriangleLl *)).
func skdNodeFromList(tll []*STriangle) *SKdNode {
	ret := &SKdNode{}
	if len(tll) == 0 {
		return ret
	}

	var gtc, ltc [3]int
	allc := 0
	var badness, split [3]float64

	for i := 0; i < 3; i++ {
		for _, tr := range tll {
			split[i] += tr.A.Element(i)
			split[i] += tr.B.Element(i)
			split[i] += tr.C.Element(i)
		}
		split[i] /= float64(len(tll) * 3)

		for _, tr := range tll {
			a, b, c := tr.A.Element(i), tr.B.Element(i), tr.C.Element(i)

			if a < split[i]+kdTreeEps || b < split[i]+kdTreeEps || c < split[i]+kdTreeEps {
				ltc[i]++
			}
			if a > split[i]-kdTreeEps || b > split[i]-kdTreeEps || c > split[i]-kdTreeEps {
				gtc[i]++
			}
			if i == 0 {
				allc++
			}
		}
		badness[i] = math.Pow(float64(ltc[i]), 4) + math.Pow(float64(gtc[i]), 4)
	}
	which := 2
	if badness[0] < badness[1] && badness[0] < badness[2] {
		which = 0
	} else if badness[1] < badness[2] {
		which = 1
	}

	if allc < 3 || allc == gtc[which] || allc == ltc[which] {
		ret.Tris = tll
		return ret
	}

	// C++ prepends to each list, reversing the order.
	var lgt, llt []*STriangle
	for k := len(tll) - 1; k >= 0; k-- {
		tr := tll[k]
		a, b, c := tr.A.Element(which), tr.B.Element(which), tr.C.Element(which)

		if a < split[which]+kdTreeEps || b < split[which]+kdTreeEps || c < split[which]+kdTreeEps {
			llt = append(llt, tr)
		}
		if a > split[which]-kdTreeEps || b > split[which]-kdTreeEps || c > split[which]-kdTreeEps {
			lgt = append(lgt, tr)
		}
	}

	ret.Which = which
	ret.C = split[which]
	ret.Gt = skdNodeFromList(lgt)
	ret.Lt = skdNodeFromList(llt)
	return ret
}

// ClearTags zeroes the tags of every triangle in the tree.
func (n *SKdNode) ClearTags() {
	if n.Gt != nil && n.Lt != nil {
		n.Gt.ClearTags()
		n.Lt.ClearTags()
		return
	}
	for _, tr := range n.Tris {
		tr.Tag = 0
	}
}

// AddTriangle adds tr to every leaf whose region it touches.
func (n *SKdNode) AddTriangle(tr *STriangle) {
	if n.Gt != nil && n.Lt != nil {
		ta, tb, tc := tr.A.Element(n.Which), tr.B.Element(n.Which), tr.C.Element(n.Which)
		if ta < n.C+kdTreeEps || tb < n.C+kdTreeEps || tc < n.C+kdTreeEps {
			n.Lt.AddTriangle(tr)
		}
		if ta > n.C-kdTreeEps || tb > n.C-kdTreeEps || tc > n.C-kdTreeEps {
			n.Gt.AddTriangle(tr)
		}
		return
	}
	// C++ prepends to the leaf's list.
	n.Tris = append([]*STriangle{tr}, n.Tris...)
}

// MakeMeshInto adds each triangle of the tree to m once.
func (n *SKdNode) MakeMeshInto(m *SMesh) {
	if n.Gt != nil {
		n.Gt.MakeMeshInto(m)
	}
	if n.Lt != nil {
		n.Lt.MakeMeshInto(m)
	}
	for _, tr := range n.Tris {
		if tr.Tag != 0 {
			continue
		}
		m.Triangles = append(m.Triangles, *tr)
		tr.Tag = 1
	}
}

// ListTrianglesInto lists each triangle of the tree once.
func (n *SKdNode) ListTrianglesInto(tl *[]*STriangle) {
	if n.Gt != nil {
		n.Gt.ListTrianglesInto(tl)
	}
	if n.Lt != nil {
		n.Lt.ListTrianglesInto(tl)
	}
	for _, tr := range n.Tris {
		if tr.Tag != 0 {
			continue
		}
		*tl = append(*tl, tr)
		tr.Tag = 1
	}
}

// EdgeOnInfo is what FindEdgeOn learns about an edge (C++
// SKdNode::EdgeOnInfo).
type EdgeOnInfo struct {
	Count          int
	FrontFacing    bool
	IntersectsMesh bool
	Tr             *STriangle
	Ai, Bi         int
}

// FindEdgeOn looks for triangles that are mates of the edge a -> b (an
// edge b -> a of theirs), counting them in info.Count; it also reports
// whether the edge crosses the mesh. With coplanarIsInter, an edge
// crossing a triangle on its boundary counts too. Triangles tagged cnt
// were seen already (C++ SKdNode::FindEdgeOn).
func (n *SKdNode) FindEdgeOn(a, b geom.Vector, cnt int, coplanarIsInter bool, info *EdgeOnInfo) {
	if n.Gt != nil && n.Lt != nil {
		ac, bc := a.Element(n.Which), b.Element(n.Which)
		if ac < n.C+kdTreeEps || bc < n.C+kdTreeEps {
			n.Lt.FindEdgeOn(a, b, cnt, coplanarIsInter, info)
		}
		if ac > n.C-kdTreeEps || bc > n.C-kdTreeEps {
			n.Gt.FindEdgeOn(a, b, cnt, coplanarIsInter, info)
		}
		return
	}

	// We are a leaf node; so we iterate over all the triangles in our
	// linked list.
	for _, tr := range n.Tris {
		if tr.Tag == cnt {
			continue
		}

		// Test if this triangle matches up with the given edge
		if (a.Equals(tr.B) && b.Equals(tr.A)) ||
			(a.Equals(tr.C) && b.Equals(tr.B)) ||
			(a.Equals(tr.A) && b.Equals(tr.C)) {
			info.Count++
			// Record whether this triangle is front- or back-facing.
			info.FrontFacing = tr.Normal().Z > lengthEps
			// Record the triangle
			info.Tr = tr
			// And record which vertices a and b correspond to
			info.Ai = vertexIndex(tr, a)
			info.Bi = vertexIndex(tr, b)
		} else if (a.Equals(tr.A) && b.Equals(tr.B)) ||
			(a.Equals(tr.B) && b.Equals(tr.C)) ||
			(a.Equals(tr.C) && b.Equals(tr.A)) {
			// It's an edge of this triangle, okay.
		} else {
			// Check for self-intersection
			nn := tr.Normal().WithMagnitude(1)
			d := tr.A.Dot(nn)
			pa, pb := a.Dot(nn)-d, b.Dot(nn)-d
			// It's an intersection if neither point lies in-plane,
			// and the edge crosses the plane (should handle in-plane
			// intersections separately but don't yet).
			if (pa < -lengthEps || pa > lengthEps) &&
				(pb < -lengthEps || pb > lengthEps) &&
				pa*pb < 0 {
				// The edge crosses the plane of the triangle; now see if
				// it crosses inside the triangle.
				if tr.ContainsPointProjd(b.Minus(a), a) {
					if coplanarIsInter {
						info.IntersectsMesh = true
					} else {
						p := geom.AtIntersectionOfPlaneAndLine(nn, d, a, b)
						ta, tb, tc := tr.A, tr.B, tr.C
						if p.DistanceToLine(ta, tb.Minus(ta)) < lengthEps ||
							p.DistanceToLine(tb, tc.Minus(tb)) < lengthEps ||
							p.DistanceToLine(tc, ta.Minus(tc)) < lengthEps {
							// Intersection lies on edge. This happens when
							// our edge is from a triangle coplanar with
							// another triangle in the mesh. We don't test
							// the edge against triangles whose plane contains
							// that edge, but we do end up testing against
							// the coplanar triangle's neighbours, which we
							// will intersect on their edges.
						} else {
							info.IntersectsMesh = true
						}
					}
				}
			}
		}

		// Ensure that we don't count this triangle twice if it appears
		// in two buckets of the kd tree.
		tr.Tag = cnt
	}
}

func vertexIndex(tr *STriangle, p geom.Vector) int {
	if p.Equals(tr.A) {
		return 0
	}
	if p.Equals(tr.B) {
		return 1
	}
	return 2
}

func (tr *STriangle) vertex(i int) geom.Vector {
	switch i {
	case 0:
		return tr.A
	case 1:
		return tr.B
	}
	return tr.C
}

func (tr *STriangle) vertexNormal(i int) geom.Vector {
	switch i {
	case 0:
		return tr.NA
	case 1:
		return tr.NB
	}
	return tr.NC
}

type triPair struct{ a, b *STriangle }

// checkAndAddTrianglePair reports whether the pair (a, b), in either
// order, was seen already, and records it.
func checkAndAddTrianglePair(pairs map[triPair]bool, a, b *STriangle) bool {
	if pairs[triPair{a, b}] || pairs[triPair{b, a}] {
		return true
	}
	pairs[triPair{a, b}] = true
	return false
}

// MakeCertainEdgesInto picks certain classes of edges out from the
// mesh: naked edges (with no anti-parallel mate) and self-intersecting
// edges, or turning, emphasized or sharp edges. inter and leaky, when
// not nil, report whether any edge was self-intersecting or naked (C++
// SKdNode::MakeCertainEdgesInto).
func (n *SKdNode) MakeCertainEdgesInto(sel *SEdgeList, how EdgeKind, coplanarIsInter bool,
	inter, leaky *bool, auxA int) {
	if inter != nil {
		*inter = false
	}
	if leaky != nil {
		*leaky = false
	}

	var tris []*STriangle
	n.ClearTags()
	n.ListTrianglesInto(&tris)

	edgeTris := map[triPair]bool{}
	cnt := 1234
	for _, tr := range tris {
		for j := 0; j < 3; j++ {
			a := tr.vertex(j)
			b := tr.vertex((j + 1) % 3)

			var info EdgeOnInfo
			n.FindEdgeOn(a, b, cnt, coplanarIsInter, &info)

			switch how {
			case EdgeNakedOrSelfInter:
				// there should be one anti-parllel edge
				if info.Count != 1 {
					// but there may be multiple parallel coincident edges
					var parallelInfo EdgeOnInfo
					n.FindEdgeOn(b, a, -cnt, coplanarIsInter, &parallelInfo)
					if info.Count != parallelInfo.Count {
						sel.AddEdge(a, b, auxA, 0, 0)
						if leaky != nil {
							*leaky = true
						}
					}
				}
				if info.IntersectsMesh {
					sel.AddEdge(a, b, auxA, 0, 0)
					if inter != nil {
						*inter = true
					}
				}

			case EdgeSelfInter:
				if info.IntersectsMesh {
					sel.AddEdge(a, b, auxA, 0, 0)
					if inter != nil {
						*inter = true
					}
				}

			case EdgeTurning:
				if tr.Normal().Z < lengthEps && info.Count == 1 && info.FrontFacing {
					if checkAndAddTrianglePair(edgeTris, tr, info.Tr) {
						break
					}
					// This triangle is back-facing (or on edge), and
					// this edge has exactly one mate, and that mate is
					// front-facing. So this is a turning edge.
					sel.AddEdge(a, b, auxA, 0, 0)
				}

			case EdgeEmphasized:
				if info.Count == 1 && tr.Meta.Face != info.Tr.Meta.Face {
					if checkAndAddTrianglePair(edgeTris, tr, info.Tr) {
						break
					}
					// The two triangles that join at this edge come from
					// different faces; either really different faces,
					// or one is from a face and the other is zero (i.e.,
					// not from a face).
					sel.AddEdge(a, b, auxA, 0, 0)
				}

			case EdgeSharp:
				if info.Count == 1 && isSharp(tr, j, &info) {
					if checkAndAddTrianglePair(edgeTris, tr, info.Tr) {
						break
					}
					// The two triangles that join at this edge meet at a sharp
					// angle. This implies they come from different faces.
					sel.AddEdge(a, b, auxA, 0, 0)
				}
			}

			cnt++
		}
	}
}

// isSharp reports whether the vertex normals of tr's edge j differ from
// those of its mate info.Tr.
func isSharp(tr *STriangle, j int, info *EdgeOnInfo) bool {
	na0 := tr.vertexNormal(j).WithMagnitude(1.0)
	nb0 := tr.vertexNormal((j + 1) % 3).WithMagnitude(1.0)
	na1 := info.Tr.vertexNormal(info.Ai).WithMagnitude(1.0)
	nb1 := info.Tr.vertexNormal(info.Bi).WithMagnitude(1.0)
	return !((na0.Equals(na1) && nb0.Equals(nb1)) ||
		(na0.Equals(nb1) && nb0.Equals(na1)))
}

// MakeOutlinesInto lists the edges shared by two triangles, tagged 1
// where they are emphasized or sharp (per edgeKind); untagged edges
// between triangles with the same normal are left out (C++
// SKdNode::MakeOutlinesInto).
func (n *SKdNode) MakeOutlinesInto(sol *SOutlineList, edgeKind EdgeKind) {
	var tris []*STriangle
	n.ClearTags()
	n.ListTrianglesInto(&tris)

	edgeTris := map[triPair]bool{}
	cnt := 1234
	for _, tr := range tris {
		for j := 0; j < 3; j++ {
			a := tr.vertex(j)
			b := tr.vertex((j + 1) % 3)

			var info EdgeOnInfo
			n.FindEdgeOn(a, b, cnt, false, &info)
			cnt++
			if info.Count != 1 {
				continue
			}
			if checkAndAddTrianglePair(edgeTris, tr, info.Tr) {
				continue
			}

			tag := 0
			switch edgeKind {
			case EdgeEmphasized:
				if tr.Meta.Face != info.Tr.Meta.Face {
					tag = 1
				}
			case EdgeSharp:
				if isSharp(tr, j, &info) {
					tag = 1
				}
			default:
				panic("Unexpected edge kind")
			}

			nl := tr.Normal().WithMagnitude(1.0)
			nr := info.Tr.Normal().WithMagnitude(1.0)

			// We don't add edges with the same left and right
			// normals because they can't produce outlines.
			if tag == 0 && nl.Equals(nr) {
				continue
			}
			sol.AddEdge(a, b, nl, nr, tag)
		}
	}
}
