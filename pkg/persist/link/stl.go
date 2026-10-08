package link

import (
	"bytes"
	"cmp"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/srf"
)

// This file ports solvespace/src/importmesh.cpp, which links a binary
// STL triangle mesh, and reads text STL files too; obj.go reads OBJ
// files into the same mesh and entities.

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

// vertexIndex finds the vertices by cells of minPointDistance, so that
// a mesh's vertices are made unique in time linear in their number,
// not quadratic (roadmap 17.4): the vertex a point joins is the first
// within the distance, as a search through the list in order finds.
type vertexIndex struct {
	verts []stlVertex
	cells map[[3]int64][]int
}

func (vi *vertexIndex) cell(p geom.Vector) [3]int64 {
	return [3]int64{int64(math.Floor(p.X / minPointDistance)), int64(math.Floor(p.Y / minPointDistance)),
		int64(math.Floor(p.Z / minPointDistance))}
}

func (vi *vertexIndex) addUnique(p, n geom.Vector) {
	if vi.cells == nil {
		vi.cells = map[[3]int64][]int{}
	}
	c := vi.cell(p)
	found := -1
	for dx := int64(-1); dx <= 1; dx++ {
		for dy := int64(-1); dy <= 1; dy++ {
			for dz := int64(-1); dz <= 1; dz++ {
				for _, i := range vi.cells[[3]int64{c[0] + dx, c[1] + dy, c[2] + dz}] {
					if (found < 0 || i < found) && vi.verts[i].p.EqualsTol(p, minPointDistance) {
						found = i
					}
				}
			}
		}
	}
	if found < 0 {
		found = len(vi.verts)
		vi.verts = append(vi.verts, stlVertex{p: p})
		vi.cells[c] = append(vi.cells[c], found)
	}
	vi.verts[found].normal = append(vi.verts[found].normal, n)
}

// entities collects the entities a linked file gives its group, each
// numbered in the order it is made, so that a file reloaded gives the
// same handles.
type entities struct {
	list []core.Entity
	id   int
}

// pointEntity is a point entity with the number id; its type doesn't
// matter, since the linked group copies it.
func pointEntity(id int, p geom.Vector) core.Entity {
	en := core.Entity{
		Type:       core.EntityPointNCopy,
		Group:      core.HGroup{V: 462},
		ActPoint:   p,
		Style:      core.HStyle{V: core.StyleDatum},
		ActVisible: true}
	en.H = core.HEntity{V: uint32(id) + en.Group.V*65536}
	return en
}

// point adds a point with the next number.
func (b *entities) point(p geom.Vector) core.HEntity {
	en := pointEntity(b.id, p)
	b.id++
	b.list = append(b.list, en)
	return en.H
}

// vertex adds a point numbered by its place in the list, up to the
// bound on a mesh's points.
func (b *entities) vertex(v geom.Vector) {
	if len(b.list) < 15000 {
		b.list = append(b.list, pointEntity(len(b.list), v))
	}
}

// normal adds a normal at point p.
func (b *entities) normal(normal geom.Quaternion, p core.HEntity) {
	en := core.Entity{
		Type:      core.EntityNormalNCopy,
		Group:     core.HGroup{V: 472},
		ActNormal: normal,
		Style:     core.HStyle{V: core.StyleNormals}}
	// to be visible we need to add a point.
	en.Point[0] = p
	en.ActVisible = true

	b.id++
	en.H = core.HEntity{V: uint32(b.id) + en.Group.V*65536}
	b.list = append(b.list, en)
}

// line adds a construction line from p0 to p1.
func (b *entities) line(p0, p1 core.HEntity) {
	en := core.Entity{
		Type: core.EntityLineSegment}
	en.Point[0] = p0
	en.Point[1] = p1
	en.Group = core.HGroup{V: 493}
	en.Construction = true
	en.Style = core.HStyle{V: core.StyleConstruction}
	en.ActVisible = true

	en.H = core.HEntity{V: uint32(b.id) + en.Group.V*65536}
	b.id++
	b.list = append(b.list, en)
}

// linkStl reads an STL file, binary or text, into mesh m, with entities
// for its origin, its bounding box and the vertices on its edges.
func linkStl(path string, sk *sketch.Sketch, m *srf.SMesh) ([]core.Entity, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	var tris []srf.STriangle
	if isTextStl(data) {
		if tris, err = textStlTriangles(data); err != nil {
			sk.Tell(fmt.Sprintf("The STL file '%s' can't be read: %v.", path, err))
			return nil, false
		}
	} else {
		tris = binaryStlTriangles(data)
	}
	return linkMesh(tris, m), true
}

// meshGrey is the colour of a triangle a mesh file gives none.
var meshGrey = core.RGBA{Red: 90, Green: 120, Blue: 140, Alpha: 255}

// isTextStl reports whether an STL file is text: it starts with
// "solid", and is not the size a binary file of the triangle count in
// its header would be, since some binary files start so too.
func isTextStl(data []byte) bool {
	if !bytes.HasPrefix(bytes.TrimLeft(data, " \t\r\n"), []byte("solid")) {
		return false
	}
	if len(data) >= 84 {
		n := binary.LittleEndian.Uint32(data[80:84])
		if uint64(len(data)) == 84+50*uint64(n) {
			return false
		}
	}
	return true
}

