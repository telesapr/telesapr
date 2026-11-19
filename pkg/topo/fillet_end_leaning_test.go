package topo_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/internal/sweep"
	"github.com/telesapr/telesapr/pkg/topo"
)

// Rim edges of curvedPocket picked as chains that turn at the pocket's
// inside corners and end against a wall not picked, one that leans: the
// chain's end there is on a curved face, and its end sections are
// fitted to where the swept surface meets it, as a lone edge's are.
// Refused before 2026-11-19 ("the fillet's surface can't be fitted"):
// a chain pivoted at one end was blended only as a mitred group, which
// never fitted its other end's sections to their cuts. The volumes are
// OpenCASCADE's (makeChamfer, scripts/fillet-pivot-curved-fixtures.py
// with the pocket's rim measured within a box about it, which its whole
// dish's volume isn't exact enough for); ours take lo to hi less, as
// the corner patches do (TestFilletPivotCurvedCrease). In the dish with
// both walls leaning OpenCASCADE's volumes are off by a few tenths of a
// mm³ however measured (TestFilletEndLeaningWalls), and the band is
// wider.
func TestFilletEndLeaningWall(t *testing.T) {
	if testing.Short() {
		t.Skip("chamfers of a pocket's rim in the dish ending against a leaning wall, slow")
	}
	t.Parallel()
	for _, c := range sweep.Pick([]struct {
		curvedY     bool
		picks       string
		r           float64
		occ, lo, hi float64
	}{
		{false, "x-,y-", 1, 17.719210, -0.006, -0.002},
		{false, "x-,y-", 2, 71.238717, -0.01, -0.005},
		{false, "y+,x-,y-", 1.5, 67.184730, -0.01, -0.004},
		{true, "x-,y-", 1, 15.709651, -0.02, 0.005},
		{true, "x-,y-,x+", 1.5, 40.520750, -0.03, 0.005},
		{true, "y+,x-,y-", 1, 25.650403, -0.02, 0.01},
	}, 0) {
		t.Run(fmt.Sprintf("curvedY %v %s r%g", c.curvedY, c.picks, c.r), func(t *testing.T) {
			t.Parallel()
			sh := curvedPocket(true, c.curvedY, fine(0.01))
			before := surfaceVolume(t, sh)
			if err := topo.FilletPicks(sh, curvedRim(true, c.curvedY, strings.Split(c.picks, ",")...), c.r, 1<<30, true); err != nil {
				if fe, ok := errors.AsType[*topo.FilletError](err); ok {
					t.Fatalf("%v at %v", err, fe.At)
				}
				t.Fatal(err)
			}
			if off, where := stepEdgesOffFaces(t, sh); off > 1e-6 {
				t.Errorf("written as STEP, an edge lies %.3g off its face: %s", off, where)
			}
			sound(t, sh)
			took := before - surfaceVolume(t, sh)
			t.Logf("took %.6f, OpenCASCADE %.6f, %+.5f", took, c.occ, took/c.occ-1)
			if took < c.occ*(1+c.lo) || took > c.occ*(1+c.hi) {
				t.Errorf("took %.6f, OpenCASCADE %.6f", took, c.occ)
			}
		})
	}
}

// The dish's lone y+ rim edge with both walls leaning, filleted: it ends
// against the x- wall's plane and the leaning x+ wall, neither picked.
// OpenCASCADE's makeFillet took a fifth more at r 1 by its solids'
// volumes (1.2331), and a third more at r 0.5; but its fillet's faces lie
// within 0.0005 mm of the ball's envelope from the x- wall to the x+
// wall, as ours do, and the volume is its solids' integration's, off by
// a few tenths of a mm³ in 75 000 (a fine tessellation of the same two
// solids differs by 0.948). The volumes below are the envelope's own
// integral (scripts/fillet-envelope-volume.go, at its defaults), which
// ours keep to within 0.3%.
func TestFilletEndLeaningWalls(t *testing.T) {
	if testing.Short() {
		t.Skip("rolling-ball fillets of the dish pocket's y+ rim edge, slow")
	}
	t.Parallel()
	for _, c := range []struct{ r, want float64 }{
		{0.5, 0.256720},
		{1, 0.986804},
		{1.5, 2.130773},
		{2, 3.630505},
	} {
		t.Run(fmt.Sprintf("r%g", c.r), func(t *testing.T) {
			t.Parallel()
			sh := curvedPocket(true, true, fine(0.01))
			before := surfaceVolume(t, sh)
			if err := topo.FilletPicks(sh, curvedRim(true, true, "y+"), c.r, 1<<30, false); err != nil {
				t.Fatal(err)
			}
			sound(t, sh)
			took := before - surfaceVolume(t, sh)
			t.Logf("took %.6f, the envelope %.6f, %+.5f", took, c.want, took/c.want-1)
			if d := took/c.want - 1; d < -0.003 || d > 0.003 {
				t.Errorf("took %.6f, the envelope's integral %.6f", took, c.want)
			}
		})
	}
}

// curvedPocket's x- rim edge in the block's flat top, its walls
// upright, chamfered alone: it ends against the y walls, whose edges
// with the leaning x+ wall are arcs drawn on above the top and cut
// there, which were refused as part of a longer curve before
// 2026-11-19. A square edge 12 long: d²/2 a unit of length.
func TestFilletEndLeaningWallQuick(t *testing.T) {
	t.Parallel()
	for _, d := range []float64{1, 2} {
		sh := curvedPocket(false, false, fine(0.01))
		before := surfaceVolume(t, sh)
		if err := topo.FilletPicks(sh, curvedRim(false, false, "x-"), d, 1<<30, true); err != nil {
			t.Fatal(err)
		}
		sound(t, sh)
		if took, want := before-surfaceVolume(t, sh), d*d/2*12; took < want-1e-6 || took > want+1e-6 {
			t.Errorf("d %g: took %.9f, want %.9f", d, took, want)
		}
	}
}
