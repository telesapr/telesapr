package sketch

import (
	"errors"
	"fmt"
	"slices"

	"github.com/telesapr/telesapr/pkg/core"
)

// A thread is four ordinary groups, which a user can edit one by one.
// Its helix group also keeps the circle and the options it was made
// with, and EditThread draws the four again from new options: the
// thread is then edited as one, in one step, its groups and the
// requests it draws keeping their handles where its shape allows.

// GroupThread is what made a thread, kept on its helix group: the
// circle it is on and its options. A group that is not a thread's
// helix has none (IsThread).
type GroupThread struct {
	Circle core.HEntity
	ThreadOptions
}

// IsThread reports whether the group is a thread's helix.
func (t GroupThread) IsThread() bool { return t.Circle != core.NoEntity }

// ErrNotThread is returned for a group that is not a thread's helix.
var ErrNotThread = errors.New("not a screw thread's group")

// threadGroupsOf finds the four groups of the thread whose helix group
// is hg, checking that they are still the groups a thread is: a
// sketch, a lathe of it, a sketch, and the helix of that.
func (sk *Sketch) threadGroupsOf(hg core.HGroup) (threadGroups, error) {
	hel := sk.Group(hg)
	if hel == nil || hel.Type != GroupHelix || !hel.Thread.IsThread() {
		return threadGroups{}, ErrNotThread
	}
	tg := threadGroups{helix: hg, profile: hel.OpA}
	at := slices.Index(sk.GroupOrder, tg.profile)
	profile := sk.Group(tg.profile)
	if at < 1 || profile == nil || profile.Type != GroupDrawingWorkplane {
		return tg, errors.New("the thread's ridge sketch is gone; edit its groups one by one")
	}
	tg.lathe = sk.GroupOrder[at-1]
	lathe := sk.Group(tg.lathe)
	if lathe.Type != GroupLathe {
		return tg, errors.New("the thread's turning is gone; edit its groups one by one")
	}
	tg.blank = lathe.OpA
	if blank := sk.Group(tg.blank); blank == nil || blank.Type != GroupDrawingWorkplane {
		return tg, errors.New("the thread's blank sketch is gone; edit its groups one by one")
	}
	return tg, nil
}

// ThreadGroups returns the thread's four groups, from its blank sketch
// to its helix, hg being the helix.
func (sk *Sketch) ThreadGroups(hg core.HGroup) ([]core.HGroup, error) {
	tg, err := sk.threadGroupsOf(hg)
	if err != nil {
		return nil, err
	}
	return []core.HGroup{tg.blank, tg.lathe, tg.profile, tg.helix}, nil
}

// ThreadLengthText reads a thread's length as the user typed it, in
// the document's unit: a plain number, or an expression of the
// parameters, which it evaluates. It returns the length in millimetres
// and the expression to keep, "" for a plain number.
func (sk *Sketch) ThreadLengthText(text string) (float64, string, error) {
	text = cleanText(text)
	e, names, err := sk.parseNamed(text)
	if err != nil {
		return 0, "", err
	}
	if v, plain := plainNumber(text, names); plain {
		return v * sk.Units.MmPer(), "", nil
	}
	vars, _ := sk.evaluateTable()
	return e.Eval(nil, vars) * sk.Units.MmPer(), text, nil
}

// EditThread makes the thread whose helix group is hg again with the
// options o: its sketches are drawn again in their planes, the
// requests they had taken first, their dimensions made anew, and the
// helix given its pitch and length. What the later groups refer to of
// a request the thread no longer draws (a ridge of a start taken
// away) is reported as broken. It generates the sketch, and says
// whether the solid came out sound; it changes nothing when o is
// refused.
func (sk *Sketch) EditThread(hg core.HGroup, o ThreadOptions) (ThreadResult, error) {
	tg, err := sk.threadGroupsOf(hg)
	if err != nil {
		return ThreadResult{}, err
	}
	circle := sk.Group(hg).Thread.Circle
	if err := sk.CheckThread(circle, o); err != nil {
		return ThreadResult{}, err
	}
	if !sk.Group(tg.blank).Clean || !sk.Group(hg).Clean {
		sk.GenerateAllTyped(GenerateAll)
	}

	reuse := map[core.HGroup][]core.HRequest{}
	for _, r := range sk.Requests {
		if r.Group == tg.blank || r.Group == tg.profile {
			reuse[r.Group] = append(reuse[r.Group], r.H)
		}
	}
	sk.Constraints = slices.DeleteFunc(sk.Constraints, func(c core.Constraint) bool {
		return c.Group == tg.blank || c.Group == tg.profile || c.Group == tg.helix
	})
	active := sk.Active
	d := sk.newThreadDrawer(circle, o, reuse)

	// The sketches' planes turn first, and their workplanes with them,
	// so that the points drawn land in them.
	blank, profile := sk.Group(tg.blank), sk.Group(tg.profile)
	d.placeSketch(blank, "blank", 0)
	d.placeSketch(profile, "profile", d.start)
	blank.Clean, profile.Clean = false, false
	sk.GenerateAllTyped(GenerateRegen)
	blank, profile = sk.Group(tg.blank), sk.Group(tg.profile)

	d.placeSketch(blank, "blank", 0)
	sk.Active = tg.blank
	d.drawBlank()
	d.nameSolid(sk.Group(tg.lathe), d.latheName())
	d.placeSketch(profile, "profile", d.start)
	sk.Active = tg.profile
	d.drawProfile()
	hel := sk.Group(hg)
	d.setHelix(hel, hg)
	d.nameSolid(hel, "thread")

	// What the sketches no longer draw goes, then the helix's copies
	// of the ridges are made where they now are, for its length.
	sk.Requests = slices.DeleteFunc(sk.Requests, func(r Request) bool {
		return slices.Contains(reuse[r.Group], r.H)
	})
	for _, h := range []core.HGroup{tg.blank, tg.lathe, tg.profile, tg.helix} {
		sk.Group(h).Clean = false
	}
	sk.markDirtyFrom(tg.blank)
	sk.Active = hg
	sk.GenerateAllTyped(GenerateRegen)
	d.dimensionHelix(sk.Group(hg))
	sk.Active = active
	if sk.Group(active) == nil {
		sk.Active = hg
	}

	sk.GenerateAllTyped(GenerateAll)
	sound := sk.solidIsSound(tg.lathe) && sk.solidIsSound(tg.helix)
	return ThreadResult{Group: hg, Sound: sound}, nil
}

// ThreadOf returns the options of the thread whose helix group is hg,
// with its length as its expression now gives it.
func (sk *Sketch) ThreadOf(hg core.HGroup) (ThreadOptions, error) {
	g := sk.Group(hg)
	if g == nil {
		return ThreadOptions{}, fmt.Errorf("no group %08x", hg.V)
	}
	if !g.Thread.IsThread() {
		return ThreadOptions{}, ErrNotThread
	}
	return g.Thread.ThreadOptions, nil
}
