package topo

import (
	"errors"
	"fmt"
	"math"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// A shell of a prism with curved sides (roadmap 15.2): a solid whose
// faces are two flat caps square to one direction and sides that run
// along it, flat or extrusions of arcs, as an extruded rounded
// rectangle, a cup or a holed block. Its inside is the cap's profile
// offset into the material by the thickness, lines moved square to
// themselves and arcs given a smaller or larger radius, their ends met
// again, extruded between the caps moved in, and taken away. A cap
// taken away stays where it is, as in ShellSolid.

// profilePiece is a piece of a cap's loop: a line from a to b, or an
// arc about centre of radius r from a to b, turning by angle about the
// cap's normal; into is the direction into the cap's material, square
// to the piece at its middle.
type profilePiece struct {
	a, b    geom.Vector
	arc     bool
	centre  geom.Vector
	r       float64
	angle   float64
	into    geom.Vector
	outward bool // an arc whose material lies outside its circle
}

// shellPrism hollows sh, a prism along axis with caps at levels lo and
// hi, open where opened. ok is false when sh is no such prism.
func shellPrism(t *Topology, sh *srf.SShell, opened map[int]bool, thick float64, origin uint32) (out *srf.SShell, ok bool, err error) {
	// The direction: that of a curved side's extrusion.
	var axis geom.Vector
	for f := range t.Faces {
		var of srf.SBezier
		var along geom.Vector
		if _, flat := t.planarNormal(f); !flat {
			if !t.Surface(f).IsExtrusion(&of, &along) {
				return nil, true, shellErr(t.faceAnchor(f), "a face of the solid is curved and not the side of a prism; "+
					"a shell hollows prisms of lines and arcs, and solids of flat faces")
			}
			axis = along.WithMagnitude(1)
			break
		}
	}
	if axis.Magnitude() == 0 {
		return nil, false, nil
	}
	var caps []int
	lo, hi := math.Inf(1), math.Inf(-1)
	for f := range t.Faces {
		n, flat := t.planarNormal(f)
		if !flat {
			var of srf.SBezier
			var along geom.Vector
			if !t.Surface(f).IsExtrusion(&of, &along) || along.WithMagnitude(1).Cross(axis).Magnitude() > 1e-9 {
				return nil, true, shellErr(t.faceAnchor(f), "a curved face of the solid runs across its sides; "+
					"a shell hollows prisms whose sides all run one way")
			}
			if opened[f] {
				return nil, true, shellErr(t.faceAnchor(f), "a shell of a solid with curved sides is open only at its ends")
			}
			continue
		}
		switch c := math.Abs(n.Dot(axis)); {
		case c > 1-1e-9:
			caps = append(caps, f)
			h := t.faceAnchor(f).Dot(axis)
			lo, hi = math.Min(lo, h), math.Max(hi, h)
		case c < 1e-9:
			if opened[f] {
				return nil, true, shellErr(t.faceAnchor(f), "a shell of a solid with curved sides is open only at its ends")
			}
		default:
			return nil, true, shellErr(t.faceAnchor(f), "a flat face of the solid leans; a shell hollows prisms whose sides all run one way")
		}
	}
	openLo, openHi := false, false
	var bottom []int
	for _, f := range caps {
		switch h := t.faceAnchor(f).Dot(axis); {
		case math.Abs(h-lo) < geom.LengthEps:
			bottom = append(bottom, f)
			openLo = openLo || opened[f]
		case math.Abs(h-hi) < geom.LengthEps:
			openHi = openHi || opened[f]
		default:
			return nil, true, shellErr(t.faceAnchor(f), "the solid's ends lie on more than two levels; a shell hollows prisms")
		}
	}
	z0, z1 := lo+thick, hi-thick
	if openLo {
		z0 = lo
	}
	if openHi {
		z1 = hi
	}
	if z1-z0 <= geom.LengthEps {
		return nil, true, shellErr(t.faceAnchor(caps[0]), "a thickness of %g makes the walls meet, closing up the inside", thick)
	}

	// The profile offset into the material, at level lo.
	var sbl srf.SBezierList
	for _, f := range bottom {
		n, _ := t.planarNormal(f)
		for _, l := range t.Faces[f].Loops {
			curves, err := t.offsetLoop(f, l, n, thick)
			if err != nil {
				return nil, true, err
			}
			sbl.Curves = append(sbl.Curves, curves...)
		}
	}
	var sets srf.SBezierLoopSetSet
	var poly srf.SPolygon
	allClosed, allCoplanar := false, false
	var notClosed srf.SEdge
	var notCoplanar geom.Vector
	sets.FindOuterFacesFrom(&sbl, &poly, nil, sh.Tol, &allClosed, &notClosed, &allCoplanar, &notCoplanar, nil)
	if !allClosed || !allCoplanar || len(sets.Sets) == 0 {
		return nil, true, errors.New("the inside's outline, offset from the solid's end, does not close")
	}
	inner := &srf.SShell{Tol: sh.Tol}
	color := t.Surface(bottom[0]).Color
	for i := range sets.Sets {
		inner.MakeFromExtrusionOf(&sets.Sets[i], axis.ScaledBy(z0-lo), axis.ScaledBy(z1-lo), color)
	}
	for i := range inner.Surface {
		inner.Surface[i].Origin = origin + uint32(i)
	}
	out = &srf.SShell{Tol: sh.Tol}
	out.MakeFromDifferenceOf(sh, inner)
	if out.BooleanFailed {
		return nil, true, errors.New("the Boolean that takes the inside away failed")
	}
	return out, true, nil
}

// offsetLoop offsets loop l of the flat face f, of normal n, into the
// face's material by d: each piece moved square to itself, and where
// two meet, the point where their moved copies meet.
func (t *Topology) offsetLoop(f, l int, n geom.Vector, d float64) ([]srf.SBezier, error) {
	var pieces []profilePiece
	for _, ce := range t.Loops[l].CoEdges {
		e := t.CoEdges[ce].Edge
		pts := t.Edges[e].Points
		if t.CoEdges[ce].Reversed {
			pts = reversed(pts)
		}
		a, b := pts[0], pts[len(pts)-1]
		p := profilePiece{a: a, b: b}
		if t.Straight(e) {
			p.into = t.intoFace(f, a, b, n)
		} else {
			m := pts[len(pts)/2]
			c, ok := circleThrough(a, m, b, n)
			if !ok {
				return nil, shellErr(m, "an edge of the solid's end is curved and not an arc")
			}
			p.arc, p.centre, p.r = true, c, c.Minus(a).Magnitude()
			ra, rb := a.Minus(c), b.Minus(c)
			p.angle = math.Atan2(ra.Cross(rb).Dot(n), ra.Dot(rb))
			// Through the middle point: the long way round if it is.
			if rm := m.Minus(c); math.Signbit(ra.Cross(rm).Dot(n)) != math.Signbit(p.angle) {
				p.angle -= math.Copysign(2*math.Pi, p.angle)
			}
			radial := m.Minus(c).WithMagnitude(1)
			p.outward = t.inFace(f, m.Plus(radial.ScaledBy(1e-3*p.r)), n)
			p.into = radial
			if !p.outward {
				p.into = radial.ScaledBy(-1)
			}
		}
		pieces = append(pieces, p)
	}
	keep, corner, err := offsetProfile(pieces, n, d, fmt.Sprintf("a thickness of %g", d))
	if err != nil {
		return nil, err
	}
	var out []srf.SBezier
	for j, i := range keep {
		curves, err := movedCurves(pieces[i], corner[j], corner[(j+1)%len(keep)], n, 0)
		if err != nil {
			return nil, shellErr(pieces[i].a, "a thickness of %g makes the walls meet, closing up the inside", d)
		}
		out = append(out, curves...)
	}
	return out, nil
}

// offsetProfile moves the closed loop of pieces into its material by d,
// in the plane of normal n: keep are the pieces that stay, in order, an
// arc whose material lies inside it, of radius d or less, going to a
// sharp corner, and corner[j] is where piece keep[j] starts, moved.
// what names d for messages.
func offsetProfile(pieces []profilePiece, n geom.Vector, d float64, what string) (keep []int, corner []geom.Vector, err error) {
	k := len(pieces)
	// The moved copy of each piece: a line through a point, along a
	// direction, or a circle.
	moved := func(p profilePiece) profilePiece {
		q := p
		if p.arc {
			if p.outward {
				q.r = p.r + d
			} else {
				q.r = p.r - d
			}
		} else {
			q.a, q.b = p.a.Plus(p.into.ScaledBy(d)), p.b.Plus(p.into.ScaledBy(d))
		}
		return q
	}
	mv := make([]profilePiece, k)
	for i, p := range pieces {
		mv[i] = moved(p)
		if !p.arc || mv[i].r > geom.LengthEps {
			keep = append(keep, i)
		}
	}
	if len(keep) < 2 {
		return nil, nil, shellErr(pieces[0].a, "%s is more than the radius of the outline, %g", what, pieces[0].r)
	}
	// Where the kept piece before meets each kept piece, moved.
	corner = make([]geom.Vector, len(keep))
	for j, i := range keep {
		prev := keep[(j+len(keep)-1)%len(keep)]
		v := pieces[i].a
		if back := (i + k - 1) % k; back != prev {
			v = pieces[back].centre
		}
		c, err := meetMoved(mv[prev], mv[i], v, pieces[prev], pieces[i], d, n, what)
		if err != nil {
			return nil, nil, err
		}
		corner[j] = c
	}
	return keep, corner, nil
}

// movedCurves is piece p moved to run from a to b, as a line or an arc
// about its centre, the arc in quarters (in parts at least if parts > 0).
// It fails when the piece has turned round.
func movedCurves(p profilePiece, a, b, n geom.Vector, parts int) ([]srf.SBezier, error) {
	if !p.arc {
		if b.Minus(a).Dot(p.b.Minus(p.a)) <= 0 {
			return nil, errors.New("turned round")
		}
		return []srf.SBezier{srf.SBezierFrom2(a, b)}, nil
	}
	ra, rb := a.Minus(p.centre), b.Minus(p.centre)
	angle := math.Atan2(ra.Cross(rb).Dot(n), ra.Dot(rb))
	if math.Signbit(angle) != math.Signbit(p.angle) && math.Abs(angle) > 1e-9 {
		angle += math.Copysign(2*math.Pi, p.angle)
	}
	// A whole circle, from its start round to it.
	if a.Equals(b) && math.Abs(math.Abs(p.angle)-2*math.Pi) < 1e-6 {
		angle = p.angle
	}
	if math.Abs(angle) < 1e-9 || math.Abs(angle-p.angle) > math.Pi {
		return nil, errors.New("turned round")
	}
	return arcPieces(p.centre, a, angle, n, parts), nil
}

// meetMoved is where the moved pieces p and q, which met at v before
// they moved, meet now: where they meet smoothly, v moved square to
// both; otherwise the crossing of their lines and circles nearest v.
func meetMoved(p, q profilePiece, v geom.Vector, p0, q0 profilePiece, d float64, n geom.Vector, what string) (geom.Vector, error) {
	tp, tq := pieceTangent(p0, false, n), pieceTangent(q0, true, n)
	if tp.Cross(tq).Magnitude() < 1e-9 && tp.Dot(tq) > 0 {
		into := q0.into
		if q0.arc {
			into = v.Minus(q0.centre).WithMagnitude(1)
			if !q0.outward {
				into = into.ScaledBy(-1)
			}
		}
		return v.Plus(into.ScaledBy(d)), nil
	}
	var cands []geom.Vector
	switch {
	case !p.arc && !q.arc:
		dp, dq := p.b.Minus(p.a), q.b.Minus(q.a)
		den := dp.Cross(dq).Dot(n)
		if math.Abs(den) < 1e-12 {
			return v, shellErr(v, "two edges of the outline run on one line")
		}
		s := q.a.Minus(p.a).Cross(dq).Dot(n) / den
		cands = append(cands, p.a.Plus(dp.ScaledBy(s)))
	case p.arc && q.arc:
		cands = circleCircle(p.centre, p.r, q.centre, q.r, n)
	case p.arc:
		cands = lineCircle(q.a, q.b.Minus(q.a), p.centre, p.r)
	default:
		cands = lineCircle(p.a, p.b.Minus(p.a), q.centre, q.r)
	}
	if len(cands) == 0 {
		return v, shellErr(v, "%s makes the sides meet at a corner of the outline", what)
	}
	best := cands[0]
	for _, c := range cands[1:] {
		if c.Minus(v).Magnitude() < best.Minus(v).Magnitude() {
			best = c
		}
	}
	return best, nil
}

// pieceTangent is a piece's direction of travel at its end, or with
// atStart at its start, in the plane of normal n.
func pieceTangent(p profilePiece, atStart bool, n geom.Vector) geom.Vector {
	if !p.arc {
		return p.b.Minus(p.a).WithMagnitude(1)
	}
	at := p.b
	if atStart {
		at = p.a
	}
	// Turning by a positive angle about n goes along n x radial.
	t := n.WithMagnitude(1).Cross(at.Minus(p.centre).WithMagnitude(1))
	if p.angle < 0 {
		t = t.ScaledBy(-1)
	}
	return t
}

// circleThrough is the centre of the circle through a, m and b, in the
// plane of normal n.
func circleThrough(a, m, b, n geom.Vector) (geom.Vector, bool) {
	ab, am := b.Minus(a), m.Minus(a)
	den := 2 * ab.Cross(am).Dot(n)
	if math.Abs(den) < 1e-12 {
		return geom.Vector{}, false
	}
	c := am.ScaledBy(ab.MagSquared()).Minus(ab.ScaledBy(am.MagSquared())).Cross(n).ScaledBy(1 / den)
	return a.Plus(c), true
}

// lineCircle are where the line through p along d meets the circle of
// centre c and radius r in its plane.
func lineCircle(p, d, c geom.Vector, r float64) []geom.Vector {
	dd := d.MagSquared()
	f := p.Minus(c)
	b := f.Dot(d) / dd
	disc := b*b - (f.MagSquared()-r*r)/dd
	if disc < 0 {
		if disc > -1e-9 {
			disc = 0
		} else {
			return nil
		}
	}
	s := math.Sqrt(disc)
	return []geom.Vector{p.Plus(d.ScaledBy(-b + s)), p.Plus(d.ScaledBy(-b - s))}
}

// circleCircle are where two circles of one plane, of normal n, meet.
func circleCircle(c1 geom.Vector, r1 float64, c2 geom.Vector, r2 float64, n geom.Vector) []geom.Vector {
	dv := c2.Minus(c1)
	dist := dv.Magnitude()
	if dist < 1e-12 || dist > r1+r2+1e-9 || dist < math.Abs(r1-r2)-1e-9 {
		return nil
	}
	x := (dist*dist + r1*r1 - r2*r2) / (2 * dist)
	h := math.Sqrt(math.Max(0, r1*r1-x*x))
	u := dv.ScaledBy(1 / dist)
	w := n.WithMagnitude(1).Cross(u)
	base := c1.Plus(u.ScaledBy(x))
	return []geom.Vector{base.Plus(w.ScaledBy(h)), base.Minus(w.ScaledBy(h))}
}

// arcPieces is the arc about centre from a turning by angle about n, in
// pieces of at most a quarter turn, or in parts pieces when parts > 0,
// each a rational quadratic Bezier.
func arcPieces(centre, a geom.Vector, angle float64, n geom.Vector, parts int) []srf.SBezier {
	k := quarters(angle)
	if parts > 0 {
		k = parts
	}
	step := angle / float64(k)
	var out []srf.SBezier
	p := a
	for range k {
		q := p.Minus(centre).RotatedAroundAxis(n.WithMagnitude(1), step).Plus(centre)
		mid := p.Plus(q).ScaledBy(0.5).Minus(centre)
		r := p.Minus(centre).Magnitude()
		w := math.Cos(step / 2)
		ctrl := centre.Plus(mid.WithMagnitude(r / w))
		out = append(out, arc(p, ctrl, q, w))
		p = q
	}
	return out
}

// quarters is how many pieces of at most a quarter turn an arc of angle
// takes.
func quarters(angle float64) int {
	return max(1, int(math.Ceil(math.Abs(angle)/(math.Pi/2)-1e-9)))
}
