package sketch_test

import (
	"bytes"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/script/api"
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/srf"
)

// TestMeshQualityTable has each preset read back as itself from its own
// settings, anything else as custom, and its name parse back.
func TestMeshQualityTable(t *testing.T) {
	t.Parallel()
	for _, q := range []sketch.MeshQuality{sketch.MeshQualityDraft, sketch.MeshQualityStandard, sketch.MeshQualityFine} {
		c, a, n := sketch.MeshQualitySettings(q)
		if got := sketch.QualityOf(c, a, n); got != q {
			t.Errorf("%s: settings read back as %s", q, got)
		}
		if got, err := sketch.ParseMeshQuality(q.String()); err != nil || got != q {
			t.Errorf("%s: parsed as %s, %v", q, got, err)
		}
	}
	if got, err := sketch.ParseMeshQuality("FINE"); err != nil || got != sketch.MeshQualityFine {
		t.Errorf("FINE parsed as %s, %v", got, err)
	}
	for _, bad := range []string{"custom", "", "best"} {
		if _, err := sketch.ParseMeshQuality(bad); err == nil {
			t.Errorf("%q parsed", bad)
		}
	}
	if got := sketch.QualityOf(0.3, 0, 64); got != sketch.MeshQualityCustom {
		t.Errorf("0.3 mm read as %s", got)
	}
	if got := sketch.QualityOf(0.1, 5, 64); got != sketch.MeshQualityCustom {
		t.Errorf("an angle of 5 read as %s", got)
	}
	// The defaults of the settings are the standard preset.
	es := sketch.ThawExportSettings(core.NoSettings{})
	if got := sketch.QualityOf(es.ChordTol, es.AngleTol, es.MaxSegments); got != sketch.MeshQualityStandard {
		t.Errorf("default settings read as %s", got)
	}
}

// TestStlDropsFlatTriangles has the STL writer leave out a triangle with
// two equal corners, and count only the rest in its header.
func TestStlDropsFlatTriangles(t *testing.T) {
	t.Parallel()
	a, b, c := geom.Vector{}, geom.Vector{X: 1}, geom.Vector{Y: 1}
	m := &srf.SMesh{Triangles: []srf.STriangle{
		{A: a, B: b, C: c},
		{A: a, B: b, C: b},
		{A: c, B: b, C: c},
	}}
	var out bytes.Buffer
	if err := sketch.ExportMeshAsStlTo(&out, m, 1); err != nil {
		t.Fatal(err)
	}
	if n := binary.LittleEndian.Uint32(out.Bytes()[80:]); n != 1 {
		t.Errorf("header counts %d triangles, want 1", n)
	}
	if got := out.Len(); got != 84+50 {
		t.Errorf("%d bytes, want %d", got, 84+50)
	}
}

// TestExportMeshStrict has a strict export refuse a mesh that crosses
// itself, without creating the file, and a lax one write it.
func TestExportMeshStrict(t *testing.T) {
	t.Parallel()
	d := api.New()
	block(d, 0, 0, 10, 10, 10)
	d.Features.SketchIn(d.Workplane("XY"), "")
	d.Sketch.Rect(api.V(8, 0, 0), api.V(18, 10, 0))
	d.Features.Extrude(10, "second").SetCombine(api.Assemble)

	dir := t.TempDir()
	es := sketch.ExportSettings{ChordTol: 0.1, MaxSegments: 64, Scale: 1}
	d.With(func(sk *sketch.Sketch) {
		lax := filepath.Join(dir, "lax.stl")
		if err := sk.ExportMeshTo(lax, es); err != nil {
			t.Fatalf("lax: %v", err)
		}
		if _, err := os.Stat(lax); err != nil {
			t.Errorf("lax: %v", err)
		}
		es.Strict = true
		strict := filepath.Join(dir, "strict.stl")
		if err := sk.ExportMeshTo(strict, es); !errors.Is(err, sketch.ErrMeshNotSound) {
			t.Fatalf("strict: %v", err)
		}
		if _, err := os.Stat(strict); err == nil {
			t.Error("strict: the file was created")
		}
	})
}
