package sketch

import (
	"fmt"

	"github.com/telesapr/telesapr/pkg/core"
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
	groups   map[core.HGroup]knownGroup
	requests map[core.HRequest]knownRequest
}

// knownGroup is what names a group and the entities it made: its name,
// its type, which says whether its curves were edges of a solid, and
// its remap table, which says what each entity was a copy of. The
// table is the group's own, not a copy: a regeneration replaces these
// before it changes one, and a group deleted keeps its table as it was.
type knownGroup struct {
	name  string
	typ   GroupType
	remap core.EntityMap
}

// knownRequest is what names a request and its entities: its
// description comes from its type and handle, its entities' kinds from
// its type and extra points.
type knownRequest struct {
	typ          RequestType
	group        core.HGroup
	extraPoints  int
	construction bool
}

// rememberNames records the names of the groups and requests there are.
// It runs at every regeneration, so it refills the maps it has.
func (sk *Sketch) rememberNames() {
	n := &sk.names
	if n.groups == nil {
		n.groups = make(map[core.HGroup]knownGroup, len(sk.Groups))
		n.requests = make(map[core.HRequest]knownRequest, len(sk.Requests))
	}
	clear(n.groups)
	clear(n.requests)
	for i := range sk.Groups {
		g := &sk.Groups[i]
		n.groups[g.H] = knownGroup{g.Name, g.Type, g.Remap}
	}
	for i := range sk.Requests {
		r := &sk.Requests[i]
		n.requests[r.H] = knownRequest{r.Type, r.Group, r.ExtraPoints, r.Construction}
	}
}

// anyGroupName names group h, which may have been deleted since the
// last regeneration.
// GroupName names a group for a person, as its description, or by
// handle when it is gone.
func (sk *Sketch) GroupName(h core.HGroup) string { return sk.anyGroupName(h) }

func (sk *Sketch) anyGroupName(h core.HGroup) string {
	if g := sk.Group(h); g != nil {
		return g.DescriptionString()
	}
	if known, ok := sk.names.groups[h]; ok {
		g := Group{H: h, Name: known.name}
		return g.DescriptionString()
	}
	return fmt.Sprintf("g%03x", h.V)
}

// broke records a deletion. What belonged to a deleted group is not
// recorded: it went with the group, which is.
func (sk *Sketch) broke(group core.HGroup, item, missing string) {
	sk.broken = append(sk.broken, BrokenReference{
		Group: group, GroupName: sk.anyGroupName(group), Item: item, Missing: missing})
}

// neverGroup reports whether h, a group that isn't there, never was: no
// regeneration knew it and this one didn't delete it. What a damaged
// file puts in such a group is listed when it is left out; what was in
// a group deleted goes with it unlisted.
func (sk *Sketch) neverGroup(h core.HGroup) bool {
	if _, ok := sk.names.groups[h]; ok {
		return false
	}
	for _, b := range sk.broken {
		if b.Group == h && b.Item == b.GroupName {
			return false
		}
	}
	return true
}

// publishBroken makes what the regeneration deleted the list the user
// sees; a regeneration that deleted nothing leaves the list as it was.
func (sk *Sketch) publishBroken() {
	if len(sk.broken) > 0 {
		sk.BrokenReferences = sk.broken
		sk.broken = nil
	}
}

// ClearBrokenReferences empties the list of what regenerations deleted,
// and of what the load mended.
func (sk *Sketch) ClearBrokenReferences() {
	sk.BrokenReferences = nil
	sk.Mended = nil
	sk.broken = nil
}
