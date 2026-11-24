package export

import (
	"math"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/internal/cfmt"
	"github.com/telesapr/telesapr/pkg/srf"
)

// A face's trims come in pieces, a Bezier each: where a trim crosses
// the patches of a sheet, or a face's edge is cut where it was
// intersected, a single edge between two faces is many. A STEP file
// wrote each piece as an edge of its own, with its own vertices, and a
// chamfer along a wrapped band came to forty edges between the same two
// faces. Consecutive pieces of B-spline are written instead as one
// B_SPLINE_CURVE_WITH_KNOTS, the pieces raised to one degree, with a
// knot of multiplicity the degree at each joint: the same curve, exactly.
//
// Which joints go is decided over the whole shell before a face is
// written, so that both faces along an edge join the same pieces, and
// the edge is one EDGE_CURVE that both use: a joint goes when exactly
// two loop corners of the shell lie there (the two faces of one edge, or
// a seam's face twice), the pieces either side of one are those of the
// other run the other way, and they are B-splines, not a LINE or a
// CIRCLE. A point where a third face meets is a corner more, and stays
// a vertex; so does one where an edge's pieces don't match on its two
// faces, one face's piece running on past the other's joint.

// stepJoints counts, for the joints of a shell's loops, how many loop
// corners lie at each, and whether a line or circle meets it.
type stepJoints struct {
	points  []geom.Vector
	count   []int
	blocked []bool
	// sides are, at the first corner of each joint, the curves either
	// side of it.
	sides []stepSides
	cells map[[3]int64][]int
	far   []int
}

// find is the joint at p, within stepPrecision, or -1.
func (j *stepJoints) find(p geom.Vector) int {
	best := -1
	try := func(entries []int) {
		for _, e := range entries {
			if (best < 0 || e < best) && j.points[e].EqualsTol(p, stepPrecision) {
				best = e
			}
		}
	}
	try(j.far)
	c, ok := cellOf(p)
	if !ok {
		for e := range j.points {
			if j.points[e].EqualsTol(p, stepPrecision) {
				return e
			}
		}
		return best
	}
	for dx := int64(-1); dx <= 1; dx++ {
		for dy := int64(-1); dy <= 1; dy++ {
			for dz := int64(-1); dz <= 1; dz++ {
				try(j.cells[[3]int64{c[0] + dx, c[1] + dy, c[2] + dz}])
			}
		}
	}
	return best
}

// stepSides are the curves either side of a loop's corner, by the
// start and middle of the one before and the middle and finish of the
// one after.
type stepSides [4]geom.Vector

func sidesOf(prev, next *srf.SBezier) stepSides {
	return stepSides{prev.Start(), prev.PointAt(0.5), next.PointAt(0.5), next.Finish()}
}

// match tells whether s are the sides of t, run either way: the other
// face of an edge runs its loop along it the other way, or the same way
// when one face's loops are turned round for the file (sheetJobs).
func (s stepSides) match(t stepSides) bool {
	const tol = 10 * geom.LengthEps
	same, back := true, true
	for i := range s {
		same = same && s[i].EqualsTol(t[i], tol)
		back = back && s[i].EqualsTol(t[len(t)-1-i], tol)
	}
	return same || back
}

// add records a loop corner at p, between the curves sides; blocked
// when a curve either side of it is written as a line or circle.
func (j *stepJoints) add(p geom.Vector, sides stepSides, blocked bool) {
	e := j.find(p)
	if e < 0 {
		e = len(j.points)
		j.points = append(j.points, p)
		j.count = append(j.count, 0)
		j.blocked = append(j.blocked, false)
		j.sides = append(j.sides, sides)
		if c, ok := cellOf(p); ok {
			if j.cells == nil {
				j.cells = map[[3]int64][]int{}
			}
			j.cells[c] = append(j.cells[c], e)
		} else {
			j.far = append(j.far, e)
		}
	}
	j.count[e]++
	if j.count[e] > 1 && !sides.match(j.sides[e]) {
		blocked = true
	}
	j.blocked[e] = j.blocked[e] || blocked
}

