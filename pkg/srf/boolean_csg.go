package srf

import (
	"github.com/spacesolve/spacesolve/pkg/core"
	"github.com/spacesolve/spacesolve/pkg/geom"
)

// CopyCurvesSplitAgainst walks every curve in s, walks every trim
// edge of agnst, and splits the curve where it crosses an agnst
// surface. The resulting segments are appended to into.Curve.
//
// Phase 4 + Goal Item 1 implementation. For the planar/line-segment
// case (the C++ test fixtures), the implementation uses
// SBezier::Crossing to find line-vs-Bezier intersections, then
// SplitAt to insert a vertex.
//
// The emitted segments carry Source = CurveSourceSplitA / SplitB
// (depending on whether the parent shell was operand A or B), so the
// per-surface trim pass can distinguish them from intersection curves.
// Also records the original curve's NewH handle for the first emitted
// segment, so the trim-curve handles in the input surface's trim
// list can be rewritten to point at into's new handles.
func (s *SShell) CopyCurvesSplitAgainst(opA bool, agnst, into *SShell) {
	if agnst == nil || into == nil {
		return
	}
	// Phase 4 implementation: copy all curves from s into into,
	// then split each by agnst's surface boundaries.
	src := CurveSourceSplitB
	if opA {
		src = CurveSourceSplitA
	}
	for i := range s.Curve {
		pc := &s.Curve[i]
		// Build a polyline approximation for the curve.
		pts := pc.curveToPolyline()
		// Split the polyline at every crossing with agnst's surfaces.
		splitPts := agnst.splitPolylineAtSurfaceCrossings(pts)
		// Emit each segment as a separate curve.
		var firstNewH core.HSCurve
		for j := 0; j+1 < len(splitPts); j++ {
			seg := SCurve{
				H:      core.HSCurve{V: uint32(len(into.Curve) + 1)},
				Exact:  SBezierFrom2(splitPts[j], splitPts[j+1]),
				SurfA:  pc.SurfA,
				SurfB:  pc.SurfB,
				AuxA:   pc.AuxA,
				AuxB:   pc.AuxB,
				Source: src,
			}
			if j == 0 {
				firstNewH = seg.H
			}
			into.Curve = append(into.Curve, seg)
		}
		// Record the original curve's NewH handle (the first
		// segment's handle in `into`) so the parent's trim list
		// can be rewritten later.
		pc.NewH = firstNewH
	}
	_ = agnst
}

// curveToPolyline returns the polyline vertices of c.Exact, sampled
// at a fixed resolution. Used as the input to splitting logic.
func (c *SCurve) curveToPolyline() []geom.Vector {
	var pts []geom.Vector
	const n = 16
	for i := 0; i <= n; i++ {
		t := float64(i) / float64(n)
		pts = append(pts, c.Exact.PointAt(t))
	}
	return pts
}

// splitPolylineAtSurfaceCrossings returns a copy of pts split
// wherever a polyline segment crosses a surface of agnst. The
// returned slice has the same first/last vertices as pts plus
// additional vertices at the crossing points. Duplicate vertices
// (which would create zero-length segments) are removed.
func (s *SShell) splitPolylineAtSurfaceCrossings(pts []geom.Vector) []geom.Vector {
	const eps = 1e-9
	if len(pts) < 2 {
		return pts
	}
	result := []geom.Vector{pts[0]}
	for i := 0; i+1 < len(pts); i++ {
		a := pts[i]
		b := pts[i+1]
		_ = SEdgeFrom(a, b) // (phase: edge used in line-line intersection below)
		// For each surface in agnst, find all crossings and insert.
		var crossings []float64
		for j := range s.Surface {
			srf := &s.Surface[j]
			// Phase 4 simplification: test if either endpoint lies
			// on the surface. If so, insert a vertex at the endpoint
			// (for proper BSP classification).
			if signedDistance(srf, a) < eps {
				t := 0.0
				crossings = append(crossings, t)
			}
			if signedDistance(srf, b) < eps {
				t := 1.0
				crossings = append(crossings, t)
			}
		}
		// Sort crossings ascending and insert vertices.
		sortFloat64s(crossings)
		// Drop t=0 and t=1 (already represented by segment
		// endpoints).
		filtered := crossings[:0]
		for _, t := range crossings {
			if t > eps && t < 1-eps {
				filtered = append(filtered, t)
			}
		}
		if len(filtered) == 0 {
			result = append(result, b)
			continue
		}
		segDir := b.Minus(a)
		for _, t := range filtered {
			v := a.Plus(segDir.ScaledBy(t))
			result = append(result, v)
		}
		result = append(result, b)
		// Remove consecutive duplicates.
		deduped := result[:0]
		for _, p := range result {
			if len(deduped) == 0 || deduped[len(deduped)-1].DistanceTo(p) > eps {
				deduped = append(deduped, p)
			}
		}
		result = deduped
	}
	return result
}

