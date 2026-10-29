package sketch

import (
	"errors"
	"fmt"
	"math"
	"slices"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/topo"
)

// A loft joins two profiles, each the closed loop of a sketch in a
// workplane, OpA and OpB, by ruled patches, curve i of the first to
// curve i + turn of the second, turn being ValA: a square to a square,
// a circle (four arcs) to a square, and so on. Profiles of unequal
// counts have their longest curves split until they match
// (topo.MatchCounts). With profiles LoftThrough between the two, the
// loft is one smooth surface through them all, in order (topo.
// MakeSmoothLoft), each profile after the first turned by ValA; guide
// curves Edges hold it (loftguide.go), and LoftThroughTangency holds it
// square to profiles between the ends. A closed loft (LoftClosed) goes
// from the last profile back to the first, a ring with no caps, held
// square to those between its first and last and by guides going round
// it, as an open loft is. Its
// solid joins the solid so far by its combine, as an extrusion's does.

// loftLoop is the one closed loop of a profile group, or why there is
// none.
func (sk *Sketch) loftLoop(hg core.HGroup) ([]srf.SBezier, error) {
	g := sk.Group(hg)
	if g == nil {
		return nil, fmt.Errorf("group %s is gone", sk.anyGroupName(hg))
	}
	if g.Type != GroupDrawingWorkplane {
		return nil, fmt.Errorf("%s is not a sketch in a workplane", g.DescriptionString())
	}
	if g.PolyError.How != int(core.PolyGood) {
		return nil, fmt.Errorf("the sketch of %s is not closed loops", g.DescriptionString())
	}
	sets := g.BezierLoops.Sets
	switch {
	case len(sets) == 0:
		return nil, fmt.Errorf("%s has no closed loop", g.DescriptionString())
	case len(sets) > 1 || len(sets[0].Holes) > 0:
		return nil, fmt.Errorf("%s has more than one loop; a loft joins one loop to one", g.DescriptionString())
	}
	return append([]srf.SBezier(nil), sets[0].Outer.Curves...), nil
}

// NewLoft starts a loft from the profile of group from to that of group
// to, both sketches in workplanes with one closed loop. It is what the
// interface and scripts add with AddGroupAfterActive.
func (sk *Sketch) NewLoft(from, to core.HGroup, name string) (Group, error) {
	return sk.NewSmoothLoft([]core.HGroup{from, to}, name)
}

// NewSmoothLoft starts a loft through the profiles of groups, two or
// more, in order: two make a ruled loft, more a smooth one.
func (sk *Sketch) NewSmoothLoft(profiles []core.HGroup, name string) (Group, error) {
	g := NewGroup()
	g.Type = GroupLoft
	g.Name = name
	if name == "" {
		g.Name = "loft"
	}
	g.MeshCombine = CombineUnion
	g.Color = core.NewRGBA(100, 100, 100, 255)
	if len(profiles) < 2 {
		return g, errors.New("a loft joins two profiles at least")
	}
	g.OpA, g.OpB = profiles[0], profiles[len(profiles)-1]
	g.LoftThrough = append([]core.HGroup(nil), profiles[1:len(profiles)-1]...)
	seen := map[core.HGroup]bool{}
	for _, h := range profiles {
		if seen[h] {
			return g, fmt.Errorf("the loft takes %s twice", sk.anyGroupName(h))
		}
		seen[h] = true
	}
	if _, err := g.loftShell(sk, 0); err != nil {
		return g, err
	}
	return g, nil
}