// binaryStlTriangles are the triangles of a binary STL file, with their
// colours where the file gives them.
func binaryStlTriangles(data []byte) []srf.STriangle {
	// A short file reads as zeros.
	read := func(off, n int) []byte {
		b := make([]byte, n)
		if off < len(data) {
			copy(b, data[off:])
		}
		return b
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
	var tris []srf.STriangle
	for range n {
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
			tr.Meta.RGBA = meshGrey
		}
		tris = append(tris, tr)
	}
	return tris
}

// textStlTriangles are the facets of a text STL file: "facet normal n
// n n", "outer loop", a "vertex x y z" for each corner, "endloop",
// "endfacet". A facet of more than three corners is cut into a fan.
func textStlTriangles(data []byte) ([]srf.STriangle, error) {
	var tris []srf.STriangle
	var normal geom.Vector
	var corners []geom.Vector
	words := strings.Fields(string(data))
	num := func(i int) (float64, error) {
		if i >= len(words) {
			return 0, errors.New("the file ends inside a facet")
		}
		return strconv.ParseFloat(words[i], 64)
	}
	vec := func(i int) (geom.Vector, error) {
		x, err := num(i)
		if err != nil {
			return geom.Vector{}, err
		}
		y, err := num(i + 1)
		if err != nil {
			return geom.Vector{}, err
		}
		z, err := num(i + 2)
		return geom.Vector{X: x, Y: y, Z: z}, err
	}
	for i := 0; i < len(words); i++ {
		switch strings.ToLower(words[i]) {
		case "facet":
			corners = corners[:0]
			normal = geom.Vector{}
			if i+1 < len(words) && strings.EqualFold(words[i+1], "normal") {
				n, err := vec(i + 2)
				if err != nil {
					return nil, fmt.Errorf("a facet's normal: %w", err)
				}
				normal = n
				i += 4
			}
		case "vertex":
			p, err := vec(i + 1)
			if err != nil {
				return nil, fmt.Errorf("a vertex: %w", err)
			}
			corners = append(corners, p)
			i += 3
		case "endfacet":
			if len(corners) < 3 {
				return nil, fmt.Errorf("a facet with %d corners", len(corners))
			}
			for k := 2; k < len(corners); k++ {
				tris = append(tris, srf.STriangle{Meta: srf.STriMeta{RGBA: meshGrey},
					A: corners[0], B: corners[k-1], C: corners[k], NA: normal, NB: normal, NC: normal})
			}
		}
	}
	if len(tris) == 0 {
		return nil, errors.New("it has no facets")
	}
	return tris, nil
}

// linkMesh puts triangles into mesh m, with entities for the origin,
// the bounding box and the vertices on the mesh's edges.
func linkMesh(tris []srf.STriangle, m *srf.SMesh) []core.Entity {
	var vi vertexIndex
	for _, tr := range tris {
		m.AddTriangle(tr)
		normal := tr.Normal().WithMagnitude(1.0)
		vi.addUnique(tr.A, normal)
		vi.addUnique(tr.B, normal)
		vi.addUnique(tr.C, normal)
	}
	verts := vi.verts

	b := entities{id: 1}

	// add the STL origin and normals
	origin := b.point(geom.Vector{})
	b.normal(geom.QuaternionFromBasis(geom.Vector{X: 1}, geom.Vector{Y: 1}), origin)
	b.normal(geom.QuaternionFromBasis(geom.Vector{Y: 1}, geom.Vector{Z: 1}), origin)
	b.normal(geom.QuaternionFromBasis(geom.Vector{Z: 1}, geom.Vector{X: 1}), origin)

	// An empty list has no first vertex.
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
	p[0] = b.point(geom.Vector{X: lo.X, Y: lo.Y, Z: lo.Z})
	p[1] = b.point(geom.Vector{X: hi.X, Y: lo.Y, Z: lo.Z})
	p[2] = b.point(geom.Vector{X: lo.X, Y: hi.Y, Z: lo.Z})
	p[3] = b.point(geom.Vector{X: hi.X, Y: hi.Y, Z: lo.Z})
	p[4] = b.point(geom.Vector{X: lo.X, Y: lo.Y, Z: hi.Z})
	p[5] = b.point(geom.Vector{X: hi.X, Y: lo.Y, Z: hi.Z})
	p[6] = b.point(geom.Vector{X: lo.X, Y: hi.Y, Z: hi.Z})
	p[7] = b.point(geom.Vector{X: hi.X, Y: hi.Y, Z: hi.Z})

	b.line(p[0], p[1])
	b.line(p[0], p[2])
	b.line(p[3], p[1])
	b.line(p[3], p[2])

	b.line(p[4], p[5])
	b.line(p[4], p[6])
	b.line(p[7], p[5])
	b.line(p[7], p[6])

	b.line(p[0], p[4])
	b.line(p[1], p[5])
	b.line(p[2], p[6])
	b.line(p[3], p[7])

	for i := range verts {
		// create point entities for edge vertexes
		if isEdgeVertex(&verts[i]) {
			b.vertex(verts[i].p)
		}
	}

	// The entities are kept sorted by handle.
	slices.SortStableFunc(b.list, func(a, b core.Entity) int { return cmp.Compare(a.H.V, b.H.V) })
	return b.list
}
