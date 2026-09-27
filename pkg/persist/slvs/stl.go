package slvs

import (
	"encoding/binary"
	"math"
	"os"
	"sort"

	"github.com/spacesolve/spacesolve/pkg/core"
	"github.com/spacesolve/spacesolve/pkg/geom"
	"github.com/spacesolve/spacesolve/pkg/sketch"
	"github.com/spacesolve/spacesolve/pkg/srf"
)

// This file ports solvespace/src/importmesh.cpp, which links a binary
// STL triangle mesh.

const minPointDistance = 0.001

// stlVertex is a mesh vertex with the normals of the triangles that
// share it.
type stlVertex struct {
	p      geom.Vector
	normal []geom.Vector
}

func isEdgeVertex(v *stlVertex) bool {
	result := false
	for i := range v.normal {
		for j := i; j < len(v.normal); j++ {
			if v.normal[i].Dot(v.normal[j]) < 0.9 {
				result = true
			}
		}
	}
	return result
}

func addUnique(lv *[]stlVertex, p, n geom.Vector) {
	i := 0
	for ; i < len(*lv); i++ {
		if (*lv)[i].p.EqualsTol(p, minPointDistance) {
			break
		}
	}
	if i == len(*lv) {
		*lv = append(*lv, stlVertex{p: p})
	}
	(*lv)[i].normal = append((*lv)[i].normal, n)
}

// newPoint makes a point with the next id; its type doesn't matter,
// since the linked group copies it.
func newPoint(el *[]core.Entity, id *int, p geom.Vector) core.HEntity {
	en := core.Entity{}
	en.Type = core.EntityPointNCopy
	en.Group = core.HGroup{V: 462}
	en.ActPoint = p
	en.Style = core.HStyle{V: core.StyleDatum}
	en.ActVisible = true

	en.H = core.HEntity{V: uint32(*id) + en.Group.V*65536}
	*id++
	*el = append(*el, en)
	return en.H
}

func addVertex(el *[]core.Entity, v geom.Vector) {
	if len(*el) < 15000 {
		id := len(*el)
		newPoint(el, &id, v)
	}
}

func newNormal(el *[]core.Entity, id *int, normal geom.Quaternion, p core.HEntity) core.HEntity {
	en := core.Entity{}
	en.Type = core.EntityNormalNCopy
	en.Group = core.HGroup{V: 472}
	en.ActNormal = normal
	en.Style = core.HStyle{V: core.StyleNormals}
	// to be visible we need to add a point.
	en.Point[0] = p
	en.ActVisible = true

	*id++
	en.H = core.HEntity{V: uint32(*id) + en.Group.V*65536}
	*el = append(*el, en)
	return en.H
}

func newLine(el *[]core.Entity, id *int, p0, p1 core.HEntity) core.HEntity {
	en := core.Entity{}
	en.Type = core.EntityLineSegment
	en.Point[0] = p0
	en.Point[1] = p1
	en.Group = core.HGroup{V: 493}
	en.Construction = true
	en.Style = core.HStyle{V: core.StyleConstruction}
	en.ActVisible = true

	en.H = core.HEntity{V: uint32(*id) + en.Group.V*65536}
	*id++
	*el = append(*el, en)
	return en.H
}

