package sketch_test

import (
	"fmt"
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/internal/fixtures"
	"github.com/telesapr/telesapr/pkg/internal/sweep"
	"github.com/telesapr/telesapr/pkg/script/api"
)

// nearOverlap is, for a corner between the bands a and b at the point
// at, the part of the overlap of a carried on past it and b carried back
// before it that is joined to the corner: the overlap's connected piece
// holding the corner, found on a raster of cells of side cell over the
// square of half side probeHalf about the origin. Where an arc carried on turns
// back over the other piece far from the corner, the overlap has a
// second piece there, no part of the corner's fill; held says whether a
// point lies in the near piece.
// probeHalf is the half side of the square the probe looks in: the
// closed paths reach 65 from the origin.
const probeHalf = 90.0

type nearOverlap struct {
	a, b band2
	w    float64
	cell float64
	n    int
	near []bool // the corner's piece, grown by two cells
}

func newNearOverlap(a, b band2, w float64, at [2]float64) *nearOverlap {
	no := &nearOverlap{a: a, b: b, w: w, cell: 0.1}
	no.n = int(2 * probeHalf / no.cell)
	half := probeHalf
	n := no.n
	over := make([]bool, n*n)
	centre := func(i, j int) [2]float64 {
		return [2]float64{-half + (float64(i)+0.5)*no.cell, -half + (float64(j)+0.5)*no.cell}
	}
	for j := range n {
		for i := range n {
			over[j*n+i] = no.overlap(centre(i, j))
		}
	}
	seen := make([]bool, n*n)
	si, sj := int((at[0]+half)/no.cell), int((at[1]+half)/no.cell)
	// The corner is on the overlap's edge: start from the nearest cell
	// of it.
	best, bi, bj := math.Inf(1), -1, -1
	for dj := -4; dj <= 4; dj++ {
		for di := -4; di <= 4; di++ {
			i, j := si+di, sj+dj
			if i < 0 || j < 0 || i >= n || j >= n || !over[j*n+i] {
				continue
			}
			if d := float64(di*di + dj*dj); d < best {
				best, bi, bj = d, i, j
			}
		}
	}
	comp := make([]bool, n*n)
	if bi >= 0 {
		stack := []int{bj*n + bi}
		seen[bj*n+bi] = true
		for len(stack) > 0 {
			c := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			comp[c] = true
			i, j := c%n, c/n
			for _, d := range [][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
				ii, jj := i+d[0], j+d[1]
				if ii < 0 || jj < 0 || ii >= n || jj >= n {
					continue
				}
				k := jj*n + ii
				if over[k] && !seen[k] {
					seen[k] = true
					stack = append(stack, k)
				}
			}
		}
	}
	no.near = make([]bool, n*n)
	for j := range n {
		for i := range n {
			if !comp[j*n+i] {
				continue
			}
			for dj := -2; dj <= 2; dj++ {
				for di := -2; di <= 2; di++ {
					ii, jj := i+di, j+dj
					if ii >= 0 && jj >= 0 && ii < n && jj < n {
						no.near[jj*n+ii] = true
					}
				}
			}
		}
	}
	return no
}

func (no *nearOverlap) overlap(x [2]float64) bool {
	return no.a.holds(x, no.w, no.a.size(), no.a.size()+no.a.carry()) && no.b.holds(x, no.w, -no.b.carry(), 0)
}

func (no *nearOverlap) held(x [2]float64) bool {
	if !no.overlap(x) {
		return false
	}
	i, j := int((x[0]+probeHalf)/no.cell), int((x[1]+probeHalf)/no.cell)
	return i >= 0 && j >= 0 && i < no.n && j < no.n && no.near[j*no.n+i]
}

// bandEnd is where the band ends, in the ZX plane.
func bandEnd(b band2) [2]float64 {
	if !b.arc {
		return [2]float64{b.p[0] + b.d[0]*b.l, b.p[1] + b.d[1]*b.l}
	}
	s, c := math.Sincos(b.an)
	// About +y, (x, z) turns as z→x: x' = x cos + z sin, z' = -x sin + z cos.
	r := [2]float64{b.r0[0]*c + b.r0[1]*s, -b.r0[0]*s + b.r0[1]*c}
	return [2]float64{b.c[0] + r[0], b.c[1] + r[1]}
}

