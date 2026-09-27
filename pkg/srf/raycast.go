package srf

import (
	"math"

	"github.com/spacesolve/spacesolve/pkg/geom"
)

// Class is the relationship between a curve/edge and a face of the shell.
// Used during boolean CSG to determine whether a curve lies inside,
// outside, or coincident with the shell's volume.
type Class int

const (
	// SurfInside: edge lies inside the shell (ray goes into material).
	SurfInside Class = iota
	// SurfOutside: edge lies outside the shell (ray leaves material).
	SurfOutside
	// SurfCoincSame: edge is coincident with shell surface, same normal.
	SurfCoincSame
	// SurfCoincOpp: edge is coincident with shell surface, opposite normal.
	SurfCoincOpp
)

// DotProductTol is the direction-cosine tolerance used to decide
// whether two normals are perpendicular (in shell classification).
const DotProductTol = 1e-5

// randomTable holds 8 deterministic "random" direction vectors for
// raycasting fallback. Used to pick a new direction when a ray
// happens to lie exactly on an edge of the shell.
var randomTable = [8]float64{1.278, 5.0103, 9.427, -2.331, 7.13, 2.954, 5.034, -4.777}

// ClassifyRegion decides whether an edge lies inside or outside the
// shell at a point of contact, based on the relationship between
// three normals: the edge's normal, the intersecting surface's
// normal, and the shell's face normal at that point.
func (s *SShell) ClassifyRegion(edgeN, interSurfN, edgeSurfN geom.Vector) Class {
	dot := interSurfN.DirectionCosineWith(edgeN)
	if math.Abs(dot) < DotProductTol {
		if interSurfN.Dot(edgeSurfN) > 0 {
			return SurfCoincSame
		}
		return SurfCoincOpp
	}
	if dot > 0 {
		return SurfOutside
	}
	return SurfInside
}

// ProbeTangentFace probes a face that meets the surface being
// classified tangentially at a point. Returns the signed deviation
// of the face from the surface's tangent plane, measured along nu.
// f points into the face perpendicular to the edge. flat reports
// whether the deviation is below numerical noise.
func ProbeTangentFace(srf *SSurface, p, interEdgeN, nu geom.Vector, f *geom.Vector, flat *bool) float64 {
	c00 := srf.Ctrl[0][0]
	cm0 := srf.Ctrl[srf.DegM][0]
	c0n := srf.Ctrl[0][srf.DegN]
	cmn := srf.Ctrl[srf.DegM][srf.DegN]
	size := math.Max(math.Max((cm0.Minus(c00)).Magnitude(), (c0n.Minus(c00)).Magnitude()),
		(cmn.Minus(c00)).Magnitude())
	*f = interEdgeN.ScaledBy(-1).WithMagnitude(math.Max(size/20, lengthEps*100))
	var fuv geom.Point2d
	srf.ClosestPointTo(p.Plus(*f), &fuv, false)
	dev := srf.PointAt(fuv.U, fuv.V).Minus(p).Dot(nu)
	*flat = math.Abs(dev) < math.Max(lengthEps, 1e-3*f.Magnitude())
	return dev
}

