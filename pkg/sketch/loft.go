package sketch

import (
	"errors"
	"fmt"

	"github.com/telesapr/telesapr/pkg/core"
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
// curves Edges hold it (loftguide.go). Its
// solid joins the solid so far by its combine, as an extrusion's does.

// loftLoop is the one closed loop of a profile group, or why there is
// none.
func (sk *Sketch) loftLoop(hg core.HGroup) ([]srf.SBezier, error) {
	g := sk.GetGroup(hg)
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
	g.Color = core.NewRgbaColor(100, 100, 100, 255)
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

// LoftEnds is how the loft leaves its first profile and reaches its
// last: ValB and ValC, square to each profile by that much, 0 free
// (roadmap 28.3).
func (g *Group) LoftEnds() topo.LoftEnds { return topo.LoftEnds{Start: g.ValB, End: g.ValC} }

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
	if len(guides) > 0 {
		return topo.MakeGuidedLoft(loops, int(g.ValA), g.LoftEnds(), guides, g.Color, origin)
	}
	if ends := g.LoftEnds(); len(loops) > 2 || ends != (topo.LoftEnds{}) {
		return topo.MakeSmoothLoftEnds(loops, int(g.ValA), ends, g.Color, origin)
	}
	loops = topo.MatchCounts(loops)
	a, b := loops[0], loops[1]
	k := g.LoftTurn(len(b))
	b = append(b[k:], b[:k]...)
	return topo.MakeRuledLoft(a, b, g.Color, origin)
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
