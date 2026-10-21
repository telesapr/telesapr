package sketch

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// glTF is written here rather than in pkg/export, beside 3MF, so that
// every way of exporting (window, command line, script, MCP) reaches
// the same writer through Sketch.ExportMeshTo.

// glTF lengths are metres; the model's are millimetres.
const gltfMetresPerMm = 0.001

// glTF's component types and buffer view targets, from the
// specification.
const (
	gltfFloat        = 5126
	gltfUnsignedInt  = 5125
	gltfArrayBuffer  = 34962
	gltfElementArray = 34963
	gltfTriangles    = 4
	glbMagic         = 0x46546C67 // "glTF"
	glbChunkJSON     = 0x4E4F534A // "JSON"
	glbChunkBIN      = 0x004E4942 // "BIN\0"
)

type gltfDoc struct {
	Asset       gltfAsset        `json:"asset"`
	Scene       int              `json:"scene"`
	Scenes      []gltfScene      `json:"scenes"`
	Nodes       []gltfNode       `json:"nodes"`
	Meshes      []gltfMesh       `json:"meshes,omitempty"`
	Materials   []gltfMaterial   `json:"materials,omitempty"`
	Accessors   []gltfAccessor   `json:"accessors,omitempty"`
	BufferViews []gltfBufferView `json:"bufferViews,omitempty"`
	Buffers     []gltfBuffer     `json:"buffers,omitempty"`
}

type gltfAsset struct {
	Version   string `json:"version"`
	Generator string `json:"generator"`
}

type gltfScene struct {
	Nodes []int `json:"nodes"`
}

type gltfNode struct {
	Mesh *int   `json:"mesh,omitempty"`
	Name string `json:"name,omitempty"`
}

type gltfMesh struct {
	Primitives []gltfPrimitive `json:"primitives"`
}

type gltfPrimitive struct {
	Attributes map[string]int `json:"attributes"`
	Indices    int            `json:"indices"`
	Material   int            `json:"material"`
	Mode       int            `json:"mode"`
}

type gltfMaterial struct {
	Name                 string  `json:"name"`
	PBRMetallicRoughness gltfPBR `json:"pbrMetallicRoughness"`
	AlphaMode            string  `json:"alphaMode,omitempty"`
}

type gltfPBR struct {
	BaseColorFactor [4]float64 `json:"baseColorFactor"`
	MetallicFactor  float64    `json:"metallicFactor"`
	RoughnessFactor float64    `json:"roughnessFactor"`
}

type gltfAccessor struct {
	BufferView    int       `json:"bufferView"`
	ComponentType int       `json:"componentType"`
	Count         int       `json:"count"`
	Type          string    `json:"type"`
	Min           []float32 `json:"min,omitempty"`
	Max           []float32 `json:"max,omitempty"`
}

type gltfBufferView struct {
	Buffer     int `json:"buffer"`
	ByteOffset int `json:"byteOffset"`
	ByteLength int `json:"byteLength"`
	Target     int `json:"target"`
}

type gltfBuffer struct {
	ByteLength int `json:"byteLength"`
}

// gltfVertex is a corner as glTF holds it: a position and a normal,
// shared between the triangles of one colour that have both the same.
type gltfVertex struct {
	p, n [3]float32
}

// gltfPrim is the triangles of one colour.
type gltfPrim struct {
	color   core.RGBA
	index   map[gltfVertex]uint32
	verts   []gltfVertex
	indices []uint32
}

