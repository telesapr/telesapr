package agent

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/script/api"
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

// Measuring is what Measure measures.
type Measuring struct {
	// Group is the group to measure the model up to, the active one for
	// "".
	Group string
	// Selected measures what the user has selected instead.
	Selected bool
	// Gaps, more than 0, lists the gaps between parts less than it, in
	// mm, as Analyze > Clearance and Walls does; Within has the gaps
	// between the solids of a part of several, as a linked assembly,
	// listed too.
	Gaps   float64
	Within bool
	// Walls, more than 0, measures the walls of the parts and says
	// where they are thinner than it, in mm.
	Walls float64
}

// Measure is the solid up to m.Group as Check Solid reports it, with
// the gaps and walls m asks about, or what the user has selected,
// measured.
func (s *Session) Measure(_ context.Context, m Measuring) (text string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	doc := s.engine.Doc()
	group, selected := m.Group, m.Selected
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
		if m.Gaps > 0 {
			text += "\n" + gapsText(g, m.Gaps, m.Within)
		}
		if m.Walls > 0 {
			text += "\n" + wallsText(g, m.Walls)
		}
	})
	return text, err
}

// mostListed is how many gaps or thin spots a measure lists: a thin
// wall is thin at every point measured on it.
const mostListed = 20

// gapsText lists the gaps less than below between the parts up to g.
func gapsText(g api.Group, below float64, within bool) string {
	cs := g.Analyze().Clearances(below)
	if within {
		cs = g.Analyze().ClearancesWithin(below)
	}
	if len(cs) == 0 {
		return fmt.Sprintf("No two parts are nearer than %g mm.", below)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Gaps less than %g mm, in mm, 0 where parts touch or overlap:\n", below)
	tol := 0.0
	for i, c := range cs {
		tol = max(tol, c.Tolerance)
		if i == mostListed {
			fmt.Fprintf(&b, "  and %d more\n", len(cs)-i)
			break
		}
		fmt.Fprintf(&b, "  %.6g between %s at %s and %s at %s\n", c.Gap, solidLabel(c.A, c.SolidA), point(c.PA), solidLabel(c.B, c.SolidB), point(c.PB))
	}
	fmt.Fprintf(&b, "Each gap is within %.3g mm of the true one.", tol)
	return b.String()
}

// point is a point as a measure writes it, in mm.
func point(v geom.Vector) string {
	return fmt.Sprintf("(%.6g, %.6g, %.6g)", v.X, v.Y, v.Z)
}

// solidLabel names a part, and which of its solids where they were
// taken apart.
func solidLabel(g api.Group, solid int) string {
	if solid == 0 {
		return g.String()
	}
	return fmt.Sprintf("%s (solid %d)", g.String(), solid)
}

// wallsText says how thick the walls of the parts up to g are, and
// where they are thinner than below.
func wallsText(g api.Group, below float64) string {
	r := g.Analyze().WallThickness(below)
	if r.Samples == r.Unmeasured {
		return "No wall could be measured."
	}
	var b strings.Builder
	t := r.Thinnest
	fmt.Fprintf(&b, "The thinnest wall is %.6g mm, from %s to %s.\n", t.Thickness, point(t.From), point(t.To))
	if len(r.Thin) == 0 {
		fmt.Fprintf(&b, "No wall is thinner than %g mm.\n", below)
	} else {
		fmt.Fprintf(&b, "%d of %d points measured are thinner than %g mm; the thinnest:\n", len(r.Thin), r.Samples, below)
		for _, s := range r.Thin[:min(len(r.Thin), mostListed)] {
			fmt.Fprintf(&b, "  %.6g at %s\n", s.Thickness, point(s.From))
		}
	}
	fmt.Fprintf(&b, "Each is within %.3g mm, measured at points %.3g mm apart or closer.", r.Tolerance, r.Step)
	if r.Unmeasured > 0 {
		fmt.Fprintf(&b, "\n%d points could not be measured: the mesh is open there, or parts stand in each other.", r.Unmeasured)
	}
	return b.String()
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
	var notes []string
	if err := try(func() {
		if strings.EqualFold(filepath.Ext(abs), ".go") {
			notes, xerr = s.engine.Doc().File.ExportScript(abs)
		} else {
			xerr = s.engine.Doc().File.Export(abs)
		}
	}); err != nil {
		return "", err
	}
	if xerr != nil {
		return "", xerr
	}
	fi, err := os.Stat(abs)
	if err != nil {
		return "", err
	}
	text := fmt.Sprintf("wrote %s, %d bytes", abs, fi.Size())
	if len(notes) > 0 {
		// The script says each where it would be, as a comment.
		text += "\nnot written:\n- " + strings.Join(notes, "\n- ")
	}
	return text, nil
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
