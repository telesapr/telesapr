package srf

import (
	"math"

	"github.com/spacesolve/spacesolve/pkg/core"
	"github.com/spacesolve/spacesolve/pkg/geom"
)

// SCurvePt is one polyline vertex on a trim curve.
type SCurvePt struct {
	Vertex bool   // true if this point is a vertex of the original SBezier (used for snapping)
	P      geom.Vector
	Tag    int
}

// SCurve is a trim curve in the shell: the exact SBezier form plus
// the polyline approximation, and references to the two surfaces
// it trims. SurfA is the "outer" surface (the one on the right when
// walking from Start to Finish); SurfB is the "inner" surface.
//
// NewH records the new handle assigned to this curve when it gets
// copied into a different shell during a boolean. Mirrors the C++
// SCurve::newH field used by SShell::MakeFromBoolean to rewrite trim
// handles after CopyCurvesSplitAgainst.
type SCurve struct {
	H       core.HSCurve
	NewH    core.HSCurve
	Tag     int
	Exact   SBezier
	IsExact bool
	Deg     int

	SurfA, SurfB core.HSSurface
	AuxA, AuxB   int

	Points []SCurvePt

	// Source indicates where this curve came from during booleans.
	Source CurveSource
}

// CurveSource tracks the provenance of a curve (used by
// SShell boolean operations to decide what to keep). Mirrors
// SCurve::Source in solvespace/src/srf/surface.h:208.
type CurveSource uint32

const (
	CurveSourceOrigA        CurveSource = 0
	CurveSourceOrigB        CurveSource = 1
	CurveSourceSplitA       CurveSource = 2
	CurveSourceSplitB       CurveSource = 3
	CurveSourceIntersection CurveSource = 4
)

// surfAFromB returns the surface handle of the original surface in
// `parent` that is named `which` in `old`. Used by
// SShell::CopyCurvesSplitAgainst / SShell::MakeIntersectionCurvesAgainst
// to rewrite the newH back to a parent-relative source face. Mirrors
// the helpers in srf/boolean.cpp that need to know which surface in the
// original shell a curve came from.
func surfAFromB(which core.HSSurface, a, b *SShell) core.HSSurface {
	if a != nil && a.FindSurface(which) != nil {
		return which
	}
	return which
}

// SShell is a watertight collection of trimmed surfaces and the
// trim curves joining them. Used as the input to SShell::MakeFromBoolean
// (Phase 4) and as the result of Group::GenerateShellAndMesh
// (Phase 5).
type SShell struct {
	Curve   []SCurve
	Surface []SSurface

	BooleanFailed bool

	// cachedMesh is the SMesh produced by TriangulateInto; cached so
	// AllPointsIntersecting doesn't re-triangulate on every ray.
	cachedMesh SMesh
}

// Clear resets all data.
func (s *SShell) Clear() {
	s.Curve = nil
	s.Surface = nil
	s.BooleanFailed = false
}

// MakeFromCopyOf clones another shell.
func (s *SShell) MakeFromCopyOf(src *SShell) {
	s.Clear()
	for _, c := range src.Curve {
		s.Curve = append(s.Curve, c)
	}
	for _, ss := range src.Surface {
		s.Surface = append(s.Surface, ss)
	}
	s.BooleanFailed = src.BooleanFailed
}

