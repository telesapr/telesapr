package topo

import (
	"fmt"
	"math"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// Where on a face or an edge a datum was picked, kept so that its frame
// is found again there rather than at the middle (DatumFrameAt,
// EdgeFrameAt). The place is relative, not a point in space: on a face,
// the picked point's parameters as fractions of the bounds of the face's
// outer loop in its surface's parameters; on an edge, the fraction of its
// length from its start. An edit that stretches or bends the face or the
// edge carries the place along, as it carries the middle. A place is
// kept to a ten-thousandth (placeSteps): finer means nothing to a datum,
// and the frame's origin given again names the same place, so that a
// model written as a script (Datums.NameFaceAt) and a point picked
// near it come back to the same file.

// placeSteps is how many steps a place is rounded to, end to end.
const placeSteps = 1e4

// FacePlace is where p, a point on or near the face with identity id,
// lies on the face as a datum keeps it: its nearest point's parameters
// as fractions, 0 to 1, of the bounds of the face's outer loop in its
// surface's parameters. A face a Boolean cut in pieces is taken by its
// first piece, as FacePointFrame takes it.
func (t *Topology) FacePlace(id srf.FaceID, p geom.Vector) ([]float64, error) {
	fs := t.FacesWithID(id)
	if len(fs) == 0 {
		return nil, fmt.Errorf("face %x is no longer in the solid", uint32(id))
	}
	f := fs[0]
	s := uncached(t.Surface(f))
	if s == nil {
		return nil, fmt.Errorf("face %x has no surface", uint32(id))
	}
	var outer []geom.Vector
	for _, l := range t.Faces[f].Loops {
		if t.Loops[l].Outer {
			outer = t.loopUV(s, t.Loops[l].CoEdges)
		}
	}
	if len(outer) < 3 {
		return nil, fmt.Errorf("face %x has no outer loop", uint32(id))
	}
	var u, v float64
	s.ClosestPointToUV(p, &u, &v, false)
	lo, hi := uvBounds(outer)
	return []float64{fraction(u, lo.X, hi.X), fraction(v, lo.Y, hi.Y)}, nil
}

// EdgePlace is where p, a point on or near the edge named n, lies along
// it as a datum keeps it: the fraction of the edge's length from its
// start to the point of its polyline nearest p.
func (t *Topology) EdgePlace(n Name, p geom.Vector) ([]float64, error) {
	e, err := t.Resolve(n)
	if err != nil {
		return nil, err
	}
	pts := t.Edges[e].Points
	var walked, best float64
	bestD := -1.0
	for i := 1; i < len(pts); i++ {
		d := pts[i].Minus(pts[i-1])
		l := d.Magnitude()
		if l < geom.LengthEps*geom.LengthEps {
			continue
		}
		s := min(max(p.Minus(pts[i-1]).Dot(d)/(l*l), 0), 1)
		if dist := pts[i-1].Plus(d.ScaledBy(s)).Minus(p).Magnitude(); bestD < 0 || dist < bestD {
			best, bestD = walked+s*l, dist
		}
		walked += l
	}
	if walked < geom.LengthEps {
		return nil, fmt.Errorf("%s: %w", n, errNoLength)
	}
	return []float64{roundPlace(best / walked)}, nil
}

// fraction is where x lies between lo and hi, 0 to 1, as a place; a
// half where the two are one.
func fraction(x, lo, hi float64) float64 {
	if hi-lo < geom.LengthEps*geom.LengthEps {
		return 0.5
	}
	return roundPlace(min(max((x-lo)/(hi-lo), 0), 1))
}

// roundPlace is f to the nearest step of placeSteps.
func roundPlace(f float64) float64 { return math.Round(f*placeSteps) / placeSteps }

// uvBounds are the least and greatest parameters of a loop in uv.
func uvBounds(loop []geom.Vector) (lo, hi geom.Vector) {
	lo, hi = loop[0], loop[0]
	for _, p := range loop {
		lo = geom.Vector{X: min(lo.X, p.X), Y: min(lo.Y, p.Y)}
		hi = geom.Vector{X: max(hi.X, p.X), Y: max(hi.Y, p.Y)}
	}
	return lo, hi
}
