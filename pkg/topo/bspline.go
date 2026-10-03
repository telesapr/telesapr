package topo

import (
	"errors"
	"fmt"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// AddBSplineFace puts a B-spline surface into the shell as one face:
// its Bezier patches (srf.BSplineSurface.Patches), joined along the
// knot lines, which are seams where the surface is smooth across them,
// and bounded by its four boundary curves, in pieces. The face's normal
// is the surface's, the cross product of its u and v directions. The
// patches carry sheet key sheet, so they are one face of the solid
// where they meet smoothly, and identities id, id+1 and on, one each by
// span along u then along v (srf.SSurface.Origin), so that every edge
// round them has a name, even where the surface folds along a knot and
// the patches are faces of their own. A boundary that shrinks to a
// point, a pole, makes no edge. The patches' handles come back, by span
// along u, then along v.
func AddBSplineFace(sh *srf.SShell, bs *srf.BSplineSurface, id, sheet uint32, color core.RgbaColor) ([][]core.HSSurface, error) {
	return NewFaceAdder(sh).AddBSplineFace(bs, id, sheet, color)
}

// AddBSplineFace is AddBSplineFace by the adder.
func (fa *FaceAdder) AddBSplineFace(bs *srf.BSplineSurface, id, sheet uint32, color core.RgbaColor) ([][]core.HSSurface, error) {
	ps, err := bs.Patches()
	if err != nil {
		return nil, err
	}
	out := make([][]core.HSSurface, len(ps))
	for a := range ps {
		out[a] = make([]core.HSSurface, len(ps[a]))
		for b := range ps[a] {
			p := ps[a][b]
			p.Origin, p.Sheet, p.Color = id+uint32(a*len(ps[a])+b), sheet, color
			// Clockwise in (u, v), which keeps the patch's normal (see
			// AddFace): up u = 0, along v = 1, down u = 1, back v = 0.
			var loop []srf.SBezier
			for _, side := range []int{3, 2, 1, 0} {
				c := p.EdgeCurve(side)
				if side == 1 || side == 0 {
					c.Reverse()
				}
				if c.Start().Equals(c.Finish()) && pole(&c) {
					continue
				}
				loop = append(loop, c)
			}
			if len(loop) < 2 {
				return nil, fmt.Errorf("topo: patch %d,%d of the surface has no area", a, b)
			}
			h, err := fa.Add(p, [][]srf.SBezier{loop})
			if err != nil {
				return nil, err
			}
			out[a][b] = h
		}
	}
	return out, nil
}

// pole reports whether a curve is all one point.
func pole(c *srf.SBezier) bool {
	for i := 1; i <= c.Degree; i++ {
		if !c.Ctrl[i].Equals(c.Ctrl[0]) {
			return false
		}
	}
	return true
}

// AddPlanarFace puts a flat face into the shell, bounded by loops of
// curves joined end to end, the first its outer boundary, on a plane
// that covers them (a surface's control net must cover its face; see
// planeOver). The face takes identity id. The curves must lie in one
// plane.
func AddPlanarFace(sh *srf.SShell, loops [][]srf.SBezier, id uint32, color core.RgbaColor) (core.HSSurface, error) {
	return NewFaceAdder(sh).AddPlanarFace(loops, id, color)
}

// AddPlanarFace is AddPlanarFace by the adder.
func (fa *FaceAdder) AddPlanarFace(loops [][]srf.SBezier, id uint32, color core.RgbaColor) (core.HSSurface, error) {
	return fa.AddPlanarFaceWithin(loops, id, color, geom.Vector{}, geom.LengthEps)
}

// AddPlanarFaceWithin is AddPlanarFace for curves that lie in a plane
// only within tol, as a file's do within its tolerance: normal n, when
// not zero, is the plane's, else the outer loop's. With tol past
// LengthEps the face's mesh takes the curves' own points on its edges
// (srf.SSurface.SnapToTrims).
func (fa *FaceAdder) AddPlanarFaceWithin(loops [][]srf.SBezier, id uint32, color core.RgbaColor, n geom.Vector, tol float64) (core.HSSurface, error) {
	if len(loops) == 0 || len(loops[0]) == 0 {
		return core.HSSurface{}, errors.New("topo: a face needs a loop")
	}
	var pts, poly []geom.Vector
	for li, l := range loops {
		for _, c := range l {
			for i := 0; i <= c.Degree; i++ {
				pts = append(pts, c.Ctrl[i])
			}
			if li == 0 {
				for k := 0; k < 8; k++ {
					poly = append(poly, c.PointAt(float64(k)/8))
				}
			}
		}
	}
	nl := newellOf(poly)
	if nl.Magnitude() < geom.LengthEps {
		return core.HSSurface{}, errors.New("topo: the outer loop of a flat face encloses no area")
	}
	if n.Magnitude() == 0 {
		n = nl
	} else if n.Dot(nl) < 0 {
		// The plane's side is the loop's, as AddFace turns it by them.
		n = n.ScaledBy(-1)
	}
	n = n.WithMagnitude(1)
	// The plane through the middle of the curves' spread from it.
	lo, hi := 0.0, 0.0
	for _, p := range pts {
		d := p.Minus(pts[0]).Dot(n)
		lo, hi = min(lo, d), max(hi, d)
	}
	if hi-lo > 2*tol {
		return core.HSSurface{}, fmt.Errorf("topo: a flat face's curves leave its plane by %g", hi-lo)
	}
	s := planeOver(pts, n)
	// Curves off the plane are a file's: its mesh takes their points.
	s.SnapToTrims = tol > geom.LengthEps
	if mid := (lo + hi) / 2; mid != 0 {
		for i := 0; i <= s.DegM; i++ {
			for j := 0; j <= s.DegN; j++ {
				s.Ctrl[i][j] = s.Ctrl[i][j].Plus(n.ScaledBy(mid))
			}
		}
	}
	s.Origin, s.Color = id, color
	return fa.Add(s, loops)
}
