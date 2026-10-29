package check

import (
	"math"

	"github.com/telesapr/telesapr/pkg/geom"
)

// A fitted sweep's path may turn sharply between two straight pieces,
// where the kernel mitres the sweep. Moved profiles can't make a mitre
// (turned at the corner they make a knuckle), so the oracle takes each
// straight piece beside a corner as what a mitre is: the profile, moved
// to the piece's start, carried along it, cut by the plane halving the
// corner at a corner's end and by the profile's plane moved to the end
// at any other. Along a line with no twist the frames don't turn, so
// the prism is the moved profiles there; the samples on such a piece
// are left out of the moved profiles' union.

// pathCornerCos is the cosine of the least turn between two pieces'
// tangents that makes a corner, as the sketch's.
const pathCornerCos = 1 - 1.5e-6

// pathPrism is a straight piece of a fitted sweep's path beside a
// corner: the frame (a, d, r) the profile is moved to at its start, d
// along the piece, and the planes it is cut by at each end, a point and
// a normal into the prism.
type pathPrism struct {
	a, d, r      geom.Vector
	loP, loN     geom.Vector
	hiP, hiN     geom.Vector
	center       geom.Vector
	radius       float64
	piece, first int
}

// pathPrisms finds the corners of the path and makes the prisms beside
// them, marking their pieces; verts are the profile's points, to bound
// each prism by.
func (ps *pathSweep) pathPrisms(verts []geom.Vector) {
	n := len(ps.pieces)
	corner := make([]bool, n+1)
	ps.prismed = make([]bool, n)
	into := func(i int) geom.Vector { return ps.tangent(float64(i) + 1 - 1e-9) }
	for k := 1; k < n; k++ {
		if into(k-1).Dot(ps.tangent(float64(k))) < pathCornerCos {
			corner[k], ps.prismed[k-1], ps.prismed[k] = true, true, true
		}
	}
	sgn := 1.0
	if ps.normals[0].Dot(ps.t0) < 0 {
		sgn = -1
	}
	for i := range n {
		if !ps.prismed[i] {
			continue
		}
		k0 := i * framesPerPathPiece
		pp := pathPrism{a: ps.p[k0], d: ps.t[k0], r: ps.r[k0], piece: i, first: k0}
		b := ps.p[(i+1)*framesPerPathPiece]
		if corner[i] {
			pp.loP, pp.loN = pp.a, into(i-1).Plus(pp.d).WithMagnitude(1)
		} else {
			// The profile's plane moved to the start: its point on the
			// path's side, and its normal along the path.
			pp.loN = ps.normals[k0].ScaledBy(sgn)
			pp.loP = ps.p[k0].Plus(ps.normals[k0].ScaledBy(ps.cIn))
		}
		if corner[i+1] {
			pp.hiP, pp.hiN = b, pp.d.Plus(ps.tangent(float64(i+1))).WithMagnitude(-1)
		} else {
			k1 := (i + 1) * framesPerPathPiece
			pp.hiN = ps.normals[k1].ScaledBy(-sgn)
			pp.hiP = ps.p[k1].Plus(ps.normals[k1].ScaledBy(ps.cIn))
		}
		// The profile's points carried onto both planes bound it.
		var pts []geom.Vector
		for _, v := range verts {
			m := ps.moveTo(k0, v)
			for _, pl := range [][2]geom.Vector{{pp.loP, pp.loN}, {pp.hiP, pp.hiN}} {
				if dn := pp.d.Dot(pl[1]); dn != 0 {
					pts = append(pts, m.Plus(pp.d.ScaledBy(pl[0].Minus(m).Dot(pl[1])/dn)))
				}
			}
		}
		lo := geom.Vector{X: math.Inf(1), Y: math.Inf(1), Z: math.Inf(1)}
		hi := lo.ScaledBy(-1)
		for _, q := range pts {
			q.MakeMaxMin(&hi, &lo)
			q.MakeMaxMin(&ps.hi, &ps.lo)
		}
		pp.center = lo.Plus(hi).ScaledBy(0.5)
		for _, q := range pts {
			pp.radius = math.Max(pp.radius, q.Minus(pp.center).Magnitude())
		}
		pp.radius += ps.detail
		ps.prisms = append(ps.prisms, pp)
	}
}

// inPrism reports whether x is inside the prism: between its planes,
// and carried along it back onto the profile's plane, in the profile.
func (ps *pathSweep) inPrism(pp *pathPrism, x geom.Vector) bool {
	if x.Minus(pp.loP).Dot(pp.loN) < 0 || x.Minus(pp.hiP).Dot(pp.hiN) < 0 {
		return false
	}
	q := ps.back(x, pp.a, pp.d, pp.r)
	dn := ps.t0.Dot(ps.pr.n)
	if dn == 0 {
		return false
	}
	q = q.Minus(ps.t0.ScaledBy(q.Minus(ps.pr.origin).Dot(ps.pr.n) / dn))
	d := q.Minus(ps.pr.origin)
	return ps.pr.contains(d.Dot(ps.pr.eu), d.Dot(ps.pr.ev))
}

// inPrismed reports whether sample k's step to the next lies on a
// straight piece taken as a prism.
func (ps *pathSweep) inPrismed(k int) bool {
	return len(ps.prismed) > 0 && ps.prismed[min(int(math.Floor(ps.s[k])), len(ps.prismed)-1)]
}
