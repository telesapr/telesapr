package sketch

import (
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/geom"
)

// A corner out of the plane of the arc beside it can't be rounded; one
// that turns back on itself has no circle; offsets that don't cross
// give no centre.
func TestRoundCornerRefusals(t *testing.T) {
	t.Parallel()
	sk := &Sketch{}
	up := pathPiece{start: geom.Vector{}, end: geom.Vector{Z: 10}}
	// An arc in a plane across the line's: it turns about z.
	arc := pathPiece{start: geom.Vector{Z: 10}, end: geom.Vector{X: 5, Y: 5, Z: 10}, arc: true,
		center: geom.Vector{X: 5, Z: 10}, axis: geom.Vector{Z: 1}, angle: 1.5}
	if _, _, _, err := sk.roundCorner(up, arc, 1); err == nil || !strings.Contains(err.Error(), "out of the plane") {
		t.Errorf("out of the arc's plane: %v", err)
	}
	back := pathPiece{start: geom.Vector{Z: 10}, end: geom.Vector{}}
	if _, _, _, err := sk.roundCorner(up, back, 1); err == nil || !strings.Contains(err.Error(), "turns back") {
		t.Errorf("back on itself: %v", err)
	}
	if got := crossOffsets(true, [2]float64{0, 1}, [2]float64{1, 0}, [2]float64{}, 0,
		true, [2]float64{0, 2}, [2]float64{1, 0}, [2]float64{}, 0); got != nil {
		t.Errorf("parallel lines: %v", got)
	}
	if got := lineCircle([2]float64{0, 5}, [2]float64{1, 0}, [2]float64{}, 1); got != nil {
		t.Errorf("a line missing a circle: %v", got)
	}
	if got := circleCircle([2]float64{}, 1, [2]float64{5, 0}, 1); got != nil {
		t.Errorf("circles apart: %v", got)
	}
	if got := circleCircle([2]float64{}, 1, [2]float64{1, 0}, 1); len(got) != 2 {
		t.Errorf("circles crossing: %v", got)
	}
}
