package srf

import (
	"github.com/spacesolve/spacesolve/pkg/geom"
	"sort"
)

// CopyCurvesSplitAgainst copies every curve of s into `into`, split so
// that a pwl segment never crosses a surface of agnst, and records
// each original curve's handle in `into` as its NewH (for rewriting
// the trims). Mirrors C++ SShell::CopyCurvesSplitAgainst.
func (s *SShell) CopyCurvesSplitAgainst(opA bool, agnst, into *SShell) {
	for i := range s.Curve {
		sc := &s.Curve[i]
		scn := sc.MakeCopySplitAgainst(agnst, nil,
			s.FindSurface(sc.SurfA), s.FindSurface(sc.SurfB))
		if opA {
			scn.Source = CurveSourceOrigA
		} else {
			scn.Source = CurveSourceOrigB
		}
		hsc := into.addAndAssignCurveId(scn)
		// And note the new ID so that we can rewrite the trims appropriately
		sc.NewH = hsc
	}
}

// MakeIntersectionCurvesAgainst computes the surface-surface
// intersection curves between every pair of surfaces (one from
// `s`, one from `agnst`) and appends them to into.Curve.
//
// Dispatches to the C++-equivalent SSurface::IntersectAgainst for
// each pair, which selects between plane-plane (exact line),
// plane-extrusion (exact projection or sampled lines),
// extrusion-extrusion-same-axis (exact lines), and the
// marching-squares fallback for general surfaces. Each emitted
// curve is tagged Source = CurveSourceIntersection so the
// per-surface trim pass can include it in the `inter` edge list.
func (s *SShell) MakeIntersectionCurvesAgainst(agnst, into *SShell) {
	if agnst == nil || into == nil {
		return
	}
	for i := range s.Surface {
		sa := &s.Surface[i]
		for j := range agnst.Surface {
			sb := &agnst.Surface[j]
			sa.IntersectAgainst(sb, s, agnst, into)
		}
	}
}

// CopySurfacesTrimAgainst copies every surface of s into `into`,
// trimmed against the other operand per the boolean op, and records
// each surface's handle in `into` as its NewH. Mirrors C++
// SShell::CopySurfacesTrimAgainst (boolean.cpp:750).
func (s *SShell) CopySurfacesTrimAgainst(sha, shb, into *SShell, op SurfaceCombineAs) {
	ssn := make([]SSurface, len(s.Surface))
	for i := range s.Surface {
		ssn[i] = s.Surface[i].makeCopyTrimAgainst(s, sha, shb, into, op, i)
	}
	for i := range s.Surface {
		s.Surface[i].NewH = into.addSurfaceAssignId(ssn[i])
	}
}

// RewriteSurfaceHandlesForCurves points every curve's SurfA/SurfB at
// the new handles of its surfaces in s. Mirrors C++
// SShell::RewriteSurfaceHandlesForCurves (boolean.cpp:791).
func (s *SShell) RewriteSurfaceHandlesForCurves(a, b *SShell) {
	for i := range s.Curve {
		sc := &s.Curve[i]
		sA := sc.GetSurfaceA(a, b)
		sB := sc.GetSurfaceB(a, b)
		if sA != nil {
			sc.SurfA = sA.NewH
		}
		if sB != nil {
			sc.SurfB = sB.NewH
		}
	}
}

// GetSurfaceA returns the surface named by SurfA, looked up in the
// operand the curve came from. Mirrors C++ SCurve::GetSurfaceA.
func (c *SCurve) GetSurfaceA(a, b *SShell) *SSurface {
	switch c.Source {
	case CurveSourceOrigB:
		return b.FindSurface(c.SurfA)
	default: // A, INTERSECTION
		return a.FindSurface(c.SurfA)
	}
}

// GetSurfaceB returns the surface named by SurfB, looked up in the
// operand the curve came from. Mirrors C++ SCurve::GetSurfaceB.
func (c *SCurve) GetSurfaceB(a, b *SShell) *SSurface {
	switch c.Source {
	case CurveSourceOrigA:
		return a.FindSurface(c.SurfB)
	default: // B, INTERSECTION
		return b.FindSurface(c.SurfB)
	}
}

