package srf

import "math"

// OverCurves is plane patch s carried on, as its polynomial runs, until
// it holds the curves cs that lie on it, and true, where they run past
// its sides; otherwise s and false. A face's trims can run past its
// patch: a fillet's end section on the plane face it ends against, or
// the smaller fillet's wall grown past a crease, are trims in space on
// the patch the face had. The kernel meshes them on the plane carried
// on, but a STEP file bounds the face by the surface it writes, and a
// reader cuts away what lies past it. For a plane the patch carried on
// is the same plane, exactly; a curved patch is left as it is.
func (s *SSurface) OverCurves(cs []SBezier) (SSurface, bool) {
	orig, bu, bv, ok := s.planeBasis()
	if !ok {
		return *s, false
	}
	for i := range 2 {
		for j := range 2 {
			if s.Weight[i][j] != s.Weight[0][0] {
				return *s, false
			}
		}
	}
	// A curve lies within its control points' hull, and the plane's
	// parameters are an affine map of space: the hull's box in them
	// holds the curve.
	n := bu.Cross(bv)
	tx, ty := bv.Cross(n), n.Cross(bu)
	u0, u1, v0, v1 := 0.0, 1.0, 0.0, 1.0
	for k := range cs {
		c := &cs[k]
		for i := 0; i <= c.Degree; i++ {
			d := c.Ctrl[i].Minus(orig)
			u, v := d.Dot(tx)/bu.Dot(tx), d.Dot(ty)/bv.Dot(ty)
			u0, u1 = math.Min(u0, u), math.Max(u1, u)
			v0, v1 = math.Min(v0, v), math.Max(v1, v)
		}
	}
	if u0 == 0 && u1 == 1 && v0 == 0 && v1 == 1 {
		return *s, false
	}
	out := s.Sub(u0, u1, v0, v1)
	out.H = s.H
	return out, true
}
