package tkgui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/msorc/takigo/dialog"
	"github.com/msorc/takigo/event"
	"github.com/msorc/takigo/geometry/pack"
	"github.com/msorc/takigo/option"
	tkplatform "github.com/msorc/takigo/platform"
	"github.com/msorc/takigo/widget"
	"github.com/msorc/takigo/widget/button"
	"github.com/msorc/takigo/widget/entry"
	"github.com/msorc/takigo/widget/frame"
	"github.com/msorc/takigo/widget/label"
	"github.com/msorc/takigo/widget/listbox"
	"github.com/msorc/takigo/widget/menu"
	"github.com/msorc/takigo/widget/menubutton"

	"github.com/spacesolve/spacesolve/pkg/core"
	"github.com/spacesolve/spacesolve/pkg/platform"
)

// FileDialog asks for a file to open or save (C++ FileDialogImplGtk).
// takigo's own file dialog takes neither a file name to start from nor
// a choice of filter, so this is a file chooser of its own on takigo's
// widgets: a folder listing filtered by the chosen type, the name to
// save under, the type to save as, and, as GTK's save dialog does, a
// question before an existing file is replaced.
type FileDialog struct {
	p      *Platform
	parent widget.Caregiver
	save   bool
	title  string
	// dir and name are where the dialog starts; filename is the answer.
	dir      string
	name     string
	filters  []platform.FileFilter
	filter   int // index into filters
	filename string
}

// CreateOpenFileDialog makes a dialog asking for a file to open.
func (p *Platform) CreateOpenFileDialog(parent platform.Window) platform.FileDialog {
	return &FileDialog{p: p, parent: p.parentOf(parent), title: "Open File"}
}

// CreateSaveFileDialog makes a dialog asking where to save.
func (p *Platform) CreateSaveFileDialog(parent platform.Window) platform.FileDialog {
	return &FileDialog{p: p, parent: p.parentOf(parent), save: true, title: "Save File"}
}

// SetTitle sets the title, which GTK's PrepareTitle suffixes.
func (d *FileDialog) SetTitle(s string) { d.title = s }

// SetCurrentName sets the name to save under (C++ SetCurrentName: a
// save dialog only).
func (d *FileDialog) SetCurrentName(s string) {
	if d.save {
		d.name = s
	}
}

// GetFilename is the file chosen.
func (d *FileDialog) GetFilename() string { return d.filename }

// SetFilename starts at path (C++ set_filename: its folder, and for a
// save dialog its name too).
func (d *FileDialog) SetFilename(path string) {
	d.dir = filepath.Dir(path)
	if d.save {
		d.name = filepath.Base(path)
	}
	d.filename = path
}

// SuggestFilename starts in path's folder, with its name under the
// current filter's extension (C++ SuggestFilename).
func (d *FileDialog) SuggestFilename(path string) {
	if !d.save {
		return
	}
	d.dir = filepath.Dir(path)
	d.name = fileStem(path) + "." + d.extension()
}

// AddFilter adds a file type.
func (d *FileDialog) AddFilter(f platform.FileFilter) { d.filters = append(d.filters, f) }

// AddFilters adds file types.
func (d *FileDialog) AddFilters(fs []platform.FileFilter) { d.filters = append(d.filters, fs...) }

// FreezeChoices remembers the folder and the filter (C++
// FreezeChoices).
func (d *FileDialog) FreezeChoices(s core.Settings, key string) {
	if d.filename != "" {
		s.FreezeString("Dialog_"+key+"_Folder", filepath.Dir(d.filename))
	}
	s.FreezeString("Dialog_"+key+"_Filter", d.extension())
}

// ThawChoices starts in the folder and with the filter FreezeChoices
// remembered (C++ ThawChoices).
func (d *FileDialog) ThawChoices(s core.Settings, key string) {
	if dir := s.ThawString("Dialog_"+key+"_Folder", ""); dir != "" {
		d.dir = dir
	}
	d.setExtension(s.ThawString("Dialog_"+key+"_Filter", ""))
}