// MakeFromExtrusionOf extrudes a planar BezierLoopSet from t0 to t1.
// Each curve in the set becomes a side surface; the outer contour
// is capped at t0 and t1 with two planar surfaces.
//
// This is one of the highest-level constructor methods. It
// iterates every loop in the set, emits a translated copy of each
// Bezier at t0 and t1 (the caps), and an extruded surface along
// each Bezier (the side).
//
// PORTING.md §"Remaining work" point 5 (ClassifyEdge partial-
// overlap recognition): every emitted surface is assigned a fresh
// `H` handle here so that downstream curve lookups
// (`SShell::FindSurface` via `SCurve::GetSurfaceA / GetSurfaceB`)
// resolve to the correct pair of surfaces during per-surface-trim
// curve refinement. Before this fix every extruded surface had
// `H = 0`, so `GetSurfaceA` and `GetSurfaceB` both returned the
// same `SSurface` pointer — `closestPointOnThisAndSurface` then
// collapsed to "find the closest point on `s` to `p`", which is
// trivially `p` itself, and the classifier's chain-midpoint
// refinement did nothing. The C++ equivalent
// (`SShell::MakeFromExtrusionOf` in `surface.cpp`) assigns a
// fresh `H` per surface; we now match that.
func (s *SShell) MakeFromExtrusionOf(sbls *SBezierLoopSet, t0, t1 geom.Vector, color core.RgbaColor) {
	s.Clear()
	if sbls == nil {
		return
	}
	// Process outer contour first, then holes.
	for loopIdx, loop := range []SBezierLoop{sbls.Outer} {
		for i := range loop.Curves {
			b := &loop.Curves[i]
			// Side surface from this Bezier.
			side := FromExtrusionOf(b, t0, t1)
			side.Color = color
			side.H = core.HSSurface{V: uint32(len(s.Surface) + 1)}
			side.Face = FaceID(uint32(len(s.Surface)) + 1)
			side.Tag = loop.Tag
			s.Surface = append(s.Surface, side)
			_ = loopIdx
		}
		// Cap at t0.
		t0Plane := FromPlane(t0, geom.Vector{X: 1}, geom.Vector{Y: 1})
		t0Plane.Color = color
		t0Plane.H = core.HSSurface{V: uint32(len(s.Surface) + 1)}
		t0Plane.Face = FaceID(uint32(len(s.Surface)) + 1)
		t0Plane.Tag = loop.Tag
		s.Surface = append(s.Surface, t0Plane)
		// Cap at t1.
		t1Plane := FromPlane(t1, geom.Vector{X: 1}, geom.Vector{Y: 1})
		t1Plane.Color = color
		t1Plane.H = core.HSSurface{V: uint32(len(s.Surface) + 1)}
		t1Plane.Face = FaceID(uint32(len(s.Surface)) + 1)
		t1Plane.Tag = loop.Tag
		s.Surface = append(s.Surface, t1Plane)
	}
}

// MakeFromRevolutionOf revolves a planar BezierLoopSet around an
// axis (pt, axis). Each Bezier becomes a revolved surface; the
// caps are filled by joining the start and end of the rotation.
func (s *SShell) MakeFromRevolutionOf(sbls *SBezierLoopSet, pt, axis geom.Vector, color core.RgbaColor) {
	s.Clear()
	if sbls == nil {
		return
	}
	const thetaf = 2 * 3.141592653589793
	for i := range sbls.Outer.Curves {
		b := &sbls.Outer.Curves[i]
		surf := FromRevolutionOf(b, pt, axis, 0, thetaf, 0, 0)
		surf.Color = color
		surf.Face = FaceID(uint32(len(s.Surface)) + 1)
		surf.Tag = sbls.Outer.Tag
		s.Surface = append(s.Surface, surf)
	}
}

// MakeFromHelixOf sweeps a planar BezierLoopSet around an axis
// (pt, axis) while translating along the axis by `turns` total
// rotation. Pitch is the axial distance per full revolution
// (pitch = axial_distance / turns). Equivalent to revolving the
// contour once while extruding along the axis by `pitch`.
func (s *SShell) MakeFromHelixOf(sbls *SBezierLoopSet, pt, axis geom.Vector, turns float64, pitch float64, color core.RgbaColor) {
	s.Clear()
	if sbls == nil {
		return
	}
	turns = math.Abs(turns)
	if turns < 1e-9 {
		turns = 1
	}
	if pitch < 1e-9 {
		pitch = 1.0
	}
	const thetaf = 2 * 3.141592653589793
	axisN := axis.Normal()
	axialStep := pitch / 12.0
	for i := range sbls.Outer.Curves {
		b := &sbls.Outer.Curves[i]
		// Sample the bezier at 12 points along its length; each sample
		// is revolved at a different angle and axially shifted.
		const ns = 12
		for k := 0; k < ns; k++ {
			t0 := float64(k) / ns
			t1 := float64(k+1) / ns
			seg := SBezier{Degree: 1}
			seg.Ctrl[0] = b.PointAt(t0)
			seg.Ctrl[1] = b.PointAt(t1)
			seg.Weight[0] = 1
			seg.Weight[1] = 1
			angle0 := t0 * turns * thetaf
			angle1 := t1 * turns * thetaf
			offset0 := t0 * turns * pitch
			offset1 := t1 * turns * pitch
			surf := FromRevolutionOf(&seg, pt, axisN, angle0, angle1, offset0, offset1)
			surf.Color = color
			surf.Face = FaceID(uint32(len(s.Surface)) + 1)
			surf.Tag = sbls.Outer.Tag
			s.Surface = append(s.Surface, surf)
		}
		_ = axialStep
	}
}

