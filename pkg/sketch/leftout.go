package sketch

import (
	"errors"
	"slices"

	"github.com/telesapr/telesapr/pkg/core"
)

// This file keeps what the load of a damaged file left out of the
// model. The regeneration's pruning deletes what uses something the
// file lacks, as it deletes what an edit leaves without its references;
// but what a file lacked, the user never deleted, so a save would lose
// it unasked. It is held aside instead, written back by every save and
// left out again by every load, until the file is mended by hand and
// what it lacked is there, or until the user discards it.

// ErrNothingLeftOut is DiscardLeftOut's error when the load left
// nothing out.
var ErrNothingLeftOut = errors.New("nothing was left out of the model")

// LeftOut is what the load of a damaged file left out of the model,
// each item as the file held it: the groups, requests and constraints
// that used something the file lacked, with the values of their
// parameters. Nothing in the model refers to it. It is not changed
// once made, so a copy of the sketch may share it.
type LeftOut struct {
	Groups      []Group
	Requests    []Request
	Constraints []core.Constraint
	Params      []core.Param
	// Edges are the lists of edges of the groups the load kept but
	// dropped edges from, the edges of a chamfer or a fillet, a sweep's
	// path or a loft's guide curves that the file names and doesn't
	// hold.
	Edges []LeftOutEdges
}

// LeftOutEdges is a group's list of edges as a damaged file held it,
// with what the load dropped from it because the file doesn't hold
// them. A save puts the dropped edges back, with their own distances or
// radii, so long as the group's edges are still those the load left
// it: an edit of the group's edges replaces the list, and the edges
// dropped from it go.
type LeftOutEdges struct {
	Group core.HGroup
	// Edges, Radii and Seconds are the group's edges and their own
	// values as the file held them.
	Edges          []core.HEntity
	Radii, Seconds []EdgeRadius
	// Kept are the edges the load left the group, in order.
	Kept []core.HEntity
	// Names name the dropped edges, as the load knew them, for the
	// user.
	Names []string
}

// LeftOutNames names each item the load of a damaged file left out, a
// line each: "g003-on-missing", "r008-datum-point of g003-on-missing",
// "c001-pt-pt-distance of g002-sketch-in-plane".
func (sk *Sketch) LeftOutNames() []string {
	l := sk.LeftOut
	if l == nil {
		return nil
	}
	var out []string
	for i := range l.Groups {
		out = append(out, l.Groups[i].DescriptionString())
	}
	in := func(h core.HGroup) string {
		for i := range l.Groups {
			if l.Groups[i].H == h {
				return " of " + l.Groups[i].DescriptionString()
			}
		}
		return " of " + sk.anyGroupName(h)
	}
	for i := range l.Requests {
		out = append(out, l.Requests[i].DescriptionString()+in(l.Requests[i].Group))
	}
	for i := range l.Constraints {
		out = append(out, l.Constraints[i].DescriptionString()+in(l.Constraints[i].Group))
	}
	for _, e := range sk.keptEdges() {
		for _, n := range e.Names {
			out = append(out, "edge of "+sk.anyGroupName(e.Group)+": "+n)
		}
	}
	return out
}

// loadedItems is a copy of what a file held, taken before its first
// regeneration prunes it.
type loadedItems struct {
	groups      []Group
	requests    []Request
	constraints []core.Constraint
	params      []core.Param
}

// loaded copies what the sketch holds: the pruning changes a group's
// edges in place before it deletes the group, so the groups are copied
// whole.
func (sk *Sketch) loaded() loadedItems {
	l := loadedItems{
		groups:      make([]Group, 0, len(sk.Groups)),
		requests:    slices.Clone(sk.Requests),
		constraints: slices.Clone(sk.Constraints),
		params:      slices.Clone(sk.Params),
	}
	for i := range sk.Groups {
		l.groups = append(l.groups, sk.Groups[i].defining())
	}
	return l
}

// keepLeftOut makes LeftOut what was is and the sketch, regenerated,
// no longer holds: the groups, requests and constraints the pruning
// deleted, and the values of their parameters.
func (sk *Sketch) keepLeftOut(was loadedItems) {
	out := &LeftOut{}
	groups := map[core.HGroup]bool{}
	for _, g := range was.groups {
		if sk.Group(g.H) == nil {
			out.Groups = append(out.Groups, g)
			groups[g.H] = true
		}
	}
	requests := map[core.HRequest]bool{}
	for _, r := range was.requests {
		if sk.Request(r.H) == nil {
			out.Requests = append(out.Requests, r)
			requests[r.H] = true
		}
	}
	constraints := map[core.HConstraint]bool{}
	for _, c := range was.constraints {
		if sk.Constraint(c.H) == nil {
			out.Constraints = append(out.Constraints, c)
			constraints[c.H] = true
		}
	}
	for _, g := range was.groups {
		if e, ok := sk.droppedEdges(&g); ok {
			out.Edges = append(out.Edges, e)
		}
	}
	if len(out.Groups)+len(out.Requests)+len(out.Constraints)+len(out.Edges) == 0 {
		sk.LeftOut = nil
		return
	}
	for _, p := range was.params {
		h := p.H.V
		switch {
		case h&0x80000000 != 0:
			if !groups[core.HGroup{V: (h >> 16) & 0x3FFF}] {
				continue
			}
		case h&0x40000000 != 0:
			if !constraints[core.HConstraint{V: h &^ 0x40000000}] {
				continue
			}
		default:
			if !requests[p.H.Request()] {
				continue
			}
		}
		out.Params = append(out.Params, p)
	}
	sk.LeftOut = out
}

