package tkgui

import (
	"strconv"
	"unicode"

	"github.com/takigo/takigo/dialog"
	"github.com/takigo/takigo/event"
	"github.com/takigo/takigo/geometry/pack"
	"github.com/takigo/takigo/option"
	tkplatform "github.com/takigo/takigo/platform"
	"github.com/takigo/takigo/widget"
	"github.com/takigo/takigo/widget/button"
	"github.com/takigo/takigo/widget/checkbutton"
	"github.com/takigo/takigo/widget/entry"
	"github.com/takigo/takigo/widget/label"
	"github.com/takigo/takigo/widget/radiobutton"

	"github.com/telesapr/telesapr/pkg/platform"
)

// formLabelWidth is the width, in characters, the labels of a form are
// given so that the fields line up.
const formLabelWidth = 24

// FormDialog is a modal Tk popup with a row for each field.
type FormDialog struct {
	p        *Platform
	parent   widget.Caregiver
	title    string
	accept   string
	cancel   string
	fields   []*formField
	onChange func(key, value string) platform.FormValues
	// applying is set while the values onChange returned are put into
	// the fields, whose own change notices must not call it again.
	applying bool
}

// formField is one row: the widget that holds its value, and the way
// to set it from a string.
type formField struct {
	key   string
	label string
	hint  string
	// Exactly one of the three is set, by the kind of the field.
	choices []string
	choice  *widget.Variable[string]
	text    string
	entry   *entry.Entry
	check   *widget.Variable[bool]
}

// CreateFormDialog makes a form dialog over parent.
func (p *Platform) CreateFormDialog(parent platform.Window) platform.FormDialog {
	return &FormDialog{p: p, parent: p.parentOf(parent), accept: "OK", cancel: "Cancel"}
}

func (d *FormDialog) SetTitle(s string) { d.title = s }

func (d *FormDialog) SetButtons(accept, cancel string) { d.accept, d.cancel = accept, cancel }

func (d *FormDialog) AddChoice(key, label string, choices []string, selected int) {
	d.fields = append(d.fields, &formField{key: key, label: label, choices: choices,
		choice: widget.NewVariable(strconv.Itoa(selected))})
}

func (d *FormDialog) AddEntry(key, label, text, hint string) {
	d.fields = append(d.fields, &formField{key: key, label: label, text: text, hint: hint})
}

func (d *FormDialog) AddCheck(key, label string, on bool) {
	d.fields = append(d.fields, &formField{key: key, label: label, check: widget.NewVariable(on)})
}

func (d *FormDialog) OnChange(fn func(key, value string) platform.FormValues) { d.onChange = fn }

// changed tells onChange that f now has value, which the user has
// given it, and puts what it answers into the other fields. An entry's
// value is the prospective text: the validator sees the edit before it
// is made.
func (d *FormDialog) changed(f *formField, value string) {
	if d.onChange == nil || d.applying {
		return
	}
	d.applying = true
	defer func() { d.applying = false }()
	for key, v := range d.onChange(f.key, value) {
		for _, g := range d.fields {
			if g.key == key && g != f {
				g.set(v)
			}
		}
	}
}

// value is the field's value as FormValues hold it.
func (f *formField) value() string {
	switch {
	case f.choice != nil:
		return f.choice.Get()
	case f.check != nil:
		if f.check.Get() {
			return "1"
		}
		return "0"
	}
	return f.entry.GetText()
}

func (f *formField) set(v string) {
	switch {
	case f.choice != nil:
		f.choice.Set(v)
	case f.check != nil:
		f.check.Set(v == "1")
	default:
		f.entry.SetText(v)
	}
}

