package srf

import "github.com/telesapr/telesapr/pkg/geom"

// MeshMass is what a closed mesh encloses, for a density of one: its
// volume, its centroid, and its inertia tensor about the centroid.
type MeshMass struct {
	Volume       float64
	CenterOfMass geom.Vector
	// Inertia is the inertia tensor about the centre of mass, in mm⁵
	// for mm: on the diagonal the moments about the axes through it
	// along x, y and z, off it the products of inertia, negated.
	Inertia [3][3]float64
}

// Mass integrates the mesh as a sum of tetrahedra, one from each
// triangle to a point in the middle of its box, signed so that the
// parts outside the mesh cancel. The moments are taken about that
// point rather than the origin: for a solid far from the origin they
// would be the small difference of two large sums. A mesh that is not
// closed gives a volume that depends on the point.
func (m *SMesh) Mass() MeshMass {
	var mm MeshMass
	if len(m.Triangles) == 0 {
		return mm
	}
	hi, lo := m.Bounding()
	p0 := hi.Plus(lo).ScaledBy(0.5)
	var first geom.Vector
	var second [3][3]float64
	for i := range m.Triangles {
		tr := &m.Triangles[i]
		a, b, c := tr.A.Minus(p0), tr.B.Minus(p0), tr.C.Minus(p0)
		det := a.Dot(b.Cross(c)) // six times the tetrahedron's volume
		s := a.Plus(b).Plus(c)
		mm.Volume += det / 6
		first = first.Plus(s.ScaledBy(det / 24))
		// The integral of x xᵀ over the tetrahedron 0, a, b, c is
		// det/120 (a aᵀ + b bᵀ + c cᵀ + s sᵀ).
		for r := range 3 {
			for q := r; q < 3; q++ {
				second[r][q] += det / 120 * (a.Element(r)*a.Element(q) + b.Element(r)*b.Element(q) +
					c.Element(r)*c.Element(q) + s.Element(r)*s.Element(q))
			}
		}
	}
	if mm.Volume == 0 {
		return mm
	}
	c := first.ScaledBy(1 / mm.Volume)
	mm.CenterOfMass = c.Plus(p0)
	var cov [3][3]float64
	for r := range 3 {
		for q := r; q < 3; q++ {
			x := second[r][q] - mm.Volume*c.Element(r)*c.Element(q)
			cov[r][q], cov[q][r] = x, x
		}
	}
	tr := cov[0][0] + cov[1][1] + cov[2][2]
	for r := range 3 {
		for q := range 3 {
			mm.Inertia[r][q] = -cov[r][q]
			if r == q {
				mm.Inertia[r][q] += tr
			}
		}
	}
	return mm
}
