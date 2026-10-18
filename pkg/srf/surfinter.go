package srf

import (
	"math"
	"slices"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
)

// This file ports solvespace/src/srf/surfinter.cpp: how to intersect
// two surfaces, to get some type of curve. This is either an exact
// special case (e.g., two planes to make a line), or a numerical thing.

// AddExactIntersectionCurve adds the exact curve sb, where s meets
// srfB, to into: its pwl follows an identical curve already in into, or
// else is split against the operand shells.
func (s *SSurface) AddExactIntersectionCurve(sb *SBezier, srfB *SSurface, agnstA, agnstB *SShell, into *SShell) {
	s.addExactIntersectionCurve(sb, srfB, agnstA, agnstB, &curveSink{shell: into})
}

func (s *SSurface) addExactIntersectionCurve(sb *SBezier, srfB *SSurface, agnstA, agnstB *SShell, into *curveSink) {
	// Important to keep the order of (surfA, surfB) consistent; when we later
	// rewrite the identifiers, we rewrite surfA from A and surfB from B.
	sc := SCurve{
		SurfA:   s.H,
		SurfB:   srfB.H,
		Exact:   *sb,
		IsExact: true,
	}

	// Now we have to piecewise linearize the curve. If there's already an
	// identical curve in the shell, then follow that pwl exactly, otherwise
	// calculate from scratch.
	var split SCurve
	existing := into.findExact(sb)
	ex := exactCurve{s: s, srfB: srfB, sb: *sb, agnstA: agnstA, agnstB: agnstB, match: existing}
	if existing.curve != nil {
		sc.Points = append([]SCurvePt(nil), existing.curve.Points...)
		if existing.backwards {
			for i, j := 0, len(sc.Points)-1; i < j; i, j = i+1, j-1 {
				sc.Points[i], sc.Points[j] = sc.Points[j], sc.Points[i]
			}
		}
		split = sc
	} else {
		sc.Points = makePwlSCurvePoints(sb, s.Tol, 0)
		// and split the line where it intersects our existing surfaces
		split = sc.MakeCopySplitAgainst(agnstA, agnstB, s, srfB)
	}

	// Test if the curve lies entirely outside one of the
	withinA, withinB := false, false
	for _, scpt := range split.Points {
		const tol = 0.01
		var puv geom.Point2d
		s.ClosestPointTo(scpt.P, &puv, true)
		if puv.U > -tol && puv.U < 1+tol && puv.V > -tol && puv.V < 1+tol {
			withinA = true
		}
		srfB.ClosestPointTo(scpt.P, &puv, true)
		if puv.U > -tol && puv.U < 1+tol && puv.V > -tol && puv.V < 1+tol {
			withinB = true
		}
		// Break out early, no sense wasting time if we already have the answer.
		if withinA && withinB {
			break
		}
	}
	if !withinA || !withinB {
		// Intersection curve lies entirely outside one of the surfaces, so
		// it's fake.
		into.putExact(nil, ex)
		return
	}

	if sb.Start().Equals(sb.Finish()) {
		// Only degenerate operands meet in a curve of no length, as a
		// sketch extruded along a direction in its own plane: the
		// Boolean fails, and the group says so, where a user's input
		// must not stop the program.
		into.fail()
		into.putExact(nil, ex)
		return
	}

	split.Source = CurveSourceIntersection
	into.putExact(&split, ex)
}

// IntersectAgainst adds the curves where s meets b to into.
func (s *SSurface) IntersectAgainst(b *SSurface, agnstA, agnstB *SShell, into *SShell) {
	s.intersectAgainst(b, agnstA, agnstB, &curveSink{shell: into})
}

