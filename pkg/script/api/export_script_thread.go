package api

import (
	"slices"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// A thread whose groups were moved apart in the history, one on its
// own, is no longer edited as one, but its helix keeps what made it, and
// moved back together it is a thread again. Its script makes it with
// the thread's call, where the first of its groups stands, and then
// puts each group where it stands: the groups written between come
// before the ones the call made that stand later, and each of those is
// moved before the rest when it is reached. The drawing made has the
// groups of the model in its order, the same solid, and the same thread.

// apartThread is a thread whose groups were moved apart.
type apartThread struct {
	helix core.HGroup
	// groups are the thread's groups in the order its call makes them:
	// its datum first when the call is ThreadOnEdge, then those
	// ThreadGroupsApart gives.
	groups []core.HGroup
	// first is the first of them in the history, where the call is
	// written; made, whether it is written.
	first core.HGroup
	made  bool
	// at, before and onEdge are threadOnEdge's, for a thread whose
	// datum the call makes.
	at     geom.Vector
	before core.HGroup
	onEdge bool
}

// findApart notes g's thread in w.apart when its groups, found by
// ThreadGroupsApart, don't stand together in the order its call makes
// them, and reports whether it did.
func (w *scriptWriter) findApart(g *sketch.Group) bool {
	sk := w.sk
	hs, err := sk.ThreadGroupsApart(g.H)
	if err != nil {
		return false
	}
	t := &apartThread{helix: g.H, groups: hs}
	if p, onFace := sk.ThreadPlaceOf(g.H); onFace {
		if c := sk.Entity(g.Thread.Circle); c != nil {
			if datum := sk.Group(c.Group); datum != nil {
				t.at, t.before, t.onEdge = threadRimFrom(sk, datum, p)
				if t.onEdge {
					t.groups = append([]core.HGroup{datum.H}, hs...)
				}
			}
		}
	}
	at := slices.Index(sk.GroupOrder, t.groups[0])
	together := at >= 0 && at+len(t.groups) <= len(sk.GroupOrder) &&
		slices.Equal(sk.GroupOrder[at:at+len(t.groups)], t.groups)
	if together {
		return false
	}
	t.first = t.groups[0]
	for _, h := range sk.GroupOrder {
		if slices.Contains(t.groups, h) {
			t.first = h
			break
		}
	}
	for _, h := range t.groups {
		w.apart[h] = t
	}
	return true
}

// writeApart writes hg, one of the groups of the thread t apart: the
// thread's call at the first of them, and each of them put where it
// stands.
func (w *scriptWriter) writeApart(t *apartThread, hg core.HGroup) {
	if t.made {
		w.placeApart(hg)
		return
	}
	t.made = true
	w.writeGroup(w.sk.Group(t.helix))
	v, ok := w.groupVar[t.helix]
	if !ok {
		return
	}
	// The call made the thread's groups together after the active
	// group, before the groups of another thread still to be put.
	k := 0
	for i, h := range t.groups {
		g := w.sk.Group(h)
		switch {
		case t.onEdge && i == 0:
			w.line("%s := %s.AsThread().Circle().Group()", w.groupName(g), v)
			continue
		case h != t.helix:
			w.line("%s := %s.AsThread().Groups()[%d]", w.groupName(g), v, k)
		}
		k++
	}
	w.pending = append(slices.Clone(t.groups), w.pending...)
	w.placeApart(hg)
}

// placeApart puts hg, a group a thread's call made, where it stands:
// after the groups written before it and before the others the call
// made, and makes it active for what is written next. Where one group
// stands later on its own, as a lathe moved to the end, that one goes
// to the end, once, in place of each of the others going before it.
func (w *scriptWriter) placeApart(hg core.HGroup) {
	v := w.groupVar[hg]
	if slices.Index(w.pending, hg) == 1 && w.endFree(w.pending[0]) {
		s := w.pending[0]
		w.line("must(%s.History().MoveToEnd())", w.groupVar[s])
		w.pending = append(w.pending[1:], s)
	}
	if w.pending[0] != hg {
		w.line("must(%s.History().MoveBefore(%s))", v, w.groupVar[w.pending[0]])
	}
	w.pending = slices.DeleteFunc(w.pending, func(h core.HGroup) bool { return h == hg })
	w.line("doc.Activate(%s)", v)
	w.last = hg
	w.wp = w.sk.Group(hg).ActiveWorkplane
}

// endFree reports whether group h, one a thread's call made, can go to
// the end of the drawing made: none of the others still to be put, the
// groups after it, uses it.
func (w *scriptWriter) endFree(h core.HGroup) bool {
	for _, user := range w.sk.GroupDependents(h) {
		if slices.Contains(w.pending, user) {
			return false
		}
	}
	return true
}
