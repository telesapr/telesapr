package ui

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/telesapr/telesapr/pkg/script"
)

// This file asks for the values of a script's form (api.Form) in a
// dialog before the script runs, as telesapr-cli run takes them with
// --set.

// scriptFormHints are what an entry of each kind shows after it.
var scriptFormHints = map[script.FieldKind]string{
	script.FieldInt:   "a whole number",
	script.FieldFloat: "a number",
}

// askScriptForm shows the form the script in path declares, if it
// declares one, and returns the values it was left with, by field
// name. ok is false when the user cancelled, or the form is wrong or
// was filled wrongly, which it says. A script it can't read has no
// form here: running it says why.
func (ss *Application) askScriptForm(path string) (values map[string]string, ok bool) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, true
	}
	form, err := script.ReadForm(string(src))
	if err != nil {
		ss.Error("The script's form can't be shown: %s.", formReason(err))
		return nil, false
	}
	if form == nil {
		return nil, true
	}
	d := ss.Platform.CreateFormDialog(ss.GW.window)
	title := form.Title
	if title == "" {
		title = filepath.Base(path)
	}
	d.SetTitle(title)
	d.SetButtons("&Run", "Cancel")
	for _, fd := range form.Fields {
		switch fd.Kind {
		case script.FieldBool:
			d.AddCheck(fd.Name, fd.Label, fd.Value == "true")
		case script.FieldChoice:
			d.AddChoice(fd.Name, fd.Label, fd.Choices, slices.Index(fd.Choices, fd.Value))
		default:
			d.AddEntry(fd.Name, fd.Label, fd.Value, scriptFormHints[fd.Kind])
		}
	}
	got, ok := d.RunModal()
	if !ok {
		return nil, false
	}
	values = make(map[string]string, len(got))
	for _, fd := range form.Fields {
		v, given := got[fd.Name]
		if !given {
			continue
		}
		switch fd.Kind {
		case script.FieldBool:
			v = strconv.FormatBool(v == "1")
		case script.FieldChoice:
			i, err := strconv.Atoi(v)
			if err != nil || i < 0 || i >= len(fd.Choices) {
				continue
			}
			v = fd.Choices[i]
		}
		values[fd.Name] = v
	}
	if _, err := form.Apply(string(src), values); err != nil {
		ss.Error("The script can't run with those values: %s.", formReason(err))
		return nil, false
	}
	return values, true
}

// formReason is what is wrong with a form or its values, without the
// error's prefix, which only says it is about a form.
func formReason(err error) string {
	if errors.Is(err, script.ErrForm) {
		return strings.TrimPrefix(err.Error(), script.ErrForm.Error()+": ")
	}
	return err.Error()
}
