package topo

import (
	"errors"
	"math"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// Saddle corners with a ridge and a valley rounded side by side, the
// other two edges left sharp: four flat faces and four straight edges
// at the middle of a saddle, the two rounded sharing a face, fmo, and
// the two sharp sharing one, fss. OpenCASCADE fills the corner with one
// five-sided patch; in stages, each ending on the faces across it
// (fillet_crease_end.go), the corner depends on which goes first.
//
// Here each fillet runs its tangent line on its other face, the one it
// shares with a sharp edge, up to that edge, m's to Pm, o's to Po, as
// beside the sharp edge of saddleSharpAt; each is cut obliquely from a
// point S on fmo up its rounded face to its P. The hole is then five
// sided: o's cut from So to Po, the sharp edge from Po back to the
// corner V, the other from V to Pm, m's cut from Pm to Sm, and fmo's
// curve from Sm to So. The two sharp edges' pieces stay sharp, the patch
// meeting fss at an angle there (setbackSide.free), and the hole's
// corner at V is wider than a straight angle (setbackSide.reflex): a
// ladder's rung must leave it. With fss's piece of Po's edge split in
// two and fmo's curve in three, the hole has eight sides and a ladder of
// three patches whose rungs run from V and from the split on the sharp
// edge to the splits on fmo's curve.

// saddleSide is a saddle corner with two edges rounded side by side, as
// saddleSideOf works it out: m and o the fillets, sharing fmo, each also
// along fsm and fso, which it shares with sharp edge sm or so; fss the
// face of both sharp edges; pm and po where m's and o's tangent lines
// meet their sharp edges, sm and so where they are cut on fmo, x where
// their tangent lines cross there.
type saddleSide struct {
	p                  geom.Vector
	m, o               *filletEdge
	sm, so             int
	fmo, fsm, fso, fss int
	am, ao             geom.Vector
	pm, po             geom.Vector
	smp, sop, x        geom.Vector
	rmax               float64
	facing             geom.Vector
}

// saddleSideTakes reports whether vertex v is a corner saddleSideAt
// fills: four flat faces and four straight edges, two side by side
// rounded, alike and not circular, and the other two left sharp.
func (t *Topology) saddleSideTakes(v int, edges []filletEdge, byEdge map[int]int) bool {
	vt := &t.Vertices[v]
	if len(vt.Edges) != 4 || len(t.facesAt(v)) != 4 {
		return false
	}
	var round []*filletEdge
	var sharp []int
	for _, x := range vt.Edges {
		if !t.Straight(x) {
			return false
		}
		i, ok := byEdge[x]
		if !ok {
			sharp = append(sharp, x)
			continue
		}
		if edges[i].circ || edges[i].varies() {
			return false
		}
		round = append(round, &edges[i])
	}
	if len(round) != 2 || sharedFace(round[0], round[1]) < 0 {
		return false
	}
	a, b := t.EdgeFaces(sharp[0]), t.EdgeFaces(sharp[1])
	if a[0] != b[0] && a[0] != b[1] && a[1] != b[0] && a[1] != b[1] {
		return false
	}
	for _, f := range t.facesAt(v) {
		if _, ok := t.planarNormal(f); !ok {
			return false
		}
	}
	return true
}

// saddleSideOf finds the corner's fillets and faces, where each fillet
// meets its sharp edge, and where each is cut on the face they share.
func (t *Topology) saddleSideOf(v int, edges []filletEdge, byEdge map[int]int) (*saddleSide, error) {
	g := &saddleSide{p: t.Vertices[v].P}
	p := g.p
	var sharp []int
	for _, x := range t.Vertices[v].Edges {
		i, ok := byEdge[x]
		if !ok {
			sharp = append(sharp, x)
			continue
		}
		if g.m == nil {
			g.m = &edges[i]
		} else {
			g.o = &edges[i]
		}
	}
	m, o := g.m, g.o
	g.fmo = sharedFace(m, o)
	g.fsm, g.fso = otherFace(m, o), otherFace(o, m)
	along := func(x, f int) bool {
		ef := t.EdgeFaces(x)
		return ef[0] == f || ef[1] == f
	}
	switch {
	case along(sharp[0], g.fsm) && along(sharp[1], g.fso):
		g.sm, g.so = sharp[0], sharp[1]
	case along(sharp[1], g.fsm) && along(sharp[0], g.fso):
		g.sm, g.so = sharp[1], sharp[0]
	default:
		return nil, filletErr(p, "the corner's edges and faces don't go round it one after another")
	}
	g.fss = -1
	for _, f := range t.EdgeFaces(g.sm) {
		if along(g.so, f) {
			g.fss = f
		}
	}
	if g.fss < 0 {
		return nil, filletErr(p, "the corner's edges and faces don't go round it one after another")
	}
	g.am, g.ao = awayFrom(m, p, t), awayFrom(o, p, t)
	line := func(fe *filletEdge, f int) geom.Vector {
		k := faceK(fe, f)
		return p.Plus(fe.u[k].ScaledBy(fe.ss[k]))
	}
	var okm, oko bool
	g.pm, okm = lineMeet(line(m, g.fsm), g.am, p, t.edgeAway(g.sm, v))
	g.po, oko = lineMeet(line(o, g.fso), g.ao, p, t.edgeAway(g.so, v))
	if !okm || !oko {
		return nil, filletErr(p, "a fillet runs along an edge left sharp")
	}
	for _, x := range []struct {
		q geom.Vector
		e int
	}{{g.pm, g.sm}, {g.po, g.so}} {
		if reach := x.q.Minus(p).Dot(t.edgeAway(x.e, v)); reach < 10*geom.LengthEps || reach > t.straightLen(x.e)-10*geom.LengthEps {
			return nil, errSaddlePastEdge
		}
	}
	g.rmax = math.Max(m.r, o.r)
	share := setbackShare * g.rmax
	var ok bool
	g.x, ok = lineMeet(line(m, g.fmo), g.am, line(o, g.fmo), g.ao)
	if !ok {
		return nil, filletErr(p, "two fillets at the corner run along one another")
	}
	dm := g.x.Minus(line(m, g.fmo)).Dot(g.am) + share
	do := g.x.Minus(line(o, g.fmo)).Dot(g.ao) + share
	if dm > t.edgeLen(m)-10*geom.LengthEps || do > t.edgeLen(o)-10*geom.LengthEps {
		return nil, filletErr(p, "the edge is too short for a fillet this large")
	}
	g.smp = line(m, g.fmo).Plus(g.am.ScaledBy(dm))
	g.sop = line(o, g.fmo).Plus(g.ao.ScaledBy(do))
	for _, f := range t.facesAt(v) {
		n, _ := t.planarNormal(f)
		g.facing = g.facing.Plus(n)
	}
	return g, nil
}

// saddleSideAt works out the saddle corner at v with two edges rounded
// side by side (saddleSideTakes).
func (t *Topology) saddleSideAt(v int, edges []filletEdge, byEdge map[int]int) (*filletCorner, error) {
	g, err := t.saddleSideOf(v, edges, byEdge)
	if err != nil {
		return nil, err
	}
	p, m, o := g.p, g.m, g.o
	bulge := func(fe *filletEdge) geom.Vector { return fe.dir.Cross(fe.u[1].Minus(fe.u[0])) }
	mcut, err := planeCut(m, g.fmo, g.smp, g.pm.Minus(g.smp).Cross(bulge(m)), p)
	if err != nil {
		return nil, filletErr(p, "the fillet's end at the corner can't be made: %v", err)
	}
	ocut, err := planeCut(o, g.fmo, g.sop, g.po.Minus(g.sop).Cross(bulge(o)), p)
	if err != nil {
		return nil, filletErr(p, "the fillet's end at the corner can't be made: %v", err)
	}
	if !mcut.Finish().Equals(g.pm) || !ocut.Finish().Equals(g.po) {
		return nil, filletErr(p, "the fillet's end at the corner misses the edge left sharp")
	}
	mcut.Ctrl[0], mcut.Ctrl[patchDeg] = g.smp, g.pm
	ocut.Ctrl[0], ocut.Ctrl[patchDeg] = g.sop, g.po
	back := mcut
	back.Reverse()
	nmo, _ := t.planarNormal(g.fmo)
	nsm, _ := t.planarNormal(g.fsm)
	nso, _ := t.planarNormal(g.fso)
	line := func(a, b geom.Vector) (srf.SBezier, error) {
		sb := srf.SBezierFrom2(a, b)
		return sb.ElevatedTo(patchDeg)
	}
	la, err := line(g.po, p)
	if err != nil {
		return nil, filletErr(p, "the corner's edge can't be made: %v", err)
	}
	lb, err := line(p, g.pm)
	if err != nil {
		return nil, filletErr(p, "the corner's edge can't be made: %v", err)
	}
	var firstErr error
	for _, sp := range saddleSideSplits {
		for _, leg := range setbackLegs {
			c, err := setbackFaceCurve(g.smp, g.x, g.sop, leg)
			if err != nil {
				return nil, filletErr(p, "the corner's curve on a face can't be made: %v", err)
			}
			ca, cb, cc := c.Sub(0, sp.c[0]), c.Sub(sp.c[0], sp.c[1]), c.Sub(sp.c[1], 1)
			// The sharp edges' pieces, Po to V and V to Pm, one of them
			// split for the ladder's second rung.
			var edgeSides []setbackSide
			var fss []srf.SBezier
			if sp.onO {
				la1, la2 := la.Sub(0, sp.l), la.Sub(sp.l, 1)
				fss = []srf.SBezier{la1, la2, lb}
				edgeSides = []setbackSide{
					{ctrl: quinticCtrl(la1), e: nso, free: true},
					{ctrl: quinticCtrl(la2), e: nso, free: true, smooth: true},
					{ctrl: quinticCtrl(lb), e: nsm, free: true, reflex: true},
				}
			} else {
				lb1, lb2 := lb.Sub(0, sp.l), lb.Sub(sp.l, 1)
				fss = []srf.SBezier{la, lb1, lb2}
				edgeSides = []setbackSide{
					{ctrl: quinticCtrl(la), e: nso, free: true},
					{ctrl: quinticCtrl(lb1), e: nsm, free: true, reflex: true},
					{ctrl: quinticCtrl(lb2), e: nsm, free: true, smooth: true},
				}
			}
			sides := []setbackSide{{ctrl: quinticCtrl(ocut), arc: true, e: o.dir}}
			sides = append(sides, edgeSides...)
			sides = append(sides,
				setbackSide{ctrl: quinticCtrl(back), arc: true, e: m.dir},
				setbackSide{ctrl: quinticCtrl(ca), e: nmo},
				setbackSide{ctrl: quinticCtrl(cb), e: nmo, smooth: true},
				setbackSide{ctrl: quinticCtrl(cc), e: nmo, smooth: true})
			quads, err := setbackPatches(sides, g.facing)
			if err != nil {
				if firstErr == nil {
					firstErr = err
				}
				continue
			}
			cr := &filletCorner{kind: setbackCorner, at: map[[2]int]geom.Vector{}, long: -1, r: g.rmax,
				faceCurves: map[int]srf.SBezier{}, quads: quads,
				arcs: map[int]srf.SBezier{m.e: mcut, o.e: ocut},
				faceSplits: map[int][]srf.SBezier{
					g.fmo: {ca, cb, cc},
					g.fss: fss,
				}}
			cr.at[[2]int{m.e, g.fmo}] = g.smp
			cr.at[[2]int{m.e, g.fsm}] = g.pm
			cr.at[[2]int{o.e, g.fmo}] = g.sop
			cr.at[[2]int{o.e, g.fso}] = g.po
			cr.at[[2]int{g.sm, g.fsm}] = g.pm
			cr.at[[2]int{g.sm, g.fss}] = g.pm
			cr.at[[2]int{g.so, g.fso}] = g.po
			cr.at[[2]int{g.so, g.fss}] = g.po
			return cr, nil
		}
	}
	if _, at := errors.AsType[*FilletError](firstErr); at {
		return nil, firstErr
	}
	return nil, filletErr(p, "the corner can't be filled: %v", firstErr)
}

// saddleSideSplits are where the hole of a saddleSideAt corner is split
// for its ladder, tried in turn: on Po's sharp edge or Pm's (onO), the
// share along it (l), and the shares along fmo's curve (c).
var saddleSideSplits = []struct {
	onO bool
	l   float64
	c   [2]float64
}{
	{true, 0.5, [2]float64{1.0 / 3, 2.0 / 3}},
	{false, 0.5, [2]float64{1.0 / 3, 2.0 / 3}},
	{true, 0.5, [2]float64{0.25, 0.75}},
	{false, 0.5, [2]float64{0.25, 0.75}},
	{true, 0.35, [2]float64{0.4, 0.6}},
	{false, 0.65, [2]float64{0.4, 0.6}},
}