// RemoveShortSegments drops the polyline points that aren't vertices,
// aren't spaced deliberately along an exact curve, and aren't needed
// to meet the chord tolerance in either surface. Faithful port of C++
// SCurve::RemoveShortSegments (curve.cpp:806). When both surfaces are
// nil (a Go-only case) it does nothing.
func (c *SCurve) RemoveShortSegments(sA, sB *SSurface) {
	c.RemoveShortSegmentsWithChordTol(sA, sB, ChordTolMm())
}

// RemoveShortSegmentsWithChordTol is RemoveShortSegments with an
// explicit chord tolerance in place of ChordTolMm().
func (c *SCurve) RemoveShortSegmentsWithChordTol(sA, sB *SSurface, chordTolMm float64) {
	if len(c.Points) <= 2 {
		return
	}
	if sA == nil && sB == nil {
		return
	}
	keep := make([]SCurvePt, 0, len(c.Points))
	keep = append(keep, c.Points[0])
	prev := c.Points[0].P
	for i := 1; i < len(c.Points)-1; i++ {
		sct := &c.Points[i]
		scn := &c.Points[i+1]

		if sct.Vertex {
			keep = append(keep, *sct)
			prev = sct.P
			continue
		}

		// if the curve is exact and points are >0.05 apart wrt t, point is there
		// deliberately regardless of chord tolerance (ex: small circles)
		var tprev, t, tnext float64
		if c.IsExact {
			tprev, _ = c.Exact.ClosestPointTo(prev, true)
			t, _ = c.Exact.ClosestPointTo(sct.P, true)
			tnext, _ = c.Exact.ClosestPointTo(scn.P, true)
		}
		if t-tprev > 0.05 && tnext-t > 0.05 {
			keep = append(keep, *sct)
			prev = sct.P
			continue
		}

		// We must check against both surfaces; the piecewise linear edge
		// may have a different chord tolerance in the two surfaces. (For
		// example, a circle in the surface of a cylinder is just a straight
		// line, so it always has perfect chord tol, but that circle in
		// a plane is a circle so it doesn't).
		mustKeep := false
		for _, s := range []*SSurface{sA, sB} {
			if s == nil {
				continue
			}
			var puv, nuv geom.Point2d
			s.ClosestPointTo(prev, &puv, true)
			s.ClosestPointTo(scn.P, &nuv, true)
			if s.ChordToleranceForEdge(geom.Vector{X: nuv.U, Y: nuv.V}, geom.Vector{X: puv.U, Y: puv.V}) > chordTolMm {
				mustKeep = true
			}
		}

		if mustKeep {
			keep = append(keep, *sct)
			prev = sct.P
		}
		// else prev is unchanged, since there's no longer any point
		// in between
	}
	keep = append(keep, c.Points[len(c.Points)-1])
	c.Points = keep
}

