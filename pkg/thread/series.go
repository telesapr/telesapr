package thread

import (
	"fmt"
	"math"
	"slices"
	"strings"
)

// Series is a family of standard threads: what a list of sizes to
// pick from shows together.
type Series int

const (
	// SeriesMetric is ISO 261's M threads, coarse and fine.
	SeriesMetric Series = iota
	// SeriesUnified is ASME B1.1's UNC and UNF threads.
	SeriesUnified
	// SeriesTrapezoidal is ISO 2904's Tr threads.
	SeriesTrapezoidal
	// SeriesWhitworth is BS 84's BSW and BSF threads.
	SeriesWhitworth
	// SeriesPipe is ISO 228-1's parallel pipe threads (BSP, G).
	SeriesPipe
	// SeriesACME is ASME B1.5's general purpose ACME threads.
	SeriesACME
)

// seriesNames are the series' names in scripts and on the screen.
var seriesNames = [...]string{
	SeriesMetric: "metric", SeriesUnified: "unified", SeriesTrapezoidal: "trapezoidal",
	SeriesWhitworth: "Whitworth", SeriesPipe: "pipe", SeriesACME: "ACME",
}

func (sr Series) String() string {
	if sr >= 0 && int(sr) < len(seriesNames) {
		return seriesNames[sr]
	}
	return fmt.Sprintf("Series(%d)", int(sr))
}

// AllSeries are the series, in the order a list shows them.
func AllSeries() []Series {
	return []Series{SeriesMetric, SeriesUnified, SeriesTrapezoidal, SeriesWhitworth, SeriesPipe, SeriesACME}
}

// ParseSeries is the series of a name, as Series.String writes it, in
// any letter case.
func ParseSeries(name string) (Series, error) {
	for sr, n := range seriesNames {
		if strings.EqualFold(strings.TrimSpace(name), n) {
			return Series(sr), nil
		}
	}
	return 0, fmt.Errorf("unknown thread series %q: %s", name, strings.Join(seriesNames[:], ", "))
}

// SeriesOf is the series a thread's form and size belong to: a
// Whitworth thread of a pipe size is a pipe thread.
func SeriesOf(s Spec) Series {
	switch s.Form {
	case FormUnified:
		return SeriesUnified
	case FormTrapezoidal:
		return SeriesTrapezoidal
	case FormACME:
		return SeriesACME
	case FormWhitworth:
		if _, ok := pipeSize(s.Diameter); ok {
			return SeriesPipe
		}
		return SeriesWhitworth
	}
	return SeriesMetric
}

// Sizes are the series' standard sizes, smallest first, each at the
// pitch a designation without one gives it: the coarse metric and
// unified pitches, the preferred trapezoidal one, BSW's for Whitworth.
func (sr Series) Sizes() []Spec {
	var out []Spec
	add := func(f Form, d, pitch float64) {
		out = append(out, Spec{Form: f, Diameter: d, Pitch: pitch})
	}
	switch sr {
	case SeriesMetric:
		for _, m := range metricCoarse {
			add(FormMetric, m.d, m.pitch)
		}
	case SeriesUnified:
		for _, u := range unified {
			tpi := u.unc
			if tpi == 0 {
				tpi = u.unf
			}
			add(FormUnified, u.d*mmPerInch, mmPerInch/tpi)
		}
	case SeriesTrapezoidal:
		for _, m := range trapezoidal {
			add(FormTrapezoidal, m.d, m.pitch)
		}
	case SeriesWhitworth:
		// BSF has sizes that BSW lacks; those take their fine pitch.
		for _, w := range bsw {
			add(FormWhitworth, w.d*mmPerInch, mmPerInch/w.tpi)
		}
		for _, w := range bsf {
			if !slices.ContainsFunc(bsw, func(b inchSize) bool { return same(b.d, w.d) }) {
				add(FormWhitworth, w.d*mmPerInch, mmPerInch/w.tpi)
			}
		}
		slices.SortStableFunc(out, func(a, b Spec) int { return compareFloat(a.Diameter, b.Diameter) })
	case SeriesPipe:
		for _, p := range pipe {
			add(FormWhitworth, p.d, mmPerInch/p.tpi)
		}
	case SeriesACME:
		for _, a := range acme {
			add(FormACME, a.d*mmPerInch, mmPerInch/a.tpi)
		}
	}
	return out
}

// compareFloat orders two numbers.
func compareFloat(a, b float64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

// Pitches are the standard threads of s's size, coarsest first, each
// with s's starts and hand: the coarse and fine metric pitches, UNC and
// UNF, the trapezoidal pitches of ISO 2902, BSW and BSF. A size no
// series has has none.
func Pitches(s Spec) []Spec {
	var pitches []float64
	inches := s.Diameter / mmPerInch
	tpiOf := func(table []inchSize) {
		for _, t := range table {
			if same(t.d, inches) {
				pitches = append(pitches, mmPerInch/t.tpi)
			}
		}
	}
	switch SeriesOf(s) {
	case SeriesMetric:
		if coarse, ok := CoarsePitch(s.Diameter); ok {
			pitches = append(pitches, coarse)
			for d, fine := range metricFine {
				if same(d, s.Diameter) {
					pitches = append(pitches, fine...)
				}
			}
		}
	case SeriesUnified:
		for _, u := range unified {
			if same(u.d, inches) {
				for _, tpi := range []float64{u.unc, u.unf} {
					if tpi > 0 {
						pitches = append(pitches, mmPerInch/tpi)
					}
				}
			}
		}
	case SeriesTrapezoidal:
		for _, m := range trapezoidal {
			if same(m.d, s.Diameter) {
				pitches = append(append(pitches, m.pitch), m.others...)
			}
		}
	case SeriesWhitworth:
		tpiOf(bsw)
		tpiOf(bsf)
	case SeriesPipe:
		p, _ := pipeSize(s.Diameter)
		pitches = append(pitches, mmPerInch/p.tpi)
	case SeriesACME:
		tpiOf(acme)
	}
	slices.SortFunc(pitches, func(a, b float64) int { return compareFloat(b, a) })
	pitches = slices.CompactFunc(pitches, same)
	out := make([]Spec, len(pitches))
	for i, p := range pitches {
		out[i] = Spec{Form: s.Form, Diameter: s.Diameter, Pitch: p, Starts: s.Starts, LeftHand: s.LeftHand}
	}
	return out
}

// pipeSize is the pipe thread of major diameter d, in millimetres.
func pipeSize(d float64) (pipeThread, bool) {
	for _, p := range pipe {
		if math.Abs(p.d-d) < 1e-6 {
			return p, true
		}
	}
	return pipe[0], false
}

// nearestSize is the size of the form's series nearest to diameter d,
// at its usual pitch, for a hint.
func nearestSize(f Form, d float64) Spec {
	sr := SeriesOf(Spec{Form: f, Diameter: d})
	sizes := sr.Sizes()
	best := sizes[0]
	for _, s := range sizes {
		if math.Abs(s.Diameter-d) < math.Abs(best.Diameter-d) {
			best = s
		}
	}
	return best
}
