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

// GroupThread is what made a thread, kept on its helix group (on its
// sketch, for a cosmetic thread): the circle it is on and its options.
// A group that keeps no thread has none (IsThread).
type GroupThread struct {
	Circle core.HEntity
	ThreadOptions
}

// IsThread reports whether the group keeps a thread: a modelled
// thread's first helix, or a cosmetic thread's sketch.
func (t GroupThread) IsThread() bool { return t.Circle != core.NoEntity }

// ErrNotThread is returned for a group that is not a thread's helix.
var ErrNotThread = errors.New("not a screw thread's group")

// threadGroupsOf finds the groups of the thread whose helix group is
// hg, checking that they are still the groups a thread is: a sketch, a
// lathe of it, a sketch, and the helix of that; then, as its options
// say, a sketch and a helix of it per further level start, and a sketch
// and a lathe of it for a chamfer.
func (sk *Sketch) threadGroupsOf(hg core.HGroup) (threadGroups, error) {
	hel := sk.Group(hg)
	if hel != nil && hel.Type == GroupDrawingWorkplane && hel.Thread.IsThread() && hel.Thread.Cosmetic {
		return sk.cosmeticGroupsOf(hg), nil
	}
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

	o := hel.Thread.ThreadOptions
	next := slices.Index(sk.GroupOrder, hg) + 1
	// pair is the sketch at next and the group of it after it, of type
	// typ, if they are.
	pair := func(typ GroupType) ([2]core.HGroup, bool) {
		if next+1 >= len(sk.GroupOrder) {
			return [2]core.HGroup{}, false
		}
		p, g := sk.Group(sk.GroupOrder[next]), sk.Group(sk.GroupOrder[next+1])
		if p.Type != GroupDrawingWorkplane || g.Type != typ || g.OpA != p.H {
			return [2]core.HGroup{}, false
		}
		next += 2
		return [2]core.HGroup{p.H, g.H}, true
	}
	if o.levelStarts() {
		// Each further start is the next two groups: a sketch, and a
		// helix of it.
		for range o.Spec.NumStarts() - 1 {
			st, ok := pair(GroupHelix)
			if !ok {
				return tg, errors.New("a start of the thread is gone; edit its groups one by one")
			}
			tg.starts = append(tg.starts, st)
		}
	}
	if o.Chamfer {
		ch, ok := pair(GroupLathe)
		if !ok {
			return tg, errors.New("the thread's chamfer is gone; edit its groups one by one")
		}
		tg.chamfer = ch
	}
	return tg, nil
}

