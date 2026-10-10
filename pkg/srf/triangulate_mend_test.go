package srf

import (
	"testing"

	"github.com/telesapr/telesapr/pkg/geom"
)

// Two faces holding one triangle, either way round and its points a
// little apart in the last digits, lie on each other; two that share
// only an edge, even run the same way as at a knife edge, do not.
func TestStackedFaces(t *testing.T) {
	t.Parallel()
	a, b := geom.Vector{X: 44, Y: 4, Z: 13}, geom.Vector{X: 44.0148, Y: 4, Z: 12.9999}
	c, d := geom.Vector{X: 44.0074, Y: 4, Z: 12.99997}, geom.Vector{X: 44, Y: 5, Z: 13}
	tri := func(p, q, r geom.Vector) SMesh {
		return SMesh{Triangles: []STriangle{STriangleFrom(STriMeta{}, p, q, r)}}
	}
	nudged := a.Plus(geom.Vector{Y: 1e-11})
	if got := stackedFaces([]SMesh{tri(a, b, c), tri(nudged, c, b), tri(c, d, b)}); !got[0] || !got[1] || got[2] {
		t.Errorf("one triangle: %v, want the first two", got)
	}
	if got := stackedFaces([]SMesh{tri(a, b, c), tri(b, a, d), tri(a, b, geom.Vector{X: 44, Y: 3, Z: 14})}); got[0] || got[1] || got[2] {
		t.Errorf("an edge: %v, want none", got)
	}
}
