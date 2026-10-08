package ui

// The screen with the view's parameters.

import (
	"fmt"

	"github.com/telesapr/telesapr/pkg/core"
)

// ShowEditView is the screen with the view's parameters.
func (tw *textWindow) ShowEditView() {
	ss := tw.ss
	view := &ss.GW.view
	tw.Printf(true, "%Ft3D VIEW PARAMETERS%E")

	tw.Printf(true, "%Bd %Ftoverall scale factor%E")
	tw.Printf(false, "%Ba   %# px/%s %Fl%Ll%f[edit]%E",
		view.Scale*ss.MmPerUnit(), ss.UnitName(), lf(tw.ScreenChangeViewScale))
	tw.Printf(false, "%Bd   %Fl%Ll%fset to full scale%E", lf(tw.ScreenChangeViewToFullScale))
	tw.Printf(false, "")

	tw.Printf(false, "%Bd %Ftorigin (maps to center of screen)%E")
	tw.Printf(false, "%Ba   (%s, %s, %s) %Fl%Ll%f[edit]%E",
		ss.MmToString(-view.Offset.X, false),
		ss.MmToString(-view.Offset.Y, false),
		ss.MmToString(-view.Offset.Z, false),
		lf(tw.ScreenChangeViewOrigin))
	tw.Printf(false, "")

	n := view.ProjRight.Cross(view.ProjUp)
	tw.Printf(false, "%Bd %Ftprojection onto screen%E")
	tw.Printf(false, "%Ba   %Ftright%E (%3, %3, %3) %Fl%Ll%f[edit]%E",
		view.ProjRight.X, view.ProjRight.Y, view.ProjRight.Z, lf(tw.ScreenChangeViewProjection))
	tw.Printf(false, "%Bd   %Ftup%E    (%3, %3, %3)", view.ProjUp.X, view.ProjUp.Y, view.ProjUp.Z)
	tw.Printf(false, "%Ba   %Ftout%E   (%3, %3, %3)", n.X, n.Y, n.Z)
	tw.Printf(false, "")

	tw.Printf(false, "%Ft perspective factor (0 for parallel)%E")
	tw.Printf(false, "%Ba   %# %Fl%Ll%f%D[change]%E",
		ss.CameraTangentSetting*1000, lf(tw.ScreenChangeCameraTangent), 0)

	tw.Printf(false, "")
	tw.Printf(false, "%Ft light direction               intensity")
	for i := range 2 {
		bg := byte('a')
		if i&1 != 0 {
			bg = 'd'
		}
		d := ss.LightDir[i]
		tw.Printf(false, "%Bp   #%d  (%2,%2,%2)%Fl%D%f%Ll[c]%E "+
			"%2 %Fl%D%f%Ll[c]%E",
			bg, i, d.X, d.Y, d.Z, i, lf(tw.ScreenChangeLightDirection),
			ss.LightIntensity[i], i, lf(tw.ScreenChangeLightIntensity))
	}
	tw.Printf(false, "%Ba         ambient lighting     %2 %Fl%f%Ll[c]%E",
		ss.AmbientIntensity, lf(tw.ScreenChangeLightAmbient))

	tw.Printf(false, "")
	tw.Printf(false, "%Ft explode distance%E")
	tw.Printf(false, "%Ba   %s %Fl%Ll%f%D[change]%E",
		ss.MmToString(ss.ExplodeDistance, false), lf(tw.ScreenChangeExplodeDistance), 0)
}

func (tw *textWindow) ScreenChangeViewScale(link int, v uint32) {
	tw.edit.meaning = editViewScale
	tw.ShowEditControl(3, fmt.Sprintf("%.3f", tw.ss.GW.view.Scale*tw.ss.MmPerUnit()))
}

func (tw *textWindow) ScreenChangeViewToFullScale(link int, v uint32) {
	tw.ss.GW.view.Scale = tw.ss.GW.window.PixelDensity() / 25.4
}

