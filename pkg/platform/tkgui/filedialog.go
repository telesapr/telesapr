package tkgui

import (
	"path/filepath"
	"strings"

	"github.com/takigo/takigo/dialog"
	tkplatform "github.com/takigo/takigo/platform"
	"github.com/takigo/takigo/widget"
	"github.com/takigo/takigo/widget/frame"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/platform"
)

// FileDialog asks for a file to open or save. It keeps what it is told
// and shows takigo's chooser.
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

// SetCurrentName sets the name to save under.
func (d *FileDialog) SetCurrentName(s string) {
	if d.save {
		d.name = s
	}
}

// Filename is the file chosen.
func (d *FileDialog) Filename() string { return d.filename }

// SetFilename starts at path.
func (d *FileDialog) SetFilename(path string) {
	d.dir = filepath.Dir(path)
	if d.save {
		d.name = filepath.Base(path)
	}
	d.filename = path
}

// SuggestFilename starts in path's folder, with its name under the
// current filter's extension.
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

// FreezeChoices remembers the folder and the filter.
func (d *FileDialog) FreezeChoices(s core.Settings, key string) {
	if d.filename != "" {
		s.FreezeString("Dialog_"+key+"_Folder", filepath.Dir(d.filename))
	}
	s.FreezeString("Dialog_"+key+"_Filter", d.extension())
}

// ThawChoices starts in the folder and with the filter FreezeChoices
// remembered.
func (d *FileDialog) ThawChoices(s core.Settings, key string) {
	if dir := s.ThawString("Dialog_"+key+"_Folder", ""); dir != "" {
		d.dir = dir
	}
	d.setExtension(s.ThawString("Dialog_"+key+"_Filter", ""))
}

// extension is the current filter's first extension.
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
// first filter.
func (d *FileDialog) setExtension(ext string) {
	d.filter = 0
	for i, f := range d.filters {
		if len(f.Extensions) > 0 && f.Extensions[0] == ext {
			d.filter = i
			return
		}
	}
}

// fileStem is path's file name without its extension.
func fileStem(path string) string {
	base := filepath.Base(path)
	if dot := strings.LastIndexByte(base, '.'); dot > 0 {
		return base[:dot]
	}
	return base
}

// RunModal shows takigo's file chooser; false if it was cancelled.
// It starts with the current filter and, as GTK's save dialog does,
// adds that filter's extension to a name typed without one and asks
// before an existing file is replaced.
func (d *FileDialog) RunModal() bool {
	if d.filter < 0 || d.filter >= len(d.filters) {
		d.filter = 0
	}
	opts := []dialog.FileOption{dialog.FileTitle(prepareTitle(d.title))}
	if d.dir != "" {
		opts = append(opts, dialog.FileInitialDir(d.dir))
	}
	if d.save && d.name != "" {
		opts = append(opts, dialog.FileInitialFile(d.name))
	}
	if d.save && d.extension() != "" {
		opts = append(opts, dialog.FileDefaultExtension("."+d.extension()))
	}
	// The chooser starts on its first type and has no way to start on
	// another, so the current filter goes first.
	var types []dialog.FileType
	for i := range d.filters {
		f := d.filters[(d.filter+i)%len(d.filters)]
		types = append(types, dialog.FileType{Name: f.Name, Pattern: filterPattern(f)})
	}
	if len(types) > 0 {
		opts = append(opts, dialog.FileTypes(types...))
	}

	var path string
	var ok bool
	if d.save {
		path, ok = dialog.SaveFile(d.parent, opts...)
	} else {
		path, ok = dialog.OpenFile(d.parent, opts...)
	}
	if !ok {
		return false
	}
	d.filename = path
	// Nor does it say which type was chosen: take the one the file's
	// extension belongs to, for FreezeChoices.
	ext := strings.TrimPrefix(filepath.Ext(path), ".")
	for i, f := range d.filters {
		for _, e := range f.Extensions {
			if e != "" && strings.EqualFold(e, ext) {
				d.filter = i
				return true
			}
		}
	}
	return true
}

// filterPattern is f's extensions as the chooser's glob patterns, each
// also in upper case.
func filterPattern(f platform.FileFilter) string {
	var pats []string
	for _, e := range f.Extensions {
		if e == "" {
			return "*"
		}
		pats = append(pats, "*."+e)
		if up := strings.ToUpper(e); up != e {
			pats = append(pats, "*."+up)
		}
	}
	if len(pats) == 0 {
		return "*"
	}
	return strings.Join(pats, " ")
}

// xkKPEnter is the keypad's Enter.
const xkKPEnter = tkplatform.KeySym(0xff8d)

// newFrame is a frame packed as a container, sized by its children.
func newFrame(parent widget.Caregiver, name string) *frame.Frame {
	f := frame.New(parent, name)
	f.Window().ReqWidth, f.Window().ReqHeight = 1, 1
	f.Window().Width, f.Window().Height = 1, 1
	return f
}