func (s *SSurface) intersectAgainst(b *SSurface, agnstA, agnstB *SShell, into *curveSink) {
	var amax, amin, bmax, bmin geom.Vector
	s.AxisAlignedBounding(&amax, &amin)
	b.AxisAlignedBounding(&bmax, &bmin)

	if amax.BoundingBoxesDisjoint(amax, amin, bmax, bmin) {
		// They cannot possibly intersect, no curves to generate
		return
	}

	var alongt, alongb geom.Vector
	var oft, ofb SBezier
	isExtdt := s.IsExtrusion(&oft, &alongt)
	isExtdb := b.IsExtrusion(&ofb, &alongb)

	switch {
	case s.DegM == 1 && s.DegN == 1 && b.DegM == 1 && b.DegN == 1:
		s.intersectPlanes(b, agnstA, agnstB, into)

	case (s.DegM == 1 && s.DegN == 1 && isExtdb) ||
		(b.DegM == 1 && b.DegN == 1 && isExtdt):

		s.intersectPlaneAndExtrusion(b, agnstA, agnstB, into)

	case isExtdt && isExtdb &&
		math.Sqrt(math.Abs(alongt.Dot(alongb))) >
			math.Sqrt(alongt.Magnitude()*alongb.Magnitude())-lengthEps:

		s.intersectCoaxialExtrusions(b, &oft, alongt, agnstA, agnstB, into)

	default:
		if (s.DegM == 1 && s.DegN == 1) || (b.DegM == 1 && b.DegN == 1) {
			// we should only be here if just one surface is a plane because the
			// plane-plane case was already handled above. Need to check the other
			// nonplanar surface for trim curves that lie in the plane and are not
			// already trimming both surfaces. This happens when we cut a Lathe shell
			// on one of the seams for example.
			// This also seems necessary to merge some coincident surfaces.
			splane, sext, shext := b, s, agnstA
			if s.DegM == 1 && s.DegN == 1 { // this and other checks assume coplanar ctrl pts.
				splane, sext, shext = s, b, agnstB
			}
			foundExact := false
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
				// we have a curve belonging to the curved surface and not the plane.
				// does it lie completely in the plane?
				if splane.ContainsPlaneCurve(sc) {
					bezier := sc.Exact
					s.addExactIntersectionCurve(&bezier, b, agnstA, agnstB, into)
					foundExact = true
				}
			}
			// if we found at lest one of these we don't want to do the numerical
			// intersection as well. Sometimes it will also find the same curve but
			// with different PWLs and the polygon will fail to assemble.
			if foundExact {
				return
			}
		}

		s.intersectByMarching(b, agnstA, agnstB, into)
	}
}

// intersectPlanes is IntersectAgainst's line-line case: two planes meet
// in a line, or not at all.
func (s *SSurface) intersectPlanes(b *SSurface, agnstA, agnstB *SShell, into *curveSink) {
	na := s.NormalAt(0, 0).WithMagnitude(1)
	nb := b.NormalAt(0, 0).WithMagnitude(1)
	da := na.Dot(s.PointAt(0, 0))
	db := nb.Dot(b.PointAt(0, 0))

	dl := na.Cross(nb)
	if dl.Magnitude() < lengthEps {
		return // parallel planes
	}
	dl = dl.WithMagnitude(1)
	p := geom.AtIntersectionOfTwoPlanes(na, da, nb, db)

	// Trim it to the region 0 <= {u,v} <= 1 for each plane; not strictly
	// necessary, since line will be split and excess edges culled, but
	// this improves speed and robustness.
	tmax, tmin := veryPositive, veryNegative
	for i := range 2 {
		sr := s
		if i == 1 {
			sr = b
		}
		var tu, tv geom.Vector
		sr.TangentsAt(0, 0, &tu, &tv, true)

		var up, vp float64
		sr.ClosestPointToUV(p, &up, &vp, true)
		ud, vd := alongTangents(dl, tu, tv)

		// so u = up + t*ud
		//    v = vp + t*vd
		switch {
		case ud > lengthEps:
			tmin = geom.Max(tmin, -up/ud)
			tmax = geom.Min(tmax, (1-up)/ud)

		case ud < -lengthEps:
			tmax = geom.Min(tmax, -up/ud)
			tmin = geom.Max(tmin, (1-up)/ud)

		case up < -lengthEps || up > 1+lengthEps:
			// u is constant, and outside [0, 1]
			tmax = veryNegative
		}
		switch {
		case vd > lengthEps:
			tmin = geom.Max(tmin, -vp/vd)
			tmax = geom.Min(tmax, (1-vp)/vd)

		case vd < -lengthEps:
			tmax = geom.Min(tmax, -vp/vd)
			tmin = geom.Max(tmin, (1-vp)/vd)

		case vp < -lengthEps || vp > 1+lengthEps:
			// v is constant, and outside [0, 1]
			tmax = veryNegative
		}
	}

	if tmax > tmin+lengthEps {
		bezier := SBezierFrom2(p.Plus(dl.ScaledBy(tmin)), p.Plus(dl.ScaledBy(tmax)))
		s.addExactIntersectionCurve(&bezier, b, agnstA, agnstB, into)
	}
}

