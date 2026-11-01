package ui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/geom"
)

// TestDescribeFaceNormalOutward selects the top of an extruded
// rectangle, drawn one way round and the other, and its bottom from
// below: the screen gives each face's normal out of the solid. The
// face entities' own normals follow the winding of the rectangle, and
// one end's pointed into the solid, which end depending on which way
// round the rectangle was drawn.
func TestDescribeFaceNormalOutward(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name, corner, view, want string
	}{
		{"up and right, top", "500 170", "default", "normal=(0.000,0.000,1.000)"},
		{"down and right, top", "500 430", "default", "normal=(0.000,0.000,1.000)"},
		{"up and right, bottom", "500 170", "under", "normal=(0.000,0.000,-1.000)"},
		{"down and right, bottom", "500 430", "under", "normal=(0.000,0.000,-1.000)"},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			r := newUIScriptRunner(t)
			run := func(lines ...string) {
				for _, line := range lines {
					r.step(line)
					r.settle()
				}
			}
			run("key r", "move 300 300", "down L 300 300", "up L 300 300",
				"move "+c.corner, "down L "+c.corner, "up L "+c.corner, "key esc",
				"key x shift", "view "+c.view)
			// The rectangle runs from the origin to the corner, 5
			// pixels to the mm. Seen from below, the top's edges are
			// picked through the solid, so a few points of the face are
			// tried until one picks it alone.
			var cx, cy float64
			if _, err := fmt.Sscan(c.corner, &cx, &cy); err != nil {
				t.Fatal(err)
			}
			sk := r.ss.SK
			z := 0.0
			if c.view == "default" {
				z = sk.Group(r.ss.GW.activeGroupH()).MeasuredExtent(sk)
			}
			var rows string
			for _, f := range [][2]float64{{0.5, 0.5}, {0.3, 0.7}, {0.7, 0.3}, {0.2, 0.8}, {0.8, 0.8}} {
				at := r.ss.GW.ProjectPoint(geom.Vector{X: f[0] * (cx - 300) / 5, Y: f[1] * (300 - cy) / 5, Z: z})
				xy := fmt.Sprintf("%g %g", 300+at.U, 300-at.V)
				run("key esc", "move "+xy, "down L "+xy, "up L "+xy)
				var b strings.Builder
				r.ss.TW.dumpRows(&b)
				if rows = b.String(); strings.Contains(rows, "PLANE FACE") {
					break
				}
			}
			if !strings.Contains(rows, "PLANE FACE") {
				t.Fatalf("no face selected:\n%s", rows)
			}
			flat := strings.NewReplacer(" ", "", "-0.000", "0.000").Replace(rows)
			if !strings.Contains(flat, c.want) {
				t.Errorf("want %s:\n%s", c.want, rows)
			}
		})
	}
}
