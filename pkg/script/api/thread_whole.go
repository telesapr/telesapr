package api

import (
	"slices"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// A thread is several groups, which the window's group list folds into
// one row and moves, shows, suppresses and deletes as one; these are
// the same for a script.

// ThreadOf is the thread the group is one of the groups of, and whether
// it is one: the datum at a face's end for a thread made on a face, the
// sketches, the lathe and the helices that Features.Thread makes. The
// window's group list folds those groups into the thread's row.
func (g Group) ThreadOf() (t ThreadGroup, ok bool) {
	g.d.read(func(sk *sketch.Sketch) {
		g.get(sk)
		var h core.HGroup
		if h, ok = sk.ThreadOfGroup(g.H); ok {
			t = ThreadGroup{Group{g.d, h}}
		}
	})
	return t, ok
}

// Group is the thread's helix group, which keeps what made the thread
// and is the thread's row in the window's group list.
func (v ThreadGroup) Group() Group { return v.g }

// Visible reports whether every group of the thread is shown.
func (v ThreadGroup) Visible() (visible bool) {
	g := v.g
	g.d.read(func(sk *sketch.Sketch) { visible = sk.ThreadVisible(g.get(sk).H) })
	return visible
}

// SetVisible shows or hides every group of the thread, as the box of
// its row in the group list does.
func (v ThreadGroup) SetVisible(visible bool) {
	g := v.g
	g.d.change(func(sk *sketch.Sketch) {
		if err := sk.SetThreadVisible(g.get(sk).H, visible); err != nil {
			fail("%v", err)
		}
	})
}

// Suppressed reports whether the thread is left out of the model, as
// SetSuppressed leaves it.
func (v ThreadGroup) Suppressed() (on bool) {
	g := v.g
	g.d.read(func(sk *sketch.Sketch) { on = sk.ThreadSuppressed(g.get(sk).H) })
	return on
}

// SetSuppressed leaves the whole thread out of the model, each of its
// groups and what uses them, or brings it back, in one step: the part
// is then as it was before the thread.
func (v ThreadGroup) SetSuppressed(on bool) {
	g := v.g
	g.d.change(func(sk *sketch.Sketch) {
		if err := sk.SetThreadSuppressed(g.get(sk).H, on); err != nil {
			fail("%v", err)
		}
	})
}

// MoveBefore moves the thread, all its groups together, to just before
// other in the history, as one undo step, as the arrows of its row in
// the group list do. A move that would put a group ahead of a group it
// uses is refused with an error naming both, and nothing changes.
func (v ThreadGroup) MoveBefore(other Group) (err error) {
	g := v.g
	if other.H.V == 0 {
		fail("no group to move before")
	}
	other.d = g.d
	g.d.read(func(sk *sketch.Sketch) {
		other.get(sk)
		err = sk.CheckMoveThread(g.get(sk).H, other.H)
	})
	if err != nil {
		return err
	}
	g.d.change(func(sk *sketch.Sketch) { err = sk.MoveThread(g.H, other.H) })
	return err
}

// Delete deletes the whole thread, every one of its groups and what
// uses them, in one step: the part is left as it was before the thread.
func (v ThreadGroup) Delete() {
	g := v.g
	g.d.change(func(sk *sketch.Sketch) {
		hs, err := sk.ClearThreadItems(g.get(sk).H)
		if err != nil {
			fail("%v", err)
		}
		// The last first, so that no group is deleted for what it used.
		for _, h := range slices.Backward(hs) {
			if sk.Group(h) != nil {
				g.d.h.DeleteGroup(h)
			}
		}
	})
}
