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
//
// A part taken away from the solid before it, as a round pocket or a
// lathed groove cut into a block, takes away from the inside that part
// grown by the thickness, every face moved out but those on a face taken
// away: the solid moved in by t, less the part, is the inside moved in,
// less the part moved out. A part the solid before it is intersected
// with intersects the inside with that part's inside, every face moved
// in. The parts are taken in order, as the solid was made. A face of a
// part joined by a union lies inside the others where it lies on none of
// the union of those parts' faces; where it lies on that union's faces
// and not on the solid's, a part taken away or intersected with took it
// away, and it is moved in as a wall, which takes nothing from the
// solid.

// ShellPart is one of the solids a solid is made of: a revolution
// (Rev), or else its own solid (Shell), of flat faces, a prism, or a
// drafted extrusion when Draft says how it was drafted, and how it is
// joined to the parts before it (srf.SurfaceUnion, srf.SurfaceDiff or
// srf.SurfaceInter; zero is a union). The first part is joined by a
// union.
type ShellPart struct {
	Rev     *Revolution
	Shell   *srf.SShell
	Draft   *DraftedSolid
	Combine srf.SurfaceCombineAs
}

// combine is how the part is joined, a union for zero.
func (p ShellPart) combine() srf.SurfaceCombineAs {
	if p.Combine == 0 {
		return srf.SurfaceUnion
	}
	return p.Combine
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

// joinedInside is the inside of sh (topology t), made of parts, faces
// opened taken away and a part's faces lying inside the other parts
// joined by unions moved as inside says, and whether any does.
func joinedInside(t *Topology, sh *srf.SShell, parts []ShellPart, opened map[int]bool, inside faceMode, thick float64) (*srf.SShell, bool, error) {
	onUnion, err := unionTopology(sh, t, parts)
	if err != nil {
		return nil, false, err
	}
	buried := false
	onSolid := func(p geom.Vector) faceMode {
		f, d := t.FaceAt(p)
		switch {
		case f < 0 || d > 100*geom.LengthEps:
			return -1
		case opened[f]:
			return openFace
		}
		return keptFace
	}
	// A part joined by a union: inside the others where it lies on no
	// face of their union, a wall where a later part took it away.
	unionMode := func(p geom.Vector) faceMode {
		if m := onSolid(p); m >= 0 {
			return m
		}
		if f, d := onUnion.FaceAt(p); f >= 0 && d <= 100*geom.LengthEps {
			return keptFace
		}
		buried = true
		return inside
	}
	// A part taken away is grown, a part intersected with moved in: the
	// faces taken away it lies on are found from the points of those
	// faces (joinedFlatInside), as a point of the part's face may lie on
	// the edge of one, where the part runs on past it.
	grownMode := func(geom.Vector) faceMode { return buriedFace }
	shrunkMode := func(geom.Vector) faceMode { return keptFace }
	openPts := openPoints(t, sh, opened)
	// A face of a part taken away or intersected with that the solid is
	// open at runs out past the solid's face, into the part taken away or
	// out of the part intersected with: left on it, the inside's face
	// would lie on the solid's, which the Boolean that takes it away
	// fails to cut where curved faces meet it (a ball cut flat).
	openedAs := map[srf.SurfaceCombineAs]faceMode{srf.SurfaceUnion: openFace, srf.SurfaceDiff: keptFace, srf.SurfaceInter: buriedFace}
	color := sh.Surface[0].Color
	var inner *srf.SShell
	for k, part := range parts {
		op := part.combine()
		if k == 0 && op != srf.SurfaceUnion {
			return nil, false, errors.New("the first part of a solid must be joined by a union")
		}
		modeAt := unionMode
		switch op {
		case srf.SurfaceDiff:
			modeAt = grownMode
		case srf.SurfaceInter:
			modeAt = shrunkMode
		}
		var in *srf.SShell
		switch {
		case part.Rev != nil && op == srf.SurfaceUnion:
			in, err = joinedRevolvedInside(*part.Rev, modeAt, nil, 0, thick, sh.Tol, color)
		case part.Rev != nil:
			var openAt func(geom.Vector) bool
			if openAt, err = openOnRevolution(*part.Rev, openPts, sh.Tol, color); err != nil {
				return nil, false, err
			}
			if op == srf.SurfaceDiff {
				in, err = grownRevolution(*part.Rev, openAt, thick, sh.Tol, color)
			} else {
				// Intersected with, a piece is taken away where any of its
				// faces is, as on a revolution alone: a curved face runs out
				// past the solid's.
				in, err = joinedRevolvedInside(*part.Rev, modeAt, openAt, openFace, thick, sh.Tol, color)
			}
		case part.Draft != nil:
			in, err = joinedDraftedInside(part.Shell, *part.Draft, modeAt, openPts, openedAs[op], thick)
		default:
			in, err = joinedFlatInside(part.Shell, modeAt, openPts, openedAs[op], thick)
		}
		if err != nil {
			return nil, false, err
		}
		if inner == nil {
			inner = in
			continue
		}
		u := &srf.SShell{Tol: sh.Tol}
		u.MakeFromBoolean(inner, in, op)
		if u.BooleanFailed {
			return nil, false, fmt.Errorf("the Boolean that joins the parts' insides failed (%v)", op)
		}
		inner = u
	}
	return inner, buried, nil
}

// unionTopology is the topology of the union of parts joined by unions,
// t when every part is.
func unionTopology(sh *srf.SShell, t *Topology, parts []ShellPart) (*Topology, error) {
	if allUnions(parts) {
		return t, nil
	}
	var u *srf.SShell
	for _, part := range parts {
		if part.combine() != srf.SurfaceUnion {
			continue
		}
		s := part.Shell
		if part.Rev != nil {
			s = part.Rev.solid(sh.Tol, sh.Surface[0].Color)
		}
		if u == nil {
			u = s
			continue
		}
		next := &srf.SShell{Tol: sh.Tol}
		next.MakeFromUnionOf(u, s)
		if next.BooleanFailed {
			return nil, errors.New("the Boolean that joins the solid's parts failed")
		}
		u = next
	}
	return Build(u)
}

// allUnions reports whether every part is joined by a union.
func allUnions(parts []ShellPart) bool {
	for _, p := range parts {
		if p.combine() != srf.SurfaceUnion {
			return false
		}
	}
	return true
}

// joinedRevolvedInside is the inside of the revolution rev, a part of a
// solid, each piece of its profile moved as modeAt says of points of its
// faces, but as opened says where openAt (if not nil) reports a face
// taken away. Its ends must lie on the solid's faces, or be taken away.
func joinedRevolvedInside(rev Revolution, modeAt func(geom.Vector) faceMode, openAt func(geom.Vector) bool, opened faceMode,
	thick float64, tol srf.Tolerance, color core.RGBA) (*srf.SShell, error) {
	axis := rev.Axis.WithMagnitude(1)
	isOpen := func(p geom.Vector) bool { return openAt != nil && openAt(p) }
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
			wall, open, probed := false, false, false
			for _, q := range pts {
				q = q.RotatedAbout(rev.Pt, axis, a)
				if isOpen(q) {
					open = true
					continue
				}
				switch modeAt(q) {
				case openFace:
					open = true
				case keptFace:
					wall = true
				case probedFace:
					probed = true
				}
			}
			switch {
			case open:
				mode[k] = openFace
			case wall:
			case probed:
				mode[k] = probedFace
			default:
				// Inside another part: moved out, so the insides meet.
				mode[k] = buriedFace
			}
		}
		return mode, nil
	}
	pieceAt := func(p geom.Vector) (faceMode, bool) {
		if isOpen(p) {
			return opened, true
		}
		return modeAt(p), true
	}
	return revolvedInside(rev, pieceAt, ends, thick, tol, color)
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
//
// A face of the part a face taken away lies on is moved as opened says,
// though no point of the part's face it was asked about lies on it: a
// part intersected with can be larger than the solid by far. open are
// points of the faces taken away.
func joinedFlatInside(part *srf.SShell, modeAt func(geom.Vector) faceMode, open []openPoint, opened faceMode, thick float64) (*srf.SShell, error) {
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
	for _, o := range open {
		if f := onFaceOf(pt, o); f >= 0 {
			mode[f] = opened
		}
	}
	inner, ok, err := prismInside(pt, part, mode, thick, 0)
	if !ok {
		inner, err = flatInside(pt, part, mode, thick, 0)
	}
	return inner, err
}