// addLoop records a loop's corners: the finish of each curve, where the
// next starts.
func (j *stepJoints) addLoop(loop *srf.SBezierLoop) {
	n := len(loop.Curves)
	analytic := make([]bool, n)
	for i := range loop.Curves {
		_, _, _, _, analytic[i] = lineOrCircleOf(&loop.Curves[i])
	}
	for i := range loop.Curves {
		next := (i + 1) % n
		j.add(loop.Curves[i].Finish(), sidesOf(&loop.Curves[i], &loop.Curves[next]), n < 2 || analytic[i] || analytic[next])
	}
}

// joins tells whether the pieces meeting at p are written as one curve.
func (j *stepJoints) joins(p geom.Vector) bool {
	if j == nil {
		return false
	}
	e := j.find(p)
	return e >= 0 && j.count[e] == 2 && !j.blocked[e]
}

// before orders points as both faces of an edge order them, though each
// has its own copy of a point: a coordinate decides only where the two
// differ by more than the precision points are merged at.
func before(a, b geom.Vector) bool {
	for _, d := range [3]float64{a.X - b.X, a.Y - b.Y, a.Z - b.Z} {
		if math.Abs(d) > stepPrecision {
			return d < 0
		}
	}
	return false
}

// edgeRuns splits a loop into the runs of curves written as one edge
// each, in the loop's order: a run ends where its last curve's finish
// stays a vertex. A loop whose every joint would go keeps its first
// point; a run that comes back to where it started, as such a loop's
// does or one round a ring that touches the rest of its face's loop at
// one vertex, is cut at the joint farthest from that vertex, which both
// its faces choose alike. So no edge is closed on itself: a closed
// curve's points are not found again as one curve's
// (HasBSplineCurveAnAlias), and the other face would write its own.
func (j *stepJoints) edgeRuns(loop *srf.SBezierLoop) [][]srf.SBezier {
	cs := loop.Curves
	n := len(cs)
	keep := make([]bool, n) // the vertex at curve i's finish stays
	kept := false
	for i := range cs {
		keep[i] = n < 2 || !j.joins(cs[i].Finish())
		kept = kept || keep[i]
	}
	if !kept {
		lo := 0
		for i := range cs {
			if before(cs[i].Finish(), cs[lo].Finish()) {
				lo = i
			}
		}
		keep[lo] = true
	}
	// Start after the last vertex kept, so that the loop's first vertex
	// stays where it was when none goes.
	start := 0
	if !keep[n-1] {
		for i := range cs {
			if keep[i] {
				start = i + 1
			}
		}
	}
	var runs [][]srf.SBezier
	var run []srf.SBezier
	for k := range n {
		i := (start + k) % n
		run = append(run, cs[i])
		if keep[i] {
			runs = append(runs, splitClosed(run)...)
			run = nil
		}
	}
	return runs
}

// splitClosed cuts a run of curves that ends where it starts at the
// joint farthest from that point, a tie going to the point first in
// order (before); any other run is itself.
func splitClosed(run []srf.SBezier) [][]srf.SBezier {
	end := run[0].Start()
	if len(run) < 2 || !run[len(run)-1].Finish().EqualsTol(end, stepPrecision) {
		return [][]srf.SBezier{run}
	}
	at := 0
	var far float64
	for k := range len(run) - 1 {
		p := run[k].Finish()
		d := p.Minus(end).Magnitude()
		if k == 0 || d > far+stepPrecision || (d > far-stepPrecision && before(p, run[at].Finish())) {
			at, far = k, d
		}
	}
	return [][]srf.SBezier{run[:at+1], run[at+1:]}
}

