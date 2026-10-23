package check_test

import (
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/check"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/script/api"
)

// sharedByOracles is the volume the oracles of two solids share, the
// second moved by dz along z, sampled at n points of the box lo-hi.
func sharedByOracles(a, b *check.Oracle, dz float64, lo, hi geom.Vector, n int) float64 {
	seed := uint64(1)
	rnd := func() float64 {
		seed = seed*6364136223846793005 + 1442695040888963407
		return float64(seed>>11) / (1 << 53)
	}
	in := 0
	for range n {
		p := api.V(lo.X+rnd()*(hi.X-lo.X), lo.Y+rnd()*(hi.Y-lo.Y), lo.Z+rnd()*(hi.Z-lo.Z))
		if a.Inside(p) && b.Inside(p.Minus(api.V(0, 0, dz))) {
			in++
		}
	}
	return float64(in) / float64(n) * (hi.X - lo.X) * (hi.Y - lo.Y) * (hi.Z - lo.Z)
}

// An M8 screw and its nut, of clearance 0, linked side by side, as
// Assembly.Interferences sees them, against the oracles of the two:
// in phase their ridges collide, by the volume the oracles share, and
// half a pitch along the flanks only touch. The Boolean of their
// shells fails, their helical flanks lying on each other, and the
// volume is their meshes'; a whole part's area once set the skin the
// meshes may share where parts only touch, and the collision was lost
// in it.
func TestInterferencesOfAThreadByOracle(t *testing.T) {
	if testing.Short() {
		t.Skip("samples the oracles of a screw and a nut at 60000 points twice")
	}
	t.Parallel()
	const screwFile, nutFile = "../../testdata/thread/screw_m8.tlspr", "../../testdata/thread/nut_m8.tlspr"
	screw, err := api.Open(screwFile)
	if err != nil {
		t.Fatal(err)
	}
	nut, err := api.Open(nutFile)
	if err != nil {
		t.Fatal(err)
	}
	so, no := oracleOf(t, screw), oracleOf(t, nut)
	// Where the nut is: its height of the screw's width.
	lo, hi := screw.Active().Analyze().Bounds()
	_, nhi := nut.Active().Analyze().Bounds()
	hi.Z = nhi.Z

	d := api.New()
	if _, err := d.Assembly.Link(screwFile, "screw"); err != nil {
		t.Fatal(err)
	}
	link, err := d.Assembly.Link(nutFile, "nut")
	if err != nil {
		t.Fatal(err)
	}
	const pitch = 1.25
	inPhase := sharedByOracles(so, no, 0, lo, hi, 60000)
	found, _ := d.Assembly.Interferences()
	if len(found) != 1 || found[0].Failed || math.Abs(found[0].Volume-inPhase) > 0.15*inPhase {
		t.Errorf("in phase: %+v, the oracles share %g mm³", found, inPhase)
	}
	link.AsLink().Move(api.V(0, 0, pitch/2))
	apart := sharedByOracles(so, no, pitch/2, lo, hi, 60000)
	if found, _ := d.Assembly.Interferences(); len(found) != 0 || apart > 0.2*inPhase {
		t.Errorf("half a pitch along: %+v, the oracles share %g mm³", found, apart)
	}
}
