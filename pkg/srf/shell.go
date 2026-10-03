package srf

import (
	"math"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/internal/par"
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

// MakeFromCopyOf copies src (C++ SShell::MakeFromCopyOf): a
// transformation by the identity, which, like C++'s, still rotates
// and translates the curves' exact forms (so turns -0 into +0).
func (s *SShell) MakeFromCopyOf(src *SShell) {
	if s == src {
		panic("Can't make from copy of self")
	}
	s.MakeFromTransformationOf(src, geom.Vector{}, geom.QuaternionIdentity, 1.0)
}

// addAndAssignSurfaceId appends ss to s.Surface with a fresh
// sequential handle, leaving its face alone (C++
// IdList::AddAndAssignId).
func (s *SShell) addAndAssignSurfaceId(ss SSurface) core.HSSurface {
	return s.addSurfaceAssignId(ss)
}

// ShellGroup is what the revolution constructors read from the group
// they build a shell for (C++ passes its Group): the faces of the new
// surfaces are entities from the group's remap table.
type ShellGroup interface {
	// Remap is the handle of copy copyNumber of entity in (C++
	// Group::Remap).
	Remap(in core.HEntity, copyNumber int) core.HEntity
	// EntityExists reports whether the sketch has entity h.
	EntityExists(h core.HEntity) bool
}

// The remap keys of the faces the shell constructors assign (C++
// Group::REMAP_LINE_TO_FACE, REMAP_LATHE_START, REMAP_LATHE_END).
const (
	remapLineToFace = 1004
	remapLatheStart = 1006
	remapLatheEnd   = 1007
)

// lineToFace is the face of the surface swept by sb: the group's copy
// of the line segment sb came from, if the sketch has it.
func lineToFace(group ShellGroup, sb *SBezier) (FaceID, bool) {
	if sb.Entity == 0 {
		return 0, false
	}
	hface := group.Remap(core.HEntity{V: sb.Entity}, remapLineToFace)
	if !group.EntityExists(hface) {
		return 0, false
	}
	return FaceID(hface.V), true
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
func makePwlSCurvePoints(b *SBezier, maxDt float64) []SCurvePt {
	var raw []geom.Vector
	b.MakePwlIntoPoints(&raw, 0, maxDt)
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
				Points:  makePwlSCurvePoints(&bt0, 0),
				SurfA:   hs0,
				SurfB:   hsext,
			})

			bt1 := *b
			bt1.TransformedBy(t1, geom.QuaternionIdentity, 1.0)
			hc1 := s.addAndAssignCurveId(SCurve{
				IsExact: true,
				Exact:   bt1,
				Points:  makePwlSCurvePoints(&bt1, 0),
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
				Points:  makePwlSCurvePoints(&lineBez, 0),
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

// CheckNormalAxisRelationship reports whether the direction of
// revolution/extrusion ends up parallel to the normal of the sketch,
// on the side of the axis where the sketch is. Faithful port of C++
// `SShell::CheckNormalAxisRelationship` (shell.cpp).
func CheckNormalAxisRelationship(sbls *SBezierLoopSet, pt, axis geom.Vector, da, dx float64) bool {
	var pto geom.Vector
	md := math.Inf(-1)
	for _, sbl := range sbls.loops() {
		for bi := range sbl.Curves {
			sb := &sbl.Curves[bi]
			// Choose the point farthest from the axis; we'll get garbage
			// if we choose a point that lies on the axis, for example.
			// (And our surface will be self-intersecting if the sketch
			// spans the axis, so don't worry about that.)
			for i := 0; i <= sb.Degree; i++ {
				p := sb.Ctrl[i]
				d := p.DistanceToLine(pt, axis)
				if d > md {
					md = d
					pto = p
				}
			}
		}
	}
	ptc := pto.ClosestPointOnLine(pt, axis)
	up := axis.Cross(pto.Minus(ptc)).ScaledBy(da)
	vp := up.Plus(axis.ScaledBy(dx))

	return vp.Dot(sbls.Normal) > 0
}

// isOnAxisLine reports whether sb is a line lying on the axis of
// revolution (pt, axis); such a curve contributes no surface.
func isOnAxisLine(sb *SBezier, pt, axis geom.Vector) bool {
	return sb.Degree == 1 &&
		sb.Ctrl[0].DistanceToLine(pt, axis) < lengthEps &&
		sb.Ctrl[1].DistanceToLine(pt, axis) < lengthEps
}

// transformedCopy returns sb transformed by (t, q, scale), leaving sb
// alone. Mirrors the C++ `SBezier::TransformedBy`, which returns a
// copy.
func transformedCopy(sb *SBezier, t geom.Vector, q geom.Quaternion, scale float64) SBezier {
	ret := *sb
	ret.TransformedBy(t, q, scale)
	return ret
}

// addCurveTrimming adds sc to the shell and trims its SurfA and SurfB
// by it, SurfA with the given sense and SurfB with the opposite one.
func (s *SShell) addCurveTrimming(sc SCurve, backwardsA bool) {
	hc := s.addAndAssignCurveId(sc)
	appendTrim(s, sc.SurfA, STrimByEntireCurveFromShell(s, hc, backwardsA))
	appendTrim(s, sc.SurfB, STrimByEntireCurveFromShell(s, hc, !backwardsA))
}

// MakeFromHelicalRevolutionOf sweeps a planar BezierLoopSet around the
// axis (pt, axis) from angle angles to anglef while translating along
// the axis from dists to distf. The swept surfaces are split into
// sections of at most 90 degrees and capped by two planes. Faithful
// port of C++ `SShell::MakeFromHelicalRevolutionOf` (shell.cpp).
//
// The sketch must not contain the axis of revolution as a
// non-construction line when dist is nonzero. startFace and endFace
// are the face ids of the caps at angles and anglef (C++
// Group::Remap(NO_ENTITY, REMAP_LATHE_START / REMAP_LATHE_END)); a
// zero id leaves the cap's face at its surface handle. The C++
// REMAP_LINE_TO_FACE of the swept surfaces is not done: SBezier
// carries no source entity.
func (s *SShell) MakeFromHelicalRevolutionOf(sbls *SBezierLoopSet, pt, axis geom.Vector,
	color core.RgbaColor, group ShellGroup, angles, anglef, dists, distf float64) {
	if sbls == nil {
		return
	}
	i0 := len(s.Surface) // number of pre-existing surfaces
	dist := distf - dists
	sections := int(math.Abs(anglef-angles)/(math.Pi/2) + 1)
	wedge := (anglef - angles) / float64(sections)
	startMapping, endMapping := remapLatheStart, remapLatheEnd

	if CheckNormalAxisRelationship(sbls, pt, axis, anglef-angles, distf-dists) {
		angles, anglef = anglef, angles
		dists, distf = distf, dists
		dist = -dist
		wedge = -wedge
		startMapping, endMapping = endMapping, startMapping
	}

	// Define a coordinate system to contain the original sketch, and get
	// a bounding box in that csys
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

	// So we can now generate the end caps of the extrusion within
	// a translated and rotated (and maybe mirrored) version of that csys.
	s0 := FromPlane(orig.RotatedAbout(pt, axis, angles).Plus(axis.ScaledBy(dists)),
		u.RotatedAroundAxis(axis, angles), v.RotatedAroundAxis(axis, angles))
	s0.Color = color
	s0.TrimBuilt = true

	s1 := FromPlane(orig.Plus(u).RotatedAbout(pt, axis, anglef).Plus(axis.ScaledBy(distf)),
		u.ScaledBy(-1).RotatedAroundAxis(axis, anglef), v.RotatedAroundAxis(axis, anglef))
	s1.Color = color
	s1.TrimBuilt = true

	s0.Face = FaceID(group.Remap(core.NoEntity, startMapping).V)
	s1.Face = FaceID(group.Remap(core.NoEntity, endMapping).V)

	hs0 := s.addAndAssignSurfaceId(s0)
	hs1 := s.addAndAssignSurfaceId(s1)

	// Now we actually build and trim the swept surfaces. One loop at a time.
	curve := 0 // the curves of the loops, counted from 1
	for _, sbl := range sbls.loops() {
		nc := len(sbl.Curves)
		hsl := make([][]core.HSSurface, nc)

		// This is where all the NURBS are created
		for bi := range sbl.Curves {
			sb := &sbl.Curves[bi]
			curve++
			revs := make([]core.HSSurface, sections)
			for j := 0; j < sections; j++ {
				if dist == 0 && isOnAxisLine(sb, pt, axis) {
					// This is a line on the axis of revolution; it does
					// not contribute a surface.
					revs[j] = core.HSSurface{}
				} else {
					ss := FromRevolutionOf(sb, pt, axis,
						angles+wedge*float64(j), angles+wedge*float64(j+1),
						dists+float64(j)*dist/float64(sections), dists+float64(j+1)*dist/float64(sections))
					ss.Color = color
					ss.TrimBuilt = true
					if face, ok := lineToFace(group, sb); ok {
						ss.Face = face
					} else {
						ss.Origin = helixOrigin(curve, j)
					}
					revs[j] = s.addAndAssignSurfaceId(ss)
				}
			}
			hsl[bi] = revs
		}
		// Still the same loop. Need to create trim curves
		for i := 0; i < nc; i++ {
			revs, revsp := hsl[i], hsl[(i-1+nc)%nc]
			sb := &sbl.Curves[i]

			// we will need the grid t-values for this entire row of surfaces
			var tValues []float64
			if revs[0].V != 0 {
				tValues = append(tValues, 0.0)
				s.FindSurface(revs[0]).MakeTriangulationGridInto(&tValues, 0.0, 1.0, true, 0)
			}
			// we generate one more curve than we did surfaces
			for j := 0; j <= sections; j++ {
				qs := geom.QuaternionFromAxisAngle(axis, angles+wedge*float64(j))
				// we want Q*(x - p) + p = Q*x + (p - Q*p)
				ts := pt.Minus(qs.Rotate(pt)).Plus(axis.ScaledBy(dists + float64(j)*dist/float64(sections)))

				// If this input curve generated a surface, then trim that
				// surface with the rotated version of the input curve.
				if revs[0].V != 0 { // not revs[j] because crash on j==sections
					sc := SCurve{IsExact: true, Exact: transformedCopy(sb, ts, qs, 1.0)}
					// make the PWL for the curve based on t value list
					for x, t := range tValues {
						sc.Points = append(sc.Points, SCurvePt{
							P:      sc.Exact.PointAt(t),
							Vertex: x == 0 || x == len(tValues)-1,
						})
					}

					// the surfaces already exists so trim with this curve
					if j < sections {
						sc.SurfA = revs[j]
					} else {
						sc.SurfA = hs1 // end cap
					}
					if j > 0 {
						sc.SurfB = revs[j-1]
					} else {
						sc.SurfB = hs0 // staring cap
					}
					s.addCurveTrimming(sc, true)
				} else if j == 0 { // curve was on the rotation axis and is shared by the end caps.
					sc := SCurve{IsExact: true, Exact: transformedCopy(sb, ts, qs, 1.0)}
					sc.Points = makePwlSCurvePoints(&sc.Exact, 0)
					sc.SurfA = hs1 // end cap
					sc.SurfB = hs0 // staring cap
					s.addCurveTrimming(sc, true)
				}

				// And if this input curve and the one after it both generated
				// surfaces, then trim both of those by the appropriate
				// curve based on the control points.
				if j < sections && revs[j].V != 0 && revsp[j].V != 0 {
					ss := s.FindSurface(revs[j])

					sc := SCurve{IsExact: true, Exact: SBezierFrom3(ss.Ctrl[0][0], ss.Ctrl[0][1], ss.Ctrl[0][2])}
					sc.Exact.Weight[1] = ss.Weight[0][1]
					maxDt := 0.5
					if sc.Exact.Degree > 1 {
						maxDt = 0.125
					}
					sc.Points = makePwlSCurvePoints(&sc.Exact, maxDt)
					sc.SurfA = revs[j]
					sc.SurfB = revsp[j]
					s.addCurveTrimming(sc, false)
				}
			}
		}
	}

	if dist == 0 {
		s.MakeFirstOrderRevolvedSurfaces(pt, axis, i0)
	}
}

// helixOrigin is the local origin of the surface that section j of the
// k-th curve sweeps, for a surface no entity names: the group makes it
// its own with OriginFor. A helix has as many sections as its turns
// take, so a surface's place among the group's surfaces moves when the
// helix gets longer; this does not, and the names of its edges stay
// (roadmap 14.2).
func helixOrigin(k, j int) uint32 {
	return LocalOriginBit | uint32(k&0x1f)<<10 | uint32(j&0x3ff)
}

// LocalOriginBit marks an Origin that a shell constructor gave a
// surface from what made it, for its group to make its own with
// OriginFor in place of the surface's place in the shell.
const LocalOriginBit = 0x8000

// MakeFromRevolutionOf revolves a planar BezierLoopSet a full turn
// around the axis (pt, axis). Each curve becomes four 90-degree
// surfaces of revolution, trimmed by the rotated copies of the curve
// and by the circles swept by its endpoints. Faithful port of C++
// `SShell::MakeFromRevolutionOf` (shell.cpp).
func (s *SShell) MakeFromRevolutionOf(sbls *SBezierLoopSet, pt, axis geom.Vector, color core.RgbaColor, group ShellGroup) {
	if sbls == nil {
		return
	}
	i0 := len(s.Surface) // number of pre-existing surfaces

	if CheckNormalAxisRelationship(sbls, pt, axis, 1.0, 0.0) {
		axis = axis.ScaledBy(-1)
	}

	// Now we actually build and trim the surfaces.
	for _, sbl := range sbls.loops() {
		nc := len(sbl.Curves)
		hsl := make([][4]core.HSSurface, nc)

		for bi := range sbl.Curves {
			sb := &sbl.Curves[bi]
			for j := 0; j < 4; j++ {
				if isOnAxisLine(sb, pt, axis) {
					// This is a line on the axis of revolution; it does
					// not contribute a surface.
					hsl[bi][j] = core.HSSurface{}
				} else {
					ss := FromRevolutionOf(sb, pt, axis, (math.Pi/2)*float64(j),
						(math.Pi/2)*float64(j+1), 0.0, 0.0)
					ss.Color = color
					ss.TrimBuilt = true
					if face, ok := lineToFace(group, sb); ok {
						ss.Face = face
					}
					hsl[bi][j] = s.addAndAssignSurfaceId(ss)
				}
			}
		}

		for i := 0; i < nc; i++ {
			revs, revsp := hsl[i], hsl[(i-1+nc)%nc]
			sb := &sbl.Curves[i]

			for j := 0; j < 4; j++ {
				qs := geom.QuaternionFromAxisAngle(axis, (math.Pi/2)*float64(j))
				// we want Q*(x - p) + p = Q*x + (p - Q*p)
				ts := pt.Minus(qs.Rotate(pt))

				// If this input curve generate a surface, then trim that
				// surface with the rotated version of the input curve.
				if revs[j].V != 0 {
					sc := SCurve{IsExact: true, Exact: transformedCopy(sb, ts, qs, 1.0)}
					sc.Points = makePwlSCurvePoints(&sc.Exact, 0)
					sc.SurfA = revs[j]
					sc.SurfB = revs[(j+3)%4]
					s.addCurveTrimming(sc, true)
				}

				// And if this input curve and the one after it both generated
				// surfaces, then trim both of those by the appropriate
				// circle.
				if revs[j].V != 0 && revsp[j].V != 0 {
					ss := s.FindSurface(revs[j])

					sc := SCurve{IsExact: true, Exact: SBezierFrom3(ss.Ctrl[0][0], ss.Ctrl[0][1], ss.Ctrl[0][2])}
					sc.Exact.Weight[1] = ss.Weight[0][1]
					sc.Points = makePwlSCurvePoints(&sc.Exact, 0)
					sc.SurfA = revs[j]
					sc.SurfB = revsp[j]
					s.addCurveTrimming(sc, false)
				}
			}
		}
	}

	s.MakeFirstOrderRevolvedSurfaces(pt, axis, i0)
}

// MakeFirstOrderRevolvedSurfaces rewrites the surfaces from index i0
// on that are revolutions of a line into lower-order forms where
// possible: a revolution in a plane perpendicular to the axis becomes
// a degree (1, 1) plane, and a cylinder is transposed so that it is
// recognized as a surface of extrusion. Faithful port of C++
// `SShell::MakeFirstOrderRevolvedSurfaces` (shell.cpp).
func (s *SShell) MakeFirstOrderRevolvedSurfaces(pt, axis geom.Vector, i0 int) {
	s.makeFirstOrderRevolvedRange(pt, axis, i0, len(s.Surface))
}

// makeFirstOrderRevolvedRange is MakeFirstOrderRevolvedSurfaces for
// the surfaces from index i0 up to i1.
func (s *SShell) makeFirstOrderRevolvedRange(pt, axis geom.Vector, i0, i1 int) {
	for i := i0; i < i1; i++ {
		srf := &s.Surface[i]

		// Revolution of a line; this is potentially a plane, which we can
		// rewrite to have degree (1, 1).
		if srf.DegM != 1 || srf.DegN != 2 {
			continue
		}
		// close start, far start, far finish
		var cs, fs, ff geom.Vector
		d0 := srf.Ctrl[0][0].DistanceToLine(pt, axis)
		d1 := srf.Ctrl[1][0].DistanceToLine(pt, axis)

		if d0 > d1 {
			cs = srf.Ctrl[1][0]
			fs = srf.Ctrl[0][0]
			ff = srf.Ctrl[0][2]
		} else {
			cs = srf.Ctrl[0][0]
			fs = srf.Ctrl[1][0]
			ff = srf.Ctrl[1][2]
		}

		// origin close, origin far
		oc := cs.ClosestPointOnLine(pt, axis)
		of := fs.ClosestPointOnLine(pt, axis)

		if oc.Equals(of) {
			// This is a plane, not a (non-degenerate) cone.
			oldn := srf.NormalAt(0.5, 0.5)

			u := fs.Minus(of)
			v := axis.Cross(u).WithMagnitude(1)

			vm := ff.Minus(of).Dot(v)
			v = v.ScaledBy(vm)

			srf.DegM = 1
			srf.DegN = 1
			srf.Ctrl[0][0] = of
			srf.Ctrl[0][1] = of.Plus(u)
			srf.Ctrl[1][0] = of.Plus(v)
			srf.Ctrl[1][1] = of.Plus(u).Plus(v)
			srf.Weight[0][0] = 1
			srf.Weight[0][1] = 1
			srf.Weight[1][0] = 1
			srf.Weight[1][1] = 1

			if oldn.Dot(srf.NormalAt(0.5, 0.5)) < 0 {
				srf.Ctrl[0][0], srf.Ctrl[1][0] = srf.Ctrl[1][0], srf.Ctrl[0][0]
				srf.Ctrl[0][1], srf.Ctrl[1][1] = srf.Ctrl[1][1], srf.Ctrl[0][1]
			}
			continue
		}

		if math.Abs(d0-d1) < lengthEps {
			// This is a cylinder; so transpose it so that we'll recognize
			// it as a surface of extrusion.
			sn := *srf

			// Transposing u and v flips the normal, so reverse u to
			// flip it again and put it back where we started.
			sn.DegM = 2
			sn.DegN = 1
			for dm := 0; dm <= 1; dm++ {
				for dn := 0; dn <= 2; dn++ {
					sn.Ctrl[dn][dm] = srf.Ctrl[1-dm][dn]
					sn.Weight[dn][dm] = srf.Weight[1-dm][dn]
				}
			}

			*srf = sn
			continue
		}
	}
}

// MakeFromTransformationOf applies (translate, rotate, scale) to
// every surface and curve of src, keeping their original handles.
// MakeFromTransformationOf adds the surfaces and curves of a, scaled,
// then rotated, then translated, keeping their handles (C++
// SShell::MakeFromTransformationOf).
func (s *SShell) MakeFromTransformationOf(a *SShell, t geom.Vector, q geom.Quaternion, scale float64) {
	s.BooleanFailed = false
	for i := range a.Surface {
		s.Surface = append(s.Surface, FromTransformationOf(&a.Surface[i], t, q, scale, true))
	}

	for i := range a.Curve {
		s.Curve = append(s.Curve, FromTransformationOfSCurve(&a.Curve[i], t, q, scale))
	}
}

// TriangulateInto triangulates every surface in the shell and
// appends the resulting triangles to mesh. Faithful port of C++
// `SShell::TriangulateInto` (shell.cpp): each surface's trim curves
// (resolved against this shell) are assembled into a uv-space
// polygon and triangulated either by ear-clipping (planar surfaces)
// or by the adaptive uv grid (curved surfaces). See SSurface.TrimBuilt
// for the one Go-only fallback.
//
// The surfaces are triangulated at once, as under C++'s OpenMP, each
// into a mesh of its own; the meshes are joined in the surfaces' order,
// which is the order C++ built without OpenMP adds them in.
func (s *SShell) TriangulateInto(mesh *SMesh) {
	if len(s.Surface) < 2 || par.Workers() < 2 {
		for i := range s.Surface {
			s.Surface[i].TriangulateInto(s, mesh)
		}
		return
	}
	ms := make([]SMesh, len(s.Surface))
	par.For(len(s.Surface), func(i int) {
		s.Surface[i].TriangulateInto(s, &ms[i])
	})
	for i := range ms {
		mesh.Triangles = append(mesh.Triangles, ms[i].Triangles...)
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
	if srf.flat() {
		poly.UvTriangulateInto(mesh, srf)
	} else {
		poly.uvGridTriangulateInto(mesh, srf, &el)
	}

	meta := STriMeta{Face: shell.displayFace(srf), RgbaColor: srf.Color}
	off := srf.trimPointsOff(shell)
	at := func(uv geom.Vector) geom.Vector {
		if p, ok := off[[2]float64{uv.X, uv.Y}]; ok {
			return p
		}
		return srf.PointAt(uv.X, uv.Y)
	}
	for i := start; i < len(mesh.Triangles); i++ {
		t := &mesh.Triangles[i]
		t.Meta = meta
		t.NA = srf.NormalAt(t.A.X, t.A.Y)
		t.NB = srf.NormalAt(t.B.X, t.B.Y)
		t.NC = srf.NormalAt(t.C.X, t.C.Y)
		t.A = at(t.A)
		t.B = at(t.B)
		t.C = at(t.C)
		// Works out that the chosen contour direction is inconsistent
		// with the triangle direction (matches the C++ comment).
		t.FlipNormal()
	}
}

// trimPointsOff are the points of the surface's trims that it misses
// by more than LengthEps, or all of them for a surface that snaps to
// them, by their parameters on it: a file's edges lie on their faces
// only within its tolerance, and the mesh's vertices on a trim must be
// the trim's own, which the face beyond it shares, or the two meshes
// part there. Trims the kernel made lie on their surfaces, and none are
// listed.
func (srf *SSurface) trimPointsOff(shell *SShell) map[[2]float64]geom.Vector {
	var out map[[2]float64]geom.Vector
	for i := range srf.Trim {
		sc := shell.FindCurve(srf.Trim[i].Curve)
		if sc == nil {
			continue
		}
		for _, cp := range sc.Points {
			var u, v float64
			srf.ClosestPointToUV(cp.P, &u, &v, true)
			if srf.SnapToTrims || srf.PointAt(u, v).Minus(cp.P).Magnitude() > geom.LengthEps {
				if out == nil {
					out = map[[2]float64]geom.Vector{}
				}
				out[[2]float64{u, v}] = cp.P
			}
		}
	}
	return out
}

// displayFace is what the triangles of srf carry as their face: its
// entity, or for a surface no entity names its Origin, the smallest of
// its sheet's so that the patches of one face carry one. The edges
// between two faces are drawn where their triangles' faces differ, and
// a face is picked by it, so faces with only an Origin, a fillet's or
// a Boolean tool's, are drawn apart and picked as others are (roadmap
// 14.1).
func (s *SShell) displayFace(srf *SSurface) FaceID {
	if srf.Face != 0 || srf.Origin == 0 {
		return srf.Face
	}
	o := srf.Origin
	if srf.Sheet != 0 && s != nil {
		for i := range s.Surface {
			t := &s.Surface[i]
			if t.Sheet == srf.Sheet && t.Face == 0 && t.Origin != 0 && t.Origin < o {
				o = t.Origin
			}
		}
	}
	return FaceID(o)
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

// MakeSectionEdgesInto reports the trim curves of the shell's planar
// faces that lie in the plane n . p = d: exactly into sbl where it can,
// otherwise as pwl edges into sel (C++ SShell::MakeSectionEdgesInto).
func (s *SShell) MakeSectionEdgesInto(n geom.Vector, d float64, sel *SEdgeList, sbl *SBezierList) {
	for i := range s.Surface {
		if s.Surface[i].CoincidentWithPlane(n, d) {
			s.Surface[i].MakeSectionEdgesInto(s, sel, sbl)
		}
	}
}

// ExactSurfaceTangentAt is the direction, at p, of the curve where srfA
// meets srfB, pointing along dir (C++ SSurface::ExactSurfaceTangentAt).
func (srf *SSurface) ExactSurfaceTangentAt(p geom.Vector, srfA, srfB *SSurface, dir geom.Vector) geom.Vector {
	var puva, puvb geom.Point2d
	srfA.ClosestPointTo(p, &puva, true)
	srfB.ClosestPointTo(p, &puvb, true)
	ts := srfA.NormalAtUv(puva).Cross(srfB.NormalAtUv(puvb))
	ts = ts.WithMagnitude(1)
	if ts.Dot(dir) < 0 {
		ts = ts.ScaledBy(-1)
	}
	return ts
}

// MakeSectionEdgesInto reports our trim curves. If a trim curve is
// exact and sbl is not nil, then add its exact form to sbl. Otherwise,
// add its piecewise linearization to sel (C++
// SSurface::MakeSectionEdgesInto).
func (srf *SSurface) MakeSectionEdgesInto(shell *SShell, sel *SEdgeList, sbl *SBezierList) {
	for k := range srf.Trim {
		stb := &srf.Trim[k]
		sc := shell.FindCurve(stb.Curve)
		sb := &sc.Exact

		if sbl != nil && sc.IsExact && (sb.Degree != 1 || sel == nil) {
			var ts, tf float64
			if stb.Backwards {
				tf, _ = sb.ClosestPointTo(stb.StartP, true)
				ts, _ = sb.ClosestPointTo(stb.FinishP, true)
			} else {
				ts, _ = sb.ClosestPointTo(stb.StartP, true)
				tf, _ = sb.ClosestPointTo(stb.FinishP, true)
			}
			var junkBef, keepAft SBezier
			sb.SplitAt(ts, &junkBef, &keepAft)
			// In the kept piece, the range that used to go from ts to 1
			// now goes from 0 to 1; so rescale tf appropriately.
			tf = (tf - ts) / (1 - ts)

			var keepBef, junkAft SBezier
			keepAft.SplitAt(tf, &keepBef, &junkAft)

			sbl.Curves = append(sbl.Curves, keepBef)
		} else if sbl != nil && sel == nil && !sc.IsExact {
			// We must approximate this trim curve, as piecewise cubic sections.
			srfA := shell.FindSurface(sc.SurfA)
			srfB := shell.FindSurface(sc.SurfB)

			s, f := stb.StartP, stb.FinishP
			if stb.Backwards {
				s, f = f, s
			}

			sp := 0
			for ; sp < len(sc.Points); sp++ {
				if s.Equals(sc.Points[sp].P) {
					break
				}
			}
			if sp >= len(sc.Points) {
				return
			}
			fp := sp
			for ; fp < len(sc.Points); fp++ {
				if f.Equals(sc.Points[fp].P) {
					break
				}
			}
			if fp >= len(sc.Points) {
				return
			}
			// So now the curve we want goes from elem[sp] to elem[fp]

			for sp < fp {
				// Initially, we'll try approximating the entire trim curve
				// as a single Bezier segment
				fpt := fp

				for {
					// So construct a cubic Bezier with the correct endpoints
					// and tangents for the current span.
					st := sc.Points[sp].P
					ft := sc.Points[fpt].P
					sf := ft.Minus(st)
					m := sf.Magnitude() / 3

					stan := srf.ExactSurfaceTangentAt(st, srfA, srfB, sf)
					ftan := srf.ExactSurfaceTangentAt(ft, srfA, srfB, sf)

					sb := SBezierFrom4(st,
						st.Plus(stan.WithMagnitude(m)),
						ft.Minus(ftan.WithMagnitude(m)),
						ft)

					// And test how much this curve deviates from the
					// intermediate points (if any).
					tooFar := false
					for i := sp + 1; i <= fpt-1; i++ {
						p := sc.Points[i].P
						t, _ := sb.ClosestPointTo(p, false)
						pp := sb.PointAt(t)
						if pp.Minus(p).Magnitude() > ChordTolMm()/2 {
							tooFar = true
							break
						}
					}

					if tooFar {
						// Deviates by too much, so try a shorter span
						fpt--
						continue
					}
					// Okay, so use this piece and break.
					sbl.Curves = append(sbl.Curves, sb)
					break
				}

				// And continue interpolating, starting wherever the curve
				// we just generated finishes.
				sp = fpt
			}
		} else if sel != nil {
			srf.makeTrimEdgesInto(sel, false, sc, stb)
		}
	}
}

// RemapFaces rewrites each surface's face to the group's copy
// copyNumber of it (C++ SShell::RemapFaces).
func (s *SShell) RemapFaces(group ShellGroup, remap int) {
	for i := range s.Surface {
		ss := &s.Surface[i]
		face := core.HEntity{V: uint32(ss.Face)}
		if face == core.NoEntity {
			continue
		}

		face = group.Remap(face, remap)
		ss.Face = FaceID(face.V)
	}
}

// RemapFaces rewrites each triangle's face to the group's copy
// copyNumber of it (C++ SMesh::RemapFaces).
func (m *SMesh) RemapFaces(group ShellGroup, remap int) {
	for i := range m.Triangles {
		tr := &m.Triangles[i]
		face := core.HEntity{V: uint32(tr.Meta.Face)}
		if face == core.NoEntity {
			continue
		}

		face = group.Remap(face, remap)
		tr.Meta.Face = FaceID(face.V)
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

// Clone is a copy of the shell that shares nothing with it, to the bit:
// the surfaces with their trims, the curves with their points. What is
// built from them for a Boolean or a triangulation (the trims' BSPs and
// edge lists) is left for the copy to build again.
func (s *SShell) Clone() SShell {
	out := SShell{BooleanFailed: s.BooleanFailed}
	out.Surface = make([]SSurface, len(s.Surface))
	for i := range s.Surface {
		ss := s.Surface[i]
		ss.Trim = append([]STrimBy(nil), ss.Trim...)
		ss.Bsp = nil
		ss.Edges = SEdgeList{}
		ss.curvedEdges = SEdgeList{}
		out.Surface[i] = ss
	}
	out.Curve = make([]SCurve, len(s.Curve))
	for i := range s.Curve {
		sc := s.Curve[i]
		sc.Points = append([]SCurvePt(nil), sc.Points...)
		out.Curve[i] = sc
	}
	return out
}