// SetLoftProfiles makes the loft hg go through profiles, two or more
// sketches in order, instead of the profiles it goes through now: a
// loft's profiles can be changed after it is made (ROADMAP F7). A
// profile between the ends that the loft is held square to stays so
// where it is kept; its guides stay. The loft is tried as NewSmoothLoft
// tries it, so profiles it can't join are an error here and nothing
// changes. The caller remembers the undo state.
func (sk *Sketch) SetLoftProfiles(hg core.HGroup, profiles []core.HGroup) error {
	g, err := sk.inputGroup(hg, GroupLoft, "profiles")
	if err != nil {
		return err
	}
	if len(profiles) < 2 {
		return errors.New("a loft joins two profiles at least")
	}
	if g.LoftClosed && len(profiles) < 3 {
		return errors.New("a closed loft needs three profiles at least")
	}
	if slices.Equal(profiles, g.LoftProfiles()) {
		return nil
	}
	seen := map[core.HGroup]bool{}
	for _, h := range profiles {
		if seen[h] {
			return fmt.Errorf("the loft takes %s twice", sk.anyGroupName(h))
		}
		seen[h] = true
		if sk.Group(h) == nil {
			return fmt.Errorf("there is no group %s", sk.anyGroupName(h))
		}
		if err := sk.inputBefore(hg, h, sk.anyGroupName(h)); err != nil {
			return err
		}
	}
	through := profiles[1 : len(profiles)-1]
	middle := make([]float64, len(through))
	for i, h := range through {
		middle[i] = g.LoftProfileTangency(h)
	}
	if !slices.ContainsFunc(middle, func(x float64) bool { return x != 0 }) {
		middle = nil
	}
	trial := *g
	trial.OpA, trial.OpB = profiles[0], profiles[len(profiles)-1]
	trial.LoftThrough, trial.LoftThroughTangency = slices.Clone(through), middle
	if _, err := trial.loftShell(sk, 0); err != nil {
		return err
	}
	g.OpA, g.OpB = trial.OpA, trial.OpB
	g.LoftThrough, g.LoftThroughTangency = trial.LoftThrough, middle
	sk.MarkGroupDirty(hg, false)
	return nil
}

// LoftEnds is how the loft leaves its first profile and reaches its
// last: ValB and ValC, square to each profile by that much, 0 free
// (roadmap 28.3).
func (g *Group) LoftEnds() topo.LoftEnds { return topo.LoftEnds{Start: g.ValB, End: g.ValC} }

// loftEnds is LoftEnds as the loft is made: an end along the solid's
// sides and left at 0 leaves along them by 1.
func (g *Group) loftEnds() topo.LoftEnds {
	ends := g.LoftEnds()
	if g.LoftAlongSides[0] && ends.Start == 0 {
		ends.Start = 1
	}
	if g.LoftAlongSides[1] && ends.End == 0 {
		ends.End = 1
	}
	return ends
}

// SetLoftAlongSides has the loft leave its first profile (start) and
// reach its last (end) along the sides of the solid each is drawn on,
// carrying them on smoothly, or square to the profile's plane: what the
// loft's screen and LoftGroup.SetAlongSides set. An end along the sides
// whose tangency is 0 takes 1. The caller remembers the undo state.
func (sk *Sketch) SetLoftAlongSides(hg core.HGroup, start, end bool) error {
	g := sk.Group(hg)
	if g == nil {
		return fmt.Errorf("no group %08x", hg.V)
	}
	if g.Type != GroupLoft {
		return fmt.Errorf("%s is not a loft", g.DescriptionString())
	}
	if (start || end) && g.LoftClosed {
		return errClosedEnds
	}
	g.LoftAlongSides = [2]bool{start, end}
	sk.MarkGroupDirty(hg, false)
	return nil
}