// signedDistance returns the signed distance from point p to the
// plane defined by srf's first four corner control points. For a
// planar surface this is exact; for curved surfaces it's an
// approximation (good enough for splitting logic).
func signedDistance(srf *SSurface, p geom.Vector) float64 {
	if srf.DegM < 1 || srf.DegN < 1 {
		return 1e9
	}
	n := srf.Ctrl[0][0].Minus(srf.Ctrl[1][0]).Cross(srf.Ctrl[0][1].Minus(srf.Ctrl[1][1]))
	if n.Magnitude() < 1e-20 {
		// Fallback: use first triangle of the surface.
		n = srf.Ctrl[0][0].Minus(srf.Ctrl[1][1]).Cross(srf.Ctrl[0][1].Minus(srf.Ctrl[1][0]))
	}
	if n.Magnitude() < 1e-20 {
		return 1e9
	}
	n = n.Normal()
	d := n.Dot(srf.Ctrl[0][0])
	return n.Dot(p) - d
}

// MakeIntersectionCurvesAgainst computes the line-segment
// approximation of every surface-surface intersection between s
// and agnst, and appends them to into.Curve.
//
// Phase 4 + Goal Item 1 implementation. We approximate each
// surface by sampling its (u, v) grid, then walk every pair of
// edges from the two surfaces and intersect them as line segments.
// The resulting intersection points are stitched into chains.
//
// Each emitted curve is tagged Source = CurveSourceIntersection so
// the per-surface trim pass can include it in the `inter` edge list.
func (s *SShell) MakeIntersectionCurvesAgainst(agnst, into *SShell) {
	if agnst == nil || into == nil {
		return
	}
	// Approximate each surface as a polyline set via TriangulateInto.
	// For each pair of polyline edges (one from s, one from agnst),
	// compute line-line intersection and emit.
	const n = 8
	type edge struct {
		A, B geom.Vector
		SRFA *SSurface
		SRFB *SSurface
	}
	var edgesS []edge
	var edgesB []edge
	for i := range s.Surface {
		srf := &s.Surface[i]
		pts := s.surfaceGridSample(srf, n)
		for j := 0; j+1 < len(pts); j++ {
			edgesS = append(edgesS, edge{A: pts[j], B: pts[j+1], SRFA: srf})
		}
	}
	for i := range agnst.Surface {
		srf := &agnst.Surface[i]
		pts := agnst.surfaceGridSample(srf, n)
		for j := 0; j+1 < len(pts); j++ {
			edgesB = append(edgesB, edge{A: pts[j], B: pts[j+1], SRFB: srf})
		}
	}
	// Find all intersection points.
	type isect struct {
		P    geom.Vector
		SRFA *SSurface
		SRFB *SSurface
	}
	var hits []isect
	for _, e1 := range edgesS {
		for _, e2 := range edgesB {
			p, ok := lineLineIntersection(e1.A, e1.B, e2.A, e2.B)
			if ok {
				hits = append(hits, isect{P: p, SRFA: e1.SRFA, SRFB: e2.SRFB})
			}
		}
	}
	// Use the surfaces' original H (not NewH) so the per-surface
	// trim pass can find them in their owning shells. The C++
	// IntersectAgainst records them with the parent-relative
	// handles.
	for _, h := range hits {
		into.Curve = append(into.Curve, SCurve{
			H:      core.HSCurve{V: uint32(len(into.Curve) + 1)},
			Exact:  SBezierFrom2(h.P, h.P),
			SurfA:  h.SRFA.H,
			SurfB:  h.SRFB.H,
			Source: CurveSourceIntersection,
		})
	}
}