// MakeCopySplitAgainst returns a copy of c whose pwl is split wherever
// it crosses a (trimmed, or nearly trimmed) surface of agnstA or
// agnstB, and at any trim vertex of those shells lying on the curve.
// srfA and srfB are the surfaces c trims, whose crossings don't count.
// Mirrors C++ SCurve::MakeCopySplitAgainst (boolean.cpp).
func (c *SCurve) MakeCopySplitAgainst(agnstA, agnstB *SShell, srfA, srfB *SSurface) SCurve {
	ret := *c
	ret.Points = nil

	// First find any vertex that lies on our curve.
	var vertpts []geom.Vector
	if c.IsExact {
		if agnstA != nil {
			findVertsOnCurve(&vertpts, c, agnstA)
		}
		if agnstB != nil {
			findVertsOnCurve(&vertpts, c, agnstB)
		}
	}

	if len(c.Points) == 0 {
		return ret
	}
	prev := c.Points[0]
	ret.Points = append(ret.Points, prev)

	for k := 1; k < len(c.Points); k++ {
		p := c.Points[k]
		var il []SInter

		// Find all the intersections with the two passed shells
		if agnstA != nil {
			agnstA.AllPointsIntersecting(prev.P, p.P, &il, true, false, true)
		}
		if agnstB != nil {
			agnstB.AllPointsIntersecting(prev.P, p.P, &il, true, false, true)
		}

		if len(il) > 0 {
			// The intersections were generated by intersecting the pwl
			// edge against a surface; so they must be refined to lie
			// exactly on the original curve.
			kept := il[:0]
			for j := range il {
				pi := il[j]
				if pi.Srf == srfA || pi.Srf == srfB {
					// The edge certainly intersects the surfaces that it
					// trims (at its endpoints), but those ones don't count.
					// They are culled later, but no sense calculating them
					// and they will cause numerical problems (since two
					// of the three surfaces they're refined to lie on will
					// be identical, so the matrix will be singular).
					continue
				}

				var puv geom.Point2d
				pi.Srf.ClosestPointTo(pi.P, &puv, false)

				// Split the edge if the intersection lies within the surface's
				// trim curves, or within the chord tol of the trim curve; want
				// some slop if points are close to edge and pwl is too coarse,
				// and it doesn't hurt to split unnecessarily.
				cl := BspUvOutside
				if pi.Srf.Bsp != nil {
					cl = pi.Srf.Bsp.ClassifyPoint(puv, geom.Point2d{}, pi.Srf)
				}
				if cl == BspUvOutside {
					d := geom.VeryPositive
					if pi.Srf.Bsp != nil {
						d = pi.Srf.Bsp.MinimumDistanceToEdge(puv, pi.Srf)
					}
					if d > ChordTolMm() {
						continue
					}
				}

				// We're keeping the intersection, so actually refine it. Finding the intersection
				// to within EPS is important to match the ends of different chopped trim curves.
				// The general 3-surface intersection fails to refine for trims where surfaces
				// are tangent at the curve, but those trims are usually exact, so…
				if c.IsExact {
					pi.Srf.PointOnCurve(&c.Exact, &puv.U, &puv.V)
				} else {
					pi.Srf.PointOnSurfaces(srfA, srfB, &puv.U, &puv.V)
				}
				pi.P = pi.Srf.PointAt(puv.U, puv.V)
				kept = append(kept, pi)
			}
			il = kept
		}
		// Now add any vertex that is on this segment
		lineStart := prev.P
		lineDirection := p.P.Minus(prev.P)
		for _, vtx := range vertpts {
			t := vtx.Minus(lineStart).DivProjected(lineDirection)
			if 0.0 < t && t < 1.0 {
				il = append(il, SInter{P: vtx})
			}
		}
		if len(il) > 0 {
			// And now sort them in order along the line. Note that we must
			// do that after refining, in case the refining would make two
			// points switch places.
			sort.SliceStable(il, func(i, j int) bool {
				ta := il[i].P.Minus(lineStart).DivProjected(lineDirection)
				tb := il[j].P.Minus(lineStart).DivProjected(lineDirection)
				return ta < tb
			})

			// And now uses the intersections to generate our split pwl edge(s)
			last := geom.Vector{X: geom.VeryPositive}
			for _, pi := range il {
				// On-edge intersection will generate same split point for
				// both surfaces, so don't create zero-length edge.
				if !last.Equals(pi.P) {
					ret.Points = append(ret.Points, SCurvePt{P: pi.P, Vertex: true})
				}
				last = pi.P
			}
		}

		ret.Points = append(ret.Points, p)
		prev = p
	}
	return ret
}

// findVertsOnCurve appends to `out` every trim endpoint in `sh`
// that lies on the exact Bezier of `curve` (within LENGTH_EPS) and
// has parameter t strictly between 0 and 1 (so the curve gets split
// at that interior point). Mirrors the static FindVertsOnCurve in
// solvespace/src/srf/boolean.cpp.
//
// Issue #1452: an exact intersection curve may extend past the real
// geometry (e.g. out to the padded bounds of a surface enlarged by
// SShell::MergeCoincidentSurfaces). Splitting some other curve at
// such a phantom point — which is not a vertex of the trims adjacent
// across that other curve — produces T-junctions and naked edges in
// the triangulated shell.
func findVertsOnCurve(out *[]geom.Vector, curve *SCurve, sh *SShell) {
	if sh == nil || curve == nil {
		return
	}
	var amax, amin geom.Vector
	curve.GetAxisAlignedBounding(&amax, &amin)
	for i := range sh.Surface {
		ss := &sh.Surface[i]
		for j := range ss.Trim {
			stb := &ss.Trim[j]
			for k := 0; k < 2; k++ {
				pt := stb.StartP
				if k == 1 {
					pt = stb.FinishP
				}
				if pt.OutsideAndNotOn(amax, amin) {
					continue
				}
				t, _ := curve.Exact.ClosestPointTo(pt, false)
				d := pt.Minus(curve.Exact.PointAt(t)).Magnitude()
				if t > geom.LengthEps && t < 1-geom.LengthEps && d < geom.LengthEps {
					*out = append(*out, pt)
				}
			}
		}
	}
}
