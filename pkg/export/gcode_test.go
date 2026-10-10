package export

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// gcodeOf writes the curves as G code, one pass, with six decimals.
func gcodeOf(t *testing.T, tol float64, curves ...srf.SBezier) string {
	t.Helper()
	w := &gcodeFileWriter{}
	w.self = w
	w.o = Options{
		Tol:    srf.Tol(tol, 64),
		GCode:  GCode{Depth: 1, SafeHeight: 5, Passes: 1, Feed: 100, PlungeFeed: 50},
		Format: func(mm float64) string { return fmt.Sprintf("%.6f", mm) },
	}
	w.StartFile()
	for i := range curves {
		w.Bezier(&curves[i])
	}
	if err := w.FinishAndCloseFile(); err != nil {
		t.Fatal(err)
	}
	return w.out.String()
}

// cutPath follows the G code's cuts and returns points along them, each
// arc stepped finely, checking as it goes that an arc's ends are as far
// from its centre.
func cutPath(t *testing.T, code string) (pts []geom.Vector, g01, g02, g03 int) {
	t.Helper()
	var at geom.Vector
	for line := range strings.SplitSeq(strings.TrimSpace(code), "\r\n") {
		f := strings.Fields(line)
		v := map[byte]float64{}
		for _, w := range f[1:] {
			var x float64
			if _, err := fmt.Sscanf(w[1:], "%g", &x); err != nil {
				t.Fatalf("%q: %v", line, err)
			}
			v[w[0]] = x
		}
		_, hasX := v['X']
		to := geom.Vector{X: v['X'], Y: v['Y']}
		switch f[0] {
		case "G00":
			if hasX {
				at = to
				pts = append(pts, at)
			}
		case "G01":
			if !hasX {
				continue
			}
			g01++
			pts = append(pts, to)
			at = to
		case "G02", "G03":
			ccw := f[0] == "G03"
			if ccw {
				g03++
			} else {
				g02++
			}
			c := at.Plus(geom.Vector{X: v['I'], Y: v['J']})
			r0, r1 := at.DistanceTo(c), to.DistanceTo(c)
			if math.Abs(r0-r1) > 1e-5 {
				t.Errorf("%q: radius %g at the start, %g at the end", line, r0, r1)
			}
			m := gcodeMove{a: at, b: to, arc: true, c: c, ccw: ccw}
			sweep := m.sweepTo(to)
			a0 := math.Atan2(at.Y-c.Y, at.X-c.X)
			for k := 1; k <= 32; k++ {
				a := sweep * float64(k) / 32
				if !ccw {
					a = -a
				}
				pts = append(pts, c.Plus(geom.Vector{X: r0 * math.Cos(a0+a), Y: r0 * math.Sin(a0+a)}))
			}
			at = to
		default:
			t.Fatalf("unknown %q", line)
		}
	}
	return pts, g01, g02, g03
}

// distanceToPolyline is how far p lies from the chain of points.
func distanceToPolyline(p geom.Vector, pl []geom.Vector) float64 {
	best := math.Inf(1)
	for i := 1; i < len(pl); i++ {
		best = min(best, gcodeMove{a: pl[i-1], b: pl[i]}.distanceTo(p))
	}
	return best
}

// requireFollows checks that the cut and the curves lie within tol of
// each other, both ways.
func requireFollows(t *testing.T, what string, cut []geom.Vector, tol float64, curves ...srf.SBezier) {
	t.Helper()
	var fine [][]geom.Vector
	for i := range curves {
		var pl []geom.Vector
		for k := range 2001 {
			pl = append(pl, curves[i].PointAt(float64(k)/2000))
		}
		fine = append(fine, pl)
	}
	worst := 0.0
	for _, p := range cut {
		d := math.Inf(1)
		for _, pl := range fine {
			d = min(d, distanceToPolyline(p, pl))
		}
		worst = max(worst, d)
	}
	for _, pl := range fine {
		for k := 0; k < len(pl); k += 20 {
			worst = max(worst, distanceToPolyline(pl[k], cut))
		}
	}
	if worst > tol*1.01 {
		t.Errorf("%s: the cut strays %g from the curve, more than %g", what, worst, tol)
	}
}

// quarter is the quarter circle of radius r about the origin from angle
// a, turning counter-clockwise.
func quarter(r, a float64) srf.SBezier {
	p := func(a float64) geom.Vector { return geom.Vector{X: r * math.Cos(a), Y: r * math.Sin(a)} }
	sb := srf.SBezierFrom3(p(a), p(a+math.Pi/4).ScaledBy(math.Sqrt2), p(a+math.Pi/2))
	sb.Weight[1] = math.Sqrt2 / 2
	return sb
}

// A circle is cut as arcs, one per piece, turning the way it is drawn,
// and comes back to where it started.
func TestGCodeCircleAsArcs(t *testing.T) {
	t.Parallel()
	var ccw, cw []srf.SBezier
	for k := range 4 {
		q := quarter(5, float64(k)*math.Pi/2)
		ccw = append(ccw, q)
		q.Reverse()
		cw = append(cw, q)
	}
	for _, tc := range []struct {
		name     string
		curves   []srf.SBezier
		g02, g03 int
	}{
		{"counter-clockwise", ccw, 0, 4},
		{"clockwise", cw, 4, 0},
	} {
		code := gcodeOf(t, 0.01, tc.curves...)
		cut, g01, g02, g03 := cutPath(t, code)
		if g01 != 0 || g02 != tc.g02 || g03 != tc.g03 {
			t.Errorf("%s: %d G01, %d G02, %d G03; want 0, %d, %d:\n%s", tc.name, g01, g02, g03, tc.g02, tc.g03, code)
		}
		if !cut[0].Equals(cut[len(cut)-1]) {
			t.Errorf("%s: the cut ends at %v, not where it started at %v", tc.name, cut[len(cut)-1], cut[0])
		}
		for _, p := range cut {
			if math.Abs(p.Magnitude()-5) > 1e-5 {
				t.Errorf("%s: %v is off the circle", tc.name, p)
				break
			}
		}
	}
}

