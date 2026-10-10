package srf

import (
	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
)

// SShellClass records, during a Boolean,, for each side of a trim edge,
// how that side relates to the shell being unioned/subtracted.
type SShellClass uint32

const (
	ShellInside    SShellClass = 100
	ShellOutside   SShellClass = 200
	ShellCoincSame SShellClass = 300
	ShellCoincOpp  SShellClass = 400
)

// shellDotpTol is the dot-product tolerance used by SShell::ClassifyEdge
// (srf/raycast.cpp:16).
const shellDotpTol = 1e-5

// shellClassFromClass is the SShellClass of a Class (raycast.go); the
// two enums share their meaning and differ in history.
func shellClassFromClass(c Class) SShellClass {
	switch c {
	case SurfInside:
		return ShellInside
	case SurfOutside:
		return ShellOutside
	case SurfCoincSame:
		return ShellCoincSame
	case SurfCoincOpp:
		return ShellCoincOpp
	}
	return ShellOutside
}

// Boolean-CSG parity: keepRegion decides whether a half-plane region
// should be kept in the output shell for a given boolean op. The
// region is characterized by its classification against the other
// shell (shell) and against the original trim polygon (orig).
//
// Mirrors the static KeepRegion in solvespace/src/srf/boolean.cpp:274.
func keepRegion(t SurfaceCombineAs, opA bool, shell, orig SShellClass) bool {
	if orig != ShellInside {
		return false
	}
	switch t {
	case SurfaceUnion:
		if opA {
			return shell == ShellOutside
		}
		return shell == ShellOutside || shell == ShellCoincSame
	case SurfaceDiff:
		if opA {
			return shell == ShellOutside || shell == ShellCoincOpp
		}
		return shell == ShellInside
	case SurfaceInter:
		if opA {
			return shell == ShellInside
		}
		return shell == ShellInside || shell == ShellCoincSame
	}
	return false
}

// keepEdge decides whether a single trim edge should be kept, based on
// the classifications of the regions on each side. An edge is kept
// exactly when its in-direction is kept but its out-direction is not.
//
// Mirrors the static KeepEdge in solvespace/src/srf/boolean.cpp:310.
func keepEdge(t SurfaceCombineAs, opA bool, indirShell, outdirShell, indirOrig, outdirOrig SShellClass) bool {
	keepIn := keepRegion(t, opA, indirShell, indirOrig)
	keepOut := keepRegion(t, opA, outdirShell, outdirOrig)
	if keepIn && !keepOut {
		return true
	}
	return false
}

// tagByClassifiedEdge translates an SBspUv::Class on the original-trim
// BSP into the (indir, outdir) SShellClass pair to assume for that
// edge. Mirrors TagByClassifiedEdge in boolean.cpp:323.
func tagByClassifiedEdge(bspclass BspUvClass, indir, outdir *SShellClass) {
	switch bspclass {
	case BspUvInside:
		*indir = ShellInside
		*outdir = ShellInside
	case BspUvOutside:
		*indir = ShellOutside
		*outdir = ShellOutside
	case BspUvEdgeParallel:
		*indir = ShellInside
		*outdir = ShellOutside
	case BspUvEdgeAntipar:
		*indir = ShellOutside
		*outdir = ShellInside
	default:
		*indir = ShellOutside
		*outdir = ShellOutside
	}
}

