package topo

import (
	"math"
	"slices"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// A foot that runs off its face along the chain and comes back, not at
// an end: a chamfer of a pocket's rim whose foot on a wall that leans
// runs below the wall's foot on the pocket's floor where the rim dips
// lowest (curvedPocket's y+ edge in the dish, from 2.25). The ball
// rolls on as if the wall went on below the floor; the floor's plane
// then cuts the fillet as an end that runs off is cut by its next face
// (rollingrunoff.go): the fillet's patches there are cut along it, the
// floor grows by what lies between its edge with the wall and that cut,
// and the wall loses its strip between its edge with the floor and the
// foot, there pinched to nothing, the wall going on either side.
//
// The foot crosses the wall's edge with the floor at a, running off,
// and comes back across it at b: the wall's contact curve stops at a
// and starts again at b, the edge between them goes from both faces,
// and the floor takes the fillet's cut from a to b in its place.

// rbDip is where side j's contact curve runs off its face across the
// face's edge x with the next face, at a, and comes back at b: ro is
// the next face as an end's run-off holds it (its plane or surface,
// signed), first and after the patches it cuts, across the curves in it
// across them from a to b.
type rbDip struct {
	ro           *rbRunOff
	x            int
	a, b         geom.Vector
	first, after int
	across       []srf.SBezier
}

// dipSamples is how many points along each patch's contact curve are
// looked at for one that runs off its face.
const dipSamples = 16

// dipGraze is, in chord tolerances, how little past the next face a
// foot may run and be taken as touching it.
const dipGraze = 0.1

// dipsOn finds side k's dips: the faces across the edges of side k's
// faces, but the chain's, the ends' faces and the faces the ends run
// off onto, whose plane or surface the side's contact curve runs past
// and comes back from, crossing it on that edge both ways.
func (rb *rollingBall) dipsOn(k int, sideRow []srf.SSurface, isChain map[int]bool, endCuts []rbEndCut) ([]*rbDip, error) {
	t := rb.t
	notNext := map[int]bool{}
	for _, f := range rb.side[0] {
		notNext[f] = true
	}
	for _, f := range rb.side[1] {
		notNext[f] = true
	}
	for _, end := range rb.ends {
		if end != nil {
			notNext[end.f] = true
		}
	}
	for _, cut := range endCuts {
		for _, ro := range cut.runOffs() {
			notNext[ro.n] = true
		}
	}
	ui := 3
	if k == 1 {
		ui = 1
	}
	m := len(sideRow)
	// The contact curve's points, in order along the chain.
	type at struct {
		b int
		t float64
		p geom.Vector
	}
	var pts []at
	for b := range m {
		c := sideRow[b].EdgeCurve(ui)
		for i := range dipSamples + 1 {
			if b > 0 && i == 0 {
				continue
			}
			x := float64(i) / dipSamples
			pts = append(pts, at{b, x, c.PointAt(x)})
		}
	}
	ref := midpoint(rb.pts)
	tol := 2 * math.Max(t.shell.Tol.ChordTolMm(), 10*geom.LengthEps)
	// reach is how near an edge some point of the contact comes where
	// the contact crosses it: a step between two points, and the
	// tolerance.
	reach := tol
	for i := 1; i < len(pts); i++ {
		reach = math.Max(reach, pts[i].p.Minus(pts[i-1].p).Magnitude()+tol)
	}
	var out []*rbDip
	seen := map[int]bool{}
	for _, f := range rb.side[k] {
		for _, l := range t.Faces[f].Loops {
			for _, ce := range t.Loops[l].CoEdges {
				x := t.CoEdges[ce].Edge
				if isChain[x] || seen[x] {
					continue
				}
				seen[x] = true
				fs := t.EdgeFaces(x)
				g := fs[0]
				if g == f {
					g = fs[1]
				}
				if g < 0 || notNext[g] {
					continue
				}
				// The next face's height is worth finding only where the
				// contact comes near the edge: on a curved face each point
				// is a projection, and a chain round a barrel meets many.
				if !comesNear(pts, func(q at) geom.Vector { return q.p }, t.Edges[x].Points, reach) {
					continue
				}
				ro := rb.nextFace(k, g, t.Vertices[t.Edges[x].V[0]].P, ref)
				if ro == nil {
					continue
				}
				// Points further from the edge than the fillet's width
				// (and a step) stand clear of the next face for a dip
				// across this edge, whose points past it lie within that:
				// their heights are not found.
				far := 2*rb.r + reach
				hs := make([]float64, len(pts))
				for i, q := range pts {
					hs[i] = -far
					if comesNear(pts[i:i+1], func(q at) geom.Vector { return q.p }, t.Edges[x].Points, far) {
						hs[i] = ro.h(q.p)
					}
				}
				graze := dipGraze * t.shell.Tol.ChordTolMm()
				// standsClear says the foot stands clear of the face, by more than
				// graze, somewhere before i0 and somewhere from i1 on: a
				// contact that starts or ends on the next face, as at a
				// corner the chain turns, lies along it there within the
				// fit and is no dip.
				standsClear := func(i0, i1 int) bool {
					return slices.Min(hs[:i0]) < -graze && slices.Min(hs[i1:]) < -graze
				}
				if i := highest(hs); hs[i] <= 0 && hs[i] > -graze && i > 0 && i < len(pts)-1 && standsClear(i, i+1) &&
					distanceToPolyline(pts[i].p, t.Edges[x].Points) <= tol {
					return nil, filletErr(pts[i].p, "the %s's foot only just reaches the edge of its face here, where the next face "+
						"begins, too near to cut it there: make it a little smaller or larger", rb.what())
				}
				// Each run of points past the face, short of both ends.
				for i := 1; i < len(pts); i++ {
					if !(hs[i] > 0 && hs[i-1] <= 0) {
						continue
					}
					e, deepest := i, 0.0
					for e < len(pts) && hs[e] > 0 {
						deepest = math.Max(deepest, hs[e])
						e++
					}
					if e == len(pts) {
						break
					}
					if !standsClear(i, e) {
						i = e
						continue
					}
					root := func(i0, i1 int) (int, geom.Vector) {
						b := pts[i1].b
						t0 := pts[i0].t
						if pts[i0].b != b {
							t0 = 0
						}
						c := sideRow[b].EdgeCurve(ui)
						return b, c.PointAt(bezierRoot(&c, ro.h, t0, pts[i1].t))
					}
					ba, a := root(i-1, i)
					bb, bp := root(e-1, e)
					i = e
					pl := t.Edges[x].Points
					if distanceToPolyline(a, pl) > tol || distanceToPolyline(bp, pl) > tol {
						continue
					}
					if deepest < graze {
						// Within a tenth of the chord tolerance of the edge,
						// as where it only just reaches it from above: the
						// face is pinched to a point there, and neither cut
						// nor left whole comes out closed.
						return nil, filletErr(a, "the %s's foot only just reaches the edge of its face here, where the next face "+
							"begins, too near to cut it there: make it a little smaller or larger", rb.what())
					}
					out = append(out, &rbDip{ro: ro, x: x, a: a, b: bp, first: ba, after: bb + 1})
				}
			}
		}
	}
	return out, nil
}

// nextFace is face g as the face a foot of side k runs onto past its
// own, through point at of their edge: its plane, or its surface where
// it is curved, signed so that the chain's point ref lies on the side
// kept; nil where ref lies on it.
func (rb *rollingBall) nextFace(k, g int, at, ref geom.Vector) *rbRunOff {
	t := rb.t
	ro := &rbRunOff{j: k, n: g, w: -1, sideN: -1, endN: -1}
	if n, ok := t.planarNormal(g); ok {
		ro.np = n.WithMagnitude(1)
		ro.nd = ro.np.Dot(at)
		switch hv := ro.h(ref); {
		case hv > 0:
			ro.np, ro.nd = ro.np.ScaledBy(-1), -ro.nd
		case hv == 0:
			return nil
		}
		return ro
	}
	s := t.Surface(g)
	if s == nil {
		return nil
	}
	ns := *s
	ro.ns, ro.sign = &ns, 1
	switch hv := ro.h(ref); {
	case hv > 0:
		ro.sign = -1
	case hv == 0:
		return nil
	}
	return ro
}

// cutDips cuts the patches of the dips by their next faces, every row
// of them, their loops by row and patch into rowLoops, where an end's
// run-off must not have cut them (loops), and splits each side face's
// edge with its next face where the contact curve crosses it; the
// pieces of that edge between go (drop).
func (rb *rollingBall) cutDips(sh *srf.SShell, all [][]srf.SSurface, dips []*rbDip, loops map[int][]srf.SBezier,
	rowLoops map[[2]int][]srf.SBezier, drop func(a, b geom.Vector)) error {
	t := rb.t
	tol := math.Max(sh.Tol.ChordTolMm()/10, 1e-6)
	cut := map[int]bool{}
	for _, dp := range dips {
		var across []srf.SBezier
		for b := dp.first; b < dp.after; b++ {
			if _, ok := loops[b]; ok || cut[b] {
				return filletErr(dp.a, "the %s runs off its face onto the next one here near where it runs off "+
					"another: make it a little smaller or larger", rb.what())
			}
			cut[b] = true
			for r, row := range all {
				l, a, err := clipOne(&row[b], dp.ro, tol)
				if err != nil {
					return err
				}
				if l == nil {
					return filletErr(dp.a, "the %s runs off its face onto the next one here, and the next one "+
						"cuts it away: make it smaller", rb.what())
				}
				rowLoops[[2]int{r, b}] = l
				across = append(across, a...)
			}
		}
		// The crossings are where the patches' sides were cut: the
		// across curves' ends nearest those found.
		a, b := nearestEnd(across, dp.a), nearestEnd(across, dp.b)
		if a.Equals(b) {
			return filletErr(dp.a, "the %s runs off its face onto the next one here by too little to cut: "+
				"make it a little smaller or larger", rb.what())
		}
		chain, ok := runOffChain(across, a)
		if !ok || !chain[len(chain)-1].Finish().Equals(b) {
			return filletErr(dp.a, "the %s runs off its face onto the next one here and its cut there "+
				"doesn't join: make it a little smaller or larger", rb.what())
		}
		dp.a, dp.b, dp.across = a, b, chain
		v := t.Vertices[t.Edges[dp.x].V[0]].P
		if err := splitNearest(sh, dp.a, v); err != nil {
			return filletErr(dp.a, "the %s can't cut the next face's edge here: make it smaller", rb.what())
		}
		if err := splitNearest(sh, dp.b, dp.a); err != nil {
			return filletErr(dp.b, "the %s can't cut the next face's edge here: make it smaller", rb.what())
		}
		drop(dp.a, dp.b)
	}
	return nil
}

// comesNear reports whether any of pts, at(p) placing each, lies
// within reach of the polyline edge.
func comesNear[T any](pts []T, at func(T) geom.Vector, edge []geom.Vector, reach float64) bool {
	if len(edge) == 0 {
		return false
	}
	lo, hi := edge[0], edge[0]
	for _, q := range edge {
		lo = geom.Vector{X: math.Min(lo.X, q.X), Y: math.Min(lo.Y, q.Y), Z: math.Min(lo.Z, q.Z)}
		hi = geom.Vector{X: math.Max(hi.X, q.X), Y: math.Max(hi.Y, q.Y), Z: math.Max(hi.Z, q.Z)}
	}
	for _, pt := range pts {
		p := at(pt)
		if p.X < lo.X-reach || p.X > hi.X+reach || p.Y < lo.Y-reach || p.Y > hi.Y+reach ||
			p.Z < lo.Z-reach || p.Z > hi.Z+reach {
			continue
		}
		if distanceToPolyline(p, edge) <= reach {
			return true
		}
	}
	return false
}

// highest is the index of the largest of hs.
func highest(hs []float64) int {
	best := 0
	for i, h := range hs {
		if h > hs[best] {
			best = i
		}
	}
	return best
}

// nearestEnd is the end of any of curves nearest p.
func nearestEnd(curves []srf.SBezier, p geom.Vector) geom.Vector {
	best, bd := p, math.Inf(1)
	for _, c := range curves {
		for _, q := range []geom.Vector{c.Start(), c.Finish()} {
			if d := q.Minus(p).Magnitude(); d < bd {
				best, bd = q, d
			}
		}
	}
	return best
}

// dipPieces is contact curve piece sb of patch b cut where it runs past
// the next face of any of dips: the parts kept, their ends at the
// crossings moved onto the dip's a or b.
func dipPieces(sb srf.SBezier, b int, dips []*rbDip) []srf.SBezier {
	pieces := []srf.SBezier{sb}
	for _, dp := range dips {
		if b < dp.first || b >= dp.after {
			continue
		}
		var next []srf.SBezier
		for _, pc := range pieces {
			ts := []float64{0}
			const n = 64
			prev := dp.ro.h(pc.PointAt(0))
			for s := 1; s <= n; s++ {
				x := float64(s) / n
				hv := dp.ro.h(pc.PointAt(x))
				if prev < 0 && hv > 0 || prev > 0 && hv < 0 {
					ts = append(ts, bezierRoot(&pc, dp.ro.h, float64(s-1)/n, x))
				}
				prev = hv
			}
			ts = append(ts, 1)
			for s := 1; s < len(ts); s++ {
				t0, t1 := ts[s-1], ts[s]
				if t1-t0 < 1e-12 || dp.ro.h(pc.PointAt((t0+t1)/2)) > 0 {
					continue
				}
				q := pc.Sub(t0, t1)
				snapEnd := func(i int) {
					for _, e := range []geom.Vector{dp.a, dp.b} {
						if q.Ctrl[i].Minus(e).Magnitude() < 1e-6 {
							q.Ctrl[i] = e
						}
					}
				}
				if t0 > 0 {
					snapEnd(0)
				}
				if t1 < 1 {
					snapEnd(q.Degree)
				}
				next = append(next, q)
			}
		}
		pieces = next
	}
	return pieces
}

// closeDips puts each dip's cut into its next face: the face loses its
// edge with the side's face from a to b and takes the curves across
// the fillet between, growing past that edge to hold them where it is
// curved.
func (rb *rollingBall) closeDips(sh *srf.SShell, dips []*rbDip, h *core.HSCurve) error {
	e := &runOffEdit{rb: rb, sh: sh, h: h}
	for _, dp := range dips {
		if dp.ro.ns != nil {
			ns := sh.FindSurface(rb.t.Faces[dp.ro.n].Surface)
			if ns == nil {
				return filletErr(dp.a, "a face the %s runs onto is not in the shell", rb.what())
			}
			var pts []geom.Vector
			for _, sb := range dp.across {
				for _, x := range []float64{0, 0.25, 0.5, 0.75, 1} {
					pts = append(pts, sb.PointAt(x))
				}
			}
			growSurface(ns, pts)
		}
		if _, err := e.put(dp.ro.n, [][2]geom.Vector{{dp.a, dp.b}}, dp.across, dp.a, dp.b, dp.a); err != nil {
			return err
		}
	}
	return nil
}

// what is "chamfer" or "fillet", for a refusal.
func (rb *rollingBall) what() string {
	if rb.flat {
		return "chamfer"
	}
	return "fillet"
}

// dipsAt are the dips of side k, of all.
func dipsAt(all []*rbDip, k int) []*rbDip {
	return slices.DeleteFunc(slices.Clone(all), func(dp *rbDip) bool { return dp.ro.j != k })
}
