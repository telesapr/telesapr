package topo

import (
	"errors"
	"math"
	"slices"

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
// Where the star folds (a steep saddle, slopes of 0.8), both faces'
// curves are split in two, the hole eight sided, and filled by a ladder
// of three patches whose rungs end where the curves are split
// (setbackSide.smooth).
//
// Where m's and o's tangent lines meet the sharp edge at two points (the
// two of unlike radii), the nearer one's face beside the sharp edge is
// kept up to the further point, along the edge: the nearer fillet is cut
// short of the edge, and its face takes a curve from there to the
// further point, where the fold is (saddleSharpBack). The hole is then
// seven sided, and one face's curve is split for a ladder of three.

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

// saddleSharp is a saddle corner with an edge left sharp, as
// saddleSharpAt works it out: m and o the fillets beside the sharp edge,
// on faces fsm and fso, r the third, sharing fmr with m and fro with o;
// pm and po where m's and o's tangent lines meet the sharp edge, S2 and
// S3 where m and o are cut on fmr and fro, x2 and x3 where the tangent
// lines cross on those faces, and r's section rsec.
type saddleSharp struct {
	p                  geom.Vector
	sharp              int
	m, o, r            *filletEdge
	fsm, fso, fmr, fro int
	am, ao             geom.Vector
	pm, po             geom.Vector
	two, nearM         bool
	s2, s3, x2, x3     geom.Vector
	rsec               srf.SBezier
	rmax               float64
	facing             geom.Vector
}

// saddleSharpAt works out the saddle corner at v whose one edge is left
// sharp (saddleSharpTakes).
func (t *Topology) saddleSharpAt(v int, edges []filletEdge, byEdge map[int]int) (*filletCorner, error) {
	g, err := t.saddleSharpOf(v, edges, byEdge)
	if err != nil {
		return nil, err
	}
	backs := []float64{0}
	if g.two {
		backs = saddleSharpBacks
	}
	var firstErr error
	for _, bk := range backs {
		c, err := t.saddleSharpBack(g, bk)
		if err == nil {
			return c, nil
		}
		if firstErr == nil {
			firstErr = err
		}
	}
	if _, at := errors.AsType[*FilletError](firstErr); at {
		return nil, firstErr
	}
	return nil, filletErr(g.p, "the corner can't be filled: %v", firstErr)
}

// saddleSharpOf finds the saddle corner's fillets and faces, where each
// meets the sharp edge, and where each is cut.
func (t *Topology) saddleSharpOf(v int, edges []filletEdge, byEdge map[int]int) (*saddleSharp, error) {
	g := &saddleSharp{p: t.Vertices[v].P, sharp: -1}
	p := g.p
	var round []*filletEdge
	for _, x := range t.Vertices[v].Edges {
		i, ok := byEdge[x]
		if !ok {
			g.sharp = x
			continue
		}
		round = append(round, &edges[i])
	}
	sf := t.EdgeFaces(g.sharp)
	onSharp := func(fe *filletEdge) int {
		for _, f := range fe.f {
			if f == sf[0] || f == sf[1] {
				return f
			}
		}
		return -1
	}
	for _, fe := range round {
		switch {
		case onSharp(fe) < 0:
			g.r = fe
		case g.m == nil:
			g.m = fe
		default:
			g.o = fe
		}
	}
	m, o, r := g.m, g.o, g.r
	if m == nil || o == nil || r == nil || onSharp(m) == onSharp(o) {
		return nil, filletErr(p, "the corner's edges and faces don't go round it one after another")
	}
	g.fsm, g.fso = onSharp(m), onSharp(o)
	g.fmr, g.fro = sharedFace(m, r), sharedFace(r, o)
	if g.fmr < 0 || g.fro < 0 {
		return nil, filletErr(p, "the corner's edges and faces don't go round it one after another")
	}
	ar := awayFrom(r, p, t)
	g.am, g.ao = awayFrom(m, p, t), awayFrom(o, p, t)
	away := t.edgeAway(g.sharp, v)
	line := func(fe *filletEdge, f int) geom.Vector {
		k := faceK(fe, f)
		return p.Plus(fe.u[k].ScaledBy(fe.ss[k]))
	}
	var okm, oko bool
	g.pm, okm = lineMeet(line(m, g.fsm), g.am, p, away)
	g.po, oko = lineMeet(line(o, g.fso), g.ao, p, away)
	if !okm || !oko {
		return nil, filletErr(p, "a fillet runs along the edge left sharp")
	}
	for _, q := range []geom.Vector{g.pm, g.po} {
		if reach := q.Minus(p).Dot(away); reach < 10*geom.LengthEps || reach > t.straightLen(g.sharp)-10*geom.LengthEps {
			return nil, errSaddlePastEdge
		}
	}
	g.two = g.pm.Minus(g.po).Magnitude() > 10*geom.LengthEps
	if !g.two {
		g.po = g.pm
	}
	g.nearM = g.two && g.pm.Minus(p).Dot(away) < g.po.Minus(p).Dot(away)
	g.rmax = math.Max(m.r, math.Max(o.r, r.r))
	share := setbackShare * g.rmax
	var ok2, ok3 bool
	g.x2, ok2 = lineMeet(line(m, g.fmr), g.am, line(r, g.fmr), ar)
	g.x3, ok3 = lineMeet(line(r, g.fro), ar, line(o, g.fro), g.ao)
	if !ok2 || !ok3 {
		return nil, filletErr(p, "two fillets at the corner run along one another")
	}
	dm := g.x2.Minus(line(m, g.fmr)).Dot(g.am) + share
	do := g.x3.Minus(line(o, g.fro)).Dot(g.ao) + share
	dr := math.Max(g.x2.Minus(line(r, g.fmr)).Dot(ar), g.x3.Minus(line(r, g.fro)).Dot(ar)) + share
	for _, x := range []struct {
		fe *filletEdge
		d  float64
	}{{m, dm}, {o, do}, {r, dr}} {
		if x.d > t.edgeLen(x.fe)-10*geom.LengthEps {
			return nil, filletErr(p, "the edge is too short for a fillet this large")
		}
	}
	g.s2 = line(m, g.fmr).Plus(g.am.ScaledBy(dm))
	g.s3 = line(o, g.fro).Plus(g.ao.ScaledBy(do))
	rsec, err := quinticArc(r.section(p.Plus(ar.ScaledBy(dr)), false))
	if err != nil {
		return nil, filletErr(p, "the fillet's section at the corner can't be made: %v", err)
	}
	if faceK(r, g.fmr) == 1 {
		rsec.Reverse()
	}
	g.rsec = rsec
	for _, f := range t.facesAt(v) {
		n, _ := t.planarNormal(f)
		g.facing = g.facing.Plus(n)
	}
	return g, nil
}

// saddleSharpBack fills the corner g with the nearer of two fillets
// reaching the sharp edge at two points cut back from it by bk times the
// gap between them.
//
// That nearer one's face beside the sharp edge is kept up to the further
// point pf along the edge: it is cut short of the edge at tn, back along
// its tangent line, and its face's boundary runs from there to pf on a
// curve leaving along the line. Ended at the edge, the hole's corner
// there would be more than a straight angle, where no patch turns. The
// fold is at pf.
func (t *Topology) saddleSharpBack(g *saddleSharp, bk float64) (*filletCorner, error) {
	p, m, o, r := g.p, g.m, g.o, g.r
	pf, pn, near, nearDir := g.pm, g.po, g.fso, g.ao
	if g.nearM {
		pf, pn, near, nearDir = g.po, g.pm, g.fsm, g.am
	}
	me, oe := g.pm, g.po
	var join srf.SBezier
	if g.two {
		tn := pn.Plus(nearDir.ScaledBy(bk * g.pm.Minus(g.po).Magnitude()))
		l := tn.Minus(pf).Magnitude() / 3
		cubic := srf.SBezierFrom4(tn, tn.Minus(nearDir.ScaledBy(l)), pf.Plus(nearDir.ScaledBy(l)), pf)
		c, err := cubic.ElevatedTo(patchDeg)
		if err != nil {
			return nil, filletErr(p, "the corner's curve on a face can't be made: %v", err)
		}
		join = c
		if g.nearM {
			me = tn
			// Round the hole from the fold at pf to m's cut at tn.
			join.Reverse()
		} else {
			oe = tn
		}
	}
	// Each cut is where a plane through S and its end, across the
	// fillet's section, cuts its rounded face: an ellipse from its
	// tangent line on one face to the other's.
	bulge := func(fe *filletEdge) geom.Vector { return fe.dir.Cross(fe.u[1].Minus(fe.u[0])) }
	mcut, err := planeCut(m, g.fmr, g.s2, me.Minus(g.s2).Cross(bulge(m)), p)
	if err != nil {
		return nil, filletErr(p, "the fillet's end at the corner can't be made: %v", err)
	}
	ocut, err := planeCut(o, g.fro, g.s3, oe.Minus(g.s3).Cross(bulge(o)), p)
	if err != nil {
		return nil, filletErr(p, "the fillet's end at the corner can't be made: %v", err)
	}
	if !mcut.Finish().Equals(me) || !ocut.Finish().Equals(oe) {
		return nil, filletErr(me, "the fillet's end at the corner misses the edge left sharp")
	}
	mcut.Ctrl[0], mcut.Ctrl[patchDeg] = g.s2, me
	ocut.Ctrl[0], ocut.Ctrl[patchDeg] = g.s3, oe
	back := mcut
	back.Reverse()
	nnear, _ := t.planarNormal(near)
	nmr, _ := t.planarNormal(g.fmr)
	nro, _ := t.planarNormal(g.fro)
	r2, r3 := g.rsec.Start(), g.rsec.Finish()
	var firstErr error
	for _, lean := range saddleSharpLeans {
		fold := setbackSide{fold: true, lean: lean}
		for k := range fold.ctrl {
			fold.ctrl[k] = pf
		}
		for _, leg := range setbackLegs {
			c2, err := setbackFaceCurve(g.s2, g.x2, r2, leg)
			if err != nil {
				return nil, filletErr(p, "the corner's curve on a face can't be made: %v", err)
			}
			c3, err := setbackFaceCurve(r3, g.x3, g.s3, leg)
			if err != nil {
				return nil, filletErr(p, "the corner's curve on a face can't be made: %v", err)
			}
			sides := []setbackSide{{ctrl: quinticCtrl(ocut), arc: true, e: o.dir}}
			faceOf := map[int]int{}
			switch {
			case g.two && !g.nearM:
				faceOf[len(sides)] = g.fso
				sides = append(sides, setbackSide{ctrl: quinticCtrl(join), e: nnear}, fold)
			case g.two:
				faceOf[len(sides)+1] = g.fsm
				sides = append(sides, fold, setbackSide{ctrl: quinticCtrl(join), e: nnear})
			default:
				sides = append(sides, fold)
			}
			sides = append(sides, setbackSide{ctrl: quinticCtrl(back), arc: true, e: m.dir})
			faceOf[len(sides)] = g.fmr
			sides = append(sides, setbackSide{ctrl: quinticCtrl(c2), e: nmr},
				setbackSide{ctrl: quinticCtrl(g.rsec), arc: true, e: r.dir})
			faceOf[len(sides)] = g.fro
			sides = append(sides, setbackSide{ctrl: quinticCtrl(c3), e: nro})
			c, err := saddleSharpFill(sides, faceOf, g.two, g.facing)
			if err != nil {
				if firstErr == nil {
					firstErr = err
				}
				continue
			}
			c.r = g.rmax
			c.arcs = map[int]srf.SBezier{m.e: mcut, o.e: ocut, r.e: g.rsec}
			for f, sb := range map[int]srf.SBezier{g.fmr: c2, g.fro: c3} {
				if _, ok := c.faceSplits[f]; !ok {
					c.faceCurves[f] = sb
				}
			}
			if _, ok := c.faceSplits[near]; g.two && !ok {
				c.faceSplits[near] = []srf.SBezier{join}
			}
			c.at[[2]int{m.e, g.fmr}] = g.s2
			c.at[[2]int{m.e, g.fsm}] = me
			c.at[[2]int{o.e, g.fro}] = g.s3
			c.at[[2]int{o.e, g.fso}] = oe
			c.at[[2]int{r.e, g.fmr}] = r2
			c.at[[2]int{r.e, g.fro}] = r3
			c.at[[2]int{g.sharp, g.fsm}] = pf
			c.at[[2]int{g.sharp, g.fso}] = pf
			return c, nil
		}
	}
	return nil, firstErr
}

// errSaddlePastEdge refuses a saddle corner whose fillets beside the
// sharp edge reach it past its far end.
var errSaddlePastEdge = errors.New("the fillets beside the edge left sharp reach it past its end")

// saddleSharpLeans are the folded patch's leans tried in turn
// (setbackSide.lean): held three times harder than fairness made every
// saddle of TestFilletSaddleSharp that any did, ten some of the rest;
// not held harder, the row beside o's cut turns under it short of P.
var saddleSharpLeans = []float64{3, 10, 30}

// saddleSharpBacks are how far back from the sharp edge the nearer of
// two fillets reaching it at two points is cut, in shares of the gap
// between the points, tried in turn.
var saddleSharpBacks = []float64{1, 2, 0.5}

// saddleSharpFill fills the hole of a saddle corner with an edge left
// sharp, its sides round it and faceOf the face of each side that lies
// on one: as it is where the hole has six sides, else, or where that
// folds, with one face's curve split in two (two: seven sides) or both
// (six), for a ladder of three.
func saddleSharpFill(sides []setbackSide, faceOf map[int]int, two bool, facing geom.Vector) (*filletCorner, error) {
	var onFace []int
	for i := range sides {
		if _, ok := faceOf[i]; ok {
			onFace = append(onFace, i)
		}
	}
	var splits [][]int
	if two {
		for _, i := range onFace {
			splits = append(splits, []int{i})
		}
	} else {
		splits = [][]int{nil, onFace}
	}
	var firstErr error
	for _, sp := range splits {
		var hole []setbackSide
		split := map[int][]srf.SBezier{}
		for i, sd := range sides {
			if !slices.Contains(sp, i) {
				hole = append(hole, sd)
				continue
			}
			sb := sd.bezier()
			a, b := sb.Sub(0, 0.5), sb.Sub(0.5, 1)
			hole = append(hole, setbackSide{ctrl: quinticCtrl(a), e: sd.e},
				setbackSide{ctrl: quinticCtrl(b), e: sd.e, smooth: true})
			split[faceOf[i]] = []srf.SBezier{a, b}
		}
		quads, err := setbackPatches(hole, facing)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		return &filletCorner{kind: setbackCorner, at: map[[2]int]geom.Vector{}, long: -1,
			faceCurves: map[int]srf.SBezier{}, faceSplits: split, quads: quads}, nil
	}
	return nil, firstErr
}
