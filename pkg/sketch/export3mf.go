package sketch

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// A 3MF file (roadmap 16.5, D1) is a zip of three parts: the content
// types, the relationship that names the model, and the model, an XML
// mesh of shared vertices and triangles that index them, each triangle
// coloured from a table of base materials. The mesh is written in
// millimetres, which the model says; the export scale, which STL needs
// for want of a unit, is not applied. The zip's dates are fixed, so one
// mesh always gives the same bytes.

const (
	contentTypes3mf = `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
 <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
 <Default Extension="model" ContentType="application/vnd.ms-package.3dmanufacturing-3dmodel+xml"/>
</Types>
`
	rels3mf = `<?xml version="1.0" encoding="UTF-8"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
 <Relationship Target="/3D/3dmodel.model" Id="rel0" Type="http://schemas.microsoft.com/3dmanufacturing/2013/01/3dmodel"/>
</Relationships>
`
)

// ExportMeshAs3mfTo writes m as a 3MF file.
func ExportMeshAs3mfTo(w io.Writer, m *srf.SMesh) error {
	zw := zip.NewWriter(w)
	when := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	part := func(name string, data []byte) error {
		f, err := zw.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Deflate, Modified: when})
		if err != nil {
			return err
		}
		_, err = f.Write(data)
		return err
	}
	if err := part("[Content_Types].xml", []byte(contentTypes3mf)); err != nil {
		return err
	}
	if err := part("_rels/.rels", []byte(rels3mf)); err != nil {
		return err
	}
	if err := part("3D/3dmodel.model", model3mf(m)); err != nil {
		return err
	}
	return zw.Close()
}

// model3mf is the model part: the colours, then the mesh.
func model3mf(m *srf.SMesh) []byte {
	var b bytes.Buffer
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>
<model unit="millimeter" xml:lang="en-US" xmlns="http://schemas.microsoft.com/3dmanufacturing/core/2015/02">
 <metadata name="Application">TeleSAPR</metadata>
 <resources>
  <basematerials id="1">
`)
	colors := map[core.RGBA]int{}
	var order []core.RGBA
	for i := range m.Triangles {
		c := m.Triangles[i].Meta.RGBA
		if _, ok := colors[c]; !ok {
			colors[c] = len(order)
			order = append(order, c)
		}
	}
	for i, c := range order {
		fmt.Fprintf(&b, "   <base name=\"color %d\" displaycolor=\"#%02X%02X%02X%02X\"/>\n", i, c.Red, c.Green, c.Blue, c.Alpha)
	}
	b.WriteString("  </basematerials>\n  <object id=\"2\" type=\"model\" pid=\"1\" pindex=\"0\">\n   <mesh>\n    <vertices>\n")
	index := map[geom.Vector]int{}
	var verts []geom.Vector
	at := func(p geom.Vector) int {
		if i, ok := index[p]; ok {
			return i
		}
		index[p] = len(verts)
		verts = append(verts, p)
		return len(verts) - 1
	}
	tris := make([][3]int, len(m.Triangles))
	for i := range m.Triangles {
		t := &m.Triangles[i]
		tris[i] = [3]int{at(t.A), at(t.B), at(t.C)}
	}
	num := func(v float64) string { return strconv.FormatFloat(v, 'g', -1, 64) }
	for _, p := range verts {
		fmt.Fprintf(&b, "     <vertex x=\"%s\" y=\"%s\" z=\"%s\"/>\n", num(p.X), num(p.Y), num(p.Z))
	}
	b.WriteString("    </vertices>\n    <triangles>\n")
	for i, t := range tris {
		// A triangle whose corners are one vertex has no area, which
		// 3MF forbids.
		if t[0] == t[1] || t[1] == t[2] || t[0] == t[2] {
			continue
		}
		fmt.Fprintf(&b, "     <triangle v1=\"%d\" v2=\"%d\" v3=\"%d\" pid=\"1\" p1=\"%d\"/>\n",
			t[0], t[1], t[2], colors[m.Triangles[i].Meta.RGBA])
	}
	b.WriteString("    </triangles>\n   </mesh>\n  </object>\n </resources>\n <build>\n  <item objectid=\"2\"/>\n </build>\n</model>\n")
	return b.Bytes()
}
