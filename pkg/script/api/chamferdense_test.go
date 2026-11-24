package api

import (
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/srf"
)

// wrapPiece is a piece of a sketch curve wrapped on the tube of
// wrappedTube: its point in the sketch's (u, v) at length s in [lo, hi],
// and whether the chain goes on smoothly past each end, or turns a corner.
type wrapPiece struct {
	at     func(s float64) (u, v float64)
	lo, hi float64
	smooth [2]bool
}

// arcPiece is the arc of radius rad about (cu, cv) from angle a0 to a1,
// by its length.
func arcPiece(cu, cv, rad, a0, a1 float64) wrapPiece {
	sg := 1.0
	if a1 < a0 {
		sg = -1
	}
	return wrapPiece{at: func(s float64) (float64, float64) {
		a := a0 + sg*s/rad
		return cu + rad*math.Cos(a), cv + rad*math.Sin(a)
	}, lo: 0, hi: math.Abs(a1-a0) * rad}
}

// linePiece is the line from (u0, v0) to (u1, v1), by its length.
func linePiece(u0, v0, u1, v1 float64) wrapPiece {
	l := math.Hypot(u1-u0, v1-v0)
	return wrapPiece{at: func(s float64) (float64, float64) {
		return u0 + (u1-u0)*s/l, v0 + (v1-v0)*s/l
	}, lo: 0, hi: l}
}

// archPieces is the top edges of wrappedArch's band, by the sketch's
// lines and arcs: its legs going on smoothly into the arcs, which meet at
// the apexes, and the legs' ends.
func archPieces() []wrapPiece {
	const a, w, vs, base = 5.2, 1.0, 20.0, 10.0
	ri, ro := 2*a-w, 2*a
	ti := math.Atan2(math.Sqrt(ri*ri-a*a), a)
	top := vs + base
	ps := []wrapPiece{
		linePiece(a, base, a, top), linePiece(a-w, base, a-w, top),
		linePiece(-a+w, base, -a+w, top), linePiece(-a, base, -a, top),
		linePiece(a, base, a-w, base), linePiece(-a+w, base, -a, base),
		arcPiece(-a, top, ri, 0, ti), arcPiece(a, top, ri, math.Pi, math.Pi-ti),
		arcPiece(-a, top, ro, 0, math.Pi/3), arcPiece(a, top, ro, math.Pi, 2*math.Pi/3),
	}
	for i := range 4 {
		ps[i].smooth[1] = true
		ps[6+i].smooth[0] = true
	}
	return ps
}

// wrapAt is the point of the sketch's (u, v) carried onto the tube at
// radius rho: v along the axis, u round it at the tube's radius 7.3,
// from -z as wrappedTube's face is.
func wrapAt(u, v, rho float64) geom.Vector {
	const r0 = 7.3
	return geom.Vector{X: v, Y: rho * math.Sin(u/r0), Z: -rho * math.Cos(u/r0)}
}

// topEdges is the edges of g's solid at radius top from the tube's axis.
func topEdges(g Group, top float64) []SolidEdge {
	var edges []SolidEdge
	for _, e := range g.Solid().Edges() {
		on := true
		for _, p := range e.Points() {
			on = on && math.Abs(math.Hypot(p.Y, p.Z)-top) < 1e-6
		}
		if on {
			edges = append(edges, e)
		}
	}
	return edges
}

