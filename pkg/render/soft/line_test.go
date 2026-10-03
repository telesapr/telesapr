package soft

import (
	"bytes"
	"math"
	"math/rand/v2"
	"runtime"
	"slices"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/render"
	"github.com/telesapr/telesapr/pkg/srf"
)

// rasterLineFullScan is rasterLine as it was: every pixel of the
// bounding box tested, which the per-row column bounds must match.
func (r *Renderer) rasterLineFullScan(a, b *rvert, width float64) {
	dx, dy := b.x-a.x, b.y-a.y
	length := math.Hypot(dx, dy)
	if length == 0 || math.IsNaN(length) {
		return
	}
	ux, uy := dx/length, dy/length
	hw := width / 2
	maxCov := min(width, 1)
	ext := hw + 1
	minX := max(int(math.Floor(min(a.x, b.x)-ext)), 0)
	maxX := min(int(math.Ceil(max(a.x, b.x)+ext)), r.width-1)
	minY := max(int(math.Floor(min(a.y, b.y)-ext)), 0)
	maxY := min(int(math.Ceil(max(a.y, b.y)+ext)), r.height-1)
	for py := minY; py <= maxY; py++ {
		fy := float64(py) + 0.5 - a.y
		for px := minX; px <= maxX; px++ {
			fx := float64(px) + 0.5 - a.x
			along := fx*ux + fy*uy
			perp := math.Abs(fx*uy - fy*ux)
			cp := min(max(hw+0.5-perp, 0), maxCov)
			if cp <= 0 {
				continue
			}
			ca := min(max(min(along, length-along)+0.5, 0), 1)
			cov := cp * ca
			if cov <= 0 {
				continue
			}
			t := min(max(along/length, 0), 1)
			z := a.z + (b.z-a.z)*t
			col := r.cur.color
			col[3] *= cov
			r.fragment(px, py, z, col)
		}
	}
}

// TestRasterLineSameAsFullScan draws lines of every slope, length and
// sub-pixel position, some reaching off the frame, both ways.
func TestRasterLineSameAsFullScan(t *testing.T) {
	r := New()
	cam := testCamera(160, 120)
	rng := rand.New(rand.NewPCG(1, 2))
	var lines [][5]float64
	for range 4000 {
		x, y := rng.Float64()*200-20, rng.Float64()*160-20
		ang := rng.Float64() * 2 * math.Pi
		l := math.Pow(10, rng.Float64()*3-1)
		lines = append(lines, [5]float64{x, y, x + l*math.Cos(ang), y + l*math.Sin(ang), 0.5 + rng.Float64()*2.5})
	}
	// Nearly and exactly axis-aligned, and through pixel centres.
	for _, d := range []float64{0, 1e-12, 1e-7, 1e-3} {
		lines = append(lines,
			[5]float64{10.5, 20.5, 90.5, 20.5 + d, 1}, [5]float64{30.5, 5.5, 30.5 + d, 100.5, 1},
			[5]float64{3, 7, 150, 7 - d, 2}, [5]float64{-50, 60 + d, 300, 60, 3})
	}
	draw := func(line [5]float64, full bool) ([]byte, []float32) {
		frame(r, cam, func() {
			r.cur.colorWrite = true
			r.cur.depthWrite = true
			r.cur.color = [4]float64{1, 0.5, 0.25, 0.75}
			a := rvert{x: line[0], y: line[1], z: 0.25}
			b := rvert{x: line[2], y: line[3], z: 0.75}
			if full {
				r.rasterLineFullScan(&a, &b, line[4])
			} else {
				r.rasterLine(&a, &b, line[4])
			}
		})
		return bytes.Clone(r.frame.Pix), slices.Clone(r.depth)
	}
	blank, _ := draw([5]float64{-100, -100, -90, -100, 1}, false)
	drawn := 0
	for i, line := range lines {
		pn, dn := draw(line, false)
		pf, df := draw(line, true)
		if !bytes.Equal(pn, pf) || !slices.Equal(dn, df) {
			t.Fatalf("line %d %v differs from the full scan", i, line)
		}
		if !bytes.Equal(pn, blank) {
			drawn++
		}
	}
	if drawn < len(lines)/2 {
		t.Fatalf("only %d of %d lines drew anything", drawn, len(lines))
	}
}

// TestDrawEdgesParallelSameAsSerial checks that a big batch of lines,
// drawn in bands at once, gives the frame one thread gives, with
// overlapping, blended and stippled lines.
func TestDrawEdgesParallelSameAsSerial(t *testing.T) {
	if runtime.GOMAXPROCS(0) < 2 {
		t.Skip("needs two threads")
	}
	var el srf.SEdgeList
	for i := range 3000 {
		a := float64(i) * 0.61803398875
		x, y := 30*math.Sin(a*1.3), 20*math.Cos(a*0.7)
		dx, dy := 8*math.Cos(a*5.1), 8*math.Sin(a*5.1)
		el.AddEdge(v(x, y, math.Sin(a)), v(x+dx, y+dy, math.Cos(a)), 0, 0, 0)
	}
	draw := func() []byte {
		r := New()
		img := frame(r, testCamera(400, 300), func() {
			for _, s := range []render.Stroke{
				{Layer: render.LayerNormal, Color: core.NewRgbaColor(255, 200, 100, 180), Width: 1.5, Unit: render.UnitPX},
				{Layer: render.LayerNormal, Color: core.NewRgbaColor(100, 255, 100, 255), Width: 1, Unit: render.UnitPX,
					StipplePattern: core.StippleDash, StippleScale: 4},
			} {
				r.DrawEdges(&el, r.GetStroke(s))
			}
		})
		return bytes.Clone(img.Pix)
	}
	par := draw()
	old := runtime.GOMAXPROCS(1)
	ser := draw()
	runtime.GOMAXPROCS(old)
	if !bytes.Equal(par, ser) {
		t.Fatal("parallel lines differ from serial")
	}
}
