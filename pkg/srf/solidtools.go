package srf

import (
	"math"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
)

// WindingNumber is how many times the mesh wraps around p: the sum of
// the solid angles its triangles subtend at p, over 4π (the formula
// of Van Oosterom and Strackee). For a closed mesh with outward
// normals it is 1 inside and 0 outside, and it degrades gracefully
// for a mesh with small gaps, so a value above one half is inside.
func (m *SMesh) WindingNumber(p geom.Vector) float64 {
	sum := 0.0
	for i := range m.Triangles {
		t := &m.Triangles[i]
		a, b, c := t.A.Minus(p), t.B.Minus(p), t.C.Minus(p)
		la, lb, lc := a.Magnitude(), b.Magnitude(), c.Magnitude()
		num := a.Dot(b.Cross(c))
		den := la*lb*lc + a.Dot(b)*lc + b.Dot(c)*la + c.Dot(a)*lb
		sum += 2 * math.Atan2(num, den)
	}
	return sum / (4 * math.Pi)
}

// PolygonLoopSet is the closed polygon through pts, which lie in a
// plane with normal n, as a loop set for the shell constructors: an
// outer loop of lines wound clockwise about n, as FindOuterFacesFrom
// leaves outer loops.
func PolygonLoopSet(pts []geom.Vector, n geom.Vector) *SBezierLoopSet {
	area := geom.Vector{}
	for i := range pts {
		area = area.Plus(pts[i].Cross(pts[(i+1)%len(pts)]))
	}
	if area.Dot(n) > 0 {
		rev := make([]geom.Vector, len(pts))
		for i := range pts {
			rev[i] = pts[len(pts)-1-i]
		}
		pts = rev
	}
	sbls := &SBezierLoopSet{Normal: n.WithMagnitude(1), Point: pts[0]}
	for i := range pts {
		sbls.Outer.Curves = append(sbls.Outer.Curves, SBezierFrom2(pts[i], pts[(i+1)%len(pts)]))
	}
	return sbls
}

// MakePrism adds the prism of the plane polygon pts swept from t0 to
// t1, which must not lie in the polygon's plane.
func (s *SShell) MakePrism(pts []geom.Vector, t0, t1 geom.Vector, color core.RGBA) {
	n := pts[1].Minus(pts[0]).Cross(pts[2].Minus(pts[0]))
	s.MakeFromExtrusionOf(PolygonLoopSet(pts, n), t0, t1, color)
}