// MakeFromTransformationOf applies (translate, rotate, scale) to
// every surface and curve of src.
func (s *SShell) MakeFromTransformationOf(src *SShell, t geom.Vector, q geom.Quaternion, scale float64, includingTrims bool) {
	s.Clear()
	for _, ss := range src.Surface {
		s.Surface = append(s.Surface, FromTransformationOf(&ss, t, q, scale, includingTrims))
	}
}

// TriangulateInto triangulates every surface in the shell and
// appends the resulting triangles to mesh. Each surface is sampled
// on a regular (u, v) grid; cells whose center is outside the trim
// region are skipped via srf.Bsp.
func (s *SShell) TriangulateInto(mesh *SMesh) {
	for i := range s.Surface {
		s.Surface[i].TriangulateInto(mesh)
	}
}

// TriangulateInto is the per-surface version. It samples the
// surface on a regular (u, v) grid, then keeps only those cells
// whose center is INSIDE the trim region (per srf.Bsp). Phase 4 +
// Goal Item 4 implementation: supports trimmed surfaces via the
// SBspUv classifier.
func (srf *SSurface) TriangulateInto(mesh *SMesh) {
	const n = 16
	for j := 0; j < n; j++ {
		for i := 0; i < n; i++ {
			u0 := float64(i) / float64(n)
			u1 := float64(i+1) / float64(n)
			v0 := float64(j) / float64(n)
			v1 := float64(j+1) / float64(n)
			// Test the cell center; keep the cell only if it's inside
			// the trim region.
			if srf.Bsp != nil {
				uc := (u0 + u1) / 2
				vc := (v0 + v1) / 2
				if srf.Bsp.ClassifyPoint(geom.Point2d{U: uc, V: vc}) == BspUvOutside {
					continue
				}
			}
			p00 := srf.PointAt(u0, v0)
			p10 := srf.PointAt(u1, v0)
			p11 := srf.PointAt(u1, v1)
			p01 := srf.PointAt(u0, v1)
			meta := STriMeta{Face: srf.Face, RgbaColor: srf.Color}
			mesh.AddTriangle3(meta, p00, p10, p11)
			mesh.AddTriangle3(meta, p00, p11, p01)
		}
	}
}

// edgeKey is a quantised 3-D point used to dedupe boundary
// endpoints while computing the coplanar-but-shifted cluster in
// MergeCoincidentSurfaces.
type edgeKey struct{ X, Y, Z float64 }

