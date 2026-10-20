package gl

import (
	"unsafe"

	glapi "github.com/go-gl/gl/v2.1/gl"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
)

// vertexBatch collects the vertices of the primitive being drawn and
// sends them to GL as client-side arrays when it ends: one call for the
// run, where immediate mode is a cgo call per vertex and per normal,
// which is most of what a frame of a large mesh costs. Each vertex
// takes the normal, colour and texture coordinate current when it is
// added, as immediate mode gives it them, so the stream GL sees is the
// same.
type vertexBatch struct {
	verts []float64 // x, y, z
	norms []float64 // x, y, z
	cols  []float64 // r, g, b, a
	texs  []float64 // s, t

	normal [3]float64
	color  [4]float64
	tex    [2]float64

	// enabled says the arrays are enabled in GL, and set holds the
	// pointers GL has for them (vertices, normals, colours, texture
	// coordinates).
	enabled bool
	set     [4]unsafe.Pointer
}

// vertex adds a vertex with the current attributes.
func (b *vertexBatch) vertex(v geom.Vector) {
	b.verts = append(b.verts, v.X, v.Y, v.Z)
	b.norms = append(b.norms, b.normal[0], b.normal[1], b.normal[2])
	b.cols = append(b.cols, b.color[0], b.color[1], b.color[2], b.color[3])
	b.texs = append(b.texs, b.tex[0], b.tex[1])
}

// flush draws the vertices as primitives of mode, and empties the
// batch. GL reads client arrays during the call, so the slices are
// free to grow or be reused after it. The arrays stay enabled and
// their pointers set between runs, since every vertex goes through
// them: a run then costs one call, plus one per array whose slice
// moved.
func (b *vertexBatch) flush(mode uint32) {
	n := len(b.verts) / 3
	if n == 0 {
		return
	}
	if !b.enabled {
		glapi.EnableClientState(glapi.VERTEX_ARRAY)
		glapi.EnableClientState(glapi.NORMAL_ARRAY)
		glapi.EnableClientState(glapi.COLOR_ARRAY)
		glapi.EnableClientState(glapi.TEXTURE_COORD_ARRAY)
		b.enabled = true
	}
	if p := unsafe.Pointer(&b.verts[0]); p != b.set[0] {
		glapi.VertexPointer(3, glapi.DOUBLE, 0, p)
		b.set[0] = p
	}
	if p := unsafe.Pointer(&b.norms[0]); p != b.set[1] {
		glapi.NormalPointer(glapi.DOUBLE, 0, p)
		b.set[1] = p
	}
	if p := unsafe.Pointer(&b.cols[0]); p != b.set[2] {
		glapi.ColorPointer(4, glapi.DOUBLE, 0, p)
		b.set[2] = p
	}
	if p := unsafe.Pointer(&b.texs[0]); p != b.set[3] {
		glapi.TexCoordPointer(2, glapi.DOUBLE, 0, p)
		b.set[3] = p
	}
	glapi.DrawArrays(mode, 0, int32(n))
	b.verts, b.norms, b.cols, b.texs = b.verts[:0], b.norms[:0], b.cols[:0], b.texs[:0]
}

// vertex adds a vertex to the primitive being drawn (glVertex3d).
func (r *Renderer) vertex(v geom.Vector) { r.batch.vertex(v) }

// normal sets the normal the next vertices take (glNormal3d).
func (r *Renderer) normal(n geom.Vector) { r.batch.normal = [3]float64{n.X, n.Y, n.Z} }

// colorRGBA sets the colour the next vertices take (glColor4d).
func (r *Renderer) colorRGBA(c core.RGBA) {
	r.batch.color = [4]float64{float64(c.RedF()), float64(c.GreenF()), float64(c.BlueF()), float64(c.AlphaF())}
}

// texCoord sets the texture coordinate the next vertices take
// (glTexCoord2d).
func (r *Renderer) texCoord(s, t float64) { r.batch.tex = [2]float64{s, t} }
