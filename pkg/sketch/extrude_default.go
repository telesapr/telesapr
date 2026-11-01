package sketch

import (
	"math"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
)

// fallbackExtrudeDepth is the depth a new extrusion starts at when its
// sketch has nothing to measure, in mm.
const fallbackExtrudeDepth = 10

// defaultExtrudeDepth is the depth the extrusion g starts at, before
// anything holds it: half the longer side of the bounding box of the
// sketch's profile, measured in its workplane, taken to the nearest of
// 1, 2 or 5 times a power of ten. A 40 by 25
// rectangle gives 20, a circle 10 across 5. It depends on the sketch
// alone, so an extrusion comes out the same depth whatever the view's
// zoom; a round number is one a user can leave as it is. A sketch with
// no closed loop is measured by its open curves; one with nothing to
// measure gives 10.
func (g *Group) defaultExtrudeDepth(sk *Sketch) float64 {
	src := sk.Group(g.OpA)
	if src == nil {
		return fallbackExtrudeDepth
	}
	// One with no workplane is measured in x and y.
	u, v := geom.Vector{X: 1}, geom.Vector{Y: 1}
	if w := sk.Entity(g.Predef.EntityB); !core.IsFreeIn3D(g.Predef.EntityB) && w != nil && w.IsWorkplane() {
		nq := w.NormalEntity(sk)
		u, v = nq.NormalU(sk), nq.NormalV(sk)
	}
	var b profileBounds
	for i := range src.PolyLoops.Contours {
		for _, p := range src.PolyLoops.Contours[i].Points {
			b.add(p.P, u, v)
		}
	}
	if b.n == 0 {
		for _, l := range src.BezierOpens {
			for i := range l.Curves {
				b.add(l.Curves[i].Start(), u, v)
				b.add(l.Curves[i].Finish(), u, v)
			}
		}
	}
	return roundDepth(b.longerSide() / 2)
}

// profileBounds is the bounding box of points in a workplane's u and v.
type profileBounds struct {
	n                      int
	minU, maxU, minV, maxV float64
}

// add puts p, measured along u and v, in the box.
func (b *profileBounds) add(p, u, v geom.Vector) {
	pu, pv := p.Dot(u), p.Dot(v)
	if b.n == 0 {
		b.minU, b.maxU, b.minV, b.maxV = pu, pu, pv, pv
	}
	b.minU, b.maxU = min(b.minU, pu), max(b.maxU, pu)
	b.minV, b.maxV = min(b.minV, pv), max(b.maxV, pv)
	b.n++
}

// longerSide is the longer side of the box, 0 for an empty one.
func (b *profileBounds) longerSide() float64 {
	if b.n == 0 {
		return 0
	}
	return max(b.maxU-b.minU, b.maxV-b.minV)
}

// roundDepth takes d to the nearest of 1, 2 or 5 times a power of ten,
// nearest by ratio, so that a profile a hair under a round size, as a
// circle's polygon is, still gives that size; 0 or less gives the
// fallback.
func roundDepth(d float64) float64 {
	if !(d > geom.LengthEps) {
		return fallbackExtrudeDepth
	}
	decade := math.Pow(10, math.Floor(math.Log10(d)))
	m := d / decade // in [1, 10)
	switch {
	case m < math.Sqrt2:
		m = 1
	case m < math.Sqrt(10):
		m = 2
	case m < math.Sqrt(50):
		m = 5
	default:
		m = 10
	}
	return m * decade
}
