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
		// Lines above with the picks in the other order, against the
		// rim: step 10 meets the corner of two radii before an end
		// running back along the rim, and its stages and setbacks fail
		// at that end; they were refused, "runs back along it", until
		// the rolling ball was tried after them (filletRunsBackLast).
		{[]string{"y+", "x+", "y-"}, []float64{0.5, 1, 2}, false, 20.672361},
		{[]string{"x+", "y-"}, []float64{1, 2}, false, 19.567665},
		{[]string{"x+", "y-"}, []float64{1.5, 1}, false, 9.292513},
		{[]string{"x+", "y-"}, []float64{2, 0.5}, false, 9.732668},
		{[]string{"y+", "x+", "y-"}, []float64{0.5, 1, 2}, true, 48.4375},
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
			if off, where := stepEdgesOffFaces(t, sh); off > 1e-6 {
				t.Errorf("written as STEP, an edge lies %.3g off its face: %s", off, where)
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
// 4 mm of each corner both chains reach, sampled both ways. Where the
// smaller fillet's wall grows past the crease beyond the wall's own
// patch, the STEP file bounded the face by its patch and left the
// growth out, 0.14 mm off at a radius of 1 beside 2, until a plane face
// was written over its trims (srf.SSurface.OverCurves, 2026-11-17;
// within 0.00001 mm of our mesh since, read by FreeCADCmd). The volume
// is the mesh's carried to its surfaces (surfaceVolume) and taken from
// the dish's closed form: within 0.14% of OpenCASCADE's, most within
// 0.05%. The meshes' own volumes, the dish's meshed before less after,
// took 1.5% to 4.6% more here and up to 7% less at 0.002 mm, the cone's
// chords sagging differently once it is trimmed anew (2026-11-18). A
// chain that meets one of another radius at one corner meets
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
			picks := dishRim(c.picks...)
			for i := range picks {
				picks[i].R = c.radii[i]
			}
			if err := topo.FilletPicks(sh, picks, 1, 1<<30, false); err != nil {
				t.Fatal(err)
			}
			if off, where := stepEdgesOffFaces(t, sh); off > 1e-6 {
				t.Errorf("written as STEP, an edge lies %.3g off its face: %s", off, where)
			}
			sound(t, sh)
			took := dishVolume() - surfaceVolume(t, sh)
			if math.Abs(took-c.occ) > 0.002*c.occ {
				t.Errorf("took %g, OpenCASCADE %g", took, c.occ)
			}
		})
	}
}

// Chamfers of two distances of the same rim, the same kinked mitres.
// They were refused, "don't meet in a mitre here", until a chamfer's
// mate was taken as a plane in insideMitreRoot (2026-11-16, for a band's
// top round a pointed arch's tip). OpenCASCADE's makeChamfer of the same
// edges, each of its distance both ways, took the volumes below
// (2026-11-18, FreeCADCmd); its faces and ours, our solid exported as
// STEP, lie within 0.0006 mm of each other near each corner both chains
// reach, sampled both ways, and within 0.003 mm along the rest of the
// rim, the most at an end left against a wall not picked. Our solids,
// the mesh carried to its surfaces (surfaceVolume), take within 0.035%
// of its volumes at this tolerance, and within 0.003% of the first line's
// at 0.002 mm (TestDishPocketFineVolumes). The meshes' own volumes, the
// dish's meshed before less after, took up to 0.7% more here and 1% to
// 2% less at 0.002 mm: the cone's chords, not the corners.
func TestChamferPivotRadii(t *testing.T) {
	if testing.Short() {
		t.Skip("rolling-ball chamfers of two distances of a pocket's rim in a cone, slow")
	}
	t.Parallel()
	cases := sweep.Pick([]struct {
		picks []string
		dists []float64
		occ   float64
	}{
		{[]string{"x+", "y+"}, []float64{1, 1.5}, 32.852706},
		{[]string{"x+", "y+"}, []float64{1.5, 1}, 25.183713},
		{[]string{"x+", "y+"}, []float64{1, 2}, 53.670227},
		{[]string{"x+", "y+"}, []float64{2, 1}, 35.336555},
		{[]string{"x+", "y+"}, []float64{0.5, 2}, 48.841104},
		{[]string{"x+", "x-", "y+"}, []float64{1, 1, 1.5}, 39.077905},
		{[]string{"x+", "y+", "x-", "y-"}, []float64{1, 1.5, 1, 1.5}, 66.816747},
		{[]string{"x+", "y+", "x-"}, []float64{2, 1, 1}, 41.327499},
		{[]string{"x+", "y+", "x-", "y-"}, []float64{2, 1, 1, 1}, 54.327832},
		{[]string{"x+", "y+", "x-", "y-"}, []float64{1, 1, 1.5, 1}, 43.930187},
		{[]string{"y+", "x-", "y-"}, []float64{1, 1, 2}, 65.858070},
		{[]string{"x+", "y+", "x-", "y-"}, []float64{2, 1, 2, 1}, 72.298346},
	}, 0)
	for _, c := range cases {
		t.Run(fmt.Sprintf("%s %v", strings.Join(c.picks, ","), c.dists), func(t *testing.T) {
			t.Parallel()
			sh := dishPocket(fine(0.01))
			picks := dishRim(c.picks...)
			for i := range picks {
				picks[i].R = c.dists[i]
			}
			if err := topo.FilletPicks(sh, picks, 1, 1<<30, true); err != nil {
				t.Fatal(err)
			}
			if off, where := stepEdgesOffFaces(t, sh); off > 1e-6 {
				t.Errorf("written as STEP, an edge lies %.3g off its face: %s", off, where)
			}
			sound(t, sh)
			took := dishVolume() - surfaceVolume(t, sh)
			if math.Abs(took-c.occ) > 0.0005*c.occ {
				t.Errorf("took %g, OpenCASCADE %g", took, c.occ)
			}
		})
	}
}
