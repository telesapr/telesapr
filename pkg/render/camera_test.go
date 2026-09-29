package render

import (
	"fmt"
	"github.com/spacesolve/spacesolve/pkg/internal/golden"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spacesolve/spacesolve/pkg/geom"
	"github.com/spacesolve/spacesolve/pkg/srf"
)

// TestCameraVsCpp compares the camera's projections and the object
// picker with C++ render.cpp's, bit for bit (testdata/render/
// camera.golden, from scripts/regen-render-golden.sh).
func TestCameraVsCpp(t *testing.T) {
	want, err := os.ReadFile(filepath.Join("..", "..", "testdata", "render", "camera.golden"))
	if err != nil {
		t.Fatal(err)
	}
	var cams [3]Camera
	cams[0].LoadIdentity()
	cams[0].Width, cams[0].Height, cams[0].PixelRatio, cams[0].Scale = 600, 600, 1, 5
	cams[1] = cams[0]
	cams[1].Offset = geom.Vector{X: -3.25, Y: 7.5, Z: 1.125}
	cams[1].ProjRight = geom.Vector{X: 0.8, Y: 0.1, Z: -0.3}
	cams[1].ProjUp = geom.Vector{X: 0.2, Y: 0.9, Z: 0.4}
	cams[1].NormalizeProjectionVectors()
	cams[1].Scale = 2.75
	cams[1].GridFit = true
	cams[2] = cams[1]
	cams[2].Tangent = 0.3 / 1e3
	cams[2].GridFit = false

	pts := []geom.Vector{{}, {X: 10, Y: -20, Z: 30}, {X: -7.5, Y: 3.25, Z: -12.125}, {X: 123.4, Y: 56.7, Z: -89.1}}
	var b strings.Builder
	p := func(v geom.Vector) { fmt.Fprintf(&b, " %x %x %x", v.X, v.Y, v.Z) }
	for i := range cams {
		c := &cams[i]
		b.WriteString("camera")
		p(c.ProjRight)
		p(c.ProjUp)
		b.WriteString("\n")
		for _, pt := range pts {
			r, w := c.ProjectPoint4(pt)
			p3 := c.ProjectPoint3(pt)
			b.WriteString("pt")
			p(r)
			fmt.Fprintf(&b, " %x", w)
			p(p3)
			p(c.UnProjectPoint3(p3))
			p(c.UnProjectPoint(c.ProjectPoint(pt)))
			p(c.AlignToPixelGrid(pt))
			p(c.VectorFromProjs(pt))
			b.WriteString("\n")
		}
		bz := srf.SBezierFrom4(pts[0], pts[1], pts[2], pts[3])
		bz.Weight[1], bz.Weight[2] = 0.75, 1.5
		pb := c.ProjectBezier(bz)
		b.WriteString("bezier")
		for i := 0; i <= pb.Degree; i++ {
			p(pb.Ctrl[i])
			fmt.Fprintf(&b, " %x", pb.Weight[i])
		}
		b.WriteString("\n")

		op := &ObjectPicker{Camera: *c, SelRadius: 10}
		hcs := op.GetStroke(Stroke{Width: 3, ZIndex: 2, Unit: UnitPX})
		hcf := op.GetFill(Fill{ZIndex: 1})
		for i := 0; i < 6; i++ {
			op.Point = geom.Point2d{U: -40.0 + 17.5*float64(i), V: 25.0 - 11.25*float64(i)}
			hit := op.Pick(func() {
				op.DrawLine(pts[0], pts[1], hcs)
				op.DrawPoint(pts[2], hcs)
				op.DrawQuad(pts[0], pts[1], pts[2], pts[3], hcf)
				op.DrawVectorText("abc", 2.0, pts[0], geom.Vector{X: 1}, geom.Vector{Y: 1}, hcs)
			})
			h := 0
			if hit {
				h = 1
			}
			fmt.Fprintf(&b, "pick %d %x %x %d\n", h, op.MinDistance, op.MinDepth, op.MaxZIndex)
		}
	}
	golden.CompareDump(t, string(want), b.String())
}
