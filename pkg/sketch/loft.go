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
// a circle (four arcs) to a square, and so on. Its solid joins the
// solid so far by its combine, as an extrusion's does.

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
// to, both sketches in workplanes with one closed loop of the same
// number of curves. It is what the interface and scripts add with
// AddGroupAfterActive.
func (sk *Sketch) NewLoft(from, to core.HGroup, name string) (Group, error) {
	g := NewGroup()
	g.Type = GroupLoft
	g.Name = name
	if name == "" {
		g.Name = "loft"
	}
	g.OpA, g.OpB = from, to
	g.MeshCombine = CombineUnion
	g.Color = core.NewRgbaColor(100, 100, 100, 255)
	if from == to {
		return g, errors.New("a loft joins two profiles, and both are the same group")
	}
	a, err := sk.loftLoop(from)
	if err != nil {
		return g, err
	}
	b, err := sk.loftLoop(to)
	if err != nil {
		return g, err
	}
	if _, err := topo.MakeRuledLoft(a, b, g.Color, 0); err != nil {
		return g, err
	}
	return g, nil
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

// generateLoftShell makes the loft's shell, or says why it can't.
func (g *Group) generateLoftShell(sk *Sketch) {
	a, err := sk.loftLoop(g.OpA)
	if err != nil {
		g.setProblem(nil, "%v", err)
		return
	}
	b, err := sk.loftLoop(g.OpB)
	if err != nil {
		g.setProblem(nil, "%v", err)
		return
	}
	if len(a) != len(b) {
		g.setProblem(nil, "the profiles have %d and %d curves; a loft joins them one to one", len(a), len(b))
		return
	}
	k := g.LoftTurn(len(b))
	b = append(b[k:], b[:k]...)
	sh, err := topo.MakeRuledLoft(a, b, g.Color, srf.OriginFor(g.H.V, 0))
	if err != nil {
		g.setProblem(nil, "%v", err)
		return
	}
	g.ThisShell = *sh
}
