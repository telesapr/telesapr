package sketch

import (
	"errors"

	"github.com/telesapr/telesapr/pkg/srf"
)

// CopyPlacement is where a group that copies a solid puts one copy: the
// source scaled by Scale about the origin, 1, or -1 for a reflection
// (a point inversion, which the half turn in Move makes a mirror), then
// moved by Move. It is what the kernel's MakeFromTransformationOf takes.
type CopyPlacement struct {
	Move  srf.Rigid
	Scale float64
}

// ErrNotCopies is CopyPlacements' error for a group that makes no
// copies: not a step and repeat, a pattern along a path or a mirror.
var ErrNotCopies = errors.New("the group makes no copies")

// CopyPlacements lists where a step and repeat (translate or rotate), a
// pattern along a path or a mirror puts the copies of its source's solid,
// the copies the kernel unites into the group's own solid (generateFor-
// StepAndRepeatShell, generatePatternShell, generateMirrorShell). ofSolid
// says that the source is the solid so far rather than the source group's
// own, as for a mirror of the whole (mirrorsSolid); the group's solid is
// then the whole, not combined with what came before. The ray-cast oracle
// (pkg/check) rebuilds the copies from these.
func (g *Group) CopyPlacements(sk *Sketch) (places []CopyPlacement, ofSolid bool, err error) {
	switch g.Type {
	case GroupTranslate, GroupRotate:
		n, a0 := g.stepsOf()
		for a := a0; a < n; a++ {
			t, q := g.stepTransform(sk, a, n)
			places = append(places, CopyPlacement{Move: srf.Rigid{Q: q, T: t}, Scale: 1})
		}
		return places, false, nil
	case GroupPathPattern:
		ms, err := g.patternMotions(sk)
		if err != nil {
			return nil, false, err
		}
		for _, m := range ms {
			places = append(places, CopyPlacement{Move: m, Scale: 1})
		}
		return places, false, nil
	case GroupMirror:
		if g.mirrorPlane(sk) == nil {
			return nil, false, errors.New("the mirror plane is not a workplane or a planar face")
		}
		if !g.SkipFirst {
			places = append(places, CopyPlacement{Move: srf.RigidIdentity, Scale: 1})
		}
		t, q := g.mirrorTransform(sk)
		places = append(places, CopyPlacement{Move: srf.Rigid{Q: q, T: t}, Scale: -1})
		return places, g.mirrorsSolid(sk), nil
	}
	return nil, false, ErrNotCopies
}
