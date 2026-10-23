package thread

import "math"

// NearestForHole is the ISO metric thread a hole of diameter d is likely
// meant to carry: a hole is drawn at the thread's nominal diameter or at
// the drill a tap needs, its minor diameter, so the thread whose nominal
// or minor diameter is nearest d.
func NearestForHole(d float64) Spec {
	best, gap := metricCoarse[0], math.Inf(1)
	for _, m := range metricCoarse {
		s := Spec{Form: FormMetric, Diameter: m.d, Pitch: m.pitch}
		if g := min(math.Abs(m.d-d), math.Abs(s.MinorDiameter()-d)); g < gap {
			best, gap = m, g
		}
	}
	return Spec{Form: FormMetric, Diameter: best.d, Pitch: best.pitch}
}
