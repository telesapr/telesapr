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

type knownRequest struct {
	desc  string
	group core.HGroup
}

// rememberNames records the names of the groups and requests there are.
func (sk *Sketch) rememberNames() {
	n := knownNames{
		groups:   make(map[core.HGroup]string, len(sk.Groups)),
		requests: make(map[core.HRequest]knownRequest, len(sk.Requests)),
	}
	for i := range sk.Groups {
		n.groups[sk.Groups[i].H] = sk.Groups[i].DescriptionString()
	}
	for i := range sk.Requests {
		r := &sk.Requests[i]
		n.requests[r.H] = knownRequest{r.DescriptionString(), r.Group}
	}
	sk.names = n
}

// anyGroupName names group h, which may have been deleted since the
// last regeneration.
func (sk *Sketch) anyGroupName(h core.HGroup) string {
	if g := sk.GetGroup(h); g != nil {
		return g.DescriptionString()
	}
	if name, ok := sk.names.groups[h]; ok {
		return name
	}
	return fmt.Sprintf("g%03x", h.V)
}

// entityName names entity he, which may have been deleted since the
// last regeneration, by its request or its group.
func (sk *Sketch) entityName(he core.HEntity) string {
	if he.IsFromRequest() {
		if r := sk.GetRequest(he.Request()); r != nil {
			return r.DescriptionString() + " of " + sk.anyGroupName(r.Group)
		}
		if r, ok := sk.names.requests[he.Request()]; ok {
			return r.desc + " of " + sk.anyGroupName(r.group)
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