// alongTangents is how far u and v move per unit along d, a direction in
// the plane of the tangents tu and tv. The tangents of a sheared patch,
// as the sloping top of a wedge whose width leans, are not at right
// angles, so d's projection on each is not its component: a line along
// one edge of such a patch would seem to cross the other parameter and
// be clipped short. Tangents at right angles, to within what moves no
// clip, keep the plain projections, whose last bits the goldens record.
func alongTangents(d, tu, tv geom.Vector) (ud, vd float64) {
	uu, uv, vv := tu.MagSquared(), tu.Dot(tv), tv.MagSquared()
	du, dv := d.Dot(tu), d.Dot(tv)
	det := uu*vv - uv*uv
	if det <= 0 || math.Abs(uv) <= 1e-9*math.Sqrt(uu*vv) {
		return du / uu, dv / vv
	}
	return (du*vv - dv*uv) / det, (dv*uu - du*uv) / det
}

// intersectPlaneAndExtrusion is IntersectAgainst's case of a plane and
// a surface of extrusion.
func (s *SSurface) intersectPlaneAndExtrusion(b *SSurface, agnstA, agnstB *SShell, into *curveSink) {
	splane, sext, shext := b, s, agnstA
	if s.DegM == 1 && s.DegN == 1 {
		splane, sext, shext = s, b, agnstB
	}

	n := splane.NormalAt(0, 0).WithMagnitude(1)
	d := n.Dot(splane.PointAt(0, 0))
	var bezier SBezier
	var along geom.Vector
	sext.IsExtrusion(&bezier, &along)

	if math.Abs(n.Dot(along)) < lengthEps {
		// Direction of extrusion is parallel to plane; so intersection
		// is zero or more lines. Build a line within the plane, and
		// normal to the direction of extrusion, and intersect that line
		// against the surface; each intersection point corresponds to
		// a line.

		// a point halfway along the extrusion
		pm := sext.Ctrl[0][0].Plus(sext.Ctrl[0][1]).ScaledBy(0.5)
		alu := along.WithMagnitude(1)
		dp := n.Cross(along).WithMagnitude(1)
		// n, alu, and dp form an orthogonal csys; set n component to
		// place it on the plane, alu component to lie halfway along
		// extrusion, and dp component doesn't matter so zero
		p0 := n.ScaledBy(d).Plus(alu.ScaledBy(pm.Dot(alu)))

		// The extruded curve might be tangent to the plane, e.g. where
		// a fillet-like surface blends into the flat face it's tangent
		// to (issue #1291). The numerical line-surface intersection
		// below can't converge on such a grazing intersection; but the
		// tangency occurs where the extruded curve ends on the plane,
		// so the line of intersection is the trim curve that the
		// surface shares with its neighbour there, and that curve lies
		// entirely in the plane. Add any such exact curves directly,
		// like the exact-curve case in the general branch below.
		var exacts []SBezier
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
				s.addExactIntersectionCurve(&bz, b, agnstA, agnstB, into)
				exacts = append(exacts, bz)
			}
		}

		var inters []SInter
		sext.AllPointsIntersecting(p0, p0.Plus(dp), &inters, false, false, true)

		for _, si := range inters {
			// If this line was already added exactly above, don't add a
			// numerical (last-digits different) duplicate of it; the
			// duplicates would break the trim polygon assembly.
			duplicate := false
			for k := range exacts {
				t, _ := exacts[k].ClosestPointTo(si.P, false)
				if exacts[k].PointAt(t).Minus(si.P).Magnitude() < lengthEps {
					duplicate = true
					break
				}
			}
			if duplicate {
				continue
			}

			al := along.ScaledBy(0.5)
			bz := SBezierFrom2(si.P.Minus(al), si.P.Plus(al))
			s.addExactIntersectionCurve(&bz, b, agnstA, agnstB, into)
		}
	} else {
		// Direction of extrusion is not parallel to plane; so
		// intersection is projection of extruded curve into our plane.
		for i := 0; i <= bezier.Degree; i++ {
			p0 := bezier.Ctrl[i]
			p1 := p0.Plus(along)

			bezier.Ctrl[i] = geom.AtIntersectionOfPlaneAndLine(n, d, p0, p1)
		}

		s.addExactIntersectionCurve(&bezier, b, agnstA, agnstB, into)
	}
}