// joinedCurve is a run of Beziers as one B-spline: its degree, control
// points, weights and distinct knots with their multiplicities. Each
// piece is raised to the highest degree, and its weights put in the
// form whose end weights are 1, a change of its parameter that leaves
// the curve as it was, so that the pieces' weights agree at the joints.
// The knots are spaced by the length of each piece's control polygon.
func joinedCurve(run []srf.SBezier) (deg int, ctrl []geom.Vector, weight []float64, mults []int, knots []float64) {
	for i := range run {
		deg = max(deg, run[i].Degree)
	}
	lengths := make([]float64, len(run))
	var total float64
	for k := range run {
		b, err := run[k].ElevatedTo(deg)
		if err != nil {
			panic("export: a curve raised past its maximum degree: " + err.Error())
		}
		standardWeights(&b)
		for i := 0; i <= deg; i++ {
			if i > 0 {
				lengths[k] += b.Ctrl[i].Minus(b.Ctrl[i-1]).Magnitude()
			}
			if k > 0 && i == 0 {
				continue // the joint, which the piece before gave
			}
			ctrl = append(ctrl, b.Ctrl[i])
			weight = append(weight, b.Weight[i])
		}
		total += lengths[k]
	}
	// No span may be empty, nor so short that its knots print alike.
	floor := 1e-6 * total
	if floor <= 0 {
		floor = 1
	}
	var sum float64
	for k := range lengths {
		sum += max(lengths[k], floor)
	}
	knots = append(knots, 0)
	mults = append(mults, deg+1)
	var at float64
	for k := range run {
		at += max(lengths[k], floor)
		if k+1 < len(run) {
			knots = append(knots, at/sum)
			mults = append(mults, deg)
		}
	}
	knots = append(knots, 1)
	mults = append(mults, deg+1)
	return deg, ctrl, weight, mults, knots
}

// standardWeights scales b's weights to the form whose end weights are
// 1: weight i times s to the i, for s the d-th root of w0 over wd, then
// all over w0, which changes the curve's parameter and not the curve.
func standardWeights(b *srf.SBezier) {
	d := b.Degree
	w0, wd := b.Weight[0], b.Weight[d]
	if d == 0 || w0 <= 0 || wd <= 0 || (w0 == 1 && wd == 1) {
		return
	}
	s := math.Pow(w0/wd, 1/float64(d))
	f := 1 / w0
	for i := 0; i <= d; i++ {
		b.Weight[i] *= f
		f *= s
	}
	b.Weight[0], b.Weight[d] = 1, 1
}

// exportRun writes a run of curves as one curve and returns its id.
func (w *stepFileWriter) exportRun(run []srf.SBezier) int {
	if len(run) == 1 {
		return w.ExportCurve(&run[0])
	}
	deg, ctrl, weight, mults, knots := joinedCurve(run)
	ret := w.id
	curvePoints := make([]int, len(ctrl))
	for i, c := range ctrl {
		if !w.HasCartesianPointAnAlias(ret+1+i, c, -1, nil) {
			w.printf(stepCartesianPointFormat, ret+1+i, c.X, c.Y, c.Z)
		}
		curvePoints[i] = w.InsertPoint(ret + 1 + i)
	}
	if !w.HasBSplineCurveAnAlias(ret, curvePoints) {
		itoa := func(n int) string { return cfmt.Sprintf("%d", n) }
		ref := func(n int) string { return cfmt.Sprintf("#%d", n) }
		weigh := func(x float64) string { return cfmt.Sprintf("%.10f", x) }
		w.printf("#%d=(\n", ret)
		w.printf("BOUNDED_CURVE()\n")
		w.printf("B_SPLINE_CURVE(%d,(%s),.UNSPECIFIED.,.F.,.F.)\n", deg, stepList(curvePoints, ref))
		w.printf("B_SPLINE_CURVE_WITH_KNOTS((%s),(%s),.UNSPECIFIED.)\n", stepList(mults, itoa), stepList(knots, stepKnot))
		w.printf("CURVE()\n")
		w.printf("GEOMETRIC_REPRESENTATION_ITEM()\n")
		w.printf("RATIONAL_B_SPLINE_CURVE((%s))\n", stepList(weight, weigh))
		w.printf("REPRESENTATION_ITEM('')\n);\n")
		w.printf("\n")
	}
	w.id = ret + 1 + len(ctrl)
	return ret
}