// linkStl reads a binary STL file into mesh m, with entities for its
// origin, its bounding box and the vertices on its edges (C++
// LinkStl). A text STL isn't supported, and counts as a missing file.
func linkStl(path string, sk *sketch.Sketch, el *[]core.Entity, m *srf.SMesh) bool {
	*el = (*el)[:0]
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	// C++ reads a short file as zeros.
	read := func(off, n int) []byte {
		b := make([]byte, n)
		if off < len(data) {
			copy(b, data[off:])
		}
		return b
	}

	if string(read(0, 5)) == "solid" {
		// just returning false will trigger the warning that linked file
		// is not present; best solution is to add an importer for text STL.
		if sk.Message != nil {
			sk.Message("Text-formated STL files are not currently supported")
		}
		return false
	}

	n := binary.LittleEndian.Uint32(read(80, 4))
	off := 84
	f32 := func() float64 {
		v := math.Float32frombits(binary.LittleEndian.Uint32(read(off, 4)))
		off += 4
		return float64(v)
	}
	vec := func() geom.Vector {
		x := f32()
		y := f32()
		z := f32()
		return geom.Vector{X: x, Y: y, Z: z}
	}

	var verts []stlVertex
	for i := uint32(0); i < n; i++ {
		tr := srf.STriangle{}

		// read the triangle normal
		tr.NA = vec()
		tr.NB = tr.NA
		tr.NC = tr.NA

		tr.A = vec()
		tr.B = vec()
		tr.C = vec()

		color := binary.LittleEndian.Uint16(read(off, 2))
		off += 2
		if color&0x8000 != 0 {
			tr.Meta.Red = uint8((color >> 7) & 0xf8)
			tr.Meta.Green = uint8((color >> 2) & 0xf8)
			tr.Meta.Blue = uint8(color << 3)
			tr.Meta.Alpha = 255
		} else {
			tr.Meta.Red = 90
			tr.Meta.Green = 120
			tr.Meta.Blue = 140
			tr.Meta.Alpha = 255
		}

		m.AddTriangle(tr)
		normal := tr.Normal().WithMagnitude(1.0)
		addUnique(&verts, tr.A, normal)
		addUnique(&verts, tr.B, normal)
		addUnique(&verts, tr.C, normal)
	}

	id := 1

	// add the STL origin and normals
	origin := newPoint(el, &id, geom.Vector{})
	newNormal(el, &id, geom.QuaternionFromBasis(geom.Vector{X: 1}, geom.Vector{Y: 1}), origin)
	newNormal(el, &id, geom.QuaternionFromBasis(geom.Vector{Y: 1}, geom.Vector{Z: 1}), origin)
	newNormal(el, &id, geom.QuaternionFromBasis(geom.Vector{Z: 1}, geom.Vector{X: 1}), origin)

	// C++ reads past the end of an empty list here.
	box := core.BBox{}
	if len(verts) > 0 {
		box.MinP = verts[0].p
		box.MaxP = verts[0].p
	}

	// determine the bounding box for all vertexes
	for i := 1; i < len(verts); i++ {
		box.Include(verts[i].p, 0)
	}

	lo, hi := box.MinP, box.MaxP
	var p [8]core.HEntity
	p[0] = newPoint(el, &id, geom.Vector{X: lo.X, Y: lo.Y, Z: lo.Z})
	p[1] = newPoint(el, &id, geom.Vector{X: hi.X, Y: lo.Y, Z: lo.Z})
	p[2] = newPoint(el, &id, geom.Vector{X: lo.X, Y: hi.Y, Z: lo.Z})
	p[3] = newPoint(el, &id, geom.Vector{X: hi.X, Y: hi.Y, Z: lo.Z})
	p[4] = newPoint(el, &id, geom.Vector{X: lo.X, Y: lo.Y, Z: hi.Z})
	p[5] = newPoint(el, &id, geom.Vector{X: hi.X, Y: lo.Y, Z: hi.Z})
	p[6] = newPoint(el, &id, geom.Vector{X: lo.X, Y: hi.Y, Z: hi.Z})
	p[7] = newPoint(el, &id, geom.Vector{X: hi.X, Y: hi.Y, Z: hi.Z})

	newLine(el, &id, p[0], p[1])
	newLine(el, &id, p[0], p[2])
	newLine(el, &id, p[3], p[1])
	newLine(el, &id, p[3], p[2])

	newLine(el, &id, p[4], p[5])
	newLine(el, &id, p[4], p[6])
	newLine(el, &id, p[7], p[5])
	newLine(el, &id, p[7], p[6])

	newLine(el, &id, p[0], p[4])
	newLine(el, &id, p[1], p[5])
	newLine(el, &id, p[2], p[6])
	newLine(el, &id, p[3], p[7])

	for i := range verts {
		// create point entities for edge vertexes
		if isEdgeVertex(&verts[i]) {
			addVertex(el, verts[i].p)
		}
	}

	// C++ keeps an EntityList sorted by handle.
	sort.SliceStable(*el, func(i, j int) bool { return (*el)[i].H.V < (*el)[j].H.V })
	return true
}