// intersectCoaxialExtrusions is IntersectAgainst's case of two
// surfaces of extrusion along the same axis: they might intersect along
// some number of lines parallel to the axis.
func (s *SSurface) intersectCoaxialExtrusions(b *SSurface, oft *SBezier, alongt geom.Vector, agnstA, agnstB *SShell, into *curveSink) {
	axis := alongt.WithMagnitude(1)

	var inters []SInter
	var lv []geom.Vector

	aAxis0 := s.Ctrl[0][0].Dot(axis)
	aAxis1 := s.Ctrl[0][1].Dot(axis)
	bAxis0 := b.Ctrl[0][0].Dot(axis)
	bAxis1 := b.Ctrl[0][1].Dot(axis)

	if aAxis0 > aAxis1 {
		aAxis0, aAxis1 = aAxis1, aAxis0
	}
	if bAxis0 > bAxis1 {
		bAxis0, bAxis1 = bAxis1, bAxis0
	}

	abAxis0 := geom.Max(aAxis0, bAxis0)
	abAxis1 := geom.Min(aAxis1, bAxis1)

	if math.Abs(abAxis0-abAxis1) < lengthEps {
		// The line would be zero-length
		return
	}

	axis0 := axis.ScaledBy(abAxis0)
	axis1 := axis.ScaledBy(abAxis1)
	axisc := axis0.Plus(axis1).ScaledBy(0.5)

	oft.MakePwlIntoPoints(&lv, s.Tol, 0)

	for i := range len(lv) - 1 {
		pa, pb := lv[i], lv[i+1]
		pa = pa.Minus(axis.ScaledBy(pa.Dot(axis)))
		pb = pb.Minus(axis.ScaledBy(pb.Dot(axis)))
		pa = pa.Plus(axisc)
		pb = pb.Plus(axisc)

		b.AllPointsIntersecting(pa, pb, &inters, true, false, false)
	}

	for _, si := range inters {
		p := si.P.Minus(axis.ScaledBy(si.P.Dot(axis)))
		var ub, vb float64
		b.ClosestPointToUV(p, &ub, &vb, true)
		plane := FromPlane(p, axis.NormalDir(0), axis.NormalDir(1))

		b.PointOnSurfaces(s, &plane, &ub, &vb)

		p = b.PointAt(ub, vb)

		bezier := SBezierFrom2(p.Plus(axis0), p.Plus(axis1))
		s.addExactIntersectionCurve(&bezier, b, agnstA, agnstB, into)
	}
}