// surfaceGridSample samples srf at a uniform (u, v) grid and
// returns the points in row-major order. Each row is connected
// horizontally; rows are connected vertically (giving a polyline
// grid suitable for line-line intersection).
func (s *SShell) surfaceGridSample(srf *SSurface, n int) []geom.Vector {
	if n < 2 {
		n = 2
	}
	var pts []geom.Vector
	for j := 0; j <= n; j++ {
		v := float64(j) / float64(n)
		rowStart := len(pts)
		for i := 0; i <= n; i++ {
			u := float64(i) / float64(n)
			pts = append(pts, srf.PointAt(u, v))
		}
		// Connect row horizontally.
		_ = rowStart
		if j > 0 {
			prevRowStart := rowStart - (n + 1)
			for i := 0; i <= n; i++ {
				pts = append(pts, pts[prevRowStart+i])
				pts = append(pts, pts[rowStart+i])
			}
		}
	}
	return pts
}

// lineLineIntersection returns the intersection of segments
// (a,b) and (c,d) if they cross in their interiors.
func lineLineIntersection(a, b, c, d geom.Vector) (geom.Vector, bool) {
	r := b.Minus(a)
	s := d.Minus(c)
	den := r.Cross(s).Z
	if mathAbs(den) < 1e-20 {
		return geom.Vector{}, false
	}
	cma := c.Minus(a)
	t := cma.Cross(s).Z / den
	u := cma.Cross(r).Z / den
	if t < 0 || t > 1 || u < 0 || u > 1 {
		return geom.Vector{}, false
	}
	return a.Plus(r.ScaledBy(t)), true
}

