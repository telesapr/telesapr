package sketch_test

import (
	"bytes"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/script/api"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// A bill of materials (roadmap 16.7): a plate linked once, a bolt
// linked and repeated three times one-sided with the original (the copy
// at the original's place not counted), a nut linked and mirrored with
// the original kept, and a washer linked and left out of the model.
func TestBillOfMaterials(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	part := func(name string, w float64) string {
		d := api.New()
		d.Sketch.Rect(api.V(0, 0, 0), api.V(w, w, 0))
		d.Features.Extrude(w, "")
		path := filepath.Join(dir, name+".tlspr")
		if err := d.File.Save(path); err != nil {
			t.Fatal(err)
		}
		return path
	}
	plate, bolt, nut, washer := part("plate", 10), part("bolt", 2), part("nut", 3), part("washer", 1)

	d := api.New()
	link := func(path string) api.Group {
		g, err := d.Assembly.Link(path, "")
		if err != nil {
			t.Fatal(err)
		}
		return g
	}
	link(plate)
	link(bolt)
	d.Features.Translate(3, api.V(20, 0, 0), "bolts")
	link(nut)
	d.Features.Mirror(d.Workplane("YZ"), "")
	w := link(washer)
	w.SetSuppress(api.SuppressSolid)
	d.Activate(w)

	got := map[string]int{}
	for _, it := range d.Assembly.BillOfMaterials() {
		got[it.Part] = it.Count
	}
	want := map[string]int{"plate": 1, "bolt": 3, "nut": 2}
	if len(got) != len(want) {
		t.Errorf("parts %v, want %v", got, want)
	}
	for p, n := range want {
		if got[p] != n {
			t.Errorf("%d %s, want %d (all %v)", got[p], p, n, got)
		}
	}

	out := filepath.Join(dir, "bom.csv")
	if err := d.Assembly.ExportBOM(out); err != nil {
		t.Fatal(err)
	}
	csv, _ := os.ReadFile(out)
	lines := strings.Split(strings.TrimSpace(string(csv)), "\n")
	if len(lines) != 4 || lines[0] != "part,count,file,groups,mass_g" || !strings.HasPrefix(lines[2], "bolt,3,"+bolt+",") {
		t.Errorf("CSV:\n%s", csv)
	}
	var b bytes.Buffer
	if err := sketch.WriteBOMCSV(&b, nil); err != nil || b.String() != "part,count,file,groups,mass_g\n" {
		t.Errorf("an empty bill: %q, %v", b.String(), err)
	}
}

// A STEP file or a mesh links where it was drawn, its coordinates the
// file's; a model comes in off the middle of the view, as before
// (roadmap 16.13).
func TestLinkPlacedAsDrawn(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	src := api.New()
	src.Sketch.Rect(api.V(100, 0, 0), api.V(110, 10, 0))
	src.Features.Extrude(5, "")
	model := filepath.Join(dir, "part.tlspr")
	mesh := filepath.Join(dir, "part.stl")
	if err := src.File.Save(model); err != nil {
		t.Fatal(err)
	}
	if err := src.File.Export(mesh); err != nil {
		t.Fatal(err)
	}
	// The highest corner of the points: a mesh's origin is one too.
	highest := func(g api.Group) api.Vec {
		hi := api.V(-1e9, -1e9, -1e9)
		for _, e := range g.Entities() {
			if e.Kind() == "point" {
				p := e.Pos()
				hi = api.V(max(hi.X, p.X), max(hi.Y, p.Y), max(hi.Z, p.Z))
			}
		}
		return hi
	}
	d := api.New()
	g, err := d.Assembly.Link(mesh, "")
	if err != nil {
		t.Fatal(err)
	}
	if hi := highest(g); hi.Minus(api.V(110, 10, 5)).Magnitude() > 1e-4 {
		t.Errorf("the mesh's highest corner is at %v", hi)
	}
	g, err = d.Assembly.Link(model, "")
	if err != nil {
		t.Fatal(err)
	}
	if hi := highest(g); hi.Minus(api.V(110, 10, 5)).Magnitude() < 1 {
		t.Errorf("the model came in where it was drawn, %v", hi)
	}
}

// A linked model's own links count too (roadmap 29.3): a bracket that
// links a bolt and repeats it, linked and repeated in turn, places
// four bolts; the bill survives saving, opening and undo; and a file
// that links itself, by way of another, is counted without looping.
func TestBillOfMaterialsNested(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	save := func(d *api.Doc, name string) string {
		path := filepath.Join(dir, name+".tlspr")
		if err := d.File.Save(path); err != nil {
			t.Fatal(err)
		}
		return path
	}
	link := func(d *api.Doc, path string) api.Group {
		g, err := d.Assembly.Link(path, "")
		if err != nil {
			t.Fatal(err)
		}
		return g
	}
	part := api.New()
	part.Sketch.Rect(api.V(0, 0, 0), api.V(2, 2, 0))
	part.Features.Extrude(2, "")
	bolt := save(part, "bolt")

	bracket := api.New()
	link(bracket, bolt)
	bracket.Features.Translate(2, api.V(10, 0, 0), "")
	save(bracket, "bracket")

	top := api.New()
	link(top, filepath.Join(dir, "bracket.tlspr"))
	top.Features.Translate(2, api.V(0, 30, 0), "")
	counts := func(d *api.Doc) map[string]int {
		got := map[string]int{}
		for _, it := range d.Assembly.BillOfMaterials() {
			got[it.Part] = it.Count
		}
		return got
	}
	check := func(when string, got map[string]int) {
		t.Helper()
		if len(got) != 2 || got["bracket"] != 2 || got["bolt"] != 4 {
			t.Errorf("%s: %v, want 2 brackets and 4 bolts", when, got)
		}
	}
	check("linked", counts(top))
	for _, it := range top.Assembly.BillOfMaterials() {
		if it.Part == "bolt" && (len(it.Groups) != 2 || !strings.Contains(it.Groups[0], " > ")) {
			t.Errorf("bolts placed by %q", it.Groups)
		}
	}
	path := save(top, "top")
	opened, err := api.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	check("opened", counts(opened))
	opened.Features.Translate(3, api.V(0, 0, 40), "")
	if c := counts(opened); c["bracket"] != 6 || c["bolt"] != 12 {
		t.Errorf("a repeat of the repeat: %v", c)
	}
	opened.Edit.Undo()
	check("undone", counts(opened))

	// a links b, which links a.
	b := api.New()
	b.Sketch.Rect(api.V(0, 0, 0), api.V(1, 1, 0))
	bPath := save(b, "b")
	a := api.New()
	link(a, bPath)
	aPath := save(a, "a")
	b2 := api.New()
	link(b2, aPath)
	save(b2, "b")
	loop := api.New()
	link(loop, aPath)
	if c := counts(loop); c["a"] < 1 || c["b"] != 1 {
		t.Errorf("a loop of links: %v", c)
	}
}

// A part's mass in the bill is its volume at the density set on the
// link that places it; a part with none set has none.
func TestBillOfMaterialsMass(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	part := func(name string) string {
		d := api.New()
		d.Sketch.Rect(api.V(0, 0, 0), api.V(10, 10, 0))
		d.Features.Extrude(10, "")
		path := filepath.Join(dir, name+".tlspr")
		if err := d.File.Save(path); err != nil {
			t.Fatal(err)
		}
		return path
	}
	steel, plain := part("steel"), part("plain")
	d := api.New()
	g, err := d.Assembly.Link(steel, "")
	if err != nil {
		t.Fatal(err)
	}
	g.SetDensity(7.85)
	if _, err := d.Assembly.Link(plain, ""); err != nil {
		t.Fatal(err)
	}
	mass := map[string]float64{}
	for _, it := range d.Assembly.BillOfMaterials() {
		mass[it.Part] = it.Mass
	}
	if m := mass["steel"]; math.Abs(m-7.85) > 1e-9 || mass["plain"] != 0 {
		t.Errorf("masses %v, want steel 7.85 and plain none", mass)
	}
	var b bytes.Buffer
	if err := sketch.WriteBOMCSV(&b, d.Assembly.BillOfMaterials()); err != nil {
		t.Fatal(err)
	}
	if lines := strings.Split(b.String(), "\n"); !strings.HasSuffix(lines[1], ",7.850") || !strings.HasSuffix(lines[2], ",") {
		t.Errorf("CSV:\n%s", b.String())
	}
}

// A file is named as the model's file stores a link: relative to its
// folder, absolute while the model is unsaved.
func TestRelativePath(t *testing.T) {
	t.Parallel()
	root := t.TempDir() // absolute on every system, with a volume on Windows
	model := filepath.Join(root, "models", "top.tlspr")
	for _, c := range []struct{ save, path, want string }{
		{model, filepath.Join(root, "models", "parts", "bolt.tlspr"), filepath.Join("parts", "bolt.tlspr")},
		{model, filepath.Join(root, "shapes", "box.step"), filepath.Join("..", "shapes", "box.step")},
		{model, "parts/bolt.tlspr", "parts/bolt.tlspr"},
		{"", filepath.Join(root, "box.step"), filepath.Join(root, "box.step")},
		{model, "", ""},
	} {
		if got := sketch.RelativePath(c.save, c.path); got != c.want {
			t.Errorf("RelativePath(%q, %q) = %q, want %q", c.save, c.path, got, c.want)
		}
	}
}

// The bill names each file relative to the model's file once the model
// is saved, in the list and in the CSV, the parts a linked model links
// too; a file linked by its absolute path and by the relative one the
// model's file stored is one part.
func TestBillOfMaterialsRelative(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	parts := filepath.Join(dir, "parts")
	if err := os.MkdirAll(parts, 0o755); err != nil {
		t.Fatal(err)
	}
	save := func(d *api.Doc, path string) {
		if err := d.File.Save(path); err != nil {
			t.Fatal(err)
		}
	}
	link := func(d *api.Doc, path string) {
		if _, err := d.Assembly.Link(path, ""); err != nil {
			t.Fatal(err)
		}
	}
	part := api.New()
	part.Sketch.Rect(api.V(0, 0, 0), api.V(2, 2, 0))
	part.Features.Extrude(2, "")
	bolt := filepath.Join(parts, "bolt.tlspr")
	save(part, bolt)
	bracket := api.New()
	link(bracket, bolt)
	brk := filepath.Join(parts, "bracket.tlspr")
	save(bracket, brk)

	top := api.New()
	link(top, brk)
	files := func(d *api.Doc) map[string]string {
		got := map[string]string{}
		for _, it := range d.Assembly.BillOfMaterials() {
			got[it.Part] = it.File
		}
		return got
	}
	if got := files(top); got["bracket"] != brk || got["bolt"] != bolt {
		t.Errorf("unsaved: %v, want absolute paths", got)
	}
	model := filepath.Join(dir, "top.tlspr")
	save(top, model)
	want := map[string]string{"bracket": filepath.Join("parts", "bracket.tlspr"), "bolt": filepath.Join("parts", "bolt.tlspr")}
	if got := files(top); got["bracket"] != want["bracket"] || got["bolt"] != want["bolt"] {
		t.Errorf("saved: %v, want %v", got, want)
	}
	csvPath := filepath.Join(dir, "bom.csv")
	if err := top.Assembly.ExportBOM(csvPath); err != nil {
		t.Fatal(err)
	}
	csv, _ := os.ReadFile(csvPath)
	if !strings.Contains(string(csv), "bracket,1,"+want["bracket"]+",") || strings.Contains(string(csv), dir) {
		t.Errorf("CSV:\n%s", csv)
	}

	opened, err := api.Open(model)
	if err != nil {
		t.Fatal(err)
	}
	link(opened, brk)
	items := opened.Assembly.BillOfMaterials()
	if len(items) != 2 || items[0].File != want["bracket"] || items[0].Count != 2 || items[1].Count != 2 {
		t.Errorf("a file linked by both names: %+v", items)
	}
}
