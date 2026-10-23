package sketch

import (
	"fmt"
	"math"
	"slices"

	"github.com/telesapr/telesapr/pkg/core"
)

// A thread is several groups, which the group list folds into one row
// and which are moved, suppressed and deleted together as the thread;
// these find them and act on them as one.

// ThreadMembers are all the groups of the thread whose helix group is
// hg, in history order: the datum at its face's end, for a thread made
// on a face, then its ThreadGroups.
func (sk *Sketch) ThreadMembers(hg core.HGroup) ([]core.HGroup, error) {
	hs, err := sk.ThreadGroups(hg)
	if err != nil {
		return nil, err
	}
	if d, ok := sk.threadDatum(hg, hs[0]); ok {
		hs = append([]core.HGroup{d}, hs...)
	}
	return hs, nil
}

// threadDatum is the datum a thread made on a face was drawn in
// (AddThreadOnFace): the group of its circle, a datum workplane named
// by two faces of the solid, just before the thread's blank sketch.
func (sk *Sketch) threadDatum(hg, blank core.HGroup) (core.HGroup, bool) {
	e := sk.Entity(sk.Group(hg).Thread.Circle)
	if e == nil {
		return core.HGroup{}, false
	}
	g := sk.Group(e.Group)
	if g == nil || g.Subtype != GroupWorkplaneByDatum || len(g.SolidFaces) != 2 {
		return core.HGroup{}, false
	}
	at := slices.Index(sk.GroupOrder, blank)
	if at < 1 || sk.GroupOrder[at-1] != g.H {
		return core.HGroup{}, false
	}
	return g.H, true
}

// ThreadOfGroups maps each group that is one of a thread's
// (ThreadMembers) to that thread's helix group. A thread whose groups
// were edited one by one out of the shape of a thread has none mapped.
func (sk *Sketch) ThreadOfGroups() map[core.HGroup]core.HGroup {
	out := map[core.HGroup]core.HGroup{}
	for i := range sk.Groups {
		g := &sk.Groups[i]
		if !g.Thread.IsThread() {
			continue
		}
		hs, err := sk.ThreadMembers(g.H)
		if err != nil {
			continue
		}
		for _, h := range hs {
			out[h] = g.H
		}
	}
	return out
}

// ThreadOfGroup is the helix group of the thread that group hg is one
// of the groups of, and whether it is one.
func (sk *Sketch) ThreadOfGroup(hg core.HGroup) (core.HGroup, bool) {
	h, ok := sk.ThreadOfGroups()[hg]
	return h, ok
}

// MoveThread moves the thread whose helix group is hg, all its groups
// together, to just before group before in the history, or to the end
// for the zero handle, as MoveGroup moves one group: a move that would
// put a group ahead of one it refers to is refused with a *MoveError,
// and nothing moves. The caller remembers the undo state.
func (sk *Sketch) MoveThread(hg, before core.HGroup) error {
	newOrder, err := sk.movedThreadOrder(hg, before)
	if err != nil || newOrder == nil {
		return err
	}
	sk.applyOrder(newOrder)
	return nil
}

// CheckMoveThread reports the error MoveThread would give, without
// moving anything.
func (sk *Sketch) CheckMoveThread(hg, before core.HGroup) error {
	_, err := sk.movedThreadOrder(hg, before)
	return err
}

func (sk *Sketch) movedThreadOrder(hg, before core.HGroup) ([]core.HGroup, error) {
	hs, err := sk.ThreadMembers(hg)
	if err != nil {
		return nil, err
	}
	if before.V != 0 && sk.Group(before) == nil {
		return nil, fmt.Errorf("no group g%03x", before.V)
	}
	if before.V == core.HGroupReferences {
		return nil, fmt.Errorf("the references group stays first")
	}
	return sk.blockOrder(hg, hs, before)
}

// ThreadNeighbours are the groups just before and just after the
// thread whose helix group is hg, the zero handle where there is none:
// the places one step earlier and later that the group list's arrows
// move it to.
func (sk *Sketch) ThreadNeighbours(hg core.HGroup) (prev, next core.HGroup, err error) {
	hs, err := sk.ThreadMembers(hg)
	if err != nil {
		return prev, next, err
	}
	first, last := slices.Index(sk.GroupOrder, hs[0]), slices.Index(sk.GroupOrder, hs[len(hs)-1])
	if first > 0 {
		prev = sk.GroupOrder[first-1]
	}
	if last+1 < len(sk.GroupOrder) {
		next = sk.GroupOrder[last+1]
	}
	return prev, next, nil
}

