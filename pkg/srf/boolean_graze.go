package srf

import (
	"math"

	"github.com/telesapr/telesapr/pkg/geom"
)

// grazeMaxSin is the sine of the widest angle between two curves leaving
// a vertex along one surface at which refineGrazingMeets looks at them.
const grazeMaxSin = 0.5

// grazeSagShare is how much of the two curves' distance apart at a
// segment's middle the segment's sag off their surface may be, beside a
// grazing meet.
const grazeSagShare = 0.5

// grazeSagFloor is the share of the chord tolerance, as its inverse,
// below which a segment's sag is left alone beside a grazing meet.
const grazeSagFloor = 8

// grazeRounds is how many times refineGrazingMeets halves a segment
// beside a vertex at most.
const grazeRounds = 6

// refineGrazingMeets puts points into the Boolean's curves beside a
// vertex where two curves on one surface leave it nearly along each
// other, until each segment there sags off that surface by less than half
// of how far the two are apart at its middle, or by less than an eighth
// of the chord tolerance. Where a thread's ridge
// crosses the circle where its bore turns into the chamfer's cone, the
// ridge's foot leaves the circle at the helix's angle, 2° on a 1/2 BSF
// nut; each is chorded to the chord tolerance on its own, and at 0.5%
// the circle's chord sagged into the hole by twice as much as the curves
// were apart, through the ridge's flank beside them: crossing mesh edges
// that no meshing of the faces mends, since they are the trims' own
// segments. With rounded crests the cone's curves across a crest and the
// flank it runs into smoothly leave their vertex along each other, and
// sagged through the flank the same way. A curve's segments elsewhere
// stay as they are, and so do segments whose sag lies along the surface,
// as a cubic's beside the line it leaves tangentially in a plane.
func (s *SShell) refineGrazingMeets(a, b *SShell) {
	tol := s.Tol.ChordTolMm()
	if tol <= geom.LengthEps {
		return
	}
	// The projections below leave the operands' closest-point caches as
	// they were: the trims are found from them next, and a start moved
	// moves their points in the last digits, where nothing was refined.
	var cached []geom.Point2d
	for _, sh := range []*SShell{a, b} {
		for i := range sh.Surface {
			cached = append(cached, sh.Surface[i].Cached)
		}
	}
	defer func() {
		k := 0
		for _, sh := range []*SShell{a, b} {
			for i := range sh.Surface {
				// Written only if changed: where Booleans run side by
				// side, their caches are off and unchanged.
				if sh.Surface[i].Cached != cached[k] {
					sh.Surface[i].Cached = cached[k]
				}
				k++
			}
		}
	}()
	for range grazeRounds {
		if !s.refineGrazingOnce(a, b, tol) {
			return
		}
	}
}

// grazeEnd is a segment of a curve leaving one of its vertices: from
// Points[at] to Points[next].
type grazeEnd struct {
	curve, at, next int
}

// refineGrazingOnce halves each segment beside a grazing meet that sags
// too far, and says whether it halved any.
func (s *SShell) refineGrazingOnce(a, b *SShell, tol float64) bool {
	type cell struct{ x, y, z int64 }
	size := 10 * splitEndTol
	cellOf := func(p geom.Vector) cell {
		return cell{int64(math.Floor(p.X / size)), int64(math.Floor(p.Y / size)), int64(math.Floor(p.Z / size))}
	}
	grid := map[cell][]grazeEnd{}
	for i := range s.Curve {
		pts := s.Curve[i].Points
		for j, pt := range pts {
			if !pt.Vertex && j != 0 && j != len(pts)-1 {
				continue
			}
			c := cellOf(pt.P)
			if j > 0 {
				grid[c] = append(grid[c], grazeEnd{i, j, j - 1})
			}
			if j < len(pts)-1 {
				grid[c] = append(grid[c], grazeEnd{i, j, j + 1})
			}
		}
	}
	// halve[curve][segment's lower index] marks the segments to halve.
	halve := map[int]map[int]bool{}
	for c, ends := range grid {
		for _, e := range ends {
			p := s.Curve[e.curve].Points[e.at].P
			for dx := int64(-1); dx <= 1; dx++ {
				for dy := int64(-1); dy <= 1; dy++ {
					for dz := int64(-1); dz <= 1; dz++ {
						for _, f := range grid[cell{c.x + dx, c.y + dy, c.z + dz}] {
							if f.curve == e.curve || !s.Curve[f.curve].Points[f.at].P.EqualsTol(p, splitEndTol) {
								continue
							}
							if s.grazeSagsTooFar(a, b, e, f, tol) {
								if halve[e.curve] == nil {
									halve[e.curve] = map[int]bool{}
								}
								halve[e.curve][min(e.at, e.next)] = true
							}
						}
					}
				}
			}
		}
	}
	halved := false
	for ci, segs := range halve {
		sc := &s.Curve[ci]
		sA, sB := sc.SurfaceA(a, b), sc.SurfaceB(a, b)
		pts := make([]SCurvePt, 0, len(sc.Points)+len(segs))
		for j, pt := range sc.Points {
			pts = append(pts, pt)
			if !segs[j] {
				continue
			}
			if m, ok := sc.midpointOn(sA, sB, pt.P, sc.Points[j+1].P); ok {
				pts = append(pts, SCurvePt{P: m})
				halved = true
			}
		}
		sc.Points = pts
	}
	return halved
}

