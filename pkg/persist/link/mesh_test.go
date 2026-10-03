package link

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/srf"
)

// boxMesh is a 10 x 20 x 30 box's triangles.
func boxMesh() *srf.SMesh {
	var sh srf.SShell
	sh.MakePrism([]geom.Vector{{}, {X: 10}, {X: 10, Y: 20}, {Y: 20}}, geom.Vector{}, geom.Vector{Z: 30},
		core.NewRgbaColor(1, 2, 3, 255))
	var m srf.SMesh
	sh.TriangulateInto(&m)
	return &m
}

// textStl is m as a text STL file.
func textStl(m *srf.SMesh) []byte {
	var b bytes.Buffer
	b.WriteString("solid box\n")
	for _, t := range m.Triangles {
		n := t.Normal().WithMagnitude(1)
		fmt.Fprintf(&b, "  facet normal %g %g %g\n    outer loop\n", n.X, n.Y, n.Z)
		for _, p := range []geom.Vector{t.A, t.B, t.C} {
			fmt.Fprintf(&b, "      vertex %g %g %g\n", p.X, p.Y, p.Z)
		}
		b.WriteString("    endloop\n  endfacet\n")
	}
	b.WriteString("endsolid box\n")
	return b.Bytes()
}

// The box's corners, its faces as quads, and a material for each half.
const boxObj = `# a box
mtllib box.mtl
v 0 0 0
v 10 0 0
v 10 20 0
v 0 20 0
v 0 0 30
v 10 0 30
v 10 20 30
v 0 20 30
usemtl red
f 1 4 3 2
f 5 6 7 8
f 1 2 6 5
usemtl blue
f 2/1 3/2 7/3 6/4
f -5//1 -1//1 -2//1 -6//1
f 4 1 5 8
`

const boxMtl = `newmtl red
Kd 1 0 0
newmtl blue
Kd 0 0 1
d 0.5
`

// A mesh linked from a binary STL, one whose header starts "solid" as
// some exporters write, a text STL and an OBJ file of quads (roadmap
// 16.6): the same box, and the same entities.
func TestLinkMeshFiles(t *testing.T) {
	dir := t.TempDir()
	m := boxMesh()
	var bin bytes.Buffer
	if err := sketch.ExportMeshAsStlTo(&bin, m, 1); err != nil {
		t.Fatal(err)
	}
	solidHeader := append([]byte("solid but binary"), bin.Bytes()[16:]...)
	files := map[string][]byte{
		"binary.stl": bin.Bytes(), "solid.stl": solidHeader, "text.stl": textStl(m),
		"box.obj": []byte(boxObj), "box.mtl": []byte(boxMtl),
	}
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var first []core.Entity
	for _, name := range []string{"binary.stl", "solid.stl", "text.stl", "box.obj"} {
		sk := &sketch.Sketch{}
		var el []core.Entity
		var got srf.SMesh
		load := linkStl
		if strings.HasSuffix(name, ".obj") {
			load = linkObj
		}
		if !load(filepath.Join(dir, name), sk, &el, &got) {
			t.Fatalf("%s not linked", name)
		}
		vol := 0.0
		for _, tr := range got.Triangles {
			vol += tr.A.Dot(tr.B.Cross(tr.C)) / 6
		}
		if len(got.Triangles) != 12 || vol < 6000-1e-9 || vol > 6000+1e-9 {
			t.Errorf("%s: %d triangles, volume %g", name, len(got.Triangles), vol)
		}
		if first == nil {
			first = el
		} else if len(el) != len(first) {
			t.Errorf("%s: %d entities, %d from the binary file", name, len(el), len(first))
		}
		if name == "box.obj" {
			red, blue := 0, 0
			for _, tr := range got.Triangles {
				switch tr.Meta.RgbaColor {
				case core.RgbaColor{Red: 255, Alpha: 255}:
					red++
				case core.RgbaColor{Blue: 255, Alpha: 128}:
					blue++
				}
			}
			if red != 6 || blue != 6 {
				t.Errorf("%d red and %d blue triangles", red, blue)
			}
		}
	}
}

// What can't be read is a message, and counts as missing.
func TestLinkMeshFilesRefused(t *testing.T) {
	dir := t.TempDir()
	for name, data := range map[string]string{
		"empty.stl":    "solid nothing\nendsolid nothing\n",
		"short.stl":    "solid x\nfacet normal 0 0 1\nouter loop\nvertex 0 0\n",
		"two.stl":      "solid x\nfacet normal 0 0 1\nouter loop\nvertex 0 0 0\nvertex 1 0 0\nendloop\nendfacet\n",
		"none.obj":     "v 0 0 0\n",
		"far.obj":      "v 0 0 0\nv 1 0 0\nv 0 1 0\nf 1 2 4\n",
		"badfloat.obj": "v 0 0 x\n",
	} {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
		var said string
		sk := &sketch.Sketch{Message: func(s string) { said = s }}
		var el []core.Entity
		var m srf.SMesh
		load := linkStl
		if strings.HasSuffix(name, ".obj") {
			load = linkObj
		}
		if load(path, sk, &el, &m) || !strings.Contains(said, "can't be read") {
			t.Errorf("%s: linked, or said %q", name, said)
		}
	}
}