// denseChamfer is how far, at worst, the chamfer group g's patches lie
// from the chamfer of distance dist of a band embossed on the tube to
// the radius top, its top edges the pieces: each patch sampled on an n by
// n grid over its whole parameter square, edges and all, and each point
// measured against the chamfer's own line through it. The line is found
// from the faces' closed forms, the top a cylinder about the axis and the
// side the radial lines over the wrapped curve: it lies in the plane
// square to the edge, from the edge's point at dist on the top to that at
// dist on the side, as the chamfer is defined (topo's chamferSection).
// Points within margin of a piece's corners, where the chamfers mitre,
// are left out, and those past a piece's smooth end are its neighbour's;
// checked is how many were measured, of patches.
func denseChamfer(t *testing.T, d *Doc, g Group, pieces []wrapPiece, top, dist, margin float64, n int) (worst float64, where geom.Vector, checked, patches int) {
	t.Helper()
	var surfs []srf.SSurface
	d.read(func(sk *sketch.Sketch) {
		for _, s := range sk.Group(g.H).RunningShell.Surface {
			if s.Origin>>16 == 1<<14|g.H.V&0x3fff {
				surfs = append(surfs, s)
			}
		}
	})
	if len(surfs) == 0 {
		t.Fatal("the chamfer made no patches")
	}
	edge := func(p wrapPiece, s float64) geom.Vector {
		u, v := p.at(s)
		return wrapAt(u, v, top)
	}
	tangent := func(p wrapPiece, s float64) geom.Vector {
		const h = 1e-6
		return edge(p, s+h).Minus(edge(p, s-h)).WithMagnitude(1)
	}
	// nearest is the piece and length whose plane square to the edge
	// holds q, and how far q is from the edge there.
	nearest := func(q geom.Vector) (wrapPiece, float64, float64) {
		best, bp, bs := math.Inf(1), wrapPiece{}, 0.0
		for _, p := range pieces {
			steps := int(math.Ceil((p.hi - p.lo) / 0.05))
			for i := range steps + 1 {
				s := p.lo + (p.hi-p.lo)*float64(i)/float64(steps)
				if dd := q.Minus(edge(p, s)).Magnitude(); dd < best {
					best, bp, bs = dd, p, s
				}
			}
		}
		s := bs
		for range 50 {
			const h = 1e-6
			f := func(x float64) float64 { return q.Minus(edge(bp, x)).Dot(tangent(bp, x)) }
			df := (f(s+h) - f(s-h)) / (2 * h)
			if df == 0 {
				break
			}
			step := f(s) / df
			s -= step
			if math.Abs(step) < 1e-13 {
				break
			}
		}
		return bp, s, q.Minus(edge(bp, s)).Magnitude()
	}
	// line is the chamfer's line at s, from its point on the top, on the
	// side of the edge sg says, to its point on the side.
	line := func(p wrapPiece, s, sg float64) (geom.Vector, geom.Vector) {
		e, tg := edge(p, s), tangent(p, s)
		radial := geom.Vector{Y: e.Y, Z: e.Z}.WithMagnitude(1)
		across := tg.Cross(radial).WithMagnitude(sg)
		// On the top: e + dist(cos a across - sin a radial), a solved for
		// the point at the top's radius.
		onTop := func(a float64) geom.Vector {
			return e.Plus(across.ScaledBy(dist * math.Cos(a))).Minus(radial.ScaledBy(dist * math.Sin(a)))
		}
		a := 0.0
		for range 50 {
			const h = 1e-7
			f := func(x float64) float64 { q := onTop(x); return math.Hypot(q.Y, q.Z) - top }
			step := f(a) / ((f(a+h) - f(a-h)) / (2 * h))
			a -= step
			if math.Abs(step) < 1e-14 {
				break
			}
		}
		// On the side: the point of the radial lines over the curve, at
		// (psi, rho), in the plane square to the edge, dist from it.
		side := func(psi, rho float64) geom.Vector { u, v := p.at(psi); return wrapAt(u, v, rho) }
		res := func(psi, rho float64) (float64, float64) {
			q := side(psi, rho).Minus(e)
			return q.Dot(tg), q.Magnitude() - dist
		}
		psi, rho := s, top-dist
		for range 50 {
			const h = 1e-7
			f0, g0 := res(psi, rho)
			fa, ga := res(psi+h, rho)
			fb, gb := res(psi, rho+h)
			j11, j12, j21, j22 := (fa-f0)/h, (fb-f0)/h, (ga-g0)/h, (gb-g0)/h
			det := j11*j22 - j12*j21
			dp, dr := (f0*j22-g0*j12)/det, (j11*g0-j21*f0)/det
			psi, rho = psi-dp, rho-dr
			if math.Abs(dp)+math.Abs(dr) < 1e-14 {
				break
			}
		}
		return onTop(a), side(psi, rho)
	}
	for i := range surfs {
		sf := &surfs[i]
		for a := range n + 1 {
			for b := range n + 1 {
				q := sf.PointAt(float64(a)/float64(n), float64(b)/float64(n))
				p, s, off := nearest(q)
				lo, hi := p.lo, p.hi
				if !p.smooth[0] {
					lo += margin
				}
				if !p.smooth[1] {
					hi -= margin
				}
				if s < lo || s > hi || off > 2*dist {
					continue
				}
				// The chamfer runs on the band's side of the edge: of the
				// lines either side, the one nearer q, the other lying in
				// the air above the tube.
				dev := math.Inf(1)
				for _, sg := range []float64{1, -1} {
					f0, f1 := line(p, s, sg)
					dev = math.Min(dev, distanceToSegment(q, f0, f1))
				}
				checked++
				if dev > worst {
					worst, where = dev, q
				}
			}
		}
	}
	return worst, where, checked, len(surfs)
}

