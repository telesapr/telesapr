package ui

import (
	"fmt"
	"strings"

	"github.com/telesapr/telesapr/pkg/core"
)

// analyzeGroupDependencies runs Analyze > Group Dependencies: what the
// active group is built on, which groups are built on it, which of its
// sources move with it, and the box around its solid. It is the window's
// side of GroupHistory.DependsOn, UsedBy and OwnSources and of
// GroupAnalysis.Bounds.
func (ss *Application) analyzeGroupDependencies() {
	ss.Message("%s", ss.groupDependenciesText(ss.GW.activeGroupH()))
}

// groupDependenciesText is the text of the dialog for group h.
func (ss *Application) groupDependenciesText(h core.HGroup) string {
	sk := ss.SK
	g := sk.Group(h)
	if g == nil {
		return "There is no active group."
	}
	name := func(h core.HGroup) string {
		if g := sk.Group(h); g != nil {
			return g.Name
		}
		return "?"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Group %s\n\n", g.Name)

	var on []string
	for _, d := range sk.GroupDependencies(h) {
		if d.Group.V != core.HGroupReferences {
			on = append(on, fmt.Sprintf("%s (%s)", name(d.Group), d.Why))
		}
	}
	list := func(label string, items []string) {
		if len(items) == 0 {
			fmt.Fprintf(&b, "%s: none\n", label)
			return
		}
		fmt.Fprintf(&b, "%s:\n", label)
		for _, it := range items {
			fmt.Fprintf(&b, "    %s\n", it)
		}
	}
	list("Built on", on)

	var by []string
	for _, u := range sk.GroupDependents(h) {
		by = append(by, name(u))
	}
	list("Used by", by)

	var own []string
	for _, o := range sk.OwnSources(h) {
		if o != h {
			own = append(own, name(o))
		}
	}
	list("Moves with its own sources", own)

	b.WriteString("\n")
	g.GenerateDisplayItems(sk)
	m := &g.DisplayMesh
	if m.IsEmpty() {
		b.WriteString("The group has no solid, so no bounding box.")
		return b.String()
	}
	hi, lo := m.Bounding()
	fmt.Fprintf(&b, "Bounding box of the solid up to this group:\n    from (%s, %s, %s)\n    to   (%s, %s, %s)",
		ss.MmToString(lo.X, false), ss.MmToString(lo.Y, false), ss.MmToString(lo.Z, false),
		ss.MmToString(hi.X, false), ss.MmToString(hi.Y, false), ss.MmToString(hi.Z, false))
	return b.String()
}
