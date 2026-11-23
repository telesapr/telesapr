package topo

import (
	"cmp"
	"errors"
	"math"
	"slices"
	"strings"

	"github.com/telesapr/telesapr/pkg/srf"
)

// A foot that only just reaches its face's edge with the next face along
// the chain, the face pinched to a point there (dipsOn), comes out
// neither cut nor whole: the trough pocket's rim at 2, its walls' height
// where the top is lowest, and the dish's y+/x-/y- chain chamfered at
// 3.25, its foot on the upright y- wall touching the floor. As
// footClearSize does for a foot touching a curved side's, such a fillet
// is made at the size nearest r at which the foot either crosses the
// edge by enough to be cut there or stays clear of it, where that moves
// its surface by no more than the chord tolerance; the caller says the
// size made (FilletPicksMade).

// footAtEdgeMsg refuses a fillet or chamfer (%s) whose foot only just
// reaches the edge of its face; footAtEdgeRefusal finds it.
const footAtEdgeMsg = "the %s's " + footAtEdgeWords + ", where the next face begins, too near to cut it there: " +
	"make it a little smaller or larger"

// footAtEdgeWords are footAtEdgeMsg's words after the what.
const footAtEdgeWords = "foot only just reaches the edge of its face here"

// footAtEdgeRefusal reports whether err is footAtEdgeMsg's refusal.
func footAtEdgeRefusal(err error) bool {
	var fx *FilletError
	if !errors.As(err, &fx) {
		return false
	}
	return strings.Contains(fx.Msg, footAtEdgeWords)
}

// footAtEdgeSize makes the picks' fillet of before, refused at r for a
// foot that only just reaches its face's edge, at the size nearest r
// that makes a closed solid, moved by no more than moves its surface by
// the chord tolerance: (sqrt 2 - 1)|r - c| for a fillet, |r - c|/sqrt 2
// for a chamfer, between faces square to each other. Not for a pick of
// a size of its own, whose size would not move with r.
func footAtEdgeSize(before *srf.SShell, picks []FilletPick, r float64, origin uint32, flat bool) (srf.SShell, float64, bool) {
	for _, p := range picks {
		if p.R != 0 || p.D2 != 0 {
			return srf.SShell{}, r, false
		}
	}
	move := math.Sqrt2 - 1
	if flat {
		move = 1 / math.Sqrt2
	}
	most := before.Tol.ChordTolMm() / move
	var sizes []float64
	for _, k := range []float64{0.25, 0.5, 0.75, 1} {
		sizes = append(sizes, r-k*most, r+k*most)
	}
	slices.SortStableFunc(sizes, func(a, b float64) int { return cmp.Compare(math.Abs(a-r), math.Abs(b-r)) })
	for _, c := range sizes {
		if c <= 0 {
			continue
		}
		sh := before.Clone()
		made, err := filletPicksChecked(&sh, picks, c, origin, flat)
		if err == nil {
			return sh, made, true
		}
	}
	return srf.SShell{}, r, false
}