// distanceToSegment is how far q lies from the segment a b.
func distanceToSegment(q, a, b geom.Vector) float64 {
	ab := b.Minus(a)
	t := math.Max(0, math.Min(1, q.Minus(a).Dot(ab)/ab.Dot(ab)))
	return q.Minus(a.Plus(ab.ScaledBy(t))).Magnitude()
}

// checkDenseChamfer fails where denseChamfer finds the chamfer g off its
// closed form by more than the blend is fitted to, a tenth of the chord
// tolerance (topo's rollingBall.tol), anywhere: longer spans between
// sections must not hide a misfit between the points the blend checks.
func checkDenseChamfer(t *testing.T, d *Doc, g Group, pieces []wrapPiece, top, dist, margin float64) {
	t.Helper()
	worst, where, n, patches := denseChamfer(t, d, g, pieces, top, dist, margin, 16)
	var tol float64
	d.read(func(sk *sketch.Sketch) { tol = sk.Group(g.H).RunningShell.Tol.ChordTolMm() / 10 })
	t.Logf("%d patches, %d points, worst %.3g at %v, fitted to %.3g", patches, n, worst, where, tol)
	if n == 0 || worst > tol {
		t.Errorf("%d points checked, worst %.3g mm off the chamfer at %v, past the fit's %.3g",
			n, worst, where, tol)
	}
}

// Chamfers of 0.2 of a boss's top edges, a disc or half a disc of radius
// 3 wrapped on the tube and raised 0.4: the top a cylinder, the side the
// radial lines over the wrapped curve, so the edge bends both round the
// boss and round the tube. The disc's is one closed chain, the half
// disc's two, the arc and its chord, mitred at the corners. Every patch
// is sampled densely, edges and all, not only where the blend's fit is
// checked, against the chamfer's closed form, at the default chord
// tolerance and a fine one; the mitres, within a millimetre of the
// corners, are left out.
func TestChamferWrappedBossDense(t *testing.T) {
	t.Parallel()
	const h, dist, rad, uc, vc = 0.4, 0.2, 3.0, 5.0, 30.0
	top := 7.3 + h
	for _, c := range []struct {
		what  string
		whole bool
		chord float64
	}{{"disc", true, 0}, {"half disc", false, 0}, {"disc at 0.01%", true, 0.01}, {"half disc at 0.01%", false, 0.01}} {
		t.Run(c.what, func(t *testing.T) {
			if c.chord > 0 && testing.Short() {
				t.Skip("the boss's chamfer at a fine chord tolerance, a few seconds more")
			}
			t.Parallel()
			d := New()
			if c.chord > 0 {
				d.Edit.SetChordTolerance(c.chord)
			}
			face := wrappedTube(d)
			d.Features.SketchAt(d.Origin(), V(0, 1, 0), V(1, 0, 0), "")
			// Two half circles for the disc: a whole circle wrapped has no
			// vertex for the emboss's Boolean to start its seams at.
			ctr := d.Sketch.UV(uc, vc)
			a := d.Sketch.Arc(ctr, d.Sketch.UV(uc+rad, vc), d.Sketch.UV(uc-rad, vc))
			var b Entity
			if c.whole {
				b = d.Sketch.Arc(ctr, d.Sketch.UV(uc-rad, vc), d.Sketch.UV(uc+rad, vc))
			} else {
				b = d.Sketch.Line(d.Sketch.UV(uc-rad, vc), d.Sketch.UV(uc+rad, vc))
			}
			d.Constrain.Coincident(a.End(), b.Start())
			d.Constrain.Coincident(b.End(), a.Start())
			band := d.Features.Emboss(face, h, "")
			if p := band.Problem(); p != "" {
				t.Fatal(p)
			}
			edges := topEdges(band, top)
			if len(edges) == 0 {
				t.Fatal("the boss has no top edge")
			}
			pieces := []wrapPiece{arcPiece(uc, vc, rad, 0, math.Pi), linePiece(uc-rad, vc, uc+rad, vc)}
			margin := 1.0
			if c.whole {
				// One edge is the whole chain: the chamfer goes on round
				// the boss where the edges meet smoothly.
				edges = edges[:1]
				pieces = []wrapPiece{arcPiece(uc, vc, rad, -math.Pi, 3*math.Pi)}
				margin = 0
			}
			g := d.Features.ChamferSolid(edges, dist, "")
			if p := g.Problem(); p != "" {
				t.Fatal(p)
			}
			if r := g.Analyze().Check(); !r.Sound() {
				t.Errorf("%s", r.Summary())
			}
			checkDenseChamfer(t, d, g, pieces, top, dist, margin)
		})
	}
}
