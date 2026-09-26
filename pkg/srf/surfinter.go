package srf

import (
	"math"

	"github.com/spacesolve/spacesolve/pkg/core"
	"github.com/spacesolve/spacesolve/pkg/geom"
)

// IntersectAgainst computes the surface-surface intersection between
// this surface and `other`, appending the resulting Bezier curves to
// `into`. Mirrors SSurface::IntersectAgainst in
// solvespace/src/srf/surfinter.cpp.
//
// Dispatches to one of three specialized branches (plane-plane,
// plane-extrusion, extrusion-extrusion-same-axis) before falling
// back to the marching-squares approximation. The specialized
// branches produce exact intersection curves for the common
// boolean-CSG fixture cases; the marching-squares fallback is
// preserved for general NURBS intersections.
func (s *SSurface) IntersectAgainst(other *SSurface, agnstA, agnstB *SShell, into *SShell) {
	if other == nil || into == nil {
		return
	}
	// Bounding-box early exit.
	var amax, amin, bmax, bmin geom.Vector
	s.GetAxisAlignedBounding(&amax, &amin)
	other.GetAxisAlignedBounding(&bmax, &bmin)
	if amax.BoundingBoxesDisjoint(amax, amin, bmax, bmin) {
		return
	}

	var alongt, alongb geom.Vector
	var oft, ofb SBezier
	isExtdt := s.FillExtrusionAxis(&oft, &alongt)
	isExtdb := other.FillExtrusionAxis(&ofb, &alongb)

	// Branch 1: plane-plane intersection (DegM=1, DegN=1 on both).
	if s.DegM == 1 && s.DegN == 1 && other.DegM == 1 && other.DegN == 1 {
		s.intersectPlanePlane(other, into)
		return
	}
	// Branch 2: plane-extrusion (one is a plane, the other an extrusion).
	if (s.DegM == 1 && s.DegN == 1 && isExtdb) ||
		(other.DegM == 1 && other.DegN == 1 && isExtdt) {
		s.intersectPlaneExtrusion(other, agnstA, agnstB, into)
		return
	}
	// Branch 3: two extrusions along the same axis.
	if isExtdt && isExtdb &&
		math.Abs(alongt.Dot(alongb)) >=
			math.Sqrt(alongt.Magnitude()*alongb.Magnitude())-lengthEps {
		s.intersectExtrusionExtrusion(other, into, alongt)
		return
	}
	// Branch 4 (only one of the two surfaces is a plane; not the
	// other special cases): consult trim curves in the curved surface
	// for exact curves lying in the plane, and skip numerical
	// intersection if any are found.
	if (s.DegM == 1 && s.DegN == 1) || (other.DegM == 1 && other.DegN == 1) {
		var splane, sext *SSurface
		var shext *SShell
		if s.DegM == 1 && s.DegN == 1 {
			splane = s
			sext = other
			shext = agnstB
		} else {
			splane = other
			sext = s
			shext = agnstA
		}
		foundExact := false
		if shext != nil {
			for i := range shext.Curve {
				sc := &shext.Curve[i]
				if sc.Source == CurveSourceIntersection {
					continue
				}
				if !sc.IsExact {
					continue
				}
				if sc.SurfA != sext.H && sc.SurfB != sext.H {
					continue
				}
				if splane.ContainsPlaneCurve(sc) {
					bz := sc.Exact
					s.AddExactIntersectionCurve(&bz, other, agnstA, agnstB, into)
					foundExact = true
				}
			}
		}
		if foundExact {
			return
		}
	}
	// Branch 5: general marching-squares approximation.
	s.intersectMarchingSquares(other, agnstA, agnstB, into)
}

