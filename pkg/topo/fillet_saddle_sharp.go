package topo

import (
	"errors"
	"math"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// Saddle corners with an edge left sharp: four flat faces and four
// straight edges of both kinds, as the middle of a saddle where two
// ridges and two valleys meet, three of them rounded and the fourth
// left sharp. OpenCASCADE fills such a corner with one vertex blend;
// a setback whose hole folds to a point on the sharp edge, each face
// beside it taking a curve to that point, turns over beside it whatever
// the curves (tried 2026-10-29).
//
// Here, as at a blend corner (fillet_blend.go), the two fillets beside
// the sharp edge, m and o, run their tangent lines on the faces they
// share with it up to it, to the one point P where both lines meet it,
// and each is cut obliquely, from a point S on its other face up its
// rounded face to P. The third fillet, across the corner from the sharp
// edge, is cut square to its edge, as at a setback. The hole is then six
// sided: o's cut from S3 to P, a side folded to P, m's cut from P to S2,
// the face's curve from S2 to the third fillet's section, that section,
// and the other face's curve back to S3: a star of three patches, the
// fold one side of the first, where its normal turns from o's rounded
// face's to m's (setbackUnit).
//
// A chamfer's corner is cut the same way, its flat faces' lines in place
// of the tangent lines, each cut a straight line across its face; the
// patches between are curved as a fillet's are, as OpenCASCADE's are,
// though set back further (TestChamferSaddleSharpOracle).
//
// Only where m's and o's tangent lines meet the sharp edge at one point:
// where they don't, each would end at its own point and a face's curve
// between them along the sharp edge would make the hole seven sided,
// which a star doesn't take.

// saddleSharpTakes reports whether vertex v is a corner saddleSharpAt
// fills: four flat faces and four straight edges, one left sharp and
// three rounded, none circular.
func (t *Topology) saddleSharpTakes(v int, edges []filletEdge, byEdge map[int]int) bool {
	vt := &t.Vertices[v]
	if len(vt.Edges) != 4 || len(t.facesAt(v)) != 4 {
		return false
	}
	sharp := 0
	for _, x := range vt.Edges {
		if !t.Straight(x) {
			return false
		}
		i, ok := byEdge[x]
		if !ok {
			sharp++
			continue
		}
		if edges[i].circ || edges[i].varies() {
			return false
		}
	}
	if sharp != 1 {
		return false
	}
	for _, f := range t.facesAt(v) {
		if _, ok := t.planarNormal(f); !ok {
			return false
		}
	}
	return true
}

// saddleSharpAt works out the saddle corner at v whose one edge is left
// sharp (saddleSharpTakes).
func (t *Topology) saddleSharpAt(v int, edges []filletEdge, byEdge map[int]int) (*filletCorner, error) {
	p := t.Vertices[v].P
	sharp := -1
	var round []*filletEdge
	for _, x := range t.Vertices[v].Edges {
		i, ok := byEdge[x]
		if !ok {
			sharp = x
			continue
		}
		round = append(round, &edges[i])
	}
	sf := t.EdgeFaces(sharp)
	onSharp := func(fe *filletEdge) int {
		for _, f := range fe.f {
			if f == sf[0] || f == sf[1] {
				return f
			}
		}
		return -1
	}
	var m, o, r *filletEdge
	for _, fe := range round {
		switch {
		case onSharp(fe) < 0:
			r = fe
		case m == nil:
			m = fe
		default:
			o = fe
		}
	}
	if m == nil || o == nil || r == nil || onSharp(m) == onSharp(o) {
		return nil, filletErr(p, "the corner's edges and faces don't go round it one after another")
	}
	fsm, fso := onSharp(m), onSharp(o)
	fmr, fro := sharedFace(m, r), sharedFace(r, o)
	if fmr < 0 || fro < 0 {
		return nil, filletErr(p, "the corner's edges and faces don't go round it one after another")
	}
	am, ao, ar := awayFrom(m, p, t), awayFrom(o, p, t), awayFrom(r, p, t)
	away := t.edgeAway(sharp, v)
	line := func(fe *filletEdge, f int) geom.Vector {
		k := faceK(fe, f)
		return p.Plus(fe.u[k].ScaledBy(fe.ss[k]))
	}
	pm, okm := lineMeet(line(m, fsm), am, p, away)
	po, oko := lineMeet(line(o, fso), ao, p, away)
	if !okm || !oko {
		return nil, filletErr(p, "a fillet runs along the edge left sharp")
	}
	if pm.Minus(po).Magnitude() > 10*geom.LengthEps {
		return nil, errSaddleTwoPoints
	}
	pp := pm
	reach := pp.Minus(p).Dot(away)
	if reach < 10*geom.LengthEps || reach > t.straightLen(sharp)-10*geom.LengthEps {
		return nil, errSaddleTwoPoints
	}
	rmax := math.Max(m.r, math.Max(o.r, r.r))
	share := setbackShare * rmax
	x2, ok2 := lineMeet(line(m, fmr), am, line(r, fmr), ar)
	x3, ok3 := lineMeet(line(r, fro), ar, line(o, fro), ao)
	if !ok2 || !ok3 {
		return nil, filletErr(p, "two fillets at the corner run along one another")
	}
	dm := x2.Minus(line(m, fmr)).Dot(am) + share
	do := x3.Minus(line(o, fro)).Dot(ao) + share
	dr := math.Max(x2.Minus(line(r, fmr)).Dot(ar), x3.Minus(line(r, fro)).Dot(ar)) + share
	for _, x := range []struct {
		fe *filletEdge
		d  float64
	}{{m, dm}, {o, do}, {r, dr}} {
		if x.d > t.edgeLen(x.fe)-10*geom.LengthEps {
			return nil, filletErr(p, "the edge is too short for a fillet this large")
		}
	}
	s2 := line(m, fmr).Plus(am.ScaledBy(dm))
	s3 := line(o, fro).Plus(ao.ScaledBy(do))
	rsec, err := quinticArc(r.section(p.Plus(ar.ScaledBy(dr)), false))
	if err != nil {
		return nil, filletErr(p, "the fillet's section at the corner can't be made: %v", err)
	}
	if faceK(r, fmr) == 1 {
		rsec.Reverse()
	}
	r2, r3 := rsec.Start(), rsec.Finish()
	// Each cut is where a plane through S and P, across the fillet's
	// section, cuts its rounded face: an ellipse from its tangent line
	// on one face to the other's.
	bulge := func(fe *filletEdge) geom.Vector { return fe.dir.Cross(fe.u[1].Minus(fe.u[0])) }
	mcut, err := planeCut(m, fmr, s2, pp.Minus(s2).Cross(bulge(m)), p)
	if err != nil {
		return nil, filletErr(p, "the fillet's end at the corner can't be made: %v", err)
	}
	ocut, err := planeCut(o, fro, s3, pp.Minus(s3).Cross(bulge(o)), p)
	if err != nil {
		return nil, filletErr(p, "the fillet's end at the corner can't be made: %v", err)
	}
	if !mcut.Finish().Equals(pp) || !ocut.Finish().Equals(pp) {
		return nil, filletErr(pp, "the fillet's end at the corner misses the edge left sharp")
	}
	mcut.Ctrl[0], mcut.Ctrl[patchDeg] = s2, pp
	ocut.Ctrl[0], ocut.Ctrl[patchDeg] = s3, pp
	back := mcut
	back.Reverse()
	nmr, _ := t.planarNormal(fmr)
	nro, _ := t.planarNormal(fro)
	var facing geom.Vector
	for _, f := range t.facesAt(v) {
		n, _ := t.planarNormal(f)
		facing = facing.Plus(n)
	}
	var firstErr error
	for _, lean := range saddleSharpLeans {
		fold := setbackSide{fold: true, lean: lean}
		for k := range fold.ctrl {
			fold.ctrl[k] = pp
		}
		for _, leg := range setbackLegs {
			c2, err := setbackFaceCurve(s2, x2, r2, leg)
			if err != nil {
				return nil, filletErr(p, "the corner's curve on a face can't be made: %v", err)
			}
			c3, err := setbackFaceCurve(r3, x3, s3, leg)
			if err != nil {
				return nil, filletErr(p, "the corner's curve on a face can't be made: %v", err)
			}
			sides := []setbackSide{
				{ctrl: quinticCtrl(ocut), arc: true, e: o.dir},
				fold,
				{ctrl: quinticCtrl(back), arc: true, e: m.dir},
				{ctrl: quinticCtrl(c2), e: nmr},
				{ctrl: quinticCtrl(rsec), arc: true, e: r.dir},
				{ctrl: quinticCtrl(c3), e: nro},
			}
			quads, err := setbackPatches(sides, facing)
			if err != nil {
				if firstErr == nil {
					firstErr = err
				}
				continue
			}
			c := &filletCorner{kind: setbackCorner, at: map[[2]int]geom.Vector{}, long: -1, r: rmax,
				arcs:       map[int]srf.SBezier{m.e: mcut, o.e: ocut, r.e: rsec},
				faceCurves: map[int]srf.SBezier{fmr: c2, fro: c3}, quads: quads}
			c.at[[2]int{m.e, fmr}] = s2
			c.at[[2]int{m.e, fsm}] = pp
			c.at[[2]int{o.e, fro}] = s3
			c.at[[2]int{o.e, fso}] = pp
			c.at[[2]int{r.e, fmr}] = r2
			c.at[[2]int{r.e, fro}] = r3
			c.at[[2]int{sharp, fsm}] = pp
			c.at[[2]int{sharp, fso}] = pp
			return c, nil
		}
	}
	if _, at := errors.AsType[*FilletError](firstErr); at {
		return nil, firstErr
	}
	return nil, filletErr(p, "the corner can't be filled: %v", firstErr)
}

// errSaddleTwoPoints refuses a saddle corner whose two fillets beside
// the sharp edge meet it at different points.
var errSaddleTwoPoints = errors.New("the fillets beside the edge left sharp don't reach it at one point")

// saddleSharpLeans are the folded patch's leans tried in turn
// (setbackSide.lean): held three times harder than fairness made every
// saddle of TestFilletSaddleSharp that any did, ten some of the rest;
// not held harder, the row beside o's cut turns under it short of P.
var saddleSharpLeans = []float64{3, 10, 30}
