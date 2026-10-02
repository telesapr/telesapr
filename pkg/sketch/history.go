package sketch

import (
	"fmt"
	"sort"

	"github.com/spacesolve/spacesolve/pkg/core"
)

// This file holds history editing: which groups a group needs.

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