// intersectPlanePlane handles the case where both surfaces are
// planes. Mirrors the DegM=1, DegN=1 branch of C++
// SSurface::IntersectAgainst. The intersection is a line segment
// trimmed to the (u, v) range of both planes.
func (s *SSurface) intersectPlanePlane(other *SSurface, into *SShell) {
	na := s.NormalAt(0, 0).WithMagnitude(1)
	nb := other.NormalAt(0, 0).WithMagnitude(1)
	da := na.Dot(s.PointAt(0, 0))
	db := nb.Dot(other.PointAt(0, 0))
	dl := na.Cross(nb)
	if dl.Magnitude() < lengthEps {
		return
	}
	dl = dl.WithMagnitude(1)
	// p is the unique point on the line of intersection that also
	// lies in the plane through the origin perpendicular to dl.
	p := geom.AtIntersectionOfPlanes(na, da, nb, db, dl, 0)
	tmax := math.Inf(1)
	tmin := math.Inf(-1)
	for i := 0; i < 2; i++ {
		sr := s
		if i == 1 {
			sr = other
		}
		var tu, tv geom.Vector
		sr.TangentsAt(0, 0, &tu, &tv, false)
		var up, vp float64
		var ud, vd float64
		sr.ClosestPointToUV(p, &up, &vp, false)
		ud = dl.Dot(tu) / tu.MagSquared()
		vd = dl.Dot(tv) / tv.MagSquared()
		if ud > lengthEps {
			tmin = math.Max(tmin, -up/ud)
			tmax = math.Min(tmax, (1-up)/ud)
		} else if ud < -lengthEps {
			tmax = math.Min(tmax, -up/ud)
			tmin = math.Max(tmin, (1-up)/ud)
		} else {
			if up < -lengthEps || up > 1+lengthEps {
				tmax = math.Inf(-1)
			}
		}
		if vd > lengthEps {
			tmin = math.Max(tmin, -vp/vd)
			tmax = math.Min(tmax, (1-vp)/vd)
		} else if vd < -lengthEps {
			tmax = math.Min(tmax, -vp/vd)
			tmin = math.Max(tmin, (1-vp)/vd)
		} else {
			if vp < -lengthEps || vp > 1+lengthEps {
				tmax = math.Inf(-1)
			}
		}
	}
	if tmax > tmin+lengthEps {
		bz := SBezierFrom2(p.Plus(dl.ScaledBy(tmin)), p.Plus(dl.ScaledBy(tmax)))
		s.AddExactIntersectionCurve(&bz, other, nil, nil, into)
	}
}

// intersectPlaneExtrusion handles plane-vs-extrusion. If the
// extrusion axis is perpendicular to the plane, the intersection is
// the projection of the extrusion curve into the plane. If
// parallel, the intersection is a series of lines parallel to the
// axis at the points where a line normal to the extrusion crosses
// the curve.
func (s *SSurface) intersectPlaneExtrusion(other *SSurface, agnstA, agnstB *SShell, into *SShell) {
	var splane, sext *SSurface
	var shext *SShell
	if s.DegM == 1 && s.DegN == 1 {
		splane = s
		sext = other
		shext = agnstB
	} else {
		splane = other
		sext = s
		shext = agnstA
	}
	n := splane.NormalAt(0, 0).WithMagnitude(1)
	d := n.Dot(splane.PointAt(0, 0))
	var bezier SBezier
	var along geom.Vector
	if !sext.FillExtrusionAxis(&bezier, &along) {
		return
	}
	if math.Abs(n.Dot(along)) < lengthEps {
		// Direction of extrusion is parallel to plane; intersection is
		// zero or more lines parallel to the extrusion axis.
		pm := sext.Ctrl[0][0].Plus(sext.Ctrl[0][1]).ScaledBy(0.5)
		alu := along.WithMagnitude(1)
		dp := n.Cross(along).WithMagnitude(1)
		p0 := n.ScaledBy(d).Plus(alu.ScaledBy(pm.Dot(alu)))
		// Also add any trim curves of the extruded surface that lie in the plane.
		var exacts []SBezier
		if shext != nil {
			for i := range shext.Curve {
				sc := &shext.Curve[i]
				if sc.Source == CurveSourceIntersection {
					continue
				}
				if !sc.IsExact {
					continue
				}
				if sc.SurfA != sext.H && sc.SurfB != sext.H {
					continue
				}
				if splane.ContainsPlaneCurve(sc) {
					bz := sc.Exact
					s.AddExactIntersectionCurve(&bz, other, agnstA, agnstB, into)
					exacts = append(exacts, bz)
				}
			}
		}
		var inters []SInter
		sext.AllPointsIntersecting(p0, p0.Plus(dp), &inters, false, false, true)
		for _, si := range inters {
			duplicate := false
			for j := range exacts {
				t, _ := exacts[j].ClosestPointTo(si.P, false)
				if exacts[j].PointAt(t).DistanceTo(si.P) < lengthEps {
					duplicate = true
					break
				}
			}
			if duplicate {
				continue
			}
			al := along.ScaledBy(0.5)
			bz := SBezierFrom2(si.P.Minus(al), si.P.Plus(al))
			s.AddExactIntersectionCurve(&bz, other, agnstA, agnstB, into)
		}
	} else {
		// Direction of extrusion is not parallel to plane; intersection
		// is the projection of the extruded curve into our plane.
		for i := 0; i <= bezier.Degree; i++ {
			p0 := bezier.Ctrl[i]
			p1 := p0.Plus(along)
			bezier.Ctrl[i] = geom.AtIntersectionOfPlaneAndLine(n, d, p0, p1)
		}
		s.AddExactIntersectionCurve(&bezier, other, agnstA, agnstB, into)
	}
}

