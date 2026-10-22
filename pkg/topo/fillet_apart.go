package topo

import (
	"errors"
	"math"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// Picks apart: a pick whose fillet only the rolling ball makes (an edge
// between curved faces, or one between flat faces that ends on a curved
// face) can't share a construction with other picks, and was refused
// when picked with them. Where its chain touches no other pick's edge,
// its fillet doesn't meet theirs, and the picks are rounded one after
// another: those step 10 takes, together, then each rolling one alone in
// the solid as it then is.

// errNotApart says that filletApart doesn't take the picks: none needs
// the rolling ball, or too many do.
var errNotApart = errors.New("topo: the picks don't lie apart")

// apartOrigins is where the rolling picks' faces start in origin, past
// the stages' (filletInStages); one stage's room for each pick.
const apartOrigins = 5 * stageOrigins

// maxApart is how many rolling picks fit in the origins left.
const maxApart = (0x10000 - apartOrigins) / stageOrigins

// filletApart rounds picks of sh with radius r, or chamfers them where
// flat, one after another, the rolling ones each alone; errNotApart where none is rolling, and a
// refusal at the place where a rolling one's fillet meets another's.
// The shell is changed only when every pick is rounded.
func filletApart(sh *srf.SShell, picks []FilletPick, r float64, origin uint32, flat bool) error {
	if len(picks) < 2 {
		return errNotApart
	}
	t, err := Build(sh)
	if err != nil {
		return errNotApart
	}
	var exact, rolling []FilletPick
	var chains [][]int
	edgeVerts := map[int]int{}
	for _, p := range picks {
		e, err := t.pickEdge(p)
		if err != nil {
			return errNotApart
		}
		pr := r
		if p.R != 0 {
			pr = p.R
		}
		if !t.needsRollingBall(e, pr) {
			exact = append(exact, p)
			for _, v := range t.Edges[e].V {
				edgeVerts[v]++
			}
			continue
		}
		rolling = append(rolling, p)
		pieces, verts, closed := t.blendChain(e)
		if !closed {
			pieces, _ = t.blendChainBack(pieces, verts)
		}
		var vs []int
		for _, pc := range pieces {
			vs = append(vs, t.Edges[pc.Edge].V[0], t.Edges[pc.Edge].V[1])
		}
		chains = append(chains, vs)
	}
	if len(rolling) == 0 || len(rolling) > maxApart {
		return errNotApart
	}
	// A rolling chain's vertex at another pick's: their fillets meet
	// there, a corner neither construction rounds with the other.
	seen := map[int]int{}
	for i, vs := range chains {
		for _, v := range vs {
			j, ok := seen[v]
			if edgeVerts[v] > 0 || ok && j != i {
				return filletErr(t.Vertices[v].P, "an edge's fillet that only the rolling ball makes, "+
					"which is made on its own, meets another picked edge's fillet here: pick it alone, "+
					"and round the other after")
			}
			seen[v] = i
		}
	}

	var cp srf.SShell
	cp.MakeFromCopyOf(sh)
	if len(exact) > 0 {
		if err := FilletPicks(&cp, exact, r, origin, flat); err != nil {
			return err
		}
	}
	tol := math.Max(10*geom.LengthEps, sh.Tol.ChordTolMm())
	for k, p := range rolling {
		u, err := Build(&cp)
		if err != nil {
			return filletErr(p.A, "the solid's topology is broken: %v", err)
		}
		q := p
		if !p.Arc {
			// The fillets before may have moved the edge's ends along it,
			// where they cut into its end faces; never across.
			a, b, found := u.edgeAlong(p.A, p.B, tol)
			if !found {
				return filletErr(p.A.Plus(p.B).ScaledBy(0.5), "an edge picked was cut by another's fillet")
			}
			q.A, q.B = a, b
		}
		if err := filletOnCurvedEnd(&cp, q, r, origin+apartOrigins+uint32(k)*stageOrigins, flat); err != nil {
			return err
		}
	}
	*sh = cp
	return nil
}

// pickEdge is the edge of the solid that p picks.
func (t *Topology) pickEdge(p FilletPick) (int, error) {
	if p.Arc {
		e := t.curvedEdgeAt(p.At)
		if e < 0 {
			return -1, filletErr(p.At, "no edge of the solid runs there")
		}
		return e, nil
	}
	return t.wholeEdge(p.A, p.B)
}

// needsRollingBall reports whether only the rolling ball rounds edge e
// with radius r: it lies between faces step 10 doesn't take, or it is
// straight between flat faces and a face at one of its ends is curved.
func (t *Topology) needsRollingBall(e int, r float64) bool {
	if _, err := t.filletEdgeOf(e, r, false); err != nil {
		return t.generalBlendEdge(e)
	}
	if !t.Straight(e) {
		return false
	}
	for _, v := range t.Edges[e].V {
		for _, f := range t.facesAt(v) {
			if _, ok := t.planarNormal(f); !ok {
				return true
			}
		}
	}
	return false
}
