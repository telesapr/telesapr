package tkgui

import (
	"fmt"
	"slices"
	"strings"
	"unicode"

	"github.com/msorc/takigo/dialog"
	"github.com/msorc/takigo/event"
	"github.com/msorc/takigo/font"
	"github.com/msorc/takigo/geometry/pack"
	"github.com/msorc/takigo/geometry/place"
	"github.com/msorc/takigo/option"
	tkplatform "github.com/msorc/takigo/platform"
	"github.com/msorc/takigo/screenunit"
	"github.com/msorc/takigo/widget"
	"github.com/msorc/takigo/widget/button"
	"github.com/msorc/takigo/widget/frame"
	"github.com/msorc/takigo/widget/label"
	"github.com/msorc/takigo/window"

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
	// The dialog's toplevel while it runs.
	win *window.Window
}

type dialogButton struct {
	label     string
	underline int
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
// underlined and pressed with Alt as in GTK.
func (d *MessageDialog) AddButton(label string, r platform.Response, isDefault bool) {
	label, underline := prepareMnemonics(label)
	d.buttons = append(d.buttons, dialogButton{label, underline, r, isDefault})
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
		label.WrapLength(screenunit.In(4)), label.JustifyOpt(option.JustifyLeft), label.Anchor(option.AnchorW)),
		pack.SideOpt(pack.Top), pack.Anchor(option.AnchorW))
	if d.description != "" {
		pack.Pack(label.New(text, "desc", label.Text(d.description),
			label.WrapLength(screenunit.In(4)), label.JustifyOpt(option.JustifyLeft), label.Anchor(option.AnchorW)),
			pack.SideOpt(pack.Top), pack.Anchor(option.AnchorW), pack.PadYPair(8, 0))
	}

	// Results are 100 + the button's index, clear of dialog's own.
	// Packed from the right, so the last button goes in first.
	const base = 100
	btns := make([]*window.Window, len(d.buttons))
	def := -1
	for i := len(d.buttons) - 1; i >= 0; i-- {
		b := d.buttons[i]
		res := dialog.DialogResult(base + i)
		btn := button.New(dlg.BtnFrame, fmt.Sprintf("b%d", i), button.Text(b.label),
			button.Command(func() { dlg.Close(res) }))
		btn.Underline = b.underline
		underlineButton(btn)
		pack.Pack(btn, pack.SideOpt(pack.Right), pack.PadX(5))
		btns[i] = btn.Window()
		if b.isDefault {
			def = i
		}
	}
	// Keys go to the focused widget, so watch them all, for those in
	// this dialog (as takigo's dialog does for Escape): Return presses
	// the focused button or else the default one, Alt and a mnemonic
	// its button.
	tw := dlg.Toplevel.Window()
	d.win = tw
	d.p.app.Dispatcher().BindGlobalFor(tw.PlatformID, event.KeyPressMask, func(ev *event.Event) {
		if ev.Type != event.KeyPressType || ev.Handled || !inToplevel(tw, ev.Window) {
			return
		}
		pressed := -1
		switch {
		case ev.KeySym == tkplatform.XK_Return || ev.KeySym == xkKPEnter:
			pressed = def
			if i := slices.Index(btns, widget.FocusWindow(d.p.app)); i >= 0 {
				pressed = i
			}
		case ev.State&(tkplatform.Mod1Mask|tkplatform.ControlMask) == tkplatform.Mod1Mask:
			r := unicode.ToLower(tkplatform.KeySymToRune(ev.KeySym))
			for i, b := range d.buttons {
				if runes := []rune(b.label); r > 0 && b.underline >= 0 && b.underline < len(runes) &&
					unicode.ToLower(runes[b.underline]) == r {
					pressed = i
					break
				}
			}
		}
		if pressed >= 0 {
			dlg.Close(dialog.DialogResult(base + pressed))
		}
	})

	res := int(dlg.Run()) - base
	if res < 0 || res >= len(d.buttons) {
		return platform.ResponseNone
	}
	return d.buttons[res].response
}

// inToplevel reports whether window id is top or inside it.
func inToplevel(top *window.Window, id tkplatform.WindowID) bool {
	for w := top.Display.LookupWindow(id); w != nil; w = w.Parent {
		if w == top {
			return true
		}
		if w.IsTopLevel() {
			return false
		}
	}
	return false
}

// underlineButton draws the underline of b's mnemonic, which takigo's
// buttons keep but don't draw: a frame of the text's colour under the
// character, where Tk draws it (the text is centred, one line high).
func underlineButton(b *button.Button) {
	runes := []rune(b.Text)
	if b.Underline < 0 || b.Underline >= len(runes) || b.Font == nil {
		return
	}
	fg := "black"
	if b.Foreground != nil {
		fg = fmt.Sprintf("#%06x", b.Foreground.Pixel&0xffffff)
	}
	ul := frame.New(b, "underline", frame.Background(fg), frame.BorderWidth(0),
		frame.HighlightThickness(0))
	f := b.Font
	textW := f.MeasureString(b.Text)
	ulX := f.MeasureString(string(runes[:b.Underline]))
	ulW := f.MeasureString(string(runes[b.Underline]))
	m := f.Metrics()
	pos, ulH := font.Underline(f)
	win := b.Window()
	b.AppContext().Dispatcher().Bind(win.PlatformID, event.StructureNotifyMask, func(ev *event.Event) {
		if ev.Type != event.ConfigureType {
			return
		}
		x := (ev.ConfigWidth-textW)/2 + ulX
		y := (ev.ConfigHeight-m.Linespace())/2 + m.Ascent + pos
		place.Place(ul, place.In(b), place.X(x), place.Y(y), place.Width(ulW), place.Height(ulH))
	})
}