// intersectByMarching is IntersectAgainst's general case: find where
// each surface's trim edges cross the other surface, then march along
// the intersection from each of those points.
func (s *SSurface) intersectByMarching(b *SSurface, agnstA, agnstB *SShell, into *curveSink) {
	// The curves marched from the boundaries, to tell the closed loops
	// inside the surfaces apart from them.
	var marched [][]SCurvePt

	// Try intersecting the surfaces numerically, by a marching algorithm.
	// First, we find all the intersections between a surface and the
	// boundary of the other surface.
	var spl SPointList
	for a := range 2 {
		shA, srfA, srfB := agnstA, s, b
		if a == 1 {
			shA, srfA, srfB = agnstB, b, s
		}

		var el SEdgeList
		srfA.MakeEdgesInto(shA, &el, false, nil)

		for k := range el.Edges {
			se := &el.Edges[k]
			var lsi []SInter

			srfB.AllPointsIntersecting(se.A, se.B, &lsi, true, true, false)
			if len(lsi) == 0 {
				continue
			}

			// Find the other surface that this curve trims.
			sc := shA.FindCurve(core.HSCurve{V: uint32(se.AuxA)})
			hother := sc.SurfA
			if sc.SurfA == srfA.H {
				hother = sc.SurfB
			}
			other := shA.FindSurface(hother)

			for _, si := range lsi {
				p := si.P
				var u, v float64
				srfB.ClosestPointToUV(p, &u, &v, true)
				if sc.IsExact {
					srfB.PointOnCurve(&sc.Exact, &u, &v)
				} else {
					srfB.PointOnSurfaces(srfA, other, &u, &v)
				}
				p = srfB.PointAt(u, v)
				if !spl.ContainsPoint(p) {
					sp := SPoint{P: p}
					// We also need the edge normal, so that we know in
					// which direction to march.
					srfA.ClosestPointToUV(p, &u, &v, true)
					n := srfA.NormalAt(u, v)
					sp.AuxV = n.Cross(se.B.Minus(se.A))
					sp.AuxV = sp.AuxV.WithMagnitude(1)

					spl.Points = append(spl.Points, sp)
				}
			}
		}
	}

	for len(spl.Points) >= 2 {
		sc := SCurve{
			SurfA:   s.H,
			SurfB:   b.H,
			IsExact: false,
			Source:  CurveSourceIntersection,
		}

		start := spl.Points[0].P
		startv := spl.Points[0].AuxV
		spl.Points = spl.Points[1:]
		for k := range spl.Points {
			spl.Points[k].Tag = 0
		}

		// Our chord tolerance is whatever the user specified
		maxtol := s.Tol.ChordTolMm()
		maxsteps := max(300, s.Tol.MaxSegments()*3)

		// The curve starts at our starting point.
		padd := SCurvePt{Vertex: true, P: start}
		sc.Points = append(sc.Points, padd)

		var npc geom.Vector
		fwd := false
		// Better to start with a too-small step, so that we don't miss
		// features of the curve entirely.
		step := maxtol
		for a := 0; a < maxsteps; a++ {
			var pa, pb geom.Point2d
			s.ClosestPointTo(start, &pa, true)
			b.ClosestPointTo(start, &pb, true)

			na := s.NormalAtUv(pa).WithMagnitude(1)
			nb := b.NormalAtUv(pb).WithMagnitude(1)

			if a == 0 {
				dp := nb.Cross(na)
				// We want to march in the more inward direction.
				fwd = dp.Dot(startv) < 0
			}

			dp := nb.Cross(na).WithMagnitude(1.0)
			if !fwd {
				dp = dp.ScaledBy(-1.0)
			}
			for range 20 {
				np := start.Plus(dp.ScaledBy(step))
				npc = s.ClosestPointOnThisAndSurface(b, np)
				tol := npc.Minus(np).Magnitude()

				if tol > maxtol*0.8 {
					step *= 0.90
				} else {
					step /= 0.90
				}

				if tol < maxtol && tol > maxtol/2 {
					// If we meet the chord tolerance test, and we're
					// not too fine, then we break out.
					break
				}
			}

			// The step ends at the first seed it reaches, with any seed
			// at the same place; one beyond is where a later curve
			// starts. Where the curve runs nearly straight along a
			// patch's edge, crossing it and back (a cone along the top
			// of a ring), one long step can pass several.
			near := -1
			for k := range spl.Points {
				sp := &spl.Points[k]
				if sp.P.OnLineSegment(start, npc, 2*s.Tol.ChordTolMm()) &&
					(near < 0 || sp.P.Minus(start).Magnitude() < spl.Points[near].P.Minus(start).Magnitude()) {
					near = k
				}
			}
			if near >= 0 {
				end := spl.Points[near].P
				for k := range spl.Points {
					sp := &spl.Points[k]
					if sp.P.OnLineSegment(start, npc, 2*s.Tol.ChordTolMm()) &&
						sp.P.Minus(end).Magnitude() < 2*s.Tol.ChordTolMm() {
						sp.Tag = 1
					}
				}
				a = maxsteps
				npc = end
			}

			padd.P = npc
			padd.Vertex = a == maxsteps
			sc.Points = append(sc.Points, padd)

			start = npc
		}

		if !marchOnBothPatches(s, b, sc.Points, maxtol) {
			// Two patches that touch only where an edge of each
			// crosses the other's, as a lathe's seam crosses a ring's
			// in the plane both lie in, give a seed there and no
			// curve: the march leaves one of them at once and follows
			// the surface it extends to, a copy of the curve its
			// neighbour makes (roadmap K3). The seed it ran into may
			// start a curve of its own.
			continue
		}
		kept := spl.Points[:0]
		for _, sp := range spl.Points {
			if sp.Tag == 0 {
				kept = append(kept, sp)
			}
		}
		spl.Points = kept

		// And now we split and insert the curve
		marched = append(marched, sc.Points)
		split := sc.MakeCopySplitAgainst(agnstA, agnstB, s, b)
		into.add(split)
	}

	s.intersectClosedLoops(b, agnstA, agnstB, into, marched)
}

