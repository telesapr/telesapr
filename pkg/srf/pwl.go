package srf

import (
	"github.com/telesapr/telesapr/pkg/geom"
)

// MakePwlIntoPoints splits the curve into piecewise linear segments that
// don't deviate from it by more than tol's chord tolerance, and into
// no more than its segment limit. maxDt forces spans of no more than
// that length in t (0 means the default).
func (b *SBezier) MakePwlIntoPoints(l *[]geom.Vector, tol Tolerance, maxDt float64) {
	// Never do fewer than three intermediate points for curves; people seem to get
	// unhappy when their circles turn into squares, but maybe less
	// unhappy with octagons. Now 16-gons.
	if maxDt == 0 {
		if b.Degree == 1 {
			maxDt = 1.0
		} else {
			maxDt = 0.25
		}
	}
	*l = append(*l, b.Ctrl[0])
	// don't split first degee (lines) unless asked to by the caller via max_dt
	if b.Degree == 1 && maxDt >= 1.0 {
		*l = append(*l, b.Ctrl[1])
	} else {
		b.makePwlInitialWorker(l, 0.0, 0.5, tol, maxDt)
		b.makePwlInitialWorker(l, 0.5, 1.0, tol, maxDt)
	}
}

func (b *SBezier) makePwlWorker(l *[]geom.Vector, ta, tb float64, tol Tolerance, maxDt float64) {
	pa := b.PointAt(ta)
	pb := b.PointAt(tb)

	pm := b.PointAt((ta + tb) / 2.0)
	d := pm.DistanceToLine(pa, pb.Minus(pa))

	step := 1.0 / float64(tol.MaxSegments())
	if ((tb-ta) < step || d < tol.ChordTolMm()) && (tb-ta) <= maxDt {
		// A previous call has already added the beginning of our interval.
		*l = append(*l, pb)
	} else {
		tm := (ta + tb) / 2
		b.makePwlWorker(l, ta, tm, tol, maxDt)
		b.makePwlWorker(l, tm, tb, tol, maxDt)
	}
}

func (b *SBezier) makePwlInitialWorker(l *[]geom.Vector, ta, tb float64, tol Tolerance, maxDt float64) {
	pa := b.PointAt(ta)
	pb := b.PointAt(tb)

	tm1 := ta + (tb-ta)*0.25
	tm2 := ta + (tb-ta)*0.5
	tm3 := ta + (tb-ta)*0.75

	pm1 := b.PointAt(tm1)
	pm2 := b.PointAt(tm2)
	pm3 := b.PointAt(tm3)
	dir := pb.Minus(pa)

	d := geom.Max(pm1.DistanceToLine(pa, dir),
		geom.Max(pm2.DistanceToLine(pa, dir), pm3.DistanceToLine(pa, dir)))

	step := 1.0 / float64(tol.MaxSegments())
	if ((tb-ta) < step || d < tol.ChordTolMm()) && (tb-ta) <= maxDt {
		// A previous call has already added the beginning of our interval.
		*l = append(*l, pb)
	} else {
		tm := (ta + tb) / 2
		b.makePwlWorker(l, ta, tm, tol, maxDt)
		b.makePwlWorker(l, tm, tb, tol, maxDt)
	}
}

// MakePwlInto emits the pwl approximation as edges.
func (b *SBezier) MakePwlInto(sel *SEdgeList, tol Tolerance, maxDt float64) {
	var lv []geom.Vector
	b.MakePwlIntoPoints(&lv, tol, maxDt)
	for i := 1; i < len(lv); i++ {
		sel.AddEdge(lv[i-1], lv[i], 0, 0, 0)
	}
}

// MakePwlIntoContour appends the pwl approximation to sc.
func (b *SBezier) MakePwlIntoContour(sc *SContour, tol Tolerance, maxDt float64) {
	var lv []geom.Vector
	b.MakePwlIntoPoints(&lv, tol, maxDt)
	for _, p := range lv {
		sc.AddPoint(p)
	}
}
