package ui

import (
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"unicode"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/platform"
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/srf"
)

// MenuGroup makes a new group from the selection; linkFile is the file
// to link for a link group, or "" to ask (C++ Group::MenuGroup).
func (ss *SolveSpaceUI) MenuGroup(id Command, linkFile string) {
	gw := ss.GW
	sk := ss.SK

	g := sketch.NewGroup()
	g.LinkFile = linkFile

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

	case CmdGroupMirror:
		var plane core.HEntity
		if gs.n == 1 && (gs.workplanes == 1 || gs.faces == 1) {
			plane = gs.entity[0]
		}
		mg, err := sk.NewMirror(plane, "")
		if err != nil {
			ss.Error("Bad selection for a new mirror group. %s.\n\n"+
				"Mirror reflects the active group in a plane. Select one of:\n\n"+
				"    * a workplane\n"+
				"    * a planar face of the solid\n", capitalize(err.Error()))
			return
		}
		g = mg

	case CmdGroupChamfer:
		cg, err := sk.NewChamfer(gs.entity, gs.solidEdge, 1, "")
		if err != nil {
			ss.Error("Can't chamfer the selection. %s.\n\n"+
				"Chamfer Edges cuts edges of the solid so far. Select one or more of:\n\n"+
				"    * line segments along edges between two flat faces, "+
				"as the edges an extrusion makes\n"+
				"    * circles where a flat face meets a cylinder or a cone, as an extruded circle's\n"+
				"    * straight or circular edges of the solid, picked on the model\n", capitalize(err.Error()))
			return
		}
		g = cg

	case CmdGroupFillet:
		fg, err := sk.NewFillet(gs.entity, gs.solidEdge, 1, "")
		if err != nil {
			ss.Error("Can't fillet the selection. %s.\n\n"+
				"Fillet Edges rounds edges of the solid so far. Select one or more of:\n\n"+
				"    * line segments along edges between two flat faces, "+
				"as the edges an extrusion makes\n"+
				"    * circles where a flat face meets a cylinder or a cone, as an extruded circle's\n"+
				"    * straight or circular edges of the solid, picked on the model\n\n"+
				"Each ends at corners of three flat faces. An edge that sticks out is "+
				"rounded off; one that goes in, as a pocket's or a boss's foot, is filled. "+
				"A rounded edge takes with it the edges it meets smoothly, as round a rounded "+
				"rectangle. "+
				"Where two rounded edges meet, they are mitred; where three meet, a sphere "+
				"rounds the corner. The group's screen gives an edge a radius of its own, "+
				"where it meets no other rounded edge.", capitalize(err.Error()))
			return
		}
		g = fg

	case CmdGroupShell:
		faces := make([]srf.FaceID, len(gs.face))
		for i, he := range gs.face {
			faces[i] = srf.FaceID(he.V)
		}
		// Faces no entity names, as a fillet's, picked on the model.
		faces = append(faces, gs.solidFace...)
		shg, err := sk.NewShell(faces, 1, "")
		if err != nil {
			ss.Error("Can't shell the solid. %s.\n\n"+
				"Shell Solid hollows the solid so far to walls of one thickness, open where "+
				"faces are taken away. Select one or more of:\n\n"+
				"    * flat faces of the solid, to take away\n\n"+
				"The solid's faces must be flat, or the solid a prism of lines and arcs open only "+
				"at its ends, and the walls must not meet.", capitalize(err.Error()))
			return
		}
		g = shg

	case CmdGroupLoft:
		// The active sketch and the others selected, in the order of the
		// history from the active one, which is the first or the last.
		others := map[core.HGroup]bool{}
		for _, he := range append(append([]core.HEntity(nil), gs.entity...), gs.point...) {
			if e := sk.GetEntity(he); e != nil && e.Group != sk.Active {
				others[e.Group] = true
			}
		}
		var profiles []core.HGroup
		for _, hg := range sk.GroupOrder {
			if others[hg] || hg == sk.Active {
				profiles = append(profiles, hg)
			}
		}
		if profiles[len(profiles)-1] == sk.Active {
			slices.Reverse(profiles)
		}
		err := errors.New("nothing of another sketch is selected")
		var lg sketch.Group
		switch {
		case len(others) > 0 && profiles[0] != sk.Active:
			err = errors.New("the active sketch lies between the others in the history; a loft starts or ends at it")
		case len(others) > 0:
			lg, err = sk.NewSmoothLoft(profiles, "")
		}
		if err != nil {
			ss.Error("Can't loft. %s.\n\n"+
				"Loft To Selected Profile joins the active group's sketch, a closed loop "+
				"in a workplane, to others. Select:\n\n"+
				"    * an entity of another sketch, to join the two by ruled faces\n"+
				"    * entities of two or more other sketches, to join them all by one smooth "+
				"surface, in the order of the history from the active one\n", capitalize(err.Error()))
			return
		}
		g = lg

	case CmdGroupPathPattern:
		pg, err := sk.NewPathPattern(gs.entity, 3, "")
		if err != nil {
			ss.Error("Can't repeat along the selection. %s.\n\n"+
				"Pattern Along Path repeats the active group's solid along a path, three times "+
				"to begin with. Select:\n\n"+
				"    * the line segments and arcs of the path, joined end to end "+
				"and meeting tangent; a path that closes, the copies shared out round it\n"+
				"    * a circle, the copies shared out round it\n", capitalize(err.Error()))
			return
		}
		g = pg

	case CmdGroupSweep:
		if !gw.LockedInWorkplane() {
			ss.Error("Activate the workplane of the profile to sweep (Sketch -> In Workplane): " +
				"the active group's sketch is moved along the path.")
			return
		}
		wg, err := sk.NewSweep(gs.entity, "")
		if err != nil {
			ss.Error("Can't sweep along the selection. %s.\n\n"+
				"Sweep Along Path moves the active group's sketch along a path. Select:\n\n"+
				"    * the line segments, arcs and cubics of the path, joined end to end "+
				"and meeting tangent\n", capitalize(err.Error()))
			return
		}
		g = wg

	case CmdGroupLink:
		g.Type = sketch.GroupLinked
		g.MeshCombine = sketch.CombineAssemble
		if g.LinkFile == "" {
			dialog := ss.Platform.CreateOpenFileDialog(gw.window)
			dialog.AddFilters(platform.LinkFileFilters)
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

	gw.ClearSelection()
	ss.addNewGroup(g, true)
}

// addNewGroup adds g after the active group, activates it and
// regenerates, as the end of C++ Group::MenuGroup does.
func (ss *SolveSpaceUI) addNewGroup(g sketch.Group, rememberForUndo bool) core.HGroup {
	gw := ss.GW
	sk := ss.SK

	// The undo state is remembered before the groups are reordered, but
	// the colour is taken first, as in C++.
	if g.IsMeshGroup() && len(sk.GroupOrder) > 0 {
		if running := sk.GetRunningMeshGroupFor(gw.activeGroupH()); running != nil {
			g.Color = running.Color
		}
	}
	if rememberForUndo {
		ss.UndoRemember()
	}

	sk.BumpOrderAfterActive(&g)
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
	return hg
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