// offPatch tells whether p lies off the patch srf by more than tol: its
// nearest point on the patch, which stops at the patch's edge, is that
// far away.
func offPatch(srf *SSurface, p geom.Vector, tol float64) bool {
	var uv geom.Point2d
	srf.ClosestPointTo(p, &uv, true)
	return srf.PointAtUv(uv).Minus(p).Magnitude() > tol
}

// onBothPatches tells whether any of pts lies on both patches a and b. A
// march may leave a patch and come back, or run past its edge to where
// the classification trims it; one that never lies on both is no curve
// of theirs.
func onBothPatches(a, b *SSurface, pts []SCurvePt, tol float64) bool {
	return slices.ContainsFunc(pts, func(p SCurvePt) bool { return !offPatch(a, p.P, tol) && !offPatch(b, p.P, tol) })
}

// marchOnBothPatches tells whether a march from a seed is a curve of
// patches a and b. Its ends are seeds, on the patches' edges, and where
// it has points between, one of those must lie on both: the march's
// points are on both surfaces to far below LengthEps where they are on
// the patches at all. Where the curve runs along the seam between two
// patches of one face, within the chord tolerance of it, from a seed at
// the seam's end to one further along it (a ring's patch corner on a
// pot, roadmap K3), the march from the patch beside the curve's own
// follows it too, off its patch by less than the chord tolerance; that
// copy is its neighbour's curve, and kept, both would trim the face.
func marchOnBothPatches(a, b *SSurface, pts []SCurvePt, tol float64) bool {
	if len(pts) > 2 {
		return onBothPatches(a, b, pts[1:len(pts)-1], geom.LengthEps)
	}
	return onBothPatches(a, b, pts[1:], tol)
}

// loopDepth is how many times intersectClosedLoops halves each surface
// in turn, so a piece is an eighth of its surface each way.
const loopDepth = 6