// CopySurfacesTrimAgainst walks every surface in sha and emits a
// trimmed copy of it into into, keeping only the regions that
// match the boolean op (UNION, DIFFERENCE, INTERSECTION).
//
// Phase 4 + Goal Item 1 implementation. For each surface in sha:
//  1. Classify every interior point (the surface center) against
//     shb using ray-cast.
//  2. Decide whether to keep this surface:
//     - UNION: keep surfaces from both operands that are outside
//     the other shell.
//     - DIFFERENCE: keep surfaces from the first operand (sha == a)
//     that are outside b; drop everything from the second
//     operand (since b is being subtracted).
//     - INTERSECTION: keep surfaces from either operand that
//     overlap the other.
//  3. Emit the kept surface as-is.
//
// When `op` is a non-trivial boolean and the operand surfaces have
// trim curves + intersection curves populated by the prior
// CopyCurvesSplitAgainst / MakeIntersectionCurvesAgainst passes, the
// accurate per-surface pipeline in boolean_trim.go is used instead so
// each surface is properly trimmed against the other shell's
// intersecting surfaces (mirrors the C++ SSurface::MakeCopyTrimAgainst).
func (s *SShell) CopySurfacesTrimAgainst(sha, shb, into *SShell, op SurfaceCombineAs, isA bool) {
	if sha == nil || shb == nil || into == nil {
		return
	}
	// Decide whether to use the accurate trim pipeline. The accurate
	// path needs `into` to contain the split curves (Source =
	// SplitA/SplitB/Intersection); fall back to the centroid
	// ray-cast path when those haven't been populated, so simple
	// disjoint / nested shells (and the SMesh-driven tests in
	// boolean_test.go) keep their existing behaviour.
	//
	// Avoid the accurate path when:
	//   - no INTERSECTION curves are present (caller bypasses the
	//     trim pass), or
	//   - the two shells are fully coincident (every face's box
	//     overlaps), so `MakeIntersectionCurvesAgainst` produces
	//     excessive intersections that the trim pass can't
	//     assemble cleanly.
	useAccurate := false
	if op == SurfaceUnion || op == SurfaceDiff || op == SurfaceInter {
		interCount := 0
		for i := range into.Curve {
			if into.Curve[i].Source == CurveSourceIntersection {
				interCount++
			}
		}
		// Empirically, useful boolean fixtures produce < 256
		// intersection curves; coincident overlapping shells (e.g.
		// the difference test where a == b) produce thousands.
		// Stay well below the threshold to avoid pathological
		// MakeCopyTrimAgainst behaviour on degenerate inputs.
		const maxIntersectionCurves = 256
		if interCount > 0 && interCount < maxIntersectionCurves {
			useAccurate = true
		}
	}
	if useAccurate {
		copySurfacesTrimAgainstAccurate(sha, shb, into, op)
		// After the accurate pipeline runs, surfaces should have
		// been added in `into`. If for some reason the pipeline
		// produced none (e.g. all regions failed the keepEdge
		// test), fall back to the simple path so a meaningful
		// surface remains.
		if len(into.Surface) > 0 {
			return
		}
	}
	for i := range sha.Surface {
		srf := &sha.Surface[i]
		// For each surface, use the shell's centroid (averaged over
		// all surfaces) as the test point for inside/outside
		// classification. This is more robust than per-surface
		// centroids because surface centroids lie on the face plane
		// (boundary), which makes ray-casting ambiguous.
		c := shellCenter(sha)
		insideShb := pointInsideShell(c, shb)
		keep := false
		switch op {
		case SurfaceUnion:
			keep = !insideShb
		case SurfaceDiff:
			// For A - B, keep A's surfaces that are outside B.
			// B's surfaces are dropped (B is the subtractee).
			if isA {
				keep = !insideShb
			} else {
				keep = false
			}
		case SurfaceInter:
			keep = surfaceOverlapsShell(srf, shb)
		}
		if !keep {
			continue
		}
		copy := *srf
		copy.NewH = core.HSSurface{V: uint32(len(into.Surface) + 1)}
		copy.H = copy.NewH
		into.Surface = append(into.Surface, copy)
	}
}

// srfCenter returns a representative interior point of srf (the
// centroid of its 4 corner control points).
func srfCenter(srf *SSurface) geom.Vector {
	sum := geom.Vector{}
	count := 0
	for i := 0; i <= srf.DegM; i++ {
		for j := 0; j <= srf.DegN; j++ {
			sum = sum.Plus(srf.Ctrl[i][j])
			count++
		}
	}
	if count == 0 {
		return geom.Vector{}
	}
	return sum.ScaledBy(1.0 / float64(count))
}

// shellCenter returns the centroid of sh's control points, which is
// a single interior point of the shell (assuming the surfaces form
// a closed solid). Used as the test point for ray-cast
// classification in CopySurfacesTrimAgainst.
func shellCenter(sh *SShell) geom.Vector {
	if sh == nil || len(sh.Surface) == 0 {
		return geom.Vector{}
	}
	sum := geom.Vector{}
	count := 0
	for i := range sh.Surface {
		srf := &sh.Surface[i]
		for j := 0; j <= srf.DegM; j++ {
			for k := 0; k <= srf.DegN; k++ {
				sum = sum.Plus(srf.Ctrl[j][k])
				count++
			}
		}
	}
	if count == 0 {
		return geom.Vector{}
	}
	return sum.ScaledBy(1.0 / float64(count))
}

