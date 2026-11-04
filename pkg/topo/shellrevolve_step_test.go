package topo_test

import (
	"math"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/topo"
)

// A revolve's curved face taken away where it meets a flat face at an
// inside corner: the inside steps along the flat face's plane round the
// solid's corner, so the wall under the flat face keeps its thickness.
// Each stepped shaft below, walls 1, keeps a bottom of 1 and radius 10,
// a rim from 9 to 10 and 1 to 5 high, and a step from 6 to 9 and 4 to 5
// high: 100π + 76π + 45π = 221π, whatever the narrow part above the step
// was, when it and its end are taken away.
func TestShellRevolvedInsideCornerStep(t *testing.T) {
	t.Parallel()
	cup := 221 * math.Pi
	// Fine enough that the mesh's volume is within the test's 0.2%.
	fine := srf.Tol(0.002, 20)
	cases := []struct {
		name string
		pts  [][2]float64
		arcs map[int][2]float64
		// open are points on the faces taken away, (r, z), each face
		// found at four angles.
		open [][2]float64
		tol  srf.Tolerance
		want float64
	}{
		{"a cylinder above the step, and its top", [][2]float64{{0, 0}, {10, 0}, {10, 5}, {6, 5}, {6, 10}, {0, 10}}, nil,
			[][2]float64{{6, 8}, {3, 10}}, fine, cup},
		// The top kept: it stands apart from the cup, a disc 6 across and
		// 1 thick, as the side between them was taken away.
		{"a cylinder above the step, its top kept", [][2]float64{{0, 0}, {10, 0}, {10, 5}, {6, 5}, {6, 10}, {0, 10}}, nil,
			[][2]float64{{6, 8}}, fine, cup + 36*math.Pi},
		{"a cone above the step, and its top", [][2]float64{{0, 0}, {10, 0}, {10, 5}, {6, 5}, {4, 10}, {0, 10}}, nil,
			[][2]float64{{5, 7.5}, {2, 10}}, fine, cup},
		// The cylinder below the step, the corner at the side's end.
		{"a cylinder below the step, and its bottom", [][2]float64{{0, 0}, {6, 0}, {6, 5}, {10, 5}, {10, 10}, {0, 10}}, nil,
			[][2]float64{{6, 2}, {3, 0}}, fine, cup},
		// A quarter of a sphere of radius 6 standing on the step: the
		// inside runs up the sphere's own circle to the corner.
		{"a dome above the step", [][2]float64{{0, 0}, {10, 0}, {10, 5}, {6, 5}, {0, 11}}, map[int][2]float64{3: {0, 5}},
			[][2]float64{{6 * math.Cos(0.7), 5 + 6*math.Sin(0.7)}}, fine, cup},
	}
	for _, c := range cases {
		sh, rev := turned(meridian(t, c.pts, c.arcs, c.tol), 0, 2*math.Pi, c.tol)
		tp := build(t, sh)
		var open []int
		for _, rz := range c.open {
			for k := range 4 {
				a := math.Pi/4 + float64(k)*math.Pi/2
				f, d := tp.FaceAt(v(rz[0]*math.Cos(a), rz[0]*math.Sin(a), rz[1]))
				if f < 0 || d > 1e-6 {
					t.Fatalf("%s: no face at %v", c.name, rz)
				}
				open = append(open, f)
			}
		}
		out, err := topo.ShellRevolved(sh, rev, open, 1, 1<<30)
		if err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		if got := sound(t, out); math.Abs(got-c.want) > 0.002*c.want {
			t.Errorf("%s: volume %g, want %g", c.name, got, c.want)
		}
	}
}

// Where the face beside the inside corner is curved or sloping, the
// step has no plane to run along: refused, saying so.
func TestShellRevolvedInsideCornerSloping(t *testing.T) {
	t.Parallel()
	// A cylinder of radius 6 standing on a cone that falls from 10 to 6.
	pts := [][2]float64{{0, 0}, {10, 0}, {10, 3}, {6, 5}, {6, 10}, {0, 10}}
	sh, rev := turned(meridian(t, pts, nil, srf.Tolerance{}), 0, 2*math.Pi, srf.Tolerance{})
	tp := build(t, sh)
	var open []int
	for k := range 4 {
		a := math.Pi/4 + float64(k)*math.Pi/2
		f, _ := tp.FaceAt(v(6*math.Cos(a), 6*math.Sin(a), 8))
		open = append(open, f)
	}
	_, err := topo.ShellRevolved(sh, rev, open, 1, 1<<30)
	if err == nil || !strings.Contains(err.Error(), "inside corner") {
		t.Errorf("beside a cone: %v", err)
	}
	// A step 0.5 wide, then a second rise: the run-out by the thickness,
	// 1, would cut into the rise.
	pts = [][2]float64{{0, 0}, {10, 0}, {10, 8}, {6.5, 8}, {6.5, 5}, {6, 5}, {6, 10}, {0, 10}}
	sh, rev = turned(meridian(t, pts, nil, srf.Tolerance{}), 0, 2*math.Pi, srf.Tolerance{})
	tp = build(t, sh)
	open = open[:0]
	for k := range 4 {
		a := math.Pi/4 + float64(k)*math.Pi/2
		f, _ := tp.FaceAt(v(6*math.Cos(a), 6*math.Sin(a), 8))
		open = append(open, f)
	}
	_, err = topo.ShellRevolved(sh, rev, open, 1, 1<<30)
	if err == nil || !strings.Contains(err.Error(), "narrower") {
		t.Errorf("beside a narrow step: %v", err)
	}
}

// The same step on half a turn, its ends kept as walls: sound, its
// volume half the whole cup's and the ends' walls.
func TestShellRevolvedInsideCornerHalfTurn(t *testing.T) {
	t.Parallel()
	fine := srf.Tol(0.002, 20)
	pts := [][2]float64{{0, 0}, {10, 0}, {10, 5}, {6, 5}, {6, 10}, {0, 10}}
	sh, rev := turned(meridian(t, pts, nil, fine), 0, math.Pi, fine)
	tp := build(t, sh)
	var open []int
	for _, a := range []float64{math.Pi / 4, 3 * math.Pi / 4} {
		for _, rz := range [][2]float64{{6, 8}, {3, 10}} {
			f, d := tp.FaceAt(v(rz[0]*math.Cos(a), rz[0]*math.Sin(a), rz[1]))
			if f < 0 || d > 1e-6 {
				t.Fatalf("no face at %v", rz)
			}
			open = append(open, f)
		}
	}
	out, err := topo.ShellRevolved(sh, rev, open, 1, 1<<30)
	if err != nil {
		t.Fatal(err)
	}
	if got := sound(t, out); got <= 221*math.Pi/2 || got >= 221*math.Pi {
		t.Errorf("volume %g, want between half the whole cup's, %g, and the whole's", got, 221*math.Pi/2)
	}
}
