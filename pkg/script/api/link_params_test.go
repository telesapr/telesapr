package api

import (
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// savePlate saves a plate of L x 30 x 10 mm, L a parameter of 40, with
// configurations short (L 40) and long (L 80), as plate.tlspr in dir.
func savePlate(t *testing.T, dir string) string {
	t.Helper()
	return savePlateDepth(t, dir, 10)
}

// savePlateDepth is savePlate with a plate depth mm thick.
func savePlateDepth(t *testing.T, dir string, depth float64) string {
	t.Helper()
	p := New()
	if err := p.Params.Set("L", "40"); err != nil {
		t.Fatal(err)
	}
	sides := p.Sketch.Rect(V(0, 0, 0), V(40, 30, 0))
	if err := p.Constrain.Length(sides[0], 40).SetExpr("L"); err != nil {
		t.Fatal(err)
	}
	p.Constrain.Length(sides[1], 30)
	p.Features.Extrude(depth, "plate")
	for _, c := range []struct {
		name string
		l    string
	}{{"short", "40"}, {"long", "80"}} {
		if err := p.Params.AddConfiguration(c.name); err != nil {
			t.Fatal(err)
		}
		if err := p.Params.Set("L", c.l); err != nil {
			t.Fatal(err)
		}
	}
	if err := p.Params.ActivateConfiguration("short"); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "plate.tlspr")
	if err := p.File.Save(path); err != nil {
		t.Fatal(err)
	}
	return path
}

// partVolumes are the volumes of the parts of the model, by group.
func partVolumes(d *Doc) []float64 {
	var vs []float64
	groups := d.Groups()
	for _, p := range groups[len(groups)-1].Analyze().MassProperties().Parts {
		vs = append(vs, math.Round(p.Volume))
	}
	return vs
}

// One plate linked twice is two sizes: one link regenerates it in its
// long configuration, the other passes its length from the assembly's
// own parameter, and follows that parameter.
func TestParametricLink(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	plate := savePlate(t, dir)
	d := New()
	if err := d.Params.Set("W", "50"); err != nil {
		t.Fatal(err)
	}
	a, err := d.Assembly.Link(plate, "a")
	if err != nil {
		t.Fatal(err)
	}
	b, err := d.Assembly.Link(plate, "b")
	if err != nil {
		t.Fatal(err)
	}
	if got := a.AsLink().ModelParams(); got["L"] != 40 || len(got) != 1 {
		t.Errorf("model parameters %v", got)
	}
	if got := a.AsLink().ModelConfigurations(); !slices.Equal(got, []string{"short", "long"}) {
		t.Errorf("model configurations %q", got)
	}
	if got := partVolumes(d); !slices.Equal(got, []float64{12000, 12000}) {
		t.Fatalf("as saved: %v", got)
	}

	a.AsLink().SetConfiguration("long")
	b.AsLink().SetParam("L", "W*2")
	if got := partVolumes(d); !slices.Equal(got, []float64{24000, 30000}) {
		t.Errorf("long and W*2: %v", got)
	}
	if a.AsLink().Configuration() != "long" || b.AsLink().Param("L") != "W*2" || b.AsLink().Problem() != "" {
		t.Errorf("configuration %q, param %q, problem %q", a.AsLink().Configuration(), b.AsLink().Param("L"), b.AsLink().Problem())
	}
	// The assembly's parameter drives the part.
	if err := d.Params.Set("W", "30"); err != nil {
		t.Fatal(err)
	}
	if got := partVolumes(d); !slices.Equal(got, []float64{24000, 18000}) {
		t.Errorf("W 30: %v", got)
	}

	// Saved and opened again, the same.
	path := filepath.Join(dir, "asm.tlspr")
	if err := d.File.Save(path); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range []string{`link-configuration "long"`, "link-param L = W*2"} {
		if !strings.Contains(string(data), line) {
			t.Errorf("the file lacks %q", line)
		}
	}
	e := New()
	if err := e.File.Load(path); err != nil {
		t.Fatal(err)
	}
	if got := partVolumes(e); !slices.Equal(got, []float64{24000, 18000}) {
		t.Errorf("opened again: %v", got)
	}

	// The model's own expression stands again.
	b.AsLink().SetParam("L", "")
	if got := partVolumes(d); !slices.Equal(got, []float64{24000, 12000}) {
		t.Errorf("L taken back: %v", got)
	}
}