// Curves that are not circles are fitted with arcs within the chord
// tolerance, fewer cuts than the chords that follow them as closely.
func TestGCodeCurvesFittedWithArcs(t *testing.T) {
	t.Parallel()
	v := func(x, y float64) geom.Vector { return geom.Vector{X: x, Y: y} }
	ellipse := srf.SBezierFrom3(v(10, 0), v(10, 4), v(0, 4))
	ellipse.Weight[1] = math.Sqrt2 / 2
	for _, tc := range []struct {
		name  string
		curve srf.SBezier
	}{
		{"a cubic bend", srf.SBezierFrom4(v(0, 0), v(10, 0), v(20, 10), v(20, 20))},
		{"an S", srf.SBezierFrom4(v(0, 0), v(15, 0), v(5, 10), v(20, 10))},
		{"a loop", srf.SBezierFrom4(v(0, 0), v(30, 20), v(-10, 20), v(20, 0))},
		{"a closed cubic", srf.SBezierFrom4(v(0, 0), v(20, 20), v(-20, 20), v(0, 0))},
		{"control points that coincide", srf.SBezierFrom4(v(0, 0), v(0, 0), v(10, 10), v(10, 0))},
		{"a straight cubic", srf.SBezierFrom4(v(0, 0), v(1, 1), v(2, 2), v(3, 3))},
		{"a quarter ellipse", ellipse},
	} {
		for _, tol := range []float64{0.1, 0.01, 0.001} {
			what := fmt.Sprintf("%s at %g", tc.name, tol)
			code := gcodeOf(t, tol, tc.curve)
			cut, g01, g02, g03 := cutPath(t, code)
			requireFollows(t, what, cut, tol, tc.curve)

			var chords []geom.Vector
			tc.curve.MakePwlIntoPoints(&chords, srf.Tol(tol, 1000), 0)
			if n := g01 + g02 + g03; n >= len(chords)-1 && len(chords) > 2 {
				t.Errorf("%s: %d cuts, %d chords", what, n, len(chords)-1)
			}
		}
	}
}

// Consecutive biarc cuts meet tangentially, so the cutter doesn't stop
// between them.
func TestBiarcsTangent(t *testing.T) {
	t.Parallel()
	sb := srf.SBezierFrom4(geom.Vector{}, geom.Vector{X: 15}, geom.Vector{X: 5, Y: 10}, geom.Vector{X: 20, Y: 10})
	var moves []gcodeMove
	fitBiarcs(&sb, 0.001, func(m gcodeMove) { moves = append(moves, m) })
	dir := func(m gcodeMove, atEnd bool) geom.Vector {
		if !m.arc {
			return m.b.Minus(m.a).WithMagnitude(1)
		}
		p := m.a
		if atEnd {
			p = m.b
		}
		r := p.Minus(m.c)
		if m.ccw {
			return geom.Vector{X: -r.Y, Y: r.X}.WithMagnitude(1)
		}
		return geom.Vector{X: r.Y, Y: -r.X}.WithMagnitude(1)
	}
	for i := 1; i < len(moves); i++ {
		if !moves[i-1].b.Equals(moves[i].a) {
			t.Fatalf("cut %d ends at %v, the next starts at %v", i-1, moves[i-1].b, moves[i].a)
		}
		if a := dir(moves[i-1], true).Dot(dir(moves[i], false)); a < math.Cos(0.01) {
			t.Errorf("cuts %d and %d meet at an angle of %g rad", i-1, i, math.Acos(min(1, a)))
		}
	}
}

// Cuts are chained end to end whichever way they were drawn, and a
// chain that doesn't close is cut on its own.
func TestGCodeChains(t *testing.T) {
	t.Parallel()
	v := func(x, y float64) geom.Vector { return geom.Vector{X: x, Y: y} }
	q := quarter(5, 0)
	q.Reverse() // from (0, 5) to (5, 0)
	code := gcodeOf(t, 0.01,
		srf.SBezierFrom2(v(0, 0), v(5, 0)),
		srf.SBezierFrom2(v(0, 5), v(0, 0)),
		q,
		srf.SBezierFrom2(v(20, 0), v(30, 0)),
	)
	want := strings.Join([]string{
		"G00 X0.000000 Y0.000000",
		"G01 Z1.000000 F50.000000",
		"G01 X5.000000 Y0.000000 F100.000000",
		"G03 X0.000000 Y5.000000 I-5.000000 J0.000000 F100.000000",
		"G01 X0.000000 Y0.000000 F100.000000",
		"G00 Z5.000000",
		"G00 X20.000000 Y0.000000",
		"G01 Z1.000000 F50.000000",
		"G01 X30.000000 Y0.000000 F100.000000",
		"G00 Z5.000000",
		"",
	}, "\r\n")
	if code != want {
		t.Errorf("got\n%s\nwant\n%s", code, want)
	}
}
