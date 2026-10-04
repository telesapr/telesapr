package sketch_test

import (
	"bytes"
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
	dir := t.TempDir()
	part := func(name string, w float64) string {
		d := api.New()
		d.Rect(api.V(0, 0, 0), api.V(w, w, 0))
		d.Extrude(w, "")
		path := filepath.Join(dir, name+".tlspr")
		if err := d.Save(path); err != nil {
			t.Fatal(err)
		}
		return path
	}
	plate, bolt, nut, washer := part("plate", 10), part("bolt", 2), part("nut", 3), part("washer", 1)

	d := api.New()
	link := func(path string) api.Group {
		g, err := d.Link(path, "")
		if err != nil {
			t.Fatal(err)
		}
		return g
	}
	link(plate)
	link(bolt)
	d.Translate(3, api.V(20, 0, 0), "bolts")
	link(nut)
	d.Mirror(d.Workplane("YZ"), "")
	w := link(washer)
	w.SetSuppress(api.SuppressSolid)
	d.Activate(w)

	got := map[string]int{}
	for _, it := range d.BillOfMaterials() {
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
	if err := d.ExportBOM(out); err != nil {
		t.Fatal(err)
	}
	csv, _ := os.ReadFile(out)
	lines := strings.Split(strings.TrimSpace(string(csv)), "\n")
	if len(lines) != 4 || lines[0] != "part,count,file,groups" || !strings.HasPrefix(lines[2], "bolt,3,"+bolt+",") {
		t.Errorf("CSV:\n%s", csv)
	}
	var b bytes.Buffer
	if err := sketch.WriteBOMCSV(&b, nil); err != nil || b.String() != "part,count,file,groups\n" {
		t.Errorf("an empty bill: %q, %v", b.String(), err)
	}
}

// A STEP file or a mesh links where it was drawn, its coordinates the
// file's; a model comes in off the middle of the view, as before
// (roadmap 16.13).
func TestLinkPlacedAsDrawn(t *testing.T) {
	dir := t.TempDir()
	src := api.New()
	src.Rect(api.V(100, 0, 0), api.V(110, 10, 0))
	src.Extrude(5, "")
	model := filepath.Join(dir, "part.tlspr")
	mesh := filepath.Join(dir, "part.stl")
	if err := src.Save(model); err != nil {
		t.Fatal(err)
	}
	if err := src.Export(mesh); err != nil {
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
	g, err := d.Link(mesh, "")
	if err != nil {
		t.Fatal(err)
	}
	if hi := highest(g); hi.Minus(api.V(110, 10, 5)).Magnitude() > 1e-4 {
		t.Errorf("the mesh's highest corner is at %v", hi)
	}
	g, err = d.Link(model, "")
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
	dir := t.TempDir()
	save := func(d *api.Doc, name string) string {
		path := filepath.Join(dir, name+".tlspr")
		if err := d.Save(path); err != nil {
			t.Fatal(err)
		}
		return path
	}
	link := func(d *api.Doc, path string) api.Group {
		g, err := d.Link(path, "")
		if err != nil {
			t.Fatal(err)
		}
		return g
	}
	part := api.New()
	part.Rect(api.V(0, 0, 0), api.V(2, 2, 0))
	part.Extrude(2, "")
	bolt := save(part, "bolt")

	bracket := api.New()
	link(bracket, bolt)
	bracket.Translate(2, api.V(10, 0, 0), "")
	save(bracket, "bracket")

	top := api.New()
	link(top, filepath.Join(dir, "bracket.tlspr"))
	top.Translate(2, api.V(0, 30, 0), "")
	counts := func(d *api.Doc) map[string]int {
		got := map[string]int{}
		for _, it := range d.BillOfMaterials() {
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
	for _, it := range top.BillOfMaterials() {
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
	opened.Translate(3, api.V(0, 0, 40), "")
	if c := counts(opened); c["bracket"] != 6 || c["bolt"] != 12 {
		t.Errorf("a repeat of the repeat: %v", c)
	}
	opened.Undo()
	check("undone", counts(opened))

	// a links b, which links a.
	b := api.New()
	b.Rect(api.V(0, 0, 0), api.V(1, 1, 0))
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
