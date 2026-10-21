package api

import (
	"errors"
	"math"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/sketch"
)

// saveTile saves a tile 40 x 30 x 10 whose top face is the datum top
// and one of its top corners the datum corner, as tile.tlspr in dir.
func saveTile(t *testing.T, dir string) string {
	t.Helper()
	p := New()
	p.Sketch.Rect(V(0, 0, 0), V(40, 30, 0))
	tile := p.Features.Extrude(10, "tile")
	top := tile.Solid().FaceAt(V(20, 15, 10)).Entity()
	p.Datums.Name("top", top)
	var corner Entity
	for _, e := range tile.Entities() {
		if e.Kind() == "point" && e.Pos() == V(40, 30, 10) {
			corner = e
		}
	}
	p.Datums.Name("corner", corner)
	path := filepath.Join(dir, "tile.tlspr")
	if err := p.File.Save(path); err != nil {
		t.Fatal(err)
	}
	return path
}

// A part's datums come through a link by name, as the link places
// them, and a mate takes them: two tiles made flush by their tops.
func TestDatumsThroughLinks(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	tile := saveTile(t, dir)
	d := New()
	if _, err := d.Assembly.Link(tile, "a"); err != nil {
		t.Fatal(err)
	}
	b, err := d.Assembly.Link(tile, "b")
	if err != nil {
		t.Fatal(err)
	}
	b.AsLink().Move(V(0, 0, 25))
	if got := d.Datums.Names(); !slices.Equal(got, []string{"a.top", "a.corner", "b.top", "b.corner"}) {
		t.Fatalf("names %q", got)
	}
	if d.Datums.Get("a.top").Kind() != d.Datums.Get("b.top").Kind() || !d.Datums.Get("a.top").Valid() {
		t.Fatal("the tops are not faces alike")
	}
	if za, zb := d.Datums.Get("a.corner").Pos().Z, d.Datums.Get("b.corner").Pos().Z; math.Abs(za-zb) < 1 {
		t.Fatalf("corners at %g and %g before the mate", za, zb)
	}
	d.Assembly.Flush(d.Datums.Get("a.top"), d.Datums.Get("b.top"))
	if za, zb := d.Datums.Get("a.corner").Pos().Z, d.Datums.Get("b.corner").Pos().Z; math.Abs(za-zb) > 1e-6 {
		t.Errorf("corners at %g and %g after the mate", za, zb)
	}

	for name, want := range map[string]string{
		"c.top":    "no link called c",
		"a.bottom": "has no datum bottom",
		"bottom":   "no datum",
	} {
		func() {
			defer func() {
				err, _ := recover().(error)
				if err == nil || !strings.Contains(err.Error(), want) || !errors.Is(err, sketch.ErrNoDatum) {
					t.Errorf("%s: %v", name, err)
				}
			}()
			d.Datums.Get(name)
		}()
	}
}

// A datum is saved, named again, taken away, refused for what can't be
// one, and each change is one undo step.
func TestDatumEdits(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	tile := saveTile(t, dir)
	p := New()
	if err := p.File.Load(tile); err != nil {
		t.Fatal(err)
	}
	if got := p.Datums.List(); len(got) != 2 || got[0].Name != "top" || got[1].Name != "corner" {
		t.Fatalf("loaded %v", got)
	}
	step := p.Host().(*SketchHost).BeginEval
	step()
	corner := p.Datums.Get("corner")
	p.Datums.Name("tip", corner)
	if got := p.Datums.Names(); !slices.Equal(got, []string{"top", "tip"}) {
		t.Errorf("renamed: %q", got)
	}
	step()
	p.Datums.Delete("top")
	if got := p.Datums.Names(); !slices.Equal(got, []string{"tip"}) {
		t.Errorf("deleted: %q", got)
	}
	if !p.Edit.Undo() {
		t.Fatal("no undo")
	}
	if got := p.Datums.Names(); !slices.Equal(got, []string{"top", "tip"}) {
		t.Errorf("undone: %q", got)
	}
	if !p.Edit.Redo() {
		t.Fatal("no redo")
	}
	if got := p.Datums.Names(); !slices.Equal(got, []string{"tip"}) {
		t.Errorf("redone: %q", got)
	}

	style := p.Groups()[0]
	for name, fn := range map[string]func(){
		"bad name":    func() { p.Datums.Name("2nd", corner) },
		"delete none": func() { p.Datums.Delete("none") },
		"no entity":   func() { p.Datums.Name("g", Entity{d: p, H: style.H.Entity(0x7fff)}) },
	} {
		func() {
			defer func() {
				if _, ok := recover().(error); !ok {
					t.Errorf("%s: no error", name)
				}
			}()
			fn()
		}()
	}
}
