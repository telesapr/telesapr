package topo

import (
	"math"

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
// steps at an inside corner, or an error when it can't: beside a flat
// face, kept or taken away, the step runs along its plane; beside a
// sloping or curved face, kept, it leaves the corner into the empty
// wedge outside it (cornerOut). Beside a curved face also taken away,
// opens says which, it doesn't step: both run out, and their moved
// pieces meet in the wedge outside the corner, clear of the solid.
func cornerSteps(ps []profilePiece, ds []float64, opens []bool, i int, axis, n geom.Vector, thick float64) (cornerStep, error) {
	k := len(ps)
	start, end := insideCorners(ps, i, n)
	check := func(j int) (bool, error) {
		if opens[j] {
			return false, nil
		}
		if !across(ps[j], axis) && ds[j] != thick {
			return false, shellErr(ps[i].mid, "a curved face taken away meets, at an inside corner, a curved or sloping face "+
				"lying inside another part; a shell steps round such a corner only beside a face kept or taken away")
		}
		if ds[j] != 0 && ds[j] != thick {
			return false, shellErr(ps[i].mid, "a curved face taken away meets, at an inside corner, a face lying inside "+
				"another part; a shell steps round an inside corner only beside a face kept or taken away")
		}
		return true, nil
	}
	var err error
	if start {
		if start, err = check((i + k - 1) % k); err != nil {
			return cornerStep{}, err
		}
	}
	if end {
		if end, err = check((i + 1) % k); err != nil {
			return cornerStep{}, err
		}
	}
	return cornerStep{start, end}, nil
}

// steppedLoop is the inside's loop of the meridian ps, offset by ds,
// with the steps of step: the kept pieces moved, and where a piece
// steps, the way from the one before it round the solid's corner.
// inSolid reports whether a point of the meridian's plane lies in the
// solid's profile; axis is the revolution's.
func steppedLoop(ps []profilePiece, ds []float64, step []cornerStep, axis, n geom.Vector, inSolid func(geom.Vector) bool, what string) ([]srf.SBezier, error) {
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
		under := c.Plus(intoAt(ps[flat], c).ScaledBy(ds[flat]))
		var onFlat geom.Vector
		if across(ps[flat], axis) {
			var err error
			onFlat, err = meetMoved(ps[flat], movedPiece(ps[curved], ds[curved]), c, ps[flat], ps[curved], ds[curved], n, what)
			if err != nil {
				return err
			}
			// Past the flat face's far end, the run-out along it would cut
			// into what rises or falls from there; leaving the corner into
			// the empty wedge may still clear it.
			along := ps[flat].b.Minus(ps[flat].a)
			if s := onFlat.Minus(ps[flat].a).Dot(along) / along.MagSquared(); s <= 0 || s >= 1 ||
				onFlat.Minus(ps[flat].a).Magnitude() < geom.LengthEps || onFlat.Minus(ps[flat].b).Magnitude() < geom.LengthEps {
				p, ok := cornerOut(ps[curved], ps[flat], ds[curved], c, curvedFirst, n, inSolid)
				if !ok {
					return shellErr(c, "a curved face taken away at an inside corner beside a flat face narrower than %s; "+
						"the inside's step would run past the flat face", what)
				}
				onFlat = p
			}
		} else {
			// Beside a sloping or curved face there is no plane to run
			// along without lying on the face, which the Boolean can't
			// take away: the inside leaves the corner into the empty wedge
			// outside it, touching the solid only along the corner.
			p, ok := cornerOut(ps[curved], ps[flat], ds[curved], c, curvedFirst, n, inSolid)
			if !ok {
				return shellErr(c, "a curved face taken away at an inside corner beside a sloping or curved face; "+
					"the inside's run-out by %s would cut into the solid past the corner", what)
			}
			onFlat = p
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

// intoAt is the direction into piece p's material, square to it, at
// its point at: a line's into, or along an arc's radius.
func intoAt(p profilePiece, at geom.Vector) geom.Vector {
	if !p.arc {
		return p.into
	}
	into := at.Minus(p.centre).WithMagnitude(1)
	if !p.outward {
		into = into.ScaledBy(-1)
	}
	return into
}

// cornerOut is where the inside, leaving the inside corner c between
// the curved piece taken away and the piece beside it, crosses the
// curved piece moved out by d (d < 0): along a line into the empty
// wedge between the two at c. It is false when that line meets
// the moved piece nowhere ahead, or runs through the solid on its way.
func cornerOut(curved, beside profilePiece, d float64, c geom.Vector, curvedFirst bool, n geom.Vector, inSolid func(geom.Vector) bool) (geom.Vector, bool) {
	// The two pieces' directions leaving c.
	tc := pieceTangent(curved, true, n)
	if curvedFirst {
		tc = pieceTangent(curved, false, n).ScaledBy(-1)
	}
	ts := pieceTangent(beside, true, n)
	if !curvedFirst {
		ts = pieceTangent(beside, false, n).ScaledBy(-1)
	}
	// Leave at half the empty wedge's angle, or an eighth of a turn from
	// the face beside when the wedge is wider: the wall's end at the
	// corner runs square to that face, and a way out near it, almost
	// smooth with the wall's end across the face, leaves the Boolean a
	// curve it can't split cleanly (a cove's corner of 150°).
	perp := tc.Minus(ts.ScaledBy(ts.Dot(tc)))
	if perp.Magnitude() < 1e-9 {
		return c, false
	}
	perp = perp.WithMagnitude(1)
	angle := math.Min(math.Atan2(tc.Dot(perp), tc.Dot(ts))/2, math.Pi/4)
	dir := ts.ScaledBy(math.Cos(angle)).Plus(perp.ScaledBy(math.Sin(angle)))
	mv := movedPiece(curved, d)
	var cands []geom.Vector
	if mv.arc {
		cands = lineCircle(c, dir, mv.centre, mv.r)
	} else {
		dm := mv.b.Minus(mv.a)
		den := dir.Cross(dm).Dot(n)
		if math.Abs(den) < 1e-12 {
			return c, false
		}
		s := mv.a.Minus(c).Cross(dm).Dot(n) / den
		cands = append(cands, c.Plus(dir.ScaledBy(s)))
	}
	best, found := c, false
	for _, q := range cands {
		t := q.Minus(c).Dot(dir)
		if t <= geom.LengthEps {
			continue
		}
		if !found || t < best.Minus(c).Dot(dir) {
			best, found = q, true
		}
	}
	if !found {
		return c, false
	}
	// The way out must stay clear of the solid past the corner.
	const samples = 32
	for k := 1; k <= samples; k++ {
		q := c.Plus(best.Minus(c).ScaledBy(float64(k) / samples))
		if inSolid(q) {
			return c, false
		}
	}
	return best, true
}

// runOutFractions are the run-outs a curved face taken away tries, as
// fractions of the thickness, longest first. Past the face, the run-out
// lies in what is outside the solid; where the solid comes back within
// the thickness (a slot narrower than it, a step that rises again), a
// shorter run-out still clears the face and leaves the solid whole. The
// shortest stays far above the chord tolerance, so the inside's face
// never lies on the face it opens.
var runOutFractions = []float64{1, 0.5, 0.25, 0.125}

// runOutLoop is the inside's loop of the meridian ps, offset by ds, with
// the curved pieces opens taken away run out past the solid: by the
// thickness where that clears the solid, else by the longest of
// runOutFractions that does, the same for all of them. The shortest
// run-out's error stands when none does.
func runOutLoop(ps []profilePiece, ds []float64, opens []bool, axis, n geom.Vector, thick float64,
	inSolid func(geom.Vector) bool, what string) ([]srf.SBezier, error) {
	var last error
	for _, f := range runOutFractions {
		d := append([]float64(nil), ds...)
		for i := range ps {
			if opens[i] {
				d[i] = -thick * f
			}
		}
		curves, err := runOutTry(ps, d, opens, axis, n, thick, inSolid, what)
		if err == nil {
			return curves, nil
		}
		last = err
	}
	return nil, last
}

// runOutTry is runOutLoop at one run-out, already in ds.
func runOutTry(ps []profilePiece, ds []float64, opens []bool, axis, n geom.Vector, thick float64,
	inSolid func(geom.Vector) bool, what string) ([]srf.SBezier, error) {
	step := make([]cornerStep, len(ps))
	for i := range ps {
		if !opens[i] {
			continue
		}
		if !runOutClear(ps[i], ds[i], n, inSolid) {
			return nil, shellErr(ps[i].mid, "a curved face taken away has the solid again just outside it, nearer than "+
				"an eighth of %s; the inside's run-out past the face would cut into the solid", what)
		}
		st, err := cornerSteps(ps, ds, opens, i, axis, n, thick)
		if err != nil {
			return nil, err
		}
		step[i] = st
	}
	return steppedLoop(ps, ds, step, axis, n, inSolid, what)
}

// runOutClear reports whether the band between piece p and p moved by d
// (d < 0, out of its material) lies outside the solid, inSolid saying
// where it is: sampled away from p's ends, whose corners cornerSteps
// and steppedLoop see to.
func runOutClear(p profilePiece, d float64, n geom.Vector, inSolid func(geom.Vector) bool) bool {
	const along, out = 32, 4
	for k := 1; k < along; k++ {
		t := float64(k) / along
		var at geom.Vector
		if p.arc {
			at = p.a.RotatedAbout(p.centre, n, p.angle*t)
		} else {
			at = p.a.Plus(p.b.Minus(p.a).ScaledBy(t))
		}
		into := intoAt(p, at)
		for j := 1; j <= out; j++ {
			if inSolid(at.Plus(into.ScaledBy(d * float64(j) / out))) {
				return false
			}
		}
	}
	return true
}
