package sketch

import (
	"errors"
	"slices"

	"github.com/telesapr/telesapr/pkg/core"
)

// A thread's groups can be moved one by one in the history, as any
// group can. Moved apart, the thread is no longer edited as one
// (threadGroupsOf finds its groups by where they stand), but its helix
// still keeps what made it, and moved back together it is a thread
// again. ThreadGroupsApart finds them wherever they stand, so that File
// > Export > Script can make the thread with its call and move its
// groups back apart, as they were.

// ErrThreadGroupsUnclear is returned where the groups of a thread
// whose groups were moved apart can't be told from others: one of them
// is gone, or another group is made as they are, as a second thread's
// on the same circle.
var ErrThreadGroupsUnclear = errors.New("the thread's groups can't be told apart from others")

// ThreadGroupsApart returns the groups of the thread that group hg
// keeps, in the order ThreadGroups gives them, also where some were
// moved on their own in the history, so that ThreadGroups refuses them,
// or, for a cosmetic thread, leaves its lathe out. Each is found by what
// it is made of, not by where it stands: a sketch through the thread's
// circle's centre, and a lathe or a helix of it about the circle's axis;
// the thread's turning was made before its helix and its chamfer after.
func (sk *Sketch) ThreadGroupsApart(hg core.HGroup) ([]core.HGroup, error) {
	g := sk.Group(hg)
	if g == nil || !g.Thread.IsThread() {
		return nil, ErrNotThread
	}
	if !g.Thread.Cosmetic {
		if hs, err := sk.ThreadGroups(hg); err == nil {
			return hs, nil
		}
	}
	circle := sk.Entity(g.Thread.Circle)
	if circle == nil {
		return nil, ErrThreadGroupsUnclear
	}
	center, normal := circle.Point[0], circle.Normal
	// made are the groups about the thread's axis of a sketch through
	// its centre, but the one that keeps the thread.
	isSketch := func(s *Group) bool {
		return s != nil && s.Type == GroupDrawingWorkplane && s.Subtype == GroupWorkplaneByPointOrtho &&
			s.Predef.Origin == center
	}
	var made []*Group
	for i := range sk.Groups {
		x := &sk.Groups[i]
		if x.H == hg || (x.Type != GroupLathe && x.Type != GroupHelix) ||
			x.Predef.Origin != center || x.Predef.EntityB != normal || !isSketch(sk.Group(x.OpA)) {
			continue
		}
		made = append(made, x)
	}
	slices.SortFunc(made, func(a, b *Group) int { return int(a.H.V) - int(b.H.V) })

	if g.Thread.Cosmetic {
		// Its sketch, and the lathe of it where the part is cut.
		hs := []core.HGroup{hg}
		for _, x := range made {
			if x.OpA == hg {
				if x.Type != GroupLathe || len(hs) > 1 {
					return nil, ErrThreadGroupsUnclear
				}
				hs = append(hs, x.H)
			}
		}
		return hs, nil
	}

	if g.Type != GroupHelix || !isSketch(sk.Group(g.OpA)) {
		return nil, ErrThreadGroupsUnclear
	}
	tg := threadGroups{profile: g.OpA, helix: hg}
	var lathes, chamfers [][2]core.HGroup
	for _, x := range made {
		pair := [2]core.HGroup{x.OpA, x.H}
		switch {
		case x.Type == GroupLathe && x.H.V < hg.V:
			lathes = append(lathes, pair)
		case x.Type == GroupLathe:
			chamfers = append(chamfers, pair)
		case x.Thread.IsThread():
			// Another thread's, on the same circle.
			return nil, ErrThreadGroupsUnclear
		default:
			tg.starts = append(tg.starts, pair)
		}
	}
	o := g.Thread.ThreadOptions
	starts, chamfer := 0, 0
	if o.levelStarts() {
		starts = o.Spec.NumStarts() - 1
	}
	if o.Chamfer {
		chamfer = 1
	}
	if len(lathes) != 1 || len(tg.starts) != starts || len(chamfers) != chamfer {
		return nil, ErrThreadGroupsUnclear
	}
	tg.blank, tg.lathe = lathes[0][0], lathes[0][1]
	if chamfer == 1 {
		tg.chamfer = chamfers[0]
	}
	return tg.all(), nil
}

// ThreadDefaultNames returns the name the thread's call gives each of
// the groups of the thread whose helix group is hg, as ThreadGroupsApart
// lists them, and its datum's, which a thread made on a face makes
// first: what a name is compared with to tell whether the user changed
// it. The map is empty when the groups can't be found.
func (sk *Sketch) ThreadDefaultNames(hg core.HGroup) map[core.HGroup]string {
	hs, err := sk.ThreadGroupsApart(hg)
	if err != nil {
		return nil
	}
	o := sk.Group(hg).Thread.ThreadOptions
	d := threadDrawer{o: o}
	var names []string
	if o.Cosmetic {
		names = []string{d.name("cosmetic"), d.name(d.latheName())}
	} else {
		names = []string{d.name("blank"), d.name(d.latheName()), d.name("profile"), d.name("thread")}
		if o.levelStarts() {
			for i := 1; i < o.Spec.NumStarts(); i++ {
				names = append(names, d.name(startName("profile", i)), d.name(startName("thread", i)))
			}
		}
		if o.Chamfer {
			names = append(names, d.name("end"), d.name("chamfer"))
		}
	}
	out := map[core.HGroup]string{}
	for i, h := range hs {
		if i < len(names) {
			out[h] = names[i]
		}
	}
	if c := sk.Entity(sk.Group(hg).Thread.Circle); c != nil {
		if _, onFace := sk.ThreadPlaceOf(hg); onFace {
			out[c.Group] = d.name("end")
		}
	}
	return out
}
