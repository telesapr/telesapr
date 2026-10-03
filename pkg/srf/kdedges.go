package srf

import (
	"math"

	"github.com/telesapr/telesapr/pkg/geom"
)

// SEdgeLl is a singly-linked list of edges, the payload of an
// SKdNodeEdges leaf (C++ SEdgeLl).
type SEdgeLl struct {
	Edge *SEdge
	Next *SEdgeLl
}

// SKdNodeEdges is a kd-tree over edges, so that a query for the edges
// crossing one need only test those near it (C++ SKdNodeEdges). An
// edge near a split plane goes on both sides, and is tagged as it is
// tested so that it counts once.
type SKdNodeEdges struct {
	Which int // whether c is x, y, or z
	C     float64
	Gt    *SKdNodeEdges
	Lt    *SKdNodeEdges

	Edges *SEdgeLl
}

// SKdNodeEdgesFrom builds the tree over the edges of sel, which it
// keeps pointers into (C++ SKdNodeEdges::From(SEdgeList *)).
func SKdNodeEdgesFrom(sel *SEdgeList) *SKdNodeEdges {
	var sell *SEdgeLl
	for i := range sel.Edges {
		sell = &SEdgeLl{Edge: &sel.Edges[i], Next: sell}
	}
	return skdNodeEdgesFromList(sell)
}

// skdNodeEdgesFromList is C++ SKdNodeEdges::From(SEdgeLl *).
func skdNodeEdgesFromList(sell *SEdgeLl) *SKdNodeEdges {
	n := &SKdNodeEdges{}

	// Compute the midpoints (just mean, though median would be better)
	// of each component.
	var ptAve geom.Vector
	totaln := 0
	for flip := sell; flip != nil; flip = flip.Next {
		ptAve = ptAve.Plus(flip.Edge.A)
		ptAve = ptAve.Plus(flip.Edge.B)
		totaln++
	}
	ptAve = ptAve.ScaledBy(1.0 / float64(2*totaln))

	// For each component, see how it splits.
	var ltln, gtln [3]int
	var badness [3]float64
	for flip := sell; flip != nil; flip = flip.Next {
		for i := 0; i < 3; i++ {
			if flip.Edge.A.Element(i) < ptAve.Element(i)+kdTreeEps ||
				flip.Edge.B.Element(i) < ptAve.Element(i)+kdTreeEps {
				ltln[i]++
			}
			if flip.Edge.A.Element(i) > ptAve.Element(i)-kdTreeEps ||
				flip.Edge.B.Element(i) > ptAve.Element(i)-kdTreeEps {
				gtln[i]++
			}
		}
	}
	for i := 0; i < 3; i++ {
		badness[i] = math.Pow(float64(ltln[i]), 4) + math.Pow(float64(gtln[i]), 4)
	}

	// Choose the least bad coordinate to split along.
	switch {
	case badness[0] < badness[1] && badness[0] < badness[2]:
		n.Which = 0
	case badness[1] < badness[2]:
		n.Which = 1
	default:
		n.Which = 2
	}
	n.C = ptAve.Element(n.Which)

	if totaln < 3 || totaln == gtln[n.Which] || totaln == ltln[n.Which] {
		n.Edges = sell
		// and we're a leaf node
		return n
	}

	// Sort the edges according to which side(s) of the split plane
	// they're on.
	var gtl, ltl *SEdgeLl
	for flip := sell; flip != nil; flip = flip.Next {
		if flip.Edge.A.Element(n.Which) < n.C+kdTreeEps ||
			flip.Edge.B.Element(n.Which) < n.C+kdTreeEps {
			ltl = &SEdgeLl{Edge: flip.Edge, Next: ltl}
		}
		if flip.Edge.A.Element(n.Which) > n.C-kdTreeEps ||
			flip.Edge.B.Element(n.Which) > n.C-kdTreeEps {
			gtl = &SEdgeLl{Edge: flip.Edge, Next: gtl}
		}
	}

	n.Lt = skdNodeEdgesFromList(ltl)
	n.Gt = skdNodeEdgesFromList(gtl)
	return n
}

// AnyEdgeCrossings counts the edges of the tree that the edge from a
// to b crosses, as SEdge.EdgeCrosses decides, tagging each tested edge
// with cnt so that one on both sides of a split counts once; an edge
// already tagged cnt isn't tested (C++ SKdNodeEdges::AnyEdgeCrossings).
// A crossing is left in pi and added to spl when they are given.
func (n *SKdNodeEdges) AnyEdgeCrossings(a, b geom.Vector, cnt int, pi *geom.Vector, spl *SPointList) int {
	inters := 0
	if n.Gt != nil && n.Lt != nil {
		if a.Element(n.Which) < n.C+kdTreeEps || b.Element(n.Which) < n.C+kdTreeEps {
			inters += n.Lt.AnyEdgeCrossings(a, b, cnt, pi, spl)
		}
		if a.Element(n.Which) > n.C-kdTreeEps || b.Element(n.Which) > n.C-kdTreeEps {
			inters += n.Gt.AnyEdgeCrossings(a, b, cnt, pi, spl)
		}
	} else {
		for sell := n.Edges; sell != nil; sell = sell.Next {
			se := sell.Edge
			if se.Tag == cnt {
				continue
			}
			if se.EdgeCrosses(a, b, pi, spl) {
				inters++
			}
			se.Tag = cnt
		}
	}
	return inters
}