// ThreadGroups returns the thread's groups, from its blank sketch to
// its last, hg being the group that keeps it: four, then a sketch and a
// helix per further level start, then a sketch and a lathe for a
// chamfer; for a cosmetic thread its sketch, and the lathe after it when
// it turns or bores the part.
func (sk *Sketch) ThreadGroups(hg core.HGroup) ([]core.HGroup, error) {
	tg, err := sk.threadGroupsOf(hg)
	if err != nil {
		return nil, err
	}
	return tg.all(), nil
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
// helices given their pitch and length. The groups of level starts are
// kept when o has as many, and made anew otherwise. What
// the later groups refer to of a request the thread no longer draws (a
// ridge of a start taken away) is reported as broken. A thread made
// cosmetic, or modelled, keeps its first sketch and its lathe, and the
// group that keeps its options is then another (reshapeThread), which
// the result gives. It generates the sketch, and says whether the solid
// came out sound; it changes nothing when o is refused.
func (sk *Sketch) EditThread(hg core.HGroup, o ThreadOptions) (ThreadResult, error) {
	if _, err := sk.threadGroupsOf(hg); err != nil {
		return ThreadResult{}, err
	}
	o = sk.followFace(sk.Group(hg).Thread.Circle, o)
	if err := sk.CheckThreadEdit(hg, o); err != nil {
		return ThreadResult{}, err
	}
	if o.Cosmetic || sk.Group(hg).Thread.Cosmetic {
		return sk.reshapeThread(hg, o)
	}
	return sk.redrawThread(hg, o, 0)
}

// redrawThread is EditThread with options CheckThread has passed,
// trying the ways of drawing it from try on (newThreadDrawer) until the
// solid comes out sound or none is left.
func (sk *Sketch) redrawThread(hg core.HGroup, o ThreadOptions, try int) (ThreadResult, error) {
	for {
		res, d, err := sk.redrawThreadOnce(hg, o, try)
		try++
		if err != nil || res.Sound || !d.retries() || try == d.tries() {
			return res, err
		}
	}
}

// redrawThreadOnce draws the thread again with the try'th plane for its
// ridges.
func (sk *Sketch) redrawThreadOnce(hg core.HGroup, o ThreadOptions, try int) (ThreadResult, *threadDrawer, error) {
	tg, err := sk.threadGroupsOf(hg)
	if err != nil {
		return ThreadResult{}, nil, err
	}
	circle := sk.Group(hg).Thread.Circle
	if !sk.Group(tg.blank).Clean || !sk.Group(hg).Clean {
		sk.GenerateAllTyped(GenerateAll)
	}
	active := sk.Active
	d := sk.newThreadDrawer(circle, o, try)

	// Groups past the first helix are kept only if o has the same: the
	// starts' if as many, the chamfer's if it has one.
	starts := 0
	if o.levelStarts() {
		starts = o.Spec.NumStarts() - 1
	}
	remake := len(tg.starts) != starts
	var gone []core.HGroup
	if remake {
		for _, st := range tg.starts {
			gone = append(gone, st[0], st[1])
		}
		tg.starts = nil
	}
	if tg.chamfered() && !d.l.chamfer {
		gone = append(gone, tg.chamfer[0], tg.chamfer[1])
		tg.chamfer = [2]core.HGroup{}
	}
	sk.dropThreadGroups(gone)

	type sketchAt struct {
		h     core.HGroup
		what  string
		angle float64
		draw  func()
	}
	sketches := []sketchAt{{tg.blank, "blank", 0, d.drawBlank}, {tg.profile, "profile", d.start, d.drawProfile}}
	for i, st := range tg.starts {
		sketches = append(sketches, sketchAt{st[0], startName("profile", i+1), d.startAngle(i + 1), d.drawProfile})
	}
	if tg.chamfered() {
		sketches = append(sketches, sketchAt{tg.chamfer[0], "end", 0, d.drawChamfer})
	}
	helices := []core.HGroup{tg.helix}
	for _, st := range tg.starts {
		helices = append(helices, st[1])
	}

	reuse := map[core.HGroup][]core.HRequest{}
	for _, r := range sk.Requests {
		if slices.ContainsFunc(sketches, func(s sketchAt) bool { return s.h == r.Group }) {
			reuse[r.Group] = append(reuse[r.Group], r.H)
		}
	}
	d.reuse = reuse
	sk.Constraints = slices.DeleteFunc(sk.Constraints, func(c core.Constraint) bool {
		return slices.ContainsFunc(sketches, func(s sketchAt) bool { return s.h == c.Group }) ||
			slices.Contains(helices, c.Group)
	})

	// The sketches' planes turn first, and their workplanes with them,
	// so that the points drawn land in them.
	for _, s := range sketches {
		g := sk.Group(s.h)
		d.placeSketch(g, s.what, s.angle)
		g.Clean = false
	}
	sk.GenerateAllTyped(GenerateRegen)

	for _, s := range sketches {
		d.placeSketch(sk.Group(s.h), s.what, s.angle)
		sk.Active = s.h
		s.draw()
	}
	d.nameSolid(sk.Group(tg.lathe), d.latheName())
	if tg.chamfered() {
		d.nameSolid(sk.Group(tg.chamfer[1]), "chamfer")
	}
	for i, h := range helices {
		hel := sk.Group(h)
		d.setHelix(hel, h, i == 0)
		d.nameSolid(hel, startName("thread", i))
	}

	// What the sketches no longer draw goes, then the helices' copies
	// of the ridges are made where they now are, for their length.
	sk.Requests = slices.DeleteFunc(sk.Requests, func(r Request) bool {
		return slices.Contains(reuse[r.Group], r.H)
	})
	for _, h := range tg.all() {
		sk.Group(h).Clean = false
	}
	sk.markDirtyFrom(tg.blank)
	sk.Active = tg.all()[len(tg.all())-1]
	sk.GenerateAllTyped(GenerateRegen)
	for _, h := range helices {
		sk.Active = h
		d.dimensionHelix(sk.Group(h))
	}

	// Starts the thread didn't have are made after its first helix, a
	// chamfer it didn't have after its last start.
	if remake {
		sk.Active = tg.helix
		d.addStarts(&tg)
	}
	if d.l.chamfer && !tg.chamfered() {
		sk.Active = tg.helix
		if len(tg.starts) > 0 {
			sk.Active = tg.starts[len(tg.starts)-1][1]
		}
		d.addChamfer(&tg)
	}
	sk.Active = active
	if sk.Group(active) == nil {
		all := tg.all()
		sk.Active = all[len(all)-1]
	}

	sk.GenerateAllTyped(GenerateAll)
	return ThreadResult{Group: hg, Sound: sk.threadIsSound(tg)}, d, nil
}

// dropThreadGroups deletes the groups of a thread that it no longer
// has, with their requests and constraints: those go with their groups,
// as part of the edit, and are not counted as what the user's deletion
// took with it (Sketch.Deleted), which the window would report. What
// later groups lose by it, regeneration counts as ever.
func (sk *Sketch) dropThreadGroups(gone []core.HGroup) {
	if len(gone) == 0 {
		return
	}
	deleted := sk.Deleted
	sk.Groups = slices.DeleteFunc(sk.Groups, func(g Group) bool { return slices.Contains(gone, g.H) })
	sk.PruneOrphans()
	sk.Deleted = deleted
	sk.GenerateAllTyped(GenerateRegen)
}

// ThreadOf returns the options of the thread that group hg keeps,
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
