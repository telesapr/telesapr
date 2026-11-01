package webgl

import (
	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/render"
	"github.com/telesapr/telesapr/pkg/srf"
)

// DrawMesh draws a lit mesh: with a back fill, its back faces in that
// colour (two-sided lighting); an empty fill colour means each
// triangle's own. A back fill of no colour draws the back faces in the
// front's, which differs from the other renderers only for a front
// fill of a colour with a back fill of none.
func (r *Renderer) DrawMesh(m *srf.SMesh, hcfFront, hcfBack render.HFill) {
	front := r.Fill(hcfFront)
	var backColor core.RGBA
	if hcfBack != 0 {
		back := r.Fill(hcfBack)
		backColor = back.Color
		if front.Layer != back.Layer || front.ZIndex != back.ZIndex {
			panic("webgl: frontFill and backFill should belong to the same depth range")
		}
		if front.Pattern != back.Pattern {
			panic("webgl: frontFill and backFill should have the same pattern")
		}
	}

	p := pipe{}.withLayer(front.Layer, front.ZIndex)
	p.pattern = front.Pattern
	p.mode = modeLit
	p.twoSide = hcfBack != 0
	if !backColor.IsEmpty() {
		p.useBack, p.backCol = true, colorF32(backColor)
	}

	if r.lighting.Shaded {
		r.drawMeshShaded(m, front.Color, p)
		return
	}
	r.setPipe(p)
	for i := range m.Triangles {
		r.meshTriangle(&m.Triangles[i], front.Color)
	}
}

// meshTriangle adds a triangle of a lit mesh with its vertex normals,
// or its own where it has none, in color or if that is empty its own
// colour.
func (r *Renderer) meshTriangle(tr *srf.STriangle, color core.RGBA) {
	if color.IsEmpty() {
		color = tr.Meta.RGBA
	}
	r.batch.color = colorF32(color)
	if tr.NA.EqualsExactly(geom.Vector{}) {
		r.batch.setNormal(tr.Normal().WithMagnitude(1))
		r.batch.tri(tr.A, tr.B, tr.C)
		return
	}
	r.batch.setNormal(tr.NA)
	r.batch.vertex(tr.A)
	r.batch.setNormal(tr.NB)
	r.batch.vertex(tr.B)
	r.batch.setNormal(tr.NC)
	r.batch.vertex(tr.C)
}

// drawMeshShaded draws the mesh with render.Shade's lighting, a run for
// each material in the order the materials first appear, the triangles
// in their order within each.
func (r *Renderer) drawMeshShaded(m *srf.SMesh, color core.RGBA, p pipe) {
	p.mode = modeShaded
	runs := map[render.Material][]int{}
	var order []render.Material
	var lastFace uint32
	var lastMat render.Material
	haveLast := false
	for i := range m.Triangles {
		mat := render.Materials["plastic"]
		if r.materials != nil {
			face := uint32(m.Triangles[i].Meta.Face)
			if !haveLast || face != lastFace {
				lastFace, lastMat, haveLast = face, r.materials(face), true
			}
			mat = lastMat
		}
		if _, ok := runs[mat]; !ok {
			order = append(order, mat)
		}
		runs[mat] = append(runs[mat], i)
	}
	for _, mat := range order {
		p.mat = mat
		r.setPipe(p)
		for _, i := range runs[mat] {
			r.meshTriangle(&m.Triangles[i], color)
		}
	}
}