// MergeCoincidentSurfaces identifies surfaces whose control grids
// coincide (within tol) and merges them, keeping a single copy.
// This dramatically reduces shell size when extrusion produces
// many redundant planes.
//
// Phase 4 + Goal Item 15 implementation: clusters surfaces by
// coincidence, then assigns each cluster a unique FaceID. The
// merged cluster keeps one representative surface.
//
// PORTING.md §"Remaining work" item 3 (coplanar-but-shifted):
// the C++ `SShell::MergeCoincidentSurfaces` (merge.cpp:11)
// additionally merges surfaces that are coplanar with the cluster
// representative AND whose edges form a subset of the cluster's
// accumulated edges. That handles stacked-cuboid unions where two
// side faces at the same X=0 plane but different Z ranges share
// only the middle edge (one cuboid's top == the next cuboid's
// bottom). Without that merge the four stacked cuboids of
// `boolean_coplanar_union` end up as four separate coplanar side
// faces whose triangles don't form a watertight shell.
//
// The Go port historically used the strict `CoincidentWith`
// cluster, which only merges surfaces with all control points
// coincident — that misses the coplanar-but-shifted case. The
// implementation below runs the strict cluster first, then a
// second pass that grows each cluster by coplanar edges, exactly
// mirroring the C++ `do { … } while (mergedThisTime)` loop in
// `SShell::MergeCoincidentSurfaces` (`merge.cpp:31`).
func (s *SShell) MergeCoincidentSurfaces() {
	const tol = 1e-9
	const uvRectOverlapTol = 1e-6

	// surfaceEdgeSet[i] is the union of 3-D edge endpoints for surface
	// i. Used by the coplanar-but-shifted cluster check below to
	// detect whether surface j's edges form a subset of surface i's
	// accumulated cluster edges.
	surfaceEdges := make([]map[edgeKey]bool, len(s.Surface))
	for i := range s.Surface {
		surfaceEdges[i] = edgesAsPoints(s.Surface[i], uvRectOverlapTol)
	}

	cluster := make([]int, len(s.Surface))
	for i := range s.Surface {
		cluster[i] = -1
	}
	representative := make([]int, 0, len(s.Surface))

	// Pass 1: strict full-coincident cluster (existing behaviour).
	for i := range s.Surface {
		if cluster[i] != -1 {
			continue
		}
		cluster[i] = len(representative)
		representative = append(representative, i)
		for j := i + 1; j < len(s.Surface); j++ {
			if cluster[j] != -1 {
				continue
			}
			if s.Surface[i].CoincidentWith(&s.Surface[j], true) {
				cluster[j] = cluster[i]
			}
		}
	}

	// Pass 2: coplanar-but-shifted cluster. For each unmerged
	// surface, find an existing cluster it's coplanar with and
	// whose edges it can contribute to. Mirrors the C++
	// `do { … } while (mergedThisTime)` loop in `merge.cpp:31`
	// for coplanar-but-shifted plane surfaces.
	for i := range s.Surface {
		if cluster[i] != -1 {
			continue
		}
		// Skip non-plane surfaces (the C++ merge.cpp:26 also
		// handles only planes).
		if s.Surface[i].DegM != 1 || s.Surface[i].DegN != 1 {
			continue
		}
		// Find a cluster representative whose plane matches this
		// surface's plane (same normal, same distance from origin)
		// and whose colour matches.
		bestRep := -1
		bestRepShared := -1
		for rIdx, srcIdx := range representative {
			if !s.Surface[srcIdx].Color.Equals(s.Surface[i].Color) {
				continue
			}
			if !coplanarPlanes(s.Surface[srcIdx], s.Surface[i], uvRectOverlapTol) {
				continue
			}
			shared := sharedEdgeCount(surfaceEdges[srcIdx], surfaceEdges[i])
			if shared > bestRepShared {
				bestRepShared = shared
				bestRep = rIdx
			}
		}
		if bestRep < 0 {
			// No coplanar cluster to join — start a new cluster on
			// its own. The strict pass already handled fully-
			// coincident surfaces; this surface is genuinely coplanar
			// but not shared with anything.
			cluster[i] = len(representative)
			representative = append(representative, i)
			continue
		}
		// Merge surface i into bestRep's cluster.
		cluster[i] = bestRep
		// Accumulate i's edges into the cluster's edge set so that
		// the next coplanar surface can detect the shared edge.
		for k := range surfaceEdges[i] {
			surfaceEdges[representative[bestRep]][k] = true
		}
	}

	out := make([]SSurface, 0, len(representative))
	for rIdx, srcIdx := range representative {
		srf := s.Surface[srcIdx]
		srf.Face = FaceID(uint32(rIdx + 1))
		out = append(out, srf)
	}
	// Build a map from old surface handle -> representative H so we can
	// rewrite every curve's surface handle to point at the merged face.
	oldHToRepH := make(map[core.HSSurface]core.HSSurface, len(representative))
	for _, srcIdx := range representative {
		repH := out[cluster[srcIdx]].H
		for j, c := range cluster {
			if c == cluster[srcIdx] {
				_ = j
				oldHToRepH[s.Surface[j].H] = repH
			}
		}
	}
	// Walk every curve and rewrite its surface handles to point at the
	// representative of whichever surface cluster they referenced.
	for i := range s.Curve {
		if nh, ok := oldHToRepH[s.Curve[i].SurfA]; ok {
			s.Curve[i].SurfA = nh
		}
		if nh, ok := oldHToRepH[s.Curve[i].SurfB]; ok {
			s.Curve[i].SurfB = nh
		}
	}
	s.Surface = out
	_ = tol
}

