package sketch

import (
	"fmt"
	"sort"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/srf"
)

// This file holds history editing: which groups a group needs, and
// moving a group within the history without breaking those needs.

// Dependency is one group that another group needs, and the first
// reason found for it.
type Dependency struct {
	Group core.HGroup
	// Why names what refers to the group: "operand", "workplane",
	// "origin", "request r005", "constraint c012" and the like.
	Why string
}

// DescriptionString names the group as the property browser does:
// "g003-extrude".
func (g *Group) DescriptionString() string {
	name := g.Name
	if name == "" {
		name = "(unnamed)"
	}
	return fmt.Sprintf("g%03x-%s", g.H.V, name)
}

// entityGroup is the group entity he belongs to, worked out from the
// stored model alone, so that it holds before the sketch is
// regenerated. It is the zero handle for no entity, for free in 3d,
// and for an entity of a request that doesn't exist.
func (sk *Sketch) entityGroup(he core.HEntity) core.HGroup {
	if he == core.NoEntity || he == core.FreeIn3D {
		return core.HGroup{}
	}
	if he.IsFromRequest() {
		if r := sk.GetRequest(he.Request()); r != nil {
			return r.Group
		}
		return core.HGroup{}
	}
	return he.Group()
}

// faceGroup is the group that made a face, by its identity: an
// entity's group, or for a surface no entity names, the group in its
// origin (srf.OriginFor).
func faceGroup(id srf.FaceID) core.HGroup {
	if id&(1<<30) != 0 && id&(1<<31) == 0 {
		return core.HGroup{V: uint32(id>>16) & 0x3fff}
	}
	return core.HEntity{V: uint32(id)}.Group()
}

// GroupDependencies lists the groups group h refers to, each once, in
// history order: through its operand, its workplane and predefined
// entities, the entities its requests and constraints name, and the
// sources of its copied entities. A group never depends on itself.
func (sk *Sketch) GroupDependencies(h core.HGroup) []Dependency {
	g := sk.GetGroup(h)
	if g == nil {
		return nil
	}
	var deps []Dependency
	seen := map[core.HGroup]bool{h: true}
	add := func(on core.HGroup, why string) {
		if on.V == 0 || seen[on] {
			return
		}
		seen[on] = true
		deps = append(deps, Dependency{Group: on, Why: why})
	}
	addEntity := func(he core.HEntity, why string) {
		add(sk.entityGroup(he), why)
	}

	add(g.OpA, "operand")
	add(g.OpB, "second operand")
	addEntity(g.ActiveWorkplane, "workplane")
	addEntity(g.Predef.Origin, "origin")
	addEntity(g.Predef.EntityB, "axis or workplane")
	addEntity(g.Predef.EntityC, "second direction")
	for _, he := range g.Edges {
		addEntity(he, "edge")
	}
	for _, n := range g.SolidEdges {
		add(faceGroup(n.A), "solid edge "+n.String())
		add(faceGroup(n.B), "solid edge "+n.String())
	}
	for _, id := range g.SolidFaces {
		add(faceGroup(id), fmt.Sprintf("solid face %x", uint32(id)))
	}

	for i := range sk.Requests {
		r := &sk.Requests[i]
		if r.Group == h {
			addEntity(r.Workplane, fmt.Sprintf("workplane of request r%03x", r.H.V))
		}
	}
	for i := range sk.Constraints {
		c := &sk.Constraints[i]
		if c.Group != h {
			continue
		}
		why := fmt.Sprintf("constraint c%03x", c.H.V)
		for _, he := range []core.HEntity{c.Workplane, c.PtA, c.PtB,
			c.EntityA, c.EntityB, c.EntityC, c.EntityD} {
			addEntity(he, why)
		}
	}

	// A linked group's remap names entities of the linked file, not
	// of this sketch.
	if g.Type != GroupLinked {
		keys := make([]core.EntityKey, 0, len(g.Remap))
		for k := range g.Remap {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool {
			if keys[i].Input != keys[j].Input {
				return keys[i].Input.V < keys[j].Input.V
			}
			return keys[i].CopyNumber < keys[j].CopyNumber
		})
		for _, k := range keys {
			addEntity(k.Input, "copied entity")
		}
	}

	sort.SliceStable(deps, func(i, j int) bool {
		return sk.groupOrderOf(deps[i].Group) < sk.groupOrderOf(deps[j].Group)
	})
	return deps
}

// groupOrderOf is the order of group h, and an order after every group
// for one that doesn't exist.
func (sk *Sketch) groupOrderOf(h core.HGroup) int {
	if g := sk.GetGroup(h); g != nil {
		return g.Order
	}
	return int(^uint(0) >> 1)
}

