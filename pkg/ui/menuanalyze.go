package ui

// The Analyze and Help menus.

import (
	"fmt"
	"math"
	"os"
	"strings"

	"github.com/telesapr/telesapr/pkg/check"
	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/platform"
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/version"
)

// MenuAnalyze runs an Analyze menu command.
func (ss *Application) MenuAnalyze(id Command) {
	sk := ss.SK
	gw := ss.GW

	gw.GroupSelection()
	gs := &gw.gs

	switch id {
	case cmdStepDim:
		if gs.constraints == 1 && gs.n == 0 {
			c := sk.Constraint(gs.constraint[0])
			if c.HasLabel() && !c.Reference {
				ss.TW.stepDim.finish = c.ValA
				ss.TW.stepDim.steps = 10
				switch c.Type {
				case core.ConstraintAngle, core.ConstraintLengthRatio,
					core.ConstraintArcArcLenRatio, core.ConstraintArcLineLenRatio,
					core.ConstraintLengthDifference, core.ConstraintArcArcDifference,
					core.ConstraintArcLineDifference:
					ss.TW.stepDim.isDistance = false
				default:
					ss.TW.stepDim.isDistance = true
				}
				ss.TW.shown.constraint = c.H
				ss.TW.shown.screen = screenStepDimension

				// The step params are specified in the text window,
				// so force that to be shown.
				gw.ForceTextWindowShown()

				ss.ScheduleShowTW()
				gw.ClearSelection()
			} else {
				ss.Error("Constraint must have a label, and must not be " +
					"a reference dimension.")
			}
		} else {
			ss.Error("Bad selection for step dimension; select a constraint.")
		}

	case cmdNakedEdges:
		ss.ShowNakedEdges(false)

	case cmdInterference:
		ss.analyzeInterference()

	case cmdGroupDependencies:
		ss.analyzeGroupDependencies()

	case cmdCheckSolid:
		ss.CheckSolid()

	case cmdRayCheck:
		ss.RayCheck()

	case cmdCenterOfMass:
		ss.analyzeCenterOfMass()

	case cmdVolume:
		ss.analyzeVolume()

	case cmdArea:
		ss.analyzeArea()

	case cmdPerimeter:
		perimeter, err := sk.Length(gs.entity)
		if gs.n > 0 && gs.n == gs.entities && err == nil {
			ss.Message("The total length of the selected entities is:\n\n"+
				"    %s\n\n"+
				"Curves have been approximated as piecewise linear.\n"+
				"This introduces error, typically of around 1%%.",
				ss.MmToStringSI(perimeter, 1))
		} else {
			ss.Error("Bad selection for perimeter; select line segments, arcs, and curves.")
		}

	case cmdShowDOF:
		// This works like a normal solve, except that it calculates
		// which variables are free/bound at the same time.
		ss.GenerateAllFindFree(sketch.GenerateAll)

	case cmdTracePt:
		if gs.points == 1 && gs.n == 1 {
			ss.GW.traced.Point = gs.point[0]
			gw.ClearSelection()
		} else {
			ss.Error("Bad selection for trace; select a single point.")
		}

	case cmdBillOfMaterials:
		ss.analyzeBillOfMaterials()

	case cmdStopTracing:
		ss.analyzeStopTracing()

	default:
		panic("ui: unexpected menu ID")
	}
}

// analyzeInterference runs the Analyze menu's cmdInterference.
func (ss *Application) analyzeInterference() {
	sk := ss.SK
	gw := ss.GW

	ss.GW.nakedEdges.Clear()

	// The parts by their shells (roadmap 16.10), unless one is a mesh
	// with none: then the triangles.
	found, meshOnly := sk.Interferences(gw.activeGroupH())
	if len(meshOnly) == 0 {
		var lines []string
		for _, f := range found {
			if f.Failed {
				lines = append(lines, fmt.Sprintf("    %s and %s: the Boolean failed",
					sk.GroupName(f.A), sk.GroupName(f.B)))
				continue
			}
			about := ""
			if f.Approximate {
				about = " (about: by their meshes)"
			}
			lines = append(lines, fmt.Sprintf("    %s and %s share %s%s",
				sk.GroupName(f.A), sk.GroupName(f.B), ss.MmToStringSI(f.Volume, 3), about))
			for _, tr := range f.Mesh.Triangles {
				ss.GW.nakedEdges.AddEdge(tr.A, tr.B, 0, 0, 0)
				ss.GW.nakedEdges.AddEdge(tr.B, tr.C, 0, 0, 0)
				ss.GW.nakedEdges.AddEdge(tr.C, tr.A, 0, 0, 0)
			}
		}
		gw.Invalidate(false)
		switch len(lines) {
		case 0:
		case 1:
			ss.Error("Two parts interfere, bad:\n\n%s", lines[0])
		default:
			ss.Error("%d pairs of parts interfere, bad:\n\n%s", len(lines), strings.Join(lines, "\n"))
		}
		if len(lines) == 0 {
			ss.Message("The assembly does not interfere, good.")
		}
		return
	}

	m := &sk.Group(gw.activeGroupH()).DisplayMesh
	root := srf.SKdNodeFrom(m)
	var inters, leaks bool
	root.MakeCertainEdgesInto(&ss.GW.nakedEdges, srf.EdgeSelfInter, false, &inters, &leaks, 0)

	gw.Invalidate(false)

	if inters {
		ss.Error("%d edges interfere with other triangles, bad.", len(ss.GW.nakedEdges.Edges))
	} else {
		ss.Message("The assembly does not interfere, good.")
	}
}

