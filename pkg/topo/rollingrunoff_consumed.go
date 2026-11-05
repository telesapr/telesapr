package topo

import (
	"math"
	"slices"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// A fillet both of whose ends run off side j's face onto two flat faces
// that meet past it along a ridge, as a short plate's gabled top, while
// the ball's contact with side j's face lies above one next face or the
// other all along: the fillet takes side j's face whole. Where both
// ends run off but some of the contact stays under both next faces, the
// fillet keeps that piece of side j's face, as runOffCuts and
// runOffApart make it; here no piece is left, and the two cuts overlap
// along the edge.
//
// The fillet's patches are cut by both next faces' planes, which meet
// across the fillet at x, on the line the next faces meet along carried
// on past the ridge's end r on side j's face. Each next face loses its
// edge with side j's face, from w to r, and takes the line from r to x,
// which the other next face takes too, and its curve across the fillet
// from x to p1; the end faces take the end sections as for any run-off,
// and side j's face goes.

// rbConsumed is a fillet that takes side j's face, f, whole: the ridge's
// vertex r on that face, and x.
type rbConsumed struct {
	j, f int
	r    int
	x    geom.Vector
}

// consumedSamples is how many points along each patch's side the
// checks of consumedRunOff look at.
const consumedSamples = 16

// consumedRunOff is the fillet's taking side j's face whole, where its
// ends' cuts show it does, as the package comment above says; nil where
// they don't, and an error where the fillet would take that face whole
// but the faces round it are not as this can follow.
func (rb *rollingBall) consumedRunOff(row []srf.SSurface, cuts []rbEndCut, isChain map[int]bool) (*rbConsumed, error) {
	t := rb.t
	if len(cuts) != 2 || cuts[0].off == nil || cuts[1].off == nil {
		return nil, nil
	}
	ro0, ro1 := cuts[0].off, cuts[1].off
	j := ro0.j
	if ro1.j != j || ro0.n == ro1.n || ro0.ns != nil || ro1.ns != nil {
		return nil, nil
	}
	ui, uo := 3, 1
	if j == 1 {
		ui, uo = 1, 3
	}
	// Side j's contact, above one next face or the other all along.
	for b := range row {
		c := row[b].EdgeCurve(ui)
		for i := range consumedSamples + 1 {
			x := c.PointAt(float64(i) / consumedSamples)
			if ro0.h(x) <= 0 && ro1.h(x) <= 0 {
				return nil, nil
			}
		}
	}
	at := cuts[0].end.p0(t)
	f := cuts[0].end.face[j]
	if cuts[1].end.face[j] != f || len(rb.side[j]) != 1 || rb.side[j][0] != f || len(t.Faces[f].Loops) != 1 {
		return nil, filletErr(at, "the fillet takes the face on side %d whole, which has more edges than it can "+
			"follow: make it smaller", j+1)
	}
	// The next faces' edges with side j's face meet at r, where the edge
	// between the next faces goes on.
	r := -1
	for _, a := range t.Edges[ro0.sideN].V {
		if slices.Contains(t.Edges[ro1.sideN].V[:], a) {
			r = a
		}
	}
	if r < 0 || r == ro0.w || r == ro1.w || len(t.Vertices[r].Edges) != 3 {
		return nil, filletErr(at, "the fillet takes the face on side %d whole, and the faces it runs onto at its ends "+
			"don't meet at that face: make it smaller", j+1)
	}
	ridge := -1
	for _, e := range t.Vertices[r].Edges {
		if e == ro0.sideN || e == ro1.sideN {
			continue
		}
		fs := t.EdgeFaces(e)
		if fs[0] == ro0.n && fs[1] == ro1.n || fs[0] == ro1.n && fs[1] == ro0.n {
			ridge = e
		}
	}
	if ridge < 0 {
		return nil, filletErr(t.Vertices[r].P, "the fillet takes the face on side %d whole, and the faces it runs "+
			"onto at its ends don't meet along an edge from it: make it smaller", j+1)
	}
	// Side j's face is the edge, the end faces' edges and the next faces'
	// edges with it, and nothing more.
	want := map[int]bool{cuts[0].end.a[j]: true, cuts[1].end.a[j]: true, ro0.sideN: true, ro1.sideN: true}
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
	// Side 1-j's contact stays under both next faces, and each end's
	// section under the other end's next face wherever it is under its
	// own: where it reaches above the other's plane, carried on past the
	// ridge, above its own too, its own cuts that away first (clipConsumed
	// cuts by both), and the end face takes only what is under its own.
	for b := range row {
		c := row[b].EdgeCurve(uo)
		for i := range consumedSamples + 1 {
			x := c.PointAt(float64(i) / consumedSamples)
			if ro0.h(x) > 0 || ro1.h(x) > 0 {
				return nil, filletErr(x, "the fillet takes the face on side %d whole and runs off the face on side %d "+
					"too here: make it smaller", j+1, 2-j)
			}
		}
	}
	m := len(row)
	for k, ro := range []*rbRunOff{ro1, ro0} {
		sec, own := row[0].EdgeCurve(0), ro0
		if k == 1 {
			sec, own = row[m-1].EdgeCurve(2), ro1
		}
		for i := range consumedSamples + 1 {
			x := sec.PointAt(float64(i) / consumedSamples)
			if ro.h(x) > 0 && own.h(x) <= 0 {
				return nil, filletErr(x, "the fillet's end runs off the faces at both its ends here: make it smaller")
			}
		}
	}
	return &rbConsumed{j: j, f: f, r: r}, nil
}

// p0 is the end's vertex.
func (end *rbEnd) p0(t *Topology) geom.Vector { return t.Vertices[end.v].P }

// clipConsumed is patch p cut by both next faces' planes, as clipPatch
// cuts it by one: its loop, and its curves across in each plane.
func clipConsumed(p *srf.SSurface, ro0, ro1 *rbRunOff, tol float64) ([]srf.SBezier, [2][]srf.SBezier, error) {
	var across [2][]srf.SBezier
	loop, isAcross, err := clipLoop(p, patchLoop(p), patchSideUV, ro0, tol)
	if err != nil || loop == nil {
		return nil, across, err
	}
	// The second cut's (u, v) along the loop's curves: the patch's
	// points nearest them.
	cur := loop
	uvAt := func(i int, t float64) (float64, float64) {
		var u, v float64
		p.ClosestPointToUV(cur[i].PointAt(t), &u, &v, false)
		return math.Max(0, math.Min(1, u)), math.Max(0, math.Min(1, v))
	}
	// Which of the cut loop's curves lie in ro0's plane: those of the
	// first cut, as pieces, and none of the second's.
	inPlane0 := func(sb srf.SBezier) bool {
		for _, x := range []float64{0, 0.5, 1} {
			if math.Abs(ro0.h(sb.PointAt(x))) > 10*geom.LengthEps {
				return false
			}
		}
		return true
	}
	out, isAcross1, err := clipLoop(p, cur, uvAt, ro1, tol)
	if err != nil || out == nil {
		return nil, across, err
	}
	out, isAcross1 = dropSpecks(out, isAcross1)
	for i, sb := range out {
		switch {
		case isAcross1[i]:
			across[1] = append(across[1], sb)
		case inPlane0(sb) && anyAcross(isAcross):
			across[0] = append(across[0], sb)
		}
	}
	return out, across, nil
}

// anyAcross reports whether any of the curves is across.
func anyAcross(isAcross []bool) bool { return slices.Contains(isAcross, true) }

// consumedX is where the curves across the fillet in the two next faces'
// planes meet: the one end they share.
func consumedX(across [2][]srf.SBezier, at geom.Vector) (geom.Vector, error) {
	var xs []geom.Vector
	for _, a := range across[0] {
		for _, b := range across[1] {
			for _, p := range []geom.Vector{a.Start(), a.Finish()} {
				if (p.Equals(b.Start()) || p.Equals(b.Finish())) && !slices.ContainsFunc(xs, p.Equals) {
					xs = append(xs, p)
				}
			}
		}
	}
	if len(xs) != 1 {
		return geom.Vector{}, filletErr(at, "the fillet takes a face whole, and the faces it runs onto at its ends "+
			"meet across it %d times: make it smaller", len(xs))
	}
	return xs[0], nil
}

// consumedLoops cuts every patch of the fillet by both next faces'
// planes, giving the loops of those cut and the curves across them in
// each plane, and finds x, which both ends take.
func (rb *rollingBall) consumedLoops(sh *srf.SShell, row []srf.SSurface, cuts []rbEndCut,
	cons *rbConsumed) (map[int][]srf.SBezier, [2][]srf.SBezier, error) {
	ro0, ro1 := cuts[0].off, cuts[1].off
	loops := map[int][]srf.SBezier{}
	var across [2][]srf.SBezier
	tol := math.Max(sh.Tol.ChordTolMm()/10, 1e-6)
	at := rb.t.Vertices[cons.r].P
	for b := range row {
		l, a, err := clipConsumed(&row[b], ro0, ro1, tol)
		if err != nil {
			return nil, across, err
		}
		if l == nil {
			return nil, across, filletErr(at, "the fillet takes a face whole, and the faces it runs onto cut it away "+
				"here: make it smaller")
		}
		for _, ro := range []*rbRunOff{ro0, ro1} {
			snapEnds(l, ro.p1At, ro.p1)
			snapEnds(a[0], ro.p1At, ro.p1)
			snapEnds(a[1], ro.p1At, ro.p1)
		}
		if len(a[0]) > 0 || len(a[1]) > 0 {
			loops[b] = l
		}
		across[0] = append(across[0], a[0]...)
		across[1] = append(across[1], a[1]...)
	}
	x, err := consumedX(across, at)
	if err != nil {
		return nil, across, err
	}
	// x lies on the line the next faces meet along, on past r.
	r := rb.t.Vertices[cons.r].P
	var far geom.Vector
	for _, e := range rb.t.Vertices[cons.r].Edges {
		if e != ro0.sideN && e != ro1.sideN {
			ed := &rb.t.Edges[e]
			far = ed.Points[0]
			if far.Equals(r) {
				far = ed.Points[len(ed.Points)-1]
			}
		}
	}
	d := r.Minus(far).WithMagnitude(1)
	if x.Minus(r).Dot(d) <= 0 || x.Minus(r).Cross(d).Magnitude() > 10*geom.LengthEps {
		return nil, across, filletErr(x, "the fillet takes a face whole, and the faces it runs onto meet across it "+
			"short of their edge's end: make it smaller")
	}
	cons.x = x
	ro0.x, ro1.x = x, x
	return loops, across, nil
}

// dropSurfaceOf takes face f's surface out of the shell, with the curves
// no trim runs along any more.
func dropSurfaceOf(sh *srf.SShell, t *Topology, f int) {
	h := t.Faces[f].Surface
	sh.Surface = slices.DeleteFunc(sh.Surface, func(s srf.SSurface) bool { return s.H == h })
	dropUnusedCurves(sh)
}

// dropSpecks takes out of the loop the curves shorter than LengthEps,
// as the second cut leaves where the planes meet a hair from a patch's
// side, the curve after each taken to start where the one before ends.
func dropSpecks(loop []srf.SBezier, isAcross []bool) ([]srf.SBezier, []bool) {
	var out []srf.SBezier
	var flags []bool
	for i, sb := range loop {
		speck := true
		for d := range sb.Degree + 1 {
			if !sb.Ctrl[d].Equals(sb.Start()) {
				speck = false
			}
		}
		if speck && len(loop) > 3 {
			continue
		}
		out = append(out, sb)
		flags = append(flags, isAcross[i])
	}
	for i := range out {
		prev := out[(i+len(out)-1)%len(out)]
		out[i].Ctrl[0] = prev.Finish()
	}
	return out, flags
}