// findChainAvoiding moves a chain of adjoining edges from src to dest,
// starting from src's first edge and never extending the chain past a
// point in avoid.
func findChainAvoiding(src, dest *SEdgeList, avoid *SPointList) {
	if len(src.Edges) == 0 {
		panic("Need at least one edge")
	}
	// Start with an arbitrary edge.
	dest.Edges = append(dest.Edges, src.Edges[0])
	src.lClearTags()
	src.Edges[0].Tag = 1

	for {
		added := false
		// The start and finish of the current edge chain
		s := dest.Edges[0].A
		f := dest.Edges[len(dest.Edges)-1].B

		// We can attach a new edge at the start or finish, as long as that
		// start or finish point isn't in the list of points to avoid.
		startOkay := !avoid.ContainsPoint(s)
		finishOkay := !avoid.ContainsPoint(f)

		// Now look for an unused edge that joins at the start or finish of
		// our chain (if permitted by the avoid list).
		for i := range src.Edges {
			se := &src.Edges[i]
			if se.Tag != 0 {
				continue
			}
			switch {
			case startOkay && s.Equals(se.B):
				dest.addToBeginning(*se)
				s = se.A
				se.Tag = 1
				startOkay = !avoid.ContainsPoint(s)

			case finishOkay && f.Equals(se.A):
				dest.Edges = append(dest.Edges, *se)
				f = se.B
				se.Tag = 1
				finishOkay = !avoid.ContainsPoint(f)

			default:
				continue
			}

			added = true
		}
		if !added {
			break
		}
	}

	src.removeTagged()
}

// addToBeginning doesn't insert: it moves the first element to the end
// and then overwrites the first with se. The chains FindChainAvoiding
// builds, and so the order of the trims Booleans make, depend on that,
// and the goldens record it.
func (el *SEdgeList) addToBeginning(se SEdge) {
	el.Edges = append(el.Edges, el.Edges[0])
	el.Edges[0] = se
}

// removeTagged drops the tagged edges, keeping the order of the rest.
func (el *SEdgeList) removeTagged() {
	out := el.Edges[:0]
	for _, se := range el.Edges {
		if se.Tag == 0 {
			out = append(out, se)
		}
	}
	el.Edges = out
}

// trimFromEdgeList rebuilds srf's trims from el, merging adjoining
// edges from the same curve (and direction) into one STrimBy with xyz
// start and finish points.
func (s *SSurface) trimFromEdgeList(el *SEdgeList, asUv bool) {
	for i := range el.Edges {
		el.Edges[i].Tag = 0
	}

	for {
		// Find an edge, any edge; we'll start from there.
		var se *SEdge
		for i := range el.Edges {
			if el.Edges[i].Tag == 0 {
				se = &el.Edges[i]
				break
			}
		}
		if se == nil {
			break
		}
		se.Tag = 1
		stb := STrimBy{
			StartP:    se.A,
			FinishP:   se.B,
			Curve:     core.HSCurve{V: uint32(se.AuxA)},
			Backwards: se.AuxB != 0,
		}

		// Find adjoining edges from the same curve; those should be
		// merged into a single trim.
		for {
			merged := false
			for i := range el.Edges {
				se := &el.Edges[i]
				if se.Tag != 0 {
					continue
				}
				if se.AuxA != int(stb.Curve.V) {
					continue
				}
				if (se.AuxB != 0) != stb.Backwards {
					continue
				}

				if se.A.Equals(stb.FinishP) {
					stb.FinishP = se.B
					se.Tag = 1
					merged = true
				} else if se.B.Equals(stb.StartP) {
					stb.StartP = se.A
					se.Tag = 1
					merged = true
				}
			}
			if !merged {
				break
			}
		}

		if asUv {
			stb.StartP = s.PointAt(stb.StartP.X, stb.StartP.Y)
			stb.FinishP = s.PointAt(stb.FinishP.X, stb.FinishP.Y)
		}

		// And add the merged trim, with xyz (not uv like the polygon) pts
		s.Trim = append(s.Trim, stb)
	}
}

// The STrimBy entries built here keep their parent-relative Curve ids:
// CopyCurvesSplitAgainst records each parent curve's NewH, and
// RewriteSurfaceHandlesForCurves rewrites the handles at the end of
// MakeFromBoolean. The trim from makeSurfaceEdgesIntoUv below is
// therefore built directly from the curve in into that the trim refers
// to (looked up by raw H value).

