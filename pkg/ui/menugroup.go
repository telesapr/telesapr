package ui

import (
	"path/filepath"
	"strings"
	"unicode"

	"github.com/spacesolve/spacesolve/pkg/core"
	"github.com/spacesolve/spacesolve/pkg/geom"
	"github.com/spacesolve/spacesolve/pkg/platform"
	"github.com/spacesolve/spacesolve/pkg/sketch"
)

// MenuGroup makes a new group from the selection; linkFile is the file
// to link for a link group, or "" to ask (C++ Group::MenuGroup).
func (ss *SolveSpaceUI) MenuGroup(id Command, linkFile string) {
	gw := ss.GW
	sk := ss.SK

	g := sketch.Group{
		Visible:  true,
		Color:    rgbi(100, 100, 100),
		Scale:    1,
		LinkFile: linkFile,
	}

	gw.GroupSelection()
	gs := &gw.gs

	switch id {
	case CmdGroup3D:
		g.Type = sketch.GroupDrawing3D
		g.Name = "sketch-in-3d"

	case CmdGroupWrkpl:
		g.Type = sketch.GroupDrawingWorkplane
		g.Name = "sketch-in-plane"
		switch {
		case gs.points == 1 && gs.n == 1:
			g.Subtype = sketch.GroupWorkplaneByPointOrtho

			u, v := gw.view.ProjRight, gw.view.ProjUp
			u = u.ClosestOrtho()
			v = v.Minus(u.ScaledBy(v.Dot(u)))
			v = v.ClosestOrtho()

			g.Predef.Q = geom.QuaternionFromBasis(u, v)
			g.Predef.Origin = gs.point[0]
		case gs.points == 1 && gs.lineSegments == 2 && gs.n == 3:
			g.Subtype = sketch.GroupWorkplaneByLineSegments

			g.Predef.Origin = gs.point[0]
			g.Predef.EntityB = gs.entity[0]
			g.Predef.EntityC = gs.entity[1]

			ut := sk.GetEntity(g.Predef.EntityB).VectorGetNum(sk)
			vt := sk.GetEntity(g.Predef.EntityC).VectorGetNum(sk)
			ut = ut.WithMagnitude(1)
			vt = vt.WithMagnitude(1)

			if abs(gw.view.ProjUp.Dot(vt)) < abs(gw.view.ProjUp.Dot(ut)) {
				ut, vt = vt, ut
				g.Predef.SwapUV = true
			}
			if gw.view.ProjRight.Dot(ut) < 0 {
				g.Predef.NegateU = true
			}
			if gw.view.ProjUp.Dot(vt) < 0 {
				g.Predef.NegateV = true
			}
		case gs.workplanes == 1 && gs.n == 1:
			if gs.entity[0].IsFromRequest() {
				wrkpl := sk.GetEntity(gs.entity[0])
				normal := sk.GetEntity(wrkpl.Normal)
				g.Subtype = sketch.GroupWorkplaneByPointOrtho
				g.Predef.Origin = wrkpl.Point[0]
				g.Predef.Q = normal.NormalGetNum(sk)
			} else {
				wrkplg := sk.GetGroup(gs.entity[0].Group())
				g.Subtype = wrkplg.Subtype
				g.Predef.Origin = wrkplg.Predef.Origin
				switch wrkplg.Subtype {
				case sketch.GroupWorkplaneByLineSegments:
					g.Predef.EntityB = wrkplg.Predef.EntityB
					g.Predef.EntityC = wrkplg.Predef.EntityC
					g.Predef.SwapUV = wrkplg.Predef.SwapUV
					g.Predef.NegateU = wrkplg.Predef.NegateU
					g.Predef.NegateV = wrkplg.Predef.NegateV
				case sketch.GroupWorkplaneByPointOrtho:
					g.Predef.Q = wrkplg.Predef.Q
				case sketch.GroupWorkplaneByPointNormal:
					g.Predef.Q = wrkplg.Predef.Q
					g.Predef.EntityB = wrkplg.Predef.EntityB
				default:
					panic("ui: unexpected workplane subtype")
				}
			}
		case gs.anyNormals == 1 && gs.points == 1 && gs.n == 2:
			g.Subtype = sketch.GroupWorkplaneByPointNormal
			g.Predef.EntityB = gs.anyNormal[0]
			g.Predef.Q = sk.GetEntity(gs.anyNormal[0]).NormalGetNum(sk)
			g.Predef.Origin = gs.point[0]
		default:
			ss.Error("Bad selection for new sketch in workplane. This " +
				"group can be created with:\n\n" +
				"    * a point (through the point, orthogonal to coordinate axes)\n" +
				"    * a point and two line segments (through the point, " +
				"parallel to the lines)\n" +
				"    * a point and a normal (through the point, " +
				"orthogonal to the normal)\n" +
				"    * a workplane (copy of the workplane)\n")
			return
		}

	case CmdGroupExtrude:
		if !gw.LockedInWorkplane() {
			ss.Error("Activate a workplane (Sketch -> In Workplane) before " +
				"extruding. The sketch will be extruded normal to the " +
				"workplane.")
			return
		}
		g.Type = sketch.GroupExtrude
		g.OpA = gw.activeGroupH()
		g.Predef.EntityB = gw.ActiveWorkplane()
		g.Subtype = sketch.GroupOneSided
		g.Name = "extrude"

	case CmdGroupLathe, CmdGroupRevolve, CmdGroupHelix:
		what := map[Command]string{CmdGroupLathe: "Lathe", CmdGroupRevolve: "Revolve", CmdGroupHelix: "Helix"}[id]
		if !gw.LockedInWorkplane() {
			ss.Error("%s operation can only be applied to planar sketches.", what)
			return
		}
		switch {
		case gs.points == 1 && gs.vectors == 1 && gs.n == 2:
			g.Predef.Origin = gs.point[0]
			g.Predef.EntityB = gs.vector[0]
		case gs.lineSegments == 1 && gs.n == 1:
			g.Predef.Origin = sk.GetEntity(gs.entity[0]).Point[0]
			g.Predef.EntityB = gs.entity[0]
			// since a line segment is a vector
		default:
			ss.Error("Bad selection for new %s group. This group can "+
				"be created with:\n\n"+
				"    * a point and a line segment or normal "+
				"(revolved about an axis parallel to line / "+
				"normal, through point)\n"+
				"    * a line segment (revolved about line segment)\n", strings.ToLower(what))
			return
		}
		g.OpA = gw.activeGroupH()
		switch id {
		case CmdGroupLathe:
			g.Type = sketch.GroupLathe
			g.Name = "lathe"
		case CmdGroupRevolve:
			g.Type = sketch.GroupRevolve
			g.ValA = 2
			g.Subtype = sketch.GroupOneSided
			g.Name = "revolve"
		default:
			g.Type = sketch.GroupHelix
			g.ValA = 2
			g.Subtype = sketch.GroupOneSided
			g.Name = "helix"
		}

	case CmdGroupRot:
		switch {
		case gs.points == 1 && gs.n == 1 && gw.LockedInWorkplane():
			g.Predef.Origin = gs.point[0]
			w := sk.GetEntity(gw.ActiveWorkplane())
			g.Predef.EntityB = w.NormalEntity(sk).H
			g.ActiveWorkplane = w.H
		case gs.points == 1 && gs.vectors == 1 && gs.n == 2:
			g.Predef.Origin = gs.point[0]
			g.Predef.EntityB = gs.vector[0]
		default:
			ss.Error("Bad selection for new rotation. This group can " +
				"be created with:\n\n" +
				"    * a point, while locked in workplane (rotate " +
				"in plane, about that point)\n" +
				"    * a point and a line or a normal (rotate about " +
				"an axis through the point, and parallel to " +
				"line / normal)\n")
			return
		}
		g.Type = sketch.GroupRotate
		g.OpA = gw.activeGroupH()
		g.ValA = 3
		g.Subtype = sketch.GroupOneSided
		g.Name = "rotate"

	case CmdGroupTrans:
		g.Type = sketch.GroupTranslate
		g.OpA = gw.activeGroupH()
		g.ValA = 3
		g.Subtype = sketch.GroupOneSided
		g.Predef.EntityB = gw.ActiveWorkplane()
		g.ActiveWorkplane = gw.ActiveWorkplane()
		g.Name = "translate"

	case CmdGroupLink:
		g.Type = sketch.GroupLinked
		g.MeshCombine = sketch.CombineAssemble
		if g.LinkFile == "" {
			dialog := ss.Platform.CreateOpenFileDialog(gw.window)
			dialog.AddFilters(platform.SolveSpaceLinkFileFilters)
			dialog.ThawChoices(ss.Settings, "LinkSketch")
			if !dialog.RunModal() {
				return
			}
			dialog.FreezeChoices(ss.Settings, "LinkSketch")
			g.LinkFile = dialog.GetFilename()
		}
		// Assign the default name of the group based on the name of the
		// linked file.
		stem := strings.TrimSuffix(filepath.Base(g.LinkFile), filepath.Ext(g.LinkFile))
		g.Name = strings.Map(func(r rune) rune {
			if r >= 0x80 || unicode.IsLetter(r) || unicode.IsDigit(r) {
				return r
			}
			// convert punctuation to dashes
			return '-'
		}, stem)

	default:
		panic("ui: unexpected menu ID")
	}

	// Copy color from the previous mesh-contributing group.
	if g.IsMeshGroup() && len(sk.GroupOrder) > 0 {
		if running := sk.GetRunningMeshGroupFor(gw.activeGroupH()); running != nil {
			g.Color = running.Color
		}
	}

	gw.ClearSelection()
	ss.UndoRemember()

	afterActive := false
	for _, hg := range sk.GroupOrder {
		gi := sk.GetGroup(hg)
		if afterActive {
			gi.Order++
		}
		if gi.H == gw.activeGroupH() {
			g.Order = gi.Order + 1
			afterActive = true
		}
	}

	hg := sk.AddGroup(g)
	gg := sk.GetGroup(hg)

	if gg.Type == sketch.GroupLinked {
		ss.reloadAllLinked()
	}
	gg.Clean = false
	gw.setActiveGroup(gg.H)
	ss.GenerateAll(sketch.GenerateAll)
	gg = sk.GetGroup(hg)
	if gg.Type == sketch.GroupDrawingWorkplane {
		// Can't set the active workplane for this one until after we've
		// regenerated, because the workplane doesn't exist until then.
		gg.ActiveWorkplane = gg.H.Entity(0)
	}
	ss.groupActivate(gg)
	ss.TW.ScreenSelectGroup(0, gg.H.V)
	gw.AnimateOntoWorkplane()
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

// reloadAllLinked reloads the linked files relative to the save file.
func (ss *SolveSpaceUI) reloadAllLinked() {
	if ss.SK.ReloadAllLinked != nil {
		ss.SK.ReloadAllLinked(ss.SK)
	}
}

// groupExists is SolveSpaceUI::GroupExists.
func (ss *SolveSpaceUI) groupExists(hg core.HGroup) bool {
	return ss.SK.GetGroup(hg) != nil
}