// intersectClosedLoops finds the curves where s meets b that cross no
// trim edge of either, such as the circle where a sphere bulges through
// the middle of a face, which the boundary-seeded marching cannot
// reach. It halves both surfaces in turn, keeping the pairs of pieces
// whose control hulls overlap, takes a seed from each pair left that is
// off the curves already found, projects it onto the intersection, and
// marches from it until it comes back to where it started.
func (s *SSurface) intersectClosedLoops(b *SSurface, agnstA, agnstB *SShell, into *curveSink,
	marched [][]SCurvePt) {
	maxtol := s.Tol.ChordTolMm()
	near := func(p geom.Vector, d float64) bool {
		for _, pts := range marched {
			for k := 1; k < len(pts); k++ {
				if distanceToSegment(p, pts[k-1].P, pts[k].P) < d {
					return true
				}
			}
		}
		return false
	}
	var tried []geom.Vector
	var visit func(pa, pb *SSurface, depth int)
	visit = func(pa, pb *SSurface, depth int) {
		var amax, amin, bmax, bmin geom.Vector
		pa.AxisAlignedBounding(&amax, &amin)
		pb.AxisAlignedBounding(&bmax, &bmin)
		if amax.BoundingBoxesDisjoint(amax, amin, bmax, bmin) {
			return
		}
		if depth < loopDepth {
			var a0, a1, b0, b1 SSurface
			pa.SplitInHalf(depth%2 == 0, &a0, &a1)
			pb.SplitInHalf(depth%2 == 0, &b0, &b1)
			for _, x := range []*SSurface{&a0, &a1} {
				for _, y := range []*SSurface{&b0, &b1} {
					visit(x, y, depth+1)
				}
			}
			return
		}
		// Where the boxes overlap; a curve already found within a box's
		// size of it is the one that brought them together.
		lo := geom.Vector{X: math.Max(amin.X, bmin.X), Y: math.Max(amin.Y, bmin.Y), Z: math.Max(amin.Z, bmin.Z)}
		hi := geom.Vector{X: math.Min(amax.X, bmax.X), Y: math.Min(amax.Y, bmax.Y), Z: math.Min(amax.Z, bmax.Z)}
		guess := lo.Plus(hi).ScaledBy(0.5)
		size := amax.Minus(amin).Magnitude() + bmax.Minus(bmin).Magnitude()
		if near(guess, size+2*maxtol) {
			return
		}
		// Neighbouring pairs project to the same point.
		for _, q := range tried {
			if q.Minus(guess).Magnitude() < size {
				return
			}
		}
		seed, ok := s.seedOnIntersection(b, guess)
		tried = append(tried, seed)
		if !ok || near(seed, 2*maxtol+lengthEps) {
			return
		}
		pts, ok := s.marchLoop(b, seed)
		if !ok {
			return
		}
		marched = append(marched, pts)
		// A trim runs from its start point to its finish, so a curve
		// that ends where it starts can't be one: the loop goes in as
		// pieces, as a circle goes in as arcs.
		for _, piece := range splitLoop(pts, loopPieces) {
			sc := SCurve{
				SurfA:   s.H,
				SurfB:   b.H,
				IsExact: false,
				Source:  CurveSourceIntersection,
				Points:  piece,
			}
			split := sc.MakeCopySplitAgainst(agnstA, agnstB, s, b)
			into.add(split)
		}
	}
	visit(s, b, 0)
}

// loopPieces is how many curves a closed loop is cut into.
const loopPieces = 4

// splitLoop cuts the closed polyline pts, whose last point is its
// first, into n pieces of about equal point count, each ending where
// the next starts.
func splitLoop(pts []SCurvePt, n int) [][]SCurvePt {
	segs := len(pts) - 1
	n = min(n, segs)
	var out [][]SCurvePt
	for k := range n {
		from, to := k*segs/n, (k+1)*segs/n
		piece := append([]SCurvePt(nil), pts[from:to+1]...)
		piece[0].Vertex = true
		piece[len(piece)-1].Vertex = true
		out = append(out, piece)
	}
	return out
}

// inTrim tells whether p lies on srf, inside its trim. A loop that
// crosses no trim edge lies inside the trims of both surfaces; a curve
// that leaves either is one the boundary-seeded marching follows.
func inTrim(srf *SSurface, p geom.Vector) bool {
	var uv geom.Point2d
	srf.ClosestPointTo(p, &uv, false)
	if srf.PointAtUv(uv).Minus(p).Magnitude() >= lengthEps {
		return false
	}
	return inTrimUv(srf, uv)
}

