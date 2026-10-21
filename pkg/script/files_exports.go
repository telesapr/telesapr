package script

import (
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/telesapr/telesapr/pkg/check"
	"github.com/telesapr/telesapr/pkg/script/api"
)

// This file is what a script whose files are guarded (Files) imports
// in place of the generated tables: os, path/filepath and package
// check with what takes a path checked, package api with the documents
// it makes guarded, and without the packages that would reach files
// past the guard.

// unguardedPackages are packages a guarded script doesn't get: their
// calls read and write files the guard can't see. Package api is how
// a script works on a document.
var unguardedPackages = []string{
	"github.com/telesapr/telesapr/pkg/sketch/sketch",
	"github.com/telesapr/telesapr/pkg/persist/tlspr/tlspr",
	"github.com/telesapr/telesapr/pkg/persist/convert/convert",
}

// unguardedAPI are what package api has that makes a document whose
// host is not the guarded one, or hands out a host.
var unguardedAPI = []string{"NewDoc", "NewSketchHost", "SetCurrent", "Import", "SketchHost"}

// errRefused is what a guarded script gets for what it may not do at
// all.
func errRefused(what string) error {
	return fmt.Errorf("%s: %w to a script whose files are guarded", what, ErrNotAllowed)
}

// guardedHost is a document's host for a script whose files are
// guarded: package api asks it before it reads or writes a file.
type guardedHost struct {
	api.Host
	files *Files
}

func (h guardedHost) CheckFile(path string, write bool) error { return h.files.Check(path, write) }

// ForgetChanges passes File.ForgetChanges on to the host it guards,
// which package api would find on that host itself.
func (h guardedHost) ForgetChanges() {
	if f, ok := h.Host.(interface{ ForgetChanges() }); ok {
		f.ForgetChanges()
		return
	}
	h.Sketch().Unsaved = false
}

var _ api.FileGuard = guardedHost{}

// guard puts a host behind the engine's guard, if it has one.
func (e *Engine) guard(h api.Host) api.Host {
	if e.opts.Files == nil || h == nil {
		return h
	}
	return guardedHost{h, e.opts.Files}
}

// guardExports replaces, in what a guarded script imports, what
// reaches files past the guard.
func (e *Engine) guardExports(exports map[string]map[string]reflect.Value) {
	files := e.opts.Files
	if files == nil {
		return
	}
	for _, path := range unguardedPackages {
		delete(exports, path)
	}

	key := apiPath + "/api"
	pkg := exports[key]
	for _, name := range unguardedAPI {
		delete(pkg, name)
	}
	pkg["Open"] = reflect.ValueOf(func(path string) (*api.Doc, error) {
		if err := files.Check(path, false); err != nil {
			return nil, err
		}
		if e.opts.Open != nil {
			h, err := e.opts.Open(path)
			if err != nil {
				return nil, err
			}
			return api.NewDoc(e.guard(h)), nil
		}
		d, err := api.Open(path)
		if err != nil {
			return nil, err
		}
		return api.NewDoc(e.guard(d.Host())), nil
	})
	pkg["New"] = reflect.ValueOf(func() *api.Doc {
		if e.opts.New != nil {
			return api.NewDoc(e.guard(e.opts.New()))
		}
		return api.NewDoc(e.guard(api.New().Host()))
	})

	own(exports, "github.com/telesapr/telesapr/pkg/check/check")["ReadBaseline"] = reflect.ValueOf(
		func(path string) (map[string]string, []string, error) {
			if err := files.Check(path, false); err != nil {
				return nil, nil, err
			}
			return check.ReadBaseline(path)
		})

	std := own(exports, "path/filepath/filepath")
	for name, fn := range guardedFilepath(files) {
		std[name] = reflect.ValueOf(fn)
	}
	std = own(exports, "os/os")
	for name, fn := range guardedOS(files) {
		std[name] = reflect.ValueOf(fn)
	}
}

// own makes the table of a package in exports a copy of its own, to
// change: the generated tables are every engine's.
func own(exports map[string]map[string]reflect.Value, path string) map[string]reflect.Value {
	pkg := maps.Clone(exports[path])
	exports[path] = pkg
	return pkg
}

