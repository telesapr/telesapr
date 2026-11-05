package topo

import (
	"math"

	"github.com/telesapr/telesapr/pkg/srf"
)

// A crease that fades out part way along its edge and turns back past
// there, as a loft's from a corner of its square foot does above the
// first ring (its faces turn by 138° at the foot, by nothing at the
// ring, by a few degrees the other way above): the fillet narrows to
// nothing where the crease fades out and ends there (rbEnd.fade), and
// the edge goes on past it with the crease turned the other way. That
// piece is rounded too, the other way, by a fillet of its own that
// widens from nothing where the first ended (rollingBall.fadeIn) and
// narrows to nothing again where the crease fades out once more.

// turnBackOrigin is added to a fillet's origin for the faces of the
// fillets made past where its creases turned back: within the 16 bits
// srf.OriginFor leaves the group's faces, clear of the first fillets'.
const turnBackOrigin = 1 << 15

// filletTurnedBack rounds, on sh, where FilletPicks has just made picks
// on before, the pieces of the picked edges a fillet ended short of
// because their creases faded out there and turned back. Where none is
// left, or the pieces can't be rounded (the crease only fades out, the
// faces past it tangent), sh is left as it is.
func filletTurnedBack(sh, before *srf.SShell, picks []FilletPick, r float64, origin uint32, flat bool) {
	rest := turnedBackPicks(sh, before, picks)
	if len(rest) == 0 {
		return
	}
	cp := sh.Clone()
	if err := filletPicksAny(&cp, rest, r, origin+turnBackOrigin, flat); err != nil || unsound(&cp) {
		return
	}
	*sh = cp
}

// turnedBackPicks are the edges of sh that run on from an end of a
// picked edge of before along it, short of its other end: what a fillet
// that faded out left of the edge.
func turnedBackPicks(sh, before *srf.SShell, picks []FilletPick) []FilletPick {
	// Built from copies: Build fills a shell's caches, which moves the
	// last digits of what is made from it after.
	bc, sc := before.Clone(), sh.Clone()
	tb, err := Build(&bc)
	if err != nil {
		return nil
	}
	t, err := Build(&sc)
	if err != nil {
		return nil
	}
	tol := math.Max(1e-6, sh.Tol.ChordTolMm()/10)
	// The piece's points are the curve's chorded again where the fillet
	// split the edge: within the chord tolerance of the picked edge's
	// chords, not of their points (a tenth of it refused the loft's piece
	// at 0.05% and finer, 0.0045 mm off).
	onTol := math.Max(1e-6, sh.Tol.ChordTolMm())
	var rest []FilletPick
	for _, p := range picks {
		var e int
		if p.Arc {
			e = tb.curvedEdgeAt(p.At)
		} else {
			e, _ = tb.wholeEdge(p.A, p.B)
		}
		if e < 0 {
			continue
		}
		orig := tb.Edges[e].Points
		ends := [2]int{tb.Edges[e].V[0], tb.Edges[e].V[1]}
		for x := range t.Edges {
			ed := &t.Edges[x]
			if len(ed.Points) < 2 || len(ed.CoEdges) != 2 {
				continue
			}
			a, b := t.Vertices[ed.V[0]].P, t.Vertices[ed.V[1]].P
			atEnd := func(q int) bool {
				pq := t.Vertices[q].P
				for _, v := range ends {
					if pq.Minus(tb.Vertices[v].P).Magnitude() < tol {
						return true
					}
				}
				return false
			}
			// One end at an end of the picked edge, the other short of
			// both: a piece of it, not the whole.
			if atEnd(ed.V[0]) == atEnd(ed.V[1]) {
				continue
			}
			on := true
			for _, q := range ed.Points {
				if distanceToPolyline(q, orig) > onTol {
					on = false
					break
				}
			}
			if !on {
				continue
			}
			// Where the piece is short of the picked edge's end the faces
			// meet tangent: the crease faded out there.
			short := a
			if atEnd(ed.V[0]) {
				short = b
			}
			if !t.smoothAt(x, short) {
				continue
			}
			q := p
			q.A, q.B, q.At, q.Arc = a, b, midpoint(ed.Points), true
			rest = append(rest, q)
		}
	}
	return rest
}
