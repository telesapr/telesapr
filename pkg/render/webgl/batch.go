package webgl

import "github.com/telesapr/telesapr/pkg/geom"

// vertexFloats is the size of a vertex in the stream: position,
// normal, colour (RGBA) and texture coordinate.
const vertexFloats = 3 + 3 + 4 + 2

// batch collects the triangles of the run being drawn as a stream of
// vertices, each taking the normal, colour and texture coordinate
// current when it is added, as GL's immediate mode gives them. Quads
// and fans are cut into triangles on the way in, so the device draws
// one primitive type.
type batch struct {
	v []float32

	normal [3]float32
	color  [4]float32
	tex    [2]float32
}

// vertices is the number of vertices in the stream.
func (b *batch) vertices() int { return len(b.v) / vertexFloats }

// reset empties the stream and keeps its memory.
func (b *batch) reset() { b.v = b.v[:0] }

func (b *batch) setNormal(n geom.Vector) {
	b.normal = [3]float32{float32(n.X), float32(n.Y), float32(n.Z)}
}

func (b *batch) setTex(s, t float64) { b.tex = [2]float32{float32(s), float32(t)} }

// vertex adds a vertex with the current attributes.
func (b *batch) vertex(p geom.Vector) {
	b.v = append(b.v, float32(p.X), float32(p.Y), float32(p.Z),
		b.normal[0], b.normal[1], b.normal[2],
		b.color[0], b.color[1], b.color[2], b.color[3],
		b.tex[0], b.tex[1])
}

// tri adds a triangle.
func (b *batch) tri(p, q, r geom.Vector) {
	b.vertex(p)
	b.vertex(q)
	b.vertex(r)
}

// quad adds the quad p, q, r, s as two triangles.
func (b *batch) quad(p, q, r, s geom.Vector) {
	b.tri(p, q, r)
	b.tri(p, r, s)
}

// fan adds the convex polygon pts as a fan of triangles about its
// first point.
func (b *batch) fan(pts []geom.Vector) {
	for i := 2; i < len(pts); i++ {
		b.tri(pts[0], pts[i-1], pts[i])
	}
}
