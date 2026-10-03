package srf

import (
	"math"
	"sync/atomic"

	"github.com/telesapr/telesapr/pkg/geom"
)

// chordTolMm and maxSegments mirror SS.ChordTolMm() and
// SS.GetMaxSegments(). The C++ computes chordTolCalculated in
// SolveSpaceUI::GenerateAll from the entity bounding box
// (generate.cpp:225); sketch.GenerateAll does the same via
// SetChordTolMm.
// The chord tolerance is atomic: a sketch generated on another
// goroutine (the solids made off the interface's thread, roadmap 17.3)
// sets it to the value the interface's own generation does.
var chordTolBits atomic.Uint64

func init() { chordTolBits.Store(math.Float64bits(0.1)) }

var (
	maxSegments = 20

	// While exporting, the export tolerance and segment limit apply
	// instead (C++ SS.exportMode, ExportChordTolMm, exportMaxSegments).
	exportMode        bool
	exportChordTolMm  float64
	exportMaxSegments int
)

// ChordTolMm returns the absolute chord tolerance in mm: the export
// one in export mode (C++ SS.ChordTolMm).
func ChordTolMm() float64 {
	if exportMode {
		return exportChordTolMm
	}
	return math.Float64frombits(chordTolBits.Load())
}

// SetExportMode turns export mode on, with the given chord tolerance in
// mm and segment limit, or off (C++ SS.exportMode).
func SetExportMode(on bool, chordTolMm float64, maxSegments int) {
	exportMode = on
	exportChordTolMm = chordTolMm
	exportMaxSegments = maxSegments
}

// ExportMode reports whether export mode is on.
func ExportMode() bool { return exportMode }

// SetChordTolMm sets the absolute chord tolerance in mm. Like C++
// chordTolCalculated it can be 0, for a sketch whose entities all sit
// at one point; the pwl routines are then bounded by maxSegments.
func SetChordTolMm(t float64) {
	chordTolBits.Store(math.Float64bits(t))
}

// SetMaxSegments sets the maximum number of pwl segments per curve
// outside export mode (C++ SS.maxSegments).
func SetMaxSegments(n int) {
	maxSegments = n
}

// GetMaxSegments returns the maximum number of pwl segments per curve:
// the export one in export mode (C++ SS.GetMaxSegments).
func GetMaxSegments() int {
	if exportMode {
		return exportMaxSegments
	}
	return maxSegments
}

// MakePwlIntoPoints splits the curve into piecewise linear segments that
// don't deviate from it by more than chordTol (0 means the default).
// maxDt forces spans of no more than that length in t (0 means the
// default). Mirrors C++ SBezier::MakePwlInto(List<Vector> *, ...)
// (ratpoly.cpp:233).
func (b *SBezier) MakePwlIntoPoints(l *[]geom.Vector, chordTol, maxDt float64) {
	if chordTol == 0 {
		// Use the default chord tolerance.
		chordTol = ChordTolMm()
	}
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
		b.makePwlInitialWorker(l, 0.0, 0.5, chordTol, maxDt)
		b.makePwlInitialWorker(l, 0.5, 1.0, chordTol, maxDt)
	}
}

func (b *SBezier) makePwlWorker(l *[]geom.Vector, ta, tb, chordTol, maxDt float64) {
	pa := b.PointAt(ta)
	pb := b.PointAt(tb)

	pm := b.PointAt((ta + tb) / 2.0)
	d := pm.DistanceToLine(pa, pb.Minus(pa))

	step := 1.0 / float64(GetMaxSegments())
	if ((tb-ta) < step || d < chordTol) && (tb-ta) <= maxDt {
		// A previous call has already added the beginning of our interval.
		*l = append(*l, pb)
	} else {
		tm := (ta + tb) / 2
		b.makePwlWorker(l, ta, tm, chordTol, maxDt)
		b.makePwlWorker(l, tm, tb, chordTol, maxDt)
	}
}

func (b *SBezier) makePwlInitialWorker(l *[]geom.Vector, ta, tb, chordTol, maxDt float64) {
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

	step := 1.0 / float64(GetMaxSegments())
	if ((tb-ta) < step || d < chordTol) && (tb-ta) <= maxDt {
		// A previous call has already added the beginning of our interval.
		*l = append(*l, pb)
	} else {
		tm := (ta + tb) / 2
		b.makePwlWorker(l, ta, tm, chordTol, maxDt)
		b.makePwlWorker(l, tm, tb, chordTol, maxDt)
	}
}

// MakePwlInto emits the pwl approximation as edges. Mirrors C++
// SBezier::MakePwlInto(SEdgeList *, ...).
func (b *SBezier) MakePwlInto(sel *SEdgeList, chordTol, maxDt float64) {
	var lv []geom.Vector
	b.MakePwlIntoPoints(&lv, chordTol, maxDt)
	for i := 1; i < len(lv); i++ {
		sel.AddEdge(lv[i-1], lv[i], 0, 0, 0)
	}
}

// MakePwlIntoContour appends the pwl approximation to sc. Mirrors C++
// SBezier::MakePwlInto(SContour *, ...).
func (b *SBezier) MakePwlIntoContour(sc *SContour, chordTol, maxDt float64) {
	var lv []geom.Vector
	b.MakePwlIntoPoints(&lv, chordTol, maxDt)
	for _, p := range lv {
		sc.AddPoint(p)
	}
}
