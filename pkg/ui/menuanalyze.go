package ui

// The Analyze and Help menus (C++ SolveSpaceUI::MenuAnalyze,
// ShowNakedEdges, UpdateCenterOfMass and MenuHelp).

import (
	"fmt"
	"os"
	"strings"

	"github.com/telesapr/telesapr/pkg/check"
	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/platform"
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/version"
)

// MenuAnalyze runs an Analyze menu command (C++
// SolveSpaceUI::MenuAnalyze).
func (ss *SolveSpaceUI) MenuAnalyze(id Command) {
	settings := ss.Settings
	sk := ss.SK
	gw := ss.GW

	gw.GroupSelection()
	gs := &gw.gs

	switch id {
	case CmdStepDim:
		if gs.constraints == 1 && gs.n == 0 {
			c := sk.GetConstraint(gs.constraint[0])
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
				ss.TW.shown.screen = ScreenStepDimension

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

	case CmdNakedEdges:
		ss.ShowNakedEdges(false)

	case CmdInterference:
		ss.NakedEdges.Clear()

		m := &sk.GetGroup(gw.activeGroupH()).DisplayMesh
		root := srf.SKdNodeFrom(m)
		var inters, leaks bool
		root.MakeCertainEdgesInto(&ss.NakedEdges, srf.EdgeSelfInter, false, &inters, &leaks, 0)

		gw.Invalidate(false)

		if inters {
			ss.Error("%d edges interfere with other triangles, bad.", len(ss.NakedEdges.Edges))
		} else {
			ss.Message("The assembly does not interfere, good.")
		}

	case CmdCheckSolid:
		ss.CheckSolid()

	case CmdCenterOfMass:
		ss.UpdateCenterOfMass()
		ss.CenterOfMass.Draw = true
		gw.Invalidate(false)

	case CmdVolume:
		g := sk.GetGroup(gw.activeGroupH())
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

	case CmdArea:
		g := sk.GetGroup(gw.activeGroupH())
		gw.GroupSelection()

		if gs.faces > 0 {
			faces := []uint32{gs.face[0].V}
			if gs.faces > 1 {
				faces = append(faces, gs.face[1].V)
			}
			area := g.DisplayMesh.CalculateSurfaceArea(faces)
			ss.Message("The surface area of the selected faces is:\n\n"+
				"    %s\n\n"+
				"Curves have been approximated as piecewise linear.\n"+
				"This introduces error, typically of around 1%%.",
				ss.MmToStringSI(area, 2))
			break
		}

		if g.PolyError.How != int(core.PolyGood) {
			ss.Error("This group does not contain a correctly-formed " +
				"2d closed area. It is open, not coplanar, or self-" +
				"intersecting.")
			break
		}
		var sel srf.SEdgeList
		g.PolyLoops.MakeEdgesInto(&sel)
		var sp srf.SPolygon
		sel.AssemblePolygon(&sp, nil, true)
		sp.Normal = sp.ComputeNormal()
		sp.FixContourDirections()
		area := sp.SignedArea()
		ss.Message("The area of the region sketched in this group is:\n\n"+
			"    %s\n\n"+
			"Curves have been approximated as piecewise linear.\n"+
			"This introduces error, typically of around 1%%.",
			ss.MmToStringSI(area, 2))

	case CmdPerimeter:
		if gs.n > 0 && gs.n == gs.entities {
			perimeter := 0.0
			for i := range gs.entities {
				en := sk.GetEntity(gs.entity[i])
				el := gw.getOrGenerateEdges(en)
				for _, e := range el.Edges {
					perimeter += e.B.Minus(e.A).Magnitude()
				}
			}
			ss.Message("The total length of the selected entities is:\n\n"+
				"    %s\n\n"+
				"Curves have been approximated as piecewise linear.\n"+
				"This introduces error, typically of around 1%%.",
				ss.MmToStringSI(perimeter, 1))
		} else {
			ss.Error("Bad selection for perimeter; select line segments, arcs, and curves.")
		}

	case CmdShowDOF:
		// This works like a normal solve, except that it calculates
		// which variables are free/bound at the same time.
		ss.GenerateAllFindFree(sketch.GenerateAll)

	case CmdTracePt:
		if gs.points == 1 && gs.n == 1 {
			ss.Traced.Point = gs.point[0]
			gw.ClearSelection()
		} else {
			ss.Error("Bad selection for trace; select a single point.")
		}

	case CmdStopTracing:
		if ss.Traced.Point.V == 0 {
			break
		}
		dialog := ss.Platform.CreateSaveFileDialog(gw.window)
		dialog.AddFilters(platform.CsvFileFilters)
		dialog.ThawChoices(settings, "Trace")
		dialog.SetFilename(ss.SaveFile)
		if dialog.RunModal() {
			dialog.FreezeChoices(settings, "Trace")

			var b strings.Builder
			s := ss.ExportScale
			for _, pt := range ss.Traced.Path.Points {
				p := pt.P
				fmt.Fprintf(&b, "%.10f, %.10f, %.10f\r\n", p.X/s, p.Y/s, p.Z/s)
			}
			if err := os.WriteFile(dialog.GetFilename(), []byte(b.String()), 0o644); err != nil {
				ss.Error("Couldn't write to '%s'", dialog.GetFilename())
			}
		}
		// Clear the trace, and stop tracing
		ss.Traced.Point = core.HEntity{}
		ss.Traced.Path.Points = nil
		gw.Invalidate(false)

	default:
		panic("ui: unexpected menu ID")
	}
}

// ShowNakedEdges finds the naked and self-intersecting edges of the
// active group's mesh and reports them (C++
// SolveSpaceUI::ShowNakedEdges).
func (ss *SolveSpaceUI) ShowNakedEdges(reportOnlyWhenNotOkay bool) {
	ss.NakedEdges.Clear()

	g := ss.SK.GetGroup(ss.GW.activeGroupH())
	root := srf.SKdNodeFrom(&g.DisplayMesh)
	var inters, leaks bool
	root.MakeCertainEdgesInto(&ss.NakedEdges, srf.EdgeNakedOrSelfInter, true, &inters, &leaks, 0)

	if reportOnlyWhenNotOkay && !inters && !leaks && len(ss.NakedEdges.Edges) == 0 {
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

	if len(ss.NakedEdges.Edges) == 0 {
		ss.Message("%s\n\n%s\n\nZero problematic edges, good.%s",
			intersMsg, leaksMsg, cntMsg)
	} else {
		ss.Error("%s\n\n%s\n\n%d problematic edges, bad.%s",
			intersMsg, leaksMsg, len(ss.NakedEdges.Edges), cntMsg)
	}
}

// CheckSolid checks the active group's solid with check.Group, marks
// its naked and self-intersecting edges, and reports what it found in
// the view units.
func (ss *SolveSpaceUI) CheckSolid() {
	sk := ss.SK
	g := sk.GetGroup(ss.GW.activeGroupH())
	r := check.Group(sk, g)
	ss.NakedEdges.Clear()
	ss.NakedEdges.Edges = append(ss.NakedEdges.Edges, r.Problems.Edges...)
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

// UpdateCenterOfMass finds the center of mass of the active group's
// mesh (C++ SolveSpaceUI::UpdateCenterOfMass).
func (ss *SolveSpaceUI) UpdateCenterOfMass() {
	m := &ss.SK.GetGroup(ss.GW.activeGroupH()).DisplayMesh
	ss.CenterOfMass.Position = m.GetCenterOfMass()
	ss.CenterOfMass.Dirty = false
}

// MenuHelp runs a Help menu command (C++ SolveSpaceUI::MenuHelp).
func (ss *SolveSpaceUI) MenuHelp(id Command) {
	switch id {
	case CmdWebsite:
		ss.Platform.OpenInBrowser(version.Guide)

	case CmdAbout:
		ss.Message(
			"This is TeleSAPR version %s.\n"+
				"\n"+
				"TeleSAPR began as a port of SolveSpace version %s;\n"+
				"for SolveSpace, see https://solvespace.com/\n"+
				"\n"+
				"TeleSAPR is free software: you are free to modify\n"+
				"and/or redistribute it under the terms of the GNU\n"+
				"General Public License (GPL) version 3 or later.\n"+
				"\n"+
				"There is NO WARRANTY, to the extent permitted by\n"+
				"law. For details, visit http://gnu.org/licenses/\n"+
				"\n"+
				"© 2008-%d Jonathan Westhues and the other SolveSpace\n"+
				"authors, and the TeleSAPR authors.\n",
			version.Version, version.SolveSpaceVersion, 2026)

	case CmdGithub:
		ss.Platform.OpenInBrowser(version.Repository)

	default:
		panic("ui: unexpected menu ID")
	}
}
