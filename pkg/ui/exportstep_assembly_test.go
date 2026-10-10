package ui

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/persist/step"
	"github.com/telesapr/telesapr/pkg/script/api"
)

// A STEP file of an assembly (roadmap 16.8): a part linked twice, the
// second turned and moved, and a block assembled beside; three
// occurrences of two products, which read back, with their placements,
// as the model's three solids.
func TestExportStepAssembly(t *testing.T) {
	ss, _ := newTestApp(t)
	dir := t.TempDir()
	partDoc := api.New()
	partDoc.Sketch.Rect(api.V(0, 0, 0), api.V(10, 4, 0))
	partDoc.Features.Extrude(2, "")
	part := filepath.Join(dir, "bar.tlspr")
	if err := partDoc.File.Save(part); err != nil {
		t.Fatal(err)
	}

	doc := api.NewDoc(ss.ScriptHost())
	doc.Sketch.Rect(api.V(30, 0, 0), api.V(35, 5, 0))
	doc.Features.Extrude(5, "block")
	if _, err := doc.Assembly.Link(part, ""); err != nil {
		t.Fatal(err)
	}
	b, err := doc.Assembly.Link(part, "")
	if err != nil {
		t.Fatal(err)
	}
	sk := ss.SK
	// The second bar turned a quarter about z and moved.
	sk.MustParam(b.H.Param(0)).Val = -20
	sk.MustParam(b.H.Param(3)).Val = math.Sqrt2 / 2
	sk.MustParam(b.H.Param(6)).Val = math.Sqrt2 / 2
	doc.Edit.Regen()
	want := doc.Active().Analyze().Check()

	out := filepath.Join(dir, "assembly.step")
	if keep := os.Getenv("TELESAPR_KEEP_STEP"); keep != "" {
		out = keep
	}
	if err := doc.File.Export(out); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if n := strings.Count(text, "NEXT_ASSEMBLY_USAGE_OCCURRENCE"); n != 3 {
		t.Errorf("%d occurrences", n)
	}
	if n := strings.Count(text, " PRODUCT('") + strings.Count(text, "=PRODUCT('"); n != 3 {
		t.Errorf("%d products, the model's and two parts'", n)
	}
	if n := strings.Count(text, "MANIFOLD_SOLID_BREP"); n != 2 {
		t.Errorf("%d solids written, want one a product", n)
	}

	// Read back, placed: the same volume and box.
	res, err := step.ReadFile(out, step.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Report.Shapes != 2 || res.Report.Placements != 3 {
		t.Errorf("read back %d shapes in %d placements", res.Report.Shapes, res.Report.Placements)
	}
	if got := res.Mesh.CalculateVolume(); math.Abs(got-want.Volume) > 1e-3*want.Volume {
		t.Errorf("read back: volume %g, want %g", got, want.Volume)
	}
	var lo, hi = api.V(1e9, 1e9, 1e9), api.V(-1e9, -1e9, -1e9)
	for _, tr := range res.Mesh.Triangles {
		for _, p := range []api.Vec{tr.A, tr.B, tr.C} {
			lo = api.V(min(lo.X, p.X), min(lo.Y, p.Y), min(lo.Z, p.Z))
			hi = api.V(max(hi.X, p.X), max(hi.Y, p.Y), max(hi.Z, p.Z))
		}
	}
	if lo.Minus(want.Min).Magnitude() > 1e-3 || hi.Minus(want.Max).Magnitude() > 1e-3 {
		t.Errorf("read back from %v to %v, want %v to %v", lo, hi, want.Min, want.Max)
	}
}
