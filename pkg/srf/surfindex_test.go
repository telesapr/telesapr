package srf

import (
	"math"
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/telesapr/telesapr/pkg/geom"
)

// TestSurfaceIndexFindsWhatTheLoopFinds puts the index to random
// segments and lines over a shell of scattered planes, some flat along
// an axis, and requires the surfaces the loop over every one of them
// keeps, in the same order.
func TestSurfaceIndexFindsWhatTheLoopFinds(t *testing.T) {
	t.Parallel()
	rng := rand.New(rand.NewPCG(1, 2))
	v := func(scale float64) geom.Vector {
		return geom.Vector{X: (rng.Float64() - 0.5) * scale, Y: (rng.Float64() - 0.5) * scale,
			Z: (rng.Float64() - 0.5) * scale}
	}
	var sh SShell
	for i := range 300 {
		u, w := v(4), v(4)
		switch i % 3 {
		case 1:
			u.Z, w.Z = 0, 0 // flat in z
		case 2:
			u.X, w.X = 0, 0 // flat in x
		}
		sh.Surface = append(sh.Surface, FromPlane(v(100), u, w))
	}
	sh.index = newSurfaceIndex(&sh)

	queries := 0
	for range 3000 {
		a := v(120)
		var b geom.Vector
		switch rng.IntN(4) {
		case 0:
			b = a.Plus(v(10))
		case 1:
			b = a.Plus(geom.Vector{X: rng.Float64() * 50}) // along an axis
		case 2:
			// From one surface's corner, so the line touches its box.
			b = sh.Surface[rng.IntN(len(sh.Surface))].Ctrl[0][0]
		default:
			b = a.Plus(v(200))
		}
		if a.Equals(b) {
			continue
		}
		for _, asSegment := range []bool{true, false} {
			var want []int
			for i := range sh.Surface {
				if !sh.Surface[i].LineEntirelyOutsideBbox(a, b, asSegment) {
					want = append(want, i)
				}
			}
			got := sh.surfacesNear(a, b, asSegment, nil)
			if !slices.Equal(got, want) {
				t.Fatalf("line %v %v, segment %v: index %v, loop %v", a, b, asSegment, got, want)
			}
			queries++
		}
	}
	if queries < 5000 {
		t.Fatalf("only %d queries", queries)
	}

	// The ends of trims near a box: those the loop over every trim
	// keeps, in its order.
	for i := range sh.Surface {
		for range rng.IntN(4) {
			a, b := v(100), v(100)
			if rng.IntN(50) == 0 {
				a.Y = math.NaN()
			}
			sh.Surface[i].Trim = append(sh.Surface[i].Trim, STrimBy{StartP: a, FinishP: b})
		}
	}
	sh.index = newSurfaceIndex(&sh)
	for range 2000 {
		lo := v(120)
		hi := lo.Plus(geom.Vector{X: rng.Float64() * 30, Y: rng.Float64() * 30, Z: rng.Float64() * 30})
		var want []geom.Vector
		for i := range sh.Surface {
			for _, tr := range sh.Surface[i].Trim {
				for _, p := range []geom.Vector{tr.StartP, tr.FinishP} {
					if !p.OutsideAndNotOn(hi, lo) {
						want = append(want, p)
					}
				}
			}
		}
		near, ok := sh.index.trimEndsNear(&sh, hi, lo, nil)
		if !ok {
			t.Fatal("the index doesn't fit the shell it was made for")
		}
		var got []geom.Vector
		for _, e := range near {
			if !e.p.OutsideAndNotOn(hi, lo) {
				got = append(got, e.p)
			}
		}
		if !slices.EqualFunc(got, want, func(a, b geom.Vector) bool { return a.EqualsExactly(b) || a.Y != a.Y && b.Y != b.Y }) {
			t.Fatalf("box %v %v: index %v, loop %v", lo, hi, got, want)
		}
	}

	// A shell that has changed since is searched whole.
	sh.Surface = sh.Surface[:10]
	if _, ok := sh.index.near(&sh, geom.Vector{}, geom.Vector{X: 1}, true, nil); ok {
		t.Error("an index of 300 surfaces answered for a shell of 10")
	}
	sh.CleanupAfterBoolean()
	if sh.index != nil {
		t.Error("CleanupAfterBoolean kept the index")
	}
}