// extension is the current filter's first extension (C++
// GetExtension).
func (d *FileDialog) extension() string {
	if len(d.filters) == 0 {
		return ""
	}
	i := d.filter
	if i < 0 || i >= len(d.filters) {
		i = 0
	}
	if len(d.filters[i].Extensions) == 0 {
		return ""
	}
	return d.filters[i].Extensions[0]
}

// setExtension picks the filter whose first extension is ext, or the
// first filter (C++ SetExtension).
func (d *FileDialog) setExtension(ext string) {
	d.filter = 0
	for i, f := range d.filters {
		if len(f.Extensions) > 0 && f.Extensions[0] == ext {
			d.filter = i
			return
		}
	}
}

// fileStem is path's file name without its extension (C++
// Path::FileStem).
func fileStem(path string) string {
	base := filepath.Base(path)
	if dot := strings.LastIndexByte(base, '.'); dot > 0 {
		return base[:dot]
	}
	return base
}

// filterLabel names a filter as GTK's chooser lists it: the name and
// its patterns (C++ AddFilter).
func filterLabel(f platform.FileFilter) string {
	var pats []string
	for _, e := range f.Extensions {
		if e == "" {
			pats = append(pats, "*")
		} else {
			pats = append(pats, "*."+e)
		}
	}
	return f.Name + " (" + strings.Join(pats, ", ") + ")"
}

// filterMatches reports whether name has one of f's extensions,
// ignoring case as GTK's patterns do.
func filterMatches(f platform.FileFilter, name string) bool {
	if len(f.Extensions) == 0 {
		return true
	}
	ext := strings.TrimPrefix(filepath.Ext(name), ".")
	for _, e := range f.Extensions {
		if e == "" || strings.EqualFold(e, ext) {
			return true
		}
	}
	return false
}

// RunModal shows the dialog; false if it was cancelled. takigo's
// dialogs are destroyed when they close, so an answer that is refused
// (a folder typed by name, a missing file to open, a replacement
// declined) shows the chooser again where it was.
func (d *FileDialog) RunModal() bool {
	currentDir := d.dir
	if currentDir == "" {
		currentDir, _ = os.Getwd()
	}
	if abs, err := filepath.Abs(currentDir); err == nil {
		currentDir = abs
	}
	if d.filter < 0 || d.filter >= len(d.filters) {
		d.filter = 0
	}
	name := d.name
	if !d.save {
		name = ""
	}
	for {
		answer, ok := d.show(&currentDir, name)
		if !ok {
			return false
		}
		name = answer
		path := name
		if !filepath.IsAbs(path) {
			path = filepath.Join(currentDir, name)
		}
		if st, err := os.Stat(path); err == nil && st.IsDir() {
			// A folder typed by name: go there instead.
			currentDir = path
			name = ""
			continue
		}
		if d.save {
			if filepath.Ext(name) == "" && d.extension() != "" {
				path += "." + d.extension()
			}
			if _, err := os.Stat(path); err == nil && !d.confirmReplace(path) {
				continue
			}
		} else if _, err := os.Stat(path); err != nil {
			continue
		}
		d.filename = path
		return true
	}
}

