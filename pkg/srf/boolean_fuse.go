package srf

import (
	"math"

	"github.com/telesapr/telesapr/pkg/geom"
)

// fuseVertexTol is how close, as a fraction of the chord tolerance,
// two vertices of a Boolean's curves must be to be made one.
const fuseVertexTol = 0.01

// fuseVertices makes the vertices of s's curves that lie within a
// hundredth of the chord tolerance of each other a single point. Where a seam of one operand passes over a seam of
// the other close to where an intersection curve crosses both (a ring's
// patch corner on a pot's seam, roadmap K3), the curves are split where
// each seam meets the other surface and where the curve meets each seam:
// three or four vertices a few hundred-thousandths of a millimetre
// apart. In a patch's uv those are one point, so the trim drops the
// segment between them and the mesh of one side does without a vertex
// the other side has. Made one, both sides share it.
//
// Vertices closer than splitEndTol are made one too, though a split
// that close to a curve's end is none (MakeCopySplitAgainst): a DIN 513
// nut's ridges met its bore at 0.5% chord in a pair 1.2e-6 mm apart, past
// LengthEps, and the mesh took them as two points, four edges naked. The
// ends of a and b's own trims stay where they are: the trims find
// their part of a curve by them. A curve all of whose vertices would
// become one point is left as it is.
func (s *SShell) fuseVertices(a, b *SShell) {
	tol := s.Tol.ChordTolMm() * fuseVertexTol
	if tol <= geom.LengthEps {
		return
	}
	type cell struct{ x, y, z int64 }
	cellOf := func(p geom.Vector) cell {
		return cell{int64(math.Floor(p.X / tol)), int64(math.Floor(p.Y / tol)), int64(math.Floor(p.Z / tol))}
	}
	grid := map[cell][]geom.Vector{}
	add := func(p geom.Vector) {
		c := cellOf(p)
		grid[c] = append(grid[c], p)
	}
	// nearest is the point already in the grid nearest p within tol.
	nearest := func(p geom.Vector) (geom.Vector, bool) {
		c := cellOf(p)
		best, found := geom.Vector{}, false
		for dx := int64(-1); dx <= 1; dx++ {
			for dy := int64(-1); dy <= 1; dy++ {
				for dz := int64(-1); dz <= 1; dz++ {
					for _, q := range grid[cell{c.x + dx, c.y + dy, c.z + dz}] {
						if d := q.Minus(p).Magnitude(); d < tol && (!found || d < best.Minus(p).Magnitude()) {
							best, found = q, true
						}
					}
				}
			}
		}
		return best, found
	}

	// The trims' ends go in first, so that the others move to them.
	fixed := map[cell][]geom.Vector{}
	for _, sh := range []*SShell{a, b} {
		for i := range sh.Surface {
			for _, t := range sh.Surface[i].Trim {
				for _, p := range []geom.Vector{t.StartP, t.FinishP} {
					fixed[cellOf(p)] = append(fixed[cellOf(p)], p)
					add(p)
				}
			}
		}
	}
	isFixed := func(p geom.Vector) bool {
		c := cellOf(p)
		for dx := int64(-1); dx <= 1; dx++ {
			for dy := int64(-1); dy <= 1; dy++ {
				for dz := int64(-1); dz <= 1; dz++ {
					for _, q := range fixed[cell{c.x + dx, c.y + dy, c.z + dz}] {
						if q.Equals(p) {
							return true
						}
					}
				}
			}
		}
		return false
	}

	for i := range s.Curve {
		sc := &s.Curve[i]
		moved := make([]geom.Vector, len(sc.Points))
		changed := false
		for k, pt := range sc.Points {
			moved[k] = pt.P
			if !pt.Vertex || isFixed(pt.P) {
				continue
			}
			q, ok := nearest(pt.P)
			if !ok {
				add(pt.P)
				continue
			}
			if !q.Equals(pt.P) {
				moved[k] = q
				changed = true
			}
		}
		if !changed {
			continue
		}
		pts := make([]SCurvePt, 0, len(sc.Points))
		for k, pt := range sc.Points {
			pt.P = moved[k]
			if n := len(pts); n > 0 && pts[n-1].P.Equals(pt.P) {
				// A point of no length from the one before: keep the
				// vertex of the two.
				pts[n-1].Vertex = pts[n-1].Vertex || pt.Vertex
				if pt.Vertex {
					pts[n-1].P = pt.P
				}
				continue
			}
			pts = append(pts, pt)
		}
		if len(pts) < 2 {
			continue
		}
		sc.Points = pts
	}
}
