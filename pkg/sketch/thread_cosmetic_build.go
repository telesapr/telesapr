package sketch

import (
	"cmp"
	"math"
	"slices"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
)

// A cosmetic thread is recorded on the part rather than modelled turn
// by turn, as drawings show a thread anyway and most threads need no
// more: it costs no helix and no Boolean against coincident flanks.
// Its sketch keeps what made it (Group.Thread), drawn in the plane a
// modelled thread's first sketch is; the part is cut only where it
// must be. A screw's rod thicker than the thread's major diameter is
// turned down to it, and one as thick is left as it is: turning it to
// its own size would lay one cylinder on another, which a Boolean
// can't be sure of. A nut's part is bored to the minor diameter, the
// tap drill's, unless a hole at least that wide is there already along
// the whole thread. A chamfered end is cut by the same lathe, at the
// cone a modelled thread's chamfer has, which a part left at its size
// is lathed for alone. Either cut is a lathe of the sketch, as a
// modelled thread's first lathe is, so editing a thread from one kind
// to the other keeps the two groups and what later groups refer to of
// them.

// cosmeticTurnSlack is how much thicker than the major diameter, in
// millimetres of radius, a screw's rod must be to be turned down: a
// rod of the thread's size, drawn to a micrometre off it, is left.
const cosmeticTurnSlack = 0.01

// cosmeticHoleProbe is the part of the minor radius at which a nut's
// part is probed for a hole already there: within it, a hole meshed at
// its chord tolerance still counts.
const cosmeticHoleProbe = 0.95

// cosmeticGroupsOf are the groups of the cosmetic thread whose sketch
// is hg: the sketch, and the lathe of it that follows, if any.
func (sk *Sketch) cosmeticGroupsOf(hg core.HGroup) threadGroups {
	tg := threadGroups{blank: hg}
	if at := slices.Index(sk.GroupOrder, hg); at >= 0 && at+1 < len(sk.GroupOrder) {
		if next := sk.Group(sk.GroupOrder[at+1]); next.Type == GroupLathe && next.OpA == hg {
			tg.lathe = next.H
		}
	}
	return tg
}

// cosmeticCuts tells whether the cosmetic thread o on the circle he
// cuts the solid of part: a screw's rod thicker than the thread, a
// nut's part with material inside the minor diameter.
func (sk *Sketch) cosmeticCuts(he core.HEntity, o ThreadOptions, l threadLayout, part *Group) bool {
	crest := l.profile.CrestRadius
	if !o.Internal {
		return sk.MustEntity(he).CircleGetRadiusNum(sk) > crest+cosmeticTurnSlack
	}
	if part == nil {
		return true
	}
	origin, along, u, v := sk.threadAxis(he, o)
	return newPartProbe(&part.RunningShell, &part.RunningMesh).occupied(
		origin, along, u, v, cosmeticHoleProbe*crest, o.Length)
}

// occupied tells whether the solid has material anywhere along the
// thread's length, from the circle's plane, at radius r on any of the
// probes.
func (p partProbe) occupied(origin, along, u, v geom.Vector, r, length float64) bool {
	if p.hits == nil {
		return false
	}
	behind := length + p.span
	for _, angle := range threadPartProbes {
		at := origin.Plus(u.ScaledBy(r * math.Cos(angle))).Plus(v.ScaledBy(r * math.Sin(angle)))
		hits := p.hits(at.Minus(along.ScaledBy(behind)), at.Plus(along.ScaledBy(length+1)))
		slices.SortFunc(hits, func(x, y threadHit) int { return cmp.Compare(x.t, y.t) })
		// The line starts outside; each hit enters or leaves.
		inside, from := false, 0.0
		for _, h := range hits {
			leaving := h.normal.Dot(along) > 0
			switch {
			case !inside && !leaving:
				inside, from = true, h.t
			case inside && leaving:
				inside = false
				if min(h.t, behind+length)-max(from, behind) > threadPartSlack {
					return true
				}
			}
		}
		if inside && behind+length-max(from, behind) > threadPartSlack {
			return true
		}
	}
	return false
}

// cosmeticBlank lays out what a cosmetic thread's lathe takes away: a
// screw's rod outside the major diameter, a nut's part inside the
// minor; no run-out, which shapes only the drawing.
func (d *threadDrawer) cosmeticBlank() {
	crest := d.l.profile.CrestRadius
	d.l.runOut = false
	if d.o.Internal {
		d.l.blankIn, d.l.blankOut = 0, crest
	} else {
		d.l.blankIn = crest
	}
}

// addCosmeticThread makes the cosmetic thread o, which CheckThread has
// passed, on the circle he after the active group: its sketch, and the
// lathe of it if the part is to be cut.
func (sk *Sketch) addCosmeticThread(he core.HEntity, o ThreadOptions) ThreadResult {
	d := sk.newThreadDrawer(he, o, 0)
	cut := sk.cosmeticCuts(he, o, d.l, sk.solidAt(sk.ActiveGroup()))
	d.cosmeticBlank()

	var tg threadGroups
	tg.blank = d.addSketch("cosmetic", 0)
	if d.drawCosmeticCut(cut) {
		tg.lathe = d.addSolid(Group{Type: GroupLathe}, d.latheName(), CombineDifference).H
	}
	sk.Group(tg.blank).Thread = GroupThread{Circle: he, ThreadOptions: o}
	sk.GenerateAllTyped(GenerateAll)
	return ThreadResult{Group: tg.blank, Sound: sk.threadIsSound(tg)}
}

