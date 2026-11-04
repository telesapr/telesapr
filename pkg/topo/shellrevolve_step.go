package topo

import (
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// A revolve's curved face taken away runs out past the solid by the
// thickness, so the inside's face does not lie on it, which the Boolean
// can't take away. Where that face meets a flat face at an inside
// corner, as a stepped shaft's thinner side meets the step, running out
// would cut into the wall under the flat face. There the inside steps:
// it runs square to the flat face, beside the wall, to the solid's
// corner, out along the flat face's plane to the run-out, and on from
// there. Between the corner and the run-out it touches the flat
// face from the side the solid is not.

// cornerStep says at which ends a piece of a meridian steps at an
// inside corner.
type cornerStep struct{ start, end bool }

// insideCorners reports whether piece i of the closed loop ps meets the
// piece before it (start) and the one after it (end) at an inside
// corner, its material's angle there more than a half turn.
func insideCorners(ps []profilePiece, i int, n geom.Vector) (start, end bool) {
	k := len(ps)
	prev, next := ps[(i+k-1)%k], ps[(i+1)%k]
	p := ps[i]
	// Turning from one piece onto the next away from the first's
	// material is turning round an inside corner.
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
	return inside(prev, p, p.a), inside(p, next, p.b)
}

// cornerSteps is where piece i, a curved face taken away and moved out,
// steps at an inside corner, or an error when it can't: the step runs
// along the neighbour's plane, so the neighbour must be a flat face,
// kept or taken away.
func cornerSteps(ps []profilePiece, ds []float64, i int, axis, n geom.Vector, thick float64) (cornerStep, error) {
	k := len(ps)
	start, end := insideCorners(ps, i, n)
	var beside []int
	if start {
		beside = append(beside, (i+k-1)%k)
	}
	if end {
		beside = append(beside, (i+1)%k)
	}
	for _, j := range beside {
		if !across(ps[j], axis) {
			return cornerStep{}, shellErr(ps[i].mid, "a curved face taken away meets a curved or sloping face at an inside corner; "+
				"a shell opens a revolve's curved faces at an inside corner only beside a flat face square to the axis")
		}
		if ds[j] != 0 && ds[j] != thick {
			return cornerStep{}, shellErr(ps[i].mid, "a curved face taken away meets, at an inside corner, a face lying inside "+
				"another part; a shell steps round an inside corner only beside a face kept or taken away")
		}
	}
	return cornerStep{start, end}, nil
}

// steppedLoop is the inside's loop of the meridian ps, offset by ds,
// with the steps of step: the kept pieces moved, and where a piece
// steps, the way from the one before it round the solid's corner.
func steppedLoop(ps []profilePiece, ds []float64, step []cornerStep, n geom.Vector, what string) ([]srf.SBezier, error) {
	keep, corner, err := offsetProfile(ps, n, ds, what)
	if err != nil {
		return nil, err
	}
	k, nk := len(ps), len(keep)
	starts := append([]geom.Vector(nil), corner...)
	ends := make([]geom.Vector, nk)
	for j := range keep {
		ends[j] = corner[(j+1)%nk]
	}
	// way[j] is the way from the end of keep[j] to the start of the
	// next, round the solid's corner.
	way := make([][]srf.SBezier, nk)
	stepAt := func(flat, curved int, jFlat, jCurved int, curvedFirst bool) error {
		c := ps[curved].a
		if curvedFirst {
			c = ps[curved].b
		}
		// The wall under the flat face is all of the solid within the
		// thickness of it, out to the corner: the inside meets it on the
		// line through the corner square to the flat face. The flat
		// one where it was meets the curved one moved at the run-out.
		under := c.Plus(ps[flat].into.ScaledBy(ds[flat]))
		onFlat, err := meetMoved(ps[flat], movedPiece(ps[curved], ds[curved]), c, ps[flat], ps[curved], ds[curved], n, what)
		if err != nil {
			return err
		}
		// Past the flat face's far end, the run-out would cut into what
		// rises or falls from there.
		along := ps[flat].b.Minus(ps[flat].a)
		if s := onFlat.Minus(ps[flat].a).Dot(along) / along.MagSquared(); s <= 0 || s >= 1 ||
			onFlat.Minus(ps[flat].a).Magnitude() < geom.LengthEps || onFlat.Minus(ps[flat].b).Magnitude() < geom.LengthEps {
			return shellErr(c, "a curved face taken away at an inside corner beside a flat face narrower than %s; "+
				"the inside's step would run past the flat face", what)
		}
		var path []geom.Vector
		if curvedFirst {
			// Off the curved piece's end: out to the flat line, along it
			// to the corner, then down to the flat one moved.
			ends[jCurved], starts[jFlat] = onFlat, under
			path = []geom.Vector{onFlat, c, under}
		} else {
			ends[jFlat], starts[jCurved] = under, onFlat
			path = []geom.Vector{under, c, onFlat}
		}
		for m := range len(path) - 1 {
			if path[m].Equals(path[m+1]) {
				continue
			}
			jw := jFlat
			if curvedFirst {
				jw = jCurved
			}
			way[jw] = append(way[jw], srf.SBezierFrom2(path[m], path[m+1]))
		}
		return nil
	}
	for j, i := range keep {
		if step[i].start {
			jp := (j + nk - 1) % nk
			if keep[jp] != (i+k-1)%k {
				return nil, shellErr(ps[i].a, "%s closes up the face beside an inside corner", what)
			}
			if err := stepAt(keep[jp], i, jp, j, false); err != nil {
				return nil, err
			}
		}
		if step[i].end {
			jn := (j + 1) % nk
			if keep[jn] != (i+1)%k {
				return nil, shellErr(ps[i].b, "%s closes up the face beside an inside corner", what)
			}
			if err := stepAt(keep[jn], i, jn, j, true); err != nil {
				return nil, err
			}
		}
	}
	var out []srf.SBezier
	for j, i := range keep {
		curves, err := movedCurves(ps[i], starts[j], ends[j], n, 0)
		if err != nil {
			return nil, shellErr(ps[i].a, "%s makes the walls meet, closing up the inside", what)
		}
		out = append(out, curves...)
		out = append(out, way[j]...)
	}
	return out, nil
}
