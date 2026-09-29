package tkgui

import (
	"fmt"

	"github.com/msorc/takigo/dialog"
	"github.com/msorc/takigo/event"
	"github.com/msorc/takigo/geometry/pack"
	tkplatform "github.com/msorc/takigo/platform"
	"github.com/msorc/takigo/widget"
	"github.com/msorc/takigo/widget/button"
	"github.com/msorc/takigo/widget/label"

	"github.com/spacesolve/spacesolve/pkg/platform"
)

// prepareTitle is the GTK backend's window title: the application's
// name after the window's own (C++ PrepareTitle).
func prepareTitle(title string) string { return title + " — SolveSpace" }

func (p *Platform) parentOf(w platform.Window) widget.Caregiver {
	if tw, ok := w.(*Window); ok && tw != nil {
		return tw.top
	}
	return p.app
}

// MessageDialog is a modal Tk dialog with the application's buttons
// (C++ MessageDialogImplGtk).
type MessageDialog struct {
	p           *Platform
	parent      widget.Caregiver
	typ         platform.DialogType
	title       string
	message     string
	description string
	buttons     []dialogButton
}

type dialogButton struct {
	label     string
	response  platform.Response
	isDefault bool
}

// CreateMessageDialog makes a message dialog over parent.
func (p *Platform) CreateMessageDialog(parent platform.Window) platform.MessageDialog {
	return &MessageDialog{p: p, parent: p.parentOf(parent)}
}

func (d *MessageDialog) SetType(t platform.DialogType) { d.typ = t }
func (d *MessageDialog) SetTitle(s string)             { d.title = s }
func (d *MessageDialog) SetMessage(s string)           { d.message = s }
func (d *MessageDialog) SetDescription(s string)       { d.description = s }

// AddButton adds a button; a '&' in its label marks the mnemonic,
// which Tk buttons don't show.
func (d *MessageDialog) AddButton(label string, r platform.Response, isDefault bool) {
	label, _ = prepareMnemonics(label)
	d.buttons = append(d.buttons, dialogButton{label, r, isDefault})
}

// RunModal shows the dialog and returns the button pressed, or
// ResponseNone if it was closed otherwise.
func (d *MessageDialog) RunModal() platform.Response {
	title := d.title
	if title == "" {
		title = "Message"
	}
	dlg := dialog.New(d.parent, prepareTitle(title), 360, 120)
	icon := map[platform.DialogType]string{
		platform.DialogInformation: "ℹ", platform.DialogQuestion: "?",
		platform.DialogWarning: "⚠", platform.DialogError: "✖",
	}[d.typ]
	pack.Pack(label.New(dlg.Content, "icon", label.Text(icon), label.FontOpt("TkHeadingFont")),
		pack.SideOpt(pack.Left), pack.PadX(12), pack.PadY(10))
	text := d.message
	if d.description != "" {
		text += "\n\n" + d.description
	}
	pack.Pack(label.New(dlg.Content, "msg", label.Text(text), label.WrapLength("4i"),
		label.JustifyOpt(0)),
		pack.SideOpt(pack.Left), pack.FillOpt(pack.FillBoth), pack.Expand(true), pack.PadX(6), pack.PadY(10))

	// Results are 100 + the button's index, clear of dialog's own.
	const base = 100
	for i, b := range d.buttons {
		res := dialog.DialogResult(base + i)
		btn := button.New(dlg.BtnFrame, fmt.Sprintf("b%d", i), button.Text(b.label),
			button.Command(func() { dlg.Close(res) }))
		pack.Pack(btn, pack.SideOpt(pack.Left), pack.PadX(5))
		if b.isDefault {
			d.p.app.Dispatcher().Bind(dlg.Toplevel.Window().PlatformID, event.KeyPressMask, func(ev *event.Event) {
				if ev.KeySym == tkplatform.XK_Return || ev.KeySym == xkKPEnter {
					dlg.Close(res)
				}
			})
		}
	}
	res := int(dlg.Run()) - base
	if res < 0 || res >= len(d.buttons) {
		return platform.ResponseNone
	}
	return d.buttons[res].response
}
