package sketch

import (
	"fmt"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/topo"
)

// A chamfer's edge can have distances of its own: its first distance
// in EdgeRadii, as a fillet's edge has its radius there, and its second
// in EdgeSeconds. An edge with none takes the group's, ValB and ValC,
// so a chamfer whose edges have none is the chamfer it always was.

// EdgeDistanceField names a chamfer's edge's own distance in messages
// and in the file.
func EdgeDistanceField(i int) string { return fmt.Sprintf("distance of edge %d", i+1) }

// EdgeSecondField names a chamfer's edge's own second distance in
// messages and in the file.
func EdgeSecondField(i int) string { return fmt.Sprintf("second distance of edge %d", i+1) }

// edgeValueList is one of a group's lists of edges' own values, with
// how its values are named and what is said of one not more than zero.
type edgeValueList struct {
	list    *[]EdgeRadius
	field   func(int) string
	refusal string
}

// edgeValueLists are g's lists of edges' own values: a fillet's radii,
// a chamfer's distances and second distances.
func (g *Group) edgeValueLists() []edgeValueList {
	if g.Type == GroupChamfer {
		return []edgeValueList{
			{&g.EdgeRadii, EdgeDistanceField, "a chamfer's distance must be more than zero"},
			{&g.EdgeSeconds, EdgeSecondField, "a chamfer's second distance must be more than zero"},
		}
	}
	return []edgeValueList{{&g.EdgeRadii, EdgeRadiusField, "a fillet's radius must be more than zero"}}
}

// EdgeSecond is the second distance a chamfer gives its edge i,
// counting Edges and then SolidEdges, and its expression: the edge's
// own, or the group's, which is 0 for a chamfer of one distance.
func (g *Group) EdgeSecond(i int) (float64, string, bool) {
	for _, er := range g.EdgeSeconds {
		if er.Edge == i {
			return er.Val, er.Expr, true
		}
	}
	return g.ValC, g.ExprC, false
}

// SetEdgeDistanceText gives a chamfer's edge i a distance of its own
// from what the user typed, in the document's unit, as SetGroupText
// does for the group's distance; "" gives it the group's again. It is
// what the group's screen and scripts both call.
func (sk *Sketch) SetEdgeDistanceText(hg core.HGroup, i int, text string) error {
	g, err := sk.chamferGroup(hg)
	if err != nil {
		return err
	}
	return sk.setEdgeValueText(g, &g.EdgeRadii, i, text, EdgeDistanceField(i), "a chamfer's distance must be more than zero")
}

// SetEdgeSecondText gives a chamfer's edge i a second distance of its
// own, as SetEdgeDistanceText gives it a first; "" gives it the
// group's again.
func (sk *Sketch) SetEdgeSecondText(hg core.HGroup, i int, text string) error {
	g, err := sk.chamferGroup(hg)
	if err != nil {
		return err
	}
	return sk.setEdgeValueText(g, &g.EdgeSeconds, i, text, EdgeSecondField(i), "a chamfer's second distance must be more than zero")
}

// chamferPicks are a chamfer's segments as topo picks them: each with
// its own distance where radii gives it one (else the group's, which
// FilletPicks is given), and its second distance from seconds, else
// d2.
func chamferPicks(segs []chamferSegment, d2 float64, radii, seconds []EdgeRadius) []topo.FilletPick {
	out := make([]topo.FilletPick, len(segs))
	for i, s := range segs {
		out[i] = s.pick(d2)
	}
	for _, er := range radii {
		if er.Edge >= 0 && er.Edge < len(out) {
			out[er.Edge].R = er.Val
		}
	}
	for _, er := range seconds {
		if er.Edge >= 0 && er.Edge < len(out) {
			out[er.Edge].D2 = er.Val
		}
	}
	return out
}
