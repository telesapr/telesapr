package srf

import (
	"math"

	"github.com/telesapr/telesapr/pkg/geom"
)

// flatPatchTol is how far, relative to the patch's size, a control
// point of a patch may lie off the plane of the others and the patch
// still be taken as flat.
const flatPatchTol = 1e-9

// FlattenPlanarPatches makes each patch of the shell whose control
// points lie in a plane, as a fitted sweep's are where a straight side
// of its profile is swept along a path in a plane parallel to it, a
// plane patch spanning the same points: Booleans and the coplanar
// merges know a plane lying along another (snapToPlanes,
// MergeCoincidentSurfaces) but not a curved patch that happens to be
// flat. The trims, kept in space, are left as they are; the plane's
// normal points as the patch's did.
func (s *SShell) FlattenPlanarPatches() {
	for i := range s.Surface {
		sf := &s.Surface[i]
		if sf.DegM <= 1 && sf.DegN <= 1 {
			continue
		}
		n := sf.NormalAt(0.5, 0.5)
		if n.Magnitude() == 0 {
			continue
		}
		n = n.WithMagnitude(1)
		o := sf.Ctrl[0][0]
		size, flat := 0.0, true
		for a := 0; a <= sf.DegM; a++ {
			for b := 0; b <= sf.DegN; b++ {
				size = math.Max(size, sf.Ctrl[a][b].Minus(o).Magnitude())
			}
		}
		for a := 0; a <= sf.DegM && flat; a++ {
			for b := 0; b <= sf.DegN; b++ {
				if math.Abs(sf.Ctrl[a][b].Minus(o).Dot(n)) > flatPatchTol*math.Max(size, 1) {
					flat = false
					break
				}
			}
		}
		if !flat || size == 0 {
			continue
		}
		u := sf.Ctrl[sf.DegM][0].Minus(o)
		u = u.Minus(n.ScaledBy(u.Dot(n)))
		if u.Magnitude() < flatPatchTol*size {
			u = n.NormalDir(0)
		}
		u = u.WithMagnitude(1)
		v := n.Cross(u)
		lo, hi := [2]float64{math.Inf(1), math.Inf(1)}, [2]float64{math.Inf(-1), math.Inf(-1)}
		for a := 0; a <= sf.DegM; a++ {
			for b := 0; b <= sf.DegN; b++ {
				d := sf.Ctrl[a][b].Minus(o)
				x, y := d.Dot(u), d.Dot(v)
				lo = [2]float64{math.Min(lo[0], x), math.Min(lo[1], y)}
				hi = [2]float64{math.Max(hi[0], x), math.Max(hi[1], y)}
			}
		}
		at := func(x, y float64) geom.Vector { return o.Plus(u.ScaledBy(x)).Plus(v.ScaledBy(y)) }
		sf.DegM, sf.DegN = 1, 1
		sf.Ctrl = [MaxDegree + 1][MaxDegree + 1]geom.Vector{}
		sf.Weight = [MaxDegree + 1][MaxDegree + 1]float64{}
		sf.Ctrl[0][0], sf.Ctrl[1][0] = at(lo[0], lo[1]), at(hi[0], lo[1])
		sf.Ctrl[0][1], sf.Ctrl[1][1] = at(lo[0], hi[1]), at(hi[0], hi[1])
		sf.Weight[0][0], sf.Weight[1][0], sf.Weight[0][1], sf.Weight[1][1] = 1, 1, 1, 1
	}
}