// What a model lacks is refused, and a file that is no model takes no
// values.
func TestParametricLinkRefused(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	plate := savePlate(t, dir)
	d := New()
	a, err := d.Assembly.Link(plate, "a")
	if err != nil {
		t.Fatal(err)
	}
	for name, fn := range map[string]func(){
		"configuration": func() { a.AsLink().SetConfiguration("huge") },
		"parameter":     func() { a.AsLink().SetParam("H", "3") },
		"expression":    func() { a.AsLink().SetParam("L", "nope*2") },
		"not a link":    func() { d.Groups()[0].AsLink().SetParam("L", "3") },
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
	step, err := d.Assembly.Link(filepath.Join("..", "..", "..", "testdata", "step", "shapes", "cylinder.step"), "")
	if err != nil {
		t.Fatal(err)
	}
	step.AsLink().SetParam("L", "3")
	if p := step.AsLink().Problem(); !strings.Contains(p, "not a model") {
		t.Errorf("problem %q", p)
	}
}

// A part linked with values is regenerated with each set of values,
// and again when the part's file changes, not from what was kept of it.
func TestParametricLinkCache(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	plate := savePlate(t, dir)
	d := New()
	a, err := d.Assembly.Link(plate, "a")
	if err != nil {
		t.Fatal(err)
	}
	a.AsLink().SetParam("L", "50")
	if got := partVolumes(d); !slices.Equal(got, []float64{15000}) {
		t.Fatalf("L 50: %v", got)
	}
	a.AsLink().SetParam("L", "60")
	a.AsLink().SetParam("L", "50")
	if got := partVolumes(d); !slices.Equal(got, []float64{15000}) {
		t.Errorf("L 50 again: %v", got)
	}
	// The part changes: saved again 20 mm thick.
	savePlateDepth(t, dir, 20)
	later := time.Now().Add(time.Minute)
	if err := os.Chtimes(plate, later, later); err != nil {
		t.Fatal(err)
	}
	a.AsLink().SetParam("L", "60")
	a.AsLink().SetParam("L", "50")
	if got := partVolumes(d); !slices.Equal(got, []float64{30000}) {
		t.Errorf("L 50 of the new part: %v", got)
	}
}

// Relinking reads the other file, keeps the placement, and is one undo
// step.
func TestRelink(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	plate := savePlate(t, dir)
	block := New()
	block.Sketch.Rect(V(0, 0, 0), V(10, 10, 0))
	block.Features.Extrude(10, "block")
	other := filepath.Join(dir, "block.tlspr")
	if err := block.File.Save(other); err != nil {
		t.Fatal(err)
	}

	d := New()
	step := d.Host().(*SketchHost).BeginEval
	step()
	a, err := d.Assembly.Link(plate, "a")
	if err != nil {
		t.Fatal(err)
	}
	a.AsLink().Move(V(100, 0, 0))
	placed := a.AsLink().Offset()
	step()
	a.AsLink().SetFile(other)
	if got := partVolumes(d); !slices.Equal(got, []float64{1000}) {
		t.Errorf("relinked: %v", got)
	}
	if off := a.AsLink().Offset(); off != placed {
		t.Errorf("offset %v after relink", off)
	}
	if a.AsLink().File() != other {
		t.Errorf("file %q", a.AsLink().File())
	}
	if !d.Edit.Undo() {
		t.Fatal("no undo step")
	}
	if got := partVolumes(d); !slices.Equal(got, []float64{12000}) || a.AsLink().File() != plate {
		t.Errorf("undone: %v, %q", got, a.AsLink().File())
	}
	if !d.Edit.Redo() {
		t.Fatal("no redo")
	}
	if got := partVolumes(d); !slices.Equal(got, []float64{1000}) {
		t.Errorf("redone: %v", got)
	}
}

// A part regenerated with values is regenerated again when a file it
// links in turn changes, and a model that links itself with values is
// not regenerated inside itself without end.
func TestParametricLinkNested(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	plate := savePlate(t, dir)
	holder := New()
	if err := holder.Params.Set("N", "1"); err != nil {
		t.Fatal(err)
	}
	if _, err := holder.Assembly.Link(plate, "plate"); err != nil {
		t.Fatal(err)
	}
	holderPath := filepath.Join(dir, "holder.tlspr")
	if err := holder.File.Save(holderPath); err != nil {
		t.Fatal(err)
	}

	d := New()
	h, err := d.Assembly.Link(holderPath, "holder")
	if err != nil {
		t.Fatal(err)
	}
	h.AsLink().SetParam("N", "2")
	if got := partVolumes(d); !slices.Equal(got, []float64{12000}) {
		t.Fatalf("holder: %v", got)
	}
	// The plate the holder links changes.
	savePlateDepth(t, dir, 20)
	later := time.Now().Add(time.Minute)
	if err := os.Chtimes(plate, later, later); err != nil {
		t.Fatal(err)
	}
	h.AsLink().SetParam("N", "3")
	h.AsLink().SetParam("N", "2")
	if got := partVolumes(d); !slices.Equal(got, []float64{24000}) {
		t.Errorf("after the plate changed: %v", got)
	}

	// A model linking itself, passing itself a value.
	self := filepath.Join(dir, "self.tlspr")
	s := New()
	if err := s.Params.Set("N", "1"); err != nil {
		t.Fatal(err)
	}
	s.Sketch.Rect(V(0, 0, 0), V(10, 10, 0))
	s.Features.Extrude(10, "block")
	me, err := s.Assembly.Link(self, "me")
	if err != nil {
		t.Fatal(err)
	}
	me.AsLink().SetParam("N", "N+1")
	if err := s.File.Save(self); err != nil {
		t.Fatal(err)
	}
	e := New()
	l, err := e.Assembly.Link(self, "self")
	if err != nil {
		t.Fatal(err)
	}
	l.AsLink().SetParam("N", "5")
	if got := partVolumes(e); len(got) == 0 || got[0] != 1000 {
		t.Errorf("self link: %v", got)
	}
}