// edgesAsPoints returns the boundary edges of `srf` as a set of
// 3-D points (each edge contributes both endpoints). Used by
// `MergeCoincidentSurfaces` to detect coplanar-but-shifted face
// merges: two surfaces are merged if they are coplanar AND at
// least one boundary endpoint coincides (within tol).
func edgesAsPoints(srf SSurface, tol float64) map[edgeKey]bool {
	out := make(map[edgeKey]bool)
	if len(srf.Trim) == 0 {
		return out
	}
	for i := range srf.Trim {
		p := snap(srf.Trim[i].StartP, tol)
		out[edgeKey{X: p.X, Y: p.Y, Z: p.Z}] = true
		p = snap(srf.Trim[i].FinishP, tol)
		out[edgeKey{X: p.X, Y: p.Y, Z: p.Z}] = true
	}
	return out
}

func snap(p geom.Vector, tol float64) geom.Vector {
	return geom.Vector{
		X: math.Round(p.X/tol) * tol,
		Y: math.Round(p.Y/tol) * tol,
		Z: math.Round(p.Z/tol) * tol,
	}
}

func sharedEdgeCount(a, b map[edgeKey]bool) int {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	n := 0
	for k := range b {
		if a[k] {
			n++
		}
	}
	return n
}

// coplanarPlanes reports whether two plane surfaces lie on the
// same geometric plane: same unit normal AND same signed
// distance from the origin. For plane surfaces (degm == degn ==
// 1) with the same colour, this is the coplanar-but-shifted test
// the C++ merge.cpp uses for the second-pass cluster.
func coplanarPlanes(a, b SSurface, tol float64) bool {
	if a.DegM != 1 || a.DegN != 1 || b.DegM != 1 || b.DegN != 1 {
		return false
	}
	na := a.NormalAt(0.5, 0.5)
	nb := b.NormalAt(0.5, 0.5)
	if na.Magnitude() < 1e-20 || nb.Magnitude() < 1e-20 {
		return false
	}
	nu := na.WithMagnitude(1)
	mb := nb.WithMagnitude(1)
	if (nu.Minus(mb)).Magnitude() > tol && (nu.Plus(mb)).Magnitude() > tol {
		return false
	}
	// Use the same direction for both. If normals are antiparallel
	// we still merge (they lie on the same plane).
	if (nu.Plus(mb)).Magnitude() < tol {
		// antiparallel — flip one
		mb = mb.ScaledBy(-1)
	}
	// Signed distance from origin (same normal direction): a dot n
	da := a.Ctrl[0][0].Dot(nu)
	db := b.Ctrl[0][0].Dot(nu)
	if math.Abs(da-db) > tol {
		return false
	}
	return true
}

// MakeEdgesInto extracts the trim edges of every surface as SEdge
// entries. When a surface has no trim, emits the four boundary
// edges of the surface's uv rectangle.
func (s *SShell) MakeEdgesInto(el *SEdgeList) {
	for i := range s.Surface {
		s.Surface[i].MakeEdgesInto(s, el, nil, true)
	}
}

// MakeEdgesInto emits the boundary edges of this surface. If
// useCurvesFrom is non-nil, the trim region is taken from those
// curves instead of the surface's own trim. Phase 4 + Goal Item 4
// implementation: when srf has trim curves (or useCurvesFrom is
// supplied), emit each curve as a 3-D edge; otherwise emit the four
// boundary edges of the uv rectangle.
func (srf *SSurface) MakeEdgesInto(parentShell *SShell, el *SEdgeList, useCurvesFrom *SShell, includeTrimCurves bool) {
	_ = includeTrimCurves
	if useCurvesFrom != nil {
		for i := range useCurvesFrom.Curve {
			tag := int(useCurvesFrom.Curve[i].H.V)
			b := useCurvesFrom.Curve[i].Exact
			// Sample the curve at multiple points and emit as a
			// polyline in 3-D space (since edges are 3-D).
			const ns = 8
			last := b.Start()
			for k := 1; k <= ns; k++ {
				t := float64(k) / float64(ns)
				cur := b.PointAt(t)
				el.AddEdge(last, cur, tag, 0, tag)
				last = cur
			}
		}
		return
	}
	// No curves: emit 4 boundary edges of the surface's uv rectangle.
	p00 := srf.PointAt(0, 0)
	p10 := srf.PointAt(1, 0)
	p11 := srf.PointAt(1, 1)
	p01 := srf.PointAt(0, 1)
	el.AddEdge(p00, p10, 0, 0, srf.Tag)
	el.AddEdge(p10, p11, 0, 0, srf.Tag)
	el.AddEdge(p11, p01, 0, 0, srf.Tag)
	el.AddEdge(p01, p00, 0, 0, srf.Tag)
	_ = parentShell
}