// ExportMeshAsGlbTo writes the mesh as binary glTF (.glb): one
// primitive and one material per colour, in the order the colours first
// appear, lengths in metres as glTF asks. An empty mesh is a scene with
// one empty node, which glTF allows where it does not allow a mesh of
// no primitives or a buffer of no bytes.
func ExportMeshAsGlbTo(w io.Writer, m *srf.SMesh) error {
	prims := gltfPrims(m)
	doc := gltfDoc{
		Asset:  gltfAsset{Version: "2.0", Generator: "TeleSAPR"},
		Scenes: []gltfScene{{Nodes: []int{0}}},
		Nodes:  []gltfNode{{}},
	}
	if len(prims) > 0 {
		doc.Nodes[0].Mesh = new(0)
		doc.Meshes = []gltfMesh{{}}
	}
	var bin bytes.Buffer
	view := func(data []byte, target int) int {
		doc.BufferViews = append(doc.BufferViews, gltfBufferView{
			ByteOffset: bin.Len(), ByteLength: len(data), Target: target})
		bin.Write(data)
		return len(doc.BufferViews) - 1
	}
	for i, pr := range prims {
		pos, nrm := make([]byte, 0, len(pr.verts)*12), make([]byte, 0, len(pr.verts)*12)
		lo := [3]float32{math.MaxFloat32, math.MaxFloat32, math.MaxFloat32}
		hi := [3]float32{-math.MaxFloat32, -math.MaxFloat32, -math.MaxFloat32}
		for _, v := range pr.verts {
			for k := range 3 {
				pos = binary.LittleEndian.AppendUint32(pos, math.Float32bits(v.p[k]))
				nrm = binary.LittleEndian.AppendUint32(nrm, math.Float32bits(v.n[k]))
				lo[k], hi[k] = min(lo[k], v.p[k]), max(hi[k], v.p[k])
			}
		}
		idx := make([]byte, 0, len(pr.indices)*4)
		for _, j := range pr.indices {
			idx = binary.LittleEndian.AppendUint32(idx, j)
		}
		a := len(doc.Accessors)
		doc.Accessors = append(doc.Accessors,
			gltfAccessor{BufferView: view(pos, gltfArrayBuffer), ComponentType: gltfFloat,
				Count: len(pr.verts), Type: "VEC3", Min: lo[:], Max: hi[:]},
			gltfAccessor{BufferView: view(nrm, gltfArrayBuffer), ComponentType: gltfFloat,
				Count: len(pr.verts), Type: "VEC3"},
			gltfAccessor{BufferView: view(idx, gltfElementArray), ComponentType: gltfUnsignedInt,
				Count: len(pr.indices), Type: "SCALAR"})
		doc.Meshes[0].Primitives = append(doc.Meshes[0].Primitives, gltfPrimitive{
			Attributes: map[string]int{"POSITION": a, "NORMAL": a + 1},
			Indices:    a + 2, Material: i, Mode: gltfTriangles})
		doc.Materials = append(doc.Materials, gltfMaterialOf(pr.color, i))
	}
	if bin.Len() > 0 {
		doc.Buffers = []gltfBuffer{{ByteLength: bin.Len()}}
	}

	js, err := json.Marshal(doc)
	if err != nil {
		return fmt.Errorf("encoding glTF: %w", err)
	}
	// Each chunk is padded to four bytes: the JSON with spaces, the
	// binary with zeros.
	for len(js)%4 != 0 {
		js = append(js, ' ')
	}
	for bin.Len()%4 != 0 {
		bin.WriteByte(0)
	}
	var out bytes.Buffer
	le := func(v uint32) { out.Write(binary.LittleEndian.AppendUint32(nil, v)) }
	le(glbMagic)
	le(2)
	total := 12 + 8 + len(js)
	if bin.Len() > 0 {
		total += 8 + bin.Len()
	}
	le(uint32(total))
	le(uint32(len(js)))
	le(glbChunkJSON)
	out.Write(js)
	if bin.Len() > 0 {
		le(uint32(bin.Len()))
		le(glbChunkBIN)
		out.Write(bin.Bytes())
	}
	_, err = w.Write(out.Bytes())
	return err
}

// gltfPrims splits the mesh by colour and shares the corners that have
// the same position and normal. Triangles with no area are left out:
// viewers draw them as cracks.
func gltfPrims(m *srf.SMesh) []*gltfPrim {
	var prims []*gltfPrim
	byColor := map[core.RGBA]*gltfPrim{}
	for i := range m.Triangles {
		t := &m.Triangles[i]
		if t.A.Equals(t.B) || t.B.Equals(t.C) || t.A.Equals(t.C) {
			continue
		}
		pr := byColor[t.Meta.RGBA]
		if pr == nil {
			pr = &gltfPrim{color: t.Meta.RGBA, index: map[gltfVertex]uint32{}}
			byColor[t.Meta.RGBA] = pr
			prims = append(prims, pr)
		}
		face := t.Normal().WithMagnitude(1)
		for _, c := range [3][2]geom.Vector{{t.A, t.NA}, {t.B, t.NB}, {t.C, t.NC}} {
			p, n := c[0].ScaledBy(gltfMetresPerMm), c[1]
			// A corner with no normal of its own takes the face's.
			if n.Magnitude() < 1e-12 {
				n = face
			} else {
				n = n.WithMagnitude(1)
			}
			v := gltfVertex{
				p: [3]float32{float32(p.X), float32(p.Y), float32(p.Z)},
				n: [3]float32{float32(n.X), float32(n.Y), float32(n.Z)},
			}
			j, ok := pr.index[v]
			if !ok {
				j = uint32(len(pr.verts))
				pr.index[v] = j
				pr.verts = append(pr.verts, v)
			}
			pr.indices = append(pr.indices, j)
		}
	}
	return prims
}

// gltfMaterialOf is a plain dielectric of the colour. glTF's colour
// factors are linear, the model's sRGB.
func gltfMaterialOf(c core.RGBA, i int) gltfMaterial {
	lin := func(v uint8) float64 {
		s := float64(v) / 255
		if s <= 0.04045 {
			return round6(s / 12.92)
		}
		return round6(math.Pow((s+0.055)/1.055, 2.4))
	}
	mat := gltfMaterial{
		Name: fmt.Sprintf("color %d", i),
		PBRMetallicRoughness: gltfPBR{
			BaseColorFactor: [4]float64{lin(c.Red), lin(c.Green), lin(c.Blue), round6(float64(c.Alpha) / 255)},
			RoughnessFactor: 0.5,
		},
	}
	if c.Alpha < 255 {
		mat.AlphaMode = "BLEND"
	}
	return mat
}

// round6 keeps the file the same on every platform: the last bits of
// math.Pow are not.
func round6(v float64) float64 { return math.Round(v*1e6) / 1e6 }
