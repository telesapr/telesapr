package srf

import (
	"math/rand/v2"
	"sync"
	"testing"

	"github.com/telesapr/telesapr/pkg/geom"
)

// rayTreeShells are curved shells whose surfaces a ray cast
// subdivides: a torus, a sphere's patches and a lathed tube.
func rayTreeShells() []*SShell {
	return []*SShell{
		torusY(geom.Vector{}, 30, 8, Tolerance{}),
		patchUpSphere(20, geom.Vector{X: 5, Y: -3, Z: 2}, Tolerance{}),
		lathedTube(10, 25, -15, 15, Tolerance{}),
	}
}

// TestRayTreeSameAsRecursion holds the walk over a surface's tree to
// the recursion it replaces: the same hits in the same order, and the
// same count of pieces visited, for random lines and segments, from
// goroutines at once as a Boolean's loops cast them.
func TestRayTreeSameAsRecursion(t *testing.T) {
	t.Parallel()
	shells := rayTreeShells()
	for _, sh := range shells {
		for i := range sh.Surface {
			s := &sh.Surface[i]
			s.rays = &rayCell{}
			s.guess = &guessCell{}
			s.noCache = true
		}
	}
	rng := rand.New(rand.NewPCG(7, 11))
	type ray struct {
		a, b      geom.Vector
		asSegment bool
	}
	var rays []ray
	for range 400 {
		at := func() geom.Vector {
			return geom.Vector{X: rng.Float64()*120 - 60, Y: rng.Float64()*120 - 60, Z: rng.Float64()*80 - 40}
		}
		rays = append(rays, ray{a: at(), b: at(), asSegment: rng.IntN(2) == 0})
	}
	hits, visits := 0, 0
	for _, sh := range shells {
		for i := range sh.Surface {
			s := &sh.Surface[i]
			if s.DegM == 1 && s.DegN == 1 {
				continue
			}
			for _, r := range rays {
				cnt, level := 0, 0
				var want []surfInter
				s.AllPointsIntersectingUntrimmed(r.a, r.b, &cnt, &level, &want, r.asSegment, s)
				tree := s.rayTreeFor()
				if tree == nil {
					t.Fatalf("surface %d: no tree", i)
				}
				gotCnt := 0
				var got []surfInter
				tree.walk(&tree.root, r.a, r.b, &gotCnt, &got, r.asSegment, s)
				if gotCnt != cnt || len(got) != len(want) {
					t.Fatalf("surface %d, ray %v: tree visited %d pieces and found %d hits, the recursion %d and %d",
						i, r, gotCnt, len(got), cnt, len(want))
				}
				for k := range want {
					if got[k] != want[k] {
						t.Fatalf("surface %d, ray %v: hit %d is %v by the tree, %v by the recursion", i, r, k, got[k], want[k])
					}
				}
				hits += len(want)
				visits += cnt
			}
		}
	}
	if hits < 100 || visits < 10000 {
		t.Fatalf("%d hits over %d pieces visited: the rays miss the shells", hits, visits)
	}

	// The same from goroutines at once, each walking a tree made from
	// scratch as the others walk it.
	for _, sh := range shells {
		for i := range sh.Surface {
			s := &sh.Surface[i]
			if s.DegM == 1 && s.DegN == 1 {
				continue
			}
			s.rays = &rayCell{}
			var wg sync.WaitGroup
			errs := make(chan string, len(rays))
			for _, r := range rays {
				wg.Go(func() {
					cnt, level := 0, 0
					var want []surfInter
					s.AllPointsIntersectingUntrimmed(r.a, r.b, &cnt, &level, &want, r.asSegment, s)
					gotCnt := 0
					var got []surfInter
					tree := s.rayTreeFor()
					tree.walk(&tree.root, r.a, r.b, &gotCnt, &got, r.asSegment, s)
					if gotCnt != cnt || len(got) != len(want) {
						errs <- "a walk at once visited or found a different number"
						return
					}
					for k := range want {
						if got[k] != want[k] {
							errs <- "a walk at once found a different hit"
							return
						}
					}
				})
			}
			wg.Wait()
			close(errs)
			for e := range errs {
				t.Fatalf("surface %d: %s", i, e)
			}
		}
	}
}

// TestRayTreeStale has a surface changed since its tree was made fall
// back to the recursion, and a surface with no cell have no tree.
func TestRayTreeStale(t *testing.T) {
	t.Parallel()
	sh := torusY(geom.Vector{}, 30, 8, Tolerance{})
	s := &sh.Surface[0]
	if s.rayTreeFor() != nil {
		t.Fatal("a surface with no cell has a tree")
	}
	s.rays = &rayCell{}
	if s.rayTreeFor() == nil {
		t.Fatal("a surface with a cell has no tree")
	}
	s.Ctrl[1][1].X++
	if s.rayTreeFor() != nil {
		t.Fatal("a surface changed since still has its tree")
	}
}
