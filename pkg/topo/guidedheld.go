package topo

import (
	"fmt"
	"math"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// A loft held by guides and square to profiles between its ends, open
// or closed, starts from the loft held square alone: its pieces, one
// from each held profile to the next, give the sections across that the
// guides move, as the one surface of a loft held by guides alone does
// (guidedSections). The moved sections are then skinned piece by piece,
// each piece leaving and reaching a held profile square to its plane,
// so the loft holds the guides at the sections and is still smooth
// through the held profiles. Where a guide crosses a held profile
// slanting, the loft leaves the profile square and turns onto the guide
// within the twelfth of the way to the next section.

// heldPieces are the pieces of a loft held square at profiles, each
// with where it starts across the whole loft and how far it runs: iso
// is the section across at v of the whole, v taken round again past 1
// on a ring.
type heldPieces struct {
	pieces []*srf.BSplineSurface
	start  []float64
	span   []float64
}

// iso is the section of the pieces at v across the whole loft.
func (h heldPieces) iso(v float64) srf.BSplineCurve {
	for c, p := range h.pieces {
		for _, w := range []float64{v, v + 1} {
			if t := (w - h.start[c]) / h.span[c]; t >= 0 && t <= 1 {
				return p.IsoV(t)
			}
		}
	}
	panic(fmt.Sprintf("no piece of the loft holds %g across", v))
}

// heldSlope is the slope across, in homogeneous coordinates over a
// piece's own parameter of span, of a loft held square to profile loop
// of curve c: along the loop's normal, x times length long over the
// whole loft.
func heldSlope(loop []srf.SBezier, c srf.BSplineCurve, x, length, span float64) []geom.Vector4 {
	n := newell(loop).WithMagnitude(x * length * span)
	out := make([]geom.Vector4, len(c.Ctrl))
	for i := range out {
		out[i] = geom.Vector4FromWeighted(c.Weight[i], n)
		out[i].W = 0
	}
	return out
}

// endSlope is the homogeneous derivative across of bs at its start
// (at false) or its end, over its own parameter.
func endSlope(bs *srf.BSplineSurface, atEnd bool) []geom.Vector4 {
	dv := bs.DegV
	nv := len(bs.Ctrl[0])
	out := make([]geom.Vector4, len(bs.Ctrl))
	for i := range bs.Ctrl {
		h := func(j int) geom.Vector4 { return geom.Vector4FromWeighted(bs.Weight[i][j], bs.Ctrl[i][j]) }
		if atEnd {
			b := float64(dv) / (bs.KnotsV[len(bs.KnotsV)-1] - bs.KnotsV[len(bs.KnotsV)-dv-2])
			out[i] = h(nv - 1).Minus(h(nv - 2)).ScaledBy(b)
		} else {
			a := float64(dv) / (bs.KnotsV[dv+1] - bs.KnotsV[0])
			out[i] = h(1).Minus(h(0)).ScaledBy(a)
		}
	}
	return out
}

// GuidedLoftThroughSurfaces are the surfaces of a loft held by guides,
// as GuidedLoftSidesSurface's is, and square to the planes of profiles
// between its ends as SmoothLoftThroughSurfaces says by middle: one
// surface from each held profile to the next, in order from the first
// profile. With none held it is GuidedLoftSidesSurface's one surface;
// with no guides, SmoothLoftThroughSurfaces.
func GuidedLoftThroughSurfaces(loops [][]srf.SBezier, turn int, ends LoftEnds, sides LoftSides, middle []float64, guides []LoftGuide) ([]*srf.BSplineSurface, error) {
	cuts := []int{0}
	for j, x := range middle {
		if x != 0 && !math.IsNaN(x) && !math.IsInf(x, 0) {
			cuts = append(cuts, j+1)
		}
	}
	if len(guides) == 0 || len(cuts) == 1 {
		if len(guides) == 0 {
			return SmoothLoftThroughSurfaces(loops, turn, ends, sides, middle)
		}
		if err := checkMiddle(loops, middle); err != nil {
			return nil, err
		}
		bs, err := GuidedLoftSidesSurface(loops, turn, ends, sides, guides)
		if err != nil {
			return nil, err
		}
		return []*srf.BSplineSurface{bs}, nil
	}
	if len(guides) > 2 {
		return nil, fmt.Errorf("a loft takes two guides at most, not %d", len(guides))
	}
	pieces, err := SmoothLoftThroughSurfaces(loops, turn, ends, sides, middle)
	if err != nil {
		return nil, err
	}
	base, err := smoothLoftBase(loops, turn, ends, sides)
	if err != nil {
		return nil, err
	}
	k := len(base.curves)
	cuts = append(cuts, k-1)
	if len(pieces) != len(cuts)-1 {
		return nil, fmt.Errorf("the loft held square came in %d pieces, not %d", len(pieces), len(cuts)-1)
	}
	hp := heldPieces{pieces: pieces}
	for c := 1; c < len(cuts); c++ {
		hp.start = append(hp.start, base.ts[cuts[c-1]])
		hp.span = append(hp.span, base.ts[cuts[c]]-base.ts[cuts[c-1]])
	}
	gs, err := guideTracks(base, guides)
	if err != nil {
		return nil, err
	}
	sections, vs, moved, err := guidedSections(hp.iso, base.ts, base.curves, k, gs)
	if err != nil {
		return nil, err
	}
	length := 0.0
	for i := 1; i < k; i++ {
		length += centroid(base.loops[i]).Minus(centroid(base.loops[i-1])).Magnitude()
	}
	// The slopes across at the loft's ends, over the whole loft's
	// parameter: the held loft's own, and the guides' turn there too
	// unless the end is set square, as GuidedLoftSidesSurface takes them.
	last := len(sections) - 1
	start := endSlope(pieces[0], false)
	end := endSlope(pieces[len(pieces)-1], true)
	for i := range start {
		w0, w1 := start[i].W, end[i].W
		start[i] = start[i].ScaledBy(1 / hp.span[0])
		end[i] = end[i].ScaledBy(1 / hp.span[len(pieces)-1])
		if ends.Start == 0 {
			w := sections[1].Weight[i]
			start[i] = start[i].Plus(geom.Vector4FromWeighted(w, moved[1][i]).ScaledBy(1 / (vs[1] - vs[0])))
			start[i].W = w0 / hp.span[0]
		}
		if ends.End == 0 {
			w := sections[last-1].Weight[i]
			end[i] = end[i].Minus(geom.Vector4FromWeighted(w, moved[last-1][i]).ScaledBy(1 / (vs[last] - vs[last-1])))
			end[i].W = w1 / hp.span[len(pieces)-1]
		}
	}
	var out []*srf.BSplineSurface
	for c := 1; c < len(cuts); c++ {
		from, to := cuts[c-1], cuts[c]
		span := hp.span[c-1]
		var a, b []geom.Vector4
		if from == 0 {
			a = scaled(start, span)
		} else {
			a = heldSlope(base.loops[from], base.curves[from], middle[from-1], length, span)
		}
		if to == k-1 {
			b = scaled(end, span)
		} else {
			b = heldSlope(base.loops[to], base.curves[to], middle[to-1], length, span)
		}
		lo, hi := from*guideSamples, to*guideSamples
		ts := make([]float64, 0, hi-lo+1)
		for s := lo; s <= hi; s++ {
			ts = append(ts, (vs[s]-vs[lo])/span)
		}
		ts[len(ts)-1] = 1
		piece, err := srf.SkinThrough(sections[lo:hi+1], ts, a, b)
		if err != nil {
			return nil, fmt.Errorf("from profile %d to %d: %w", from+1, to+1, err)
		}
		out = append(out, piece)
	}
	return out, nil
}

// MakeGuidedLoftThrough is the shell of GuidedLoftThroughSurfaces, all
// one face with its two caps, as MakeSmoothLoftThrough's.
func MakeGuidedLoftThrough(loops [][]srf.SBezier, turn int, ends LoftEnds, sides LoftSides, middle []float64, guides []LoftGuide, color core.RGBA, origin uint32, tol srf.Tolerance) (*srf.SShell, error) {
	bss, err := GuidedLoftThroughSurfaces(loops, turn, ends, sides, middle, guides)
	if err != nil {
		return nil, err
	}
	return closeSkins([][]*srf.BSplineSurface{bss}, color, origin, origin+1, origin+2, tol)
}

// checkMiddle is why middle can't hold a loft through loops, or nil.
func checkMiddle(loops [][]srf.SBezier, middle []float64) error {
	if len(middle) > max(len(loops)-2, 0) {
		return fmt.Errorf("a loft of %d profiles has %d between its ends, not %d", len(loops), max(len(loops)-2, 0), len(middle))
	}
	for _, x := range middle {
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return fmt.Errorf("a loft's tangency at a profile is a number, not %g", x)
		}
	}
	return nil
}

