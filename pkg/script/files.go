package script

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Files says which files a script may read and write. An engine with
// Files guards every way a script has of naming a file: the functions
// of os and path/filepath that take a path, the calls of package api
// that read or write a file (api.FileGuard), and the packages a script
// could reach files through unguarded, which it doesn't get: sketch,
// persist/tlspr and persist/convert, and the api functions that make a
// document of their own. It takes from a script what would let it past
// the guard or change the program around it: os.Chdir, the
// environment, processes and file descriptors. An engine without
// Files lets a script use any file its user may.
//
// A path is taken as the system would take it: made absolute against
// the working directory, with the symbolic links of the part that
// exists followed, so a link inside an allowed folder to a file outside
// it is outside. A path with a ".." element is refused, since what it
// names depends on the links before it.
//
// What the guard doesn't see: the files a model links (link groups,
// images) are read where they are when it regenerates, and a file
// opened in an allowed folder stays open as long as the script keeps
// it.
type Files struct {
	// Read are folders whose files a script may read.
	Read []string
	// Write are folders whose files a script may read and write.
	Write []string
	// Folder, if set, gives another folder a script may read and
	// write, asked at each check: the folder of the document, which
	// changes when another is opened. "" for none.
	Folder func() string
	// Temp, if set, is a folder of the script's own that it may read
	// and write, where os.CreateTemp and os.MkdirTemp put what they
	// make when given no folder.
	Temp string
}

// ErrNotAllowed is the error of a file a script whose files are
// guarded may not use, or of what such a script may not do at all.
var ErrNotAllowed = errors.New("not allowed")

// folders are the folders a script may read in, and those it may write
// in, absolute and with their links followed.
func (f *Files) folders() (read, write []string) {
	add := func(list []string, dir string) []string {
		if dir == "" {
			return list
		}
		if real, err := resolve(dir); err == nil {
			return append(list, real)
		}
		return list
	}
	for _, d := range f.Write {
		write = add(write, d)
	}
	if f.Folder != nil {
		write = add(write, f.Folder())
	}
	write = add(write, f.Temp)
	read = slices.Clone(write)
	for _, d := range f.Read {
		read = add(read, d)
	}
	return read, write
}

// Check returns an error if a script may not read the file at path,
// or, with write, write it.
func (f *Files) Check(path string, write bool) error {
	if hasDotDot(path) {
		return fmt.Errorf("%s: %w: a guarded script names files without ..", path, ErrNotAllowed)
	}
	real, err := resolve(path)
	if err != nil {
		return fmt.Errorf("%s: %w: %v", path, ErrNotAllowed, err)
	}
	read, writable := f.folders()
	allowed := read
	if write {
		allowed = writable
	}
	if slices.ContainsFunc(allowed, func(dir string) bool { return within(real, dir) }) {
		return nil
	}
	return fmt.Errorf("%s: %w: %s", path, ErrNotAllowed, f.describe(write))
}

// describe says which folders a script may use, to say why a file was
// refused.
func (f *Files) describe(write bool) string {
	read, writable := f.folders()
	if write {
		if len(writable) == 0 {
			return "a guarded script writes no files here"
		}
		return "a guarded script writes files only in " + strings.Join(writable, ", ")
	}
	if len(read) == 0 {
		return "a guarded script reads no files here"
	}
	return "a guarded script reads files only in " + strings.Join(read, ", ")
}

// hasDotDot reports whether path has a ".." element.
func hasDotDot(path string) bool {
	return slices.Contains(strings.Split(filepath.ToSlash(path), "/"), "..")
}

// resolve is path made absolute, with the links of its longest part
// that exists followed; the rest doesn't exist, so has no links.
func resolve(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	head, tail := abs, ""
	for {
		if _, err := os.Lstat(head); err == nil {
			break
		}
		parent := filepath.Dir(head)
		if parent == head {
			return abs, nil
		}
		tail = filepath.Join(filepath.Base(head), tail)
		head = parent
	}
	real, err := filepath.EvalSymlinks(head)
	if err != nil {
		return "", err
	}
	return filepath.Join(real, tail), nil
}

// within reports whether path is dir or in it.
func within(path, dir string) bool {
	rel, err := filepath.Rel(dir, path)
	return err == nil && !filepath.IsAbs(rel) && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// guardedFS is a folder as os.DirFS gives it, each file checked as it
// is opened, so that a link in it can't lead out.
type guardedFS struct {
	f   *Files
	dir string
}

func (g guardedFS) Open(name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
	}
	if err := g.f.Check(filepath.Join(g.dir, filepath.FromSlash(name)), false); err != nil {
		return nil, &fs.PathError{Op: "open", Path: name, Err: err}
	}
	return os.DirFS(g.dir).Open(name)
}
