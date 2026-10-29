package topo

import (
	"math"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// Skinned straight through profiles that turn round the ring, a closed
// loft's sections between them are the profiles' blend point for point,
// which shrinks across as they turn (by a twelfth between four circles a
// quarter turn apart). The ring's frame keeps them full: between each
// two profiles sections are put in as a closed sweep's are, the shape
// blended in the frame of the first and turned with it about the way the
// profiles' planes turn, its centre on a curve leaving and reaching each
// profile square to its plane. The skin then goes through those as well,
// each turned from the next by no more than ringTurn.

// ringTurn is the most the sections a closed loft is skinned through
// turn from one to the next: three circles a third of a turn apart then
// stay within 0.006 mm of a torus 25 across; a twelfth of a half turn,
// a third more sections and patches, keeps them within 0.002.
const ringTurn = math.Pi / 8

// ringSections are the sections a closed loft through the ring r is
// skinned through, with the parameter round the ring of each: the
// profiles at r.ts, and between two that turn more than ringTurn apart
// sections turned with the ring's frame, evenly in the parameter; and
// where among them each profile is, the first's again last.
func ringSections(r closedRing) (curves []srf.BSplineCurve, ts []float64, at []int) {
	k := len(r.loops)
	n := make([]geom.Vector, k)
	c := make([]geom.Vector, k)
	for i, l := range r.loops {
		n[i], c[i] = newell(l).WithMagnitude(1), centroid(l)
	}
	// t is the parameter of profile i, taken on round the ring either way.
	t := func(i int) float64 {
		return r.ts[(i%k+k)%k] + math.Floor(float64(i)/float64(k))
	}
	for i := range k {
		a, b := i, (i+1)%k
		at = append(at, len(curves))
		curves, ts = append(curves, r.curves[i]), append(ts, r.ts[i])
		axis, angle := turnBetween(n[a], n[b])
		m := int(math.Ceil(angle/ringTurn - 1e-9))
		if m < 2 {
			continue
		}
		// The shapes of the profile before, a, b and the one after, each
		// about its centre and turned back into a's frame, in homogeneous
		// coordinates.
		back := turnTo(n[b], n[a])
		local := func(j int, turn func(geom.Vector) geom.Vector) []geom.Vector4 {
			cv := r.curves[(j%k+k)%k]
			out := make([]geom.Vector4, len(cv.Ctrl))
			for q, p := range cv.Ctrl {
				out[q] = geom.Vector4FromWeighted(cv.Weight[q], turn(p.Minus(c[(j%k+k)%k])))
			}
			return out
		}
		fromPrev := turnTo(n[(a+k-1)%k], n[a])
		afterNext := turnTo(n[(b+1)%k], n[b])
		same := func(p geom.Vector) geom.Vector { return p }
		la := local(a, same)
		lb := local(a+1, back)
		lp := local(a-1, fromPrev)
		ln := local(a+2, func(p geom.Vector) geom.Vector { return back(afterNext(p)) })
		ta, tb := t(a), t(a+1)
		dt := tb - ta
		// Catmull-Rom slopes of the shape at a and at b, over the span.
		ma := make([]geom.Vector4, len(la))
		mb := make([]geom.Vector4, len(la))
		for q := range la {
			ma[q] = lb[q].Minus(lp[q]).ScaledBy(dt / (tb - t(a-1)))
			mb[q] = ln[q].Minus(la[q]).ScaledBy(dt / (t(a+2) - ta))
		}
		// The centre's curve: an arc leaving a along its normal, turning
		// with the frame, blended into one reaching b along b's, each of
		// the radius that takes a's centre to b's when the two lie on one
		// circle, as a ring's round an axis do; then the blend is that
		// circle.
		chord := c[b].Minus(c[a]).Magnitude()
		rho := chord / (2 * math.Sin(angle/2))
		pa := c[a].Plus(axis.Cross(n[a]).ScaledBy(rho))
		pb := c[b].Plus(axis.Cross(n[b]).ScaledBy(rho))
		for s := 1; s < m; s++ {
			f := float64(s) / float64(m)
			h00, h10 := 2*f*f*f-3*f*f+1, f*f*f-2*f*f+f
			h01, h11 := -2*f*f*f+3*f*f, f*f*f-f*f
			fromA := pa.Plus(geom.QuaternionFromAxisAngle(axis, f*angle).Rotate(c[a].Minus(pa)))
			fromB := pb.Plus(geom.QuaternionFromAxisAngle(axis, (f-1)*angle).Rotate(c[b].Minus(pb)))
			centre := fromA.ScaledBy(h00).Plus(fromB.ScaledBy(h01))
			spin := geom.QuaternionFromAxisAngle(axis, f*angle)
			cv := srf.BSplineCurve{Deg: r.curves[a].Deg, Knots: r.curves[a].Knots}
			cv.Ctrl = make([]geom.Vector, len(la))
			cv.Weight = make([]float64, len(la))
			for q := range la {
				h := la[q].ScaledBy(h00).Plus(ma[q].ScaledBy(h10)).Plus(lb[q].ScaledBy(h01)).Plus(mb[q].ScaledBy(h11))
				cv.Weight[q] = h.W
				cv.Ctrl[q] = centre.Plus(spin.Rotate(h.PerspectiveProject()))
			}
			curves, ts = append(curves, cv), append(ts, ta+f*dt)
		}
	}
	at = append(at, len(curves))
	curves, ts = append(curves, r.curves[k]), append(ts, 1)
	return curves, ts, at
}

// turnBetween is the axis and angle of the least turn taking the unit
// vector from onto to; the axis is any when they are one.
func turnBetween(from, to geom.Vector) (geom.Vector, float64) {
	axis := from.Cross(to)
	angle := math.Atan2(axis.Magnitude(), from.Dot(to))
	if axis.Magnitude() < 1e-12 {
		return geom.Vector{Z: 1}, 0
	}
	return axis.WithMagnitude(1), angle
}

// turnTo is the least turn taking the unit vector from onto to.
func turnTo(from, to geom.Vector) func(geom.Vector) geom.Vector {
	axis, angle := turnBetween(from, to)
	if angle == 0 {
		return func(p geom.Vector) geom.Vector { return p }
	}
	q := geom.QuaternionFromAxisAngle(axis, angle)
	return q.Rotate
}
