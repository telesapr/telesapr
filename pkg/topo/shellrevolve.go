package topo

import (
	"errors"
	"fmt"
	"math"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// A shell of a solid of revolution: a lathe, or a revolve, of a
// profile of lines and arcs. Its inside is the profile offset into its
// material by the thickness, each piece but those on the axis and
// those whose faces are taken away, turned as the solid was, and taken
// away. A revolve's flat ends, kept, are walls too: the inside is cut
// back by each end's plane moved in. The profile is the exact one, so
// cones, spheres and tori come out with exact walls.

// Revolution is how a solid of revolution was made: Profile, closed
// loops in a plane through the axis (Pt, Axis), turned about it from
// Start to End radians, a whole turn when they are 2π or more apart.
type Revolution struct {
	Profile    *srf.SBezierLoopSetSet
	Pt, Axis   geom.Vector
	Start, End float64
}

// ErrNotRevolution is ShellRevolved's error when the solid is not the
// revolution it was given, as when a feature after it changed it.
var ErrNotRevolution = errors.New("the solid is not one revolution of its profile")

// noFaces is the shell group of an inside: its surfaces are named by
// their origins, not by entities.
type noFaces struct{}

func (noFaces) Remap(core.HEntity, int) core.HEntity { return core.NoEntity }
func (noFaces) EntityExists(core.HEntity) bool       { return false }

// Whole reports whether the revolution is a whole turn.
func (r Revolution) Whole() bool { return math.Abs(r.End-r.Start) >= 2*math.Pi-1e-9 }

// ShellRevolved hollows sh, which is rev, to walls thick, taking away
// faces open (indices into its topology). The faces of the cavity get
// origins from origin up. It is ErrNotRevolution when the faces of sh
// are not those of rev.
func ShellRevolved(sh *srf.SShell, rev Revolution, open []int, thick float64, origin uint32) (*srf.SShell, error) {
	if thick <= 0 || math.IsNaN(thick) || math.IsInf(thick, 0) {
		return nil, fmt.Errorf("a shell's thickness must be more than zero, not %g", thick)
	}
	if rev.Profile == nil || len(rev.Profile.Sets) == 0 {
		return nil, ErrNotRevolution
	}
	t, err := Build(sh)
	if err != nil {
		return nil, err
	}
	if probs := t.Validate(); len(probs) > 0 {
		return nil, shellErr(probs[0].At, "the solid is not closed: %s", probs[0].Msg)
	}
	opened := map[int]bool{}
	for _, f := range open {
		if f < 0 || f >= len(t.Faces) {
			return nil, fmt.Errorf("there is no face %d of the solid", f)
		}
		opened[f] = true
	}
	if len(opened) == 0 {
		return nil, errors.New("there are no faces to take away: a shell is open where faces are taken away")
	}
	if len(opened) == len(t.Faces) {
		return nil, errors.New("every face is taken away, which leaves no wall")
	}
	axis := rev.Axis.WithMagnitude(1)
	n := rev.Profile.Sets[0].Normal.WithMagnitude(1)
	if math.Abs(n.Dot(axis)) > 1e-9 {
		return nil, ErrNotRevolution
	}

	// The profile's pieces, loop by loop, and which faces each turns
	// into.
	var loops [][]profilePiece
	var onAxis [][]bool
	var all []geom.Vector // a point of the profile off the axis
	inside := profileInside(rev.Profile, n)
	for _, set := range rev.Profile.Sets {
		for _, l := range append([]srf.SBezierLoop{set.Outer}, set.Holes...) {
			ps, axial, err := meridianPieces(l.Curves, n, rev.Pt, axis, inside)
			if err != nil {
				return nil, err
			}
			loops = append(loops, ps)
			onAxis = append(onAxis, axial)
			for i, p := range ps {
				if !axial[i] {
					all = append(all, p.a.Plus(p.b).ScaledBy(0.5))
				}
			}
		}
	}
	if len(all) == 0 {
		return nil, ErrNotRevolution
	}
	radial := radialFrom(all[0], rev.Pt, axis)
	for _, p := range all {
		if p.Minus(rev.Pt).Dot(radial) < -geom.LengthEps {
			return nil, ErrNotRevolution
		}
	}

	// Where each piece's faces are: its middle turned to the middle of
	// each of the solid's sections.
	var probes []float64
	if rev.Whole() {
		for k := range 4 {
			probes = append(probes, rev.Start+math.Pi/4+float64(k)*math.Pi/2)
		}
	} else {
		sections := int(math.Abs(rev.End-rev.Start)/(math.Pi/2) + 1)
		wedge := (rev.End - rev.Start) / float64(sections)
		for k := range sections {
			probes = append(probes, rev.Start+(float64(k)+0.5)*wedge)
		}
	}
	seen := map[int]bool{}
	ds := make([][]float64, len(loops))
	for li, ps := range loops {
		ds[li] = make([]float64, len(ps))
		for i, p := range ps {
			if onAxis[li][i] {
				continue
			}
			ds[li][i] = thick
			m := p.mid
			for _, a := range probes {
				f, d := t.FaceAt(m.RotatedAbout(rev.Pt, axis, a))
				if f < 0 || d > 100*geom.LengthEps {
					return nil, ErrNotRevolution
				}
				seen[f] = true
				if opened[f] {
					ds[li][i] = 0
				}
			}
			if ds[li][i] == 0 && !across(p, axis) {
				// A curved face taken away: the inside's face on it would
				// lie on it, which the Boolean can't take away, so it runs
				// out past it. Where it meets its neighbours at an inside
				// corner, running out would cut into them.
				if err := outsideCorners(ps, i, n); err != nil {
					return nil, err
				}
				ds[li][i] = -thick
			}
		}
	}

	// The faces no piece turned into are a revolve's ends.
	openStart, openEnd := false, false
	for f := range t.Faces {
		if seen[f] {
			continue
		}
		at, ok := revolveEnd(t, f, rev, radial)
		if !ok {
			return nil, ErrNotRevolution
		}
		if opened[f] {
			openStart = openStart || at == 0
			openEnd = openEnd || at == 1
		}
	}
	if !rev.Whole() && (!openStart || !openEnd) && math.Abs(rev.End-rev.Start) > math.Pi+1e-9 {
		return nil, shellErr(rev.Pt, "a revolve of more than half a turn is hollowed only when both its ends are taken away")
	}

	// The inside's profile.
	var sbl srf.SBezierList
	what := fmt.Sprintf("a thickness of %g", thick)
	for li, ps := range loops {
		keep, corner, err := offsetProfile(ps, n, ds[li], what)
		if err != nil {
			return nil, err
		}
		for j, i := range keep {
			curves, err := movedCurves(ps[i], corner[j], corner[(j+1)%len(keep)], n, 0)
			if err != nil {
				return nil, shellErr(ps[i].a, "%s makes the walls meet, closing up the inside", what)
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
		return nil, errors.New("the inside's profile, offset from the solid's, does not close")
	}
	color := sh.Surface[0].Color
	inner := &srf.SShell{Tol: sh.Tol}
	for i := range sets.Sets {
		if rev.Whole() {
			inner.MakeFromRevolutionOf(&sets.Sets[i], rev.Pt, axis, color, noFaces{})
		} else {
			inner.MakeFromHelicalRevolutionOf(&sets.Sets[i], rev.Pt, axis, color, noFaces{}, rev.Start, rev.End, 0, 0)
		}
	}
	if !rev.Whole() {
		for k, isOpen := range []bool{openStart, openEnd} {
			if isOpen {
				continue
			}
			// With both ends kept, the inside lies on the far side of
			// both ends' planes moved in (a half turn at most). With the
			// other end open, only what lies on this end's side of the
			// axis is cut: across it, the plane runs by the open end.
			from := math.Inf(-1)
			if openStart || openEnd {
				from = 0
				if math.Abs(rev.End-rev.Start) <= math.Pi/2+1e-9 {
					// Clear of the inside, and of the open end's plane at a
					// quarter turn.
					from = -thick / 2
				}
			}
			cut, err := endSlab(rev, k, radial, from, thick, sh.Tol, color)
			if err != nil {
				return nil, err
			}
			back := &srf.SShell{Tol: sh.Tol}
			back.MakeFromDifferenceOf(inner, cut)

			if back.BooleanFailed {
				return nil, errors.New("the Boolean that cuts the inside back from the revolve's end failed")
			}
			inner = back
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

// meridianPieces are the pieces of a loop of a profile in the plane of
// normal n, lines and arcs, into each the side where inside is, and
// which lie on the axis (pt, axis).
func meridianPieces(loop []srf.SBezier, n, pt, axis geom.Vector, inside func(geom.Vector) bool) ([]profilePiece, []bool, error) {
	var pieces []profilePiece
	var axial []bool
	for _, sb := range loop {
		a, m, b := sb.Start(), sb.PointAt(0.5), sb.Finish()
		p := profilePiece{a: a, b: b, mid: m}
		step := 1e-4 * math.Max(b.Minus(a).Magnitude(), geom.LengthEps*1e4)
		if sb.Degree == 1 || m.DistanceToLine(a, b.Minus(a)) < geom.LengthEps {
			p.into = n.Cross(b.Minus(a)).WithMagnitude(1)
			if !inside(m.Plus(p.into.ScaledBy(step))) {
				p.into = p.into.ScaledBy(-1)
			}
			pieces = append(pieces, p)
			axial = append(axial, a.DistanceToLine(pt, axis) < geom.LengthEps && b.DistanceToLine(pt, axis) < geom.LengthEps)
			continue
		}
		c, ok := circleThrough(a, m, b, n)
		r := c.Minus(a).Magnitude()
		if !ok || sb.Degree != 2 || math.Abs(sb.PointAt(0.25).Minus(c).Magnitude()-r) > geom.LengthEps {
			return nil, nil, shellErr(m, "a curve of the profile is neither a line nor an arc; "+
				"a shell hollows revolves of lines and arcs")
		}
		p.arc, p.centre, p.r = true, c, r
		ra, rb := a.Minus(c), b.Minus(c)
		p.angle = math.Atan2(ra.Cross(rb).Dot(n), ra.Dot(rb))
		if rm := m.Minus(c); math.Signbit(ra.Cross(rm).Dot(n)) != math.Signbit(p.angle) {
			p.angle -= math.Copysign(2*math.Pi, p.angle)
		}
		out := m.Minus(c).WithMagnitude(1)
		p.outward = inside(m.Plus(out.ScaledBy(step)))
		p.into = out
		if !p.outward {
			p.into = out.ScaledBy(-1)
		}
		pieces = append(pieces, p)
		axial = append(axial, false)
	}
	return pieces, axial, nil
}

// across reports whether piece p runs square to the axis, so turned it
// is flat.
func across(p profilePiece, axis geom.Vector) bool {
	return !p.arc && math.Abs(p.b.Minus(p.a).WithMagnitude(1).Dot(axis)) < 1e-9
}

// outsideCorners is an error when piece i of the closed loop ps meets
// either neighbour at an inside corner, its material's angle there
// more than a half turn.
func outsideCorners(ps []profilePiece, i int, n geom.Vector) error {
	k := len(ps)
	prev, next := ps[(i+k-1)%k], ps[(i+1)%k]
	p := ps[i]
	// Turning from one piece onto the next towards the first's
	// material is turning round an outside corner.
	inside := func(from, to profilePiece, at geom.Vector) bool {
		into := from.into
		if from.arc {
			into = at.Minus(from.centre).WithMagnitude(1)
			if !from.outward {
				into = into.ScaledBy(-1)
			}
		}
		return pieceTangent(to, true, n).Dot(into) < -1e-9
	}
	if inside(prev, p, p.a) || inside(p, next, p.b) {
		return shellErr(p.mid, "a curved face taken away meets a face kept at an inside corner; "+
			"a shell opens a revolve's curved faces only between outside corners")
	}
	return nil
}

// profileInside reports whether a point of the profile's plane, of
// normal n, lies inside the profile: inside an odd number of its loops.
func profileInside(profile *srf.SBezierLoopSetSet, n geom.Vector) func(geom.Vector) bool {
	ax := n.NormalDir(0)
	ay := n.Cross(ax)
	flat := func(p geom.Vector) geom.Vector { return geom.Vector{X: p.Dot(ax), Y: p.Dot(ay)} }
	var polys [][]geom.Vector
	for _, set := range profile.Sets {
		for _, l := range append([]srf.SBezierLoop{set.Outer}, set.Holes...) {
			var poly []geom.Vector
			for _, sb := range l.Curves {
				k := 1
				if sb.Degree > 1 {
					k = 32
				}
				for j := range k {
					poly = append(poly, flat(sb.PointAt(float64(j)/float64(k))))
				}
			}
			polys = append(polys, poly)
		}
	}
	return func(p geom.Vector) bool {
		in := false
		q := flat(p)
		for _, poly := range polys {
			if pointInPolygon(q, poly) {
				in = !in
			}
		}
		return in
	}
}

// radialFrom is the unit direction square to the axis (pt, axis) from
// it to p.
func radialFrom(p, pt, axis geom.Vector) geom.Vector {
	d := p.Minus(pt)
	return d.Minus(axis.ScaledBy(d.Dot(axis))).WithMagnitude(1)
}

// revolveEnd is which end of the revolve rev face f is, 0 at Start and
// 1 at End: a flat face through the axis at that angle.
func revolveEnd(t *Topology, f int, rev Revolution, radial geom.Vector) (int, bool) {
	if rev.Whole() {
		return 0, false
	}
	n, ok := t.planarNormal(f)
	axis := rev.Axis.WithMagnitude(1)
	if !ok || math.Abs(n.Dot(axis)) > 1e-9 {
		return 0, false
	}
	// The middle of the face's points off the axis, turned back.
	var c geom.Vector
	k := 0
	for _, e := range t.FaceEdges(f) {
		for _, p := range t.Edges[e].Points {
			if p.DistanceToLine(rev.Pt, axis) > geom.LengthEps {
				c = c.Plus(p)
				k++
			}
		}
	}
	if k == 0 {
		return 0, false
	}
	c = c.ScaledBy(1 / float64(k))
	for end, a := range []float64{rev.Start, rev.End} {
		u := radial.RotatedAroundAxis(axis, a)
		if radialFrom(c, rev.Pt, axis).Minus(u).Magnitude() < 1e-6 {
			return end, true
		}
	}
	return 0, false
}

// endSlab is a block holding what lies within thick of the plane of
// end k of rev (0 at Start, 1 at End) on the solid's side, and all
// beyond it, over the whole of the profile's size, from distance from
// along the end's radial direction on (-Inf for the whole plane).
func endSlab(rev Revolution, k int, radial geom.Vector, from, thick float64, tol srf.Tolerance, color core.RGBA) (*srf.SShell, error) {
	axis := rev.Axis.WithMagnitude(1)
	a := []float64{rev.Start, rev.End}[k]
	u := radial.RotatedAroundAxis(axis, a)
	// Into the solid from the end: the way the turn goes from Start,
	// back from End.
	step := math.Copysign(1e-3, rev.End-rev.Start)
	if k == 1 {
		step = -step
	}
	at := rev.Pt.Plus(u)
	into := axis.Cross(u).WithMagnitude(1)
	if rev.Pt.Plus(radial).RotatedAbout(rev.Pt, axis, a+step).Minus(at).Dot(into) < 0 {
		into = into.ScaledBy(-1)
	}
	lo, hi := profileBox(rev.Profile)
	size := hi.Minus(lo).Magnitude() + rev.Pt.Minus(lo).Magnitude() + thick + 1
	from = math.Max(from, -size)
	p0 := rev.Pt.Minus(axis.ScaledBy(size)).Plus(u.ScaledBy(from)).Minus(into.ScaledBy(size))
	p1 := p0.Plus(axis.ScaledBy(2 * size))
	p2 := p1.Plus(u.ScaledBy(size - from))
	p3 := p0.Plus(u.ScaledBy(size - from))
	sbl := srf.SBezierList{Curves: []srf.SBezier{srf.SBezierFrom2(p0, p1), srf.SBezierFrom2(p1, p2),
		srf.SBezierFrom2(p2, p3), srf.SBezierFrom2(p3, p0)}}
	var sets srf.SBezierLoopSetSet
	var poly srf.SPolygon
	allClosed, allCoplanar := false, false
	var notClosed srf.SEdge
	var notCoplanar geom.Vector
	sets.FindOuterFacesFrom(&sbl, &poly, nil, tol, &allClosed, &notClosed, &allCoplanar, &notCoplanar, nil)
	if !allClosed || len(sets.Sets) != 1 {
		return nil, errors.New("the block that cuts the inside back from the revolve's end does not close")
	}
	sh := &srf.SShell{Tol: tol}
	sh.MakeFromExtrusionOf(&sets.Sets[0], geom.Vector{}, into.ScaledBy(size+thick), color)
	return sh, nil
}

// profileBox is the corners of the box about the profile's curves.
func profileBox(profile *srf.SBezierLoopSetSet) (lo, hi geom.Vector) {
	lo = geom.Vector{X: math.Inf(1), Y: math.Inf(1), Z: math.Inf(1)}
	hi = lo.ScaledBy(-1)
	for _, set := range profile.Sets {
		for _, l := range append([]srf.SBezierLoop{set.Outer}, set.Holes...) {
			for _, sb := range l.Curves {
				for i := 0; i <= sb.Degree; i++ {
					p := sb.Ctrl[i]
					lo = geom.Vector{X: math.Min(lo.X, p.X), Y: math.Min(lo.Y, p.Y), Z: math.Min(lo.Z, p.Z)}
					hi = geom.Vector{X: math.Max(hi.X, p.X), Y: math.Max(hi.Y, p.Y), Z: math.Max(hi.Z, p.Z)}
				}
			}
		}
	}
	return lo, hi
}
