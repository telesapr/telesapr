package ui

import (
	"strconv"
	"strings"
)

// This file is an extension: C++ SolveSpace has no named parameters.
// The screen lists the sketch's parameter table, with what could not
// be evaluated, and edits it.

// cppComparison is set by the tests that compare the interface with
// C++ recordings, and hides the links to what C++ does not have from
// the screens C++ has, until ROADMAP-JUMP.md package 5.1 freezes those
// recordings.
var cppComparison bool

// ScreenShowParameters shows the parameter table.
func (tw *TextWindow) ScreenShowParameters(link int, v uint32) {
	tw.GoToScreen(ScreenParameters)
}

// MenuParameters shows the parameter table, from the menu.
func (ss *SolveSpaceUI) MenuParameters() {
	ss.GW.ClearSelection()
	ss.TW.GoToScreen(ScreenParameters)
	ss.GW.ForceTextWindowShown()
	ss.ScheduleShowTW()
}

// formatParameter shows a parameter's value with up to ten significant
// digits.
func formatParameter(v float64) string { return strconv.FormatFloat(v, 'g', 10, 64) }

// pad fills s with spaces to n characters.
func pad(s string, n int) string {
	if len(s) >= n {
		return s
	}
	return s + strings.Repeat(" ", n-len(s))
}

// ScreenChangeParameter starts an edit of parameter v: its expression
// ('e'), its name ('n'), or deletes it ('x'). 'a' adds one.
func (tw *TextWindow) ScreenChangeParameter(link int, v uint32) {
	ss := tw.ss
	sk := ss.SK
	switch link {
	case 'a':
		tw.ShowEditControl(3, "")
		tw.edit.meaning = EditParameterAdd
		return
	}
	i := int(v)
	if i < 0 || i >= len(sk.Parameters) {
		return
	}
	p := sk.Parameters[i]
	tw.edit.i = i
	switch link {
	case 'e':
		tw.ShowEditControl(3, p.Expr)
		tw.edit.meaning = EditParameterExpr
	case 'n':
		tw.ShowEditControl(3, p.Name)
		tw.edit.meaning = EditParameterName
	case 'x':
		ss.parameterEdit(func() error { return sk.DeleteParameter(p.Name) })
	}
}

// parameterEdit runs an edit of the table as one undo step, and shows
// why when it is refused.
func (ss *SolveSpaceUI) parameterEdit(fn func() error) bool {
	if err := ss.SK.UndoRememberFor(fn); err != nil {
		ss.Error("%s.", capitalize(err.Error()))
		return false
	}
	ss.SK.Unsaved = true
	ss.UndoEnableMenus()
	ss.SK.ScheduleGenerateAll()
	ss.ScheduleShowTW()
	return true
}

// EditControlDoneForParameters takes an edit of the parameter table,
// and reports whether the edit was one of those.
func (tw *TextWindow) EditControlDoneForParameters(s string) bool {
	ss := tw.ss
	sk := ss.SK
	name := func() string {
		if tw.edit.i < len(sk.Parameters) {
			return sk.Parameters[tw.edit.i].Name
		}
		return ""
	}
	switch tw.edit.meaning {
	case EditParameterAdd:
		// "name = expression", or a name alone for a parameter of 0.
		n, expr, found := strings.Cut(s, "=")
		n = strings.TrimSpace(n)
		if !found {
			expr = "0"
		}
		ss.parameterEdit(func() error { return sk.AddParameter(n, expr) })
	case EditParameterExpr:
		n := name()
		ss.parameterEdit(func() error { return sk.SetParameterExpr(n, s) })
	case EditParameterName:
		n := name()
		ss.parameterEdit(func() error { return sk.RenameParameter(n, strings.TrimSpace(s)) })
	default:
		return false
	}
	return true
}

// ShowParameters is the screen of the parameter table.
func (tw *TextWindow) ShowParameters() {
	sk := tw.ss.SK
	tw.Printf(true, "%FtPARAMETERS%E")
	if len(sk.Parameters) == 0 {
		tw.Printf(false, "%Ba   (none; a dimension or a group value may name one in an expression)")
	} else {
		tw.Printf(false, "%Ft   %s%s%s%E", pad("name", 13), pad("value", 13), pad("expression", 14))
	}
	for i, p := range sk.Parameters {
		bg := byte('d')
		if i%2 == 0 {
			bg = 'a'
		}
		tw.Printf(false, "%Bp   %s%s%s %Fl%Le%D%f[edit]%E %Fl%Ln%D%f[rename]%E %Fl%Lx%D%f[delete]%E",
			bg, pad(p.Name, 13), pad(formatParameter(p.Value), 13), pad(p.Expr, 14),
			i, lf(tw.ScreenChangeParameter),
			i, lf(tw.ScreenChangeParameter),
			i, lf(tw.ScreenChangeParameter))
		if p.Err != "" {
			tw.Printf(false, "%Bp   %Fx! %s%E", bg, p.Err)
		}
	}
	tw.Printf(true, "   %Fl%La%f[add a parameter]%E   (type: name = expression)", lf(tw.ScreenChangeParameter))

	if len(sk.ExprErrors) > 0 {
		tw.Printf(true, "%FtEXPRESSIONS THAT COULD NOT BE EVALUATED%E")
		for _, e := range sk.ExprErrors {
			g := sk.GetGroup(e.Group)
			name := "?"
			if g != nil {
				name = g.Name
			}
			tw.Printf(false, "   %Fx%s%E", e.Error())
			tw.Printf(false, "     in group %Fl%Ll%D%f%s%E", e.Group.V, lf(tw.ScreenSelectGroup), name)
		}
	}
	tw.Printf(false, "")
	tw.Printf(false, "   Values are in millimetres and degrees, whatever the units of the view.")
	tw.Printf(true, "(%Fl%Ll%fback to the groups%E)", lf(tw.ScreenHome))
}