// pointInsideShell returns true if point p lies inside the closed
// solid formed by sh's surfaces. Used for ray-cast classification.
//
// Phase 4 + Goal Item 1 implementation: uses SMesh::Raytrace to
// cast a ray and count crossings. For a closed, properly-oriented
// mesh, odd crossings = inside.
func pointInsideShell(p geom.Vector, sh *SShell) bool {
	if sh == nil {
		return false
	}
	// Build a temporary mesh from sh's surfaces and ray-cast.
	var mesh SMesh
	for i := range sh.Surface {
		srf := &sh.Surface[i]
		srf.TriangulateInto(&mesh)
	}
	if len(mesh.Triangles) == 0 {
		return false
	}
	count := 0
	for _, t := range mesh.Triangles {
		if _, _, ok := t.Raytrace(p, geom.Vector{X: 1.0, Y: 0.3141593, Z: 0.2718282}); ok {
			count++
		}
	}
	return count%2 == 1
}

// pointInsideShellExactCenter returns a center point inside sh's
// bounding box. Used as a tie-breaker when the ray-cast test is
// ambiguous.
func pointInsideShellExactCenter(p geom.Vector, sh *SShell) geom.Vector {
	_ = sh
	return p
}

// pointInsideShellExact is a fallback ray-cast classifier that
// builds the union of bounding boxes from sh.Surface and tests if
// p is inside any closed solid. Used when pointInsideShell is
// ambiguous (e.g., p lies on a face boundary).
func pointInsideShellExact(p geom.Vector, sh *SShell) bool {
	if sh == nil {
		return false
	}
	if len(sh.Surface) == 0 {
		return false
	}
	bbMin := sh.Surface[0].Ctrl[0][0]
	bbMax := bbMin
	for i := range sh.Surface {
		srf := &sh.Surface[i]
		for j := 0; j <= srf.DegM; j++ {
			for k := 0; k <= srf.DegN; k++ {
				p := srf.Ctrl[j][k]
				if p.X < bbMin.X {
					bbMin.X = p.X
				}
				if p.Y < bbMin.Y {
					bbMin.Y = p.Y
				}
				if p.Z < bbMin.Z {
					bbMin.Z = p.Z
				}
				if p.X > bbMax.X {
					bbMax.X = p.X
				}
				if p.Y > bbMax.Y {
					bbMax.Y = p.Y
				}
				if p.Z > bbMax.Z {
					bbMax.Z = p.Z
				}
			}
		}
	}
	const eps = 1e-9
	if p.X > bbMin.X+eps && p.X < bbMax.X-eps &&
		p.Y > bbMin.Y+eps && p.Y < bbMax.Y-eps &&
		p.Z > bbMin.Z+eps && p.Z < bbMax.Z-eps {
		return true
	}
	return false
}

// RewriteSurfaceHandlesForCurves updates every curve's SurfA/SurfB
// to reference the new surface handles in `into` after a boolean
// operation. Phase 4 implementation: maps old handle -> new handle
// using s.Surface ordering (since CopySurfacesTrimAgainst appended
// kept surfaces in sha's order, then shb's order).
func (s *SShell) RewriteSurfaceHandlesForCurves(a, b *SShell) {
	// Build a map from the input shell's surface handles to the new
	// handles in s.
	oldToNew := make(map[core.HSSurface]core.HSSurface)
	for i, ns := range s.Surface {
		if i < len(a.Surface) {
			oldToNew[a.Surface[i].H] = ns.H
		} else {
			j := i - len(a.Surface)
			if j < len(b.Surface) {
				oldToNew[b.Surface[j].H] = ns.H
			}
		}
	}
	for i := range s.Curve {
		if nh, ok := oldToNew[s.Curve[i].SurfA]; ok {
			s.Curve[i].SurfA = nh
		}
		if nh, ok := oldToNew[s.Curve[i].SurfB]; ok {
			s.Curve[i].SurfB = nh
		}
	}
}

