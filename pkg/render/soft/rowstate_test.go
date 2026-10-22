package soft

import (
	"bytes"
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/render"
)

// TestRestoreLayerRowsSameAsWhole draws frames as the window does with
// its persistent layer: the layer put back, then lines and quads over a
// few rows, the depth cleared and more drawn in front. Restoring only
// the rows drawn on since must give the frame and depth a whole copy
// gives, frame after frame, and after the layer is saved again.
func TestRestoreLayerRowsSameAsWhole(t *testing.T) {
	t.Parallel()
	cam := testCamera(240, 180)
	persistent := func(r *Renderer, shift float64) {
		f := r.InternFill(render.Fill{Layer: render.LayerNormal, Color: blue})
		r.DrawQuad(v(-20+shift, -15, 0), v(-20+shift, 15, 0), v(10+shift, 15, 5), v(10+shift, -15, 5), f)
		s := r.InternStroke(render.Stroke{Layer: render.LayerNormal, Color: white, Width: 1.5, Unit: render.UnitPX})
		for i := range 20 {
			y := -15 + 1.5*float64(i)
			r.DrawLine(v(-22, y, 2), v(22, y+3, -2), s)
		}
	}
	rows := New()
	whole := New()
	var rowsLayer, wholeLayer Layer
	rng := rand.New(rand.NewPCG(3, 4))
	for k := range 40 {
		ox, oy := rng.Float64()*36-18, rng.Float64()*26-13
		saveAgain := k%13 == 5
		draw := func(r *Renderer, l *Layer, forget bool) ([]byte, []float32) {
			frame(r, cam, func() {
				if k == 0 || saveAgain {
					persistent(r, float64(k%3))
					r.SaveLayer(l)
				} else {
					if forget {
						r.layer = nil
					}
					if !r.RestoreLayer(l) {
						t.Fatal("the layer wasn't put back")
					}
				}
				s := r.InternStroke(render.Stroke{Layer: render.LayerNormal, Color: red, Width: 2, Unit: render.UnitPX})
				r.DrawLine(v(ox-4, oy, 10), v(ox+4, oy+1, -10), s)
				g := r.InternFill(render.Fill{Layer: render.LayerNormal, ZIndex: 1,
					Color: core.NewRGBA(40, 200, 40, 140)})
				r.DrawQuad(v(ox, oy, 3), v(ox, oy+2, 3), v(ox+6, oy+2, 3), v(ox+6, oy, 3), g)
				// The overlay: the depth cleared, a toolbar's worth drawn.
				r.SetCamera(cam)
				front := r.InternStroke(render.Stroke{Layer: render.LayerFront, Color: black, Width: 1, Unit: render.UnitPX})
				r.DrawLine(v(-24, -17+float64(k%5), 0), v(24, -17+float64(k%5), 0), front)
			})
			r.settleAll()
			return bytes.Clone(r.frame.Pix), slices.Clone(r.depth)
		}
		pr, dr := draw(rows, &rowsLayer, false)
		pw, dw := draw(whole, &wholeLayer, true)
		if !bytes.Equal(pr, pw) || !slices.Equal(dr, dw) {
			t.Fatalf("frame %d: rows restored differ from the whole layer copied", k)
		}
	}
}
