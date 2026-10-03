package sketch

import (
	"fmt"

	"github.com/telesapr/telesapr/pkg/core"
)

// This file holds whole-group suppression: a group left out of the
// model with everything that uses it, with nothing deleted.

// SetSuppressAll suppresses group h whole, or brings it back, and has
// the model regenerated. The references can't be suppressed. The
// caller remembers the undo state.
func (sk *Sketch) SetSuppressAll(h core.HGroup, on bool) error {
	g := sk.GetGroup(h)
	if g == nil {
		return fmt.Errorf("no group g%03x", h.V)
	}
	if h.V == core.HGroupReferences {
		return fmt.Errorf("the references can't be suppressed")
	}
	if g.SuppressAll == on {
		return nil
	}
	g.SuppressAll = on
	sk.MarkGroupDirty(h, false)
	return nil
}

// SuppressedReason says why group h is left out of the model, or ""
// if it is generated: "suppressed", or "uses g003-extrude, which is
// suppressed".
func (sk *Sketch) SuppressedReason(h core.HGroup) string {
	g := sk.GetGroup(h)
	if g == nil || g.SuppressedBy.V == 0 {
		return ""
	}
	if g.SuppressedBy == h {
		return "suppressed"
	}
	return fmt.Sprintf("uses %s, which is suppressed", sk.groupName(g.SuppressedBy))
}

// markSuppressed sets every group's SuppressedBy: the group itself if
// it is suppressed whole, else the first group it uses that is left
// out. If the active group is left out, the nearest group that is not
// becomes active, since nothing can be drawn in it.
func (sk *Sketch) markSuppressed() {
	any := false
	for i := range sk.Groups {
		g := &sk.Groups[i]
		g.SuppressedBy = core.HGroup{}
		if g.SuppressAll && g.H.V != core.HGroupReferences {
			any = true
		}
	}
	if !any {
		return
	}
	for _, hg := range sk.GroupOrder {
		g := sk.GetGroup(hg)
		if g.SuppressAll && hg.V != core.HGroupReferences {
			g.SuppressedBy = hg
			continue
		}
		for _, d := range sk.GroupDependencies(hg) {
			if dg := sk.GetGroup(d.Group); dg != nil && dg.SuppressedBy.V != 0 && dg.Order < g.Order {
				g.SuppressedBy = dg.SuppressedBy
				break
			}
		}
	}

	if a := sk.GetGroup(sk.Active); a != nil && a.SuppressedBy.V != 0 {
		sk.Active = sk.nearestGenerated(sk.Active)
	}
}

// nearestGenerated is the last group before h that is generated, but
// the references, or else the first after it, or else the references.
func (sk *Sketch) nearestGenerated(h core.HGroup) core.HGroup {
	var before, after core.HGroup
	seen := false
	for _, hg := range sk.GroupOrder {
		if hg == h {
			seen = true
			continue
		}
		if hg.V == core.HGroupReferences || sk.GetGroup(hg).SuppressedBy.V != 0 {
			continue
		}
		if !seen {
			before = hg
		} else if after.V == 0 {
			after = hg
		}
	}
	switch {
	case before.V != 0:
		return before
	case after.V != 0:
		return after
	}
	return core.HGroup{V: core.HGroupReferences}
}

// skipGroup is the regeneration of a group left out: it keeps the
// values of its parameters, which are in prev, for when it comes back;
// it makes no entities and has nothing to solve; and its solid is the
// solid so far.
func (sk *Sketch) skipGroup(g *Group, prev []core.Param, inRange, genForBBox bool) {
	for _, p := range prev {
		if sk.paramGroup(p.H) == g.H {
			p.Known = true
			sk.Params = append(sk.Params, p)
		}
	}
	sk.sortByHandle()

	g.PolyLoops.Clear()
	g.BezierLoops.Clear()
	g.BezierOpens = nil
	g.PolyError.How = int(core.PolyGood)
	g.Solved.How = core.SolveOkay
	g.Solved.Dof = 0
	g.Solved.Remove = nil
	g.Solved.Timeout = false
	g.SketchDiagnostic = nil
	g.BooleanFailed = false
	if !genForBBox && inRange {
		g.GenerateShellAndMesh(sk)
		g.Clean = true
	}
}

// paramGroup is the group whose request, constraint or own generation
// makes parameter h, or zero if that is unknown.
func (sk *Sketch) paramGroup(h core.HParam) core.HGroup {
	switch {
	case h.V&0x80000000 != 0:
		return core.HGroup{V: (h.V >> 16) & 0x3FFF}
	case h.V&0x40000000 != 0:
		if c := sk.GetConstraint(core.HConstraint{V: h.V &^ 0x40000000}); c != nil {
			return c.Group
		}
	default:
		if r := sk.GetRequest(h.Request()); r != nil {
			return r.Group
		}
	}
	return core.HGroup{}
}