// guardedFilepath is what package path/filepath has that reads the
// file system, checked.
func guardedFilepath(files *Files) map[string]any {
	return map[string]any{
		"EvalSymlinks": func(path string) (string, error) {
			if err := files.Check(path, false); err != nil {
				return "", err
			}
			return filepath.EvalSymlinks(path)
		},
		"Glob": func(pattern string) ([]string, error) {
			// The folder before the first wildcard is what is read.
			static := pattern
			if i := strings.IndexAny(pattern, `*?[\`); i >= 0 {
				static = pattern[:i]
			}
			if err := files.Check(filepath.Dir(static+"x"), false); err != nil {
				return nil, err
			}
			return filepath.Glob(pattern)
		},
		"Walk": func(root string, fn filepath.WalkFunc) error {
			if err := files.Check(root, false); err != nil {
				return err
			}
			return filepath.Walk(root, fn)
		},
		"WalkDir": func(root string, fn fs.WalkDirFunc) error {
			if err := files.Check(root, false); err != nil {
				return err
			}
			return filepath.WalkDir(root, fn)
		},
	}
}

// writes reports whether flags of os.OpenFile open a file to change it.
func writes(flag int) bool {
	return flag&(os.O_WRONLY|os.O_RDWR|os.O_CREATE|os.O_TRUNC|os.O_APPEND) != 0
}

// guardedOS is what package os has that names files, checked, and
// what a guarded script may not do at all, refused.
func guardedOS(files *Files) map[string]any {
	r := func(path string) error { return files.Check(path, false) }
	w := func(path string) error { return files.Check(path, true) }
	tempDir := func(dir string) string {
		switch {
		case dir != "":
			return dir
		case files.Temp != "":
			return files.Temp
		}
		return os.TempDir()
	}
	return map[string]any{
		// Reading.
		"Lstat": func(name string) (os.FileInfo, error) {
			if err := r(name); err != nil {
				return nil, err
			}
			return os.Lstat(name)
		},
		"Stat": func(name string) (os.FileInfo, error) {
			if err := r(name); err != nil {
				return nil, err
			}
			return os.Stat(name)
		},
		"Open": func(name string) (*os.File, error) {
			if err := r(name); err != nil {
				return nil, err
			}
			return os.Open(name)
		},
		"ReadDir": func(name string) ([]os.DirEntry, error) {
			if err := r(name); err != nil {
				return nil, err
			}
			return os.ReadDir(name)
		},
		"ReadFile": func(name string) ([]byte, error) {
			if err := r(name); err != nil {
				return nil, err
			}
			return os.ReadFile(name)
		},
		"Readlink": func(name string) (string, error) {
			if err := r(name); err != nil {
				return "", err
			}
			return os.Readlink(name)
		},
		"DirFS": func(dir string) fs.FS { return guardedFS{files, dir} },
		"OpenInRoot": func(dir, name string) (*os.File, error) {
			if err := r(dir); err != nil {
				return nil, err
			}
			return os.OpenInRoot(dir, name)
		},

		// Writing.
		"OpenFile": func(name string, flag int, perm os.FileMode) (*os.File, error) {
			if err := files.Check(name, writes(flag)); err != nil {
				return nil, err
			}
			return os.OpenFile(name, flag, perm)
		},
		"OpenRoot": func(dir string) (*os.Root, error) {
			// A root writes what is in it.
			if err := w(dir); err != nil {
				return nil, err
			}
			return os.OpenRoot(dir)
		},
		"Create": func(name string) (*os.File, error) {
			if err := w(name); err != nil {
				return nil, err
			}
			return os.Create(name)
		},
		"CreateTemp": func(dir, pattern string) (*os.File, error) {
			dir = tempDir(dir)
			if err := w(dir); err != nil {
				return nil, err
			}
			return os.CreateTemp(dir, pattern)
		},
		"MkdirTemp": func(dir, pattern string) (string, error) {
			dir = tempDir(dir)
			if err := w(dir); err != nil {
				return "", err
			}
			return os.MkdirTemp(dir, pattern)
		},
		"WriteFile": func(name string, data []byte, perm os.FileMode) error {
			if err := w(name); err != nil {
				return err
			}
			return os.WriteFile(name, data, perm)
		},
		"Mkdir": func(name string, perm os.FileMode) error {
			if err := w(name); err != nil {
				return err
			}
			return os.Mkdir(name, perm)
		},
		"MkdirAll": func(name string, perm os.FileMode) error {
			if err := w(name); err != nil {
				return err
			}
			return os.MkdirAll(name, perm)
		},
		"Remove": func(name string) error {
			if err := w(name); err != nil {
				return err
			}
			return os.Remove(name)
		},
		"RemoveAll": func(name string) error {
			if err := w(name); err != nil {
				return err
			}
			return os.RemoveAll(name)
		},
		"Rename": func(oldpath, newpath string) error {
			if err := errors.Join(w(oldpath), w(newpath)); err != nil {
				return err
			}
			return os.Rename(oldpath, newpath)
		},
		"Link": func(oldname, newname string) error {
			if err := errors.Join(w(oldname), w(newname)); err != nil {
				return err
			}
			return os.Link(oldname, newname)
		},
		"Symlink": func(oldname, newname string) error {
			// The link's target is resolved from the link's folder.
			target := oldname
			if !filepath.IsAbs(target) {
				target = filepath.Join(filepath.Dir(newname), target)
			}
			if err := errors.Join(r(target), w(newname)); err != nil {
				return err
			}
			return os.Symlink(oldname, newname)
		},
		"Truncate": func(name string, size int64) error {
			if err := w(name); err != nil {
				return err
			}
			return os.Truncate(name, size)
		},
		"Chmod": func(name string, mode os.FileMode) error {
			if err := w(name); err != nil {
				return err
			}
			return os.Chmod(name, mode)
		},
		"Chown": func(name string, uid, gid int) error {
			if err := w(name); err != nil {
				return err
			}
			return os.Chown(name, uid, gid)
		},
		"Lchown": func(name string, uid, gid int) error {
			if err := w(name); err != nil {
				return err
			}
			return os.Lchown(name, uid, gid)
		},
		"Chtimes": func(name string, atime, mtime time.Time) error {
			if err := w(name); err != nil {
				return err
			}
			return os.Chtimes(name, atime, mtime)
		},
		"CopyFS": func(dir string, fsys fs.FS) error {
			if err := w(dir); err != nil {
				return err
			}
			return os.CopyFS(dir, fsys)
		},

		// What changes the program around the script, or reaches past
		// the guard: refused. The environment is the interpreter's own,
		// which a guarded script gets empty (newInterpreter).
		"Chdir":        func(string) error { return errRefused("os.Chdir") },
		"NewFile":      func(uintptr, string) *os.File { return nil },
		"FindProcess":  func(int) (*os.Process, error) { return nil, errRefused("os.FindProcess") },
		"StartProcess": func(string, []string, *os.ProcAttr) (*os.Process, error) { return nil, errRefused("os.StartProcess") },
	}
}
