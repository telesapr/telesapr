package ui

import (
	"math"
	"os"
	"time"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/libm"
	"github.com/telesapr/telesapr/pkg/platform"
	"github.com/telesapr/telesapr/pkg/render"
	"github.com/telesapr/telesapr/pkg/sketch"
)

func (gw *GraphicsWindow) sk() *sketch.Sketch { return gw.ss.SK }

// activeGroupH is the active group (C++ GW.activeGroup), which the
// sketch keeps as Sketch.Active.
func (gw *GraphicsWindow) activeGroupH() core.HGroup { return gw.sk().Active }

func (gw *GraphicsWindow) setActiveGroup(hg core.HGroup) { gw.sk().Active = hg }

// ActivateCommand runs a command as its menu item would.
func (gw *GraphicsWindow) ActivateCommand(cmd Command) {
	for _, m := range menuTable {
		if cmd == m.cmd {
			if m.fn != nil {
				m.fn(gw.ss, m.cmd)
			}
			break
		}
	}
}

// AcceleratorForCommand is the key that runs cmd.
func (gw *GraphicsWindow) AcceleratorForCommand(cmd Command) platform.KeyboardEvent {
	rawAccel := 0
	for _, m := range menuTable {
		if cmd == m.cmd {
			rawAccel = m.accel
			break
		}
	}
	accel := platform.KeyboardEvent{Type: platform.KeyPress}
	if rawAccel&shiftMask != 0 {
		accel.ShiftDown = true
	}
	if rawAccel&ctrlMask != 0 {
		accel.ControlDown = true
	}
	if rawAccel&fnMask != 0 {
		accel.Key = platform.KeyFunction
		accel.Num = rawAccel & 0xff
	} else {
		accel.Key = platform.KeyCharacter
		accel.Chr = rune(rawAccel & 0xff)
	}
	return accel
}

// KeyboardEvent handles a key: Backspace is Escape, '=' is '+', and the
// menu accelerators (C++ GraphicsWindow::KeyboardEvent).
func (gw *GraphicsWindow) KeyboardEvent(event platform.KeyboardEvent) bool {
	if event.Type == platform.KeyRelease {
		return true
	}
	if event.Key == platform.KeyCharacter {
		switch event.Chr {
		case '\b':
			// Treat backspace identically to escape.
			gw.MenuEdit(CmdUnselectAll)
			return true
		case '=':
			// Treat = as +, as on US keyboards.
			gw.MenuView(CmdZoomIn)
			return true
		}
	}
	// The platform doesn't handle the accelerators, so do it here.
	for _, m := range menuTable {
		if m.label == "" || m.cmd == CmdNone {
			continue
		}
		if gw.AcceleratorForCommand(m.cmd).Equals(event) {
			gw.ActivateCommand(m.cmd)
			return true
		}
	}
	return false
}

// PopulateMainMenu builds the menu bar from the menu table.
func (gw *GraphicsWindow) PopulateMainMenu() {
	mainMenu, unique := gw.ss.Platform.GetOrCreateMainMenu()
	if unique {
		mainMenu.Clear()
	}

	var currentSubMenu platform.Menu
	var subMenuStack []platform.Menu
	for _, m := range menuTable {
		for m.level > 0 && m.level <= len(subMenuStack) {
			currentSubMenu = subMenuStack[len(subMenuStack)-1]
			subMenuStack = subMenuStack[:len(subMenuStack)-1]
		}
		if m.label == "" {
			currentSubMenu.AddSeparator()
			continue
		}
		switch {
		case m.level == 0:
			currentSubMenu = mainMenu.AddSubMenu(m.label)
		case m.cmd == CmdOpenRecent:
			gw.openRecentMenu = currentSubMenu.AddSubMenu(m.label)
		case m.cmd == CmdGroupRecent:
			gw.linkRecentMenu = currentSubMenu.AddSubMenu(m.label)
		case m.fn == nil:
			subMenuStack = append(subMenuStack, currentSubMenu)
			currentSubMenu = currentSubMenu.AddSubMenu(m.label)
		default:
			cmd, fn := m.cmd, m.fn
			item := currentSubMenu.AddItem(m.label, func() { fn(gw.ss, cmd) }, true)
			item.SetIndicator(m.kind)
			if m.accel != 0 {
				item.SetAccelerator(gw.AcceleratorForCommand(m.cmd))
			}
			switch m.cmd {
			case CmdShowGrid:
				gw.showGridMenuItem = item
			case CmdDimSolidModel:
				gw.dimSolidModelMenuItem = item
			case CmdMeasure:
				gw.measureMenuItem = item
			case CmdSectionView:
				gw.sectionMenuItem = item
				gw.sectionMenuOn = false
			case CmdShowDrawingSheet:
				gw.sheetMenuItem = item
				item.SetActive(gw.sheetShown)
			case CmdPerspectiveProj:
				gw.perspectiveProjMenuItem = item
			case CmdExplodeSketch:
				gw.explodeMenuItem = item
			case CmdShowToolbar:
				gw.showToolbarMenuItem = item
			case CmdShowTextWnd:
				gw.showTextWndMenuItem = item
			case CmdScriptConsole:
				gw.scriptConsoleMenuItem = item
			case CmdScriptAgent:
				gw.scriptAgentMenuItem = item
			case CmdFullScreen:
				gw.fullScreenMenuItem = item
			case CmdUnitsMm:
				gw.unitsMmMenuItem = item
			case CmdUnitsMeters:
				gw.unitsMetersMenuItem = item
			case CmdUnitsInches:
				gw.unitsInchesMenuItem = item
			case CmdUnitsFeetInches:
				gw.unitsFeetInchesMenuItem = item
			case CmdSelWorkplane:
				gw.inWorkplaneMenuItem = item
			case CmdFreeIn3D:
				gw.in3dMenuItem = item
			case CmdUndo:
				gw.undoMenuItem = item
			case CmdRedo:
				gw.redoMenuItem = item
			}
		}
	}

	gw.PopulateRecentFiles()
	gw.ss.UndoEnableMenus()

	gw.window.SetMenuBar(mainMenu)
}

