package tkgui

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/msorc/takigo/dialog"
	"github.com/msorc/takigo/event"
	"github.com/msorc/takigo/geometry/pack"
	"github.com/msorc/takigo/option"
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

// SetMessage and SetDescription drop trailing blank lines, which
// would only pad the dialog (SolveSpace's texts end in newlines).
func (d *MessageDialog) SetMessage(s string) { d.message = strings.TrimRightFunc(s, unicode.IsSpace) }
func (d *MessageDialog) SetDescription(s string) {
	d.description = strings.TrimRightFunc(s, unicode.IsSpace)
}

// AddButton adds a button; a '&' in its label marks the mnemonic,
// which Tk buttons don't show.
func (d *MessageDialog) AddButton(label string, r platform.Response, isDefault bool) {
	label, _ = prepareMnemonics(label)
	d.buttons = append(d.buttons, dialogButton{label, r, isDefault})
}

// RunModal shows the dialog and returns the button pressed, or
// ResponseNone if it was closed otherwise. It is laid out as GTK's
// message dialog: a large icon, the message (bold when there is a
// description) over the description, both wrapped and left-aligned,
// and the buttons at the right.
func (d *MessageDialog) RunModal() platform.Response {
	title := d.title
	if title == "" {
		title = "Message"
	}
	dlg := dialog.New(d.parent, prepareTitle(title), 300, 100)
	icon := map[platform.DialogType]string{
		platform.DialogInformation: "ℹ", platform.DialogQuestion: "?",
		platform.DialogWarning: "⚠", platform.DialogError: "✖",
	}[d.typ]
	pack.Pack(label.New(dlg.Content, "icon", label.Text(icon), label.FontOpt("sans-serif 24 bold")),
		pack.SideOpt(pack.Left), pack.Anchor(option.AnchorN), pack.PadX(12), pack.PadY(12))

	text := newFrame(dlg.Content, "text")
	pack.Pack(text, pack.SideOpt(pack.Left), pack.FillOpt(pack.FillBoth), pack.Expand(true),
		pack.PadXPair(0, 12), pack.PadY(12))
	msgFont := "TkDefaultFont"
	if d.description != "" {
		msgFont = "TkHeadingFont"
	}
	pack.Pack(label.New(text, "msg", label.Text(d.message), label.FontOpt(msgFont),
		label.WrapLength("4i"), label.JustifyOpt(option.JustifyLeft), label.Anchor(option.AnchorW)),
		pack.SideOpt(pack.Top), pack.Anchor(option.AnchorW))
	if d.description != "" {
		pack.Pack(label.New(text, "desc", label.Text(d.description),
			label.WrapLength("4i"), label.JustifyOpt(option.JustifyLeft), label.Anchor(option.AnchorW)),
			pack.SideOpt(pack.Top), pack.Anchor(option.AnchorW), pack.PadYPair(8, 0))
	}

	// Results are 100 + the button's index, clear of dialog's own.
	// Packed from the right, so the last button goes in first.
	const base = 100
	for i := len(d.buttons) - 1; i >= 0; i-- {
		b := d.buttons[i]
		res := dialog.DialogResult(base + i)
		btn := button.New(dlg.BtnFrame, fmt.Sprintf("b%d", i), button.Text(b.label),
			button.Command(func() { dlg.Close(res) }))
		pack.Pack(btn, pack.SideOpt(pack.Right), pack.PadX(5))
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
