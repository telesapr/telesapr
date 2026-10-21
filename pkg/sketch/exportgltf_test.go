package sketch

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// readGlb splits a .glb into its JSON document and binary chunk,
// checking the header and the chunk framing as a reader would.
func readGlb(t *testing.T, b []byte) (gltfDoc, []byte) {
	t.Helper()
	u32 := func(at int) uint32 { return binary.LittleEndian.Uint32(b[at:]) }
	if len(b) < 28 || u32(0) != glbMagic || u32(4) != 2 || int(u32(8)) != len(b) {
		t.Fatalf("header: %d bytes, magic %x, version %d, length %d", len(b), u32(0), u32(4), u32(8))
	}
	jl := int(u32(12))
	if u32(16) != glbChunkJSON || jl%4 != 0 {
		t.Fatalf("JSON chunk: type %x, length %d", u32(16), jl)
	}
	var doc gltfDoc
	if err := json.Unmarshal(b[20:20+jl], &doc); err != nil {
		t.Fatal(err)
	}
	if 20+jl == len(b) {
		return doc, nil
	}
	bl := int(u32(20 + jl))
	if u32(24+jl) != glbChunkBIN || bl%4 != 0 || 28+jl+bl != len(b) {
		t.Fatalf("BIN chunk: type %x, length %d, file %d", u32(24+jl), bl, len(b))
	}
	return doc, b[28+jl:]
}

// A glTF export of two boxes of two colours, one of them seen through:
// a primitive and a material a colour, the boxes' corners in metres as
// the bounds of the positions, unit normals, every triangle indexed,
// the volume of the mesh it holds, and the same bytes every time.
func TestExportGlb(t *testing.T) {
	t.Parallel()
	var a, b srf.SShell
	red, blue := core.NewRGBA(200, 0, 0, 255), core.NewRGBA(0, 0, 200, 128)
	a.MakePrism([]geom.Vector{{}, {X: 10}, {X: 10, Y: 20}, {Y: 20}}, geom.Vector{}, geom.Vector{Z: 30}, red)
	b.MakePrism([]geom.Vector{{X: 20}, {X: 25}, {X: 25, Y: 5}, {X: 20, Y: 5}}, geom.Vector{}, geom.Vector{Z: 5}, blue)
	var m srf.SMesh
	a.TriangulateInto(&m)
	b.TriangulateInto(&m)

	var out, again bytes.Buffer
	if err := ExportMeshAsGlbTo(&out, &m); err != nil {
		t.Fatal(err)
	}
	if err := ExportMeshAsGlbTo(&again, &m); err != nil || !bytes.Equal(out.Bytes(), again.Bytes()) {
		t.Fatalf("a second export differs (%v)", err)
	}
	doc, bin := readGlb(t, out.Bytes())
	if doc.Asset.Version != "2.0" || len(doc.Meshes) != 1 || len(doc.Materials) != 2 || len(doc.Meshes[0].Primitives) != 2 {
		t.Fatalf("asset %+v, %d meshes, %d materials", doc.Asset, len(doc.Meshes), len(doc.Materials))
	}
	if doc.Buffers[0].ByteLength > len(bin) {
		t.Fatalf("buffer of %d bytes in a chunk of %d", doc.Buffers[0].ByteLength, len(bin))
	}
	// 200 in sRGB is 0.578 linear; 128 of alpha is a blended material.
	if r := doc.Materials[0].PBRMetallicRoughness.BaseColorFactor; math.Abs(r[0]-0.577580) > 1e-6 || r[3] != 1 || doc.Materials[0].AlphaMode != "" {
		t.Errorf("red material %+v", doc.Materials[0])
	}
	if doc.Materials[1].AlphaMode != "BLEND" {
		t.Errorf("blue material %+v", doc.Materials[1])
	}

	floats := func(acc gltfAccessor) []float32 {
		v := doc.BufferViews[acc.BufferView]
		data := bin[v.ByteOffset : v.ByteOffset+v.ByteLength]
		r := make([]float32, len(data)/4)
		for i := range r {
			r[i] = math.Float32frombits(binary.LittleEndian.Uint32(data[4*i:]))
		}
		return r
	}
	wantLo := [][3]float32{{0, 0, 0}, {0.020, 0, 0}}
	wantHi := [][3]float32{{0.010, 0.020, 0.030}, {0.025, 0.005, 0.005}}
	var tris int
	var vol float64
	for i, pr := range doc.Meshes[0].Primitives {
		pos, nrm, ix := doc.Accessors[pr.Attributes["POSITION"]], doc.Accessors[pr.Attributes["NORMAL"]], doc.Accessors[pr.Indices]
		if pos.Count != nrm.Count || ix.Count%3 != 0 || pr.Material != i {
			t.Fatalf("primitive %d: %d positions, %d normals, %d indices", i, pos.Count, nrm.Count, ix.Count)
		}
		for k := range 3 {
			if math.Abs(float64(pos.Min[k]-wantLo[i][k])) > 1e-7 || math.Abs(float64(pos.Max[k]-wantHi[i][k])) > 1e-7 {
				t.Errorf("primitive %d bounds %v %v, want %v %v", i, pos.Min, pos.Max, wantLo[i], wantHi[i])
			}
		}
		p, n := floats(pos), floats(nrm)
		for j := range nrm.Count {
			if l := math.Hypot(math.Hypot(float64(n[3*j]), float64(n[3*j+1])), float64(n[3*j+2])); math.Abs(l-1) > 1e-6 {
				t.Fatalf("normal %d of primitive %d has length %g", j, i, l)
			}
		}
		v := doc.BufferViews[ix.BufferView]
		corner := func(c int) geom.Vector {
			j := int(binary.LittleEndian.Uint32(bin[v.ByteOffset+4*c:]))
			if j >= pos.Count {
				t.Fatalf("index %d past %d positions", j, pos.Count)
			}
			return geom.Vector{X: float64(p[3*j]), Y: float64(p[3*j+1]), Z: float64(p[3*j+2])}
		}
		for c := 0; c < ix.Count; c += 3 {
			a, b, cc := corner(c), corner(c+1), corner(c+2)
			vol += a.Dot(b.Cross(cc)) / 6
		}
		tris += ix.Count / 3
	}
	if tris != len(m.Triangles) {
		t.Errorf("%d triangles of %d", tris, len(m.Triangles))
	}
	// 10×20×30 + 5×5×5 mm³, in m³.
	if want := (6000.0 + 125) * 1e-9; math.Abs(vol-want) > want*1e-5 {
		t.Errorf("volume %g m³, want %g", vol, want)
	}
}

// An empty mesh is still a file a viewer opens: a scene of one empty
// node, with no mesh, buffer or binary chunk, none of which glTF allows
// empty.
func TestExportGlbEmpty(t *testing.T) {
	t.Parallel()
	var out bytes.Buffer
	if err := ExportMeshAsGlbTo(&out, &srf.SMesh{}); err != nil {
		t.Fatal(err)
	}
	doc, bin := readGlb(t, out.Bytes())
	if len(doc.Meshes) != 0 || doc.Nodes[0].Mesh != nil || len(doc.Buffers) != 0 || bin != nil || len(doc.Scenes) != 1 {
		t.Errorf("empty mesh written as %+v", doc)
	}
}