// analyzeCenterOfMass runs the Analyze menu's cmdCenterOfMass.
func (ss *Application) analyzeCenterOfMass() {
	gw := ss.GW

	ss.UpdateCenterOfMass()
	ss.GW.centerOfMass.Draw = true
	gw.Invalidate(false)
}

// analyzeVolume runs the Analyze menu's cmdVolume.
func (ss *Application) analyzeVolume() {
	sk := ss.SK
	gw := ss.GW

	g := sk.Group(gw.activeGroupH())
	totalVol := g.DisplayMesh.CalculateVolume()
	msg := "The volume of the solid model is:\n\n    " + ss.MmToStringSI(totalVol, 3)

	var curMesh srf.SMesh
	g.ThisShell.TriangulateInto(&curMesh)
	if curVol := curMesh.CalculateVolume(); curVol > 0.0 {
		msg += "\nThe volume of current group mesh is:\n\n    " + ss.MmToStringSI(curVol, 3)
	}

	msg += "\n\nCurved surfaces have been approximated as triangles.\n" +
		"This introduces error, typically of around 1%."
	ss.Message("%s", msg)
}

// analyzeArea runs the Analyze menu's cmdArea.
func (ss *Application) analyzeArea() {
	sk := ss.SK
	gw := ss.GW
	gs := &gw.gs

	g := sk.Group(gw.activeGroupH())
	gw.GroupSelection()

	if gs.faces > 0 {
		area, err := sk.FacesArea(g.H, gs.face)
		if err != nil {
			ss.Error("Can't measure the faces' area: %s.", err)
			return
		}
		ss.Message("The surface area of the selected faces is:\n\n"+
			"    %s\n\n"+
			"Curves have been approximated as piecewise linear.\n"+
			"This introduces error, typically of around 1%%.",
			ss.MmToStringSI(area, 2))
		return
	}

	area, err := sk.RegionArea(g.H)
	if err != nil {
		ss.Error("This group does not contain a correctly-formed " +
			"2d closed area. It is open, not coplanar, or self-" +
			"intersecting.")
		return
	}
	ss.Message("The area of the region sketched in this group is:\n\n"+
		"    %s\n\n"+
		"Curves have been approximated as piecewise linear.\n"+
		"This introduces error, typically of around 1%%.",
		ss.MmToStringSI(area, 2))
}

// analyzeBillOfMaterials runs the Analyze menu's cmdBillOfMaterials.
func (ss *Application) analyzeBillOfMaterials() {
	gw := ss.GW

	gw.ClearSelection()
	ss.TW.GoToScreen(screenBillOfMaterials)
	gw.ForceTextWindowShown()
	ss.ScheduleShowTW()
}

// analyzeStopTracing runs the Analyze menu's cmdStopTracing.
func (ss *Application) analyzeStopTracing() {
	settings := ss.Settings
	gw := ss.GW

	if ss.GW.traced.Point.V == 0 {
		return
	}
	dialog := ss.Platform.CreateSaveFileDialog(gw.window)
	dialog.AddFilters(platform.CsvFileFilters)
	dialog.ThawChoices(settings, "Trace")
	dialog.SetFilename(ss.SaveFile)
	if dialog.RunModal() {
		dialog.FreezeChoices(settings, "Trace")

		var b strings.Builder
		s := ss.ExportScale
		for _, pt := range ss.GW.traced.Path.Points {
			p := pt.P
			fmt.Fprintf(&b, "%.10f, %.10f, %.10f\r\n", p.X/s, p.Y/s, p.Z/s)
		}
		if err := os.WriteFile(dialog.Filename(), []byte(b.String()), 0o644); err != nil {
			ss.Error("Couldn't write to '%s'", dialog.Filename())
		}
	}
	// Clear the trace, and stop tracing
	ss.GW.traced.Point = core.HEntity{}
	ss.GW.traced.Path.Points = nil
	gw.Invalidate(false)
}

