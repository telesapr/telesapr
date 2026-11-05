package topo

import (
	"math"
	"sync"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// loopUVCache keeps each loop's outline in its face's surface parameters
// once FaceAt has worked it out. FaceAt asks it of every face for every
// point, and a shell's faces are asked at each triangle of a face's mesh
// (faceModeOf): projecting every loop point again each time was two
// thirds of a drafted boss's shell, and grew with the triangle count.
type loopUVCache struct {
	mu  sync.Mutex
	uv  map[loopUVKey][]geom.Vector
	box map[*srf.SSurface][2]geom.Vector
}

// loopUVKey is a loop on the surface its outline was taken on.
type loopUVKey struct {
	loop int
	surf *srf.SSurface
}

// faceLoopUV is loop l's points in the parameters of s, its face's
// surface (loopUV), worked out once per topology. The topology's loops
// and edges don't change once it is built, so the outline doesn't.
func (t *Topology) faceLoopUV(l int, s *srf.SSurface) []geom.Vector {
	t.uvOnce.Do(func() { t.uvCache = t.newLoopUVCache() })
	c := t.uvCache
	k := loopUVKey{l, s}
	c.mu.Lock()
	uv, ok := c.uv[k]
	c.mu.Unlock()
	if ok {
		return uv
	}
	uv = t.loopUV(s, t.Loops[l].CoEdges)
	c.mu.Lock()
	c.uv[k] = uv
	c.mu.Unlock()
	return uv
}

func (t *Topology) newLoopUVCache() *loopUVCache {
	return &loopUVCache{uv: map[loopUVKey][]geom.Vector{}, box: map[*srf.SSurface][2]geom.Vector{}}
}

// surfaceBoxDistance is how far p lies from the box of a face's surface
// s's control points, which holds the whole patch: no point of the face
// is nearer.
func (t *Topology) surfaceBoxDistance(s *srf.SSurface, p geom.Vector) float64 {
	t.uvOnce.Do(func() { t.uvCache = t.newLoopUVCache() })
	c := t.uvCache
	c.mu.Lock()
	b, ok := c.box[s]
	if !ok {
		s.AxisAlignedBounding(&b[1], &b[0])
		c.box[s] = b
	}
	c.mu.Unlock()
	gap := func(x, lo, hi float64) float64 { return max(lo-x, 0, x-hi) }
	dx, dy, dz := gap(p.X, b[0].X, b[1].X), gap(p.Y, b[0].Y, b[1].Y), gap(p.Z, b[0].Z, b[1].Z)
	return math.Sqrt(dx*dx + dy*dy + dz*dz)
}