// show builds the chooser in *currentDir with name in its entry, runs
// it, and returns the name entered; ok is false when it was cancelled.
// *currentDir follows the folders the user opened.
func (d *FileDialog) show(currentDir *string, name string) (string, bool) {
	app := d.p.app
	dlg := dialog.New(d.parent, prepareTitle(d.title), 500, 480)

	// The folder, and a way up out of it.
	navFrame := newFrame(dlg.Content, "nav")
	pack.Pack(navFrame, pack.SideOpt(pack.Top), pack.FillOpt(pack.FillX), pack.PadY(2))
	dirLabel := label.New(navFrame, "dirlabel", label.Text(*currentDir), label.Anchor(option.AnchorW))
	pack.Pack(dirLabel, pack.SideOpt(pack.Left), pack.FillOpt(pack.FillX), pack.Expand(true), pack.PadX(5))

	// The name.
	entryFrame := newFrame(dlg.Content, "entryframe")
	pack.Pack(label.New(entryFrame, "fnlabel", label.Text("Name:")), pack.SideOpt(pack.Left), pack.PadX(5))
	fnEntry := entry.New(entryFrame, "fnentry", entry.Width(40))
	pack.Pack(fnEntry, pack.SideOpt(pack.Left), pack.FillOpt(pack.FillX), pack.Expand(true), pack.PadX(5))
	fnEntry.SetText(name)

	// The type, when there is a choice.
	var populateList func()
	if len(d.filters) > 0 {
		typeFrame := newFrame(dlg.Content, "typeframe")
		pack.Pack(typeFrame, pack.SideOpt(pack.Bottom), pack.FillOpt(pack.FillX), pack.PadY(2))
		pack.Pack(label.New(typeFrame, "ftlabel", label.Text("Type:")), pack.SideOpt(pack.Left), pack.PadX(5))
		typeMenu := menu.New(typeFrame, "typemenu", menu.TearOffOpt(false))
		typeButton := menubutton.New(typeFrame, "typebutton",
			menubutton.Text(filterLabel(d.filters[d.filter])),
			menubutton.MenuOpt(typeMenu), menubutton.OptionMenuOpt(true), menubutton.IndicatorOnOpt(true))
		for i, f := range d.filters {
			i, f := i, f
			typeMenu.AddRadiobutton(filterLabel(f), i == d.filter, func() {
				was := d.extension()
				d.filter = i
				typeButton.SetText(filterLabel(f))
				// A name under the old type takes the new one's extension
				// (C++ FilterChanged).
				if d.save {
					if n := fnEntry.GetText(); n != "" && d.extension() != "" {
						ext := strings.TrimPrefix(filepath.Ext(n), ".")
						if ext == "" || strings.EqualFold(ext, was) {
							fnEntry.SetText(fileStem(n) + "." + d.extension())
						}
					}
				}
				if populateList != nil {
					populateList()
				}
			})
		}
		pack.Pack(typeButton, pack.SideOpt(pack.Left), pack.FillOpt(pack.FillX), pack.Expand(true), pack.PadX(5))
	}

	// Packed after the type row, so it sits above it.
	pack.Pack(entryFrame, pack.SideOpt(pack.Bottom), pack.FillOpt(pack.FillX), pack.PadY(2))

	// The folder's entries, in what is left.
	fileList := listbox.New(dlg.Content, "filelist", listbox.Width(56), listbox.Height(14))
	pack.Pack(fileList, pack.SideOpt(pack.Top), pack.FillOpt(pack.FillBoth), pack.Expand(true),
		pack.PadX(5), pack.PadY(2))

	populateList = func() {
		entries, err := os.ReadDir(*currentDir)
		if err != nil {
			entries = nil
		}
		fileList.Delete(0, fileList.ItemCount()-1)
		items := []string{"../"}
		for _, e := range entries {
			if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
				items = append(items, e.Name()+"/")
			}
		}
		for _, e := range entries {
			if e.IsDir() || strings.HasPrefix(e.Name(), ".") {
				continue
			}
			if len(d.filters) == 0 || filterMatches(d.filters[d.filter], e.Name()) {
				items = append(items, e.Name())
			}
		}
		fileList.Insert(0, items...)
		dirLabel.Text = *currentDir
		dirLabel.Display()
	}
	populateList()

	enterDir := func(dirName string) {
		if dirName == ".." {
			*currentDir = filepath.Dir(*currentDir)
		} else {
			*currentDir = filepath.Join(*currentDir, dirName)
		}
		populateList()
	}

	// Like Tk's file box, a click puts a file's name in the entry and a
	// double click opens a folder or accepts a file.
	var lastClickTime tkplatform.Timestamp
	lastClickIdx := -1
	app.Dispatcher().Bind(fileList.Window().PlatformID, event.ButtonPressMask, func(ev *event.Event) {
		if ev.Button != 1 {
			return
		}
		sel := fileList.Selection()
		items := fileList.GetItems()
		if len(sel) == 0 || sel[0] >= len(items) {
			return
		}
		idx := sel[0]
		double := idx == lastClickIdx && uint64(ev.Time-lastClickTime) < 500
		lastClickTime, lastClickIdx = ev.Time, idx
		item := items[idx]
		dirName, isDir := strings.CutSuffix(item, "/")
		switch {
		case !isDir:
			fnEntry.SetText(item)
			if double {
				dlg.Close(dialog.ResultOK)
			}
		case double:
			lastClickIdx = -1
			enterDir(dirName)
		}
	})
	app.Dispatcher().Bind(fileList.Window().PlatformID, event.KeyPressMask, func(ev *event.Event) {
		if ev.KeySym == tkplatform.XK_Escape {
			dlg.Close(dialog.ResultCancel)
			return
		}
		if ev.KeySym != tkplatform.XK_Return {
			return
		}
		sel := fileList.Selection()
		items := fileList.GetItems()
		if len(sel) == 0 || sel[0] >= len(items) {
			return
		}
		if dirName, isDir := strings.CutSuffix(items[sel[0]], "/"); isDir {
			enterDir(dirName)
		} else {
			fnEntry.SetText(items[sel[0]])
		}
	})

	okText := "Open"
	if d.save {
		okText = "Save"
	}
	for _, b := range []struct {
		text string
		res  dialog.DialogResult
	}{{"Cancel", dialog.ResultCancel}, {okText, dialog.ResultOK}} {
		res := b.res
		btn := button.New(dlg.BtnFrame, "btn_"+b.text, button.Text(b.text),
			button.Command(func() { dlg.Close(res) }))
		pack.Pack(btn, pack.SideOpt(pack.Right), pack.PadX(5))
	}
	// Return accepts and Escape cancels, wherever the keyboard is.
	keys := func(ev *event.Event) {
		switch ev.KeySym {
		case tkplatform.XK_Return, xkKPEnter:
			dlg.Close(dialog.ResultOK)
		case tkplatform.XK_Escape:
			dlg.Close(dialog.ResultCancel)
		}
	}
	app.Dispatcher().Bind(fnEntry.Window().PlatformID, event.KeyPressMask, keys)
	app.Dispatcher().Bind(dlg.Toplevel.Window().PlatformID, event.KeyPressMask, keys)
	app.FocusManager().SetFocus(fnEntry.Window())
	fnEntry.SelectAll()

	if dlg.Run() != dialog.ResultOK {
		return "", false
	}
	answer := strings.TrimSpace(fnEntry.GetText())
	if answer == "" {
		return "", false
	}
	return answer, true
}

// xkKPEnter is the keypad's Enter.
const xkKPEnter = tkplatform.KeySym(0xff8d)

// confirmReplace asks whether to replace an existing file, as GTK's
// save dialog does with overwrite confirmation on.
func (d *FileDialog) confirmReplace(path string) bool {
	md := &MessageDialog{p: d.p, parent: d.parent, typ: platform.DialogQuestion}
	md.SetTitle("Replace File")
	md.SetMessage(fmt.Sprintf("A file named “%s” already exists. Do you want to replace it?", filepath.Base(path)))
	md.SetDescription(fmt.Sprintf("The file already exists in “%s”. Replacing it will overwrite its contents.",
		filepath.Dir(path)))
	md.AddButton("&Cancel", platform.ResponseCancel, false)
	md.AddButton("&Replace", platform.ResponseYes, true)
	return md.RunModal() == platform.ResponseYes
}

// newFrame is a frame packed as a container, sized by its children.
func newFrame(parent widget.Caregiver, name string) *frame.Frame {
	f := frame.New(parent, name)
	f.Window().ReqWidth, f.Window().ReqHeight = 1, 1
	f.Window().Width, f.Window().Height = 1, 1
	return f
}