// ClassifyEdge decides whether an edge (ea, eb) of a shell lies
// inside, outside, or coincident with the shell, in two directions
// (indir / outdir). The point p is on the edge, and edge_n_in /
// edge_n_out are the edge's outward normals in the two directions.
//
// Returns true on success (which is essentially always; failures
// fall back to SURF_OUTSIDE).
func (s *SShell) ClassifyEdge(indir, outdir *Class, ea, eb, p, edgeNIn, edgeNOut, surfN geom.Vector) bool {
	// 1. Edge-on-edge check.
	edgeInters := 0
	var interSurfN [2]geom.Vector
	var interEdgeN [2]geom.Vector
	var interSrf [2]*SSurface
	for i := range s.Surface {
		srf := &s.Surface[i]
		if srf.LineEntirelyOutsideBbox(ea, eb, true) {
			continue
		}
		for j := range srf.Edges.Edges {
			se := &srf.Edges.Edges[j]
			if (ea.Equals(se.A) && eb.Equals(se.B)) ||
				(eb.Equals(se.A) && ea.Equals(se.B)) ||
				p.OnLineSegment(se.A, se.B, lengthEps) {
				if edgeInters < 2 {
					var pm geom.Point2d
					srf.ClosestPointTo(p, &pm, false)
					interSurfN[edgeInters] = srf.NormalAt(pm.U, pm.V)
					interEdgeN[edgeInters] = interSurfN[edgeInters].Cross(se.B.Minus(se.A))
					interSrf[edgeInters] = srf
				}
				edgeInters++
			}
		}
	}

	if edgeInters == 2 {
		// Two faces meet at the shell's edge. Classify each side.
		var dotp [2]float64
		for i := 0; i < 2; i++ {
			dotp[i] = edgeNOut.DirectionCosineWith(interSurfN[i])
		}
		// Put the more-aligned face first.
		if math.Abs(dotp[1]) < DotProductTol {
			dotp[0], dotp[1] = dotp[1], dotp[0]
			interSurfN[0], interSurfN[1] = interSurfN[1], interSurfN[0]
			interEdgeN[0], interEdgeN[1] = interEdgeN[1], interEdgeN[0]
			interSrf[0], interSrf[1] = interSrf[1], interSrf[0]
		}
		towards0 := SurfCoincOpp
		if surfN.Dot(interSurfN[0]) > 0 {
			towards0 = SurfCoincSame
		}
		// Face 0 tangent, face 1 not tangent: probe face 0.
		if math.Abs(dotp[0]) < DotProductTol && math.Abs(dotp[1]) >= DotProductTol {
			var f geom.Vector
			var flat bool
			dev := ProbeTangentFace(interSrf[0], p, interEdgeN[0], surfN.WithMagnitude(1), &f, &flat)
			if !flat {
				if dev*surfN.Dot(interSurfN[0]) > 0 {
					towards0 = SurfInside
				} else {
					towards0 = SurfOutside
				}
			}
		}
		// Both faces tangent: probe each.
		if math.Abs(dotp[0]) < DotProductTol && math.Abs(dotp[1]) < DotProductTol {
			var f [2]geom.Vector
			var dev [2]float64
			var flat [2]bool
			nu := surfN.WithMagnitude(1)
			for i := 0; i < 2; i++ {
				dev[i] = ProbeTangentFace(interSrf[i], p, interEdgeN[i], nu, &f[i], &flat[i])
			}
			dirs := [2]*Class{indir, outdir}
			en := [2]geom.Vector{edgeNIn, edgeNOut}
			for s := 0; s < 2; s++ {
				best := -1
				for i := 0; i < 2; i++ {
					if f[i].Dot(en[s]) <= 0 {
						continue
					}
					if best < 0 || math.Abs(dev[i]) < math.Abs(dev[best]) {
						best = i
					}
				}
				if best >= 0 && flat[best] {
					if surfN.Dot(interSurfN[best]) > 0 {
						*dirs[s] = SurfCoincSame
					} else {
						*dirs[s] = SurfCoincOpp
					}
				} else if best >= 0 {
					if dev[best]*surfN.Dot(interSurfN[best]) > 0 {
						*dirs[s] = SurfInside
					} else {
						*dirs[s] = SurfOutside
					}
				} else {
					nr := 0
					if math.Abs(dev[1]) < math.Abs(dev[0]) {
						nr = 1
					}
					material := (dev[1-nr]-dev[nr])*surfN.Dot(interSurfN[nr]) < 0
					if material {
						*dirs[s] = SurfOutside
					} else {
						*dirs[s] = SurfInside
					}
				}
			}
			return true
		}
		// 5 sub-branches the C++ has at the end of `edge_inters == 2`
		// (solvespace/src/srf/raycast.cpp:602–637). The Go port
		// historically collapsed these into one
		// `interSurfN[s]·en[s]` test and discarded `towards0`
		// — which the C++ uses here. For tangent-cutters like
		// `boolean_tangent_crossing` (issue #1291) the discarded
		// `towards0` is the only thing that distinguishes the
		// material-side from the void-side of the tangent face, so
		// the bug surfaces as every chain on the cutter
		// classifying as `SurfOutside` and getting discarded.
		dirs := [2]*Class{indir, outdir}
		if math.Abs(dotp[0]) < DotProductTol && dotp[1] > DotProductTol {
			if edgeNOut.Dot(interEdgeN[0]) > 0 {
				*dirs[0] = towards0
				*dirs[1] = SurfOutside
			} else {
				*dirs[0] = SurfInside
				*dirs[1] = towards0
			}
		} else if math.Abs(dotp[0]) < DotProductTol && dotp[1] < -DotProductTol {
			if edgeNOut.Dot(interEdgeN[0]) > 0 {
				*dirs[0] = towards0
				*dirs[1] = SurfInside
			} else {
				*dirs[0] = SurfOutside
				*dirs[1] = towards0
			}
		} else if dotp[0] > DotProductTol && dotp[1] > DotProductTol {
			*indir = SurfInside
			*outdir = SurfOutside
		} else if dotp[0] < -DotProductTol && dotp[1] < -DotProductTol {
			*indir = SurfOutside
			*outdir = SurfInside
		} else {
			// Our edge is tangent to the shell at the shell's
			// edge: the two faces meeting at the shell's edge
			// lie on the same side of our surface, so the regions
			// on both sides of our edge get the same class.
			// That's outside the shell if the shell's edge is
			// convex, and inside if it's concave (reflex).
			// inter_edge_n[i] points away from face i's material,
			// so the shell's edge is convex iff face 1's material
			// direction lies behind face 0's plane.
			var c Class
			if interEdgeN[1].Dot(interSurfN[0]) > 0 {
				c = SurfOutside
			} else {
				c = SurfInside
			}
			*indir = c
			*outdir = c
		}
		return true
	}

	// 2. Knife-edge: more than two faces of the shell meet edge-on-edge
	// along the given edge. The faces along the knife divide the directions
	// perpendicular to the edge at p into sectors that are alternately
	// inside and outside the shell, so classify each side of our edge
	// against the face that lies angularly closest to it: that face bounds
	// the sector containing it, and its normal points out of that sector
	// iff the sector is outside the shell. Mirrors the C++ branch at
	// srf/raycast.cpp:641 ("knife edge from issue #1091").
	if edgeInters >= 4 && edgeInters%2 == 0 {
		edgeD := eb.Minus(ea).WithMagnitude(1)
		en := [2]geom.Vector{edgeNIn, edgeNOut}
		var ep [2]geom.Vector
		var bestDot [2]float64
		var bestN [2]geom.Vector
		for i := 0; i < 2; i++ {
			bestDot[i] = math.Inf(-1)
			perpendicular := en[i].Minus(edgeD.ScaledBy(en[i].Dot(edgeD)))
			if perpendicular.Magnitude() < lengthEps {
				ep[i] = geom.Vector{}
			} else {
				ep[i] = perpendicular.WithMagnitude(1)
			}
		}
		for i := range s.Surface {
			srf := &s.Surface[i]
			if srf.LineEntirelyOutsideBbox(ea, eb, true) {
				continue
			}
			for j := range srf.Edges.Edges {
				se := &srf.Edges.Edges[j]
				if (ea.Equals(se.A) && eb.Equals(se.B)) ||
					(eb.Equals(se.A) && ea.Equals(se.B)) ||
					p.OnLineSegment(se.A, se.B, lengthEps) {
					var pm geom.Point2d
					srf.ClosestPointTo(p, &pm, false)
					n := srf.NormalAt(pm.U, pm.V)
					f := se.B.Minus(se.A).Cross(n)
					f = f.Minus(edgeD.ScaledBy(f.Dot(edgeD)))
					if f.Magnitude() < lengthEps {
						continue
					}
					f = f.WithMagnitude(1)
					for k := 0; k < 2; k++ {
						dot := ep[k].Dot(f)
						if dot > bestDot[k] {
							bestDot[k] = dot
							bestN[k] = n
						}
					}
				}
			}
		}
		dirs := [2]*Class{indir, outdir}
		for i := 0; i < 2; i++ {
			dotn := en[i].DirectionCosineWith(bestN[i])
			if bestDot[i] > 0 && math.Abs(dotn) < DotProductTol {
				if surfN.Dot(bestN[i]) > 0 {
					*dirs[i] = SurfCoincSame
				} else {
					*dirs[i] = SurfCoincOpp
				}
			} else {
				if dotn > 0 {
					*dirs[i] = SurfOutside
				} else {
					*dirs[i] = SurfInside
				}
			}
		}
		return true
	}

	// 3. Edge-on-surface check.
	for i := range s.Surface {
		srf := &s.Surface[i]
		if srf.LineEntirelyOutsideBbox(ea, eb, true) {
			continue
		}
		var puv geom.Point2d
		srf.ClosestPointTo(p, &puv, false)
		pp := srf.PointAt(puv.U, puv.V)
		if pp.Minus(p).Magnitude() > lengthEps {
			continue
		}
		var c BspUvClass
		if srf.Bsp != nil {
			c = srf.Bsp.ClassifyPoint(puv)
		} else {
			// srf.Bsp is nil only when srf.Trim is empty
			// (buildSurfaceClassifyingBsp in boolean.go). Every real
			// extrusion-produced surface now populates Trim (see
			// SShell.MakeFromExtrusionOf), so this default is a
			// defensive fallback for a genuinely trim-less surface —
			// matching C++'s intent that "no trim boundary of its
			// own" means "its entire (u, v) domain is its trim
			// region", i.e. BspUvInside rather than BspUvOutside.
			c = BspUvInside
		}
		if c == BspUvOutside {
			continue
		}
		var pin, pout geom.Point2d
		srf.ClosestPointTo(p.Plus(edgeNIn), &pin, false)
		srf.ClosestPointTo(p.Plus(edgeNOut), &pout, false)
		surfNIn := srf.NormalAt(pin.U, pin.V)
		surfNOut := srf.NormalAt(pout.U, pout.V)
		*indir = s.ClassifyRegion(edgeNIn, surfNIn, surfN)
		*outdir = s.ClassifyRegion(edgeNOut, surfNOut, surfN)
		return true
	}

	// 4. Raycast fallback. Pick a random direction and count intersections.
	cnt := 0
	for {
		ray := geom.Vector{
			X: randomTable[cnt],
			Y: randomTable[cnt+1],
			Z: randomTable[cnt+2],
		}
		var l []SInter
		s.AllPointsIntersecting(p.Minus(ray), p.Plus(ray), &l, false, true, false)
		*indir = SurfOutside
		*outdir = SurfOutside
		dmin := math.MaxFloat64
		onEdge := false
		for _, si := range l {
			t := si.P.Minus(p).Dot(ray) / ray.MagSquared()
			if t*ray.Magnitude() < -lengthEps {
				continue
			}
			d := si.P.Minus(p).Magnitude()
			if d < dmin {
				dmin = d
				if si.SurfNormal.Dot(ray) > 0 {
					*indir = SurfInside
					*outdir = SurfInside
				} else {
					*indir = SurfOutside
					*outdir = SurfOutside
				}
				onEdge = si.OnEdge
			}
		}
		if !onEdge {
			break
		}
		cnt++
		if cnt > 5 {
			break
		}
	}
	return true
}

// PointInShell tests whether a point p lies inside, outside, or
// coincident with the shell. Uses a single raycast through the
// point and reports based on the closest intersection.
func (s *SShell) PointInShell(p geom.Vector) Class {
	var indir, outdir Class
	s.ClassifyEdge(&indir, &outdir,
		p.Plus(geom.Vector{X: -1, Y: -1, Z: -1}),
		p.Plus(geom.Vector{X: 1, Y: 1, Z: 1}),
		p,
		geom.Vector{X: 1},
		geom.Vector{X: -1},
		geom.Vector{Z: 1})
	return indir
}
