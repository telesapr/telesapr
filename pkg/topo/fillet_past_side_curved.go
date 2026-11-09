package topo

import (
	"cmp"
	"maps"
	"math"
	"slices"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// A side the foot runs into may be curved: the round rim of a disc, a
// rod's foot fillet running past it. The face is widened past it as past
// a flat side, by a slab in the face's plane, and cut back by the side's
// own cylinder. As a flat side's slab is the whole side swept outward,
// this slab takes the whole of the side's cylinder in: all round a
// disc's rim, or the whole arc of a rounded corner. A slab or cutter that
// left a piece of the side's cylinder standing beside the cutter's face
// on the same cylinder, or overlapping it, made Booleans that fail
// (tried 2026-11-09: the rim cut back where the slab ended, the trims of
// the cut cylinder lost their bottom and ends). The slab's wall toward
// the solid is a polygon a little inside the rim, since a wall on the
// cylinder would lie on the side, and the slab runs down the side's
// height, so that what it adds within the rim lies in the solid already;
// the solid is checked to have grown by just what lies past the
// cylinder (widenPastCurved). The cut back is the cylinder's ring or
// sector from the rim outward past the slab, or for a bore the bore
// itself (curvedCutter): its curved face, where it is kept, is the side
// again, grown up by the fillet's sections where the cylinder cuts them.
//
// Only a cylinder square to the face is taken, standing wholly below the
// face, whose edge with the face is one arc or the whole circle; the
// solid going on past it (a ledge round a disc) is refused.

// curvedSide is the cylinder a curved side lies on: a point o of its
// axis in the plane of the face the fillet rolls on, the axis (the
// face's outward normal), its radius, and sgn, 1 where the solid lies
// inside the cylinder (a disc's rim) and -1 outside it (a bore's wall).
// lo is how far down the axis from o the side reaches. e1, e2 measure
// angles about the axis; the side spans a0 to a1 round it, or goes all
// round where full. angles are those of the foot's points past it.
type curvedSide struct {
	o, axis geom.Vector
	r, sgn  float64
	lo      float64
	e1, e2  geom.Vector
	a0, a1  float64
	full    bool
	angles  []float64
}

// curvedStep is the most angle a polygon side of the slab spans: its
// chord lies within 0.2% of the radius of its circle.
const curvedStep = math.Pi / 32

// curvedInset is how far inside the rim, as a share of the radius and
// at most 0.5, the slab's wall toward the solid stands.
const curvedInset = 0.05

// curvedSideOf is the cylinder face s lies on, as a side the foot on a
// flat face of outward normal n at height top (along n) runs into, or
// false where s is no such side: no cylinder, not square to the face,
// reaching above the face, or not met by the face in one arc or circle
// (curvedExtent).
func (t *Topology) curvedSideOf(s int, n geom.Vector, top float64) (*curvedSide, bool) {
	sf := t.Surface(s)
	if sf == nil {
		return nil, false
	}
	var axis, centre, a, b geom.Vector
	var r float64
	if !sf.IsCylinder(&axis, &centre, &r, &a, &b) || r <= geom.LengthEps {
		return nil, false
	}
	if axis.Magnitude() <= geom.LengthEps || math.Abs(axis.Normal().Dot(n)) < 1-1e-9 {
		return nil, false
	}
	o := centre.Plus(n.ScaledBy(top - centre.Dot(n)))
	// The face's outward normal points away from the axis on a disc's
	// rim, toward it on a bore's wall.
	p := sf.PointAt(0.5, 0.5)
	radial := p.Minus(o).Minus(n.ScaledBy(p.Minus(o).Dot(n)))
	sgn := 1.0
	if sf.NormalAt(0.5, 0.5).Dot(radial) < 0 {
		sgn = -1
	}
	e1 := n.NormalDir(0).Normal()
	c := &curvedSide{o: o, axis: n, r: r, sgn: sgn, e1: e1, e2: n.Cross(e1).Normal()}
	if !t.curvedExtent(c) {
		return nil, false
	}
	return c, true
}

// onCylinder reports whether face f lies on c's cylinder, its outward
// normal the same way.
func (t *Topology) onCylinder(f int, c *curvedSide) bool {
	sf := t.Surface(f)
	if sf == nil {
		return false
	}
	var axis, centre, a, b geom.Vector
	var r float64
	if !sf.IsCylinder(&axis, &centre, &r, &a, &b) || math.Abs(r-c.r) > geom.LengthEps ||
		axis.Magnitude() <= geom.LengthEps || math.Abs(axis.Normal().Dot(c.axis)) < 1-1e-9 {
		return false
	}
	if centre.DistanceToLine(c.o, c.axis) > geom.LengthEps {
		return false
	}
	p := sf.PointAt(0.5, 0.5)
	radial := p.Minus(c.o).Minus(c.axis.ScaledBy(p.Minus(c.o).Dot(c.axis)))
	return (sf.NormalAt(0.5, 0.5).Dot(radial) < 0) == (c.sgn < 0)
}

// curvedExtent sets how far down c's cylinder its faces reach, and the
// angles they span where they meet the face the fillet rolls on, and
// reports whether they make one side: every face on the cylinder below
// the face's plane, and their edges in that plane one arc, or the whole
// circle.
func (t *Topology) curvedExtent(c *curvedSide) bool {
	type span struct{ a, b float64 }
	var spans []span
	c.lo = 0
	for f := range t.Faces {
		if !t.onCylinder(f, c) {
			continue
		}
		for _, l := range t.Faces[f].Loops {
			for _, ce := range t.Loops[l].CoEdges {
				pts := t.Edges[t.CoEdges[ce].Edge].Points
				top := true
				for _, q := range pts {
					h := q.Minus(c.o).Dot(c.axis)
					if h > geom.LengthEps {
						return false
					}
					c.lo = math.Min(c.lo, h)
					top = top && h >= -geom.LengthEps
				}
				if !top || len(pts) < 2 {
					continue
				}
				_, a := c.radial(pts[0])
				_, m := c.radial(pts[len(pts)/2])
				_, b := c.radial(pts[len(pts)-1])
				if pts[0].Equals(pts[len(pts)-1]) {
					spans = append(spans, span{0, 2 * math.Pi})
					continue
				}
				// The arc runs from a to b one way round, through m.
				if turn(a, m) < turn(a, b) {
					spans = append(spans, span{a, a + turn(a, b)})
				} else {
					spans = append(spans, span{b, b + turn(b, a)})
				}
			}
		}
	}
	if len(spans) == 0 || c.lo >= -geom.LengthEps {
		return false
	}
	for i := range spans {
		width := spans[i].b - spans[i].a
		spans[i].a = turn(0, spans[i].a)
		spans[i].b = spans[i].a + width
	}
	slices.SortFunc(spans, func(x, y span) int { return cmp.Compare(x.a, y.a) })
	const eps = 1e-6
	merged := []span{spans[0]}
	for _, s := range spans[1:] {
		last := &merged[len(merged)-1]
		if s.a <= last.b+eps {
			last.b = math.Max(last.b, s.b)
		} else {
			merged = append(merged, s)
		}
	}
	// The last may run round past the first's start.
	if len(merged) > 1 {
		first, last := merged[0], merged[len(merged)-1]
		if last.b >= first.a+2*math.Pi-eps {
			merged[0] = span{last.a, math.Max(last.b, first.b+2*math.Pi)}
			merged = merged[:len(merged)-1]
		}
	}
	if len(merged) != 1 {
		return false
	}
	if merged[0].b-merged[0].a >= 2*math.Pi-eps {
		c.full, c.a0, c.a1 = true, 0, 2*math.Pi
		return true
	}
	c.a0, c.a1 = merged[0].a, merged[0].b
	return true
}

// turn is how far round from angle a to angle b, counter-clockwise, in
// [0, 2π).
func turn(a, b float64) float64 {
	d := math.Mod(b-a, 2*math.Pi)
	if d < 0 {
		d += 2 * math.Pi
	}
	return d
}

// same reports whether c and d lie on one cylinder.
func (c *curvedSide) same(d *curvedSide) bool {
	return math.Abs(c.r-d.r) <= geom.LengthEps && c.sgn == d.sgn &&
		c.o.Minus(d.o).Magnitude() <= geom.LengthEps && c.axis.Dot(d.axis) > 1-1e-9
}

// radial is p's distance from the axis, and its angle about it.
func (c *curvedSide) radial(p geom.Vector) (float64, float64) {
	q := p.Minus(c.o)
	x, y := q.Dot(c.e1), q.Dot(c.e2)
	return math.Hypot(x, y), math.Atan2(y, x)
}

// past is how far p lies past the cylinder, out of the solid.
func (c *curvedSide) past(p geom.Vector) float64 {
	d, _ := c.radial(p)
	return c.sgn * (d - c.r)
}

// at is the point at distance d from the axis, angle a about it, and h
// along it from o.
func (c *curvedSide) at(d, a, h float64) geom.Vector {
	return c.o.Plus(c.e1.ScaledBy(d * math.Cos(a))).Plus(c.e2.ScaledBy(d * math.Sin(a))).Plus(c.axis.ScaledBy(h))
}

// curvedLayout is the slab's polygons: steps of the side's span, the
// distance from the axis of the corners toward the solid (in) and away
// from it (out, 0 for a bore filled whole), and of the cutter's far
// side.
type curvedLayout struct {
	steps        int
	in, out, far float64
}

// layout is the slab and cutter reaching depth past the side, and
// whether the side allows them: a bore's arc whose cut would reach near
// its axis does not.
func (c *curvedSide) layout(depth float64) (curvedLayout, bool) {
	span := c.a1 - c.a0
	steps := max(2, int(math.Ceil(span/curvedStep)))
	w := math.Cos(span / float64(steps) / 2)
	inset := math.Min(curvedInset*c.r, 0.5)
	if c.sgn > 0 {
		// Corners away from the solid stand out so that the chords
		// between them lie past the depth.
		out := (c.r + depth) / w
		return curvedLayout{steps: steps, in: c.r - inset, out: out, far: out + 1}, true
	}
	// Corners toward the solid of a bore stand out so that the chords
	// between them stay in the solid.
	in := (c.r + inset) / w
	if c.full {
		return curvedLayout{steps: steps, in: in}, true
	}
	if c.r-depth < 0.25*c.r {
		return curvedLayout{}, false
	}
	return curvedLayout{steps: steps, in: in, out: c.r - depth, far: math.Max(c.r-depth-1, 0.25*c.r)}, true
}

// ring is the corners at distance d from the axis at the layout's steps
// over the side's span, at height h, from a0 to a1; for a full side the
// last, which is the first again, is left out.
func (c *curvedSide) ring(l curvedLayout, d, h float64) []geom.Vector {
	var pts []geom.Vector
	n := l.steps
	if c.full {
		n--
	}
	for i := range n + 1 {
		pts = append(pts, c.at(d, c.a0+(c.a1-c.a0)*float64(i)/float64(l.steps), h))
	}
	return pts
}

// lines is the closed polygon pts as lines.
func lines(pts []geom.Vector) []srf.SBezier {
	var out []srf.SBezier
	for i := range pts {
		out = append(out, srf.SBezierFrom2(pts[i], pts[(i+1)%len(pts)]))
	}
	return out
}

// pastSpan is the shortest range of angles, from p0 up to p1, holding
// every one of the foot's points past the side, and whether there are
// any.
func (c *curvedSide) pastSpan() (p0, p1 float64, ok bool) {
	if len(c.angles) == 0 {
		return 0, 0, false
	}
	as := slices.Clone(c.angles)
	slices.Sort(as)
	// The largest gap between neighbours, round the turn, is left out.
	gap, at := as[0]+2*math.Pi-as[len(as)-1], 0
	for i := 1; i < len(as); i++ {
		if g := as[i] - as[i-1]; g > gap {
			gap, at = g, i
		}
	}
	p0 = as[at]
	return p0, p0 + turn(p0, as[(at+len(as)-1)%len(as)]), true
}

// seamClear is how far round past the foot's points past the side the
// cutter's curved face has no seam, and maxArc the most a piece of its
// circle may span there.
const (
	seamClear = 0.05
	maxArc    = 0.9 * math.Pi
)

// arcBreaks is where the pieces of the cutter's circle meet, from a0 to
// a1 round the side (a whole turn for a full one): no more than a
// quarter turn apart, but for the piece the fillet's foot runs past the
// side over, which is one piece. A seam of the cutter's curved face
// under the fillet, where the fillet's section meets the cylinder,
// left the trims open there (tried 2026-11-09).
func (c *curvedSide) arcBreaks() []float64 {
	from, to := c.a0, c.a1
	var keep []float64
	if p0, p1, ok := c.pastSpan(); ok && p1-p0+2*seamClear <= maxArc {
		s0 := p0 - seamClear
		if c.full {
			from, to = s0, s0+2*math.Pi
			keep = []float64{s0, s0 + p1 - p0 + 2*seamClear}
		} else {
			s0 = from + turn(from, s0)
			if s0 > to {
				s0 -= 2 * math.Pi
			}
			keep = []float64{math.Max(s0, from), math.Min(s0+p1-p0+2*seamClear, to)}
		}
	}
	quarters := func(out []float64, a, b float64) []float64 {
		n := max(1, int(math.Ceil((b-a)/(math.Pi/2)-1e-9)))
		for i := 1; i <= n; i++ {
			out = append(out, a+(b-a)*float64(i)/float64(n))
		}
		return out
	}
	out := []float64{from}
	if keep == nil || keep[1] <= keep[0] {
		return quarters(out, from, to)
	}
	if keep[0] > from+geom.LengthEps {
		out = quarters(out, from, keep[0])
	}
	out = append(out, keep[1])
	if keep[1] < to-geom.LengthEps {
		out = quarters(out, keep[1], to)
	} else {
		out[len(out)-1] = to
	}
	return out
}

// arcs is the circle at distance d from the axis through the angles
// breaks, in their order, at height h, as rational quadratics.
func (c *curvedSide) arcs(d float64, breaks []float64, h float64) []srf.SBezier {
	var out []srf.SBezier
	for i := 0; i+1 < len(breaks); i++ {
		a, b := breaks[i], breaks[i+1]
		w := math.Cos((b - a) / 2)
		sb := srf.SBezierFrom3(c.at(d, a, h), c.at(d/w, (a+b)/2, h), c.at(d, b, h))
		sb.Weight[1] = w
		out = append(out, sb)
	}
	return out
}

// prismOf is the extrusion along by of the plane region the closed
// curves bound (an outer loop and the holes in it), and whether they
// close into one region.
func prismOf(curves []srf.SBezier, by geom.Vector, color core.RGBA, tol srf.Tolerance) (srf.SShell, bool) {
	sbl := srf.SBezierList{Curves: curves}
	var sets srf.SBezierLoopSetSet
	var poly srf.SPolygon
	allClosed, allCoplanar := false, false
	var notClosed srf.SEdge
	var notCoplanar geom.Vector
	sets.FindOuterFacesFrom(&sbl, &poly, nil, tol, &allClosed, &notClosed, &allCoplanar, &notCoplanar, nil)
	if !allClosed || len(sets.Sets) != 1 {
		return srf.SShell{}, false
	}
	sh := srf.SShell{Tol: tol}
	sh.MakeFromExtrusionOf(&sets.Sets[0], geom.Vector{}, by, color)
	return sh, true
}

// fan is twice the area of the polygon whose corners lie at distance d
// from the axis at the layout's steps over the side's span, closed
// through the axis, or the whole polygon for a full side.
func (c *curvedSide) fan(l curvedLayout, d float64) float64 {
	return float64(l.steps) * d * d * math.Sin((c.a1-c.a0)/float64(l.steps))
}

// curvedSlab is the slab that widens the face past the curved side: the
// region between the layout's corners over the side's span, from the
// face's plane down the side's height; and the volume of it that lies
// past the cylinder, out of the solid.
func (c *curvedSide) curvedSlab(l curvedLayout, color core.RGBA, tol srf.Tolerance) (srf.SShell, float64, bool) {
	in := c.ring(l, l.in, 0)
	var curves []srf.SBezier
	switch {
	case c.full && l.out == 0:
		curves = lines(in)
	case c.full:
		curves = append(lines(in), lines(c.ring(l, l.out, 0))...)
	default:
		curves = lines(append(in, reversed(c.ring(l, l.out, 0))...))
	}
	slab, ok := prismOf(curves, c.axis.ScaledBy(c.lo), color, tol)
	past := -c.lo * math.Abs(c.fan(l, l.out)-c.r*c.r*(c.a1-c.a0)) / 2
	return slab, past, ok
}

// curvedCutter is the solid that cuts the widened solid back to the
// side's cylinder: its ring or sector from the rim out past the slab, or
// a bore's cylinder, from below the side to above the face by more than
// the fillet's size r. Its faces take the side's colour and identity, so
// that its curved face, where it is kept, is the side again. ok is false
// where any of sh lies in it.
func (c *curvedSide) curvedCutter(l curvedLayout, r float64, side *srf.SSurface, sh *srf.SShell) (srf.SShell, bool) {
	h0, h1 := c.lo-1, r+1
	in := func(p geom.Vector) bool {
		d, a := c.radial(p)
		h := p.Minus(c.o).Dot(c.axis)
		// A bore's wall is chorded inside its cylinder, by up to the chord
		// tolerance.
		inside := c.sgn*(d-c.r) > 2*sh.Tol.ChordTolMm()+10*geom.LengthEps && h > h0 && h < h1
		if l.far != 0 {
			inside = inside && math.Abs(d-c.r) < math.Abs(l.far-c.r)
		}
		return inside && (c.full || turn(c.a0, a) < c.a1-c.a0)
	}
	var m srf.SMesh
	cs := sh.Clone()
	cs.TriangulateInto(&m)
	for _, tr := range m.Triangles {
		if in(tr.A) || in(tr.B) || in(tr.C) || in(tr.A.Plus(tr.B).Plus(tr.C).ScaledBy(1.0/3)) {
			return srf.SShell{}, false
		}
	}
	breaks := c.arcBreaks()
	back := slices.Clone(breaks)
	slices.Reverse(back)
	var curves []srf.SBezier
	switch {
	case c.full && l.far == 0:
		curves = c.arcs(c.r, breaks, h0)
	case c.full:
		curves = append(c.arcs(c.r, breaks, h0), c.arcs(l.far, breaks, h0)...)
	default:
		curves = c.arcs(c.r, breaks, h0)
		curves = append(curves, srf.SBezierFrom2(c.at(c.r, c.a1, h0), c.at(l.far, c.a1, h0)))
		curves = append(curves, c.arcs(l.far, back, h0)...)
		curves = append(curves, srf.SBezierFrom2(c.at(l.far, c.a0, h0), c.at(c.r, c.a0, h0)))
	}
	cut, ok := prismOf(curves, c.axis.ScaledBy(h1-h0), side.Color, sh.Tol)
	if !ok {
		return srf.SShell{}, false
	}
	for i := range cut.Surface {
		cut.Surface[i].Face = side.Face
		cut.Surface[i].Origin = side.Origin
		cut.Surface[i].Sheet = side.Sheet
	}
	return cut, true
}

// meshVolume is the volume of sh's mesh.
func meshVolume(sh *srf.SShell) float64 {
	var m srf.SMesh
	c := sh.Clone()
	c.TriangulateInto(&m)
	return m.CalculateVolume()
}

// widenPastCurved adds to wide, the solid sh being widened, the slab past
// the curved side s, and gives the cutter that takes it back, and
// whether both could be made: the slab must add to the solid just what
// lies past the cylinder, within what the mesh's chords allow, or it
// filled a hole or a notch near the side.
func widenPastCurved(wide *srf.SShell, sh *srf.SShell, s *runOffSide, off *runOff, r float64) (srf.SShell, bool) {
	c := s.curved
	l, ok := c.layout(math.Max(s.past, 0) + r + 1)
	if !ok {
		return srf.SShell{}, false
	}
	cut, ok := c.curvedCutter(l, r, s.surf, sh)
	if !ok {
		return srf.SShell{}, false
	}
	slab, adds, ok := c.curvedSlab(l, off.color, sh.Tol)
	if !ok {
		return srf.SShell{}, false
	}
	before := meshVolume(wide)
	var u srf.SShell
	u.MakeFromUnionOf(wide, &slab)
	// The slab's top merges with the face the fillet rolls on, so that
	// the foot lies inside one face.
	u.MergeCoincidentSurfaces()
	allow := 2*sh.Tol.ChordTolMm()*c.r*(c.a1-c.a0)*(-c.lo) + 1e-6
	if unsound(&u) || math.Abs(meshVolume(&u)-before-adds) > allow {
		return srf.SShell{}, false
	}
	*wide = u
	return cut, true
}

// offIntoCurved is offInto for a foot's point p running past an edge
// that isn't straight, into face s across it: off with s's cylinder
// added as a curved side, or nil where s is no curved side curvedSideOf
// takes. The faces of one cylinder (a rim in halves) are one side.
func (t *Topology) offIntoCurved(off *runOff, s int, p, n geom.Vector) *runOff {
	side := off.sides[s]
	if side == nil {
		c, ok := t.curvedSideOf(s, n, off.at.Dot(n))
		if !ok {
			return nil
		}
		for _, o := range off.sides {
			if o.curved != nil && o.curved.same(c) {
				side = o
				break
			}
		}
		if side == nil {
			side = &runOffSide{past: math.Inf(-1), surf: t.Surface(s), curved: c}
		}
		off.sides[s] = side
	}
	if side.curved == nil {
		return nil
	}
	past := side.pastOf(p)
	side.past = math.Max(side.past, past)
	if past > 0 {
		_, a := side.curved.radial(p)
		side.curved.angles = append(side.curved.angles, a)
	}
	return off
}

// distinct is off's sides, each once: the faces of one cylinder, and
// curved sides found on several of the network's circular edges, are
// one side.
func (off *runOff) distinct() []*runOffSide {
	keys := slices.Sorted(maps.Keys(off.sides))
	var out []*runOffSide
	for _, k := range keys {
		s := off.sides[k]
		if slices.Contains(out, s) {
			continue
		}
		if s.curved != nil {
			if i := slices.IndexFunc(out, func(o *runOffSide) bool { return o.curved != nil && o.curved.same(s.curved) }); i >= 0 {
				out[i].past = math.Max(out[i].past, s.past)
				out[i].curved.angles = append(out[i].curved.angles, s.curved.angles...)
				continue
			}
		}
		out = append(out, s)
	}
	return out
}