// SetThreadSuppressed leaves the thread whose helix group is hg out of
// the model, every one of its groups (and so what uses them), or brings
// it back. The caller remembers the undo state.
func (sk *Sketch) SetThreadSuppressed(hg core.HGroup, on bool) error {
	hs, err := sk.ThreadMembers(hg)
	if err != nil {
		return err
	}
	for _, h := range hs {
		if err := sk.SetSuppressAll(h, on); err != nil {
			return err
		}
	}
	return nil
}

// ThreadSuppressed reports whether the thread whose helix group is hg
// is left out of the model as SetThreadSuppressed leaves it: each of
// its groups suppressed.
func (sk *Sketch) ThreadSuppressed(hg core.HGroup) bool {
	hs, err := sk.ThreadMembers(hg)
	if err != nil {
		return false
	}
	for _, h := range hs {
		if !sk.Group(h).SuppressAll {
			return false
		}
	}
	return true
}

// ThreadPartBeyond is how far the part goes on past the end of the
// thread whose helix group is hg, along its root: what a thread made
// Through turns down or bores past its length as a groove, where the
// part doesn't end with the thread. It is 0 for a part that ends there
// and for a thread that is not Through.
func (sk *Sketch) ThreadPartBeyond(hg core.HGroup) float64 {
	tg, err := sk.threadGroupsOf(hg)
	if err != nil {
		return 0
	}
	t := sk.Group(hg).Thread
	circle := sk.Entity(t.Circle)
	part := sk.solidAt(tg.blank)
	if !t.Through || circle == nil || !circle.IsCircle() || part == nil {
		return 0
	}
	l, err := t.layout(circle.CircleGetRadiusNum(sk))
	if err != nil {
		return 0
	}
	probe := newPartProbe(&part.RunningShell, &part.RunningMesh)
	origin, along, u, v := sk.threadAxis(t.Circle, t.ThreadOptions)
	reach := probe.reach(origin, along, u, v, l.baseR, t.Length+probe.span)
	if math.IsInf(reach, 1) {
		return probe.span
	}
	return max(0, reach-t.Length)
}

// SetThreadVisible shows or hides every group of the thread whose helix
// group is hg, as the group list's box for a folded thread does.
func (sk *Sketch) SetThreadVisible(hg core.HGroup, on bool) error {
	hs, err := sk.ThreadMembers(hg)
	if err != nil {
		return err
	}
	for _, h := range hs {
		sk.Group(h).Visible = on
		// A group shown may not have been generated while it was hidden;
		// hiding one changes nothing to make again, and solving a solved
		// sketch again could move it by an ulp.
		if on {
			sk.MarkGroupDirty(h, false)
		}
	}
	return nil
}

// ThreadVisible reports whether every group of the thread whose helix
// group is hg is shown.
func (sk *Sketch) ThreadVisible(hg core.HGroup) bool {
	hs, err := sk.ThreadMembers(hg)
	if err != nil {
		return false
	}
	for _, h := range hs {
		if !sk.Group(h).Visible {
			return false
		}
	}
	return true
}

// ClearThreadItems takes the requests and constraints out of the groups
// of the thread whose helix group is hg, which are about to be deleted
// with it, and returns those groups: deleted as meant, a regeneration
// doesn't count them among what was deleted for want of what it used.
func (sk *Sketch) ClearThreadItems(hg core.HGroup) ([]core.HGroup, error) {
	hs, err := sk.ThreadMembers(hg)
	if err != nil {
		return nil, err
	}
	in := func(h core.HGroup) bool { return slices.Contains(hs, h) }
	sk.Requests = slices.DeleteFunc(sk.Requests, func(r Request) bool { return in(r.Group) })
	sk.Constraints = slices.DeleteFunc(sk.Constraints, func(c core.Constraint) bool { return in(c.Group) })
	return hs, nil
}
