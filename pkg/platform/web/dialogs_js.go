//go:build js && wasm

package web

import (
	"strconv"
	"strings"
	"syscall/js"
	"unicode"

	"github.com/telesapr/telesapr/pkg/platform"
)

// funcSet holds the functions a modal dialog gives the page, released
// when it is over.
type funcSet []js.Func

func (s *funcSet) add(f func(args []js.Value) any) js.Func {
	jf := js.FuncOf(func(_ js.Value, args []js.Value) any { return f(args) })
	*s = append(*s, jf)
	return jf
}

func (s *funcSet) release() {
	for _, f := range *s {
		f.Release()
	}
	*s = nil
}

// MessageDialog is a modal HTML dialog with the application's buttons.
type MessageDialog struct {
	p           *Platform
	typ         platform.DialogType
	title       string
	message     string
	description string
	buttons     []dialogButton
}

type dialogButton struct {
	Label    string `json:"label"`
	Default  bool   `json:"default"`
	response platform.Response
}

var _ platform.MessageDialog = (*MessageDialog)(nil)

// CreateMessageDialog makes a message dialog.
func (p *Platform) CreateMessageDialog(platform.Window) platform.MessageDialog {
	return &MessageDialog{p: p}
}

func (d *MessageDialog) SetType(t platform.DialogType) { d.typ = t }
func (d *MessageDialog) SetTitle(s string)             { d.title = s }

// SetMessage and SetDescription drop trailing blank lines, which would
// only pad the dialog (the application's texts end in newlines).
func (d *MessageDialog) SetMessage(s string) { d.message = strings.TrimRightFunc(s, unicode.IsSpace) }
func (d *MessageDialog) SetDescription(s string) {
	d.description = strings.TrimRightFunc(s, unicode.IsSpace)
}

// AddButton adds a button; a '&' in its label marks the mnemonic,
// underlined and pressed with Alt.
func (d *MessageDialog) AddButton(label string, r platform.Response, isDefault bool) {
	d.buttons = append(d.buttons, dialogButton{Label: label, Default: isDefault, response: r})
}

// RunModal shows the dialog, runs the event queue until the page has
// the answer, and returns the button pressed, or ResponseNone if the
// dialog was closed otherwise (Escape).
func (d *MessageDialog) RunModal() platform.Response {
	title := d.title
	if title == "" {
		title = "Message"
	}
	spec := jsonString(map[string]any{
		"type": int(d.typ), "title": title, "message": d.message,
		"description": d.description, "buttons": d.buttons,
	})
	result := -1
	var fns funcSet
	defer fns.release()
	d.p.runModal(func(end func(func())) {
		d.p.w.Call("messageDialog", spec, fns.add(func(a []js.Value) any {
			i := a[0].Int()
			end(func() { result = i })
			return nil
		}))
	})
	if result < 0 || result >= len(d.buttons) {
		return platform.ResponseNone
	}
	return d.buttons[result].response
}

// FormDialog is a modal HTML form with a row for each field.
type FormDialog struct {
	p        *Platform
	title    string
	accept   string
	cancel   string
	fields   []formField
	onChange func(key, value string) platform.FormValues
}

type formField struct {
	Key     string   `json:"key"`
	Label   string   `json:"label"`
	Kind    string   `json:"kind"`
	Choices []string `json:"choices,omitempty"`
	Hint    string   `json:"hint,omitempty"`
	Value   string   `json:"value"`
}

var _ platform.FormDialog = (*FormDialog)(nil)

// CreateFormDialog makes a form dialog.
func (p *Platform) CreateFormDialog(platform.Window) platform.FormDialog {
	return &FormDialog{p: p, accept: "OK", cancel: "Cancel"}
}

func (d *FormDialog) SetTitle(s string)                { d.title = s }
func (d *FormDialog) SetButtons(accept, cancel string) { d.accept, d.cancel = accept, cancel }

func (d *FormDialog) AddChoice(key, label string, choices []string, selected int) {
	d.fields = append(d.fields, formField{Key: key, Label: label, Kind: "choice", Choices: choices,
		Value: strconv.Itoa(selected)})
}

func (d *FormDialog) AddEntry(key, label, text, hint string) {
	d.fields = append(d.fields, formField{Key: key, Label: label, Kind: "entry", Value: text, Hint: hint})
}

func (d *FormDialog) AddCheck(key, label string, on bool) {
	v := "0"
	if on {
		v = "1"
	}
	d.fields = append(d.fields, formField{Key: key, Label: label, Kind: "check", Value: v})
}

func (d *FormDialog) OnChange(fn func(key, value string) platform.FormValues) { d.onChange = fn }

// RunModal shows the form and runs the event queue until it is
// answered; the change callback runs on the loop as the user edits, and
// what it answers goes into the other fields.
func (d *FormDialog) RunModal() (platform.FormValues, bool) {
	title := d.title
	if title == "" {
		title = "Options"
	}
	spec := jsonString(map[string]any{
		"title": title, "accept": d.accept, "cancel": d.cancel, "fields": d.fields,
	})
	var values platform.FormValues
	var ok bool
	var fns funcSet
	defer fns.release()
	var dlg js.Value
	d.p.runModal(func(end func(func())) {
		change := fns.add(func(a []js.Value) any {
			key, value := a[0].String(), a[1].String()
			d.p.q.post(func() {
				if d.onChange == nil {
					return
				}
				for k, v := range d.onChange(key, value) {
					if k != key {
						dlg.Call("set", k, v)
					}
				}
			})
			return nil
		})
		done := fns.add(func(a []js.Value) any {
			text := a[0].String()
			end(func() {
				if text == "" {
					return
				}
				var v platform.FormValues
				if err := jsonUnmarshal(text, &v); err == nil {
					values, ok = v, true
				}
			})
			return nil
		})
		dlg = d.p.w.Call("formDialog", spec, change, done)
	})
	return values, ok
}
