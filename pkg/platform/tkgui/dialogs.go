package tkgui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/msorc/takigo/dialog"
	"github.com/msorc/takigo/event"
	"github.com/msorc/takigo/geometry/pack"
	tkplatform "github.com/msorc/takigo/platform"
	"github.com/msorc/takigo/widget"
	"github.com/msorc/takigo/widget/button"
	"github.com/msorc/takigo/widget/label"

	"github.com/spacesolve/spacesolve/pkg/core"
	"github.com/spacesolve/spacesolve/pkg/platform"
)

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
		title = "SolveSpace"
	}
	dlg := dialog.New(d.parent, title, 360, 120)
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
				if ev.KeySym == tkplatform.XK_Return {
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

// FileDialog is Tk's file dialog (C++ FileDialogImplGtk). Tk's has no
// default extension, so the chosen filter's first one is appended to a
// name without one.
type FileDialog struct {
	p        *Platform
	parent   widget.Caregiver
	save     bool
	title    string
	dir      string
	name     string
	filters  []platform.FileFilter
	filename string
}

// CreateOpenFileDialog makes a dialog asking for a file to open.
func (p *Platform) CreateOpenFileDialog(parent platform.Window) platform.FileDialog {
	return &FileDialog{p: p, parent: p.parentOf(parent)}
}

// CreateSaveFileDialog makes a dialog asking where to save.
func (p *Platform) CreateSaveFileDialog(parent platform.Window) platform.FileDialog {
	return &FileDialog{p: p, parent: p.parentOf(parent), save: true}
}

func (d *FileDialog) SetTitle(s string)       { d.title = s }
func (d *FileDialog) SetCurrentName(s string) { d.name = s }
func (d *FileDialog) GetFilename() string     { return d.filename }

// SetFilename sets the folder and name.
func (d *FileDialog) SetFilename(path string) {
	d.dir, d.name = filepath.Dir(path), filepath.Base(path)
	d.filename = path
}

// SuggestFilename sets the folder, and the name with the first
// filter's extension.
func (d *FileDialog) SuggestFilename(path string) {
	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	if len(d.filters) > 0 && len(d.filters[0].Extensions) > 0 {
		name += "." + d.filters[0].Extensions[0]
	}
	d.dir, d.name = filepath.Dir(path), name
}

func (d *FileDialog) AddFilter(f platform.FileFilter)     { d.filters = append(d.filters, f) }
func (d *FileDialog) AddFilters(fs []platform.FileFilter) { d.filters = append(d.filters, fs...) }

// FreezeChoices remembers the folder (C++ FreezeChoices).
func (d *FileDialog) FreezeChoices(s core.Settings, key string) {
	if d.filename != "" {
		s.FreezeString("Dialog_"+key+"_Folder", filepath.Dir(d.filename))
	}
}

// ThawChoices starts in the folder FreezeChoices remembered.
func (d *FileDialog) ThawChoices(s core.Settings, key string) {
	if dir := s.ThawString("Dialog_"+key+"_Folder", ""); dir != "" {
		d.dir = dir
	}
}

// RunModal shows the dialog; false if it was cancelled.
func (d *FileDialog) RunModal() bool {
	var types []dialog.FileType
	for _, f := range d.filters {
		var pats []string
		for _, e := range f.Extensions {
			pats = append(pats, "*."+e)
		}
		types = append(types, dialog.FileType{
			Name:    f.Name + " (" + strings.Join(pats, " ") + ")",
			Pattern: strings.Join(pats, " "),
		})
	}
	dir := d.dir
	if dir == "" {
		dir, _ = os.Getwd()
	}
	opts := []dialog.FileOption{dialog.FileTitle(d.title), dialog.FileInitialDir(dir), dialog.FileTypes(types...)}
	var path string
	var ok bool
	if d.save {
		path, ok = dialog.SaveFile(d.parent, opts...)
	} else {
		path, ok = dialog.OpenFile(d.parent, opts...)
	}
	if !ok || path == "" {
		return false
	}
	if d.save && filepath.Ext(path) == "" && len(d.filters) > 0 && len(d.filters[0].Extensions) > 0 {
		path += "." + d.filters[0].Extensions[0]
	}
	d.filename = path
	return true
}
