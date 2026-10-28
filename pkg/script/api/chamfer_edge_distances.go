package api

import (
	"math"
	"strconv"

	"github.com/telesapr/telesapr/pkg/sketch"
)

// EdgeDistance is the distance in mm a chamfer cuts its edge i at,
// counting the entities Edges gives and then the edges SolidEdges
// gives, from 0: the edge's own, or the chamfer's.
func (v ChamferGroup) EdgeDistance(i int) (mm float64) {
	g := v.g
	g.d.read(func(sk *sketch.Sketch) {
		group := g.get(sk)
		g.mustBe(group, sketch.GroupChamfer, "EdgeDistance")
		if i < 0 || i >= len(group.Edges)+len(group.SolidEdges) {
			fail("the chamfer has no edge %d", i)
		}
		mm, _, _ = group.EdgeRadius(i)
	})
	return mm
}

// SetEdgeDistance gives a chamfer's edge i, counted as EdgeDistance
// counts, a distance of its own in mm, or with 0 the chamfer's again.
// Chamfered edges of different distances that meet at a corner whose
// third edge stays sharp meet in a mitre; at a corner the chamfer's
// local edit can't make, as one of three chamfered edges of different
// distances, each straight edge is cut by a wedge instead.
func (v ChamferGroup) SetEdgeDistance(i int, mm float64) {
	text := v.g.edgeMmText(mm, "a chamfer's distance must be more than zero")
	if err := v.SetEdgeDistanceExpr(i, text); err != nil {
		fail("%v", err)
	}
}

// SetEdgeDistanceExpr gives a chamfer's edge i, counted as EdgeDistance
// counts, a distance of its own from an expression in the document's
// unit, which may name parameters; "" gives it the chamfer's distance
// again.
func (v ChamferGroup) SetEdgeDistanceExpr(i int, expr string) (err error) {
	g := v.g
	g.d.change(func(sk *sketch.Sketch) {
		group := g.get(sk)
		g.mustBe(group, sketch.GroupChamfer, "SetEdgeDistanceExpr")
		err = sk.SetEdgeDistanceText(g.H, i, expr)
	})
	return err
}

// EdgeSecondDistance is the distance in mm a chamfer cuts its edge i at
// on the edge's second face, counted as EdgeDistance counts: the edge's
// own, or the chamfer's SecondDistance, 0 when that is the first
// distance on both faces.
func (v ChamferGroup) EdgeSecondDistance(i int) (mm float64) {
	g := v.g
	g.d.read(func(sk *sketch.Sketch) {
		group := g.get(sk)
		g.mustBe(group, sketch.GroupChamfer, "EdgeSecondDistance")
		if i < 0 || i >= len(group.Edges)+len(group.SolidEdges) {
			fail("the chamfer has no edge %d", i)
		}
		mm, _, _ = group.EdgeSecond(i)
	})
	return mm
}

// SetEdgeSecondDistance gives a chamfer's edge i, counted as
// EdgeDistance counts, a second distance of its own in mm, on the
// edge's second face, or with 0 the chamfer's again.
func (v ChamferGroup) SetEdgeSecondDistance(i int, mm float64) {
	text := v.g.edgeMmText(mm, "a chamfer's second distance must be more than zero")
	if err := v.SetEdgeSecondDistanceExpr(i, text); err != nil {
		fail("%v", err)
	}
}

// SetEdgeSecondDistanceExpr gives a chamfer's edge i a second distance
// of its own from an expression in the document's unit, as
// SetEdgeDistanceExpr gives it a first; "" gives it the chamfer's
// again.
func (v ChamferGroup) SetEdgeSecondDistanceExpr(i int, expr string) (err error) {
	g := v.g
	g.d.change(func(sk *sketch.Sketch) {
		group := g.get(sk)
		g.mustBe(group, sketch.GroupChamfer, "SetEdgeSecondDistanceExpr")
		err = sk.SetEdgeSecondText(g.H, i, expr)
	})
	return err
}

// edgeMmText is mm as the screen takes an edge's own value, in the
// document's unit, or "" for 0, which takes the edge's own value off;
// a value below zero fails with refusal.
func (g Group) edgeMmText(mm float64, refusal string) string {
	if mm == 0 {
		return ""
	}
	if mm < 0 || math.IsNaN(mm) || math.IsInf(mm, 0) {
		fail("%s, not %g", refusal, mm)
	}
	var per float64
	g.d.read(func(sk *sketch.Sketch) { per = sk.Units.MmPer() })
	return strconv.FormatFloat(mm/per, 'g', -1, 64)
}
