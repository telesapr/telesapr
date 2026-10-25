package soft

import (
	"bytes"
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/render"
)

// A stroke with nothing to dash by is drawn continuous: a stipple scale
// a file or a script can give that is zero, negative, not a number,
// infinite or under a pixel, for dashes and waves alike. The dash walk
// steps by the scale, so a negative one never finished the frame, and
// the others drew nothing.
func TestDashedLineWithNoDashes(t *testing.T) {
	t.Parallel()
	r := New()
	cam := testCamera(120, 40)
	draw := func(p core.StipplePattern, scale float64) []byte {
		img := frame(r, cam, func() {
			hcs := r.InternStroke(render.Stroke{Color: green, Width: 1.5, Unit: render.UnitPX,
				StipplePattern: p, StippleScale: scale})
			r.DrawLine(v(-10, 0, 0), v(10, 0, 0), hcs)
		})
		// The renderer reuses its frame.
		return bytes.Clone(img.Pix)
	}
	want := draw(core.StippleContinuous, 6)
	if countLit(want) == 0 {
		t.Fatal("the continuous line drew nothing")
	}
	cases := []struct {
		p     core.StipplePattern
		scale float64
	}{
		{core.StippleZigzag, 0},
		{core.StippleFreehand, -1},
		{core.StippleDash, 0},
		{core.StippleDash, -1},
		{core.StippleDot, -6},
		{core.StippleDash, math.NaN()},
		{core.StippleDash, math.Inf(1)},
		{core.StippleDashDot, 1e-9},
	}
	for _, c := range cases {
		if got := draw(c.p, c.scale); !bytes.Equal(got, want) {
			t.Errorf("%s at scale %g: %d pixels lit, the continuous line %d",
				core.StipplePatternName(c.p), c.scale, countLit(got), countLit(want))
		}
	}
}

// Freehand is drawn as a wave all along the line and zigzag as straight
// runs broken by a zigzag, as the styles screen shows them, swinging
// twice the line's width across it; they were drawn as short dashes,
// and as nothing.
func TestWavyLines(t *testing.T) {
	t.Parallel()
	r := New()
	cam := testCamera(140, 40)
	for _, p := range []core.StipplePattern{core.StippleFreehand, core.StippleZigzag} {
		img := frame(r, cam, func() {
			hcs := r.InternStroke(render.Stroke{Color: green, Width: 1.5, Unit: render.UnitPX,
				StipplePattern: p, StippleScale: 6})
			r.DrawLine(v(-10, 0, 0), v(10, 0, 0), hcs)
		})
		b := img.Bounds()
		rows, longest := 0, 0
		for y := range b.Dy() {
			lit, run := 0, 0
			for x := range b.Dx() {
				if img.Pix[img.PixOffset(x, y)+1] >= 100 {
					lit++
					run++
					longest = max(longest, run)
				} else {
					run = 0
				}
			}
			if lit > 0 {
				rows++
			}
		}
		// 3 px each way of a line 1.5 px wide.
		if rows < 6 {
			t.Errorf("%s: drawn over %d rows, want a wave across 6 or more",
				core.StipplePatternName(p), rows)
		}
		// Zigzag's straight runs are two thirds of its period of 18 px;
		// the wave has none.
		if straight := longest >= 10; straight != (p == core.StippleZigzag) {
			t.Errorf("%s: longest run along a row %d px", core.StipplePatternName(p), longest)
		}
	}
}

// The dots of a dotted line are as wide as the line in pixels: they
// were given its width in millimetres, and so drawn a fraction of a
// pixel long and nearly invisible.
func TestDottedLineDots(t *testing.T) {
	t.Parallel()
	r := New()
	cam := testCamera(140, 40)
	for _, width := range []float64{1.5, 6} {
		img := frame(r, cam, func() {
			hcs := r.InternStroke(render.Stroke{Color: green, Width: width, Unit: render.UnitPX,
				StipplePattern: core.StippleDot, StippleScale: 6})
			r.DrawLine(v(-10, 0, 0), v(10, 0, 0), hcs)
		})
		// 100 pixels of line, a dot every 3.
		dots, lit := 0, false
		y := img.Bounds().Dy() / 2
		for x := range img.Bounds().Dx() {
			on := img.Pix[img.PixOffset(x, y)+1] >= 100
			if on && !lit {
				dots++
			}
			lit = on
		}
		if width < 3 && (dots < 25 || dots > 40) {
			t.Errorf("width %g: %d dots on the line's row, want about 33", width, dots)
		}
		// Wide dots run together into a line of discs.
		if n := countLit(img.Pix); n < int(100*width*0.8) {
			t.Errorf("width %g: %d pixels lit, want discs of the line's width", width, n)
		}
	}
}

// countLit counts the pixels whose green is at least half on.
func countLit(pix []byte) int {
	n := 0
	for i := 1; i < len(pix); i += 4 {
		if pix[i] >= 100 {
			n++
		}
	}
	return n
}
