package srf

import (
	"sync"

	"github.com/telesapr/telesapr/pkg/geom"
)

// guessGrid is the surface's points at the centres of a res by res grid
// of its (u, v) square, where ClosestPointTo looks for a first guess to
// refine. While a Boolean's loops run at once the surfaces' closest-
// point caches are off, so every closest point on a curved surface
// searched the grid, evaluating the surface at each of its points (half
// of regenerating a thread); the points are now evaluated by the first
// search of a loop (setNoCache gives each curved surface a guessCell)
// and read by the rest. The surface it was made from is kept, so that a
// copy of the surface since changed searches as it always did.
type guessGrid struct {
	res    int
	degM   int
	degN   int
	ctrl   [MaxDegree + 1][MaxDegree + 1]geom.Vector
	weight [MaxDegree + 1][MaxDegree + 1]float64
	pts    []geom.Vector // pts[i*res+j] is the point at grid cell (i, j)
}

// guessResolution is the size of the grid ClosestPointTo searches.
func (s *SSurface) guessResolution() int {
	if max(s.DegM, s.DegN) == 2 {
		return 7
	}
	return 20
}

// guessU is the parameter at the centre of grid cell i of res.
func guessU(i, res int) float64 { return (float64(i) + 0.5) / float64(res) }

// newGuessGrid evaluates the surface at its grid.
func newGuessGrid(s *SSurface) *guessGrid {
	res := s.guessResolution()
	g := &guessGrid{res: res, degM: s.DegM, degN: s.DegN, ctrl: s.Ctrl, weight: s.Weight,
		pts: make([]geom.Vector, res*res)}
	for i := range res {
		for j := range res {
			g.pts[i*res+j] = s.PointAt(guessU(i, res), guessU(j, res))
		}
	}
	return g
}

// guessCell holds a surface's grid, made by the first search that
// needs it: a surface with few closest points to find, as the sides of
// extruded text, would spend more on the grid than on searching.
type guessCell struct {
	once sync.Once
	grid *guessGrid
}

// guessPoints is the points of the surface's grid, made now if it
// wasn't, or nil if it has no cell or a grid made from it as it was.
func (s *SSurface) guessPoints() []geom.Vector {
	c := s.guess
	if c == nil {
		return nil
	}
	c.once.Do(func() { c.grid = newGuessGrid(s) })
	g := c.grid
	if g.degM != s.DegM || g.degN != s.DegN || g.ctrl != s.Ctrl || g.weight != s.Weight {
		return nil
	}
	return g.pts
}

// makeGuessGrids gives the curved surfaces of the shells a cell for
// their grids and for their ray trees (raycast_tree.go), where they
// have none yet: a cell made in one loop of a Boolean serves the next,
// since what it holds is checked against the surface as it is. With on
// false it does nothing; CleanupAfterBoolean takes the cells away.
func makeGuessGrids(on bool, shells ...*SShell) {
	if !on {
		return
	}
	for _, sh := range shells {
		for i := range sh.Surface {
			s := &sh.Surface[i]
			if _, _, _, plane := s.planeBasis(); plane {
				s.guess, s.rays = nil, nil
				continue
			}
			if s.guess == nil {
				s.guess = &guessCell{}
			}
			if s.rays == nil {
				s.rays = &rayCell{}
			}
		}
	}
}

// KeepClosestGuesses gives the shell's curved surfaces the grid of
// points ClosestPointTo starts its search from, as a Boolean's loops
// have (makeGuessGrids), or takes it away: for a caller that projects
// many points on them, as a fillet's rolling ball does, which without
// it evaluated the surface at every point of the grid for each. The
// points found are the same.
func (s *SShell) KeepClosestGuesses(keep bool) {
	if keep {
		makeGuessGrids(true, s)
		return
	}
	for i := range s.Surface {
		s.Surface[i].guess, s.Surface[i].rays = nil, nil
	}
}