// intersectExtrusionExtrusion handles two surfaces extruded along
// the same axis. The intersection is zero or more line segments
// parallel to the axis, one per crossing of the projected
// cross-section curves.
func (s *SSurface) intersectExtrusionExtrusion(other *SSurface, into *SShell, alongt geom.Vector) {
	axis := alongt.WithMagnitude(1)
	var lv []geom.Vector
	var oft SBezier
	if !s.FillExtrusionAxis(&oft, &geom.Vector{}) {
		return
	}
	oft.MakePwlIntoPoints(&lv, defaultChordTolMm, defaultChordTolMm)
	a0 := s.Ctrl[0][0].Dot(axis)
	a1 := s.Ctrl[0][1].Dot(axis)
	b0 := other.Ctrl[0][0].Dot(axis)
	b1 := other.Ctrl[0][1].Dot(axis)
	if a0 > a1 {
		a0, a1 = a1, a0
	}
	if b0 > b1 {
		b0, b1 = b1, b0
	}
	ab0 := math.Max(a0, b0)
	ab1 := math.Min(a1, b1)
	if math.Abs(ab0-ab1) < lengthEps {
		return
	}
	axis0 := axis.ScaledBy(ab0)
	axis1 := axis.ScaledBy(ab1)
	axisc := axis0.Plus(axis1).ScaledBy(0.5)
	var inters []SInter
	for i := 0; i+1 < len(lv); i++ {
		pa := lv[i]
		pb := lv[i+1]
		pa = pa.Minus(axis.ScaledBy(pa.Dot(axis)))
		pb = pb.Minus(axis.ScaledBy(pb.Dot(axis)))
		pa = pa.Plus(axisc)
		pb = pb.Plus(axisc)
		other.AllPointsIntersecting(pa, pb, &inters, true, false, false)
	}
	for _, si := range inters {
		p := si.P.Minus(axis.ScaledBy(si.P.Dot(axis)))
		var ub, vb float64
		other.ClosestPointToUV(p, &ub, &vb, true)
		// Build an auxiliary plane perpendicular to the axis through p
		// to refine the intersection onto the three-surface curve.
		plane := FromPlane(p, axis.Cross(geom.Vector{Z: 1}).Normal(), axis.Cross(geom.Vector{X: 1}).Normal())
		other.PointOnSurfaces(s, &plane, &ub, &vb)
		p = other.PointAt(ub, vb)
		bz := SBezierFrom2(p.Plus(axis0), p.Plus(axis1))
		s.AddExactIntersectionCurve(&bz, other, nil, nil, into)
	}
}

