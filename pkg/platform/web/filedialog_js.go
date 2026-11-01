//go:build js && wasm

package web

import (
	"path"
	"strings"
	"syscall/js"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/platform"
)

// FileDialog asks for a file to open or save. A browser has no file
// system to browse: opening takes a file the user picks (or drops) and
// puts it in the in-memory file system, saving takes a name there and
// the file is offered as a download once the application has written it.
type FileDialog struct {
	p      *Platform
	save   bool
	title  string
	dir    string
	name   string
	filter int
	// filters are the file types; filename is the answer.
	filters  []platform.FileFilter
	filename string
}

var _ platform.FileDialog = (*FileDialog)(nil)

// CreateOpenFileDialog makes a dialog asking for a file to open.
func (p *Platform) CreateOpenFileDialog(platform.Window) platform.FileDialog {
	return &FileDialog{p: p, title: "Open File"}
}

// CreateSaveFileDialog makes a dialog asking where to save.
func (p *Platform) CreateSaveFileDialog(platform.Window) platform.FileDialog {
	return &FileDialog{p: p, save: true, title: "Save File"}
}

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
func (d *FileDialog) SetFilename(p string) {
	d.dir = folderOrHome(p)
	if d.save {
		d.name = path.Base(p)
	}
	d.filename = p
}

// SuggestFilename starts in the folder of path, with its name under the
// current filter's extension.
func (d *FileDialog) SuggestFilename(p string) {
	if !d.save {
		return
	}
	d.dir = folderOrHome(p)
	d.name = stem(p)
	if ext := firstExtension(d.filters, d.filter); ext != "" {
		d.name += "." + ext
	}
}

func (d *FileDialog) AddFilter(f platform.FileFilter)     { d.filters = append(d.filters, f) }
func (d *FileDialog) AddFilters(fs []platform.FileFilter) { d.filters = append(d.filters, fs...) }

// FreezeChoices remembers the filter.
func (d *FileDialog) FreezeChoices(s core.Settings, key string) {
	s.FreezeString("Dialog_"+key+"_Filter", firstExtension(d.filters, d.filter))
}

// ThawChoices picks the filter FreezeChoices remembered. The folder is
// not kept: it is the browser's home folder.
func (d *FileDialog) ThawChoices(s core.Settings, key string) {
	ext := s.ThawString("Dialog_"+key+"_Filter", "")
	d.filter = 0
	for i, f := range d.filters {
		if len(f.Extensions) > 0 && f.Extensions[0] == ext {
			d.filter = i
			return
		}
	}
}

// RunModal shows the dialog and runs the event queue until it is
// answered; false if it was cancelled.
func (d *FileDialog) RunModal() bool {
	if d.filter < 0 || d.filter >= len(d.filters) {
		d.filter = 0
	}
	if d.save {
		return d.runSave()
	}
	return d.runOpen()
}

func (d *FileDialog) runOpen() bool {
	spec := jsonString(map[string]any{"title": d.title, "accept": acceptAttribute(d.filters)})
	var paths []string
	var fns funcSet
	defer fns.release()
	d.p.runModal(func(end func(func())) {
		d.p.w.Call("openFileDialog", spec, fns.add(func(a []js.Value) any {
			text := a[0].String()
			end(func() {
				if text != "" {
					_ = jsonUnmarshal(text, &paths)
				}
			})
			return nil
		}))
	})
	if len(paths) == 0 {
		return false
	}
	d.filename = paths[0]
	d.pickFilter(d.filename)
	return true
}

func (d *FileDialog) runSave() bool {
	type filter struct {
		Name string `json:"name"`
		Ext  string `json:"ext"`
	}
	var fs []filter
	for _, f := range d.filters {
		fs = append(fs, filter{f.Name, firstExtension([]platform.FileFilter{f}, 0)})
	}
	name := d.name
	if name == "" {
		name = withExtension("untitled", firstExtension(d.filters, d.filter))
	}
	spec := jsonString(map[string]any{"title": d.title, "name": name, "filters": fs, "filter": d.filter})
	var answer struct {
		Name   string `json:"name"`
		Filter int    `json:"filter"`
	}
	var fns funcSet
	defer fns.release()
	d.p.runModal(func(end func(func())) {
		d.p.w.Call("saveFileDialog", spec, fns.add(func(a []js.Value) any {
			text := a[0].String()
			end(func() {
				if text != "" {
					_ = jsonUnmarshal(text, &answer)
				}
			})
			return nil
		}))
	})
	if answer.Name == "" {
		return false
	}
	if answer.Filter >= 0 && answer.Filter < len(d.filters) {
		d.filter = answer.Filter
	}
	dir := d.dir
	if dir == "" {
		dir = homeDir
	}
	file := withExtension(strings.TrimSpace(answer.Name), firstExtension(d.filters, d.filter))
	d.filename = path.Join(dir, path.Base(file))
	d.pickFilter(d.filename)
	d.p.offerDownload(d.filename)
	return true
}

// pickFilter takes the filter the extension of p belongs to, for
// FreezeChoices.
func (d *FileDialog) pickFilter(p string) {
	ext := strings.TrimPrefix(path.Ext(p), ".")
	for i, f := range d.filters {
		for _, e := range f.Extensions {
			if e != "" && strings.EqualFold(e, ext) {
				d.filter = i
				return
			}
		}
	}
}
