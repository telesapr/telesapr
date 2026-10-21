package agent

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/telesapr/telesapr/pkg/sketch"
)

// Listing is a directory as Files lists it.
type Listing struct {
	// Dir is the directory listed, as an absolute path.
	Dir string `json:"dir"`
	// Entries are its files and directories by name, but for those
	// whose names start with a dot, at most MaxEntries of them.
	Entries []Entry `json:"entries"`
	// More is how many entries were left out past MaxEntries.
	More int `json:"more,omitempty"`
}

// Entry is a file or a directory of a Listing.
type Entry struct {
	Name string `json:"name"`
	Dir  bool   `json:"dir,omitempty"`
	// Size is a file's length in bytes.
	Size int64 `json:"size,omitempty"`
}

// ErrUnsaved is Open's error for a drawing with changes that aren't
// saved: it is not replaced.
var ErrUnsaved = errors.New("the drawing open in TeleSAPR has changes that aren't saved; ask the user to save it, or to let the changes go, before opening another")

// MaxEntries is the most entries a Listing holds: a directory of
// thousands of files would fill a model's context for nothing.
const MaxEntries = 500

// Measure is the solid up to group as Check Solid reports it, or what
// the user has selected, measured.
func (s *Session) Measure(_ context.Context, group string, selected bool) (text string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	doc := s.engine.Doc()
	err = try(func() {
		if selected {
			es := doc.Edit.Selection()
			if len(es) == 0 {
				panic(errors.New("nothing is selected in the drawing; ask the user to select what to measure, or measure without selected"))
			}
			m := doc.Analyze.Measure(es...)
			unit := "mm"
			if m.Kind == sketch.MeasureAngle {
				unit = "degrees"
			}
			text = fmt.Sprintf("%s: %.6g %s", m.What, m.Value, unit)
			return
		}
		g := doc.Active()
		if group != "" {
			g = groupNamed(doc, group)
		}
		r := g.Analyze().Check()
		text = fmt.Sprintf("the model up to %s:\n%s", g.String(), r.Summary())
	})
	return text, err
}

// Export writes the drawing to a file.
func (s *Session) Export(_ context.Context, path string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	var xerr error
	if err := try(func() { xerr = s.engine.Doc().File.Export(abs) }); err != nil {
		return "", err
	}
	if xerr != nil {
		return "", xerr
	}
	fi, err := os.Stat(abs)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("wrote %s, %d bytes", abs, fi.Size()), nil
}

// Open replaces the drawing by a file's.
func (s *Session) Open(_ context.Context, path string) (text string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	doc := s.engine.Doc()
	var lerr error
	if err := try(func() {
		// A script's File.Load replaces the drawing as it is; a program
		// that isn't the user must not lose the user's work.
		if doc.File.Unsaved() {
			panic(ErrUnsaved)
		}
		lerr = doc.File.Load(abs)
	}); err != nil {
		return "", err
	}
	if lerr != nil {
		return "", lerr
	}
	err = try(func() { text = "opened " + abs + "\n" + doc.Describe() })
	return text, err
}

// Files lists a directory, one the session's scripts may read when
// their files are guarded.
func (s *Session) Files(_ context.Context, dir string) (Listing, error) {
	if guard := s.engine.Files(); guard != nil {
		if dir == "" {
			dir = "."
		}
		if err := guard.Check(dir, false); err != nil {
			return Listing{}, fmt.Errorf("listing %s: %w", dir, err)
		}
	}
	return ListFiles(dir)
}

// ListFiles is what any Backend's Files gives on this machine.
func ListFiles(dir string) (Listing, error) {
	if dir == "" {
		dir = "."
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return Listing{}, err
	}
	des, err := os.ReadDir(abs)
	if err != nil {
		return Listing{}, fmt.Errorf("listing %s: %w", abs, err)
	}
	l := Listing{Dir: abs, Entries: []Entry{}}
	for _, de := range des {
		if strings.HasPrefix(de.Name(), ".") {
			continue
		}
		if len(l.Entries) == MaxEntries {
			l.More++
			continue
		}
		e := Entry{Name: de.Name(), Dir: de.IsDir()}
		if !e.Dir {
			if fi, err := de.Info(); err == nil {
				e.Size = fi.Size()
			}
		}
		l.Entries = append(l.Entries, e)
	}
	return l, nil
}
