package script

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// A script library is a folder of Go packages that scripts import by
// their path in it: "gears" is <library>/gears, "shop/brackets" is
// <library>/shop/brackets. The interpreter reads them from source, as
// it would from a GOPATH, but without the src level a GOPATH has, so
// that the folder holds nothing but the packages.

// DefaultLibrary is the script library used when none is set:
// telesapr/library under the user's configuration directory, beside
// the settings. It is "" where no such directory is known.
func DefaultLibrary() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "telesapr", "library")
}

// scriptTag is a build tag the interpreter has and the go tool
// doesn't: a script that imports from the library, which the go tool
// can't find, carries "//go:build telesapr" to be left out of go build
// and go vet, and still runs here.
const scriptTag = "telesapr"

// libraryRoot is what the interpreter is told GOPATH is: it looks for
// a package under libraryRoot/src/<import path>, a path libraryFS
// takes into the library.
const libraryRoot = "."

// libraryFS is the source the interpreter imports packages from: the
// engine's library, as it is at each read.
type libraryFS struct{ e *Engine }

func (l libraryFS) Open(name string) (fs.File, error) {
	rest, ok := strings.CutPrefix(name, "src/")
	dir := l.e.Library()
	if !ok || dir == "" {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}
	return os.DirFS(dir).Open(rest)
}

// SetLibrary sets the folder packages are imported from, "" for none;
// a package already imported stays as it was.
func (e *Engine) SetLibrary(dir string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.opts.Library = dir
}

// Library is the folder packages are imported from, "" for none.
func (e *Engine) Library() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.opts.Library
}

// notFound is the interpreter's error for an import it has no package
// for.
var notFound = regexp.MustCompile(`unable to find source related to: "([^"]*)"`)

// errNoPackage is what an import of a package that is neither one of
// the program's nor in the library says.
var errNoPackage = errors.New("no such package")

// importError says what an import the interpreter couldn't find is
// missing from, or returns err as it was.
func (e *Engine) importError(err error) error {
	m := notFound.FindStringSubmatch(err.Error())
	if m == nil {
		return err
	}
	if dir := e.Library(); dir != "" {
		return fmt.Errorf("%w %q: it is not one scripts have, nor in the script library %s", errNoPackage, m[1], dir)
	}
	return fmt.Errorf("%w %q: it is not one scripts have, and no script library is set", errNoPackage, m[1])
}
