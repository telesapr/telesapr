package topo

import (
	"errors"
	"math"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// A lathe or revolve taken away from a solid, or intersected with it,
// as a part of the solid a shell hollows (shelljoined.go). Taken away,
// it is grown by the thickness: its profile moved out and turned, and
// a revolve's flat ends moved out too, each end's plane by the
// thickness, which a turn through a wider angle doesn't do (the gap
// widens with the distance from the axis). So a revolve of less than a
// whole turn grown is the whole turn of its profile moved out, cut by
// the ends' planes moved out: what lies past both for half a turn or
// less, past either for more. A face taken away lying on such a part is
// moved as on a part of flat faces: into a part taken away, out of one
// intersected with, so the inside's face doesn't lie on the solid's.

// openOnRevolution reports, of a point, whether it lies on a face of
// rev, a part taken away or intersected with, on which a face taken
// away (open) lies; nil when none does.
func openOnRevolution(rev Revolution, open []openPoint, tol srf.Tolerance, color core.RGBA) (func(geom.Vector) bool, error) {
	if len(open) == 0 {
		return nil, nil
	}
	rt, err := Build(rev.solid(tol, color))
	if err != nil {
		return nil, err
	}
	faces := map[int]bool{}
	for _, o := range open {
		if f := onFaceOf(rt, o); f >= 0 {
			faces[f] = true
		}
	}
	if len(faces) == 0 {
		return nil, nil
	}
	return func(p geom.Vector) bool {
		f, d := rt.FaceAt(p)
		return f >= 0 && d <= 100*geom.LengthEps && faces[f]
	}, nil
}

// grownRevolution is the inside's cutter of rev, a part taken away: rev
// grown by the thickness, but where its faces lie on a face taken away
// (openAt), moved in by it.
func grownRevolution(rev Revolution, openAt func(geom.Vector) bool, thick float64, tol srf.Tolerance, color core.RGBA) (*srf.SShell, error) {
	if rev.Whole() {
		return joinedRevolvedInside(rev, func(geom.Vector) faceMode { return buriedFace }, openAt, keptFace, thick, tol, color)
	}
	axis := rev.Axis.WithMagnitude(1)
	pts := profilePoints(rev.Profile)
	if len(pts) == 0 {
		return nil, ErrNotRevolution
	}
	radial := radialFrom(pts[0], rev.Pt, axis)
	// The whole turn's pieces are asked about all round; a piece is
	// moved in where it is on a face taken away at an angle of the
	// revolve, between its ends, not on them, where a piece's point lies
	// on the end's edge.
	const samples = 16
	whole := rev
	whole.End = whole.Start + 2*math.Pi
	modeAt := func(p geom.Vector) (faceMode, bool) {
		if openAt != nil {
			r := radialFrom(p, rev.Pt, axis)
			at := math.Atan2(radial.Cross(r).Dot(axis), radial.Dot(r))
			for k := range samples {
				a := rev.Start + (float64(k)+0.5)/samples*(rev.End-rev.Start)
				if openAt(p.RotatedAbout(rev.Pt, axis, a-at)) {
					return keptFace, true
				}
			}
		}
		return buriedFace, true
	}
	noEnds := func(geom.Vector) ([2]faceMode, error) { return [2]faceMode{}, nil }
	inner, err := revolvedInside(whole, modeAt, noEnds, thick, tol, color)
	if err != nil {
		return nil, err
	}

	// Each end's plane moved out by the thickness, or in where the end
	// is taken away.
	var shift [2]float64
	for k, a := range []float64{rev.Start, rev.End} {
		shift[k] = -thick
		for _, q := range pts {
			if openAt != nil && openAt(q.RotatedAbout(rev.Pt, axis, a)) {
				shift[k] = thick
				break
			}
		}
	}
	return cutToEnds(inner, rev, radial, shift, thick, tol, color)
}

// endsMoved is the inside of rev, a revolve of half a turn or less, of
// the inside's profile sets, where an end lies inside another part:
// the whole turn of the profile cut by each end's plane moved as its
// mode says, in by the thickness as a wall, out by it inside another
// part (twice that probed), where it is taken away.
func endsMoved(rev Revolution, sets *srf.SBezierLoopSetSet, modes [2]faceMode, radial geom.Vector, thick float64,
	tol srf.Tolerance, color core.RGBA) (*srf.SShell, error) {
	axis := rev.Axis.WithMagnitude(1)
	inner := &srf.SShell{Tol: tol}
	for i := range sets.Sets {
		inner.MakeFromRevolutionOf(&sets.Sets[i], rev.Pt, axis, color, noFaces{})
	}
	var shift [2]float64
	for k, m := range modes {
		switch m {
		case keptFace:
			shift[k] = thick
		case buriedFace:
			shift[k] = -thick
		case probedFace:
			shift[k] = -2 * thick
		}
	}
	return cutToEnds(inner, rev, radial, shift, thick, tol, color)
}

// cutToEnds is inner, a whole turn, cut back to the planes of rev's ends
// moved by shift into its material: what lies past both taken away for
// half a turn or less, past either for more.
func cutToEnds(inner *srf.SShell, rev Revolution, radial geom.Vector, shift [2]float64, thick float64,
	tol srf.Tolerance, color core.RGBA) (*srf.SShell, error) {
	axis := rev.Axis.WithMagnitude(1)
	turn := math.Abs(rev.End - rev.Start)
	half := math.Abs(turn-math.Pi) < 1e-9
	if half && shift[0] != shift[1] {
		// The two ends lie in one plane, either side of the axis: moved
		// apart, the plane would step at the axis.
		return nil, shellErr(rev.Pt.Plus(radial.RotatedAroundAxis(axis, rev.End)), "the two ends of a half turn lie in one plane "+
			"and are not moved alike, one taken away, a wall or inside another solid and the other not; a shell moves the ends of a half turn alike only")
	}
	var beyond [2]*srf.SShell
	for k := range 2 {
		beyond[k] = beyondEnd(rev, k, radial, shift[k], thick, tol, color)
	}
	cuts := beyond[:]
	switch {
	case half:
		cuts = beyond[:1]
	case turn > math.Pi:
		// More than half a turn: what lies past both planes.
		both := &srf.SShell{Tol: tol}
		both.MakeFromIntersectionOf(beyond[0], beyond[1])
		if both.BooleanFailed {
			return nil, errors.New("the Boolean that finds what lies past a revolve's ends failed")
		}
		cuts = []*srf.SShell{both}
	}
	for _, c := range cuts {
		back := &srf.SShell{Tol: tol}
		back.MakeFromDifferenceOf(inner, c)
		if back.BooleanFailed {
			return nil, errors.New("the Boolean that cuts a revolve's inside back to its ends failed")
		}
		inner = back
	}
	return inner, nil
}

// beyondEnd is a block holding all that lies on the far side from the
// solid of the plane of end k of rev (0 at Start, 1 at End) moved by
// shift into the solid, over the whole of the profile's size.
func beyondEnd(rev Revolution, k int, radial geom.Vector, shift, thick float64, tol srf.Tolerance, color core.RGBA) *srf.SShell {
	axis := rev.Axis.WithMagnitude(1)
	u, into := endFrame(rev, k, radial)
	lo, hi := profileBox(rev.Profile)
	size := 2*(hi.Minus(lo).Magnitude()+rev.Pt.Minus(lo).Magnitude()) + 2*thick + 1
	p0 := rev.Pt.Minus(axis.ScaledBy(size)).Minus(u.ScaledBy(size)).Plus(into.ScaledBy(shift - size))
	p1 := p0.Plus(axis.ScaledBy(2 * size))
	p2 := p1.Plus(u.ScaledBy(2 * size))
	p3 := p0.Plus(u.ScaledBy(2 * size))
	sh := &srf.SShell{Tol: tol}
	sh.MakePrism([]geom.Vector{p0, p1, p2, p3}, geom.Vector{}, into.ScaledBy(size), color)
	return sh
}
