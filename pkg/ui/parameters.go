package ui

import (
	"strconv"
	"strings"
)

// This file is an extension: C++ SolveSpace has no named parameters.
// The screen lists the sketch's parameter table, with what could not
// be evaluated, and edits it.

// ScreenShowParameters shows the parameter table.
func (tw *TextWindow) ScreenShowParameters(link int, v uint32) {
	tw.GoToScreen(ScreenParameters)
}

// MenuParameters shows the parameter table, from the menu.
func (ss *Application) MenuParameters() {
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
	if link == 'a' {
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

// ScreenChangeConfiguration activates configuration v-1, or none for v
// = 0 ('c'), renames ('r') or deletes it ('x'), or adds one ('A').
func (tw *TextWindow) ScreenChangeConfiguration(link int, v uint32) {
	ss := tw.ss
	sk := ss.SK
	if link == 'A' {
		tw.ShowEditControl(3, "")
		tw.edit.meaning = EditConfigurationAdd
		return
	}
	if link == 'c' && v == 0 {
		ss.parameterEdit(func() error { return sk.ActivateConfiguration("") })
		return
	}
	i := int(v) - 1
	if i < 0 || i >= len(sk.Configurations) {
		return
	}
	name := sk.Configurations[i].Name
	switch link {
	case 'c':
		ss.parameterEdit(func() error { return sk.ActivateConfiguration(name) })
	case 'r':
		tw.edit.i = i
		tw.ShowEditControl(3, name)
		tw.edit.meaning = EditConfigurationName
	case 'x':
		ss.parameterEdit(func() error { return sk.DeleteConfiguration(name) })
	}
}

// parameterEdit runs an edit of the table as one undo step, and shows
// why when it is refused.
func (ss *Application) parameterEdit(fn func() error) {
	if err := ss.SK.UndoRememberFor(fn); err != nil {
		ss.Error("%s.", capitalize(err.Error()))
		return
	}
	ss.SK.Unsaved = true
	ss.UndoEnableMenus()
	ss.SK.ScheduleGenerateAll()
	ss.ScheduleShowTW()
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
	case EditConfigurationAdd:
		ss.parameterEdit(func() error { return sk.AddConfiguration(strings.TrimSpace(s)) })
	case EditConfigurationName:
		if tw.edit.i < len(sk.Configurations) {
			from := sk.Configurations[tw.edit.i].Name
			ss.parameterEdit(func() error { return sk.RenameConfiguration(from, strings.TrimSpace(s)) })
		}
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

	// The configurations: the table filled one way or another (roadmap
	// 16.2).
	radio := func(b bool) string {
		if b {
			return radioTrue
		}
		return radioFalse
	}
	tw.Printf(true, "%FtCONFIGURATIONS%E   (each a set of the expressions above; one may be active)")
	tw.Printf(false, "%Ba   %Fl%Lc%D%f%s none%E   (the table changes alone)",
		0, lf(tw.ScreenChangeConfiguration), radio(sk.ActiveConfiguration == ""))
	for i, c := range sk.Configurations {
		bg := byte('d')
		if i%2 == 1 {
			bg = 'a'
		}
		tw.Printf(false, "%Bp   %Fl%Lc%D%f%s %s%E %Fl%Lr%D%f[rename]%E %Fl%Lx%D%f[delete]%E",
			bg, i+1, lf(tw.ScreenChangeConfiguration), radio(sk.ActiveConfiguration == c.Name), c.Name,
			i+1, lf(tw.ScreenChangeConfiguration),
			i+1, lf(tw.ScreenChangeConfiguration))
	}
	tw.Printf(false, "   %Fl%LA%f[add a configuration of the table as it is]%E   (type its name)",
		lf(tw.ScreenChangeConfiguration))

	if len(sk.ExprErrors) > 0 {
		tw.Printf(true, "%FtEXPRESSIONS THAT COULD NOT BE EVALUATED%E")
		for _, e := range sk.ExprErrors {
			g := sk.Group(e.Group)
			name := "?"
			if g != nil {
				name = g.Name
			}
			tw.Printf(false, "   %Fx%s%E", e.Error())
			tw.Printf(false, "     %Fl%Ll%D%fin group %s%E", e.Group.V, lf(tw.ScreenSelectGroup), name)
		}
	}
	tw.Printf(false, "")
	tw.Printf(false, "   A parameter driving a length counts in the document's unit, %s;", tw.ss.UnitName())
	tw.Printf(false, "   one driving an angle, in degrees.")
	tw.Printf(true, "(%Fl%Ll%fback to the groups%E)", lf(tw.ScreenHome))
}
