package tlspr

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime/trace"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/persist/link"
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/srf"
)

// Extension is the file extension of the format, without its dot.
const Extension = "tlspr"

func init() {
	link.RegisterReporting(Extension, loadLinked)
	link.RegisterSketchReader(Extension, readLinkedSketch)
}

// readLinkedSketch reads a linked file's sketch, for the parts it
// links in turn.
func readLinkedSketch(path string) (*sketch.Sketch, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	doc, err := Decode(data)
	if err != nil {
		return nil, err
	}
	return doc.Sketch, nil
}

// ErrCancelled is LoadFile's error when the user cancelled loading at
// a missing linked file.
var ErrCancelled = link.ErrCancelled

// LoadFile opens a document: it decodes the file, loads what the
// sketch links, asking through link.LocateMissingFile for a missing
// file, and regenerates the sketch without solving. The saved entities
// are regenerated, not loaded. The sketch's ReloadAllLinked reloads its
// links relative to path.
func LoadFile(path string) (*Document, error) {
	defer trace.StartRegion(context.Background(), "load").End()
	if IsPacked(path) {
		return LoadPacked(path)
	}
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, err
	}
	return Load(data, path)
}

// Load is LoadFile for a file's contents, data, with links relative to
// path.
func Load(data []byte, path string) (doc *Document, err error) {
	doc, err = Decode(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	sk := doc.Sketch
	sk.Entities = sk.Entities[:0]
	doc.Mesh, doc.Shell = srf.SMesh{}, srf.SShell{}
	if !link.ReloadAll(sk, path, link.Options{CanCancel: true}) {
		return nil, ErrCancelled
	}
	sk.ReloadAllLinked = func(sk *sketch.Sketch) { link.ReloadAllLinked(sk, path) }
	defer func() {
		// A sketch that breaks regeneration, such as one missing the
		// reference workplanes, is an error.
		if rec := recover(); rec != nil {
			doc, err = nil, fmt.Errorf("%s: can't regenerate: %v", path, rec)
		}
	}()
	sk.UpgradeLegacyData()
	return doc, nil
}

// loadLinked reads the saved entities, mesh and shell of a linked
// .tlspr file, and imports its styles that the sketch doesn't have. It
// reports the model's parameters and configurations, which a link can
// pass values into.
func loadLinked(path string, sk *sketch.Sketch, m *srf.SMesh, sh *srf.SShell, rep *link.Report) ([]core.Entity, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	doc, err := Decode(data)
	if err != nil {
		return nil, false
	}
	for _, st := range doc.Sketch.Styles {
		// Linked file contains a style that we don't have yet, so
		// import it.
		if sk.Style(st.H) == nil {
			sk.Styles = append(sk.Styles, st)
		}
	}
	rep.Model = true
	rep.Parameters = doc.Sketch.Parameters
	rep.Datums = doc.Sketch.Datums
	rep.Threads = doc.Sketch.LinkThreads()
	for _, c := range doc.Sketch.Configurations {
		rep.Configurations = append(rep.Configurations, c.Name)
	}
	m.Triangles = append(m.Triangles, doc.Mesh.Triangles...)
	sh.Surface = append(sh.Surface, doc.Shell.Surface...)
	sh.Curve = append(sh.Curve, doc.Shell.Curve...)
	return append([]core.Entity(nil), doc.Sketch.Entities...), true
}

// SaveFile regenerates nothing: it writes a sketch the caller has just
// regenerated to path, with its links rebased from the file the sketch
// was in (from, or "" for a new sketch) to path. When the write fails
// the links are put back.
func SaveFile(sk *sketch.Sketch, from, path string, o Options) error {
	if IsPacked(path) {
		return Pack(sk, from, path, o)
	}
	if bad, err := link.RebaseLinks(sk, from, path); err != nil {
		return &LinkError{Link: bad, Err: err}
	}
	if err := os.WriteFile(path, Encode(sk, o), 0o644); err != nil {
		// Put the links back relative to where the sketch still is.
		_, _ = link.RebaseLinks(sk, path, from)
		return err
	}
	return nil
}

// LinkError is SaveFile's error for a link that can't be written
// relative to the new file, such as one on another Windows volume.
type LinkError struct {
	Link string
	Err  error
}

func (e *LinkError) Error() string {
	return fmt.Sprintf("the sketch links '%s', which can't be named relative to the file being saved: %v", e.Link, e.Err)
}

func (e *LinkError) Unwrap() error { return e.Err }
