package sketch

import (
	"fmt"

	"github.com/spacesolve/spacesolve/pkg/core"
)

// This file records what a regeneration deletes because what it refers
// to is gone, so that no deletion is silent.

// BrokenReference is a request, constraint or group that a
// regeneration deleted, because something it referred to was gone.
type BrokenReference struct {
	// Group is the group the deleted item belonged to, or the deleted
	// group itself. It may be gone too.
	Group core.HGroup
	// GroupName names Group, as it was: "g004-sketch-in-plane".
	GroupName string
	// Item names what was deleted: "c012-pt-pt-distance",
	// "r007-line-segment" or "g005-extrude".
	Item string
	// Missing names what it referred to and is gone: "r005-line-segment
	// of g002-sketch-in-plane", or a group, "g002-sketch-in-plane".
	Missing string
}

// String says what was deleted and why: "deleted c012-pt-pt-distance
// of g004-sketch-in-plane, which used r005-line-segment of
// g002-sketch-in-plane".
func (b BrokenReference) String() string {
	if b.Item == b.GroupName {
		return fmt.Sprintf("deleted %s, which used %s", b.Item, b.Missing)
	}
	return fmt.Sprintf("deleted %s of %s, which used %s", b.Item, b.GroupName, b.Missing)
}

// knownNames are the names of the groups and requests at the end of the
// last regeneration, so that what has been deleted since can be named.
type knownNames struct {
	groups   map[core.HGroup]string
	requests map[core.HRequest]knownRequest
}

// knownRequest is what names a request: its description comes from
// its type and handle.
type knownRequest struct {
	typ   RequestType
	group core.HGroup
}

// rememberNames records the names of the groups and requests there are.
// It runs at every regeneration, so it refills the maps it has.
func (sk *Sketch) rememberNames() {
	n := &sk.names
	if n.groups == nil {
		n.groups = make(map[core.HGroup]string, len(sk.Groups))
		n.requests = make(map[core.HRequest]knownRequest, len(sk.Requests))
	}
	clear(n.groups)
	clear(n.requests)
	for i := range sk.Groups {
		g := &sk.Groups[i]
		n.groups[g.H] = g.Name
	}
	for i := range sk.Requests {
		r := &sk.Requests[i]
		n.requests[r.H] = knownRequest{r.Type, r.Group}
	}
}

// anyGroupName names group h, which may have been deleted since the
// last regeneration.
// GroupName names a group for a person, as its description, or by
// handle when it is gone.
func (sk *Sketch) GroupName(h core.HGroup) string { return sk.anyGroupName(h) }

func (sk *Sketch) anyGroupName(h core.HGroup) string {
	if g := sk.GetGroup(h); g != nil {
		return g.DescriptionString()
	}
	if name, ok := sk.names.groups[h]; ok {
		g := Group{H: h, Name: name}
		return g.DescriptionString()
	}
	return fmt.Sprintf("g%03x", h.V)
}

// entityName names entity he, which may have been deleted since the
// last regeneration, by its request or its group.
// EntityName names an entity for a message: a request's entity by its
// request, "line-segment r005 of g002-sketch", another by its handle
// and group.
func (sk *Sketch) EntityName(he core.HEntity) string { return sk.entityName(he) }

func (sk *Sketch) entityName(he core.HEntity) string {
	if he.IsFromRequest() {
		if r := sk.GetRequest(he.Request()); r != nil {
			return r.DescriptionString() + " of " + sk.anyGroupName(r.Group)
		}
		if r, ok := sk.names.requests[he.Request()]; ok {
			gone := Request{H: he.Request(), Type: r.typ}
			return gone.DescriptionString() + " of " + sk.anyGroupName(r.group)
		}
		return fmt.Sprintf("r%03x", he.Request().V)
	}
	return fmt.Sprintf("entity %x of %s", he.V, sk.anyGroupName(he.Group()))
}

// broke records a deletion. What belonged to a deleted group is not
// recorded: it went with the group, which is.
func (sk *Sketch) broke(group core.HGroup, item, missing string) {
	sk.broken = append(sk.broken, BrokenReference{
		Group: group, GroupName: sk.anyGroupName(group), Item: item, Missing: missing})
}

// publishBroken makes what the regeneration deleted the list the user
// sees; a regeneration that deleted nothing leaves the list as it was.
func (sk *Sketch) publishBroken() {
	if len(sk.broken) > 0 {
		sk.BrokenReferences = sk.broken
		sk.broken = nil
	}
}

// ClearBrokenReferences empties the list of what regenerations deleted.
func (sk *Sketch) ClearBrokenReferences() {
	sk.BrokenReferences = nil
	sk.broken = nil
}
