package topo

import (
	"errors"
	"fmt"
	"math"
	"slices"

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
// open (indices into its topology). The faces of the cavity get origins
// from origin up.
func ShellDrafted(sh *srf.SShell, d DraftedSolid, open []int, thick float64, origin uint32) (*srf.SShell, error) {
	if thick <= 0 || math.IsNaN(thick) || math.IsInf(thick, 0) {
		return nil, fmt.Errorf("a shell's thickness must be more than zero, not %g", thick)
	}
	t, opened, err := shellFaces(sh, open)
	if err != nil {
		return nil, err
	}
	modeOf := func(f int) faceMode {
		if opened[f] {
			return openFace
		}
		return keptFace
	}
	inner, err := draftedInsideOf(t, d, modeOf, thick, sh.Tol)
	if err != nil {
		return nil, err
	}
	for i := range inner.Surface {
		inner.Surface[i].Color = sh.Surface[0].Color
		inner.Surface[i].Origin = origin + uint32(i)
	}
	out := &srf.SShell{Tol: sh.Tol}
	out.MakeFromDifferenceOf(sh, inner)
	if out.BooleanFailed {
		return nil, errors.New("the Boolean that takes the inside away failed")
	}
	return out, nil
}

// draftedInsideOf is the inside of d, whose solid has topology t, each
// face moved as modeOf says: a wall moved in square to itself, a face
// taken away left where it is (a curved side, which the Boolean can't
// take away where it lies on the solid's, run out past it), a face
// lying inside another part moved out, and a face probed moved out by
// twice the thickness. A side is moved as the most of its faces says:
// taken away where any is, a wall where any is one.
func draftedInsideOf(t *Topology, d DraftedSolid, modeOf func(f int) faceMode, thick float64, tol srf.Tolerance) (*srf.SShell, error) {
	if len(d.Sets) == 0 || len(d.Sets[0].Loops) == 0 || len(d.Sets[0].Loops[0]) == 0 || d.Dir.Magnitude() == 0 {
		return nil, ErrNotDrafted
	}
	dir := d.Dir.WithMagnitude(1)
	level := d.Sets[0].Loops[0][0].Start().Dot(dir)
	// The ends' modes, from the flat faces square to dir at their levels.
	var ends [2][]faceMode
	for f := range t.Faces {
		n, flat := t.planarNormal(f)
		if !flat || math.Abs(math.Abs(n.Dot(dir))-1) > 1e-9 {
			continue
		}
		switch h := t.faceAnchor(f).Dot(dir) - level; {
		case math.Abs(h-d.Lo) < geom.LengthEps:
			ends[0] = append(ends[0], modeOf(f))
		case math.Abs(h-d.Hi) < geom.LengthEps:
			ends[1] = append(ends[1], modeOf(f))
		}
	}
	zlo, zhi := d.Lo+endShift(ends[0], thick), d.Hi-endShift(ends[1], thick)
	if zhi-zlo <= geom.LengthEps {
		return nil, shellErr(t.faceAnchor(0), "a thickness of %g makes the walls meet, closing up the inside", thick)
	}
	sides, err := draftedSides(t, d, modeOf)
	if err != nil {
		return nil, err
	}

	var inner *srf.SShell
	for k, set := range d.Sets {
		// Each way from the neutral plane the inside reaches: where it
		// starts and ends, as distances from that plane. A solid on one
		// side of the plane only has its inside carried on past it the
		// way its sides go, where an end moves out.
		type way struct{ way, from, to float64 }
		var ways []way
		switch {
		case d.Lo > -geom.LengthEps:
			ways = []way{{1, zlo, zhi}}
		case d.Hi < geom.LengthEps:
			ways = []way{{-1, -zhi, -zlo}}
		default:
			ways = []way{{-1, -min(zhi, 0), -zlo}, {1, max(zlo, 0), zhi}}
		}
		for _, w := range ways {
			if w.to-w.from <= geom.LengthEps {
				continue
			}
			part, err := draftedInside(set, sides[k], dir, w.way, w.from, w.to, thick, tol)
			if err != nil {
				return nil, err
			}
			if inner == nil {
				inner = part
				continue
			}
			u := &srf.SShell{Tol: tol}
			u.MakeFromUnionOf(inner, part)
			if u.BooleanFailed {
				return nil, errors.New("the Boolean that joins the inside's drafted parts failed")
			}
			inner = u
		}
	}
	if inner == nil {
		return nil, ErrNotDrafted
	}
	return inner, nil
}

// endShift is how far an end whose faces have modes moves in: by the
// thickness as a wall, not at all taken away, out by it lying inside
// another part, by twice that probed.
func endShift(modes []faceMode, thick float64) float64 {
	switch strongest(modes) {
	case openFace:
		return 0
	case buriedFace:
		return -thick
	case probedFace:
		return -2 * thick
	}
	return thick
}

// strongest is the mode of a side or end of faces of modes: taken away
// where any is, a wall where any is one, else probed where any is,
// else inside another part. A wall with no faces.
func strongest(modes []faceMode) faceMode {
	if len(modes) == 0 {
		return keptFace
	}
	has := map[faceMode]bool{}
	for _, m := range modes {
		has[m] = true
	}
	for _, m := range []faceMode{openFace, keptFace, probedFace} {
		if has[m] {
			return m
		}
	}
	return buriedFace
}

// draftedSides is, of each set's loops' curves, how far in units of the
// thickness its side moves in square to itself, found from the faces of
// t its side lies on, at points between its ends and the solid's.
func draftedSides(t *Topology, d DraftedSolid, modeOf func(f int) faceMode) ([][][]float64, error) {
	dir := d.Dir.WithMagnitude(1)
	out := make([][][]float64, len(d.Sets))
	for k, set := range d.Sets {
		out[k] = make([][]float64, len(set.Loops))
		for l, loop := range set.Loops {
			if len(set.Angles) <= l || len(set.Angles[l]) != len(loop) {
				return nil, ErrNotDrafted
			}
			// The material on the left going along dir, as draftedInside
			// has it, for the way into it from each curve.
			oriented := loop
			flip := (newell(loop).Dot(dir) > 0) != (l == 0)
			if flip {
				oriented = reverseLoop(loop)
			}
			pieces, err := loopPieces(oriented, dir)
			if err != nil {
				return nil, err
			}
			ms := make([]float64, len(loop))
			for i, sb := range loop {
				pi := i
				if flip {
					pi = len(loop) - 1 - i
				}
				p := pieces[pi]
				tan := math.Tan(set.Angles[l][i])
				var modes []faceMode
				for _, h := range []float64{0.25, 0.5, 0.75} {
					s := d.Lo + (d.Hi-d.Lo)*h
					for _, u := range []float64{0.25, 0.5, 0.75} {
						q := sb.PointAt(u)
						into := p.into
						if p.arc {
							into = q.Minus(p.centre).WithMagnitude(1)
							if !p.outward {
								into = into.ScaledBy(-1)
							}
						}
						q = q.Plus(dir.ScaledBy(s)).Plus(into.ScaledBy(math.Abs(s) * tan))
						f, dist := t.FaceAt(q)
						if f < 0 || dist > 100*geom.LengthEps {
							return nil, ErrNotDrafted
						}
						modes = append(modes, modeOf(f))
					}
				}
				switch strongest(modes) {
				case keptFace:
					ms[i] = 1
				case buriedFace:
					ms[i] = -1
				case probedFace:
					ms[i] = -2
				case openFace:
					if p.arc {
						// Run out past the solid's side, which it would lie
						// on: not where it meets a neighbour at an inside
						// corner, which running out would cut into.
						p.mid = sb.PointAt(0.5)
						pieces[pi] = p
						if err := outsideCorners(pieces, pi, dir); err != nil {
							return nil, err
						}
						ms[i] = -1
					}
				}
			}
			out[k][l] = ms
		}
	}
	return out, nil
}

// draftedInside is the inside of set's drafted prism going way along
// dir from the neutral plane, from distance from to distance to: the
// profile offset by moves·thick/cos α and by from·tan α, drafted at the
// same angles.
func draftedInside(set DraftedSet, moves [][]float64, dir geom.Vector, way, from, to, thick float64, tol srf.Tolerance) (*srf.SShell, error) {
	along := dir.ScaledBy(way)
	what := fmt.Sprintf("a thickness of %g", thick)
	var loops [][]srf.SBezier
	var angles [][]float64
	for k, loop := range set.Loops {
		if len(set.Angles) <= k || len(set.Angles[k]) != len(loop) || len(moves) <= k || len(moves[k]) != len(loop) {
			return nil, ErrNotDrafted
		}
		as, ms := set.Angles[k], moves[k]
		// As DraftedPrismAngles has them: the outer loop counter-clockwise
		// about the way it goes, the holes clockwise, the material on the
		// left of each.
		if nl := newell(loop); (nl.Dot(along) > 0) != (k == 0) {
			loop = reverseLoop(loop)
			as, ms = slices.Clone(as), slices.Clone(ms)
			slices.Reverse(as)
			slices.Reverse(ms)
		}
		pieces, err := loopPieces(loop, along)
		if err != nil {
			return nil, err
		}
		ds := make([]float64, len(pieces))
		for i, a := range as {
			ds[i] = ms[i]*thick/math.Cos(a) + from*math.Tan(a)
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
