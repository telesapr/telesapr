package srf

import (
	"math"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
)

// MakeFromRevolutionBetween revolves a planar BezierLoopSet about the
// axis (pt, axis) through the angles breaks, in increasing order and
// measured from the loop set's own plane: each curve sweeps one surface
// of revolution between each pair of neighbouring breaks, so the seams
// of the swept surfaces lie where the caller puts them, not at the
// quarter turns MakeFromRevolutionOf takes. A piece must span less than
// half a turn. Where the breaks span a whole turn the solid closes on
// itself; otherwise planes cap it at the first and last break.
//
// A cutter that a fillet's foot runs past (pkg/topo, a cone the face is
// cut back by) needs no seam of its curved face under the fillet: a
// seam there, where the fillet's section meets the face, left the
// Boolean's trims open.
func (s *SShell) MakeFromRevolutionBetween(sbls *SBezierLoopSet, pt, axis geom.Vector, color core.RGBA, group ShellGroup, breaks []float64) {
	if sbls == nil || len(breaks) < 2 {
		return
	}
	i0 := len(s.Surface)
	as := append([]float64(nil), breaks...)
	full := math.Abs(as[len(as)-1]-as[0]-2*math.Pi) < 1e-9
	if CheckNormalAxisRelationship(sbls, pt, axis, as[len(as)-1]-as[0], 0) {
		// Swept the other way round, as MakeFromHelicalRevolutionOf
		// does, so that the surfaces' normals point out of the solid.
		for i, j := 0, len(as)-1; i < j; i, j = i+1, j-1 {
			as[i], as[j] = as[j], as[i]
		}
	}
	sections := len(as) - 1

	var hs0, hs1 core.HSSurface
	if !full {
		// The caps, in a frame of the loop set's plane turned to the
		// first and last break.
		n := sbls.Normal.ScaledBy(-1)
		u := n.NormalDir(0)
		v := n.NormalDir(1)
		orig := sbls.Point
		umin, umax := math.Inf(1), math.Inf(-1)
		sbls.BoundingProjd(u, orig, &umin, &umax)
		vmin, vmax := math.Inf(1), math.Inf(-1)
		sbls.BoundingProjd(v, orig, &vmin, &vmax)
		orig = orig.Plus(u.ScaledBy(umin)).Plus(v.ScaledBy(vmin))
		u = u.ScaledBy(umax - umin)
		v = v.ScaledBy(vmax - vmin)
		a0, a1 := as[0], as[sections]
		s0 := FromPlane(orig.RotatedAbout(pt, axis, a0), u.RotatedAroundAxis(axis, a0), v.RotatedAroundAxis(axis, a0))
		s1 := FromPlane(orig.Plus(u).RotatedAbout(pt, axis, a1), u.ScaledBy(-1).RotatedAroundAxis(axis, a1), v.RotatedAroundAxis(axis, a1))
		for _, c := range []*SSurface{&s0, &s1} {
			c.Color = color
			c.TrimBuilt = true
		}
		s0.Face = FaceID(group.Remap(core.NoEntity, remapLatheStart).V)
		s1.Face = FaceID(group.Remap(core.NoEntity, remapLatheEnd).V)
		hs0 = s.addAndAssignSurfaceID(s0)
		hs1 = s.addAndAssignSurfaceID(s1)
	}

	for _, sbl := range sbls.loops() {
		nc := len(sbl.Curves)
		hsl := make([][]core.HSSurface, nc)
		for bi := range sbl.Curves {
			sb := &sbl.Curves[bi]
			revs := make([]core.HSSurface, sections)
			for j := range sections {
				if isOnAxisLine(sb, pt, axis) {
					continue
				}
				ss := FromRevolutionOf(sb, pt, axis, as[j], as[j+1], 0, 0)
				ss.Color = color
				ss.TrimBuilt = true
				if face, ok := lineToFace(group, sb); ok {
					ss.Face = face
				}
				revs[j] = s.addAndAssignSurfaceID(ss)
			}
			hsl[bi] = revs
		}
		for i := range nc {
			revs, revsp := hsl[i], hsl[(i-1+nc)%nc]
			sb := &sbl.Curves[i]
			last := sections
			if full {
				// The curve at the last break is the one at the first.
				last = sections - 1
			}
			for j := 0; j <= last; j++ {
				qs := geom.QuaternionFromAxisAngle(axis, as[j])
				ts := pt.Minus(qs.Rotate(pt))
				on := revs[0].V == 0
				switch {
				case !on:
					sc := SCurve{IsExact: true, Exact: transformedCopy(sb, ts, qs)}
					sc.Points = makePwlSCurvePoints(&sc.Exact, s.Tol, 0)
					switch {
					case j < sections:
						sc.SurfA = revs[j]
					default:
						sc.SurfA = hs1
					}
					switch {
					case j > 0:
						sc.SurfB = revs[j-1]
					case full:
						sc.SurfB = revs[sections-1]
					default:
						sc.SurfB = hs0
					}
					s.addCurveTrimming(sc, true)
				case j == 0 && !full:
					// A line on the axis, shared by the two caps.
					sc := SCurve{IsExact: true, Exact: transformedCopy(sb, ts, qs)}
					sc.Points = makePwlSCurvePoints(&sc.Exact, s.Tol, 0)
					sc.SurfA = hs1
					sc.SurfB = hs0
					s.addCurveTrimming(sc, true)
				}
				if j < sections && revs[j].V != 0 && revsp[j].V != 0 {
					ss := s.FindSurface(revs[j])
					sc := SCurve{IsExact: true, Exact: SBezierFrom3(ss.Ctrl[0][0], ss.Ctrl[0][1], ss.Ctrl[0][2])}
					sc.Exact.Weight[1] = ss.Weight[0][1]
					sc.Points = makePwlSCurvePoints(&sc.Exact, s.Tol, 0)
					sc.SurfA = revs[j]
					sc.SurfB = revsp[j]
					s.addCurveTrimming(sc, false)
				}
			}
		}
	}
	s.MakeFirstOrderRevolvedSurfaces(pt, axis, i0)
}
