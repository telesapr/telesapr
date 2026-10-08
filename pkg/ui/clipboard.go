package ui

import (
	"fmt"
	"math"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// DeleteSelection deletes the selected requests and constraints; this
// regenerates and clears the selection, to avoid lingering references
// to the deleted items.
func (gw *GraphicsWindow) DeleteSelection() {
	// This regenerates and clears the selection, to avoid lingering
	// references to the just-deleted items.
	gw.sk().DeleteSelection(gw.selectionForEngine())
}

// CopySelection copies the selection to the clipboard.
func (gw *GraphicsWindow) CopySelection() {
	gw.sk().CopySelection(gw.selectionForEngine())
}

// PasteClipboard pastes the clipboard moved by trans, rotated by theta
// and scaled, and selects what was pasted.
func (gw *GraphicsWindow) PasteClipboard(trans geom.Vector, theta, scale float64) {
	for _, s := range gw.sk().PasteClipboard(trans, theta, scale) {
		gw.MakeSelected(&Selection{Entity: s.Entity, Constraint: s.Constraint})
	}
}

// MenuClipboard runs a cut, copy, paste or delete command.
func (gw *GraphicsWindow) MenuClipboard(id Command) {
	ss := gw.ss
	sk := gw.sk()
	if id != CmdDelete && !gw.LockedInWorkplane() {
		ss.Error("Cut, paste, and copy work only in a workplane.\n\n" +
			"Activate one with Sketch -> In Workplane.")
		return
	}

	switch id {
	case CmdPaste:
		ss.UndoRemember()
		trans := gw.view.ProjRight.ScaledBy(80 / gw.view.Scale).Plus(gw.view.ProjUp.ScaledBy(40 / gw.view.Scale))
		gw.ClearSelection()
		gw.PasteClipboard(trans, 0, 1)

	case CmdPasteTransform:
		if len(sk.Clipboard.Requests) == 0 {
			ss.Error("Clipboard is empty; nothing to paste.")
			break
		}
		wrkpl := sk.Entity(gw.ActiveWorkplane())
		p := sk.Entity(wrkpl.Point[0]).PointGetNum(sk)
		paste := &ss.TW.shown.paste
		paste.times = 1
		paste.trans = geom.Vector{}
		paste.theta = 0
		paste.origin = p
		paste.scale = 1
		ss.TW.GoToScreen(ScreenPasteTransformed)
		gw.ForceTextWindowShown()
		ss.ScheduleShowTW()

	case CmdCopy:
		gw.CopySelection()
		gw.ClearSelection()

	case CmdCut:
		ss.UndoRemember()
		gw.CopySelection()
		gw.DeleteSelection()

	case CmdDelete:
		ss.UndoRemember()
		gw.DeleteSelection()

	default:
		panic("ui: unexpected menu ID")
	}
}

// EditControlDoneForPaste takes an edited paste-transformed parameter,
// and reports whether the edit was one of those.
func (tw *TextWindow) EditControlDoneForPaste(s string) bool {
	ss := tw.ss
	switch tw.edit.meaning {
	case EditPasteTimesRepeated:
		e := ss.exprFromString(s)
		if e == nil {
			break
		}
		v := int(ss.evalExpr(e))
		if v > 0 {
			tw.shown.paste.times = v
		} else {
			ss.Error("Number of copies to paste must be at least one.")
		}

	case EditPasteAngle:
		e := ss.exprFromString(s)
		if e == nil {
			break
		}
		tw.shown.paste.theta = srf.WrapSymmetric(ss.evalExpr(e)*math.Pi/180, 2*math.Pi)

	case EditPasteScale:
		e := ss.exprFromString(s)
		if e == nil {
			// The expression may be nil here.
			break
		}
		v := ss.evalExpr(e)
		if math.Abs(v) > 1e-6 {
			if tw.shown.paste.scale < 0 {
				tw.shown.paste.scale = -v
			} else {
				tw.shown.paste.scale = v
			}
		} else {
			ss.Error("Scale cannot be zero.")
		}

	default:
		return false
	}
	return true
}

// ScreenChangePasteTransformed edits a paste-transformed parameter.
func (tw *TextWindow) ScreenChangePasteTransformed(link int, v uint32) {
	switch link {
	case 't':
		tw.ShowEditControl(13, fmt.Sprintf("%d", tw.shown.paste.times))
		tw.edit.meaning = EditPasteTimesRepeated

	case 'r':
		tw.ShowEditControl(13, fmt.Sprintf("%.3f", tw.shown.paste.theta*180/math.Pi))
		tw.edit.meaning = EditPasteAngle

	case 's':
		tw.ShowEditControl(13, fmt.Sprintf("%.3f", math.Abs(tw.shown.paste.scale)))
		tw.edit.meaning = EditPasteScale

	case 'f':
		tw.shown.paste.scale *= -1
	}
}

// ScreenPasteTransformed takes the origin or translation from the
// selection, or pastes.
func (tw *TextWindow) ScreenPasteTransformed(link int, v uint32) {
	ss := tw.ss
	sk := ss.SK
	gw := ss.GW
	gw.GroupSelection()
	paste := &tw.shown.paste
	switch link {
	case 'o':
		if gw.gs.points == 1 && gw.gs.n == 1 {
			e := sk.MustEntity(gw.gs.point[0])
			paste.origin = e.PointGetNum(sk)
		} else {
			ss.Error("Select one point to define origin of rotation.")
		}
		gw.ClearSelection()

	case 't':
		if gw.gs.points == 2 && gw.gs.n == 2 {
			pa := sk.MustEntity(gw.gs.point[0])
			pb := sk.MustEntity(gw.gs.point[1])
			paste.trans = pb.PointGetNum(sk).Minus(pa.PointGetNum(sk))
		} else {
			ss.Error("Select two points to define translation vector.")
		}
		gw.ClearSelection()

	case 'g':
		if math.Abs(paste.theta) < core.LengthEps && paste.trans.Magnitude() < core.LengthEps &&
			paste.times != 1 {
			ss.Message("Transformation is identity. So all copies will be " +
				"exactly on top of each other.")
		}
		if paste.times*len(sk.Clipboard.Requests) > 100 {
			ss.Error("Too many items to paste; split this into smaller " +
				"pastes.")
			break
		}
		if !gw.LockedInWorkplane() {
			ss.Error("No workplane active.")
			break
		}
		wrkpl := sk.MustEntity(gw.ActiveWorkplane())
		wrkpln := sk.MustEntity(wrkpl.Normal)
		wn := wrkpln.NormalN(sk)
		ss.UndoRemember()
		gw.ClearSelection()
		for i := range paste.times {
			trans := paste.trans.ScaledBy(float64(i + 1))
			origin := paste.origin
			theta := paste.theta * float64(i+1)
			// desired transformation is Q*(p - o) + o + t =
			// Q*p - Q*o + o + t = Q*p + (t + o - Q*o)
			t := trans.Plus(origin).Minus(origin.RotatedAroundAxis(wn, theta))

			gw.PasteClipboard(t, theta, paste.scale)
		}
		tw.GoToScreen(ScreenListOfGroups)
		ss.ScheduleShowTW()
	}
}

// ShowPasteTransformed is the screen to paste with a transformation.
func (tw *TextWindow) ShowPasteTransformed() {
	ss := tw.ss
	paste := &tw.shown.paste
	tw.Printf(true, "%FtPASTE TRANSFORMED%E")
	plural := "s"
	if paste.times == 1 {
		plural = ""
	}
	tw.Printf(true, "%Ba   %Ftrepeat%E    %d time%s %Fl%Lt%f[change]%E",
		paste.times, plural, lf(tw.ScreenChangePasteTransformed))
	tw.Printf(false, "%Bd   %Ftrotate%E    %@ degrees %Fl%Lr%f[change]%E",
		paste.theta*180/math.Pi, lf(tw.ScreenChangePasteTransformed))
	tw.Printf(false, "%Ba   %Ftabout pt%E  (%s, %s, %s) %Fl%Lo%f[use selected]%E",
		ss.MmToString(paste.origin.X, false),
		ss.MmToString(paste.origin.Y, false),
		ss.MmToString(paste.origin.Z, false),
		lf(tw.ScreenPasteTransformed))
	tw.Printf(false, "%Bd   %Fttranslate%E (%s, %s, %s) %Fl%Lt%f[use selected]%E",
		ss.MmToString(paste.trans.X, false),
		ss.MmToString(paste.trans.Y, false),
		ss.MmToString(paste.trans.Z, false),
		lf(tw.ScreenPasteTransformed))
	tw.Printf(false, "%Ba   %Ftscale%E     %@ %Fl%Ls%f[change]%E",
		math.Abs(paste.scale), lf(tw.ScreenChangePasteTransformed))
	flip := checkFalse
	if paste.scale < 0 {
		flip = checkTrue
	}
	tw.Printf(false, "%Ba   %Ftmirror%E    %Fd%Lf%f%s  flip%E",
		lf(tw.ScreenChangePasteTransformed), flip)

	tw.Printf(true, " %Fl%Lg%fpaste transformed now%E", lf(tw.ScreenPasteTransformed))

	tw.Printf(true, "(or %Fl%Ll%fcancel operation%E)", lf(tw.ScreenHome))
}