// inTrimUv tells whether uv lies within srf's parameter range and not
// outside its trim.
func inTrimUv(srf *SSurface, uv geom.Point2d) bool {
	const margin = 1e-6
	if uv.U < -margin || uv.U > 1+margin || uv.V < -margin || uv.V > 1+margin {
		return false
	}
	return srf.Bsp == nil || srf.Bsp.ClassifyPoint(uv, geom.Point2d{}, srf) != BspUvOutside
}

// seedOnIntersection projects p onto the curve where s meets b, and
// tells whether that converged to a point inside both trims.
func (s *SSurface) seedOnIntersection(b *SSurface, p geom.Vector) (geom.Vector, bool) {
	q := s.ClosestPointOnThisAndSurface(b, p)
	if math.IsNaN(q.X) || math.IsNaN(q.Y) || math.IsNaN(q.Z) {
		return q, false
	}
	return q, inTrim(s, q) && inTrim(b, q)
}

// marchLoop marches along the curve where s meets b from start, as
// intersectByMarching does, and returns its points if it comes back to
// start without leaving either trim.
func (s *SSurface) marchLoop(b *SSurface, start geom.Vector) ([]SCurvePt, bool) {
	maxtol := s.Tol.ChordTolMm()
	maxsteps := max(300, s.Tol.MaxSegments()*3)
	pts := []SCurvePt{{Vertex: true, P: start}}
	at := start
	step := maxtol
	travelled := 0.0
	for range maxsteps {
		var pa, pb geom.Point2d
		s.ClosestPointTo(at, &pa, true)
		b.ClosestPointTo(at, &pb, true)
		na := s.NormalAtUv(pa).WithMagnitude(1)
		nb := b.NormalAtUv(pb).WithMagnitude(1)
		dp := nb.Cross(na)
		if dp.Magnitude() < lengthEps {
			// The surfaces are tangent here: no curve to follow.
			return nil, false
		}
		dp = dp.WithMagnitude(1)
		var npc geom.Vector
		for range 20 {
			np := at.Plus(dp.ScaledBy(step))
			npc = s.ClosestPointOnThisAndSurface(b, np)
			tol := npc.Minus(np).Magnitude()
			if tol > maxtol*0.8 {
				step *= 0.90
			} else {
				step /= 0.90
			}
			if tol < maxtol && tol > maxtol/2 {
				break
			}
		}
		if !inTrim(s, npc) || !inTrim(b, npc) {
			return nil, false
		}
		// Back at the start, once well away from it.
		if travelled > 4*maxtol && start.OnLineSegment(at, npc, 2*maxtol) {
			pts = append(pts, SCurvePt{Vertex: true, P: start})
			return pts, len(pts) > 3
		}
		travelled += npc.Minus(at).Magnitude()
		pts = append(pts, SCurvePt{P: npc})
		at = npc
	}
	return nil, false
}

// MakeCoincidentEdgesInto copies into el the trim polygons of every
// surface in the shell that is coincident with proto (with the same or
// opposite normal, as asked), in proto's uv coordinates.
func (s *SShell) MakeCoincidentEdgesInto(proto *SSurface, sameNormal bool, el *SEdgeList, useCurvesFrom *SShell) {
	for i := range s.Surface {
		ss := &s.Surface[i]
		if proto.CoincidentWith(ss, sameNormal) {
			ss.MakeEdgesInto(s, el, false, useCurvesFrom)
		}
	}

	for i := range el.Edges {
		se := &el.Edges[i]
		var ua, va, ub, vb float64
		proto.ClosestPointToUV(se.A, &ua, &va, true)
		proto.ClosestPointToUV(se.B, &ub, &vb, true)

		if sameNormal {
			se.A = geom.Vector{X: ua, Y: va}
			se.B = geom.Vector{X: ub, Y: vb}
		} else {
			// Flip normal, so flip all edge directions
			se.B = geom.Vector{X: ua, Y: va}
			se.A = geom.Vector{X: ub, Y: vb}
		}
	}
}
