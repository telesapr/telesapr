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