// grazeSagsTooFar tells whether e's segment, beside f's leaving the same
// vertex along a surface both curves lie on, at an angle under
// grazeMaxSin's, sags off that surface by more than grazeSagShare of how
// far the two are apart at its middle.
func (s *SShell) grazeSagsTooFar(a, b *SShell, e, f grazeEnd, tol float64) bool {
	ce, cf := &s.Curve[e.curve], &s.Curve[f.curve]
	eA, eB := ce.SurfaceA(a, b), ce.SurfaceB(a, b)
	fA, fB := cf.SurfaceA(a, b), cf.SurfaceB(a, b)
	var shared *SSurface
	for _, x := range []*SSurface{eA, eB} {
		if x != nil && (x == fA || x == fB) {
			shared = x
		}
	}
	p, q := ce.Points[e.at].P, ce.Points[e.next].P
	// The surfaces a curve names are its operands' as they were; one
	// the vertex isn't on tells nothing of the meet here.
	if shared == nil || !onSurface(eA, p) || !onSurface(eB, p) || !onSurface(fA, p) || !onSurface(fB, p) {
		return false
	}
	de := q.Minus(p)
	df := cf.Points[f.next].P.Minus(cf.Points[f.at].P)
	l := de.Magnitude()
	if l < geom.LengthEps || df.Magnitude() < geom.LengthEps {
		return false
	}
	// The angle is the curves' own, from their surfaces' normals at the
	// vertex, not their chords'.
	te, ok := tangentAlong(eA, eB, p, de)
	if !ok {
		return false
	}
	tf, ok := tangentAlong(fA, fB, p, df)
	if !ok {
		return false
	}
	cos := te.Dot(tf)
	if cos <= 0 {
		return false
	}
	sin := math.Sqrt(max(0, 1-cos*cos))
	if sin >= grazeMaxSin {
		return false
	}
	// A curve lying on the other, as where an operand's edge and an
	// intersection curve coincide, or two faces' outlines do, is no
	// grazing meet: f's next point lies on e's segment.
	fq := cf.Points[f.next].P
	if fq.OnLineSegment(p, q, splitEndTol) {
		return false
	}
	// The sag that counts is off the surface both curves lie on: within
	// it, as along a plane, the chord stays between the curves.
	mid := p.Plus(q).ScaledBy(0.5)
	var uv geom.Point2d
	shared.ClosestPointTo(mid, &uv, true)
	sag := shared.PointAt(uv.U, uv.V).Minus(mid).Magnitude()
	if sag < tol/grazeSagFloor {
		return false
	}
	return sag > grazeSagShare*sin*l/2
}

// grazeMinSin is the sine of the angle under which two surfaces meet
// tangentially along a curve, for refineGrazingMeets: such a curve, a
// fillet's edge along its face, runs nearly along its neighbours by its
// nature, and its points are ill made off it.
const grazeMinSin = 0.2

// tangentAlong is the unit tangent at p of the curve where sA and sB
// meet, turned to go the way of d. It is false where they meet
// tangentially there, at an angle under grazeMinSin's.
func tangentAlong(sA, sB *SSurface, p, d geom.Vector) (geom.Vector, bool) {
	if sA == nil || sB == nil {
		return geom.Vector{}, false
	}
	var a, b geom.Point2d
	sA.ClosestPointTo(p, &a, true)
	sB.ClosestPointTo(p, &b, true)
	t := sA.NormalAt(a.U, a.V).WithMagnitude(1).Cross(sB.NormalAt(b.U, b.V).WithMagnitude(1))
	if t.Magnitude() < grazeMinSin {
		return geom.Vector{}, false
	}
	t = t.WithMagnitude(1)
	if t.Dot(d) < 0 {
		t = t.ScaledBy(-1)
	}
	return t, true
}

// midpointOn is the point of c's curve nearest the middle of its segment
// from p to q: on its exact curve, or on both the surfaces it lies on.
// It is false where none is found near the segment.
func (c *SCurve) midpointOn(sA, sB *SSurface, p, q geom.Vector) (geom.Vector, bool) {
	mid := p.Plus(q).ScaledBy(0.5)
	l := q.Minus(p).Magnitude()
	var m geom.Vector
	switch {
	case c.IsExact:
		tp, _ := c.Exact.ClosestPointTo(p, true)
		tq, _ := c.Exact.ClosestPointTo(q, true)
		m = c.Exact.PointAt((tp + tq) / 2)
	case sA != nil && sB != nil:
		m = sA.ClosestPointOnThisAndSurface(sB, mid)
	default:
		return geom.Vector{}, false
	}
	// The point must lie between the ends, near the middle: where the
	// surfaces meet tangentially the closest point on both is ill made
	// and can slide along them.
	d := q.Minus(p)
	if t := m.Minus(p).Dot(d) / d.Dot(d); t < 0.25 || t > 0.75 || m.Minus(mid).Magnitude() > l/4 {
		return geom.Vector{}, false
	}
	if !c.IsExact && (!onSurface(sA, m) || !onSurface(sB, m)) {
		return geom.Vector{}, false
	}
	return m, true
}