func (gw *GraphicsWindow) populateMenuWithPathnames(menu platform.Menu, pathnames []string, onTrigger func(string)) {
	menu.Clear()
	if len(pathnames) == 0 {
		item := menu.AddItem("(no recent files)", nil, true)
		item.SetEnabled(false)
		return
	}
	for _, pathname := range pathnames {
		pathname := pathname
		menu.AddItem(pathname, func() {
			if _, err := os.Stat(pathname); err == nil {
				onTrigger(pathname)
			} else {
				gw.ss.Error("File '%s' does not exist.", pathname)
			}
		}, false)
	}
}

// PopulateRecentFiles fills the Open Recent and Link Recent menus.
func (gw *GraphicsWindow) PopulateRecentFiles() {
	if gw.openRecentMenu == nil {
		return
	}
	gw.populateMenuWithPathnames(gw.openRecentMenu, gw.ss.RecentFiles, func(path string) {
		if !gw.ss.OkayToStartNewFile() {
			return
		}
		gw.ss.Load(path)
	})
	gw.populateMenuWithPathnames(gw.linkRecentMenu, gw.ss.RecentFiles, func(path string) {
		gw.ss.MenuGroup(CmdGroupLink, path)
	})
}

// Init resets the view, activates the last group, and the first time
// makes the window and its menus (C++ GraphicsWindow::Init).
func (gw *GraphicsWindow) Init() {
	gw.view.Scale = 5
	gw.view.Offset = geom.Vector{}
	gw.view.ProjRight = geom.Vector{X: 1}
	gw.view.ProjUp = geom.Vector{Y: 1}

	// Make sure those are valid; could get a mouse move without a mouse
	// down if someone depresses the button, then drags into our window.
	gw.orig.projRight = gw.view.ProjRight
	gw.orig.projUp = gw.view.ProjUp

	// And with the last group active.
	sk := gw.sk()
	if len(sk.GroupOrder) == 0 {
		panic("ui: group order can't be empty since we will activate the last group")
	}
	// A file may name the group that was active when it was saved.
	if sk.Active.V == 0 || sk.GetGroup(sk.Active) == nil {
		gw.setActiveGroup(sk.GroupOrder[len(sk.GroupOrder)-1])
	}
	gw.ss.groupActivate(sk.GetGroup(gw.activeGroupH()))

	gw.show = sketch.Visibility{Points: true, Normals: true, Construction: true, Edges: true}
	gw.showConstraints = SCMShowAll
	gw.showShaded = true
	gw.showMesh = false
	gw.showFacesDrawing = false
	gw.showFacesNonDrawing = true
	gw.drawOccludedAs = OccludedInvisible

	gw.showTextWindow = true

	gw.showSnapGrid = false
	gw.dimSolidModel = true
	gw.context.active = false
	gw.toolbarHovered = CmdNone

	if gw.window == nil {
		gw.window = gw.ss.Platform.CreateWindow(platform.WindowToplevel, nil)
		if gw.window != nil {
			// Do this first, so that if it causes an onRender event we
			// don't try to paint without a canvas.
			gw.window.SetMinContentSize(720, 32*18+3*16+8+4)
			ev := gw.window.Events()
			ev.OnClose = func() { gw.ss.MenuFile(CmdExit) }
			ev.OnContextLost = func() {
				gw.canvas = nil
				gw.persistentDirty = true
			}
			ev.OnRender = gw.Paint
			ev.OnKeyboardEvent = gw.KeyboardEvent
			ev.OnMouseEvent = gw.MouseEvent
			ev.OnEditingDone = gw.EditControlDone
			gw.PopulateMainMenu()
		}
	}

	if gw.window != nil {
		gw.canvas = gw.window.CreateRenderer(gw.ss.UseGL())
		gw.persistentDirty = true
	}

	// Do this last, so that all the menus get updated correctly.
	gw.ClearSuper()
}

// AnimateOntoWorkplane turns the view onto the active workplane.
func (gw *GraphicsWindow) AnimateOntoWorkplane() {
	if !gw.LockedInWorkplane() {
		return
	}
	sk := gw.sk()
	w := sk.GetEntity(gw.ActiveWorkplane())
	quatf := w.NormalEntity(sk).NormalGetNum(sk)

	// Get Z pointing vertical, if we're on turntable nav mode:
	if gw.ss.TurntableNav {
		normalRight := quatf.RotationU()
		normalUp := quatf.RotationV()
		normal := normalRight.Cross(normalUp)
		if normalRight.Z != 0 {
			theta := libm.Atan2(normalUp.Z, normalRight.Z)
			theta -= libm.Atan2(1, 0)
			normalRight = normalRight.RotatedAroundAxis(normal, theta)
			normalUp = normalUp.RotatedAroundAxis(normal, theta)
			quatf = geom.QuaternionFromBasis(normalRight, normalUp)
		}
	}

	offsetf := sk.GetEntity(w.Point[0]).PointGetNum(sk).ScaledBy(-1)

	// If the view screen is open, then we need to refresh it.
	gw.ss.ScheduleShowTW()

	gw.AnimateOnto(quatf, offsetf)
}

// AnimateOnto animates the view to orientation quatf and offset
// offsetf (C++ GraphicsWindow::AnimateOnto).
func (gw *GraphicsWindow) AnimateOnto(quatf geom.Quaternion, offsetf geom.Vector) {
	// Get our initial orientation and translation.
	quat0 := geom.QuaternionFromBasis(gw.view.ProjRight, gw.view.ProjUp)
	offset0 := gw.view.Offset

	// Make sure we take the shorter of the two possible paths.
	mp := quatf.Minus(quat0).Magnitude()
	mm := quatf.Plus(quat0).Magnitude()
	if mp > mm {
		quatf = quatf.ScaledBy(-1)
		mp = mm
	}
	mo := offset0.Minus(offsetf).Magnitude() * gw.view.Scale

	// Animate transition, unless it's a tiny move.
	t0 := time.Now()
	speed := float64(gw.ss.AnimationSpeed)
	dt := int32(0)
	if !(mp < 0.01 && mo < 10) {
		dt = int32(speed*0.75*mp + speed*0.0005*mo)
	}
	// Apply a minimum animation time, for small moves. This gets
	// overridden by the maximum setting so setting the animation speed to
	// 0 disables animations entirely.
	dt = max(dt, 100)
	// Don't ever animate for longer than animationSpeed ms.
	dt = min(dt, int32(gw.ss.AnimationSpeed))
	// If the resulting animation time is very short, disable it.
	if dt < 10 {
		dt = -20
	}

	dq := quatf.Times(quat0.Inverse())

	if gw.animateTimer == nil {
		gw.animateTimer = gw.ss.Platform.CreateTimer()
	}
	gw.animating = true
	gw.animateEnd = func() {
		gw.animating = false
		gw.view.ProjRight = quatf.RotationU()
		gw.view.ProjUp = quatf.RotationV()
		gw.view.Offset = offsetf
	}
	gw.animateTimer.SetOnTimeout(func() {
		if !gw.animating {
			return
		}
		tn := time.Since(t0).Milliseconds()
		if tn < int64(dt) {
			gw.animateTimer.RunAfterNextFrame()

			s := float64(tn) / float64(dt)
			gw.view.Offset = offset0.ScaledBy(1 - s).Plus(offsetf.ScaledBy(s))
			quat := dq.ToThe(s).Times(quat0).WithMagnitude(1)

			gw.view.ProjRight = quat.RotationU()
			gw.view.ProjUp = quat.RotationV()
		} else {
			gw.animateEnd()
		}
		gw.window.Invalidate()
	})
	gw.animateTimer.RunAfterNextFrame()
}

