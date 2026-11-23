package topo_test

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/internal/sweep"
	"github.com/telesapr/telesapr/pkg/topo"
)

// deepFloor is the floor of curvedPocketFloor's pocket deep enough that
// no chamfer of 3.5 of its rim reaches it: the y+ wall leans in to
// meet the y- wall at about 1.44.
const deepFloor = 2.5

// pastFloor is what the chamfer of curvedPocket's rim edges picks (in
// the dish, both walls leaning) at d takes, where its foot on a wall
// runs below the wall's foot on the floor, z = 4: measured apart from
// the fillet made, as the same chamfer of the pocket whose floor is at
// deepFloor, which its foot doesn't reach, less what that takes below
// z = 4. The floor's plane cuts the chamfer as it cuts that one: a
// solid whose floor is the plane and whose pocket above it is the
// deeper one's.
func pastFloor(t *testing.T, picks []string, d float64) float64 {
	t.Helper()
	deep := curvedPocketFloor(true, true, deepFloor, fine(0.01))
	before := surfaceVolumeAbove(t, deep, 4)
	if err := topo.FilletPicks(deep, curvedRim(true, true, picks...), d, 1<<30, true); err != nil {
		t.Fatalf("on the deeper pocket: %v", err)
	}
	sound(t, deep)
	return before - surfaceVolumeAbove(t, deep, 4)
}

// The dish's y+ rim edge chamfered, alone and in the chain y+/x-/y-,
// both walls leaning, from 2.25 on: where the rim dips lowest, near
// x = 0, the chamfer's foot on the y+ wall runs below the wall's foot
// on the floor, and comes back (rollingrunoff_dip.go); from 3.5 the
// upright y- wall's too. The floor's plane cuts the chamfer there, the
// floor grows to the cut and the wall is pinched to nothing between.
// Refused as not closed before 2026-11-19, the wall's trims crossing
// its edge with the floor. Each volume within 0.15% of the same chamfer
// of the deeper pocket less what lies below the floor (pastFloor; 0.02%
// to 0.11% under it, the meshes' triangles cut by the floor's plane
// taken as chords), and the edge alone within 0.3% of the chamfer's
// integral too (scripts/fillet-envelope-volume.go -chamfer -n 120
// -dx 0.04 -floor 4; ours within 0.03% of it at 2.5, 3 and 3.5, where without
// the floor's cut it is 0.11% and 2.3% more).
// The chain of 3.25, whose foot on the y- wall, 3.25 tall at x = 0,
// just touches the floor there, is TestFilletEndLeaningWallsChamferTouchesFloor's.
func TestFilletEndLeaningWallsChamferPastFloor(t *testing.T) {
	if testing.Short() {
		t.Skip("chamfers of the dish pocket's rim past the walls' feet, slow")
	}
	t.Parallel()
	for _, c := range sweep.Pick([]struct {
		picks    string
		d        float64
		integral float64
	}{
		// In this order the ordinary run's four (sweep.Pick) take both
		// integrals' ends and the chain past both walls' feet.
		{"y+", 2.5, 54.181806},
		{"y+", 2.25, 0},
		{"y+,x-,y-", 3.5, 0},
		{"y+", 2.75, 0},
		{"y+", 3.5, 94.331227},
		{"y+", 3, 73.621150},
		{"y+,x-,y-", 2.75, 0},
		{"y+,x-,y-", 2.25, 0},
		{"y+,x-,y-", 2.5, 0},
		{"y+,x-,y-", 3, 0},
		{"y+", 3.25, 0},
	}, 4) {
		t.Run(fmt.Sprintf("%s d%g", c.picks, c.d), func(t *testing.T) {
			t.Parallel()
			picks := strings.Split(c.picks, ",")
			sh := curvedPocket(true, true, fine(0.01))
			before := surfaceVolume(t, sh)
			if err := topo.FilletPicks(sh, curvedRim(true, true, picks...), c.d, 1<<30, true); err != nil {
				if fe, ok := errors.AsType[*topo.FilletError](err); ok {
					t.Fatalf("%v at %v", err, fe.At)
				}
				t.Fatal(err)
			}
			sound(t, sh)
			took := before - surfaceVolume(t, sh)
			want := pastFloor(t, picks, c.d)
			t.Logf("took %.6f, the deeper pocket's above the floor %.6f, %+.5f", took, want, took/want-1)
			if d := took/want - 1; d < -0.0015 || d > 0.0015 {
				t.Errorf("took %.6f, the deeper pocket's chamfer above the floor %.6f", took, want)
			}
			if c.integral > 0 {
				t.Logf("the integral %.6f, %+.5f", c.integral, took/c.integral-1)
				if d := took/c.integral - 1; d < -0.003 || d > 0.003 {
					t.Errorf("took %.6f, the chamfer's integral %.6f", took, c.integral)
				}
			}
		})
	}
}

// The chain y+/x-/y- chamfered at 3.25: its foot on the upright y- wall,
// 3.25 tall where the rim dips lowest, touches the floor there, the
// wall pinched to a point, which neither cut nor left whole comes out
// closed. Refused by name before 2026-11-23; now made at the nearest
// size within the chord tolerance at which the foot crosses the floor's
// edge or stays clear of it (footAtEdgeSize), sound and within 0.15% of
// the deeper pocket's chamfer at that size less what lies below the
// floor, as TestFilletEndLeaningWallsChamferPastFloor's are.
func TestFilletEndLeaningWallsChamferTouchesFloor(t *testing.T) {
	if testing.Short() {
		t.Skip("the dish pocket's rim chamfered at 3.25, slow; TestFilletTroughTouchesFloor is the quick one")
	}
	t.Parallel()
	picks := []string{"y+", "x-", "y-"}
	sh := curvedPocket(true, true, fine(0.01))
	before := surfaceVolume(t, sh)
	made, err := topo.FilletPicksMade(sh, curvedRim(true, true, picks...), 3.25, 1<<30, true)
	if err != nil {
		if fe, ok := errors.AsType[*topo.FilletError](err); ok {
			t.Fatalf("%v at %v", err, fe.At)
		}
		t.Fatal(err)
	}
	if made == 3.25 || math.Abs(made-3.25) > sh.Tol.ChordTolMm()*math.Sqrt2 {
		t.Errorf("made at %g, want a size near 3.25 but not it", made)
	}
	sound(t, sh)
	took := before - surfaceVolume(t, sh)
	want := pastFloor(t, picks, made)
	t.Logf("made at %.6f, took %.6f, the deeper pocket's above the floor %.6f, %+.5f", made, took, want, took/want-1)
	if d := took/want - 1; d < -0.0015 || d > 0.0015 {
		t.Errorf("took %.6f, the deeper pocket's chamfer above the floor %.6f", took, want)
	}
}