// loftSides are the directions the loft leaves and reaches its end
// profiles in, along the sides of the solid before it, where it says
// so.
func (g *Group) loftSides(sk *Sketch, loops [][]srf.SBezier) (topo.LoftSides, error) {
	var sides topo.LoftSides
	if !g.LoftAlongSides[0] && !g.LoftAlongSides[1] {
		return sides, nil
	}
	before := g.PreviousGroup(sk)
	if before == nil {
		before = sk.Group(sk.Active)
	}
	var solid *Group
	if before != nil {
		solid = sk.solidAt(before.H)
	}
	if solid == nil || len(solid.RunningShell.Surface) == 0 {
		return sides, errors.New("there is no solid before the loft for it to follow the sides of")
	}
	// Each end's normal away from the solid, toward the loft.
	away := func(l, toward []srf.SBezier) geom.Vector {
		n := topo.LoopNormal(l)
		if n.Dot(topo.LoopCentroid(toward).Minus(topo.LoopCentroid(l))) < 0 {
			n = n.ScaledBy(-1)
		}
		return n
	}
	k := len(loops)
	if g.LoftAlongSides[0] {
		f, err := topo.SideTangents(&solid.RunningShell, away(loops[0], loops[1]))
		if err != nil {
			return sides, err
		}
		sides.Start = f
	}
	if g.LoftAlongSides[1] {
		f, err := topo.SideTangents(&solid.RunningShell, away(loops[k-1], loops[k-2]))
		if err != nil {
			return sides, err
		}
		// The way the loft goes arriving: back against the way out.
		sides.End = func(p geom.Vector) (geom.Vector, error) {
			d, err := f(p)
			return d.ScaledBy(-1), err
		}
	}
	return sides, nil
}

// LoftProfiles are the groups a loft joins, in order: OpA, LoftThrough
// and OpB.
func (g *Group) LoftProfiles() []core.HGroup {
	return append(append([]core.HGroup{g.OpA}, g.LoftThrough...), g.OpB)
}

// LoftTurn is how many curves on the loft joins the second profile's
// curves turned by, in 0 to the count less one.
func (g *Group) LoftTurn(n int) int {
	if n <= 0 {
		return 0
	}
	k := int(g.ValA) % n
	if k < 0 {
		k += n
	}
	return k
}

// loftShell is the shell of the loft, its faces' origins from origin.
func (g *Group) loftShell(sk *Sketch, origin uint32) (*srf.SShell, error) {
	var loops [][]srf.SBezier
	for _, h := range g.LoftProfiles() {
		l, err := sk.loftLoop(h)
		if err != nil {
			return nil, err
		}
		loops = append(loops, l)
	}
	guides, err := g.loftGuides(sk)
	if err != nil {
		return nil, err
	}
	if g.LoftClosed {
		if err := g.closedLoftConflict(); err != nil {
			return nil, err
		}
		// Round the ring the first and last profiles are as any other,
		// but only those between them are held: a ring has no ends.
		held := append(append([]float64{0}, g.loftMiddle()...), 0)
		if len(guides) > 0 {
			return topo.MakeClosedGuidedLoftHeld(loops, held, guides, g.Color, origin, sk.tol)
		}
		return topo.MakeClosedLoftHeld(loops, held, g.Color, origin, sk.tol)
	}
	middle := g.loftMiddle()
	if len(guides) > 0 {
		sides, err := g.loftSides(sk, loops)
		if err != nil {
			return nil, err
		}
		return topo.MakeGuidedLoftThrough(loops, int(g.ValA), g.loftEnds(), sides, middle, guides, g.Color, origin, sk.tol)
	}
	if ends := g.loftEnds(); len(loops) > 2 || ends != (topo.LoftEnds{}) {
		sides, err := g.loftSides(sk, loops)
		if err != nil {
			return nil, err
		}
		return topo.MakeSmoothLoftThrough(loops, int(g.ValA), ends, sides, middle, g.Color, origin, sk.tol)
	}
	loops = topo.MatchCounts(loops)
	a, b := loops[0], loops[1]
	k := g.LoftTurn(len(b))
	b = append(b[k:], b[:k]...)
	return topo.MakeRuledLoft(a, b, g.Color, origin, sk.tol)
}

