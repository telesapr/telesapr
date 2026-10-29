package topo

import (
	"errors"
	"fmt"
	"math"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// A shell of a drafted extrusion whose sides are cones, the drafted
// sides of arcs, which ShellSolid's flat faces and prisms don't take.
// A side leaning by α, moved square to itself by the thickness t, is the
// same plane or cone moved t/cos α across in any plane square to the
// extrusion: so the inside is the drafted extrusion, with the same
// angles, of the profile offset into its material by t/cos α plus how
// far that profile has leant in at the level the inside starts at,
// between the ends moved in. A two-sided draft is two such prisms from
// its neutral plane, joined, as the solid is (pkg/sketch draft.go).

// DraftedSolid is how a drafted extrusion was made: the loop sets of
// its profile on the neutral plane, and how far it reaches from that
// plane along Dir, from Lo ≤ 0 to Hi ≥ 0 (DraftedPrismAngles).
type DraftedSolid struct {
	Sets   []DraftedSet
	Dir    geom.Vector
	Lo, Hi float64
}

// DraftedSet is one loop set of a drafted profile: its loops, the outer
// one first, and the angle in radians of the side of each loop's curves.
type DraftedSet struct {
	Loops  [][]srf.SBezier
	Angles [][]float64
}

// ErrNotDrafted is ShellDrafted's error when the solid's faces are not
// those of the drafted extrusion it was given.
var ErrNotDrafted = errors.New("the solid is not one drafted extrusion of its profile")

// ShellDrafted hollows sh, which is d, to walls thick, taking away faces
// open (indices into its topology), which must be its ends. The faces of
// the cavity get origins from origin up.
func ShellDrafted(sh *srf.SShell, d DraftedSolid, open []int, thick float64, origin uint32) (*srf.SShell, error) {
	if thick <= 0 || math.IsNaN(thick) || math.IsInf(thick, 0) {
		return nil, fmt.Errorf("a shell's thickness must be more than zero, not %g", thick)
	}
	if len(d.Sets) == 0 || len(d.Sets[0].Loops) == 0 || len(d.Sets[0].Loops[0]) == 0 || d.Dir.Magnitude() == 0 {
		return nil, ErrNotDrafted
	}
	t, opened, err := shellFaces(sh, open)
	if err != nil {
		return nil, err
	}
	dir := d.Dir.WithMagnitude(1)
	level := d.Sets[0].Loops[0][0].Start().Dot(dir)
	openLo, openHi := false, false
	for f := range t.Faces {
		n, flat := t.planarNormal(f)
		if !flat || math.Abs(math.Abs(n.Dot(dir))-1) > 1e-9 {
			if opened[f] {
				return nil, shellErr(t.faceAnchor(f), "a shell of a drafted extrusion with curved sides is open only at its ends")
			}
			continue
		}
		switch h := t.faceAnchor(f).Dot(dir) - level; {
		case math.Abs(h-d.Lo) < geom.LengthEps:
			openLo = openLo || opened[f]
		case math.Abs(h-d.Hi) < geom.LengthEps:
			openHi = openHi || opened[f]
		case opened[f]:
			return nil, shellErr(t.faceAnchor(f), "a shell of a drafted extrusion with curved sides is open only at its ends")
		}
	}
	zlo, zhi := d.Lo+thick, d.Hi-thick
	if openLo {
		zlo = d.Lo
	}
	if openHi {
		zhi = d.Hi
	}
	if zhi-zlo <= geom.LengthEps {
		return nil, shellErr(t.faceAnchor(0), "a thickness of %g makes the walls meet, closing up the inside", thick)
	}

	color := sh.Surface[0].Color
	inner := &srf.SShell{Tol: sh.Tol}
	first := true
	for _, set := range d.Sets {
		// Each way from the neutral plane the inside reaches: where it
		// starts and ends, as distances from that plane.
		for _, way := range []float64{-1, 1} {
			from, to := max(zlo, 0), zhi
			if way < 0 {
				from, to = -min(zhi, 0), -zlo
			}
			if to-from <= geom.LengthEps {
				continue
			}
			part, err := draftedInside(set, dir, way, from, to, thick, sh.Tol)
			if err != nil {
				return nil, err
			}
			if first {
				inner, first = part, false
				continue
			}
			u := &srf.SShell{Tol: sh.Tol}
			u.MakeFromUnionOf(inner, part)
			if u.BooleanFailed {
				return nil, errors.New("the Boolean that joins the inside's drafted parts failed")
			}
			inner = u
		}
	}
	for i := range inner.Surface {
		inner.Surface[i].Color = color
		inner.Surface[i].Origin = origin + uint32(i)
	}
	out := &srf.SShell{Tol: sh.Tol}
	out.MakeFromDifferenceOf(sh, inner)
	if out.BooleanFailed {
		return nil, errors.New("the Boolean that takes the inside away failed")
	}
	return out, nil
}

// draftedInside is the inside of set's drafted prism going way along
// dir from the neutral plane, from distance from to distance to: the
// profile offset by thick/cos α and by from·tan α, drafted at the same
// angles.
func draftedInside(set DraftedSet, dir geom.Vector, way, from, to, thick float64, tol srf.Tolerance) (*srf.SShell, error) {
	along := dir.ScaledBy(way)
	what := fmt.Sprintf("a thickness of %g", thick)
	var loops [][]srf.SBezier
	var angles [][]float64
	for k, loop := range set.Loops {
		if len(set.Angles) <= k || len(set.Angles[k]) != len(loop) {
			return nil, ErrNotDrafted
		}
		as := set.Angles[k]
		// As DraftedPrismAngles has them: the outer loop counter-clockwise
		// about the way it goes, the holes clockwise, the material on the
		// left of each.
		if nl := newell(loop); (nl.Dot(along) > 0) != (k == 0) {
			loop = reverseLoop(loop)
			as = append([]float64(nil), as...)
			for i, j := 0, len(as)-1; i < j; i, j = i+1, j-1 {
				as[i], as[j] = as[j], as[i]
			}
		}
		pieces, err := loopPieces(loop, along)
		if err != nil {
			return nil, err
		}
		ds := make([]float64, len(pieces))
		for i, a := range as {
			ds[i] = thick/math.Cos(a) + from*math.Tan(a)
		}
		keep, corner, err := offsetProfile(pieces, along, ds, what)
		if err != nil {
			return nil, err
		}
		var out []srf.SBezier
		var outAngles []float64
		for j, i := range keep {
			curves, err := movedCurves(pieces[i], corner[j], corner[(j+1)%len(keep)], along, 0)
			if err != nil {
				return nil, shellErr(pieces[i].a, "%s makes the walls meet, closing up the inside", what)
			}
			for c := range curves {
				for j := 0; j <= curves[c].Degree; j++ {
					curves[c].Ctrl[j] = curves[c].Ctrl[j].Plus(along.ScaledBy(from))
				}
				outAngles = append(outAngles, as[i])
			}
			out = append(out, curves...)
		}
		loops = append(loops, out)
		angles = append(angles, outAngles)
	}
	part, err := DraftedPrismAngles(loops, angles, along.ScaledBy(to-from), core.RGBA{}, 0, tol)
	if err != nil {
		return nil, fmt.Errorf("the inside can't be drafted: %w", err)
	}
	return part, nil
}

// shellFaces is the topology of sh, closed, and which of its faces open
// takes away: some, not all.
func shellFaces(sh *srf.SShell, open []int) (*Topology, map[int]bool, error) {
	t, err := Build(sh)
	if err != nil {
		return nil, nil, err
	}
	if probs := t.Validate(); len(probs) > 0 {
		return nil, nil, shellErr(probs[0].At, "the solid is not closed: %s", probs[0].Msg)
	}
	opened := map[int]bool{}
	for _, f := range open {
		if f < 0 || f >= len(t.Faces) {
			return nil, nil, fmt.Errorf("there is no face %d of the solid", f)
		}
		opened[f] = true
	}
	if len(opened) == 0 {
		return nil, nil, errors.New("there are no faces to take away: a shell is open where faces are taken away")
	}
	if len(opened) == len(t.Faces) {
		return nil, nil, errors.New("every face is taken away, which leaves no wall")
	}
	return t, opened, nil
}
