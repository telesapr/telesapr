package sketch

import (
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// meshBall appends a ball of radius r about c, cut into stacks and
// twice as many slices, its triangles facing out.
func meshBall(m *srf.SMesh, c geom.Vector, r float64, stacks int) {
	slices := 2 * stacks
	at := func(i, j int) geom.Vector {
		th := math.Pi * float64(i) / float64(stacks)
		ph := 2 * math.Pi * float64(j) / float64(slices)
		return c.Plus(geom.Vector{X: r * math.Sin(th) * math.Cos(ph), Y: r * math.Sin(th) * math.Sin(ph), Z: r * math.Cos(th)})
	}
	for i := range stacks {
		for j := range slices {
			a, b, cc, d := at(i, j), at(i+1, j), at(i+1, j+1), at(i, j+1)
			if i > 0 {
				m.AddTriangle(srf.STriangle{Tag: -1, A: a, B: b, C: d})
			}
			if i < stacks-1 {
				m.AddTriangle(srf.STriangle{Tag: -1, A: b, B: cc, C: d})
			}
		}
	}
}

// meshPitBlock appends a block 40 × 40 × 10 with a pit from its top, a
// triangle 0.3 across at its mouth narrowing to a point 0.2 over the
// floor, as TestWallThicknessUnderADimple's dimple: no ray measured
// from the floor passes under the pit's point, and the pit's own rays
// run sideways into the block.
func meshPitBlock(m *srf.SMesh) {
	v := func(x, y, z float64) geom.Vector { return geom.Vector{X: x, Y: y, Z: z} }
	sq := [][2]float64{{0, 0}, {40, 0}, {40, 40}, {0, 40}}
	add := func(a, b, c geom.Vector) { m.AddTriangle(srf.STriangle{Tag: -1, A: a, B: b, C: c}) }
	for i := range 4 {
		k := (i + 1) % 4
		b0, b1 := v(sq[i][0], sq[i][1], 0), v(sq[k][0], sq[k][1], 0)
		s0, s1 := v(sq[i][0], sq[i][1], 10), v(sq[k][0], sq[k][1], 10)
		add(b0, b1, s1)
		add(b0, s1, s0)
	}
	add(v(0, 0, 0), v(40, 40, 0), v(40, 0, 0))
	add(v(0, 0, 0), v(0, 40, 0), v(40, 40, 0))
	s := []geom.Vector{v(0, 0, 10), v(40, 0, 10), v(40, 40, 10), v(0, 40, 10)}
	h := []geom.Vector{v(23.8, 19.85, 10), v(24.1, 20, 10), v(23.8, 20.15, 10)}
	add(s[0], s[1], h[0])
	add(s[1], h[1], h[0])
	add(s[1], s[2], h[1])
	add(s[2], h[2], h[1])
	add(s[2], s[3], h[2])
	add(s[3], h[0], h[2])
	add(s[3], s[0], h[0])
	p := v(23.9, 20, 0.2)
	add(h[0], h[1], p)
	add(h[1], h[2], p)
	add(h[2], h[0], p)
}

// A part of a fine ball 0.8 across and, after it in the mesh, a block
// with a pit whose point is 0.2 over the floor. Every pair of the ball's
// triangles facing each other lies within its diameter, the thinnest
// the rays find: the box round each one's prism took in over 4M pairs,
// past which the rest, the block's among them, were left, and its
// thinnest, 0.2 under the pit, missed. The prisms looked up along their
// centres' tracks take in a few pairs a triangle, and find it.
func TestWallNearPastManyPairs(t *testing.T) {
	if testing.Short() {
		t.Skip("a ball of 159k triangles: seconds")
	}
	t.Parallel()
	w := &clearancePart{chord: 1e-4}
	meshBall(&w.mesh, geom.Vector{X: 100, Y: 100, Z: 5}, 0.4, 200)
	meshPitBlock(&w.mesh)
	w.tree = srf.NewMeshTree(&w.mesh)
	r := WallReport{Thinnest: ThinSpot{Thickness: math.Inf(1)}}
	r.Step = wallStep([]*clearancePart{w}, 0)
	r.measure(w, 0)
	if r.NearCut || math.Abs(r.Thinnest.Thickness-0.2) > 1e-5 {
		t.Fatalf("thinnest %g at %v (cut %v), want 0.2 under the pit's point", r.Thinnest.Thickness, r.Thinnest.From, r.NearCut)
	}
}