// generateLoftShell makes the loft's shell, or says why it can't.
func (g *Group) generateLoftShell(sk *Sketch) {
	sh, err := g.loftShell(sk, srf.OriginFor(g.H.V, 0))
	if err != nil {
		if pe, ok := errors.AsType[*pathError](err); ok {
			g.setProblem(pe.at, "%s", pe.msg)
		} else {
			g.setProblem(nil, "%v", err)
		}
		return
	}
	g.ThisShell = *sh
}

// errClosedEnds is a setting of a loft's ends on a closed loft, which
// has none.
var errClosedEnds = errors.New("a closed loft has no ends to set")

// closedLoftConflict is why the closed loft g can't be made as it is
// set, or nil: a ring has no ends to be square to or to carry a solid's
// sides on and nothing to turn its profiles against. Profiles between
// its first and last can be held square, and the ring held by guides
// going round it, as an open loft is held.
func (g *Group) closedLoftConflict() error {
	switch {
	case len(g.LoftThrough) == 0:
		return errors.New("a closed loft needs three profiles at least")
	case g.ValB != 0 || g.ValC != 0 || g.LoftAlongSides != [2]bool{}:
		return errClosedEnds
	case int(g.ValA) != 0:
		return errors.New("a closed loft can't turn its profiles; draw them starting at the same side")
	}
	return nil
}

// loftMiddle is LoftThroughTangency, one for each profile between the
// ends.
func (g *Group) loftMiddle() []float64 {
	out := make([]float64, len(g.LoftThrough))
	copy(out, g.LoftThroughTangency)
	return out
}

// SetLoftClosed has the smooth loft hg go from its last profile back to
// its first, a ring with no caps, or not: what the loft's screen and
// LoftGroup.SetClosed set. A closed loft goes through three profiles at
// least and has no ends to set. The caller remembers the undo state.
func (sk *Sketch) SetLoftClosed(hg core.HGroup, closed bool) error {
	g := sk.Group(hg)
	if g == nil {
		return fmt.Errorf("no group %08x", hg.V)
	}
	if g.Type != GroupLoft {
		return fmt.Errorf("%s is not a loft", g.DescriptionString())
	}
	if closed && len(g.LoftThrough) == 0 {
		return errors.New("a closed loft needs three profiles at least")
	}
	g.LoftClosed = closed
	sk.MarkGroupDirty(hg, false)
	return nil
}

// SetLoftProfileTangency holds the smooth loft hg square to the plane
// of profile, one of those between its ends, the surface's slope
// across there t times the loft's length, as an end's tangency; 0
// leaves it free. What the loft's screen and
// LoftGroup.SetProfileTangency set. The caller remembers the undo
// state.
func (sk *Sketch) SetLoftProfileTangency(hg, profile core.HGroup, t float64) error {
	g := sk.Group(hg)
	if g == nil {
		return fmt.Errorf("no group %08x", hg.V)
	}
	if g.Type != GroupLoft {
		return fmt.Errorf("%s is not a loft", g.DescriptionString())
	}
	if math.IsNaN(t) || math.IsInf(t, 0) {
		return fmt.Errorf("a loft's tangency is a number, not %g", t)
	}
	i := slices.Index(g.LoftThrough, profile)
	if i < 0 {
		return fmt.Errorf("%s is not a profile between the loft's ends", sk.anyGroupName(profile))
	}
	middle := g.loftMiddle()
	middle[i] = t
	if !slices.ContainsFunc(middle, func(x float64) bool { return x != 0 }) {
		middle = nil
	}
	g.LoftThroughTangency = middle
	sk.MarkGroupDirty(hg, false)
	return nil
}

// LoftProfileTangency is how square the loft holds profile, one of
// those between its ends, 0 for free.
func (g *Group) LoftProfileTangency(profile core.HGroup) float64 {
	if i := slices.Index(g.LoftThrough, profile); i >= 0 && i < len(g.LoftThroughTangency) {
		return g.LoftThroughTangency[i]
	}
	return 0
}
