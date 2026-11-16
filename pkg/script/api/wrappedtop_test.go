package api

import (
	"math"
	"testing"
)

// The top edges of a pointed arch of a band embossed 0.4 on the tube
// (wrappedArch), chamfered and rounded at 0.2: ten convex edges between
// the band's top, a cylinder, and its sides, flat along the legs and
// ruled round the arcs. At the arch's inner apex the sides meet in a
// concave crease, where a chamfer's planes, carried on past it, stay
// above each other's lines all the way back from where the mitre is
// looked for (insideMitreRoot).
//
// Across a leg the side is a plane through the axis and the top a circle
// of radius R, so where the rounding and the cut leave the faces is known
// exactly: the ball's centre lies r in from the side and r under the top,
// touching the side at sqrt((R-r)²-r²) from the axis and the top
// asin(r/(R-r)) round from the edge; the chamfer leaves the side at R-d
// and the top d along it, d/R round. OpenCASCADE's fillet of the same
// edges, on the tube of the barrel this came from (R 7.85), touches at
// 7.6474 and 1.498° as this does, and our faces lie within 0.00004 mm of
// its; its chamfer is no oracle here, cutting one edge of a leg 0.29
// along the top and the other 0.11.
func TestWrappedArchTop(t *testing.T) {
	if testing.Short() {
		t.Skip("ten rolling-ball chains with mitres at both apexes, half a minute each")
	}
	t.Parallel()
	const h, r = 0.4, 0.2
	R := 7.3 + h
	for _, kind := range []string{"fillet", "chamfer"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			d := New()
			band := wrappedArch(d, wrappedTube(d), h)
			if p := band.Problem(); p != "" {
				t.Fatal(p)
			}
			before := band.Analyze().Check().Volume
			var top []SolidEdge
			var legs []float64
			for _, e := range band.Solid().Edges() {
				on := true
				for _, p := range e.Points() {
					on = on && math.Abs(math.Hypot(p.Y, p.Z)-R) < 1e-6
				}
				if !on {
					continue
				}
				top = append(top, e)
				if e.Straight() {
					p := e.Points()[0]
					legs = append(legs, math.Atan2(p.Y, p.Z))
				}
			}
			if len(top) != 10 || len(legs) != 4 {
				t.Fatalf("the arch's top is %d edges, %d of them straight; want 10 and 4", len(top), len(legs))
			}
			var g Group
			wall, round := math.Sqrt((R-r)*(R-r)-r*r), math.Asin(r/(R-r))
			if kind == "fillet" {
				g = d.Features.FilletSolid(top, r, "")
			} else {
				g = d.Features.ChamferSolid(top, r, "")
				wall, round = R-r, r/R
			}
			if p := g.Problem(); p != "" {
				t.Fatal(p)
			}
			c := g.Analyze().Check()
			if took := before - c.Volume; !c.Sound() || took <= 0 {
				t.Errorf("%s, took %g", c.Summary(), took)
			}
			// Each leg's edge leaves a line on its side and one on the top,
			// part of a chain that goes on round the arch: the points of the
			// edges over the legs' length that lie near either line lie on it.
			type near struct {
				n     int
				worst float64
			}
			for _, at := range legs {
				var side, onTop near
				for _, e := range g.Solid().Edges() {
					for _, p := range e.Points() {
						if p.X < 11 || p.X > 29 {
							continue
						}
						rr := math.Hypot(p.Y, p.Z)
						turn := math.Abs(math.Remainder(math.Atan2(p.Y, p.Z)-at, 2*math.Pi))
						for _, c := range []struct {
							l          *near
							r, off, rr float64
						}{{&side, wall, turn * R, rr}, {&onTop, R, math.Abs(turn-round) * R, rr}} {
							if dev := math.Hypot(c.rr-c.r, c.off); dev < 0.01 {
								c.l.n++
								c.l.worst = math.Max(c.l.worst, dev)
							}
						}
					}
				}
				if side.n == 0 || onTop.n == 0 || side.worst > 5e-4 || onTop.worst > 5e-4 {
					t.Errorf("the leg's edge at %.4f rad: on its side at %.4f %d points, off by %.2g; on the top %.4f round %d, off by %.2g",
						at, wall, side.n, side.worst, round, onTop.n, onTop.worst)
				}
			}
		})
	}
}

