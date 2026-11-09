package topo_test

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/internal/sweep"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/topo"
)

// Rim edges of a pocket in a flat face (pocketed) of two radii that
// meet at inside corners: a kinked mitre (rollingmitre_inside.go), the
// smaller fillet carried past the crease up to the larger's swept
// surface, the larger cut by that surface above the smaller's foot and
// by the smaller's wall, grown past the crease, below it. OpenCASCADE's
// makeFillet and makeChamfer of the same edges (2026-11-09,
// FreeCADCmd) took the volumes below; its chamfers' corners take
// d²D/2 − d³/6 each, d the smaller distance and D the larger, exactly as
// ours do. Our fillets' meshes, at this tolerance, take up to 0.17% more
// than OpenCASCADE's exact volumes, as the meshes of a fillet of one
// radius do (TestFilletPocketRimFlat).
func TestFilletPocketRimRadii(t *testing.T) {
	t.Parallel()
	rim := map[string]topo.FilletPick{
		"y-": {A: v(10, 10, 20), B: v(30, 10, 20)},
		"x+": {A: v(30, 10, 20), B: v(30, 20, 20)},
		"y+": {A: v(30, 20, 20), B: v(10, 20, 20)},
		"x-": {A: v(10, 20, 20), B: v(10, 10, 20)},
	}
	for _, c := range sweep.Pick([]struct {
		picks []string
		radii []float64
		flat  bool
		occ   float64
	}{
		{[]string{"y-", "x+"}, []float64{1, 1.5}, false, 9.292513},
		{[]string{"y-", "x+"}, []float64{1.5, 1}, false, 11.975036},
		{[]string{"y-", "x+"}, []float64{1, 2}, false, 13.129610},
		{[]string{"y-", "x+"}, []float64{2, 1}, false, 19.567665},
		{[]string{"y-", "x+"}, []float64{0.5, 2}, false, 9.732668},
		{[]string{"y-", "x+", "y+"}, []float64{1, 1.5, 1}, false, 13.756485},
		{[]string{"y-", "x+", "y+"}, []float64{1, 1, 2}, false, 23.955571},
		{[]string{"y-", "x+", "y+"}, []float64{2, 1, 0.5}, false, 20.672361},
		// The mirror image of the line above, kept: the chain kinked at its
		// first end and copying the larger's mitre at its second, whose
		// mate must cut the crease first (cutOrder).
		{[]string{"y-", "x+", "y+"}, []float64{0.5, 1, 2}, false, 20.672361},
		{[]string{"y-", "x+", "y+", "x-"}, []float64{1, 1.5, 1, 1.5}, false, 18.928896},
		{[]string{"y-", "x+"}, []float64{1, 1.5}, true, 21.833333},
		{[]string{"y-", "x+"}, []float64{2, 1}, true, 45.833333},
		{[]string{"y-", "x+"}, []float64{0.5, 2}, true, 22.729167},
		{[]string{"y-", "x+", "y+"}, []float64{1, 1.5, 1}, true, 32.416667},
		{[]string{"y-", "x+", "y+"}, []float64{2, 1, 0.5}, true, 48.4375},
		{[]string{"y-", "x+", "y+"}, []float64{0.5, 1, 2}, true, 48.4375},
		{[]string{"y-", "x+", "y+", "x-"}, []float64{1, 1.5, 1, 1.5}, true, 44.833333},
	}, 0) {
		t.Run(fmt.Sprintf("%s %v flat %v", strings.Join(c.picks, ","), c.radii, c.flat), func(t *testing.T) {
			t.Parallel()
			sh := pocketed(srf.Tol(0.001, 20))
			var picks []topo.FilletPick
			for i, k := range c.picks {
				p := rim[k]
				p.R = c.radii[i]
				picks = append(picks, p)
			}
			if err := topo.FilletPicks(sh, picks, 1, 1<<30, c.flat); err != nil {
				t.Fatal(err)
			}
			took := 40*30*20 - 20*10*12 - sound(t, sh)
			lo, hi := c.occ*(1-1e-6), c.occ*(1+0.003)
			if c.flat {
				hi = c.occ * (1 + 1e-6)
			}
			if took < lo || took > hi {
				t.Errorf("took %g, OpenCASCADE %g", took, c.occ)
			}
		})
	}
}

// The same of the rim of a pocket in a cone (dishPocket), where the two
// fillets' feet lie at two depths on the crease. OpenCASCADE's
// makeFillet took the volumes below (2026-11-09). Its faces and ours,
// our solid exported as STEP, lie within 0.0006 mm of each other within
// 4 mm of each corner both chains reach, sampled both ways, but where
// the smaller fillet's wall grows past the crease beyond the wall's own
// patch: the STEP file bounds the face by its patch and leaves the
// growth out, 0.11 mm off at a radius of 1 beside 2. The meshes here
// are coarser, some 4% off, and the volume only rules out a corner gone
// wrong. A chain that meets one of another radius at one corner meets
// one of its own radius at its other end in a kinked mitre too, kinked
// at the deeper foot, in place of a corner patch (insideMitres), and so
// on round the rim: the last five lines, whose corners of one radius
// lie within 0.0005 mm of OpenCASCADE's (2026-11-09).
func TestFilletPivotRadii(t *testing.T) {
	if testing.Short() {
		t.Skip("rolling-ball fillets of two radii of a pocket's rim in a cone, slow")
	}
	t.Parallel()
	cases := sweep.Pick([]struct {
		picks []string
		radii []float64
		occ   float64
	}{
		{[]string{"x+", "y+"}, []float64{1, 1.5}, 7.369233},
		{[]string{"x+", "y+"}, []float64{1.5, 1}, 5.308922},
		{[]string{"x+", "y+"}, []float64{1, 2}, 12.112489},
		{[]string{"x+", "y+"}, []float64{2, 1}, 7.238776},
		{[]string{"x+", "y+"}, []float64{0.5, 2}, 11.227978},
		{[]string{"x+", "x-", "y+"}, []float64{1, 1, 1.5}, 8.513201},
		{[]string{"x+", "y+", "x-", "y-"}, []float64{1, 1.5, 1, 1.5}, 14.852177},
		{[]string{"x+", "y+", "x-"}, []float64{2, 1, 1}, 8.356940},
		{[]string{"x+", "y+", "x-", "y-"}, []float64{2, 1, 1, 1}, 11.289349},
		{[]string{"x+", "y+", "x-", "y-"}, []float64{1, 1, 1.5, 1}, 9.319660},
		{[]string{"y+", "x-", "y-"}, []float64{1, 1, 2}, 14.927771},
		{[]string{"x+", "y+", "x-", "y-"}, []float64{2, 1, 2, 1}, 14.671230},
	}, 0)
	for _, c := range cases {
		t.Run(fmt.Sprintf("%s %v", strings.Join(c.picks, ","), c.radii), func(t *testing.T) {
			t.Parallel()
			sh := dishPocket(fine(0.01))
			before := meshVolume(sh)
			picks := dishRim(c.picks...)
			for i := range picks {
				picks[i].R = c.radii[i]
			}
			if err := topo.FilletPicks(sh, picks, 1, 1<<30, false); err != nil {
				t.Fatal(err)
			}
			took := before - sound(t, sh)
			if math.Abs(took-c.occ) > 0.06*c.occ {
				t.Errorf("took %g, OpenCASCADE %g", took, c.occ)
			}
		})
	}
}