// cornerFillRegions are, for the bands of a path round its corners
// (closed, each band's end a corner; open, the corners between), the
// area of the region a profile reaching w sweeps with each corner filled
// by its overlap's near piece, and, where whole is asked for, the same
// with the whole overlap.
func cornerFillRegions(bands []band2, closed bool, w float64, rows int, wantWhole bool) (near, whole float64) {
	k := len(bands) - 1
	if closed {
		k = len(bands)
	}
	overs := make([]*nearOverlap, k)
	for i := range k {
		a, b := bands[i], bands[(i+1)%len(bands)]
		overs[i] = newNearOverlap(a, b, w, bandEnd(a))
	}
	piece := func(x [2]float64) bool {
		for _, b := range bands {
			if b.holds(x, w, 0, b.size()) {
				return true
			}
		}
		return false
	}
	near = regionAreaIn(func(x [2]float64) bool {
		if piece(x) {
			return true
		}
		for _, o := range overs {
			if o.held(x) {
				return true
			}
		}
		return false
	}, rows, probeHalf)
	if !wantWhole {
		return near, 0
	}
	whole = regionAreaIn(func(x [2]float64) bool {
		if piece(x) {
			return true
		}
		for _, o := range overs {
			if o.overlap(x) {
				return true
			}
		}
		return false
	}, rows, probeHalf)
	return near, whole
}

// Sharp corners beside arcs, open paths and closed, with the bars: each
// sweep made is the region whose corners are filled only by the piece
// of their overlap at the corner, within 0.3% (the rows, the raster and
// the mesh's chords). Carried a quarter turn, an arc can come back over
// the other piece carried on, far from the corner (a corner of 150°
// beside an arc of radius 15 or 20 turning against it): that piece of
// the overlap is no part of the sweep, and the solid holds none of it.
// Of the 291 made of the whole grids on 2026-11-05, 60 have such a piece
// and none holds it.
func TestSweepCornerFillNear(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("each point sweeps and integrates the region: a second or two")
	}
	check := func(d *api.Doc, path []api.Entity, bands []band2, closed bool, profile float64) error {
		s := d.Features.Sweep(path, "")
		if s.Problem() != "" {
			return nil
		}
		r := s.Analyze().Check()
		if !r.Sound() {
			return fmt.Errorf("made unsound: %s", r.String())
		}
		w, h := 1.0, 4.0
		if profile == 2 {
			w, h = 2, 2
		}
		near, _ := cornerFillRegions(bands, closed, w, 2400, false)
		if math.Abs(r.Volume-h*near) > 3e-3*h*near {
			_, whole := cornerFillRegions(bands, closed, w, 2400, true)
			return fmt.Errorf("volume %.3f, want %.3f (with the whole overlaps %.3f)", r.Volume, h*near, h*whole)
		}
		return nil
	}
	bar := func(d *api.Doc, profile float64) {
		d.Features.SketchIn(d.Workplane("XY"), "profile")
		if profile == 0 {
			d.Sketch.Rect(api.V(-1, -2, 0), api.V(1, 2, 0))
		} else {
			d.Sketch.Rect(api.V(-2, -1, 0), api.V(2, 1, 0))
		}
		step(d)
	}
	sample := 12
	if fixtures.RaceDetector {
		sample = 3
	}
	open := sweep.Grid{
		Axes: []sweep.Axis{
			sweep.Bools("arcFirst"),
			sweep.Floats("corner", -170, -150, 150, 170),
			sweep.Floats("turn", -100, -30, 30, 100),
			sweep.Floats("lead", 10, 25),
			sweep.Floats("rad", 4, 12, 15, 20),
			sweep.Floats("profile", 0, 2),
		},
		Sample: sample,
	}
	open.Run(func(p sweep.Point) error {
		af, corner, turn, lead, rad := p.Bool("arcFirst"), p.Float("corner"), p.Float("turn"), p.Float("lead"), p.Float("rad")
		d := api.New()
		path := arcCornerPath(d, af, corner, turn, lead, rad)
		bar(d, p.Float("profile"))
		return check(d, path, arcCornerBands(af, corner, turn, lead, rad), false, p.Float("profile"))
	}).Require(t)
	closed := sweep.Grid{
		Axes: []sweep.Axis{
			sweep.Floats("corner", -170, -150, 150, 170),
			sweep.Floats("turn", -100, -60, -30, -10, 10, 30),
			sweep.Floats("rad", 4, 12, 20),
			sweep.Floats("lead", 20, 30),
			sweep.Floats("profile", 0, 2),
		},
		Sample: sample,
	}
	closed.Run(func(p sweep.Point) error {
		c := closedCornerCase{p.Float("corner"), p.Float("turn"), p.Float("lead"), p.Float("rad"), p.Float("profile")}
		if _, _, _, _, _, ok := closedCornerPoints(c.corner, c.turn, c.lead, c.rad); !ok {
			return nil
		}
		d, path := c.build(0)
		return check(d, path, closedCornerBands(c.corner, c.turn, c.lead, c.rad), true, c.profile)
	}).Require(t)
}