// intersectMarchingSquares is the general-case fallback. It walks
// each surface's edges, finds intersections with the other
// surface, and then marches between them to chain up intersection
// curves.
func (s *SSurface) intersectMarchingSquares(other *SSurface, agnstA, agnstB *SShell, into *SShell) {
	const n = 16
	type sample struct{ u, v, d float64 }
	var gridA [n + 1][n + 1]sample
	for i := 0; i <= n; i++ {
		for j := 0; j <= n; j++ {
			u := float64(i) / float64(n)
			v := float64(j) / float64(n)
			pA := s.PointAt(u, v)
			var puv geom.Point2d
			other.ClosestPointTo(pA, &puv, false)
			pB2 := other.PointAt(puv.U, puv.V)
			gridA[i][j] = sample{u: u, v: v, d: pA.DistanceTo(pB2)}
		}
	}
	interp := func(s1, s2 sample) geom.Vector {
		t := s1.d / (s1.d - s2.d)
		if math.IsNaN(t) || t < 0 {
			t = 0
		}
		if t > 1 {
			t = 1
		}
		ua := s1.u + t*(s2.u-s1.u)
		va := s1.v + t*(s2.v-s1.v)
		return s.PointAt(ua, va)
	}
	var points []geom.Vector
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			s00 := gridA[i][j]
			s10 := gridA[i+1][j]
			s11 := gridA[i+1][j+1]
			s01 := gridA[i][j+1]
			if (s00.d <= 0) != (s10.d <= 0) {
				points = append(points, interp(s00, s10))
			}
			if (s10.d <= 0) != (s11.d <= 0) {
				points = append(points, interp(s10, s11))
			}
			if (s11.d <= 0) != (s01.d <= 0) {
				points = append(points, interp(s11, s01))
			}
			if (s01.d <= 0) != (s00.d <= 0) {
				points = append(points, interp(s01, s00))
			}
		}
	}
	for k := 0; k+1 < len(points); k += 2 {
		bz := SBezierFrom2(points[k], points[k+1])
		s.AddExactIntersectionCurve(&bz, other, agnstA, agnstB, into)
	}
}

// AddExactIntersectionCurve adds an exact Bezier intersection curve
// to `into`. Mirrors SSurface::AddExactIntersectionCurve in
// solvespace/src/srf/surfinter.cpp. Deduplicates against existing
// curves in `into` (matching by exact Bezier equality). When no
// existing curve matches, the polyline form is refined against the
// operand shells so the resulting polyline lies exactly on the
// three surfaces (s, srfB, and any surfaces in agnstA/B).
func (s *SSurface) AddExactIntersectionCurve(sb *SBezier, srfB *SSurface, agnstA, agnstB *SShell, into *SShell) {
	if sb == nil || into == nil {
		return
	}
	if sb.Start().Equals(sb.Finish()) {
		return
	}
	sc := SCurve{
		H:       s.nextCurveHandle(into),
		SurfA:   s.H,
		SurfB:   srfB.H,
		Exact:   *sb,
		IsExact: true,
	}
	// Deduplicate against existing curves.
	var existing *SCurve
	backwards := false
	for i := range into.Curve {
		oc := &into.Curve[i]
		if !oc.IsExact {
			continue
		}
		if oc.Exact.Equals(sb) {
			existing = oc
			break
		}
		rev := *sb
		rev.Reverse()
		if oc.Exact.Equals(&rev) {
			existing = oc
			backwards = true
			break
		}
	}
	var split SCurve
	if existing != nil {
		sc.Points = make([]SCurvePt, len(existing.Points))
		copy(sc.Points, existing.Points)
		if backwards {
			for i, j := 0, len(sc.Points)-1; i < j; i, j = i+1, j-1 {
				sc.Points[i], sc.Points[j] = sc.Points[j], sc.Points[i]
			}
		}
		split = sc
	} else {
		sc.IsExact = false
		pts := make([]geom.Vector, 0, 16)
		sb.MakePwlIntoPoints(&pts, defaultChordTolMm, defaultChordTolMm)
		for _, p := range pts {
			sc.Points = append(sc.Points, SCurvePt{P: p})
		}
		split = sc.makeCopySplitAgainstExact(agnstA, agnstB, s, srfB)
	}
	// Discard if entirely outside one surface.
	withinA, withinB := false, false
	for _, scpt := range split.Points {
		var puv geom.Point2d
		s.ClosestPointTo(scpt.P, &puv, false)
		if puv.U > -0.01 && puv.U < 1.01 && puv.V > -0.01 && puv.V < 1.01 {
			withinA = true
		}
		var puvB geom.Point2d
		srfB.ClosestPointTo(scpt.P, &puvB, false)
		if puvB.U > -0.01 && puvB.U < 1.01 && puvB.V > -0.01 && puvB.V < 1.01 {
			withinB = true
		}
		if withinA && withinB {
			break
		}
	}
	if !(withinA && withinB) {
		return
	}
	split.Source = CurveSourceIntersection
	split.H = s.nextCurveHandle(into)
	into.Curve = append(into.Curve, split)
}