// droppedEdges is what the load dropped from was's edges, was being
// the group as the file held it, if the group is still there and lost
// some.
func (sk *Sketch) droppedEdges(was *Group) (LeftOutEdges, bool) {
	g := sk.Group(was.H)
	if g == nil || len(g.Edges) == len(was.Edges) {
		return LeftOutEdges{}, false
	}
	e := LeftOutEdges{
		Group:   was.H,
		Edges:   was.Edges,
		Radii:   was.EdgeRadii,
		Seconds: was.EdgeSeconds,
		Kept:    slices.Clone(g.Edges),
	}
	for _, he := range was.Edges {
		if !slices.Contains(e.Kept, he) {
			e.Names = append(e.Names, sk.entityName(he))
		}
	}
	return e, true
}

// keeps reports whether a save puts e's dropped edges back into g: g's
// edges are still those the load left it.
func (e *LeftOutEdges) keeps(g *Group) bool {
	return g != nil && slices.Equal(g.Edges, e.Kept)
}

// keptEdges is what the load dropped from the edges of the groups
// there are and a save still puts back.
func (sk *Sketch) keptEdges() []*LeftOutEdges {
	if sk.LeftOut == nil {
		return nil
	}
	var out []*LeftOutEdges
	for i := range sk.LeftOut.Edges {
		e := &sk.LeftOut.Edges[i]
		if e.keeps(sk.Group(e.Group)) {
			out = append(out, e)
		}
	}
	return out
}

// LeftOutEdgeNames names the edges the load of a damaged file dropped
// from group hg because the file doesn't hold them, and that a save
// still writes back; none once the group's edges have been edited.
func (sk *Sketch) LeftOutEdgeNames(hg core.HGroup) []string {
	for _, e := range sk.keptEdges() {
		if e.Group == hg {
			return e.Names
		}
	}
	return nil
}

// AsSaved is g as a save writes it: with the edges the load of a
// damaged file dropped from it put back where the file had them, with
// their own values, while its edges are still those the load left it.
// It is g itself when there are none.
func (sk *Sketch) AsSaved(g *Group) *Group {
	for _, e := range sk.keptEdges() {
		if e.Group != g.H {
			continue
		}
		c := *g
		c.Edges = slices.Clone(e.Edges)
		c.EdgeRadii = e.restore(g.EdgeRadii, e.Radii)
		c.EdgeSeconds = e.restore(g.EdgeSeconds, e.Seconds)
		return &c
	}
	return g
}

// restore is the group's own values of its edges, now, counted as the
// file counted its edges: an edge kept takes its place in the file's
// list, a solid edge (counted after them) moves on by the edges
// dropped, and each edge dropped brings back its value from the file.
func (e *LeftOutEdges) restore(now, file []EdgeRadius) []EdgeRadius {
	keptAt := make([]int, 0, len(e.Kept))
	dropped := map[int]bool{}
	k := 0
	for i, he := range e.Edges {
		if k < len(e.Kept) && e.Kept[k] == he {
			keptAt = append(keptAt, i)
			k++
		} else {
			dropped[i] = true
		}
	}
	var out []EdgeRadius
	for _, er := range now {
		if er.Edge < len(keptAt) {
			er.Edge = keptAt[er.Edge]
		} else {
			er.Edge += len(dropped)
		}
		out = append(out, er)
	}
	for _, er := range file {
		if dropped[er.Edge] {
			out = append(out, er)
		}
	}
	slices.SortStableFunc(out, func(a, b EdgeRadius) int { return a.Edge - b.Edge })
	return out
}

// DiscardLeftOut forgets what the load of a damaged file left out, so
// that the next save no longer writes it. The caller remembers the
// undo state.
func (sk *Sketch) DiscardLeftOut() error {
	if sk.LeftOut == nil {
		return ErrNothingLeftOut
	}
	sk.LeftOut = nil
	sk.Unsaved = true
	return nil
}

// topHandles are the largest group, request and constraint handles
// what was left out holds or refers to, so that a new item takes none
// of them: a request made in a new group with the handle of the group a
// left-out request was in would bring it back into that group.
func (l *LeftOut) topHandles() (group, request, constraint uint32) {
	if l == nil {
		return 0, 0, 0
	}
	for i := range l.Groups {
		group = max(group, l.Groups[i].H.V)
	}
	for i := range l.Requests {
		group = max(group, l.Requests[i].Group.V)
		request = max(request, l.Requests[i].H.V)
	}
	for i := range l.Constraints {
		group = max(group, l.Constraints[i].Group.V)
		constraint = max(constraint, l.Constraints[i].H.V)
	}
	// A dropped edge would come back as whatever new took its handle.
	for _, e := range l.Edges {
		for _, he := range e.Edges {
			if he.IsFromRequest() {
				request = max(request, he.Request().V)
			} else {
				group = max(group, he.Group().V)
			}
		}
	}
	return group, request, constraint
}
