package topo

import (
	"cmp"
	"errors"
	"math"
	"slices"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// A closed fillet round a rod through a plate whose foot, a circle
// round the rod on the plate's top, reaches past the plate's sides runs
// off the top. The ball rolls on as if the top went on; the sides are
// grown up by the fillet's sections where their planes cut it. That is
// made as it reads: the plate is widened past those sides, the fillet
// made on it, where its foot lies inside the top, and the widened
// solid cut back by the sides' planes.

// runOff is where a circular fillet's foot runs off its flat face: the
// face, the fillet's side k on it, and the flat faces square to it that
// the foot runs into, by face index of the topology it was found in.
type runOff struct {
	face int
	k    int
	// n is the face's outward normal and at a point of it.
	n, at geom.Vector
	// color is the face's, which the slabs take so that their tops
	// merge with it.
	color core.RGBA
	sides map[int]*runOffSide
	// varies is set where a fillet's radius varies along the edge.
	varies bool
}

// runOffSide is a flat side face the foot runs into: its plane (a
// point and outward normal), its outline, how far past its plane the
// foot goes, and the surface whose colour and identity a cut on its
// plane takes.
type runOffSide struct {
	p0, n geom.Vector
	poly  []geom.Vector
	past  float64
	surf  *srf.SSurface
	// along is the way the face is widened past the side, and slant is
	// set where the side leans to the face (slabAlong).
	along geom.Vector
	slant bool
	// curved is set where the side is a cylinder square to the face
	// (fillet_past_side_curved.go); p0 and n are then unused.
	curved *curvedSide
}

// pastOf is how far p lies past the side, out of the solid.
func (s *runOffSide) pastOf(p geom.Vector) float64 {
	if s.curved != nil {
		return s.curved.past(p)
	}
	return p.Minus(s.p0).Dot(s.n)
}

// offInto adds to off the side the foot's point p runs into: past the
// edge of the face nearest p, or within the margin of it. off becomes
// nil where that edge isn't straight, or the face across it isn't flat,
// or leans to the face the fillet rolls on (normal n) by more than
// maxSlant. A side with
// holes (a cross bore in a plate's side) is taken by its outer loop:
// the slab caps the holes' mouths, and cutting it back opens them.
func (t *Topology) offInto(off *runOff, p, n geom.Vector, others [][]geom.Vector, otherEdges []int) *runOff {
	best, bd := -1, math.Inf(1)
	for i, pts := range others {
		if d := distanceToPolyline(p, pts); d < bd {
			best, bd = i, d
		}
	}
	if best < 0 {
		return nil
	}
	e := otherEdges[best]
	var s = -1
	for _, f := range t.EdgeFaces(e) {
		if f >= 0 && f != off.face {
			s = f
		}
	}
	if s < 0 {
		return nil
	}
	if !straightPoints(t.Edges[e].Points) {
		return t.offIntoCurved(off, s, p, n)
	}
	side := off.sides[s]
	if side != nil && side.curved != nil {
		return nil
	}
	if side == nil {
		ns, ok := t.planarNormal(s)
		if !ok {
			return nil
		}
		along, slant, ok := slabAlong(ns, n)
		if !ok {
			return nil
		}
		poly, ok := t.straightOutline(s)
		if !ok {
			return nil
		}
		side = &runOffSide{p0: poly[0], n: ns, poly: poly, past: math.Inf(-1), surf: t.Surface(s), along: along, slant: slant}
		off.sides[s] = side
	}
	side.past = math.Max(side.past, side.pastOf(p))
	return off
}

// straightPoints reports whether pts lie on the line through their
// ends.
func straightPoints(pts []geom.Vector) bool {
	if len(pts) < 2 {
		return false
	}
	a, b := pts[0], pts[len(pts)-1]
	d := b.Minus(a)
	if d.Magnitude() <= geom.LengthEps {
		return false
	}
	for _, q := range pts[1 : len(pts)-1] {
		if q.DistanceToLine(a, d) > geom.LengthEps {
			return false
		}
	}
	return true
}

// straightOutline is the corners of face f's one loop, in its order,
// when every edge of it is straight.
func (t *Topology) straightOutline(f int) ([]geom.Vector, bool) {
	var poly []geom.Vector
	for _, ce := range t.Loops[t.Faces[f].Loops[0]].CoEdges {
		pts := t.Edges[t.CoEdges[ce].Edge].Points
		if !straightPoints(pts) {
			return nil, false
		}
		if t.CoEdges[ce].Reversed {
			pts = reversed(pts)
		}
		q := pts[0]
		if k := len(poly); k >= 2 && q.Minus(poly[k-1]).Cross(poly[k-1].Minus(poly[k-2])).Magnitude() <= geom.LengthEps {
			poly[k-1] = q
			continue
		}
		poly = append(poly, q)
	}
	return poly, len(poly) >= 3
}

// justPast is the share of the fillet's size within which a foot that
// runs past a side's plane may make no sound solid: the plane that cuts
// the widened solid back meets the fillet nearly tangent along the
// foot, a sliver of the side of the square of how far past over the
// fillet's diameter, and some such cuts come out open (nearSize).
const justPast = 0.01

// filletPastSides makes the picks' fillet whose foot runs off its flat
// face as off says: on the solid widened past the sides, then cut back
// by their planes. A foot that reaches a side exactly pinches the face
// it rolls on to a point there, two faces touching (assembleTrimPolygon,
// splitTouchingHoles); one that misses the side or runs past it by no
// more than a tenth of the chord tolerance is made at the size that
// reaches it (touchingSize). A fillet whose foot runs only just past a
// side (justPast) and comes out open is made at the nearest size that
// doesn't, within the chord tolerance of r (nearSize). It leaves sh as
// it was where the result would not be sound, and gives refusal, or for
// a foot that only just runs past a side, why that is refused. made is
// the size made, r unless one near it was.
func filletPastSides(sh *srf.SShell, picks []FilletPick, r float64, origin uint32, flat bool, off *runOff, refusal error) (made float64, err error) {
	made = r
	out, ok := widenedFillet(sh, picks, r, origin, flat, off)
	if !ok {
		if rs, snap := touchingSize(sh, picks, r, off); snap {
			out, ok = widenedFillet(sh, picks, rs, origin, flat, off)
			made = rs
		}
	}
	if !ok && !flat {
		out, made, ok = nearSize(sh, picks, r, origin, off)
	}
	if ok {
		*sh = out
		return made, nil
	}
	for _, s := range off.sides {
		if s.past > 0 && s.past < justPast*r {
			what := "fillet"
			if flat {
				what = "chamfer"
			}
			var at geom.Vector
			if fx, ok := errors.AsType[*FilletError](refusal); ok {
				at = fx.At
			}
			return r, filletErr(at, "the %s's foot only just runs past the side of the face here, too little for "+
				"the side to cut it back: make it a little smaller or larger", what)
		}
	}
	for _, s := range off.sides {
		if s.curved != nil && s.curved.blocked {
			what := "fillet"
			if flat {
				what = "chamfer"
			}
			var at geom.Vector
			if fx, ok := errors.AsType[*FilletError](refusal); ok {
				at = fx.At
			}
			return r, filletErr(at, "the %s runs off the face here into a curved side, and the solid stands where that side, "+
				"carried on past the face, would cut the %s back: make it smaller", what, what)
		}
	}
	return r, refusal
}

// touchingSize is the size at which the fillet's foot reaches a side's
// plane exactly, where at r it misses it, or runs past it, by no more
// than a tenth of the chord tolerance: as close as that the Booleans
// that cut the widened solid back can't tell the foot from the side.
// The foot runs along its face at the fillet's size from the edge, so
// it moves with the size one for one. Not for a radius that varies, or
// a pick of a size of its own, nor where sides disagree.
func touchingSize(sh *srf.SShell, picks []FilletPick, r float64, off *runOff) (float64, bool) {
	near := sh.Tol.ChordTolMm() / 10
	past, ok := sidesPast(picks, off, func(p float64) bool { return math.Abs(p) <= near })
	if !ok || math.Abs(past) <= geom.LengthEps {
		return 0, false
	}
	return r - past, true
}

// sidesPast is how far the foot runs past the sides whose past within
// says, where they all agree on it: not for a radius that varies or a
// pick of a size of its own, where the foot moves with the size other
// than one for one.
func sidesPast(picks []FilletPick, off *runOff, within func(float64) bool) (float64, bool) {
	if off.varies {
		return 0, false
	}
	for _, p := range picks {
		if p.R != 0 {
			return 0, false
		}
	}
	for _, s := range off.sides {
		// Past a leaning side the foot moves with the size along the
		// face, not along the side's normal, and past a curved one not
		// one for one either.
		if s.slant || s.curved != nil {
			return 0, false
		}
	}
	var past float64
	found := false
	for _, s := range off.sides {
		if !within(s.past) {
			continue
		}
		if found && math.Abs(s.past-past) > geom.LengthEps {
			return 0, false
		}
		past, found = s.past, true
	}
	return past, found
}

// clearHeight is how far above the face, at a side's plane, a fillet's
// arc must stand for the cut along the plane to tell the fillet from
// the face: a few times LengthEps.
const clearHeight = 3 * geom.LengthEps

// nearSize makes the fillet whose foot runs only just past a side
// (justPast) at a size near r that the kernel can make, where that
// moves the fillet's surface by no more than the chord tolerance, and
// whether one was. Past the side by p, the fillet's arc stands above
// the face at the side's plane by r - sqrt(r² - p²), about p²/2r: round
// a rod of 4 through a plate 12 wide, a fillet of 2.001 stands 2.5e-7
// above the top there, under LengthEps, and the cut along the side's
// plane can't tell the fillet from the top; it dropped the top (refused
// from 2.0002 to 2.0028 at a chord setting of 0.003, 2026-11-04). The
// sizes tried, nearest first: the one at which the foot reaches the
// side exactly (the top pinched to a point there), and those at which
// the arc stands clearHeight or more above the top at the side. A
// fillet of size c in place of r moves its surface by at most
// (sqrt 2 - 1)|r - c|, at the middle of its arc. made is the size made.
func nearSize(sh *srf.SShell, picks []FilletPick, r float64, origin uint32, off *runOff) (out srf.SShell, made float64, ok bool) {
	past, ok := sidesPast(picks, off, func(p float64) bool { return p > 0 && p < justPast*r })
	if !ok {
		return srf.SShell{}, r, false
	}
	touch := r - past
	step := math.Sqrt(clearHeight * (2*touch - clearHeight))
	sizes := []float64{touch}
	for _, k := range []float64{1, 1.25, 1.5, 2} {
		sizes = append(sizes, touch+k*step)
	}
	slices.SortFunc(sizes, func(a, b float64) int { return cmp.Compare(math.Abs(a-r), math.Abs(b-r)) })
	tol := sh.Tol.ChordTolMm()
	for _, c := range sizes {
		if (math.Sqrt2-1)*math.Abs(r-c) > tol || math.Abs(r-c) <= geom.LengthEps {
			continue
		}
		if out, ok := widenedFillet(sh, picks, c, origin, false, off); ok {
			return out, c, true
		}
	}
	return srf.SShell{}, r, false
}

// widenedFillet is filletPastSides' solid, and whether it is sound.
func widenedFillet(sh *srf.SShell, picks []FilletPick, r float64, origin uint32, flat bool, off *runOff) (srf.SShell, bool) {
	wide := sh.Clone()
	var cuts []srf.SShell
	for _, s := range off.distinct() {
		if s.curved != nil {
			cut, ok := widenPastCurved(&wide, sh, s, off, r)
			if !ok {
				return srf.SShell{}, false
			}
			cuts = append(cuts, cut)
			continue
		}
		depth := math.Max(s.past, 0) + r + 1
		var slab srf.SShell
		slab.MakePrism(s.poly, geom.Vector{}, s.along.ScaledBy(depth/s.along.Dot(s.n)), off.color)
		var u srf.SShell
		u.MakeFromUnionOf(&wide, &slab)
		// The slab's top merges with the face the fillet rolls on, so
		// that the foot lies inside one face.
		u.MergeCoincidentSurfaces()
		wide = u
		cut, ok := cutBeyond(s, off, depth, r, sh)
		if !ok {
			return srf.SShell{}, false
		}
		cuts = append(cuts, cut)
	}
	if err := filletPicksWith(&wide, picks, r, origin, flat, nil, false, false, false); err != nil {
		return srf.SShell{}, false
	}
	out := wide
	for i := range cuts {
		var d srf.SShell
		d.MakeFromDifferenceOf(&out, &cuts[i])
		out = d
	}
	if unsound(&out) {
		return srf.SShell{}, false
	}
	return keepBeyond(&out, sh, off, r)
}

// cutBeyond is the box that cuts the widened solid back to side s's
// plane: from the plane outward past the slab of depth, across the
// side's breadth and from below it to above the face the fillet rolls
// on by more than the fillet's size r. Its faces take the side's
// colour and identity, so the cut on the plane is the side again.
// Where any of the solid sh lies in it, the box is cut by sh first, so
// that what goes on past the side stays (beyondCutter).
func cutBeyond(s *runOffSide, off *runOff, depth, r float64, sh *srf.SShell) (srf.SShell, bool) {
	up := off.n
	if s.slant {
		var reach float64
		up, reach = slantUp(s, off.n, r)
		r = reach
	}
	across := s.n.Cross(up)
	lo, hi := [2]float64{math.Inf(1), math.Inf(1)}, [2]float64{math.Inf(-1), math.Inf(-1)}
	for _, q := range slabCorners(s, depth) {
		for i, a := range []geom.Vector{across, up} {
			lo[i] = math.Min(lo[i], q.Dot(a))
			hi[i] = math.Max(hi[i], q.Dot(a))
		}
	}
	if s.slant {
		// The side's top is its edge on the face, the highest of it.
		hi[1] += r
	} else {
		hi[1] = math.Max(hi[1], off.at.Dot(up)+r)
	}
	for i := range 2 {
		lo[i]--
		hi[i]++
	}
	base := s.n.ScaledBy(s.p0.Dot(s.n))
	at := func(a, b float64) geom.Vector { return base.Plus(across.ScaledBy(a)).Plus(up.ScaledBy(b)) }
	far := depth + 1
	var m srf.SMesh
	c := sh.Clone()
	c.TriangulateInto(&m)
	in := func(p geom.Vector) bool {
		d := p.Minus(s.p0).Dot(s.n)
		a, b := p.Dot(across), p.Dot(up)
		return d > 10*geom.LengthEps && d < far && a > lo[0] && a < hi[0] && b > lo[1] && b < hi[1]
	}
	beyond := false
	for _, tr := range m.Triangles {
		if in(tr.A) || in(tr.B) || in(tr.C) || in(tr.A.Plus(tr.B).Plus(tr.C).ScaledBy(1.0/3)) {
			beyond = true
			break
		}
	}
	var box srf.SShell
	box.MakePrism([]geom.Vector{at(lo[0], lo[1]), at(hi[0], lo[1]), at(hi[0], hi[1]), at(lo[0], hi[1])},
		geom.Vector{}, s.n.ScaledBy(far), s.surf.Color)
	for i := range box.Surface {
		box.Surface[i].Face = s.surf.Face
		box.Surface[i].Origin = s.surf.Origin
		box.Surface[i].Sheet = s.surf.Sheet
	}
	if beyond {
		return beyondCutter(&box, sh)
	}
	return box, true
}
