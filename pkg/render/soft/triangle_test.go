package soft

import (
	"bytes"
	"math"
	"math/rand/v2"
	"slices"
	"testing"
)

// rasterTriangleFullScan is rasterTriangle with every pixel of the
// bounding box put to the three edge tests, which the run of pixels
// rasterTriangleRows finds in each row must match.
func (r *Renderer) rasterTriangleFullScan(a, b, c *rvert) {
	r.settleAll()
	area := (b.x-a.x)*(c.y-a.y) - (b.y-a.y)*(c.x-a.x)
	if area == 0 || math.IsNaN(area) {
		return
	}
	if area < 0 {
		b, c = c, b
		area = -area
	}
	minX := max(int(math.Floor(min(a.x, b.x, c.x))), 0)
	maxX := min(int(math.Ceil(max(a.x, b.x, c.x))), r.width-1)
	minY := max(int(math.Floor(min(a.y, b.y, c.y))), 0)
	maxY := min(int(math.Ceil(max(a.y, b.y, c.y))), r.height-1)
	type edge struct{ ox, oy, dx, dy float64 }
	edges := [3]edge{
		{b.x, b.y, c.x - b.x, c.y - b.y},
		{c.x, c.y, a.x - c.x, a.y - c.y},
		{a.x, a.y, b.x - a.x, b.y - a.y},
	}
	invArea := 1 / area
	for py := minY; py <= maxY; py++ {
		fy := float64(py) + 0.5
		for px := minX; px <= maxX; px++ {
			fx := float64(px) + 0.5
			var w [3]float64
			in := true
			for i, e := range edges {
				w[i] = e.dx*(fy-e.oy) - e.dy*(fx-e.ox)
				if w[i] < 0 || (w[i] == 0 && !edgeIncludes(e.dx, e.dy)) {
					in = false
				}
			}
			if !in {
				continue
			}
			l0, l1, l2 := w[0]*invArea, w[1]*invArea, w[2]*invArea
			z := l0*a.z + l1*b.z + l2*c.z
			q0, q1, q2 := l0*a.invW, l1*b.invW, l2*c.invW
			qs := 1 / (q0 + q1 + q2)
			var col [4]float64
			for k := range col {
				col[k] = (q0*a.col[k] + q1*b.col[k] + q2*c.col[k]) * qs
			}
			r.fragment(px, py, z, col)
		}
	}
}

// TestRasterTriangleSameAsFullScan draws triangles of every shape,
// size and sub-pixel position, some reaching off the frame, some with
// corners on pixel centres and sides along rows and columns, over one
// another so that the depth test decides too.
func TestRasterTriangleSameAsFullScan(t *testing.T) {
	t.Parallel()
	r := New()
	cam := testCamera(160, 120)
	rng := rand.New(rand.NewPCG(5, 6))
	vert := func(x, y float64) rvert {
		return rvert{x: x, y: y, z: rng.Float64(), invW: 0.5 + rng.Float64(),
			col: [4]float64{rng.Float64(), rng.Float64(), rng.Float64(), 0.4 + 0.6*rng.Float64()}}
	}
	var tris [][3]rvert
	for range 3000 {
		cx, cy := rng.Float64()*200-20, rng.Float64()*160-20
		s := math.Pow(10, rng.Float64()*2.5-1)
		var tr [3]rvert
		for i := range tr {
			tr[i] = vert(cx+s*(rng.Float64()-0.5), cy+s*(rng.Float64()-0.5))
		}
		tris = append(tris, tr)
	}
	for _, d := range []float64{0, 1e-12, 1e-7} {
		tris = append(tris,
			[3]rvert{vert(10.5, 20.5), vert(90.5, 20.5+d), vert(50.5, 60.5)},
			[3]rvert{vert(30.5, 5.5), vert(30.5+d, 100.5), vert(80, 50)},
			[3]rvert{vert(3, 7), vert(150, 7-d), vert(150, 90)})
	}
	draw := func(tr [3]rvert, under [3]rvert, full bool) ([]byte, []float32) {
		frame(r, cam, func() {
			r.cur.colorWrite, r.cur.depthWrite = true, true
			r.cur.depthFunc = depthLEqual
			for _, x := range [][3]rvert{under, tr} {
				if full {
					r.rasterTriangleFullScan(&x[0], &x[1], &x[2])
				} else {
					r.rasterTriangle(&x[0], &x[1], &x[2], fragColor)
				}
			}
		})
		r.settleAll()
		return bytes.Clone(r.frame.Pix), slices.Clone(r.depth)
	}
	for i, tr := range tris {
		under := tris[(i*7+3)%len(tris)]
		pn, dn := draw(tr, under, false)
		pf, df := draw(tr, under, true)
		if !bytes.Equal(pn, pf) || !slices.Equal(dn, df) {
			t.Fatalf("triangle %d %v differs from the full scan", i, tr)
		}
	}
}
