package topo

import (
	"errors"
	"fmt"
	"math"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// A shell of a solid joined from parts, each of which a shell can
// hollow alone: a lathe or a revolve, a solid of flat faces, or a prism
// of lines and arcs, as a turned body with an extruded boss or a block
// with a round boss on it. The inside of the union is the union of the
// parts' insides, each part's walls moved in by the thickness, but a
// face of a part that lies inside the others, as a boss's foot sunk into
// the body, is moved out by it instead: there the part's inside reaches
// the others', so the walls round the joint keep the thickness and the
// insides meet with no wall between them. Each face's mode is found by
// where it lies on the solid: on a face taken away, on a wall, or on
// none. A part's inside moved out of the solid is refused, and so is one
// leaving a wall thinner than the thickness beyond a face moved out:
// the insides made again with every face a wall and those faces moved
// out by twice the thickness must lie in the solid.

// ShellPart is one of the solids a solid is the union of: a revolution
// (Rev), or else its own solid (Shell), of flat faces or a prism.
type ShellPart struct {
	Rev   *Revolution
	Shell *srf.SShell
}

// ShellJoined hollows sh, the union of parts, to walls thick, taking
// away faces open (indices into its topology). The faces of the cavity
// get origins from origin up.
func ShellJoined(sh *srf.SShell, parts []ShellPart, open []int, thick float64, origin uint32) (*srf.SShell, error) {
	if thick <= 0 || math.IsNaN(thick) || math.IsInf(thick, 0) {
		return nil, fmt.Errorf("a shell's thickness must be more than zero, not %g", thick)
	}
	if len(parts) == 0 {
		return nil, errors.New("there are no parts to hollow")
	}
	t, opened, err := shellFaces(sh, open)
	if err != nil {
		return nil, err
	}
	inner, buried, err := joinedInside(t, sh, parts, opened, buriedFace, thick)
	if err != nil {
		return nil, err
	}
	if buried {
		// The parts' insides with every face a wall, so that none lies on
		// a face taken away, and the faces moved out moved out by twice
		// the thickness: what of them lies outside the solid is where the
		// wall beyond a face moved out is thinner than the thickness, or
		// where it came out of the solid.
		check, _, err := joinedInside(t, sh, parts, nil, probedFace, thick)
		if err != nil {
			return nil, err
		}
		if err := insideOf(check, sh, thick); err != nil {
			return nil, err
		}
	}
	for i := range inner.Surface {
		inner.Surface[i].Origin = origin + uint32(i)
	}
	out := &srf.SShell{Tol: sh.Tol}
	out.MakeFromDifferenceOf(sh, inner)
	if out.BooleanFailed {
		return nil, errors.New("the Boolean that takes the inside away failed")
	}
	return out, nil
}

// joinedInside is the union of the insides of parts of sh (topology t),
// faces opened taken away and a part's faces lying inside the others
// moved as inside says, and whether any does.
func joinedInside(t *Topology, sh *srf.SShell, parts []ShellPart, opened map[int]bool, inside faceMode, thick float64) (*srf.SShell, bool, error) {
	buried := false
	modeAt := func(p geom.Vector) faceMode {
		f, d := t.FaceAt(p)
		switch {
		case f < 0 || d > 100*geom.LengthEps:
			buried = true
			return inside
		case opened[f]:
			return openFace
		}
		return keptFace
	}
	color := sh.Surface[0].Color
	var inner *srf.SShell
	for _, part := range parts {
		var in *srf.SShell
		var err error
		if part.Rev != nil {
			in, err = joinedRevolvedInside(*part.Rev, modeAt, thick, sh.Tol, color)
		} else {
			in, err = joinedFlatInside(part.Shell, modeAt, thick)
		}
		if err != nil {
			return nil, false, err
		}
		if inner == nil {
			inner = in
			continue
		}
		u := &srf.SShell{Tol: sh.Tol}
		u.MakeFromUnionOf(inner, in)
		if u.BooleanFailed {
			return nil, false, errors.New("the Boolean that joins the parts' insides failed")
		}
		inner = u
	}
	return inner, buried, nil
}

// joinedRevolvedInside is the inside of the revolution rev, a part of a
// solid, each piece of its profile moved as modeAt says of points of its
// faces. Its ends must lie on the solid's faces.
func joinedRevolvedInside(rev Revolution, modeAt func(geom.Vector) faceMode, thick float64, tol srf.Tolerance, color core.RGBA) (*srf.SShell, error) {
	axis := rev.Axis.WithMagnitude(1)
	// An end's mode by points inside the profile turned onto it: taken
	// away where any lies on a face taken away, a wall where any lies on
	// one.
	ends := func(radial geom.Vector) ([2]faceMode, error) {
		var mode [2]faceMode
		if rev.Whole() {
			return mode, nil
		}
		pts := profilePoints(rev.Profile)
		if len(pts) == 0 {
			return mode, ErrNotRevolution
		}
		for k, a := range []float64{rev.Start, rev.End} {
			wall, open := false, false
			for _, q := range pts {
				switch modeAt(q.RotatedAbout(rev.Pt, axis, a)) {
				case openFace:
					open = true
				case keptFace:
					wall = true
				}
			}
			switch {
			case open:
				mode[k] = openFace
			case !wall:
				return mode, shellErr(pts[0].RotatedAbout(rev.Pt, axis, a), "an end of a revolve lies inside another solid it is joined to; "+
					"a shell hollows a revolve joined to others only where its ends are walls or taken away")
			}
		}
		return mode, nil
	}
	return revolvedInside(rev, func(p geom.Vector) (faceMode, bool) { return modeAt(p), true }, ends, thick, tol, color)
}

// profilePoints are points of a grid over the profile's box, in its
// plane, that lie inside it.
func profilePoints(profile *srf.SBezierLoopSetSet) []geom.Vector {
	n := profile.Sets[0].Normal.WithMagnitude(1)
	inside := profileInside(profile, n)
	ax := n.NormalDir(0)
	ay := n.Cross(ax)
	lo, hi := profileBox(profile)
	u0, u1, v0, v1 := math.Inf(1), math.Inf(-1), math.Inf(1), math.Inf(-1)
	for _, c := range []geom.Vector{lo, hi, {X: lo.X, Y: lo.Y, Z: hi.Z}, {X: lo.X, Y: hi.Y, Z: lo.Z}, {X: hi.X, Y: lo.Y, Z: lo.Z},
		{X: lo.X, Y: hi.Y, Z: hi.Z}, {X: hi.X, Y: lo.Y, Z: hi.Z}, {X: hi.X, Y: hi.Y, Z: lo.Z}} {
		u0, u1 = min(u0, c.Dot(ax)), max(u1, c.Dot(ax))
		v0, v1 = min(v0, c.Dot(ay)), max(v1, c.Dot(ay))
	}
	w := n.ScaledBy(lo.Dot(n))
	var out []geom.Vector
	const k = 12
	for i := 1; i < k; i++ {
		for j := 1; j < k; j++ {
			p := w.Plus(ax.ScaledBy(u0 + (u1-u0)*float64(i)/k)).Plus(ay.ScaledBy(v0 + (v1-v0)*float64(j)/k))
			if inside(p) {
				out = append(out, p)
			}
		}
	}
	return out
}

// joinedFlatInside is the inside of part, a solid of flat faces or a
// prism, each face moved as modeAt says of points of it.
func joinedFlatInside(part *srf.SShell, modeAt func(geom.Vector) faceMode, thick float64) (*srf.SShell, error) {
	pt, err := Build(part)
	if err != nil {
		return nil, err
	}
	if probs := pt.Validate(); len(probs) > 0 {
		return nil, shellErr(probs[0].At, "a part of the solid is not closed: %s", probs[0].Msg)
	}
	mode := make([]faceMode, len(pt.Faces))
	for f := range pt.Faces {
		mode[f] = faceModeOf(pt, part, f, modeAt)
	}
	inner, ok, err := prismInside(pt, part, mode, thick, 0)
	if !ok {
		inner, err = flatInside(pt, part, mode, thick, 0)
	}
	return inner, err
}

// faceModeOf is face f's mode, of a part's solid of topology t, by the
// middles of its triangles on it: taken away where any lies on a face taken
// away, a wall where any lies on one, inside the others where none
// lies on the solid (moved as modeAt says of such points).
func faceModeOf(t *Topology, part *srf.SShell, f int, modeAt func(geom.Vector) faceMode) faceMode {
	var m srf.SMesh
	s := t.Surface(f)
	s.TriangulateInto(part, &m)
	wall, open := false, false
	inside := buriedFace
	for _, tr := range m.Triangles {
		// The middle, a chord's sag off a curved face, put back on it.
		var u, v float64
		s.ClosestPointToUV(tr.A.Plus(tr.B).Plus(tr.C).ScaledBy(1.0/3), &u, &v, true)
		switch modeAt(s.PointAt(u, v)) {
		case openFace:
			open = true
		case keptFace:
			wall = true
		case probedFace:
			inside = probedFace
		}
	}
	switch {
	case open:
		return openFace
	case wall:
		return keptFace
	}
	return inside
}

// insideOf is an error when inner, the insides of a solid's parts with
// every face a wall and those inside the others moved out by twice the
// thickness, comes out of sh: beyond a face moved out the wall is
// thinner than the thickness, or there is none.
func insideOf(inner, sh *srf.SShell, thick float64) error {
	out := &srf.SShell{Tol: sh.Tol}
	out.MakeFromDifferenceOf(inner, sh)
	if out.BooleanFailed {
		return errors.New("the Boolean that checks the inside lies in the solid failed")
	}
	var m srf.SMesh
	out.TriangulateInto(&m)
	if v := m.CalculateVolume(); v > geom.LengthEps*thick*thick {
		at := geom.Vector{}
		if len(m.Triangles) > 0 {
			at = m.Triangles[0].A
		}
		return shellErr(at, "a thickness of %g leaves a wall thinner than that, or none, beyond where a part's inside meets another's; "+
			"the part it meets is thinner than twice that there", thick)
	}
	return nil
}
