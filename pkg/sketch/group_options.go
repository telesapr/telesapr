package sketch

import (
	"errors"
	"fmt"
	"math"

	"github.com/telesapr/telesapr/pkg/core"
)

// HasCombine reports whether the group's own combine mode is what joins
// its solid to the solid so far. A step and repeat, a mirror and a
// pattern take their source group's mode at each regeneration, a
// chamfer is a cut and a fillet or a shell edits the solid so far, so a
// mode set on one of those would change nothing.
func (g *Group) HasCombine() bool {
	switch g.Type {
	case GroupExtrude, GroupLathe, GroupRevolve, GroupHelix, GroupLinked, GroupSweep, GroupLoft, GroupWrap:
		return true
	}
	return false
}

// HasColor reports whether the group's colour is the colour of the
// faces it makes. A link brings its own colours, and the groups that
// edit or copy the solid so far keep the colours of what they edit.
func (g *Group) HasColor() bool {
	switch g.Type {
	case GroupExtrude, GroupLathe, GroupRevolve, GroupHelix, GroupSweep, GroupLoft, GroupWrap:
		return true
	}
	return false
}

// HasAppearance reports whether the group makes faces of its own to
// give an appearance to: every group that makes a solid does.
func (g *Group) HasAppearance() bool {
	switch g.Type {
	case GroupDrawing3D, GroupDrawingWorkplane:
		return false
	}
	return true
}

// HasSolidSuppress reports whether leaving the group's solid out of the
// model is something it does. A fillet and a shell return the solid so
// far edited in place and have no solid of their own to leave out.
func (g *Group) HasSolidSuppress() bool {
	switch g.Type {
	case GroupDrawing3D, GroupDrawingWorkplane, GroupFillet, GroupShell:
		return false
	}
	return true
}

// Errors of the group options, for callers that say them in their own
// words.
var (
	// ErrNoColor is a colour or opacity asked of a group whose solid
	// has none of its own (see HasColor).
	ErrNoColor = errors.New("this kind of group has no colour of its own")
	// ErrNoCombine is a combine mode asked of a group that has none
	// of its own (see HasCombine).
	ErrNoCombine = errors.New("this kind of group has no combine mode of its own")
)

// SetGroupSkewed makes an extrusion's sides lean with its sketch
// (skewed) or stand straight, in either its one-sided or two-sided
// mode. The caller remembers the undo state.
func (sk *Sketch) SetGroupSkewed(hg core.HGroup, skewed bool) error {
	g := sk.Group(hg)
	if g == nil {
		return fmt.Errorf("no group %08x", hg.V)
	}
	if g.Type != GroupExtrude {
		return errors.New("only an extrusion can be skewed")
	}
	switch {
	case skewed && g.Subtype == GroupOneSided:
		g.Subtype = GroupOneSkewed
	case skewed && g.Subtype == GroupTwoSided:
		g.Subtype = GroupTwoSkewed
	case !skewed && g.Subtype == GroupOneSkewed:
		g.Subtype = GroupOneSided
	case !skewed && g.Subtype == GroupTwoSkewed:
		g.Subtype = GroupTwoSided
	default:
		return nil
	}
	sk.MarkGroupDirty(hg, false)
	return nil
}

// SetGroupSkipFirst has a one-sided step and repeat start with its
// first copy (skip) rather than the original; for a mirror it leaves
// the original out of the solid, so that only the reflection is left.
// The count and a rotate's
// angle are left as they are: the group screen respaces a free rotate
// when its count is typed (SetGroupCopies), not here. The caller
// remembers the undo state.
func (sk *Sketch) SetGroupSkipFirst(hg core.HGroup, skip bool) error {
	g := sk.Group(hg)
	if g == nil {
		return fmt.Errorf("no group %08x", hg.V)
	}
	if g.Type != GroupTranslate && g.Type != GroupRotate && g.Type != GroupMirror {
		return errors.New("only a step and repeat or a mirror can leave out the original")
	}
	if g.Subtype != GroupOneSided {
		return errors.New("a two-sided step and repeat always has the original")
	}
	g.SkipFirst = skip
	sk.MarkGroupDirty(hg, false)
	return nil
}

// SetGroupOpacity sets how opaque the group's solid is, from 0 to 1,
// leaving its colour. The caller remembers the undo state.
func (sk *Sketch) SetGroupOpacity(hg core.HGroup, alpha float64) error {
	g := sk.Group(hg)
	if g == nil {
		return fmt.Errorf("no group %08x", hg.V)
	}
	if math.IsNaN(alpha) || alpha < 0 || alpha > 1 {
		return fmt.Errorf("opacity must be between zero and one, not %g", alpha)
	}
	if !g.HasColor() {
		return ErrNoColor
	}
	g.Color.Alpha = uint8(int(float64(float32(255.1)) * alpha))
	sk.MarkGroupDirty(hg, false)
	return nil
}

// SetGroupColor colours the group's solid with the red, green and blue
// of c, keeping its opacity. The caller remembers the undo state.
func (sk *Sketch) SetGroupColor(hg core.HGroup, c core.RGBA) error {
	g := sk.Group(hg)
	if g == nil {
		return fmt.Errorf("no group %08x", hg.V)
	}
	if !g.HasColor() {
		return ErrNoColor
	}
	g.Color = core.NewRGBA(c.Red, c.Green, c.Blue, g.Color.Alpha)
	sk.MarkGroupDirty(hg, false)
	return nil
}

// SetGroupAppearance gives the group's solid an appearance by name, ""
// for the default. The names are the renderer's, which this package
// does not know, so the caller has checked it. The caller remembers
// the undo state.
func (sk *Sketch) SetGroupAppearance(hg core.HGroup, name string) error {
	g := sk.Group(hg)
	if g == nil {
		return fmt.Errorf("no group %08x", hg.V)
	}
	if !g.HasAppearance() {
		return errors.New("this kind of group makes no solid to give an appearance")
	}
	g.Appearance = name
	return nil
}

// ErrNoSolidSuppress is a solid suppressed of a group that has none of
// its own to leave out (see HasSolidSuppress).
var ErrNoSolidSuppress = errors.New("this kind of group has no solid of its own to suppress")

// SetGroupSuppressSolid leaves the group's solid out of the model, or
// puts it back, keeping the group's sketch and solve. Putting back
// always works. The caller remembers the undo state.
func (sk *Sketch) SetGroupSuppressSolid(hg core.HGroup, on bool) error {
	g := sk.Group(hg)
	if g == nil {
		return fmt.Errorf("no group %08x", hg.V)
	}
	if on && !g.HasSolidSuppress() {
		return ErrNoSolidSuppress
	}
	g.Suppress = on
	sk.MarkGroupDirty(hg, false)
	return nil
}