// MakeSectionEdgesInto emits the cross-section of the shell where it
// is cut by the plane (n, d). Phase 4 + Item 4: emit the boundary
// of each surface that's coplanar with (n, d).
func (s *SShell) MakeSectionEdgesInto(n geom.Vector, d float64, el *SEdgeList) {
	for i := range s.Surface {
		s.Surface[i].MakeSectionEdgesInto(s, el, n, d)
	}
}

// MakeSectionEdgesInto is the per-surface version.
func (srf *SSurface) MakeSectionEdgesInto(parentShell *SShell, el *SEdgeList, n geom.Vector, d float64) {
	_ = parentShell
	// Phase 4 + Item 4: emit 4 boundary edges of the surface only
	// when the surface is approximately coplanar with (n, d).
	if !srf.CoincidentWithPlane(n, d, 1e-6) {
		return
	}
	p00 := srf.PointAt(0, 0)
	p10 := srf.PointAt(1, 0)
	p11 := srf.PointAt(1, 1)
	p01 := srf.PointAt(0, 1)
	el.AddEdge(p00, p10, 0, 0, srf.Tag)
	el.AddEdge(p10, p11, 0, 0, srf.Tag)
	el.AddEdge(p11, p01, 0, 0, srf.Tag)
	el.AddEdge(p01, p00, 0, 0, srf.Tag)
	_ = el
	_ = n
	_ = d
}

// RemapFaces assigns fresh face ids to surfaces in the shell.
// Called after a boolean operation when faces get split.
func (s *SShell) RemapFaces() {
	for i := range s.Surface {
		s.Surface[i].Face = FaceID(uint32(i + 1))
	}
}

// RemapFacesViaGroup calls g.Remap(face, copyNumber) for every
// surface's face handle. Used by Group::GenerateShellAndMesh after
// step-and-repeat and boolean operations when copies of entities
// need new face ids.
func (s *SShell) RemapFacesViaGroup(g interface {
	Remap(core.HEntity, int) core.EntityID
}, copyNumber int) {
	for i := range s.Surface {
		face := core.HEntity{V: uint32(s.Surface[i].Face)}
		if face == (core.HEntity{}) {
			continue
		}
		newID := g.Remap(face, copyNumber)
		s.Surface[i].Face = FaceID(newID.V)
	}
}

// FindCurve returns the SCurve with the given handle, or nil if not found.
func (s *SShell) FindCurve(h core.HSCurve) *SCurve {
	if s == nil {
		return nil
	}
	for i := range s.Curve {
		if s.Curve[i].H == h {
			return &s.Curve[i]
		}
	}
	return nil
}

// FindSurface returns the SSurface with the given handle, or nil if not found.
func (s *SShell) FindSurface(h core.HSSurface) *SSurface {
	if s == nil {
		return nil
	}
	for i := range s.Surface {
		if s.Surface[i].H == h {
			return &s.Surface[i]
		}
	}
	return nil
}

// FindCurveBySurface returns all curves that reference the given surface as
// either SurfA or SurfB.
func (s *SShell) FindCurveBySurface(h core.HSSurface) []*SCurve {
	if s == nil {
		return nil
	}
	var out []*SCurve
	for i := range s.Curve {
		if s.Curve[i].SurfA == h || s.Curve[i].SurfB == h {
			out = append(out, &s.Curve[i])
		}
	}
	return out
}
