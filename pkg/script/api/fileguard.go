package api

import "errors"

// FileGuard is a Host that says which files a script may use: the
// engine of a script whose files are guarded (script.Files) gives its
// documents such a host, and every call of this package that reads or
// writes a file the script names asks it first.
type FileGuard interface {
	// CheckFile returns an error if the script may not read the file
	// at path, or, with write, write it.
	CheckFile(path string, write bool) error
}

// errGuardedHost is what Host says to a script whose files are
// guarded: the host would let it past the guard.
var errGuardedHost = errors.New("the program around the drawing is not available to a script whose files are guarded")

// checkFile asks the host whether the script may read the file at
// path, or, with write, write it; a host that guards nothing allows
// all.
func (d *Doc) checkFile(path string, write bool) error {
	if g, ok := d.h.(FileGuard); ok {
		return g.CheckFile(path, write)
	}
	return nil
}
