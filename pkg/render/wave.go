package render

import (
	"math"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
)

// waveKnot is a point of a wave's profile: at so many stipple scales
// along the line, off so many amplitudes across it.
type waveKnot struct{ at, off float64 }

// wave is the profile of a pattern drawn as a line that leaves the
// straight: its knots over one period, from 0 to period, both ends on
// the line.
type wave struct {
	period float64
	knots  []waveKnot
}

// waves are the patterns drawn across the line rather than in dashes,
// as the styles screen shows them: freehand a wave all along (~~~~),
// zigzag straight runs broken by one zigzag (__~__~).
var waves = map[core.StipplePattern]wave{
	core.StippleFreehand: {1, sineKnots(0, 1, 8)},
	core.StippleZigzag: {3, []waveKnot{
		{0, 0}, {2, 0}, {2.25, 1}, {2.75, -1}, {3, 0},
	}},
}

// sineKnots are n+1 knots of one period of a sine from at to at+length.
func sineKnots(at, length float64, n int) []waveKnot {
	knots := make([]waveKnot, 0, n+1)
	for i := range n + 1 {
		f := float64(i) / float64(n)
		knots = append(knots, waveKnot{at + f*length, math.Sin(2 * math.Pi * f)})
	}
	return knots
}

// offset is the profile's offset u stipple scales into a period.
func (w *wave) offset(u float64) float64 {
	u -= math.Floor(u/w.period) * w.period
	for i := 1; i < len(w.knots); i++ {
		k0, k1 := w.knots[i-1], w.knots[i]
		if u <= k1.at {
			if k1.at == k0.at {
				return k1.off
			}
			return k0.off + (k1.off-k0.off)*(u-k0.at)/(k1.at-k0.at)
		}
	}
	return 0
}

// Wave is the polyline that draws segment a-b of a stroke whose pattern
// is drawn across the line (freehand, zigzag), starting phase
// millimetres into the pattern, so that the edges of a curve carry it
// on; nil when the stroke is not such a pattern, or its period is under
// a pixel or not a number, and the segment is drawn straight. The wave
// swings across the line as the screen shows it, twice the line's
// width in pixels each way.
func (s *Stroke) Wave(c *Camera, a, b geom.Vector, phase float64) []geom.Vector {
	w, ok := waves[s.StipplePattern]
	if !ok {
		return nil
	}
	scale := s.StippleScaleMm(c)
	period := w.period * scale
	if !(period*c.Scale >= 1) || math.IsInf(scale, 0) {
		return nil
	}
	ab := b.Minus(a)
	length := ab.Magnitude()
	if !(length > 0) || math.IsInf(length, 0) {
		return nil
	}
	dir := ab.ScaledBy(1 / length)
	// Across the line in the plane of the screen; nothing for a line
	// seen end on.
	gn := c.ProjRight.Cross(c.ProjUp)
	across := ab.Cross(gn)
	across = across.Minus(gn.ScaledBy(gn.Dot(across)))
	if across.Magnitude() < geom.LengthEps*length {
		return nil
	}
	across = across.WithMagnitude(2 * max(s.WidthPx(c), 1) / c.Scale)

	phase -= math.Floor(phase/period) * period
	at := func(t float64) geom.Vector {
		return a.Plus(dir.ScaledBy(t)).Plus(across.ScaledBy(w.offset((phase + t) / scale)))
	}
	pts := []geom.Vector{at(0)}
	// The knots inside the segment, a period at a time; a period's last
	// knot is the next one's first.
	for base := 0.0; ; base += w.period {
		for _, k := range w.knots[:len(w.knots)-1] {
			t := (base+k.at)*scale - phase
			if t <= 0 {
				continue
			}
			if t >= length {
				return append(pts, at(length))
			}
			pts = append(pts, at(t))
		}
	}
}
