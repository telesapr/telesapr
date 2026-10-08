package soft

import (
	"path/filepath"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/persist/slvs"
	"github.com/telesapr/telesapr/pkg/render"
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/srf"
)

// fixtureDisplay loads a fixture and returns its last group's display
// mesh and outlines.
func fixtureDisplay(t testing.TB, name string) (*srf.SMesh, *srf.SOutlineList) {
	doc, err := slvs.LoadFile(filepath.Join("..", "..", "..", "testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	sk := doc.Sketch
	sk.GenerateAll()
	var g *sketch.Group
	for _, hg := range sk.GroupOrder {
		g = sk.Group(hg)
		g.GenerateDisplayItems(sk)
	}
	return &g.DisplayMesh, &g.DisplayOutlines
}

// fitCamera centres the mesh and scales it to fill most of the frame.
func fitCamera(c render.Camera, m *srf.SMesh) render.Camera {
	first := true
	var lo, hi geom.Vector
	for _, tr := range m.Triangles {
		for _, p := range []geom.Vector{tr.A, tr.B, tr.C} {
			if first {
				lo, hi, first = p, p, false
			}
			p.MakeMaxMin(&hi, &lo)
		}
	}
	c.Offset = lo.Plus(hi).ScaledBy(-0.5)
	c.Scale = 0.7 * min(c.Width, c.Height) / hi.Minus(lo).Magnitude()
	return c
}

func drawMeshScene(r *Renderer, m *srf.SMesh, ol *srf.SOutlineList) {
	front := r.InternFill(render.Fill{Layer: render.LayerNormal})
	r.DrawMesh(m, front, 0)
	edge := r.InternStroke(render.Stroke{Layer: render.LayerNormal, ZIndex: 1, Color: core.NewRGBA(255, 255, 255, 255),
		Width: 1.5, Unit: render.UnitPX})
	r.DrawOutlines(ol, edge, render.EmphasizedAndContour)
	hidden := r.InternStroke(render.Stroke{Layer: render.LayerOccluded, ZIndex: 1, Color: core.NewRGBA(120, 120, 120, 255),
		Width: 1, Unit: render.UnitPX, StipplePattern: core.StippleDash, StippleScale: 4})
	r.DrawOutlines(ol, hidden, render.EmphasizedAndContour)
}

func TestSoftMesh(t *testing.T) {
	t.Parallel()
	m, ol := fixtureDisplay(t, "group/boolean_tangent_fillet/normal.slvs")
	r := New()
	cam := fitCamera(isoCamera(300, 240), m)
	checkGolden(t, "mesh_iso", frame(r, cam, func() { drawMeshScene(r, m, ol) }))

	cam.Tangent = 2 / (cam.Scale * 100)
	checkGolden(t, "mesh_perspective", frame(r, cam, func() { drawMeshScene(r, m, ol) }))

	// Two-sided, as SolveSpace draws a mesh with its back faces in red.
	cam.Tangent = 0
	checkGolden(t, "mesh_twosided", frame(r, cam, func() {
		front := r.InternFill(render.Fill{Color: core.NewRGBA(160, 160, 160, 255)})
		back := r.InternFill(render.Fill{Color: core.NewRGBA(255, 0, 0, 255)})
		r.DrawMesh(m, front, back)
	}))
}

func BenchmarkSoftMesh1080p(b *testing.B) {
	m, ol := fixtureDisplay(b, "group/boolean_tangent_fillet/normal.slvs")
	r := New()
	cam := fitCamera(isoCamera(1920, 1080), m)
	b.ReportMetric(float64(len(m.Triangles)), "triangles")
	for b.Loop() {
		frame(r, cam, func() { drawMeshScene(r, m, ol) })
	}
}