// edgeNormalsWithinSurface returns a point on the uv edge auv->buv
// (refined onto the curve auxA it approximates) and the edge's inner
// and outer normals within srf, in xyz; surfN gets srf's normal there.
func (s *SSurface) edgeNormalsWithinSurface(auv, buv geom.Point2d, shell, sha, shb *SShell,
	auxA int, surfN *geom.Vector) (pt, enin, enout geom.Vector) {
	// the midpoint of the edge
	muv := auv.Plus(buv).ScaledBy(0.5)

	pt = s.PointAtUv(muv)

	// If this edge just approximates a curve, then refine our midpoint so
	// so that it actually lies on that curve too. Otherwise stuff like
	// point-on-face tests will fail, since the point won't actually lie
	// on the other face.
	if sc := shell.FindCurve(core.HSCurve{V: uint32(auxA)}); sc != nil {
		if sc.IsExact && sc.Exact.Degree != 1 {
			t, _ := sc.Exact.ClosestPointTo(pt, false)
			pt = sc.Exact.PointAt(t)
		} else if !sc.IsExact {
			trimmedA := sc.SurfaceA(sha, shb)
			trimmedB := sc.SurfaceB(sha, shb)
			if trimmedA != nil && trimmedB != nil {
				pt = trimmedA.ClosestPointOnThisAndSurface(trimmedB, pt)
			}
		}
	}
	s.ClosestPointTo(pt, &muv, true)

	*surfN = s.NormalAt(muv.U, muv.V)

	// Compute the edge's inner normal in xyz space. Use the configured
	// chord tolerance as the probe distance, but never more than a
	// fraction of this edge's own length: the classification must probe
	// the surfaces close to the edge, and a coarse tolerance on a small
	// model would otherwise evaluate them far away, or even extrapolate
	// them outside their domain, and misclassify.
	ab := s.PointAtUv(auv).Minus(s.PointAtUv(buv))
	enxyz := ab.Cross(*surfN).WithMagnitude(geom.Min(s.Tol.ChordTolMm(), ab.Magnitude()/10))
	// And based on that, compute the edge's inner normal in uv space. This
	// vector is perpendicular to the edge in xyz, but not necessarily in uv.
	var tu, tv geom.Vector
	s.TangentsAt(muv.U, muv.V, &tu, &tv, true)
	n := tu.Cross(tv)
	// since tu and tv may not be orthogonal, use y in place of v, x in place of u.
	// |y| = |v|sin(theta) where theta is the angle between tu and tv.
	ty := n.Cross(tu).ScaledBy(1.0 / tu.MagSquared())
	tx := tv.Cross(n).ScaledBy(1.0 / tv.MagSquared())

	enuv := geom.Point2d{
		U: enxyz.Dot(tx) / tx.MagSquared(),
		V: enxyz.Dot(ty) / ty.MagSquared(),
	}

	// Compute the inner and outer normals of this edge (within the srf),
	// in xyz space. These are not necessarily antiparallel, if the
	// surface is curved.
	pin := s.PointAtUv(muv.Minus(enuv))
	pout := s.PointAtUv(muv.Plus(enuv))
	enin = pin.Minus(pt)
	enout = pout.Minus(pt)
	return
}

// classifyEdge returns the (indir, outdir) SShellClass pair for the
// edge ea→eb, where p is a representative midpoint on the surface
// and edge_n_in / edge_n_out are the half-plane normal probe
// directions. Mirrors SShell::ClassifyEdge in raycast.cpp:463.
//
// Result: returns true if classification succeeded and writes
// shell classification into *indir and *outdir.
func (s *SShell) classifyEdge(indir, outdir *SShellClass,
	ea, eb, p, edgeNIn, edgeNOut, surfN geom.Vector) {
	var inDir, outDir Class
	if !s.ClassifyEdge(&inDir, &outDir, ea, eb, p, edgeNIn, edgeNOut, surfN) {
		return
	}
	*indir = shellClassFromClass(inDir)
	*outdir = shellClassFromClass(outDir)
}