// reshapeThread is EditThread for a thread that is or becomes cosmetic,
// with options CheckThreadEdit has passed. The thread's first sketch
// and its lathe are kept, the sketch drawn again (or emptied, where the
// part is no longer cut) and the lathe made or taken away as the part
// needs; a modelled thread's ridges and helices go, or are made after
// them. The group that keeps the thread is the sketch of a cosmetic
// thread and the first helix of a modelled one, and the result gives
// it.
func (sk *Sketch) reshapeThread(hg core.HGroup, o ThreadOptions) (ThreadResult, error) {
	tg, err := sk.threadGroupsOf(hg)
	if err != nil {
		return ThreadResult{}, err
	}
	circle := sk.Group(hg).Thread.Circle
	if !sk.Group(tg.blank).Clean || !sk.Group(hg).Clean {
		sk.GenerateAllTyped(GenerateAll)
	}
	// Where the thread was active, its last group stays so: what is
	// made next goes after it, as after making the thread.
	active, inThread := sk.Active, slices.Contains(tg.all(), sk.Active)
	d := sk.newThreadDrawer(circle, o, 0)
	cut := true
	if o.Cosmetic {
		cut = sk.cosmeticCuts(circle, o, d.l, sk.solidAt(tg.blank))
		d.cosmeticBlank()
	}

	// What the thread no longer has goes: a modelled thread's ridges and
	// helices, and the lathe of a part no longer cut.
	var gone []core.HGroup
	if o.Cosmetic {
		gone = append(gone, tg.extra()...)
		if tg.profile != (core.HGroup{}) {
			gone = append(gone, tg.profile, tg.helix)
		}
		if tg.chamfered() {
			gone = append(gone, tg.chamfer[0], tg.chamfer[1])
		}
		tg.profile, tg.helix, tg.starts, tg.chamfer = core.HGroup{}, core.HGroup{}, nil, [2]core.HGroup{}
	}
	lathe := cut || (o.Cosmetic && d.l.chamfer)
	if !lathe && tg.lathe != (core.HGroup{}) {
		gone = append(gone, tg.lathe)
		tg.lathe = core.HGroup{}
	}
	sk.dropThreadGroups(gone)

	// The sketch is drawn again, taking its requests first.
	blank := sk.Group(tg.blank)
	blank.Thread = GroupThread{}
	var reuse []core.HRequest
	for _, r := range sk.Requests {
		if r.Group == tg.blank {
			reuse = append(reuse, r.H)
		}
	}
	d.reuse = map[core.HGroup][]core.HRequest{tg.blank: reuse}
	sk.Constraints = slices.DeleteFunc(sk.Constraints, func(c core.Constraint) bool { return c.Group == tg.blank })
	what := "blank"
	if o.Cosmetic {
		what = "cosmetic"
	}
	d.placeSketch(blank, what, 0)
	blank.Clean = false
	sk.GenerateAllTyped(GenerateRegen)
	d.placeSketch(sk.Group(tg.blank), what, 0)
	sk.Active = tg.blank
	if o.Cosmetic {
		d.drawCosmeticCut(cut)
	} else {
		d.drawBlank()
	}
	sk.Requests = slices.DeleteFunc(sk.Requests, func(r Request) bool {
		return slices.Contains(d.reuse[tg.blank], r.H)
	})

	if lathe && tg.lathe == (core.HGroup{}) {
		tg.lathe = d.addSolid(Group{Type: GroupLathe}, d.latheName(), CombineDifference).H
	} else if lathe {
		d.nameSolid(sk.Group(tg.lathe), d.latheName())
	}
	keeper := tg.blank
	if o.Cosmetic {
		sk.Group(tg.blank).Thread = GroupThread{Circle: circle, ThreadOptions: o}
	} else {
		// The ridges and helices of a modelled thread follow its lathe.
		sk.Active = tg.lathe
		tg.profile = d.addSketch("profile", d.start)
		d.drawProfile()
		tg.helix = d.addHelix("thread", true).H
		d.addStarts(&tg)
		d.addChamfer(&tg)
		keeper = tg.helix
	}
	for _, h := range tg.all() {
		sk.Group(h).Clean = false
	}
	sk.markDirtyFrom(tg.blank)

	sk.Active = active
	if inThread || sk.Group(active) == nil {
		all := tg.all()
		sk.Active = all[len(all)-1]
	}
	sk.GenerateAllTyped(GenerateAll)
	res := ThreadResult{Group: keeper, Sound: sk.threadIsSound(tg)}
	if !res.Sound && !o.Cosmetic && d.retries() {
		return sk.redrawThread(keeper, o, 1)
	}
	return res, nil
}

// drawCosmeticCut draws, in the active sketch, what a cosmetic
// thread's lathe takes away: the turning or the boring, chamfered at a
// chamfered end, where the part is cut; else only the chamfers. It
// tells whether there is anything to lathe.
func (d *threadDrawer) drawCosmeticCut(cut bool) bool {
	switch {
	case cut:
		d.drawBlank()
	case d.l.chamfer:
		d.drawChamfer()
	default:
		return false
	}
	return true
}
