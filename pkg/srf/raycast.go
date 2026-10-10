package srf

import (
	"math"

	"github.com/telesapr/telesapr/pkg/geom"
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
	size := geom.Max(geom.Max((cm0.Minus(c00)).Magnitude(), (c0n.Minus(c00)).Magnitude()),
		(cmn.Minus(c00)).Magnitude())
	*f = interEdgeN.ScaledBy(-1).WithMagnitude(geom.Max(size/20, lengthEps*100))
	var fuv geom.Point2d
	srf.ClosestPointTo(p.Plus(*f), &fuv, false)
	dev := srf.PointAt(fuv.U, fuv.V).Minus(p).Dot(nu)
	*flat = math.Abs(dev) < geom.Max(lengthEps, 1e-3*f.Magnitude())
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
	var buf [16]int
	near := s.surfacesNear(ea, eb, true, buf[:])
	for _, i := range near {
		srf := &s.Surface[i]
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
		for i := range 2 {
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
			for i := range 2 {
				dev[i] = ProbeTangentFace(interSrf[i], p, interEdgeN[i], nu, &f[i], &flat[i])
			}
			dirs := [2]*Class{indir, outdir}
			en := [2]geom.Vector{edgeNIn, edgeNOut}
			for s := range 2 {
				best := -1
				for i := range 2 {
					if f[i].Dot(en[s]) <= 0 {
						continue
					}
					if best < 0 || math.Abs(dev[i]) < math.Abs(dev[best]) {
						best = i
					}
				}
				switch {
				case best >= 0 && flat[best]:
					if surfN.Dot(interSurfN[best]) > 0 {
						*dirs[s] = SurfCoincSame
					} else {
						*dirs[s] = SurfCoincOpp
					}

				case best >= 0:
					if dev[best]*surfN.Dot(interSurfN[best]) > 0 {
						*dirs[s] = SurfInside
					} else {
						*dirs[s] = SurfOutside
					}

				default:
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
		// Five cases of two intersections, which need towards0: for a
		// tangent cutter (boolean_tangent_crossing) it is the only
		// thing that tells the material side of the tangent face from
		// the void side; without it every chain on the cutter
		// classifies as SurfOutside and is discarded.
		dirs := [2]*Class{indir, outdir}
		switch {
		case math.Abs(dotp[0]) < DotProductTol && dotp[1] > DotProductTol:
			if edgeNOut.Dot(interEdgeN[0]) > 0 {
				*dirs[0] = towards0
				*dirs[1] = SurfOutside
			} else {
				*dirs[0] = SurfInside
				*dirs[1] = towards0
			}

		case math.Abs(dotp[0]) < DotProductTol && dotp[1] < -DotProductTol:
			if edgeNOut.Dot(interEdgeN[0]) > 0 {
				*dirs[0] = towards0
				*dirs[1] = SurfInside
			} else {
				*dirs[0] = SurfOutside
				*dirs[1] = towards0
			}

		case dotp[0] > DotProductTol && dotp[1] > DotProductTol:
			*indir = SurfInside
			*outdir = SurfOutside

		case dotp[0] < -DotProductTol && dotp[1] < -DotProductTol:
			*indir = SurfOutside
			*outdir = SurfInside

		default:
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
	// along the given edge. The faces along the knife divide the
	// directions perpendicular to the edge at p into sectors that are
	// alternately inside and outside the shell, so classify each side
	// of our edge against the face that lies angularly closest to it:
	// that face bounds the sector containing it, and its normal points
	// out of that sector iff the sector is outside the shell.
	if edgeInters >= 4 && edgeInters%2 == 0 {
		edgeD := eb.Minus(ea).WithMagnitude(1)
		en := [2]geom.Vector{edgeNIn, edgeNOut}
		var ep [2]geom.Vector
		var bestDot [2]float64
		var bestN [2]geom.Vector
		for i := range 2 {
			bestDot[i] = veryNegative
			ep[i] = en[i].Minus(edgeD.ScaledBy(en[i].Dot(edgeD))).WithMagnitude(1)
		}
		for _, i := range near {
			srf := &s.Surface[i]
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
					for k := range 2 {
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
		for i := range 2 {
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
	for _, i := range near {
		srf := &s.Surface[i]
		var puv geom.Point2d
		srf.ClosestPointTo(p, &puv, false)
		pp := srf.PointAt(puv.U, puv.V)
		if pp.Minus(p).Magnitude() > lengthEps {
			continue
		}
		c := BspUvOutside
		if srf.Bsp != nil {
			c = srf.Bsp.ClassifyPoint(puv, geom.Point2d{}, srf)
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

	// 4. Raycast fallback: the nearest surface along a ray tells which
	// side p is on. A hit is not trusted when it lies on a trim edge, or
	// within the chord tolerance of a curved trim's pwl, where the trim
	// BSP can put it on the wrong side of the true curve; then another
	// direction is tried. If none is clear, the ray whose nearest hit is
	// farthest from its surface's trim edges decides.
	c, _ := s.raycastClass(p)
	*indir, *outdir = c, c
	return true
}

// raycastTries is how many directions raycastClass tries.
const raycastTries = 16

// rayDirection is the i-th direction raycastClass tries: six
// axis-aligned ones first, then points spread evenly over the sphere.
func rayDirection(i int) geom.Vector {
	if i < 6 {
		return geom.Vector{X: randomTable[i], Y: randomTable[i+1], Z: randomTable[i+2]}
	}
	// A Fibonacci lattice, at about the length of the first ones.
	k, n := float64(i-6)+0.5, float64(raycastTries-6)
	z := 1 - 2*k/n
	r := math.Sqrt(1 - z*z)
	phi := k * math.Pi * (3 - math.Sqrt(5))
	return geom.Vector{X: r * math.Cos(phi), Y: r * math.Sin(phi), Z: z}.ScaledBy(10)
}

// raycastClass classifies p by the nearest surface along rays, and
// returns how many directions it tried.
func (s *SShell) raycastClass(p geom.Vector) (Class, int) {
	tol := s.Tol.ChordTolMm() + lengthEps
	best, bestClear := SurfOutside, -1.0
	var buf [16]int
	near := buf[:]
	for try := range raycastTries {
		ray := rayDirection(try)
		var l []SInter
		near = s.surfacesNear(p.Minus(ray), p.Plus(ray), false, near)
		for _, i := range near {
			s.Surface[i].allPointsIntersecting(p.Minus(ray), p.Plus(ray), &l, false, true, false, tol)
		}
		c := SurfOutside
		dmin := veryPositive
		var nearest *SInter
		for k := range l {
			si := &l[k]
			t := si.P.Minus(p).Dot(ray) / ray.MagSquared()
			if t*ray.Magnitude() < -lengthEps {
				continue
			}
			if d := si.P.Minus(p).Magnitude(); d < dmin {
				dmin = d
				nearest = si
				if si.SurfNormal.Dot(ray) > 0 {
					c = SurfInside
				} else {
					c = SurfOutside
				}
			}
		}
		// How far the hit is from where its surface's trim could be
		// wrong; a ray that meets nothing is clearance.
		clearance := veryPositive
		switch {
		case nearest == nil || !nearest.OnEdge:
		case nearest.nearTrim:
			clearance = distanceToEdges(nearest.P, &nearest.Srf.Edges)
		default:
			clearance = 0
		}
		if clearance >= tol {
			return c, try + 1
		}
		if clearance > bestClear {
			best, bestClear = c, clearance
		}
	}
	return best, raycastTries
}
