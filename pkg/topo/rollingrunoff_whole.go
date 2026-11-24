package topo

import (
	"math"

	"github.com/telesapr/telesapr/pkg/srf"
)

// A fillet both of whose ends run off side j's face onto the same flat
// face, with the ball's foot on side j past that face's plane all along
// the chain: a pocket's rim filleted deeper than its wall is high, the
// foot on the wall below the floor from end to end (the trough's rim at
// 3.5). The floor's plane then cuts the fillet over its whole length, as
// an end's run-off cuts its end (rollingrunoff.go) and a dip its middle
// (rollingrunoff_dip.go), the two ends' cuts one: the fillet takes side
// j's face whole. The next face loses its edge with side j's face, from
// one end's w to the other's, and takes the fillet's cut from the one
// end's p1 to the other's, and the lines from each p1 to its w, which it
// shares with that end's face; the end faces take their end sections up
// to p1 as for any run-off, and side j's face goes.

// wholeRunOff is the fillet's taking side j's face whole where both ends
// run off it onto the same flat face, the foot past that face all along;
// nil where the ends' run-offs are not so, and an error where the fillet
// would take that face whole but the faces round it are not as this can
// follow.
func (rb *rollingBall) wholeRunOff(row []srf.SSurface, cuts []rbEndCut, isChain map[int]bool) (*rbConsumed, error) {
	t := rb.t
	ro0, ro1 := cuts[0].off, cuts[1].off
	j := ro0.j
	if ro0.sideN != ro1.sideN || ro0.ns != nil || ro1.ns != nil {
		return nil, nil
	}
	ui, uo := 3, 1
	if j == 1 {
		ui, uo = 1, 3
	}
	// Side j's contact past the next face all along; where some of it
	// comes back under, the end's run-off cuts the fillet to there.
	for b := range row {
		c := row[b].EdgeCurve(ui)
		for i := range consumedSamples + 1 {
			if ro0.h(c.PointAt(float64(i)/consumedSamples)) <= 0 {
				return nil, nil
			}
		}
	}
	at := cuts[0].end.p0(t)
	if cuts[0].also != nil || cuts[1].also != nil {
		return nil, filletErr(at, "the fillet takes the face on side %d whole and runs off both its faces at "+
			"one end: make it smaller", j+1)
	}
	f := cuts[0].end.face[j]
	if cuts[1].end.face[j] != f || len(rb.side[j]) != 1 || rb.side[j][0] != f || len(t.Faces[f].Loops) != 1 {
		return nil, filletErr(at, "the fillet takes the face on side %d whole, which has more edges than it can "+
			"follow: make it smaller", j+1)
	}
	// Side j's face is the edge, the end faces' edges and its one edge
	// with the next face, and nothing more.
	want := map[int]bool{cuts[0].end.a[j]: true, cuts[1].end.a[j]: true, ro0.sideN: true}
	for e := range isChain {
		want[e] = true
	}
	edges := t.FaceEdges(f)
	if len(edges) != len(want) {
		return nil, filletErr(at, "the fillet takes the face on side %d whole, which has more edges than it can "+
			"follow: make it smaller", j+1)
	}
	for _, e := range edges {
		if !want[e] {
			return nil, filletErr(at, "the fillet takes the face on side %d whole, which has more edges than it can "+
				"follow: make it smaller", j+1)
		}
	}
	// Side 1-j's contact stays under the next face: the fillet keeps a
	// strip of every patch.
	for b := range row {
		c := row[b].EdgeCurve(uo)
		for i := range consumedSamples + 1 {
			if x := c.PointAt(float64(i) / consumedSamples); ro0.h(x) > 0 {
				return nil, filletErr(x, "the fillet takes the face on side %d whole and runs off the face on side %d "+
					"too here: make it smaller", j+1, 2-j)
			}
		}
	}
	return &rbConsumed{j: j, f: f, r: -1, whole: true}, nil
}

// wholeLoops cuts every patch of a fillet that takes side j's face whole
// onto one next face by that face's plane: the loops of those cut, and
// the cut across them, run from the end at 0's p1 to the end at 1's.
func (rb *rollingBall) wholeLoops(sh *srf.SShell, row []srf.SSurface, cuts []rbEndCut) (map[int][]srf.SBezier,
	[]srf.SBezier, error) {
	ro0, ro1 := cuts[0].off, cuts[1].off
	loops := map[int][]srf.SBezier{}
	var across []srf.SBezier
	tol := math.Max(sh.Tol.ChordTolMm()/10, 1e-6)
	for b := range row {
		l, a, err := clipPatch(&row[b], ro0, tol)
		if err != nil {
			return nil, nil, err
		}
		if l == nil || len(a) == 0 {
			return nil, nil, filletErr(row[b].PointAt(0.5, 0.5), "the fillet takes a face whole, and the face it runs "+
				"onto cuts it away here: make it smaller")
		}
		for _, ro := range []*rbRunOff{ro0, ro1} {
			snapEnds(l, ro.p1At, ro.p1)
			snapEnds(a, ro.p1At, ro.p1)
		}
		loops[b] = l
		across = append(across, a...)
	}
	chain, ok := runOffChain(across, ro0.p1)
	if !ok || !chain[len(chain)-1].Finish().Equals(ro1.p1) {
		return nil, nil, filletErr(ro0.p1, "the fillet takes a face whole, and its cut by the face it runs onto "+
			"doesn't join from end to end: make it a little smaller or larger")
	}
	return loops, chain, nil
}