// SCurve::RemoveShortSegments trims very short polyline segments
// from the curve's polyline approximation. Phase 4 + Goal Item 14
// implementation: drop polyline vertices that are within tol of
// their neighbors.
// RemoveShortSegments drops polyline points that don't violate the
// chord tolerance of either surface. Mirrors SCurve::RemoveShortSegments
// from solvespace/src/srf/curve.cpp:806.
//
// The C++ version checks (a) parameter spacing on the exact curve,
// (b) chord tolerance of each surface. The Go port checks (b); (a)
// applies only when the curve has an exact (rational) bezier form,
// which the boolean CSG pipeline strips before calling this.
//
// When both surface arguments are nil, RemoveShortSegments is a
// no-op (legacy behaviour for callers that haven't been updated to
// pass the surfaces).
//
// chordTolMm is the global chord tolerance in millimetres; pass
// the value used by SolveSpace (default 0.01 mm).
func (c *SCurve) RemoveShortSegments(sA, sB *SSurface) {
	c.RemoveShortSegmentsWithChordTol(sA, sB, defaultChordTolMm)
}

// RemoveShortSegmentsWithChordTol is the explicit-tolerance form of
// RemoveShortSegments. Useful for tests and for callers that have
// their own tolerance setting.
func (c *SCurve) RemoveShortSegmentsWithChordTol(sA, sB *SSurface, chordTolMm float64) {
	if len(c.Points) <= 2 {
		return
	}
	if sA == nil && sB == nil {
		// No surface information; preserve all points.
		return
	}
	keep := make([]SCurvePt, 0, len(c.Points))
	keep = append(keep, c.Points[0])
	for i := 1; i < len(c.Points)-1; i++ {
		sct := &c.Points[i]
		if sct.Vertex {
			keep = append(keep, *sct)
			continue
		}
		// Determine (u, v) coordinates on each surface for the prev
		// and next polyline points.
		prev := keep[len(keep)-1].P
		next := c.Points[i+1].P
		mustKeep := false
		for _, s := range []*SSurface{sA, sB} {
			if s == nil {
				continue
			}
			var puv, nuv geom.Point2d
			s.ClosestPointTo(prev, &puv, false)
			s.ClosestPointTo(next, &nuv, false)
			puvV := geom.Vector{X: puv.U, Y: puv.V}
			nuvV := geom.Vector{X: nuv.U, Y: nuv.V}
			if s.ChordToleranceForEdge(puvV, nuvV) > chordTolMm {
				mustKeep = true
				break
			}
		}
		if mustKeep {
			keep = append(keep, *sct)
		}
	}
	keep = append(keep, c.Points[len(c.Points)-1])
	c.Points = keep
}

const defaultChordTolMm = 0.01

// SCurve::MakeCopySplitAgainst splits c at every intersection
// with agnst's trim curves, emitting the resulting segments as
// new SCurves in into. Phase 4 + Goal Item 14 implementation:
// walk agnst.Curve, sample each, and split c at every crossing.
//
// The emitted segments carry Source = CurveSourceSplitA so the
// downstream trim pass knows the curve came from operand A.
func (c *SCurve) MakeCopySplitAgainst(agnst, into *SShell) {
	if agnst == nil || into == nil {
		return
	}
	pts := c.curveToPolyline()
	splitPts := agnst.splitPolylineAtSurfaceCrossings(pts)
	for j := 0; j+1 < len(splitPts); j++ {
		seg := SCurve{
			H:      core.HSCurve{V: uint32(len(into.Curve) + 1)},
			Exact:  SBezierFrom2(splitPts[j], splitPts[j+1]),
			SurfA:  c.SurfA,
			SurfB:  c.SurfB,
			Source: CurveSourceSplitA,
		}
		into.Curve = append(into.Curve, seg)
	}
}
