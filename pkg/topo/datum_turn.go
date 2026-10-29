package topo

import "github.com/telesapr/telesapr/pkg/geom"

// EdgeFrameNear is EdgeFrameAt for a datum whose frame was last found at
// last. A place counts along an edge from its start, and which end is
// the start comes of the order the solid's surfaces and trims are made
// in, which an edit can turn round (a profile's curves bent the other
// way, its loop then wound the other way): the place is then the same
// fraction from the other end. Of the place and the place counted from
// the other end, the one whose frame lies nearer last is taken, and
// returned as the edge now counts it, so the datum keeps where it was
// picked.
func (t *Topology) EdgeFrameNear(n Name, place []float64, last FaceFrame) (FaceFrame, DatumKind, []float64, error) {
	fr, kind, err := t.EdgeFrameAt(n, place)
	if err != nil || kind != DatumEdgePoint || len(place) != 1 {
		return fr, kind, place, err
	}
	// The edge's frame was found at place, so it is found at back too.
	back := []float64{roundPlace(1 - place[0])}
	bf, _, _ := t.EdgeFrameAt(n, back)
	if bf.Origin.Minus(last.Origin).Magnitude() < fr.Origin.Minus(last.Origin).Magnitude()-geom.LengthEps {
		return bf, kind, back, nil
	}
	return fr, kind, place, nil
}
