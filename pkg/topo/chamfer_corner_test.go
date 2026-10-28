package topo_test

import (
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/topo"
)

// trianglePrism is the prism of height 20 on the triangle of corners
// (0,0), (20,0) and 20 along the angle th from the x axis, at the
// tolerance tol.
func trianglePrism(th float64, tol srf.Tolerance) *srf.SShell {
	sh := srf.SShell{Tol: tol}
	sh.MakePrism([]geom.Vector{v(0, 0, 0), v(20, 0, 0), v(20*math.Cos(th), 20*math.Sin(th), 0)}, v(0, 0, 0), v(0, 0, 20),
		core.NewRGBA(100, 100, 100, 255))
	for i := range sh.Surface {
		sh.Surface[i].Face = srf.FaceID(i + 1)
	}
	return &sh
}

// planesOnly reports whether every surface of sh is a plane.
func planesOnly(sh *srf.SShell) bool {
	for _, s := range sh.Surface {
		if s.DegM != 1 || s.DegN != 1 {
			return false
		}
	}
	return true
}

// A corner of three chamfers of one distance is cut by a plane, not a
// patch of a sphere, and the volume is OpenCASCADE's makeChamfer's: at a
// box's corner the triangle of the points where the strips' lines
// cross; at the foot of an equilateral prism's upright edge, where the
// faces don't all meet square, a four-sided face, the strip along one
// bottom edge ending between where its lines cross the others' and the
// other two cut square to their edges.
func TestChamferCornerOfOneDistance(t *testing.T) {
	t.Parallel()
	sh := box(srf.Tol(0.002, 20))
	picks := []topo.FilletPick{
		{A: v(10, 20, 30), B: v(10, 20, 0)},
		{A: v(10, 20, 30), B: v(0, 20, 30)},
		{A: v(10, 20, 30), B: v(10, 0, 30)}}
	if err := topo.FilletPicks(sh, picks, 1, 1<<30, true); err != nil {
		t.Fatal(err)
	}
	// The three prisms less what OpenCASCADE's 10 cube gives back at
	// the corner (15 - 14.3333).
	want := 6000 - (30+10+20)*0.5 + 2.0/3
	if got := sound(t, sh); math.Abs(got-want) > 1e-6 || !planesOnly(sh) {
		t.Errorf("box corner: volume %.9f, want %.9f; planes only %v", got, want, planesOnly(sh))
	}

	sh = trianglePrism(math.Pi/3, srf.Tol(0.002, 20))
	picks = []topo.FilletPick{
		{A: v(0, 0, 0), B: v(20, 0, 0)},
		{A: v(0, 0, 0), B: v(10, 20*math.Sin(math.Pi/3), 0)},
		{A: v(0, 0, 0), B: v(0, 0, 20)}}
	if err := topo.FilletPicks(sh, picks, 1, 1<<30, true); err != nil {
		t.Fatal(err)
	}
	// OpenCASCADE's volume, FreeCAD's Part::Chamfer of 1 on the three
	// edges.
	const occ = 3436.447005377
	if got := sound(t, sh); math.Abs(got-occ) > 1e-4 {
		t.Errorf("prism corner: volume %.9f, OpenCASCADE's %.9f", got, occ)
	}
}
