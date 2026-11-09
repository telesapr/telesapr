package topo

import (
	"math"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// An end that runs off onto a curved next face past it, as a chamfer of
// a band's side on a cone does past the band's round top, has its end
// section cross that face's surface carried on (rollingrunoff.go). A top
// that curves down away from the band's side, carried on as its own
// polynomial does, comes round past upright and back under the band: a
// wide chamfer's end section then lies wholly outside it, side 1-j's end
// too, though that end is far below the band's top (the x band's
// chamfers of 2.5 at 20 and 21, TestFilletRunsOffOntoCylinder). Carried
// on by its tangent plane along its edge with side j's face instead,
// the top goes on flat past that edge, the end section crosses it, and
// the end is made as on a flat next face: the plane is a new flat face
// beside the top, meeting it smoothly along the edge from w to p2, the
// fillet's cut on it from p2 to p1 and the end face's from p1 to w. It
// is one face of the solid with the top (the same Sheet), as the top
// grown flat.
//
// Taken only where the surface carried on refuses the end, and only
// where the top's edge with side j's face is a line in the tangent
// plane, as a cylinder's ruling is: along a curved edge the plane would
// leave the edge, and the face could not share it.

// tangentFace is the outline of a flat face an end that runs off adds
// beside the curved next face n, put in after the fillet's patches,
// whose cuts it shares.
type tangentFace struct {
	n    int
	loop []srf.SBezier
	// ruled is the face's surface where it is not flat: the top carried
	// on along a curved edge (rollingrunoff_ruled.go).
	ruled *srf.SSurface
}

// tangentIfPast replaces ro's curved next face by its tangent plane
// along its edge with side j's face where end k's section lies wholly
// past the surface carried on but crosses the plane.
func (rb *rollingBall) tangentIfPast(row []srf.SSurface, k int, ro *rbRunOff) {
	if ro.ns == nil || ro.consumed {
		return
	}
	t := rb.t
	m := len(row)
	sec := row[0].EdgeCurve(0)
	if k == 1 {
		sec = row[m-1].EdgeCurve(2)
	}
	if crossesOut(ro, ro.j, sec) {
		// The surface carried on takes the end (runOffCross).
		return
	}
	w := t.Vertices[ro.w].P
	u, v, _ := nearestOnWithin(ro.ns, w, 0)
	n := ro.ns.NormalAt(u, v)
	if n.Magnitude() == 0 {
		return
	}
	// Pointing away from the solid: the edge's end lies under the plane.
	n = n.WithMagnitude(1)
	vp := t.Vertices[rb.ends[k].v].P
	if n.Dot(vp.Minus(w)) > 0 {
		n = n.ScaledBy(-1)
	}
	plane := &rbRunOff{np: n, nd: n.Dot(w)}
	// The edge with side j's face: a line, in the plane.
	pts := t.Edges[ro.sideN].Points
	a, b := pts[0], pts[len(pts)-1]
	tol := math.Max(t.shell.Tol.ChordTolMm()/10, 1e-6)
	for _, p := range pts {
		if math.Abs(plane.h(p)) > tol || p.DistanceToLine(a, b.Minus(a)) > tol {
			// A curved edge: the top is carried on along it by its
			// tangents across it instead (rollingrunoff_ruled.go).
			rb.ruledIfPast(sec, vp, ro)
			return
		}
	}
	if !crossesOut(plane, ro.j, sec) || plane.h(vp) >= 0 {
		return
	}
	ro.ns, ro.np, ro.nd, ro.tangent = nil, plane.np, plane.nd, true
}

// crossesOut reports whether the end section sec crosses ro's next face
// the way an end that runs off past side j's face does: side j's end
// past it, the other's under.
func crossesOut(ro *rbRunOff, j int, sec srf.SBezier) bool {
	inSide, outSide := ro.h(sec.Start()), ro.h(sec.Finish())
	if j == 0 {
		inSide, outSide = outSide, inSide
	}
	return inSide < 0 && outSide > 0
}

// keepTangentFace keeps the outline of the face ro's end adds: from w
// along the top's edge with side j's face to p2, the fillet's cut across
// to p1, and the end face's edge with it, shared, back to w.
func (rb *rollingBall) keepTangentFace(ro *rbRunOff, across, shared []srf.SBezier) error {
	w := rb.t.Vertices[ro.w].P
	first := srf.SBezierFrom2(w, ro.p2)
	if ro.ruled != nil {
		first = ro.ruled.along(ro.p2)
	}
	loop := []srf.SBezier{first}
	left := append([]srf.SBezier(nil), across...)
	at := ro.p2
	for len(left) > 0 {
		found := false
		for i, sb := range left {
			if !sb.Start().Equals(at) && !sb.Finish().Equals(at) {
				continue
			}
			if !sb.Start().Equals(at) {
				sb.Reverse()
			}
			loop = append(loop, sb)
			at = sb.Finish()
			left = append(left[:i], left[i+1:]...)
			found = true
			break
		}
		if !found {
			return filletErr(at, "the fillet's cut by the plane its end runs onto doesn't join here")
		}
	}
	if !at.Equals(ro.p1) {
		return filletErr(at, "the fillet's cut by the plane its end runs onto doesn't reach its end here")
	}
	if !ro.p1.Equals(w) {
		loop = append(loop, shared...)
	}
	var poly []geom.Vector
	for _, sb := range loop {
		for i := range 8 {
			poly = append(poly, sb.PointAt(float64(i)/8))
		}
	}
	if newellOf(poly).Magnitude() < geom.LengthEps {
		// A sliver the flat face can't hold: the end only just runs off
		// (a fillet of 2.5 on the band at 22 whose top sags 2, p2 0.014
		// from w and p1 at w).
		return filletErr(ro.p2, "the fillet's end only just runs off the face on side %d here: make it a little "+
			"smaller or larger", ro.j+1)
	}
	tf := tangentFace{n: ro.n, loop: loop}
	if ro.ruled != nil {
		tf.ruled = &ro.ruled.s
	}
	rb.tangentFaces = append(rb.tangentFaces, tf)
	return nil
}

// putTangentFaces puts in the flat faces the fillet's ends that run off
// add, after its patches: each takes the next face's identity, so that
// the two are one face of the solid.
func (rb *rollingBall) putTangentFaces(fa *FaceAdder) error {
	for _, tf := range rb.tangentFaces {
		s := rb.t.Surface(tf.n)
		if s == nil {
			return filletErr(tf.loop[0].Start(), "the face the fillet's end runs onto is not in the shell")
		}
		var h core.HSSurface
		var err error
		if tf.ruled != nil {
			rs := *tf.ruled
			rs.Origin, rs.Color, rs.Tol = s.Origin, s.Color, s.Tol
			h, err = fa.Add(rs, [][]srf.SBezier{tf.loop})
		} else {
			var n geom.Vector
			h, err = fa.AddPlanarFaceWithin([][]srf.SBezier{tf.loop}, s.Origin, s.Color, n, geom.LengthEps)
		}
		if err != nil {
			return filletErr(tf.loop[0].Start(), "the flat face the fillet's end runs onto can't be put in: %v", err)
		}
		if ns := fa.sh.FindSurface(h); ns != nil {
			ns.Face, ns.Sheet = s.Face, s.Sheet
		}
	}
	return nil
}
