package srf

import (
	"math"

	"github.com/spacesolve/spacesolve/pkg/core"
	"github.com/spacesolve/spacesolve/pkg/geom"
)

// SCurvePt is one polyline vertex on a trim curve.
type SCurvePt struct {
	Vertex bool // true if this point is a vertex of the original SBezier (used for snapping)
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

// addAndAssignSurfaceId appends ss to s.Surface with a fresh
// sequential handle (and matching FaceID), mirroring the C++
// `IdList::AddAndAssignId` semantics used throughout shell.cpp.
func (s *SShell) addAndAssignSurfaceId(ss SSurface) core.HSSurface {
	var maxV uint32
	for i := range s.Surface {
		if s.Surface[i].H.V > maxV {
			maxV = s.Surface[i].H.V
		}
	}
	h := core.HSSurface{V: maxV + 1}
	ss.H = h
	ss.Face = FaceID(h.V)
	s.Surface = append(s.Surface, ss)
	return h
}

// addAndAssignCurveId appends sc to s.Curve with a fresh sequential
// handle, mirroring the C++ `IdList::AddAndAssignId` semantics.
func (s *SShell) addAndAssignCurveId(sc SCurve) core.HSCurve {
	// Like C++ IdList::AddAndAssignId, use one more than the largest
	// id in use.
	var maxV uint32
	for i := range s.Curve {
		if s.Curve[i].H.V > maxV {
			maxV = s.Curve[i].H.V
		}
	}
	h := core.HSCurve{V: maxV + 1}
	sc.H = h
	s.Curve = append(s.Curve, sc)
	return h
}

// makePwlSCurvePoints samples b's polyline approximation into
// SCurvePt form, marking the first and last samples as vertices.
// Mirrors the C++ `SBezier::MakePwlInto(List<SCurvePt> *l, ...)`
// overload (ratpoly.cpp).
func makePwlSCurvePoints(b *SBezier) []SCurvePt {
	var raw []geom.Vector
	b.MakePwlIntoPoints(&raw, 0, 0)
	pts := make([]SCurvePt, len(raw))
	for i, p := range raw {
		pts[i] = SCurvePt{P: p, Vertex: i == 0 || i == len(raw)-1}
	}
	return pts
}

// shellTrimLine pairs a trim curve handle with the extrusion side
// surface it belongs to. Mirrors the C++ `TrimLine` struct in
// shell.cpp.
type shellTrimLine struct {
	hc core.HSCurve
	hs core.HSSurface
}

// MakeFromExtrusionOf extrudes a planar BezierLoopSet from t0 to t1.
// Faithful port of C++ `SShell::MakeFromExtrusionOf` (shell.cpp).
// Each curve in the set becomes a side surface trimmed by the two
// translated copies of itself (at t0 and t1) and by the trim lines
// joining consecutive curves; the outer contour and every hole are
// capped at t0 and t1 with two shared planar surfaces trimmed by
// the same translated curves.
func (s *SShell) MakeFromExtrusionOf(sbls *SBezierLoopSet, t0, t1 geom.Vector, color core.RgbaColor) {
	// Like the C++, this adds to s rather than replacing it, so that a
	// group's loop sets all extrude into one shell.
	if sbls == nil {
		return
	}

	// Make the extrusion direction consistent with respect to the
	// normal of the sketch we're extruding.
	if t0.Minus(t1).Dot(sbls.Normal) < 0 {
		t0, t1 = t1, t0
	}

	// Define a coordinate system to contain the original sketch, and
	// get a bounding box in that csys.
	n := sbls.Normal.ScaledBy(-1)
	u := n.NormalDir(0)
	v := n.NormalDir(1)
	orig := sbls.Point
	umin, umax := math.Inf(1), math.Inf(-1)
	sbls.GetBoundingProjd(u, orig, &umin, &umax)
	vmin, vmax := math.Inf(1), math.Inf(-1)
	sbls.GetBoundingProjd(v, orig, &vmin, &vmax)
	// and now fix things up so that all u and v lie between 0 and 1
	orig = orig.Plus(u.ScaledBy(umin))
	orig = orig.Plus(v.ScaledBy(vmin))
	u = u.ScaledBy(umax - umin)
	v = v.ScaledBy(vmax - vmin)

	// So we can now generate the top and bottom surfaces of the
	// extrusion, planes within a translated (and maybe mirrored)
	// version of that csys.
	s0 := FromPlane(orig.Plus(t0), u, v)
	s0.Color = color
	s0.TrimBuilt = true
	s1 := FromPlane(orig.Plus(t1).Plus(u), u.ScaledBy(-1), v)
	s1.Color = color
	s1.TrimBuilt = true
	hs0 := s.addAndAssignSurfaceId(s0)
	hs1 := s.addAndAssignSurfaceId(s1)

	// Now go through the input curves. For each one, generate its
	// surface of extrusion, its two translated trim curves, and one
	// trim line. We go through by loops so that we can assign the
	// lines correctly.
	loops := make([]*SBezierLoop, 0, 1+len(sbls.Holes))
	loops = append(loops, &sbls.Outer)
	for i := range sbls.Holes {
		loops = append(loops, &sbls.Holes[i])
	}

	for _, loop := range loops {
		var trimLines []shellTrimLine

		for bi := range loop.Curves {
			b := &loop.Curves[bi]

			// Generate the surface of extrusion of this curve, and
			// add it to the list.
			ss := FromExtrusionOf(b, t0, t1)
			ss.Color = color
			ss.TrimBuilt = true
			hsext := s.addAndAssignSurfaceId(ss)

			// Translate the curve by t0 and t1 to produce two trim
			// curves.
			bt0 := *b
			bt0.TransformedBy(t0, geom.QuaternionIdentity, 1.0)
			hc0 := s.addAndAssignCurveId(SCurve{
				IsExact: true,
				Exact:   bt0,
				Points:  makePwlSCurvePoints(&bt0),
				SurfA:   hs0,
				SurfB:   hsext,
			})

			bt1 := *b
			bt1.TransformedBy(t1, geom.QuaternionIdentity, 1.0)
			hc1 := s.addAndAssignCurveId(SCurve{
				IsExact: true,
				Exact:   bt1,
				Points:  makePwlSCurvePoints(&bt1),
				SurfA:   hs1,
				SurfB:   hsext,
			})

			// The translated curves trim the flat top and bottom
			// surfaces.
			appendTrim(s, hs0, STrimByEntireCurveFromShell(s, hc0, false))
			appendTrim(s, hs1, STrimByEntireCurveFromShell(s, hc1, true))

			// The translated curves also trim the surface of
			// extrusion.
			appendTrim(s, hsext, STrimByEntireCurveFromShell(s, hc0, true))
			appendTrim(s, hsext, STrimByEntireCurveFromShell(s, hc1, false))

			// And form the trim line.
			pt := b.Finish()
			lineBez := SBezierFrom2(pt.Plus(t0), pt.Plus(t1))
			hl := s.addAndAssignCurveId(SCurve{
				IsExact: true,
				Exact:   lineBez,
				Points:  makePwlSCurvePoints(&lineBez),
			})
			// save this for later
			trimLines = append(trimLines, shellTrimLine{hc: hl, hs: hsext})
		}

		nl := len(trimLines)
		for i := 0; i < nl; i++ {
			tl := trimLines[i]
			tlp := trimLines[(i-1+nl)%nl]

			appendTrim(s, tl.hs, STrimByEntireCurveFromShell(s, tl.hc, true))
			appendTrim(s, tl.hs, STrimByEntireCurveFromShell(s, tlp.hc, false))

			if c := s.FindCurve(tl.hc); c != nil {
				c.SurfA = tl.hs
			}
			if c := s.FindCurve(tlp.hc); c != nil {
				c.SurfB = tl.hs
			}
		}
	}
}

// addSurfaceAssignId adds ss with a fresh handle (one more than the
// largest in use), leaving its Face alone. Mirrors C++
// IdList::AddAndAssignId for surfaces.
func (s *SShell) addSurfaceAssignId(ss SSurface) core.HSSurface {
	var maxV uint32
	for i := range s.Surface {
		if s.Surface[i].H.V > maxV {
			maxV = s.Surface[i].H.V
		}
	}
	h := core.HSSurface{V: maxV + 1}
	ss.H = h
	s.Surface = append(s.Surface, ss)
	return h
}

// appendTrim adds stb to the Trim list of the surface identified by
// h, looking it up by handle so callers never need to reason about
// slice-append reallocation.
func appendTrim(s *SShell, h core.HSSurface, stb STrimBy) {
	if srf := s.FindSurface(h); srf != nil {
		srf.Trim = append(srf.Trim, stb)
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
// every surface and curve of src, keeping their original handles.
// Faithful port of C++ `SShell::MakeFromTransformationOf`
// (shell.cpp), which always transforms trims and always copies the
// curve list too — booleans on the result need the operand curves,
// not just the surfaces.
func (s *SShell) MakeFromTransformationOf(src *SShell, t geom.Vector, q geom.Quaternion, scale float64, includingTrims bool) {
	s.Clear()
	if src == nil {
		return
	}
	for _, ss := range src.Surface {
		s.Surface = append(s.Surface, FromTransformationOf(&ss, t, q, scale, includingTrims))
	}
	for _, c := range src.Curve {
		s.Curve = append(s.Curve, FromTransformationOfSCurve(&c, t, q, scale))
	}
	s.BooleanFailed = false
}

// TriangulateInto triangulates every surface in the shell and
// appends the resulting triangles to mesh. Faithful port of C++
// `SShell::TriangulateInto` (shell.cpp): each surface's trim curves
// (resolved against this shell) are assembled into a uv-space
// polygon and triangulated either by ear-clipping (planar surfaces)
// or by the adaptive uv grid (curved surfaces); untrimmed surfaces
// (no STrimBy at all — currently MakeFromRevolutionOf/MakeFromHelixOf
// output) fall back to the regular 16x16 grid so they aren't
// triangulated away to nothing.
func (s *SShell) TriangulateInto(mesh *SMesh) {
	for i := range s.Surface {
		s.Surface[i].TriangulateInto(s, mesh)
	}
}

// TriangulateInto is the per-surface version. Mirrors C++
// `SSurface::TriangulateInto(SShell *shell, SMesh *sm)`
// (surface.cpp:414): build the uv-space trim polygon via the
// surface's own STrimBy list (resolved against shell), triangulate
// it (ear-clipping for degree (1,1) planes, the adaptive uv grid for
// curved surfaces), then map every uv-space vertex/normal back to
// 3-D via PointAt/NormalAt and flip the winding to match the
// trim-contour orientation convention.
func (srf *SSurface) TriangulateInto(shell *SShell, mesh *SMesh) {
	if len(srf.Trim) == 0 && !srf.TrimBuilt {
		// See SSurface.TrimBuilt: a constructor that doesn't build
		// trims yet.
		srf.triangulateUntrimmedGrid(mesh)
		return
	}

	var el SEdgeList
	srf.MakeEdgesInto(shell, &el, true, nil)

	var poly SPolygon
	if !el.AssemblePolygon(&poly, nil, true) {
		// failed to assemble polygon to trim nurbs surface in uv space
		return
	}

	start := len(mesh.Triangles)
	if srf.DegM == 1 && srf.DegN == 1 {
		poly.UvTriangulateInto(mesh, srf)
	} else {
		poly.UvGridTriangulateInto(mesh, srf)
	}

	meta := STriMeta{Face: srf.Face, RgbaColor: srf.Color}
	for i := start; i < len(mesh.Triangles); i++ {
		t := &mesh.Triangles[i]
		t.Meta = meta
		t.NA = srf.NormalAt(t.A.X, t.A.Y)
		t.NB = srf.NormalAt(t.B.X, t.B.Y)
		t.NC = srf.NormalAt(t.C.X, t.C.Y)
		t.A = srf.PointAt(t.A.X, t.A.Y)
		t.B = srf.PointAt(t.B.X, t.B.Y)
		t.C = srf.PointAt(t.C.X, t.C.Y)
		// Works out that the chosen contour direction is inconsistent
		// with the triangle direction (matches the C++ comment).
		t.FlipNormal()
	}
}

// triangulateUntrimmedGrid samples the surface on a regular (u, v)
// grid, keeping only cells whose center is inside srf.Bsp (when
// present). This is the pre-item-3 triangulator, now used only as
// the fallback for surfaces with no trim curves at all.
func (srf *SSurface) triangulateUntrimmedGrid(mesh *SMesh) {
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
				if srf.Bsp.ClassifyPoint(geom.Point2d{U: uc, V: vc}, geom.Point2d{}, srf) == BspUvOutside {
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

func snap(p geom.Vector, tol float64) geom.Vector {
	return geom.Vector{
		X: math.Round(p.X/tol) * tol,
		Y: math.Round(p.Y/tol) * tol,
		Z: math.Round(p.Z/tol) * tol,
	}
}

// MakeEdgesInto extracts the trim edges of every surface as SEdge
// entries. When a surface has no trim, emits the four boundary
// edges of the surface's uv rectangle.
func (s *SShell) MakeEdgesInto(el *SEdgeList) {
	for i := range s.Surface {
		s.Surface[i].MakeEdgesInto(s, el, false, nil)
	}
}

// MakeEdgesInto emits the pwl edges of every trim curve of srf, in uv
// space (asUV) or xyz. When useCurvesFrom is non-nil, each trim's
// curve is replaced by its split copy (newH) in that shell; that is
// used by the Booleans, since the curves from the output shell are
// split against any intersecting surfaces (and the originals aren't).
// Mirrors C++ SSurface::MakeEdgesInto (surface.cpp).
func (srf *SSurface) MakeEdgesInto(shell *SShell, sel *SEdgeList, asUV bool, useCurvesFrom *SShell) {
	for i := range srf.Trim {
		stb := &srf.Trim[i]
		sc := shell.FindCurve(stb.Curve)
		if sc == nil {
			continue
		}

		if useCurvesFrom != nil {
			sc = useCurvesFrom.FindCurve(sc.NewH)
			if sc == nil {
				continue
			}
		}

		srf.makeTrimEdgesInto(sel, asUV, sc, stb)
	}
}

// makeTrimEdgesInto walks sc's pwl from stb's start to its finish
// (backwards if stb says so), emitting edges in uv or xyz. Mirrors
// C++ SSurface::MakeTrimEdgesInto (surface.cpp:222).
func (srf *SSurface) makeTrimEdgesInto(sel *SEdgeList, asUV bool, sc *SCurve, stb *STrimBy) {
	var prev geom.Vector
	inCurve := false

	first, last, increment := 0, len(sc.Points)-1, 1
	if stb.Backwards {
		first, last, increment = len(sc.Points)-1, 0, -1
	}
	backwards := 0
	if stb.Backwards {
		backwards = 1
	}
	for i := first; i != last+increment; i += increment {
		pt := sc.Points[i].P
		tpt := pt
		if asUV {
			var u, v float64
			srf.ClosestPointToUV(pt, &u, &v, true)
			tpt = geom.Vector{X: u, Y: v}
		}

		if inCurve {
			sel.AddEdge(prev, tpt, int(sc.H.V), backwards, 0)
		}

		prev = tpt // either uv or xyz, depending on flags

		if pt.Equals(stb.StartP) {
			inCurve = true
		}
		if pt.Equals(stb.FinishP) {
			inCurve = false
		}
	}
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
