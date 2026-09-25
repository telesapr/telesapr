package srf

import "github.com/spacesolve/spacesolve/pkg/geom"

// SBezierLoopSetSet is the top-level container for an entire sketch:
// a list of SBezierLoopSet (one per outer contour) plus an optional
// list of open paths.
type SBezierLoopSetSet struct {
	Sets    []SBezierLoopSet
	Opens   SBezierList
}

func (s *SBezierLoopSetSet) Clear() {
	s.Sets = s.Sets[:0]
	s.Opens.Clear()
}

// AddOuter appends a new empty SBezierLoopSet and returns a pointer
// to it.
func (s *SBezierLoopSetSet) AddOuter() *SBezierLoopSet {
	s.Sets = append(s.Sets, SBezierLoopSet{})
	return &s.Sets[len(s.Sets)-1]
}

// AddOpenPath appends an open path Bezier.
func (s *SBezierLoopSetSet) AddOpenPath(b SBezier) {
	s.Opens.Add(b)
}

// FindOuterFacesFrom computes the outer faces of the supplied Bezier
// list, given that they project onto the plane spanned by u and v.
// The function populates s.Sets and returns the open paths through
// the opensOut parameter.
//
// Phase 3 + Goal Item 12 implementation: chains curves whose
// endpoints match within tol. Closed chains become SBezierLoopSets
// (outer loops); chains that don't close are emitted as open
// paths. The full C++ version uses point-line classification; this
// version uses greedy endpoint matching that handles the common
// case.
func (s *SBezierLoopSetSet) FindOuterFacesFrom(in *SBezierList, sp *SPolygon, opensOut *SBezierList, u, v geom.Vector) {
	const tol = 1e-9
	_ = u
	_ = v
	used := make([]bool, len(in.Curves))
	for i := 0; i < len(in.Curves); i++ {
		if used[i] {
			continue
		}
		used[i] = true
		start := in.Curves[i].Start()
		chain := []SBezier{in.Curves[i]}
		current := in.Curves[i].Finish()
		for {
			found := false
			for j := 0; j < len(in.Curves); j++ {
				if used[j] {
					continue
				}
				if in.Curves[j].Start().DistanceTo(current) < tol {
					chain = append(chain, in.Curves[j])
					current = in.Curves[j].Finish()
					used[j] = true
					found = true
					break
				}
			}
			if !found {
				break
			}
		}
		if current.DistanceTo(start) < tol {
			set := s.AddOuter()
			set.Outer.Curves = chain
		} else if opensOut != nil {
			opensOut.Add(in.Curves[i])
		}
	}
	if sp != nil {
		for i := range s.Sets {
			s.Sets[i].MakePwlInto(sp)
		}
	}
}
