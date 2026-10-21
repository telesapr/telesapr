package ui

import (
	"fmt"
	"strconv"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/platform"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// The rows of a link group's screen that point it at another file and
// pass the linked model a configuration and parameter values: the
// window's side of LinkGroup.SetFile, SetConfiguration and SetParam.

// showLinkModel shows the configuration a link regenerates its model
// in and the values it passes the model's parameters, for a model.
func (tw *textWindow) showLinkModel(g *sketch.Group) {
	if !g.LinkModelRead {
		return
	}
	if len(g.LinkConfigurations) > 0 {
		format := "%Ba   %Ftconfiguration%E %f%D%Ll%Fd%s as saved%E"
		args := []any{lf(tw.ScreenChangeLinkConfiguration), uint32(0), radioMark(g.LinkConfiguration == "")}
		for i, name := range g.LinkConfigurations {
			format += "  %f%D%Ll%Fd%s %s%E"
			args = append(args, lf(tw.ScreenChangeLinkConfiguration), uint32(i+1), radioMark(g.LinkConfiguration == name), name)
		}
		tw.Printf(false, format, args...)
	}
	for i, p := range g.LinkPartParameters {
		if lp := g.LinkParam(p.Name); lp != nil {
			value := strconv.FormatFloat(lp.Value, 'g', -1, 64)
			note := exprNote(lp.Expr)
			if lp.Expr == value {
				note = ""
			}
			tw.Printf(false, "%Bd   %Ft%s%E = %s%s %Fl%Lc%f%D[change]%E %Fl%Lx%f%D[model's]%E",
				p.Name, value, note,
				lf(tw.ScreenChangeLinkParam), uint32(i), lf(tw.ScreenChangeLinkParam), uint32(i))
			continue
		}
		tw.Printf(false, "%Bd   %Ft%s%E = %s %Fdin the model%E %Fl%Lc%f%D[pass a value]%E",
			p.Name, strconv.FormatFloat(p.Value, 'g', -1, 64), lf(tw.ScreenChangeLinkParam), uint32(i))
	}
}

// ScreenRelink points the shown link group at a file the user chooses.
func (tw *textWindow) ScreenRelink(link int, v uint32) {
	ss := tw.ss
	g := ss.SK.Group(core.HGroup{V: v})
	if g == nil || g.Type != sketch.GroupLinked {
		return
	}
	dialog := ss.Platform.CreateOpenFileDialog(ss.GW.window)
	dialog.AddFilters(platform.LinkFileFilters)
	dialog.ThawChoices(ss.Settings, "LinkSketch")
	if !dialog.RunModal() {
		return
	}
	dialog.FreezeChoices(ss.Settings, "LinkSketch")
	ss.changeLink(g.H, func(sk *sketch.Sketch) error { return sk.SetLinkFile(g.H, dialog.Filename()) })
}

// ScreenChangeLinkConfiguration regenerates the shown link's model in
// its configuration v, counting from 1, or with 0 reads it as saved.
func (tw *textWindow) ScreenChangeLinkConfiguration(link int, v uint32) {
	ss := tw.ss
	g := ss.SK.Group(tw.shown.group)
	if g == nil || g.Type != sketch.GroupLinked || int(v) > len(g.LinkConfigurations) {
		return
	}
	name := ""
	if v > 0 {
		name = g.LinkConfigurations[v-1]
	}
	if name == g.LinkConfiguration {
		return
	}
	ss.changeLink(g.H, func(sk *sketch.Sketch) error { return sk.SetLinkConfiguration(g.H, name) })
}

// ScreenChangeLinkParam edits the value the shown link passes to its
// model's parameter v ('c'), or ('x') lets the model's own stand.
func (tw *textWindow) ScreenChangeLinkParam(link int, v uint32) {
	ss := tw.ss
	g := ss.SK.Group(tw.shown.group)
	if g == nil || g.Type != sketch.GroupLinked || int(v) >= len(g.LinkPartParameters) {
		return
	}
	p := g.LinkPartParameters[v]
	if link == 'x' {
		ss.changeLink(g.H, func(sk *sketch.Sketch) error { return sk.SetLinkParam(g.H, p.Name, "") })
		return
	}
	text := strconv.FormatFloat(p.Value, 'g', -1, 64)
	if lp := g.LinkParam(p.Name); lp != nil {
		text = lp.Expr
	}
	tw.ShowEditControl(len(p.Name)+6, text)
	tw.edit.meaning = editLinkParam
	tw.edit.group = g.H
	tw.edit.i = int(v)
}

// editLinkParamDone passes the typed value to the linked model's
// parameter being edited.
func (tw *textWindow) editLinkParamDone(s string) {
	ss := tw.ss
	g := ss.SK.Group(tw.edit.group)
	if g == nil || tw.edit.i >= len(g.LinkPartParameters) {
		return
	}
	name := g.LinkPartParameters[tw.edit.i].Name
	ss.changeLink(g.H, func(sk *sketch.Sketch) error { return sk.SetLinkParam(g.H, name, s) })
}

// changeLink makes a change of a link group as one undo step, and
// shows why when it is refused.
func (ss *Application) changeLink(hg core.HGroup, fn func(sk *sketch.Sketch) error) {
	if err := ss.SK.UndoRememberFor(func() error { return fn(ss.SK) }); err != nil {
		ss.Report(fmt.Errorf("can't change the link: %w", err))
		return
	}
	ss.SK.Unsaved = true
	ss.UndoEnableMenus()
	ss.GW.ClearSuper()
}