// joinedDraftedInside is the inside of part, the drafted extrusion d,
// each face moved as modeAt says of points of it, or as opened says
// where a face taken away (a point of open) lies on it.
func joinedDraftedInside(part *srf.SShell, d DraftedSolid, modeAt func(geom.Vector) faceMode, open []openPoint, opened faceMode, thick float64) (*srf.SShell, error) {
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
	for _, o := range open {
		if f := onFaceOf(pt, o); f >= 0 {
			mode[f] = opened
		}
	}
	inner, err := draftedInsideOf(pt, d, func(f int) faceMode { return mode[f] }, thick, part.Tol)
	if err != nil {
		return nil, err
	}
	for i := range inner.Surface {
		inner.Surface[i].Color = part.Surface[0].Color
	}
	return inner, nil
}

// faceModeOf is face f's mode, of a part's solid of topology t, by
// points of its triangles on it: taken away where any lies on a face taken
// away, a wall where any lies on one, inside the others where none
// lies on the solid (moved as modeAt says of such points).
func faceModeOf(t *Topology, part *srf.SShell, f int, modeAt func(geom.Vector) faceMode) faceMode {
	var m srf.SMesh
	s := t.Surface(f)
	s.TriangulateInto(part, &m)
	wall, open := false, false
	inside := buriedFace
	// Points across each triangle of a flat face of few, not its middle
	// alone: such a face is two or three large triangles, whose middles
	// can all lie where another part stands on it. Other faces are asked
	// at their triangles' middles, each put back on the face, a chord's
	// sag off a curved one.
	samples := triangleSamples[:1]
	if _, flat := t.planarNormal(f); flat && len(m.Triangles) <= 32 {
		samples = triangleSamples
	}
	for _, tr := range m.Triangles {
		for _, w := range samples {
			var u, v float64
			p := tr.A.ScaledBy(w[0]).Plus(tr.B.ScaledBy(w[1])).Plus(tr.C.ScaledBy(w[2]))
			s.ClosestPointToUV(p, &u, &v, true)
			switch modeAt(s.PointAt(u, v)) {
			case openFace:
				open = true
			case keptFace:
				wall = true
			case probedFace:
				inside = probedFace
			}
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

// triangleSamples are the barycentric weights of points inside a
// triangle, a grid of sixths, the middle first.
var triangleSamples = func() [][3]float64 {
	out := [][3]float64{{1.0 / 3, 1.0 / 3, 1.0 / 3}}
	for a := 1; a <= 4; a++ {
		for b := 1; a+b <= 5; b++ {
			if a == 2 && b == 2 {
				continue
			}
			out = append(out, [3]float64{float64(a) / 6, float64(b) / 6, float64(6-a-b) / 6})
		}
	}
	return out
}()

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

// onFaceOf is the face of t the open point o lies on, along it, or -1.
func onFaceOf(t *Topology, o openPoint) int {
	f, d := t.FaceAt(o.p)
	if f < 0 || d > 100*geom.LengthEps {
		return -1
	}
	// On the face, not across it: a point of a face taken away can lie
	// on a side running up from its edge.
	s := t.Surface(f)
	var u, v float64
	s.ClosestPointToUV(o.p, &u, &v, true)
	if math.Abs(s.NormalAt(u, v).WithMagnitude(1).Dot(o.n)) < 1-1e-6 {
		return -1
	}
	return f
}

// openPoint is a point of a face taken away, and the face's normal there.
type openPoint struct{ p, n geom.Vector }

// openPoints are the middles of the triangles of sh's faces opened (of
// topology t), put back on their surfaces.
func openPoints(t *Topology, sh *srf.SShell, opened map[int]bool) []openPoint {
	var out []openPoint
	for f := range t.Faces {
		if !opened[f] {
			continue
		}
		var m srf.SMesh
		s := t.Surface(f)
		s.TriangulateInto(sh, &m)
		for _, tr := range m.Triangles {
			var u, v float64
			s.ClosestPointToUV(tr.A.Plus(tr.B).Plus(tr.C).ScaledBy(1.0/3), &u, &v, true)
			out = append(out, openPoint{p: s.PointAt(u, v), n: s.NormalAt(u, v).WithMagnitude(1)})
		}
	}
	return out
}