// FinishAnimation puts the view where a turn AnimateOnto started ends,
// at once, so that what reads the view sees where it is going.
func (gw *GraphicsWindow) FinishAnimation() {
	if gw.animating {
		gw.animateEnd()
	}
}

func (gw *GraphicsWindow) handlePointForZoomToFit(p geom.Vector, pmax, pmin *geom.Point2d,
	wmin *float64, usePerspective bool, camera *render.Camera) {
	pp, w := camera.ProjectPoint4(p)
	// If usePerspective is true, then we calculate a perspective
	// projection of the point. If not, then we do a parallel projection
	// regardless of the current scale factor.
	if usePerspective {
		pp = pp.ScaledBy(1.0 / w)
	}
	pmax.U = geom.Max(pmax.U, pp.X)
	pmax.V = geom.Max(pmax.V, pp.Y)
	pmin.U = geom.Min(pmin.U, pp.X)
	pmin.V = geom.Min(pmin.V, pp.Y)
	*wmin = geom.Min(*wmin, w)
}

func (gw *GraphicsWindow) loopOverPoints(entities []*core.Entity, constraints []*core.Constraint,
	faces []core.HEntity, pmax, pmin *geom.Point2d, wmin *float64,
	usePerspective, includeMesh bool, camera *render.Camera) {
	sk := gw.sk()
	for _, e := range entities {
		switch {
		case e.IsPoint():
			gw.handlePointForZoomToFit(e.PointGetNum(sk), pmax, pmin, wmin, usePerspective, camera)
		case e.Type == core.EntityCircle:
			// Lots of entities can extend outside the bbox of their
			// points, but circles are particularly bad.
			r := e.CircleGetRadiusNum(sk)
			c := sk.GetEntity(e.Point[0]).PointGetNum(sk)
			q := sk.GetEntity(e.Normal).NormalGetNum(sk)
			for j := 0; j < 4; j++ {
				var p geom.Vector
				switch j {
				case 0:
					p = c.Plus(q.RotationU().ScaledBy(r))
				case 1:
					p = c.Plus(q.RotationU().ScaledBy(-r))
				case 2:
					p = c.Plus(q.RotationV().ScaledBy(r))
				default:
					p = c.Plus(q.RotationV().ScaledBy(-r))
				}
				gw.handlePointForZoomToFit(p, pmax, pmin, wmin, usePerspective, camera)
			}
		default:
			// We have to iterate children points, because we can select
			// entities without points.
			for i := 0; i < core.MaxPointsInEntity; i++ {
				if e.Point[i].V == 0 {
					break
				}
				p := sk.GetEntity(e.Point[i]).PointGetNum(sk)
				gw.handlePointForZoomToFit(p, pmax, pmin, wmin, usePerspective, camera)
			}
		}
	}

	for _, c := range constraints {
		for _, p := range gw.constraintReferencePoints(c, camera) {
			gw.handlePointForZoomToFit(p, pmax, pmin, wmin, usePerspective, camera)
		}
	}

	if !includeMesh && len(faces) == 0 {
		return
	}

	g := sk.GetGroup(gw.activeGroupH())
	g.GenerateDisplayItems(sk)
	for i := range g.DisplayMesh.Triangles {
		tr := &g.DisplayMesh.Triangles[i]
		if !includeMesh {
			found := false
			for _, face := range faces {
				if face.V == uint32(tr.Meta.Face) {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}
		gw.handlePointForZoomToFit(tr.A, pmax, pmin, wmin, usePerspective, camera)
		gw.handlePointForZoomToFit(tr.B, pmax, pmin, wmin, usePerspective, camera)
		gw.handlePointForZoomToFit(tr.C, pmax, pmin, wmin, usePerspective, camera)
	}
	if !includeMesh {
		return
	}
	for _, sc := range g.PolyLoops.Contours {
		for _, sp := range sc.Points {
			gw.handlePointForZoomToFit(sp.P, pmax, pmin, wmin, usePerspective, camera)
		}
	}
}

// ZoomToFit scales and centres the view on the sketch, or on the
// selection with useSelection.
func (gw *GraphicsWindow) ZoomToFit(includingInvisibles, useSelection bool) {
	if gw.window == nil || gw.ss.Windowless {
		return
	}
	camera := gw.GetCamera()
	gw.view.Scale = gw.zoomToFit(&camera, includingInvisibles, useSelection)
}

func (gw *GraphicsWindow) zoomToFit(camera *render.Camera, includingInvisibles, useSelection bool) float64 {
	sk := gw.sk()
	var entities []*core.Entity
	var constraints []*core.Constraint
	var faces []core.HEntity

	if useSelection {
		for i := range gw.selection {
			s := &gw.selection[i]
			if s.Entity.V != 0 {
				e := sk.GetEntity(s.Entity)
				if e.IsFace() {
					faces = append(faces, e.H)
					continue
				}
				entities = append(entities, e)
			}
			if s.Constraint.V != 0 {
				constraints = append(constraints, sk.GetConstraint(s.Constraint))
			}
		}
	}

	selectionUsed := len(entities) > 0 || len(constraints) > 0 || len(faces) > 0

	if !selectionUsed {
		for i := range sk.Entities {
			e := &sk.Entities[i]
			// We don't want to handle separate points, because we will
			// iterate them inside entities.
			if e.IsPoint() {
				continue
			}
			if !includingInvisibles && !sk.EntityIsVisible(e) {
				continue
			}
			entities = append(entities, e)
		}
		for i := range sk.Constraints {
			c := &sk.Constraints[i]
			if !gw.constraintIsVisible(c) {
				continue
			}
			constraints = append(constraints, c)
		}
	}

	// On the first run, ignore perspective.
	pmax, pmin := geom.Point2d{U: -1e12, V: -1e12}, geom.Point2d{U: 1e12, V: 1e12}
	wmin := 1.0
	gw.loopOverPoints(entities, constraints, faces, &pmax, &pmin, &wmin, false, !selectionUsed, camera)

	xm, ym := (pmax.U+pmin.U)/2, (pmax.V+pmin.V)/2
	dx, dy := pmax.U-pmin.U, pmax.V-pmin.V

	gw.view.Offset = gw.view.Offset.Plus(gw.view.ProjRight.ScaledBy(-xm)).Plus(
		gw.view.ProjUp.ScaledBy(-ym))

	// And based on this, we calculate the scale and offset
	var scale float64
	if dx == 0 && dy == 0 {
		scale = 5
	} else {
		scalex, scaley := 1e12, 1e12
		if dx != 0 {
			scalex = 0.9 * camera.Width / dx
		}
		if dy != 0 {
			scaley = 0.9 * camera.Height / dy
		}
		scale = geom.Min(scalex, scaley)
		scale = geom.Min(300.0, scale)
		scale = geom.Max(0.003, scale)
	}

	// Then do another run, considering the perspective.
	pmax, pmin = geom.Point2d{U: -1e12, V: -1e12}, geom.Point2d{U: 1e12, V: 1e12}
	wmin = 1
	gw.loopOverPoints(entities, constraints, faces, &pmax, &pmin, &wmin, true, !selectionUsed, camera)

	// Adjust the scale so that no points are behind the camera
	if wmin < 0.1 {
		k := camera.Tangent
		// w = 1+k*scale*z
		zmin := (wmin - 1) / (k * scale)
		// 0.1 = 1 + k*scale*zmin
		// (0.1 - 1)/(k*zmin) = scale
		scale = geom.Min(scale, (0.1-1)/(k*zmin))
	}
	return scale
}

// ZoomToMouse zooms by 1.2 to the power zoomMultiplyer about the mouse.
func (gw *GraphicsWindow) ZoomToMouse(zoomMultiplyer float64) {
	v := &gw.view
	offsetRight := v.Offset.Dot(v.ProjRight)
	offsetUp := v.Offset.Dot(v.ProjUp)

	righti := gw.currentMousePosition.U/v.Scale - offsetRight
	upi := gw.currentMousePosition.V/v.Scale - offsetUp

	// zoomMultiplyer of 1 gives a default zoom factor of 1.2x.
	v.Scale *= math.Exp(0.1823216 * zoomMultiplyer) // ln(1.2) = 0.1823216

	rightf := gw.currentMousePosition.U/v.Scale - offsetRight
	upf := gw.currentMousePosition.V/v.Scale - offsetUp

	v.Offset = v.Offset.Plus(v.ProjRight.ScaledBy(rightf - righti))
	v.Offset = v.Offset.Plus(v.ProjUp.ScaledBy(upf - upi))

	if gw.ss.TW.shown.screen == ScreenEditView {
		if gw.havePainted {
			gw.ss.ScheduleShowTW()
		}
	}
	gw.havePainted = false
	gw.Invalidate(false)
}

// MenuView runs a View menu command.
func (gw *GraphicsWindow) MenuView(id Command) {
	ss := gw.ss
	switch id {
	case CmdZoomIn:
		gw.ZoomToMouse(1)
	case CmdZoomOut:
		gw.ZoomToMouse(-1)
	case CmdZoomToFit:
		gw.ZoomToFit(false, true)
		ss.ScheduleShowTW()
	case CmdShowGrid:
		gw.showSnapGrid = !gw.showSnapGrid
		gw.EnsureValidActives()
		gw.Invalidate(false)
		if gw.showSnapGrid && !gw.LockedInWorkplane() {
			ss.Message("No workplane is active, so the grid will not appear.")
		}
	case CmdDimSolidModel:
		gw.dimSolidModel = !gw.dimSolidModel
		gw.EnsureValidActives()
		gw.Invalidate(true)
	case CmdPerspectiveProj:
		ss.UsePerspectiveProj = !ss.UsePerspectiveProj
		gw.EnsureValidActives()
		gw.Invalidate(false)
		if ss.CameraTangentSetting < 1e-6 {
			ss.Error("The perspective factor is set to zero, so the view will " +
				"always be a parallel projection.\n\n" +
				"For a perspective projection, modify the perspective " +
				"factor in the configuration screen. A value around 0.3 " +
				"is typical.")
		}
	case CmdExplodeSketch:
		ss.Explode = !ss.Explode
		gw.EnsureValidActives()
		ss.SK.MarkGroupDirty(gw.activeGroupH(), true)
	case CmdOntoWorkplane, CmdNearestOrtho, CmdNearestIso:
		if id == CmdOntoWorkplane && gw.LockedInWorkplane() {
			gw.AnimateOntoWorkplane()
			break
		}
		// If not in 2d mode use ORTHO logic.
		ortho := [3]geom.Vector{{X: 1}, {Y: 1}, {Z: 1}}
		sqrt2, sqrt6 := math.Sqrt(2.0), math.Sqrt(6.0)
		quat0 := geom.QuaternionFromBasis(gw.view.ProjRight, gw.view.ProjUp)
		quatf := quat0
		dmin := 1e10

		// There are 24 possible views (3*2*2*2), if all are allowed. If
		// the user is in turn-table mode, the isometric view must have
		// the z-axis facing up, leaving 8 possible views (2*1*2*2).
		requireTurntable := id == CmdNearestIso && ss.TurntableNav
		for i := 0; i < 3; i++ {
			for j := 0; j < 3; j++ {
				if i == j {
					continue
				}
				if requireTurntable && j != 2 {
					continue
				}
				for negi := 0; negi < 2; negi++ {
					for negj := 0; negj < 2; negj++ {
						ou, ov := ortho[i], ortho[j]
						if negi != 0 {
							ou = ou.ScaledBy(-1)
						}
						if negj != 0 {
							ov = ov.ScaledBy(-1)
						}
						on := ou.Cross(ov)

						var u, v geom.Vector
						if id == CmdNearestOrtho || id == CmdOntoWorkplane {
							u, v = ou, ov
						} else {
							u = ou.ScaledBy(1 / sqrt2).Plus(on.ScaledBy(-1 / sqrt2))
							v = ou.ScaledBy(-1 / sqrt6).Plus(ov.ScaledBy(2 / sqrt6).Plus(on.ScaledBy(-1 / sqrt6)))
						}

						quatt := geom.QuaternionFromBasis(u, v)
						d := geom.Min(quatt.Minus(quat0).Magnitude(), quatt.Plus(quat0).Magnitude())
						if d < dmin {
							dmin = d
							quatf = quatt
						}
					}
				}
			}
		}
		gw.AnimateOnto(quatf, gw.view.Offset)
	case CmdCenterView:
		gw.GroupSelection()
		if gw.gs.n == 1 && gw.gs.points == 1 {
			// Offset is the selected point, quaternion is same as before
			sk := gw.sk()
			pt := sk.GetEntity(gw.gs.point[0]).PointGetNum(sk)
			quat0 := geom.QuaternionFromBasis(gw.view.ProjRight, gw.view.ProjUp)
			gw.ClearSelection()
			gw.AnimateOnto(quat0, pt.ScaledBy(-1))
		} else {
			ss.Error("Select a point; this point will become the center of the view on screen.")
		}
	case CmdShowToolbar:
		ss.ShowToolbar = !ss.ShowToolbar
		gw.EnsureValidActives()
		gw.Invalidate(false)
	case CmdShowTextWnd:
		gw.showTextWindow = !gw.showTextWindow
		gw.EnsureValidActives()
	case CmdUnitsInches:
		ss.SetDocumentUnits(core.UnitInches)
	case CmdUnitsFeetInches:
		ss.SetDocumentUnits(core.UnitFeetInches)
	case CmdUnitsMm:
		ss.SetDocumentUnits(core.UnitMM)
	case CmdUnitsMeters:
		ss.SetDocumentUnits(core.UnitMeters)
	case CmdFullScreen:
		gw.window.SetFullScreen(!gw.window.IsFullScreen())
		gw.EnsureValidActives()
	default:
		panic("ui: unexpected menu ID")
	}
	gw.Invalidate(false)
}

// EnsureValidActives makes sure the active group and workplane exist,
// and updates the menus' check marks.
func (gw *GraphicsWindow) EnsureValidActives() {
	ss := gw.ss
	sk := gw.sk()
	change := false
	// The active group must exist, and not be the references.
	g := sk.GetGroup(gw.activeGroupH())
	if g == nil || g.H.V == core.HGroupReferences {
		i := 0
		for ; i < len(sk.GroupOrder); i++ {
			if sk.GroupOrder[i].V != core.HGroupReferences {
				break
			}
		}
		if i >= len(sk.GroupOrder) {
			// This can happen if the user deletes all the groups in the
			// sketch. There needs to be something, so create a plane
			// drawing group and activate that.
			gw.setActiveGroup(sk.CreateDefaultDrawingGroup())
			// We've created the default group, but not the workplane
			// entity; do it now so that drawing mode isn't switched to
			// "Free in 3d".
			ss.GenerateAll(sketch.GenerateAll)
		} else {
			gw.setActiveGroup(sk.GroupOrder[i])
		}
		ss.groupActivate(sk.GetGroup(gw.activeGroupH()))
		change = true
	}

	// The active coordinate system must also exist.
	if gw.LockedInWorkplane() {
		if e := sk.GetEntity(gw.ActiveWorkplane()); e != nil {
			hgw := e.Group
			if hgw != gw.activeGroupH() && ss.GroupsInOrder(gw.activeGroupH(), hgw) {
				// The active workplane is in a group that comes after the
				// active group; so any request or constraint will fail.
				gw.SetWorkplaneFreeIn3d()
				change = true
			}
		} else {
			gw.SetWorkplaneFreeIn3d()
			change = true
		}
	}

	gw.view.Tangent = ss.CameraTangent()

	if gw.window == nil {
		return
	}

	// And update the checked state for various menus
	locked := gw.LockedInWorkplane()
	gw.in3dMenuItem.SetActive(!locked)
	gw.inWorkplaneMenuItem.SetActive(locked)

	ss.UndoEnableMenus()

	switch ss.ViewUnits {
	case core.UnitMM, core.UnitMeters, core.UnitInches, core.UnitFeetInches:
	default:
		ss.ViewUnits = core.UnitMM
	}
	gw.unitsMmMenuItem.SetActive(ss.ViewUnits == core.UnitMM)
	gw.unitsMetersMenuItem.SetActive(ss.ViewUnits == core.UnitMeters)
	gw.unitsInchesMenuItem.SetActive(ss.ViewUnits == core.UnitInches)
	gw.unitsFeetInchesMenuItem.SetActive(ss.ViewUnits == core.UnitFeetInches)

	if ss.TW.window != nil {
		ss.TW.window.SetVisible(gw.showTextWindow)
	}
	gw.showTextWndMenuItem.SetActive(gw.showTextWindow)

	gw.showGridMenuItem.SetActive(gw.showSnapGrid)
	gw.dimSolidModelMenuItem.SetActive(gw.dimSolidModel)
	gw.perspectiveProjMenuItem.SetActive(ss.UsePerspectiveProj)
	gw.explodeMenuItem.SetActive(ss.Explode)
	gw.showToolbarMenuItem.SetActive(ss.ShowToolbar)
	gw.fullScreenMenuItem.SetActive(gw.window.IsFullScreen())

	if change {
		ss.ScheduleShowTW()
	}
}

// SetWorkplaneFreeIn3d draws the active group in 3d.
func (gw *GraphicsWindow) SetWorkplaneFreeIn3d() {
	// Entity::FREE_IN_3D, which is no entity.
	gw.sk().GetGroup(gw.activeGroupH()).ActiveWorkplane = core.HEntity{}
}

// ActiveWorkplane is the active group's workplane, or FreeIn3D.
func (gw *GraphicsWindow) ActiveWorkplane() core.HEntity {
	if g := gw.sk().GetGroup(gw.activeGroupH()); g != nil {
		return g.ActiveWorkplane
	}
	return core.HEntity{}
}

// LockedInWorkplane reports whether a workplane is active.
func (gw *GraphicsWindow) LockedInWorkplane() bool {
	return !core.IsFreeIn3D(gw.ActiveWorkplane())
}

// ForceTextWindowShown shows the text window.
func (gw *GraphicsWindow) ForceTextWindowShown() {
	if !gw.showTextWindow {
		gw.showTextWindow = true
		gw.showTextWndMenuItem.SetActive(true)
		gw.ss.TW.window.SetVisible(true)
	}
}

// DeleteTaggedRequests deletes the tagged requests and updates the
// display (C++ GraphicsWindow::DeleteTaggedRequests).
func (gw *GraphicsWindow) DeleteTaggedRequests() {
	gw.sk().DeleteTaggedRequests()
}

// regenerateAfterDelete is the end of C++
// GraphicsWindow::DeleteTaggedRequests, after the requests are gone.
func (gw *GraphicsWindow) regenerateAfterDelete() {
	// An edit might be in progress for the just-deleted item. So now
	// it's not.
	gw.window.HideEditor()
	gw.ss.TW.HideEditControl()
	// And clear out the selection, which could contain that item.
	gw.ClearSuper()
	// And regenerate to get rid of what it generates, plus anything
	// that references it (since the regen code checks for that).
	gw.ss.GenerateAll(sketch.GenerateAll)
	gw.EnsureValidActives()
	gw.ss.ScheduleShowTW()
}

// SnapToGrid rounds p to the snap grid in the active workplane.
func (gw *GraphicsWindow) SnapToGrid(p geom.Vector) geom.Vector {
	if !gw.LockedInWorkplane() {
		return p
	}
	sk := gw.sk()
	wrkpl := sk.GetEntity(gw.ActiveWorkplane())
	norm := wrkpl.NormalEntity(sk)
	wo := sk.GetEntity(wrkpl.Point[0]).PointGetNum(sk)
	wu, wv, wn := norm.NormalU(sk), norm.NormalV(sk), norm.NormalN(sk)

	pp := p.Minus(wo).DotInToCsys(wu, wv, wn)
	g := gw.ss.GridSpacing
	pp.X = math.Floor((pp.X/g)+0.5) * g
	pp.Y = math.Floor((pp.Y/g)+0.5) * g
	pp.Z = 0

	return pp.ScaleOutOfCsys(wu, wv, wn).Plus(wo)
}

// MenuEdit runs an Edit menu command.
func (gw *GraphicsWindow) MenuEdit(id Command) {
	ss := gw.ss
	sk := gw.sk()
	switch id {
	case CmdUnselectAll:
		// Esc stops the solid being made off the thread first.
		if ss.stopShellJob() {
			break
		}
		gw.GroupSelection()
		// If there's nothing selected to de-select, and no operation to
		// cancel, then perhaps they want to return to the home screen in
		// the text window.
		if gw.gs.n == 0 && gw.gs.constraints == 0 && gw.pending.operation == PendingNone {
			if !(ss.TW.window.IsEditorVisible() || gw.window.IsEditorVisible()) {
				if ss.TW.shown.screen == ScreenStyleInfo {
					ss.TW.GoToScreen(ScreenListOfStyles)
				} else {
					ss.TW.ClearSuper()
				}
			}
		}
		// Some pending operations need an Undo to properly clean up on ESC.
		switch gw.pending.operation {
		case PendingDraggingNewPoint, PendingDraggingNewLinePoint,
			PendingDraggingNewArcPoint, PendingDraggingNewRadius:
			gw.ClearSuper()
			ss.UndoUndo()
		}
		gw.ClearSuper()
		ss.TW.HideEditControl()
		ss.NakedEdges.Clear()
		ss.JustExportedInfo.Draw = false
		ss.CenterOfMass.Draw = false
		// This clears the marks drawn to indicate which points are still
		// free to drag.
		for i := range sk.Params {
			sk.Params[i].Free = false
		}
		if ss.ExportMode {
			ss.ExportMode = false
			ss.GenerateAll(sketch.GenerateAll)
		}
		gw.persistentDirty = true

	case CmdSelectAll:
		for i := range sk.Entities {
			e := &sk.Entities[i]
			if e.Group != gw.activeGroupH() {
				continue
			}
			if e.IsFace() || e.IsDistance() {
				continue
			}
			if !sk.EntityIsVisible(e) {
				continue
			}
			gw.MakeSelectedEntity(e.H)
		}
		gw.Invalidate(false)
		ss.ScheduleShowTW()

	case CmdSelectChain:
		newlySelected := 0
		for {
			didSomething := false
			for i := range sk.Entities {
				e := &sk.Entities[i]
				if e.Group != gw.activeGroupH() {
					continue
				}
				if !e.HasEndpoints() {
					continue
				}
				if !sk.EntityIsVisible(e) {
					continue
				}
				st, fi := e.EndpointStart(sk), e.EndpointFinish(sk)

				onChain, alreadySelected := false, false
				for j := range gw.selection {
					s := &gw.selection[j]
					if s.Entity.V == 0 {
						continue
					}
					if s.Entity == e.H {
						alreadySelected = true
						continue
					}
					se := sk.GetEntity(s.Entity)
					if !se.HasEndpoints() {
						continue
					}
					sst, sfi := se.EndpointStart(sk), se.EndpointFinish(sk)
					if sst.Equals(st) || sst.Equals(fi) || sfi.Equals(st) || sfi.Equals(fi) {
						onChain = true
					}
				}
				if onChain && !alreadySelected {
					gw.MakeSelectedEntity(e.H)
					newlySelected++
					didSomething = true
				}
			}
			if !didSomething {
				break
			}
		}
		gw.Invalidate(false)
		ss.ScheduleShowTW()
		if newlySelected == 0 {
			ss.Error("No additional entities share endpoints with the selected entities.")
		}

	case CmdRotate90:
		gw.GroupSelection()
		var e *core.Entity
		if gw.gs.n == 1 && gw.gs.points == 1 {
			e = sk.GetEntity(gw.gs.point[0])
		} else if gw.gs.n == 1 && gw.gs.entities == 1 {
			e = sk.GetEntity(gw.gs.entity[0])
		}
		gw.ClearSelection()

		hg := gw.activeGroupH()
		if e != nil {
			hg = e.Group
		}
		g := sk.GetGroup(hg)
		if g.Type != sketch.GroupLinked {
			ss.Error("To use this command, select a point or other " +
				"entity from an linked part, or make a link " +
				"group the active group.")
			break
		}

		ss.UndoRemember()
		// Rotate by ninety degrees about the coordinate axis closest to
		// the screen normal.
		norm := gw.view.ProjRight.Cross(gw.view.ProjUp)
		norm = norm.ClosestOrtho()
		norm = norm.WithMagnitude(1)
		qaa := geom.QuaternionFromAxisAngle(norm, math.Pi/2)

		g.TransformImportedBy(sk, geom.Vector{}, qaa)

		// and regenerate as necessary.
		sk.MarkGroupDirty(hg, false)

	case CmdSnapToGrid:
		if !gw.LockedInWorkplane() {
			ss.Error("No workplane is active. Activate a workplane " +
				"(with Sketch -> In Workplane) to define the plane " +
				"for the snap grid.")
			break
		}
		gw.GroupSelection()
		if gw.gs.points == 0 && gw.gs.constraintLabels == 0 {
			ss.Error("Can't snap these items to grid; select points, " +
				"text comments, or constraints with a label. " +
				"To snap a line, select its endpoints.")
			break
		}
		ss.UndoRemember()

		for i := range gw.selection {
			s := &gw.selection[i]
			if s.Entity.V != 0 {
				hp := s.Entity
				ep := sk.GetEntity(hp)
				if !ep.IsPoint() {
					continue
				}
				p := ep.PointGetNum(sk)
				ep.PointForceTo(sk, gw.SnapToGrid(p))
				gw.pending.points = append(gw.pending.points, hp)
				sk.MarkGroupDirty(ep.Group, false)
			} else if s.Constraint.V != 0 {
				c := sk.GetConstraint(s.Constraint)
				camera := gw.GetCamera()
				refs := gw.constraintReferencePoints(c, &camera)
				c.Disp.Offset = c.Disp.Offset.Plus(gw.SnapToGrid(refs[0]).Minus(refs[0]))
			}
		}
		// Regenerate, with these points marked as dragged so that they
		// get placed as close as possible to our snap grid.
		gw.ClearSelection()

	case CmdUndo:
		ss.UndoUndo()

	case CmdRedo:
		ss.UndoRedo()

	case CmdRegenAll:
		sk.Images = nil
		if sk.ReloadAllLinked != nil {
			sk.ReloadAllLinked(sk)
		}
		ss.GenerateAll(sketch.GenerateUntilActive)
		ss.ScheduleShowTW()

	case CmdEditLineStyles:
		ss.TW.GoToScreen(ScreenListOfStyles)
		gw.ForceTextWindowShown()
		ss.ScheduleShowTW()
	case CmdViewProjection:
		ss.TW.GoToScreen(ScreenEditView)
		gw.ForceTextWindowShown()
		ss.ScheduleShowTW()
	case CmdConfiguration:
		ss.TW.GoToScreen(ScreenConfiguration)
		gw.ForceTextWindowShown()
		ss.ScheduleShowTW()

	default:
		panic("ui: unexpected menu ID")
	}
}

// MenuRequest runs a Sketch menu command.
func (gw *GraphicsWindow) MenuRequest(id Command) {
	ss := gw.ss
	sk := gw.sk()
	var s string
	switch id {
	case CmdSelWorkplane:
		gw.GroupSelection()
		g := sk.GetGroup(gw.activeGroupH())

		switch {
		case gw.gs.n == 1 && gw.gs.workplanes == 1:
			// A user-selected workplane
			g.ActiveWorkplane = gw.gs.entity[0]
			gw.EnsureValidActives()
			ss.ScheduleShowTW()
		case g.Type == sketch.GroupDrawingWorkplane:
			// The group's default workplane
			g.ActiveWorkplane = g.H.Entity(0)
			ss.MessageAndRun(func() {
				// Align the view with the selected workplane
				gw.ClearSuper()
				gw.AnimateOntoWorkplane()
			}, "No workplane selected. Activating default workplane for this group.")
		default:
			ss.Error("No workplane is selected, and the active group does " +
				"not have a default workplane. Try selecting a " +
				"workplane, or activating a sketch-in-new-workplane " +
				"group.")
			// update checkboxes in the menus
			gw.EnsureValidActives()
		}
		return

	case CmdFreeIn3D:
		gw.SetWorkplaneFreeIn3d()
		gw.EnsureValidActives()
		ss.ScheduleShowTW()
		gw.Invalidate(false)
		return

	case CmdTangentArc:
		gw.GroupSelection()
		switch {
		case gw.gs.n == 1 && gw.gs.points == 1:
			gw.MakeTangentArc()
		case gw.gs.n != 0:
			ss.Error("Bad selection for tangent arc at point. Select a " +
				"single point, or select nothing to set up arc " +
				"parameters.")
		default:
			ss.TW.GoToScreen(ScreenTangentArc)
			gw.ForceTextWindowShown()
			ss.ScheduleShowTW()
			gw.Invalidate(false) // repaint toolbar
		}
		return

	case CmdArc:
		s = "click point on arc (draws anti-clockwise)"
	case CmdDatumPoint:
		s = "click to place datum point"
	case CmdLineSegment:
		s = "click first point of line segment"
	case CmdConstrSegment:
		s = "click first point of construction line segment"
	case CmdCubic:
		s = "click first point of cubic segment"
	case CmdCircle:
		s = "click center of circle"
	case CmdWorkplane:
		s = "click origin of workplane"
	case CmdRectangle:
		s = "click one corner of rectangle"
	case CmdTTFText:
		s = "click top left of text"
	case CmdImage:
		file, ok := ss.chooseLinkedImage()
		if !ok {
			return
		}
		gw.pending.filename = file
		s = "click top left of image"

	case CmdConstruction:
		// if we are drawing
		switch gw.pending.operation {
		case PendingDraggingNewPoint, PendingDraggingNewLinePoint, PendingDraggingNewArcPoint,
			PendingDraggingNewCubicPoint, PendingDraggingNewRadius:
			for _, hr := range gw.pending.requests {
				r := sk.GetRequest(hr)
				r.Construction = !r.Construction
				sk.MarkGroupDirty(r.Group, false)
			}
			gw.Invalidate(false)
			return
		}
		gw.GroupSelection()
		if gw.gs.entities == 0 {
			ss.Error("No entities are selected. Select entities before " +
				"trying to toggle their construction state.")
			return
		}
		ss.UndoRemember()
		for _, he := range gw.gs.entity {
			if !he.IsFromRequest() {
				continue
			}
			r := sk.GetRequest(he.Request())
			r.Construction = !r.Construction
			sk.MarkGroupDirty(r.Group, false)
		}
		gw.ClearSelection()
		return

	case CmdSplitCurves:
		gw.SplitLinesOrCurves()
		return

	default:
		panic("ui: unexpected menu ID")
	}

	gw.pending.operation = PendingCommand
	gw.pending.command = id
	gw.pending.description = s
	ss.ScheduleShowTW()
	gw.Invalidate(false) // repaint toolbar
}

// ClearSuper cancels whatever is in progress and clears the selection.
func (gw *GraphicsWindow) ClearSuper() {
	if gw.window != nil {
		gw.window.HideEditor()
	}
	gw.ClearPending(true)
	gw.ClearSelection()
	gw.hover.Clear()
	gw.EnsureValidActives()
}

// ToggleBool flips a show flag and does what it needs (C++
// GraphicsWindow::ToggleBool).
func (gw *GraphicsWindow) ToggleBool(v *bool) {
	*v = !*v

	// The faces are shown as special stippling on the shaded triangle
	// mesh, so not meaningful to show them and hide the shaded.
	if !gw.showShaded {
		gw.showFaces = false
	}

	// If the mesh or edges were previously hidden, they haven't been
	// generated, and if we are going to show them, we need to generate
	// them first.
	sk := gw.sk()
	g := sk.GetGroup(gw.activeGroupH())
	if *v && len(g.DisplayOutlines.Outlines) == 0 && (v == &gw.show.Edges || v == &gw.show.Outlines) {
		gw.ss.GenerateAll(sketch.GenerateUntilActive)
	}

	if v == &gw.showFaces {
		if g.Type == sketch.GroupDrawingWorkplane || g.Type == sketch.GroupDrawing3D {
			gw.showFacesDrawing = gw.showFaces
		} else {
			gw.showFacesNonDrawing = gw.showFaces
		}
	}

	gw.Invalidate(true)
	gw.ss.ScheduleShowTW()
}

// SuggestConstraint suggests a constraint for a new line, arc or
// circle, the request, as it is drawn (sketch.SuggestConstraint), in
// the active group, when automatic constraints are on. Lines and arcs
// get one only locked in a workplane.
func (gw *GraphicsWindow) SuggestConstraint(request core.HRequest) (sketch.Suggestion, bool) {
	if !gw.ss.AutomaticLineConstraints {
		return sketch.Suggestion{}, false
	}
	wp := core.HEntity{}
	if gw.LockedInWorkplane() {
		wp = gw.ActiveWorkplane()
	}
	return gw.sk().SuggestConstraint(request, wp, gw.activeGroupH())
}

// suggestFor sets the pending suggestion for the arc or circle of
// request being drawn; none while Ctrl is held.
func (gw *GraphicsWindow) suggestFor(request core.HRequest, ctrlDown bool) {
	if ctrlDown {
		gw.pending.hasSuggestion = false
		return
	}
	gw.pending.suggestion, gw.pending.hasSuggestion = gw.SuggestConstraint(request)
}

// applySuggestion adds the suggested constraint, unless it would be
// redundant.
func (gw *GraphicsWindow) applySuggestion(s sketch.Suggestion) {
	gw.sk().ApplySuggestion(s)
}

// GetCamera is the camera for the current view and window.
func (gw *GraphicsWindow) GetCamera() render.Camera {
	var c render.Camera
	if gw.window != nil && !gw.ss.Windowless {
		c.Width, c.Height = gw.window.GetContentSize()
		c.PixelRatio = gw.window.GetDevicePixelRatio()
		c.GridFit = gw.window.GetDevicePixelRatio() == 1
	} else {
		c.Width = 297.0 // A4? Whatever...
		c.Height = 210.0
		c.PixelRatio = 1.0
		c.GridFit = c.PixelRatio == 1.0
	}
	c.Offset = gw.view.Offset
	c.ProjUp = gw.view.ProjUp
	c.ProjRight = gw.view.ProjRight
	c.Scale = gw.view.Scale
	c.Tangent = gw.ss.CameraTangent()
	return c
}

// GetLighting is the lighting from the settings.
func (gw *GraphicsWindow) GetLighting() render.Lighting {
	ss := gw.ss
	return render.Lighting{
		BackgroundColor:  ss.BackgroundColor,
		AmbientIntensity: ss.AmbientIntensity,
		LightIntensity:   ss.LightIntensity,
		LightDirection:   ss.LightDir,
		Shaded:           ss.Shaded,
	}
}

// ProjectPoint is p on the screen, in pixels from the centre.
func (gw *GraphicsWindow) ProjectPoint(p geom.Vector) geom.Point2d {
	gw.view.Tangent = gw.ss.CameraTangent()
	return gw.view.ProjectPoint(p)
}

// ProjectPoint3 is p in screen pixels, depth included.
func (gw *GraphicsWindow) ProjectPoint3(p geom.Vector) geom.Vector {
	gw.view.Tangent = gw.ss.CameraTangent()
	return gw.view.ProjectPoint3(p)
}

// UnProjectPoint is the point in the plane through the offset that
// projects to p.
func (gw *GraphicsWindow) UnProjectPoint(p geom.Point2d) geom.Vector {
	return gw.view.UnProjectPoint(p)
}

// NormalizeProjectionVectors keeps the view orthonormal.
func (gw *GraphicsWindow) NormalizeProjectionVectors() {
	gw.view.NormalizeProjectionVectors()
}

// Invalidate asks for a repaint; clearPersistent also redraws the
// things that don't change with the view.
func (gw *GraphicsWindow) Invalidate(clearPersistent bool) {
	if gw.window != nil {
		if clearPersistent {
			gw.persistentDirty = true
		}
		gw.window.Invalidate()
	}
}
