package sketch

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"io"
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// model3mfXML is the part of a 3MF model the test reads.
type model3mfXML struct {
	Unit      string `xml:"unit,attr"`
	Materials []struct {
		Color string `xml:"displaycolor,attr"`
	} `xml:"resources>basematerials>base"`
	Vertices []struct {
		X float64 `xml:"x,attr"`
		Y float64 `xml:"y,attr"`
		Z float64 `xml:"z,attr"`
	} `xml:"resources>object>mesh>vertices>vertex"`
	Triangles []struct {
		V1 int `xml:"v1,attr"`
		V2 int `xml:"v2,attr"`
		V3 int `xml:"v3,attr"`
		P1 int `xml:"p1,attr"`
	} `xml:"resources>object>mesh>triangles>triangle"`
}

// A 3MF export (roadmap 16.5) of two boxes of two colours: a zip of the
// three parts, the model in mm, eight shared vertices a box, the
// triangles' colours from the table, the volume of the mesh it holds,
// and the same bytes every time.
func TestExport3mf(t *testing.T) {
	var a, b srf.SShell
	red, blue := core.NewRGBA(200, 0, 0, 255), core.NewRGBA(0, 0, 200, 255)
	a.MakePrism([]geom.Vector{{}, {X: 10}, {X: 10, Y: 20}, {Y: 20}}, geom.Vector{}, geom.Vector{Z: 30}, red)
	b.MakePrism([]geom.Vector{{X: 20}, {X: 25}, {X: 25, Y: 5}, {X: 20, Y: 5}}, geom.Vector{}, geom.Vector{Z: 5}, blue)
	var m srf.SMesh
	a.TriangulateInto(&m)
	b.TriangulateInto(&m)

	var out, again bytes.Buffer
	if err := ExportMeshAs3mfTo(&out, &m); err != nil {
		t.Fatal(err)
	}
	if err := ExportMeshAs3mfTo(&again, &m); err != nil || !bytes.Equal(out.Bytes(), again.Bytes()) {
		t.Fatalf("a second export differs (%v)", err)
	}
	zr, err := zip.NewReader(bytes.NewReader(out.Bytes()), int64(out.Len()))
	if err != nil {
		t.Fatal(err)
	}
	parts := map[string][]byte{}
	for _, f := range zr.File {
		r, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		parts[f.Name], _ = io.ReadAll(r)
		_ = r.Close()
	}
	for _, name := range []string{"[Content_Types].xml", "_rels/.rels", "3D/3dmodel.model"} {
		if len(parts[name]) == 0 {
			t.Errorf("no part %s", name)
		}
	}
	var model model3mfXML
	if err := xml.Unmarshal(parts["3D/3dmodel.model"], &model); err != nil {
		t.Fatal(err)
	}
	if model.Unit != "millimeter" || len(model.Vertices) != 16 || len(model.Triangles) != len(m.Triangles) {
		t.Fatalf("unit %q, %d vertices, %d triangles of %d", model.Unit, len(model.Vertices), len(model.Triangles), len(m.Triangles))
	}
	if len(model.Materials) != 2 || model.Materials[0].Color != "#C80000FF" || model.Materials[1].Color != "#0000C8FF" {
		t.Errorf("colours %+v", model.Materials)
	}
	vol, blues := 0.0, 0
	for _, tr := range model.Triangles {
		p := func(i int) geom.Vector {
			v := model.Vertices[i]
			return geom.Vector{X: v.X, Y: v.Y, Z: v.Z}
		}
		vol += p(tr.V1).Dot(p(tr.V2).Cross(p(tr.V3))) / 6
		if tr.P1 == 1 {
			blues++
		}
	}
	if math.Abs(vol-(6000+125)) > 1e-9 || blues != 12 {
		t.Errorf("volume %g, %d blue triangles", vol, blues)
	}
}