// ShowNakedEdges finds the naked and self-intersecting edges of the
// active group's mesh and reports them.
func (ss *Application) ShowNakedEdges(reportOnlyWhenNotOkay bool) {
	ss.GW.nakedEdges.Clear()

	g := ss.SK.Group(ss.GW.activeGroupH())
	root := srf.SKdNodeFrom(&g.DisplayMesh)
	var inters, leaks bool
	root.MakeCertainEdgesInto(&ss.GW.nakedEdges, srf.EdgeNakedOrSelfInter, true, &inters, &leaks, 0)

	if reportOnlyWhenNotOkay && !inters && !leaks && len(ss.GW.nakedEdges.Edges) == 0 {
		return
	}
	ss.GW.Invalidate(false)

	intersMsg := "The mesh is not self-intersecting (okay, valid)."
	if inters {
		intersMsg = "The mesh is self-intersecting (NOT okay, invalid)."
	}
	leaksMsg := "The mesh is watertight (okay, valid)."
	if leaks {
		leaksMsg = "The mesh has naked edges (NOT okay, invalid)."
	}

	cntMsg := fmt.Sprintf("\n\nThe model contains %d triangles, from %d surfaces.",
		len(g.DisplayMesh.Triangles), len(g.RunningShell.Surface))

	if len(ss.GW.nakedEdges.Edges) == 0 {
		ss.Message("%s\n\n%s\n\nZero problematic edges, good.%s",
			intersMsg, leaksMsg, cntMsg)
	} else {
		ss.Error("%s\n\n%s\n\n%d problematic edges, bad.%s",
			intersMsg, leaksMsg, len(ss.GW.nakedEdges.Edges), cntMsg)
	}
}

// CheckSolid checks the active group's solid with check.Group, marks
// its naked and self-intersecting edges, and reports what it found in
// the view units.
func (ss *Application) CheckSolid() {
	sk := ss.SK
	g := sk.Group(ss.GW.activeGroupH())
	r := check.Group(sk, g)
	ss.GW.nakedEdges.Clear()
	ss.GW.nakedEdges.Edges = append(ss.GW.nakedEdges.Edges, r.Problems.Edges...)
	ss.GW.nakedEdges.Edges = append(ss.GW.nakedEdges.Edges, r.TopologyEdges.Edges...)
	ss.GW.Invalidate(false)

	if r.Empty() {
		ss.Message("The active group has no solid to check.")
		return
	}
	var b strings.Builder
	if r.Sound() {
		b.WriteString("The solid is sound.\n\nIt is closed, it does not intersect itself, " +
			"and its Boolean succeeded.\n\n")
	} else {
		b.WriteString("The solid is NOT sound.\n\n")
		if r.NakedEdges > 0 {
			fmt.Fprintf(&b, "    %d naked edges, so it is not closed\n", r.NakedEdges)
		}
		if r.SelfIntersecting > 0 {
			fmt.Fprintf(&b, "    %d self-intersecting edges\n", r.SelfIntersecting)
		}
		if r.BooleanFailed {
			b.WriteString("    the Boolean failed; see the group's page\n")
		}
		if r.TopologyProblems > 0 {
			fmt.Fprintf(&b, "    %d problems with its topology, the first: %s\n",
				r.TopologyProblems, r.Topology[0].Msg)
		}
		b.WriteString("\nThe problem edges are marked.\n\n")
	}
	c, d := r.CenterOfMass, r.Max.Minus(r.Min)
	if r.Patches != r.Faces {
		fmt.Fprintf(&b, "Faces: %d, of %d patches\n", r.Faces, r.Patches)
	} else {
		fmt.Fprintf(&b, "Faces: %d\n", r.Faces)
	}
	fmt.Fprintf(&b, "Triangles: %d\nVolume: %s\n", r.Triangles, ss.MmToStringSI(r.Volume, 3))
	fmt.Fprintf(&b, "Center of mass: (%s, %s, %s)\n", ss.MmToString(c.X, false),
		ss.MmToString(c.Y, false), ss.MmToString(c.Z, false))
	fmt.Fprintf(&b, "Extent: %s x %s x %s", ss.MmToString(d.X, false),
		ss.MmToString(d.Y, false), ss.MmToString(d.Z, false))
	if r.Sound() {
		ss.Message("%s", b.String())
	} else {
		ss.Error("%s", b.String())
	}
}