// GroupDependents lists the groups that refer to group h directly, in
// history order.
func (sk *Sketch) GroupDependents(h core.HGroup) []core.HGroup {
	var out []core.HGroup
	for _, hg := range sk.orderedGroups() {
		for _, d := range sk.GroupDependencies(hg) {
			if d.Group == h {
				out = append(out, hg)
				break
			}
		}
	}
	return out
}

// groupName describes group h for a message, or names its handle if
// it doesn't exist.
func (sk *Sketch) groupName(h core.HGroup) string {
	if g := sk.GetGroup(h); g != nil {
		return g.DescriptionString()
	}
	return fmt.Sprintf("g%03x", h.V)
}

// MoveError is why a group can't move where it was asked to: Group
// would come before On, which it refers to through Why.
type MoveError struct {
	Moved core.HGroup
	Group core.HGroup
	On    core.HGroup
	Why   string
	msg   string
}

func (e *MoveError) Error() string { return e.msg }

// MoveGroup moves group h to just before group before in the history,
// or to the end for the zero handle. It refuses, with a *MoveError
// naming the reference, a move that would put a group ahead of a group
// it refers to, either h ahead of what it uses or something that uses
// h ahead of h. The references group stays first. The groups keep the
// set of order numbers they had, so that moving a group back restores
// the history exactly. The caller remembers the undo state.
func (sk *Sketch) MoveGroup(h, before core.HGroup) error {
	newOrder, err := sk.movedOrder(h, before)
	if err != nil || newOrder == nil {
		return err
	}
	order := sk.orderedGroups()
	vals := make([]int, len(order))
	for i, hg := range order {
		vals[i] = sk.GetGroup(hg).Order
	}
	// A file may give two groups one order; make them distinct, or
	// the sort could undo the move.
	for i := 1; i < len(vals); i++ {
		if vals[i] <= vals[i-1] {
			vals[i] = vals[i-1] + 1
		}
	}
	for i, hg := range newOrder {
		sk.GetGroup(hg).Order = vals[i]
	}
	sk.GroupOrder = append(sk.GroupOrder[:0], newOrder...)
	for i := range newOrder {
		if newOrder[i] != order[i] {
			sk.MarkGroupDirty(newOrder[i], false)
			break
		}
	}
	return nil
}

// CheckMoveGroup reports the error MoveGroup(h, before) would give,
// without moving anything.
func (sk *Sketch) CheckMoveGroup(h, before core.HGroup) error {
	_, err := sk.movedOrder(h, before)
	return err
}

// movedOrder is the history with group h moved before group before, or
// nil if the move changes nothing.
func (sk *Sketch) movedOrder(h, before core.HGroup) ([]core.HGroup, error) {
	if sk.GetGroup(h) == nil {
		return nil, fmt.Errorf("no group g%03x", h.V)
	}
	if before.V != 0 && sk.GetGroup(before) == nil {
		return nil, fmt.Errorf("no group g%03x", before.V)
	}
	if h.V == core.HGroupReferences || before.V == core.HGroupReferences {
		return nil, fmt.Errorf("the references group stays first")
	}
	if h == before {
		return nil, nil
	}
	order := sk.orderedGroups()
	newOrder := make([]core.HGroup, 0, len(order))
	for _, hg := range order {
		if hg == before {
			newOrder = append(newOrder, h)
		}
		if hg != h {
			newOrder = append(newOrder, hg)
		}
	}
	if before.V == 0 {
		newOrder = append(newOrder, h)
	}
	same := true
	for i := range order {
		if order[i] != newOrder[i] {
			same = false
			break
		}
	}
	if same {
		return nil, nil
	}

	pos := make(map[core.HGroup]int, len(newOrder))
	for i, hg := range newOrder {
		pos[hg] = i
	}
	for i, hg := range newOrder {
		for _, d := range sk.GroupDependencies(hg) {
			// Only references to or from h can change; a history
			// broken before the move is not the move's fault.
			at, ok := pos[d.Group]
			if !ok || at < i || (hg != h && d.Group != h) {
				continue
			}
			e := &MoveError{Moved: h, Group: hg, On: d.Group, Why: d.Why}
			if hg == h {
				e.msg = fmt.Sprintf("%s can't move before %s, which it uses (%s)",
					sk.groupName(h), sk.groupName(d.Group), d.Why)
			} else {
				e.msg = fmt.Sprintf("%s can't move after %s, which uses it (%s)",
					sk.groupName(h), sk.groupName(hg), d.Why)
			}
			return nil, e
		}
	}
	return newOrder, nil
}