// guideTracks finds where each guide meets the open loft base's
// profiles, as GuidedLoftSidesSurface does.
func guideTracks(base loftBase, guides []LoftGuide) ([]*guideTrack, error) {
	gs := make([]*guideTrack, len(guides))
	for i, g := range guides {
		var err error
		if gs[i], err = newGuideTrack(base, g, i+1); err != nil {
			return nil, err
		}
	}
	return gs, twoGuidesApart(gs, len(base.curves))
}

// twoGuidesApart is why two guides can't hold a loft of k profiles,
// meeting one of them at one point, or nil.
func twoGuidesApart(gs []*guideTrack, k int) error {
	if len(gs) != 2 {
		return nil
	}
	for i := range k {
		if gs[0].points[i].Minus(gs[1].points[i]).Magnitude() < guideTol {
			return fmt.Errorf("the two guides meet profile %d at one point", i+1)
		}
	}
	return nil
}

// scaled is each of vs scaled by f.
func scaled(vs []geom.Vector4, f float64) []geom.Vector4 {
	out := make([]geom.Vector4, len(vs))
	for i, v := range vs {
		out[i] = v.ScaledBy(f)
	}
	return out
}

// ClosedGuidedLoftSurfaces are the surfaces of a closed loft held by
// guides going round it, as ClosedGuidedLoftSurface's is, and square to
// profiles as ClosedLoftSurfaces says by held: one surface from each
// held profile round to the next. With none held it is
// ClosedGuidedLoftSurface's one surface; with no guides,
// ClosedLoftSurfaces.
func ClosedGuidedLoftSurfaces(loops [][]srf.SBezier, held []float64, guides []LoftGuide) ([]*srf.BSplineSurface, error) {
	var at []int
	for j, x := range held {
		if x != 0 && !math.IsNaN(x) && !math.IsInf(x, 0) {
			at = append(at, j)
		}
	}
	if len(guides) == 0 || len(at) == 0 {
		if len(guides) == 0 {
			return ClosedLoftSurfaces(loops, held)
		}
		if _, err := ClosedLoftSurfaces(loops, held); err != nil {
			return nil, err
		}
		bs, err := ClosedGuidedLoftSurface(loops, guides)
		if err != nil {
			return nil, err
		}
		return []*srf.BSplineSurface{bs}, nil
	}
	if len(guides) > 2 {
		return nil, fmt.Errorf("a loft takes two guides at most, not %d", len(guides))
	}
	pieces, err := ClosedLoftSurfaces(loops, held)
	if err != nil {
		return nil, err
	}
	r, err := closedLoftRing(loops)
	if err != nil {
		return nil, err
	}
	k := len(r.loops)
	// Each piece runs from a held profile round to the next, past the
	// seam where it lies between them.
	hp := heldPieces{pieces: pieces}
	to := make([]int, len(at))
	for c, from := range at {
		to[c] = at[(c+1)%len(at)]
		if to[c] <= from {
			to[c] += k
		}
		hp.start = append(hp.start, r.ts[from])
		hp.span = append(hp.span, float64(to[c]/k)+r.ts[to[c]%k]-r.ts[from])
	}
	gs := make([]*guideTrack, len(guides))
	for i, g := range guides {
		if gs[i], err = newClosedGuideTrack(r, g, i+1); err != nil {
			return nil, err
		}
	}
	if err := twoGuidesApart(gs, k); err != nil {
		return nil, err
	}
	sections, vs, _, err := guidedSections(hp.iso, r.ts, r.curves, k, gs)
	if err != nil {
		return nil, err
	}
	length := 0.0
	for i := range k {
		length += centroid(r.loops[(i+1)%k]).Minus(centroid(r.loops[i])).Magnitude()
	}
	// The sections round the ring, the last the first again: section s
	// past the seam is s less a whole ring's, one further round.
	ring := k * guideSamples
	var out []*srf.BSplineSurface
	for c, from := range at {
		span := hp.span[c]
		lo, hi := from*guideSamples, to[c]*guideSamples
		secs := make([]srf.BSplineCurve, 0, hi-lo+1)
		ts := make([]float64, 0, hi-lo+1)
		for s := lo; s <= hi; s++ {
			secs = append(secs, sections[s%ring])
			ts = append(ts, (vs[s%ring]+float64(s/ring)-vs[lo])/span)
		}
		ts[len(ts)-1] = 1
		e := to[c] % k
		piece, err := srf.SkinThrough(secs, ts,
			heldSlope(r.loops[from], r.curves[from], held[from], length, span),
			heldSlope(r.loops[e], r.curves[e], held[e], length, span))
		if err != nil {
			return nil, fmt.Errorf("from profile %d to %d: %w", from+1, e+1, err)
		}
		out = append(out, piece)
	}
	return out, nil
}

// MakeClosedGuidedLoftHeld is the shell of ClosedGuidedLoftSurfaces, as
// MakeClosedLoft's: one face of every piece's patches, no caps.
func MakeClosedGuidedLoftHeld(loops [][]srf.SBezier, held []float64, guides []LoftGuide, color core.RGBA, origin uint32, tol srf.Tolerance) (*srf.SShell, error) {
	bss, err := ClosedGuidedLoftSurfaces(loops, held, guides)
	if err != nil {
		return nil, err
	}
	return joinSkins([][]*srf.BSplineSurface{bss}, color, origin, tol)
}