// makeCopySplitAgainstExact is the local helper used by
// AddExactIntersectionCurve to refine a polyline curve against the
// operand shells. Mirrors the refinement loop in the C++
// SSurface::AddExactIntersectionCurve that calls MakeCopySplitAgainst
// after MakePwlInto.
func (sc *SCurve) makeCopySplitAgainstExact(agnstA, agnstB *SShell, s, srfB *SSurface) SCurve {
	if len(sc.Points) == 0 {
		return *sc
	}
	out := *sc
	out.Points = nil
	out.Points = append(out.Points, sc.Points[0])
	prev := sc.Points[0]
	for i := 1; i < len(sc.Points); i++ {
		cur := sc.Points[i]
		var il []SInter
		if agnstA != nil {
			agnstA.AllPointsIntersecting(prev.P, cur.P, &il, true, false, true)
		}
		if agnstB != nil {
			agnstB.AllPointsIntersecting(prev.P, cur.P, &il, true, false, true)
		}
		// Cull intersections on s or srfB (those are not real).
		for j := range il {
			if il[j].Srf == s || il[j].Srf == srfB {
				il[j].Tag = 1
			}
		}
		filtered := il[:0]
		for _, x := range il {
			if x.Tag != 1 {
				filtered = append(filtered, x)
			}
		}
		il = filtered
		// Refine each remaining intersection onto the curve's exact form.
		for j := range il {
			pi := &il[j]
			var puv geom.Point2d
			if pi.Srf != nil {
				pi.Srf.ClosestPointTo(pi.P, &puv, false)
				if sc.IsExact {
					pi.Srf.PointOnCurve(&sc.Exact, &puv.U, &puv.V)
				} else if s != nil && srfB != nil {
					// Skip refinement when either operand surface is
					// unknown — the caller may have not threaded them
					// through, in which case the polyline is still
					// approximately on both surfaces (marching-squares
					// approximation accuracy).
					pi.Srf.PointOnSurfaces(s, srfB, &puv.U, &puv.V)
				}
				pi.P = pi.Srf.PointAt(puv.U, puv.V)
			}
		}
		// Sort along the segment.
		for i := range il {
			for j := i + 1; j < len(il); j++ {
				di := il[i].P.Minus(prev.P).MagSquared()
				dj := il[j].P.Minus(prev.P).MagSquared()
				if di > dj {
					il[i], il[j] = il[j], il[i]
				}
			}
		}
		var lastP geom.Vector
		for _, pi := range il {
			if !pi.P.Equals(lastP) {
				out.Points = append(out.Points, SCurvePt{P: pi.P, Vertex: true})
				lastP = pi.P
			}
		}
		out.Points = append(out.Points, cur)
		prev = cur
	}
	return out
}

// nextCurveHandle allocates a fresh handle for a curve being added
// to `into`. Used by AddExactIntersectionCurve so the new curve's
// handle doesn't collide with existing entries.
func (s *SSurface) nextCurveHandle(into *SShell) core.HSCurve {
	if into == nil {
		return core.HSCurve{V: 1}
	}
	return core.HSCurve{V: uint32(len(into.Curve) + 1)}
}

// MakeCoincidentEdgesInto copies the trim polygon of every
// surface in this shell that is coincident with `proto` into `el`,
// in uv coordinates of `proto`. Mirrors
// SShell::MakeCoincidentEdgesInto in srf/surfinter.cpp. Used by
// CopySurfacesTrimAgainst when the operand shells share coincident
// faces (e.g. boolean_coplanar_union) so the coplanar faces are
// recognized as a single trim boundary.
func (s *SShell) MakeCoincidentEdgesInto(proto *SSurface, sameNormal bool, el *SEdgeList, useCurvesFrom *SShell) {
	if proto == nil || el == nil {
		return
	}
	for i := range s.Surface {
		ss := &s.Surface[i]
		if !proto.CoincidentWith(ss, sameNormal) {
			continue
		}
		ss.MakeEdgesInto(s, el, useCurvesFrom, true)
	}
	// Convert each edge from XYZ back to UV on the prototype.
	for i := range el.Edges {
		se := &el.Edges[i]
		var ua, va, ub, vb float64
		proto.ClosestPointToUV(se.A, &ua, &va, false)
		proto.ClosestPointToUV(se.B, &ub, &vb, false)
		if sameNormal {
			se.A = geom.Vector{X: ua, Y: va}
			se.B = geom.Vector{X: ub, Y: vb}
		} else {
			se.A = geom.Vector{X: ub, Y: vb}
			se.B = geom.Vector{X: ua, Y: va}
		}
	}
}