// RayCheck checks the active group's solid by the ray-cast oracle
// (check.RayCheck): its volume against the mesh's, its centre of mass
// and principal moments of inertia, the rays along which they
// disagree, which are marked, and the mesh's half-edges.
func (ss *Application) RayCheck() {
	sk := ss.SK
	g := sk.Group(ss.GW.activeGroupH())
	r, err := check.RayCheck(sk, g, 24)
	if err != nil {
		ss.Error("The ray-cast check can't build this solid.\n\n%v.", err)
		return
	}
	if r.Measure.Volume == 0 && r.MeshVolume == 0 {
		ss.Message("The active group has no solid to check.")
		return
	}
	ss.GW.nakedEdges.Clear()
	for _, d := range r.Disagree {
		for _, iv := range append(append([]check.Interval(nil), d.Oracle...), d.Mesh...) {
			ss.GW.nakedEdges.AddEdge(d.From.Plus(d.Dir.ScaledBy(iv.From)), d.From.Plus(d.Dir.ScaledBy(iv.To)), 0, 0, 0)
		}
	}
	ss.GW.Invalidate(false)

	var b strings.Builder
	m := r.Measure
	diff := 0.0
	if m.Volume != 0 {
		diff = 100 * (r.MeshVolume - m.Volume) / m.Volume
	}
	fmt.Fprintf(&b, "Ray-cast volume: %s (three grids of %d rays a side agree within %.3f%%)\n",
		ss.MmToStringSI(m.Volume, 3), m.Rays, 100*m.Spread())
	fmt.Fprintf(&b, "The mesh's volume: %s, %+.3f%%\n", ss.MmToStringSI(r.MeshVolume, 3), diff)
	c := m.CenterOfMass
	fmt.Fprintf(&b, "Ray-cast center of mass: (%s, %s, %s)\n", ss.MmToString(c.X, false),
		ss.MmToString(c.Y, false), ss.MmToString(c.Z, false))
	// Moments are mm⁵ for a density of one; in the view's unit, its fifth
	// power, as the SI prefixes go no further than volumes.
	p := m.PrincipalMoments()
	u5 := math.Pow(ss.MmPerUnit(), 5)
	fmt.Fprintf(&b, "Ray-cast principal moments of inertia, density 1: %.5g, %.5g, %.5g %s⁵\n\n",
		p[0]/u5, p[1]/u5, p[2]/u5, ss.UnitName())
	if len(r.Disagree) == 0 {
		fmt.Fprintf(&b, "All %d rays agree with the mesh.\n", r.Rays)
	} else {
		fmt.Fprintf(&b, "%d of %d rays disagree with the mesh by more than twice the chord tolerance; "+
			"they are marked.\n", len(r.Disagree), r.Rays)
	}
	mf := r.Manifold
	if mf.Closed() {
		b.WriteString("Every edge of the mesh is used twice, once each way.")
	} else {
		fmt.Fprintf(&b, "The mesh's edges: %d used once, %d used more than twice, %d used twice the same way; "+
			"%d degenerate triangles.", mf.Unmatched, mf.Excess, mf.Misoriented, mf.Degenerate)
	}
	if len(r.Disagree) == 0 && mf.Closed() {
		ss.Message("%s", b.String())
	} else {
		ss.Error("%s", b.String())
	}
}

// UpdateCenterOfMass finds the center of mass of the active group's
// mesh.
func (ss *Application) UpdateCenterOfMass() {
	m := &ss.SK.Group(ss.GW.activeGroupH()).DisplayMesh
	ss.GW.centerOfMass.Position = m.CenterOfMass()
	ss.GW.centerOfMass.Dirty = false
}

// MenuHelp runs a Help menu command.
func (ss *Application) MenuHelp(id Command) {
	switch id {
	case cmdWebsite:
		ss.Platform.OpenInBrowser(version.Guide)

	case cmdAbout:
		ss.Message(
			"This is TeleSAPR version %s.\n"+
				"\n"+
				"TeleSAPR is free software: you are free to modify\n"+
				"and/or redistribute it under the terms of the GNU\n"+
				"General Public License (GPL) version 3 or later.\n"+
				"\n"+
				"There is NO WARRANTY, to the extent permitted by\n"+
				"law. For details, visit http://gnu.org/licenses/\n"+
				"\n"+
				"TeleSAPR began as a port of SolveSpace with the\n"+
				"last version referred %s.\n"+
				"For SolveSpace, see https://solvespace.com/\n"+
				"\n"+
				"© 2008-%d Jonathan Westhues and the other SolveSpace\n"+
				"authors.\n"+
				"© 2025-2026 Mikhail Sorochan and the TeleSAPR authors.\n",
			version.Version, version.SolveSpaceVersion, 2026)

	case cmdGithub:
		ss.Platform.OpenInBrowser(version.Repository)

	default:
		panic("ui: unexpected menu ID")
	}
}
