package topo

import (
	"math"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// Where the solid goes on past a side the fillet's foot runs into (a
// ledge below the side, a tab standing off it), the box that cuts the
// widened solid back to the side's plane would cut that away too. The
// box is cut first by the solid as it was before the fillet: what is
// left of it is only the room past the side the solid never filled,
// which holds the slab and the fillet beyond the side and nothing of
// the solid's own. Its faces where it met the solid lie on the solid's
// (the ledge's top, the tab's sides), and taking it from the filleted
// solid leaves those faces as they were.
//
// A fillet that takes material away (a hole's rim) can reach past the
// side's plane below the side's foot, into what stands there: a chamfer
// of 3.5 on the rim of a hole 2 from the side cuts 1.5 down at the
// plane, through a riser 1 high into the ledge below it. The fillet is
// made as if the face went on past its sides and cut back by their
// planes, so past a side the solid is what it was: there the solid
// before the fillet is put back (keepBeyond).

// beyondCutter is box less the solid sh before the fillet, its faces
// taking the side's identity as box's do, and whether it is sound. Where
// none of sh lies in box, box is the cutter as it is (cutBeyond).
func beyondCutter(box *srf.SShell, sh *srf.SShell) (srf.SShell, bool) {
	before := sh.Clone()
	var c srf.SShell
	c.MakeFromDifferenceOf(box, &before)
	if unsound(&c) {
		return srf.SShell{}, false
	}
	return c, true
}

// keepBeyond is out, the filleted solid cut back by the sides of off,
// with the solid sh before the fillet put back past each side where
// out differs from it there by more than the chord tolerance allows,
// and whether the result is sound. A side with nothing of sh past it is
// left as out has it.
func keepBeyond(out, sh *srf.SShell, off *runOff, r float64) (srf.SShell, bool) {
	tol := sh.Tol.ChordTolMm() * r * r
	for _, s := range off.sides {
		was := volumeBeyond(sh, s.p0, s.n)
		if was <= geom.LengthEps {
			continue
		}
		if math.Abs(volumeBeyond(out, s.p0, s.n)-was) <= tol {
			continue
		}
		big := halfSpaceBox(sh, s)
		// Joined with what of sh lies past the side, which out holds
		// all of but the fillet's cut into it: the faces they share lie
		// on each other alike. Cutting out back to the plane first and
		// joining the two there failed (the chamfer's cone meets the
		// plane along the riser's foot, 2026-11-05).
		var kept, joined srf.SShell
		before := sh.Clone()
		kept.MakeFromIntersectionOf(&before, &big)
		joined.MakeFromUnionOf(out, &kept)
		joined.MergeCoincidentSurfaces()
		if unsound(&joined) || math.Abs(volumeBeyond(&joined, s.p0, s.n)-was) > tol {
			return srf.SShell{}, false
		}
		*out = joined
	}
	return *out, true
}

// volumeBeyond is the volume of the closed solid sh past the plane
// through p0 with unit normal n. By the divergence theorem with the
// field ((p - p0)·n) n, whose divergence is 1 and which vanishes on the
// plane, it is the flux of that field through sh's mesh clipped to the
// plane, exact for flat triangles.
func volumeBeyond(sh *srf.SShell, p0, n geom.Vector) float64 {
	var m srf.SMesh
	c := sh.Clone()
	c.TriangulateInto(&m)
	h := func(p geom.Vector) float64 { return p.Minus(p0).Dot(n) }
	var v float64
	for _, tr := range m.Triangles {
		poly := clipBeyond([]geom.Vector{tr.A, tr.B, tr.C}, h)
		for i := 1; i+1 < len(poly); i++ {
			a, b, c := poly[0], poly[i], poly[i+1]
			area := b.Minus(a).Cross(c.Minus(a)).ScaledBy(0.5)
			v += (h(a) + h(b) + h(c)) / 3 * area.Dot(n)
		}
	}
	return v
}

// clipBeyond is the polygon poly clipped to where h is 0 or more.
func clipBeyond(poly []geom.Vector, h func(geom.Vector) float64) []geom.Vector {
	var out []geom.Vector
	for i, a := range poly {
		b := poly[(i+1)%len(poly)]
		ha, hb := h(a), h(b)
		if ha >= 0 {
			out = append(out, a)
		}
		if (ha >= 0) != (hb >= 0) {
			out = append(out, a.Plus(b.Minus(a).ScaledBy(ha/(ha-hb))))
		}
	}
	return out
}

// halfSpaceBox is a box from side s's plane outward past all of sh,
// and wider than sh across it, its faces taking the side's colour and
// identity as cutBeyond's do.
func halfSpaceBox(sh *srf.SShell, s *runOffSide) srf.SShell {
	// u, w and n in cutBeyond's order (across, up, n), so the prism
	// faces out as its box does.
	u := s.n.NormalDir(0)
	w := u.Cross(s.n)
	var m srf.SMesh
	c := sh.Clone()
	c.TriangulateInto(&m)
	lo, hi := [3]float64{math.Inf(1), math.Inf(1), math.Inf(1)}, [3]float64{math.Inf(-1), math.Inf(-1), math.Inf(-1)}
	for _, tr := range m.Triangles {
		for _, p := range []geom.Vector{tr.A, tr.B, tr.C} {
			for i, a := range []geom.Vector{u, w, s.n} {
				lo[i] = math.Min(lo[i], p.Dot(a))
				hi[i] = math.Max(hi[i], p.Dot(a))
			}
		}
	}
	base := s.n.ScaledBy(s.p0.Dot(s.n))
	at := func(a, b float64) geom.Vector { return base.Plus(u.ScaledBy(a)).Plus(w.ScaledBy(b)) }
	far := hi[2] - s.p0.Dot(s.n) + 1
	var box srf.SShell
	box.MakePrism([]geom.Vector{at(lo[0]-1, lo[1]-1), at(hi[0]+1, lo[1]-1), at(hi[0]+1, hi[1]+1), at(lo[0]-1, hi[1]+1)},
		geom.Vector{}, s.n.ScaledBy(far), s.surf.Color)
	for i := range box.Surface {
		box.Surface[i].Face = s.surf.Face
		box.Surface[i].Origin = s.surf.Origin
		box.Surface[i].Sheet = s.surf.Sheet
	}
	return box
}