// RunModal shows the form, with the rows packed from the top: a label
// and the field's widgets side by side.
func (d *FormDialog) RunModal() (platform.FormValues, bool) {
	title := d.title
	if title == "" {
		title = "Options"
	}
	dlg := dialog.New(d.parent, prepareTitle(title), 360, 100)
	body := newFrame(dlg.Content, "form")
	_ = pack.Pack(body, pack.SideOpt(pack.Top), pack.FillOpt(pack.FillBoth), pack.Expand(true),
		pack.PadX(12), pack.PadY(10))

	for i, f := range d.fields {
		name := "row" + strconv.Itoa(i)
		row := newFrame(body, name)
		_ = pack.Pack(row, pack.SideOpt(pack.Top), pack.FillOpt(pack.FillX), pack.PadY(3))
		if f.check == nil {
			_ = pack.Pack(label.New(row, "label", label.Text(f.label), label.Width(formLabelWidth),
				label.Anchor(option.AnchorW)), pack.SideOpt(pack.Left))
		}
		switch {
		case f.choice != nil:
			for j, c := range f.choices {
				rb := radiobutton.New(row, "c"+strconv.Itoa(j), radiobutton.Text(c),
					radiobutton.Value(strconv.Itoa(j)), radiobutton.Var(f.choice),
					radiobutton.Command(func() { d.changed(f, f.value()) }))
				_ = pack.Pack(rb, pack.SideOpt(pack.Left), pack.PadXPair(0, 8))
			}
		case f.check != nil:
			cb := checkbutton.New(row, "check", checkbutton.Text(f.label), checkbutton.BoolVar(f.check),
				checkbutton.Command(func() { d.changed(f, f.value()) }))
			_ = pack.Pack(cb, pack.SideOpt(pack.Left))
		default:
			f.entry = entry.New(row, "entry", entry.Text(f.text), entry.Width(12),
				entry.ValidateOpt("key"), entry.ValidateCmdOpt(func(prospective string) bool {
					d.changed(f, prospective)
					return true
				}))
			_ = pack.Pack(f.entry, pack.SideOpt(pack.Left))
			if f.hint != "" {
				_ = pack.Pack(label.New(row, "hint", label.Text(f.hint)), pack.SideOpt(pack.Left), pack.PadX(8))
			}
		}
	}

	const base = 100
	accept := dialog.DialogResult(base)
	labels := [2]string{d.accept, d.cancel}
	results := [2]dialog.DialogResult{accept, dialog.ResultCancel}
	var underlines [2]int
	// Packed from the right, so the accept button goes in last.
	for i := len(labels) - 1; i >= 0; i-- {
		text, underline := prepareMnemonics(labels[i])
		labels[i], underlines[i] = text, underline
		res := results[i]
		btn := button.New(dlg.BtnFrame, "b"+strconv.Itoa(i), button.Text(text),
			button.Command(func() { dlg.Close(res) }))
		btn.Underline = underline
		underlineButton(btn)
		_ = pack.Pack(btn, pack.SideOpt(pack.Right), pack.PadX(5))
	}
	// Return accepts, and Alt with a mnemonic presses its button,
	// wherever the focus is in the dialog.
	tw := dlg.Toplevel.Window()
	d.p.app.Dispatcher().BindGlobalFor(tw.PlatformID, event.KeyPressMask, func(ev *event.Event) {
		if ev.Type != event.KeyPressType || ev.Handled || !inToplevel(tw, ev.Window) {
			return
		}
		switch {
		case ev.KeySym == tkplatform.XK_Return || ev.KeySym == xkKPEnter:
			dlg.Close(accept)
		case ev.State&(tkplatform.Mod1Mask|tkplatform.ControlMask) == tkplatform.Mod1Mask:
			r := unicode.ToLower(tkplatform.KeySymToRune(ev.KeySym))
			for i, text := range labels {
				if runes := []rune(text); r > 0 && underlines[i] >= 0 && underlines[i] < len(runes) &&
					unicode.ToLower(runes[underlines[i]]) == r {
					dlg.Close(results[i])
					return
				}
			}
		}
	})

	res := dlg.Run()
	if res != accept {
		return nil, false
	}
	v := make(platform.FormValues, len(d.fields))
	for _, f := range d.fields {
		v[f.key] = f.value()
	}
	return v, true
}
