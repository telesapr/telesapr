package sketch

import (
	"errors"
	"fmt"
	"math"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
)

// SetGroupLinkAsSolid has a linked STEP file read as a solid (on) or as
// a reference mesh, and reads the file again. A mesh forces its group to
// one; a solid that was one no longer. The caller remembers the undo
// state.
func (sk *Sketch) SetGroupLinkAsSolid(hg core.HGroup, on bool) error {
	g := sk.Group(hg)
	if g == nil {
		return fmt.Errorf("no group %08x", hg.V)
	}
	if g.Type != GroupLinked {
		return errors.New("only a link is read as a solid or as a mesh")
	}
	g.LinkAsSolid = on
	if sk.ReloadAllLinked != nil {
		sk.ReloadAllLinked(sk)
	}
	if on && len(g.ImpShell.Surface) > 0 {
		g.ForceToMesh = false
	}
	sk.MarkGroupDirty(hg, false)
	return nil
}

// SetSweepUp holds a sweep's frames to the direction up, made a unit
// vector, or lets them turn least when up is zero. The caller remembers
// the undo state.
func (sk *Sketch) SetSweepUp(hg core.HGroup, up geom.Vector) error {
	g := sk.Group(hg)
	if g == nil {
		return fmt.Errorf("no group %08x", hg.V)
	}
	if g.Type != GroupSweep {
		return errors.New("only a sweep has frames to hold")
	}
	if s := up.X + up.Y + up.Z; math.IsNaN(s) || math.IsInf(s, 0) {
		return fmt.Errorf("an up direction is a vector of numbers, not %v", up)
	}
	if up.Magnitude() > 0 {
		up = up.WithMagnitude(1)
	}
	g.SweepUp = up
	sk.MarkGroupDirty(hg, false)
	return nil
}
