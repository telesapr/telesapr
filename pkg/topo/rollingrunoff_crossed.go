package topo

import (
	"slices"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// An end that runs off both its faces whose two cuts cross on the
// fillet before its end section (rollingrunoff_both.go): the end section
// lies wholly past one next face or the other, as a plate's foot on a
// cone does at its low end where the cone's flat comes in close beside
// the plate. The two next faces then meet across the fillet's end: the
// line their planes share runs from x, where their cuts on the fillet
// cross, to m, where it meets the end face's plane, and both faces
// grow to it. The fillet's patches past x are cut away whole; the patch
// with x in it keeps the corner the two cuts make there. The end face
// takes no end section: it loses its edges with the sides' faces and
// takes the lines from each next face's w on to m, each next face's
// edge with it carried on; each next face loses its edge with its side's
// face up to p2 and takes its cut across the fillet from p2 to x, the
// line from x to m, which the other next face shares, and the line from
// m back to its w.
//
// Made for flat next faces and a flat end face, where the lines are
// lines; a curved one is refused as before.

// crossedRunOffs marks an end that runs off both its faces whose cuts
// cross on the fillet (cut's end section wholly past one next face or
// the other), and finds m; refused by name where a face is curved or
// takes material away, or m lies where the faces can't grow to it.
func (rb *rollingBall) crossedRunOffs(cut rbEndCut, at geom.Vector) error {
	t := rb.t
	a, b := cut.off, cut.also
	refuse := filletErr(at, "the fillet's end runs off both its faces here, and the faces past them meet across "+
		"its end: make it smaller")
	if cut.end.curved {
		return refuse
	}
	for _, ro := range []*rbRunOff{a, b} {
		if ro.ns != nil || ro.tangent || ro.away {
			return refuse
		}
	}
	// m lies on the end face's edge with a's next face, carried on past
	// a's w, where b's plane cuts it.
	wa := t.Vertices[a.w].P
	pts := t.Edges[a.endN].Points
	far := pts[0]
	if far.Minus(wa).Magnitude() < pts[len(pts)-1].Minus(wa).Magnitude() {
		far = pts[len(pts)-1]
	}
	d := far.Minus(wa)
	den := b.np.Dot(d)
	if den == 0 {
		return refuse
	}
	m := wa.Minus(d.ScaledBy(b.h(wa) / den))
	for _, ro := range []*rbRunOff{a, b} {
		w := t.Vertices[ro.w].P
		if m.Minus(w).Magnitude() < 100*geom.LengthEps || alongFrom(m, w, t.Edges[ro.endN].Points) != -1 {
			return refuse
		}
	}
	a.crossed, b.crossed = true, true
	a.meet, b.meet = m, m
	return nil
}

// crossedX is where the cuts of a crossed end's two next faces meet on
// the fillet, x: the one end their curves across it share.
func crossedX(a, b []srf.SBezier, at geom.Vector) (geom.Vector, error) {
	xs := sharedEnds(a, b)
	if len(xs) != 1 {
		return geom.Vector{}, filletErr(at, "the fillet's end runs off both its faces here, and the faces past them "+
			"meet across it %d times: make it smaller", len(xs))
	}
	return xs[0], nil
}

// sharedEnds is the ends any curve of a shares with any of b, once each.
func sharedEnds(a, b []srf.SBezier) []geom.Vector {
	var xs []geom.Vector
	for _, x := range a {
		for _, y := range b {
			for _, p := range []geom.Vector{x.Start(), x.Finish()} {
				if (p.Equals(y.Start()) || p.Equals(y.Finish())) && !slices.ContainsFunc(xs, p.Equals) {
					xs = append(xs, p)
				}
			}
		}
	}
	return xs
}

// closeRunOffCrossed puts a crossed end into the end face and the two
// next faces, as the package comment above says.
func (e *runOffEdit) closeRunOffCrossed(cut rbEndCut, across map[*rbRunOff][]srf.SBezier) error {
	t := e.rb.t
	v := t.Vertices[cut.end.v].P
	a, b := cut.off, cut.also
	wa, wb, m := t.Vertices[a.w].P, t.Vertices[b.w].P, a.meet
	ends := []srf.SBezier{srf.SBezierFrom2(wa, m), srf.SBezierFrom2(m, wb)}
	if _, err := e.put(cut.end.f, [][2]geom.Vector{{v, wa}, {v, wb}}, ends, wa, wb, wa); err != nil {
		return err
	}
	for _, ro := range []*rbRunOff{a, b} {
		w := t.Vertices[ro.w].P
		fill := append(slices.Clone(across[ro]), srf.SBezierFrom2(ro.x, m), srf.SBezierFrom2(m, w))
		if _, err := e.put(ro.n, [][2]geom.Vector{{w, ro.p2}}, fill, ro.p2, w, ro.p2); err != nil {
			return err
		}
	}
	return nil
}