func (tw *textWindow) ScreenChangeViewOrigin(link int, v uint32) {
	ss := tw.ss
	o := ss.GW.view.Offset
	editValue := fmt.Sprintf("%s, %s, %s",
		ss.MmToString(-o.X, true), ss.MmToString(-o.Y, true), ss.MmToString(-o.Z, true))

	tw.edit.meaning = editViewOrigin
	tw.ShowEditControl(3, editValue)
}

func (tw *textWindow) ScreenChangeViewProjection(link int, v uint32) {
	r := tw.ss.GW.view.ProjRight
	editValue := fmt.Sprintf("%.3f, %.3f, %.3f", r.X, r.Y, r.Z)
	tw.edit.meaning = editViewProjRight
	tw.ShowEditControl(10, editValue)
}

func (tw *textWindow) ScreenChangeLightDirection(link int, v uint32) {
	d := tw.ss.LightDir[v]
	tw.ShowEditControl(8, fmt.Sprintf("%.2f, %.2f, %.2f", d.X, d.Y, d.Z))
	tw.edit.meaning = editLightDirection
	tw.edit.i = int(v)
}

func (tw *textWindow) ScreenChangeLightIntensity(link int, v uint32) {
	tw.ShowEditControl(31, fmt.Sprintf("%.2f", tw.ss.LightIntensity[v]))
	tw.edit.meaning = editLightIntensity
	tw.edit.i = int(v)
}

func (tw *textWindow) ScreenChangeLightAmbient(link int, v uint32) {
	tw.ShowEditControl(31, fmt.Sprintf("%.2f", tw.ss.AmbientIntensity))
	tw.edit.meaning = editLightAmbient
	tw.edit.i = 0
}

func (tw *textWindow) ScreenChangeCameraTangent(link int, v uint32) {
	tw.ShowEditControl(3, fmt.Sprintf("%.3f", 1000*tw.ss.CameraTangentSetting))
	tw.edit.meaning = editCameraTangent
}

func (tw *textWindow) ScreenChangeExplodeDistance(link int, v uint32) {
	tw.ShowEditControl(3, tw.ss.MmToString(tw.ss.ExplodeDistance, true))
	tw.edit.meaning = editExplodeDistance
}

// EditControlDoneForView takes an edited view parameter, and reports
// whether the edit was one of the view's.
func (tw *textWindow) EditControlDoneForView(s string) bool {
	ss := tw.ss
	view := &ss.GW.view
	switch tw.edit.meaning {
	case editViewScale:
		if e := ss.exprFromString(s); e != nil {
			v := ss.evalExpr(e) / ss.MmPerUnit()
			if v > core.LengthEps {
				view.Scale = v
			} else {
				ss.Error("Scale cannot be zero or negative.")
			}
		}

	case editViewOrigin:
		if x, y, z, ok := scanVector3(s); ok {
			pt := vec3(x, y, z).ScaledBy(ss.MmPerUnit())
			view.Offset = pt.ScaledBy(-1)
		} else {
			ss.Error("Bad format: specify x, y, z")
		}

	case editViewProjRight, editViewProjUp:
		x, y, z, ok := scanVector3(s)
		if !ok {
			ss.Error("Bad format: specify x, y, z")
			break
		}
		pt := vec3(x, y, z)
		if tw.edit.meaning == editViewProjRight {
			view.ProjRight = pt
			ss.GW.NormalizeProjectionVectors()
			tw.edit.meaning = editViewProjUp
			tw.HideEditControl()
			u := view.ProjUp
			tw.ShowEditControl(10, fmt.Sprintf("%.3f, %.3f, %.3f", u.X, u.Y, u.Z), tw.editControl.halfRow+2)
			tw.edit.showAgain = true
		} else {
			view.ProjUp = pt
			ss.GW.NormalizeProjectionVectors()
		}

	default:
		return false
	}
	return true
}