// makeCopyTrimAgainst is the per-surface counterpart of
// SShell::MakeFromBoolean. For a single surface from `parent`, it
// produces a trimmed copy whose boundary includes both the parent's
// (split) trim curves and the new intersection curves computed
// against `agnst`. The resulting surface is appended to `into`,
// along with its STrimBy list. Each region of the surface is then
// classified against `agnst`; only the regions appropriate for the
// boolean op (UNION / DIFFERENCE / INTERSECTION) are kept.
//
// It handles the planar, coplanar and cylindrical cases; edge-on-edge
// and tangent faces are not fully handled.
//
// The copy is written to ret, which the caller owns: a surface is too
// large to hand back by value in this loop.
func (s *SSurface) makeCopyTrimAgainst(parent, sha, shb, into *SShell,
	t SurfaceCombineAs, ret *SSurface) (failed bool) {
	*ret = *s
	// The copy is this call's own, so it can use its closest-point cache
	// whatever goes for the operands' surfaces.
	ret.noCache = false
	ret.guess = nil
	if parent == nil || sha == nil || shb == nil || into == nil {
		return false
	}
	opA := parent == sha
	var agnst *SShell
	if opA {
		agnst = shb
	} else {
		agnst = sha
	}
	ret.Trim = nil
	ret.TrimBuilt = true

	// Step 1: rewrite trim curves to use the new (split) curve handles.
	// CopyCurvesSplitAgainst stored the parent-relative curve's NewH
	// (the handle in `into` of the first emitted segment). Look up each
	// trim curve in the parent shell and use that NewH as the rewritten
	// handle.
	for i := range s.Trim {
		stb := s.Trim[i]
		if pc := parent.FindCurve(stb.Curve); pc != nil {
			stb.Curve = pc.NewH
		}
		ret.Trim = append(ret.Trim, stb)
	}

	// Step 2: the second operand of DIFFERENCE gets reversed.
	if t == SurfaceDiff && !opA {
		ret.Reverse()
	}

	// Step 3: build the orig trim polygon from the new (split) curves.
	orig := &SEdgeList{}
	ret.MakeEdgesInto(into, orig, true, nil)
	ret.Trim = nil
	origBsp := SBspUvFrom(orig, ret)

	// Step 4: build the inter edge list from the intersection curves in
	// `into`.
	inter := &SEdgeList{}
	for i := range into.Curve {
		ic := &into.Curve[i]
		if ic.Source != CurveSourceIntersection {
			continue
		}
		var ss *SSurface
		if opA {
			if ic.SurfA != s.H {
				continue
			}
			ss = shb.FindSurface(ic.SurfB)
		} else {
			if ic.SurfB != s.H {
				continue
			}
			ss = sha.FindSurface(ic.SurfA)
		}
		if ss == nil {
			continue
		}
		for k := 1; k < len(ic.Points); k++ {
			a := ic.Points[k-1].P
			b := ic.Points[k].P

			var auv, buv geom.Point2d
			ss.ClosestPointTo(a, &auv, true)
			ss.ClosestPointTo(b, &buv, true)

			c := BspUvOutside
			if ss.Bsp != nil {
				c = ss.Bsp.ClassifyEdge(auv, buv, ss)
			}
			if c == BspUvOutside {
				continue
			}
			var tap, tbp geom.Point2d
			ret.ClosestPointTo(a, &tap, true)
			ret.ClosestPointTo(b, &tbp, true)
			ta := geom.Vector{X: tap.U, Y: tap.V}
			tb := geom.Vector{X: tbp.U, Y: tbp.V}

			tn := ret.NormalAt(tap.U, tap.V)
			sn := ss.NormalAt(auv.U, auv.V)

			// We are subtracting the portion of our surface that
			// lies in the shell, so the in-plane edge normal should
			// point opposite to the surface normal.
			tnxd := tn.Cross(b.Minus(a))
			dot := tnxd.Dot(sn)
			if mathAbs(dot) < shellDotpTol*tnxd.Magnitude()*sn.Magnitude() {
				// The surfaces are tangent along this curve, so their
				// normals give no orientation for it. Add the edge in
				// both directions; the classification against the
				// shells keeps the correctly oriented one and
				// discards the other.
				inter.AddEdge(ta, tb, int(ic.H.V), 0, 0)
				inter.AddEdge(tb, ta, int(ic.H.V), 1, 0)
				continue
			}
			bkwds := true
			if dot < 0 {
				bkwds = !bkwds
			}
			if (t == SurfaceDiff && !opA) || t == SurfaceInter {
				// Invert all newly created edges for intersection
				bkwds = !bkwds
			}
			if bkwds {
				inter.AddEdge(tb, ta, int(ic.H.V), 1, 0)
			} else {
				inter.AddEdge(ta, tb, int(ic.H.V), 0, 0)
			}
		}
	}

	// Step 5: the points where other than two edges meet, which chains
	// mustn't run through.
	var choosing SPointList
	for i := range orig.Edges {
		choosing.IncrementTagFor(orig.Edges[i].A)
		choosing.IncrementTagFor(orig.Edges[i].B)
	}
	for i := range inter.Edges {
		choosing.IncrementTagFor(inter.Edges[i].A)
		choosing.IncrementTagFor(inter.Edges[i].B)
	}
	choosing.removeTagged(func(sp *SPoint) bool { return sp.Tag == 2 })

	// Step 6: walk orig chains, classify, keep.
	final := &SEdgeList{}
	for len(orig.Edges) > 0 {
		chain := &SEdgeList{}
		findChainAvoiding(orig, chain, &choosing)

		se := &chain.Edges[len(chain.Edges)/2]
		auv := geom.Point2d{U: se.A.X, V: se.A.Y}
		buv := geom.Point2d{U: se.B.X, V: se.B.Y}
		var surfN, pt, enin, enout geom.Vector
		pt, enin, enout = ret.edgeNormalsWithinSurface(auv, buv, into, sha, shb, se.AuxA, &surfN)
		indirShell, outdirShell := ShellOutside, ShellOutside
		agnst.classifyEdge(&indirShell, &outdirShell,
			ret.PointAtUv(auv), ret.PointAtUv(buv),
			pt, enin, enout, surfN)
		keep := keepEdge(t, opA,
			indirShell, outdirShell,
			ShellInside, ShellOutside)
		if keep {
			for _, e := range chain.Edges {
				final.AddEdge(e.A, e.B, e.AuxA, e.AuxB, 0)
			}
		}
		chain.Edges = nil
	}

	// Step 7: walk inter chains, classify, keep.
	for len(inter.Edges) > 0 {
		chain := &SEdgeList{}
		findChainAvoiding(inter, chain, &choosing)

		se := &chain.Edges[len(chain.Edges)/2]
		auv := geom.Point2d{U: se.A.X, V: se.A.Y}
		buv := geom.Point2d{U: se.B.X, V: se.B.Y}
		var surfN, pt, enin, enout geom.Vector
		pt, enin, enout = ret.edgeNormalsWithinSurface(auv, buv, into, sha, shb, se.AuxA, &surfN)
		indirShell, outdirShell := ShellOutside, ShellOutside
		indirOrig, outdirOrig := ShellInside, ShellOutside
		cThis := BspUvOutside
		if origBsp != nil {
			cThis = origBsp.ClassifyEdge(auv, buv, ret)
		}
		if cThis == BspUvEdgeParallel {
			chain.Edges = nil
			continue
		}
		tagByClassifiedEdge(cThis, &indirOrig, &outdirOrig)
		agnst.classifyEdge(&indirShell, &outdirShell,
			ret.PointAtUv(auv), ret.PointAtUv(buv),
			pt, enin, enout, surfN)
		keep := keepEdge(t, opA,
			indirShell, outdirShell,
			indirOrig, outdirOrig)
		if keep {
			for _, e := range chain.Edges {
				final.AddEdge(e.A, e.B, e.AuxA, e.AuxB, 0)
			}
		}
		chain.Edges = nil
	}

	// Step 8: cull duplicate / anti-parallel edges.
	final.CullExtraneousEdges(true)

	// Step 9: assemble the trim polygon from the kept edges; if it
	// fails to close, the caller marks booleanFailed in `into`.
	ret.trimFromEdgeList(final, true)
	var poly SPolygon
	final.lClearTags()
	failed = !final.AssemblePolygon(&poly, nil, true)

	return failed
}

// lClearTags is a small helper to reset the Tag field on every edge
// in el (used after the inter/orig chain extraction above).
func (el *SEdgeList) lClearTags() {
	for i := range el.Edges {
		el.Edges[i].Tag = 0
	}
}
