package topo

import (
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// RollingBallAlong rounds (or with flat chamfers) the whole edge from a
// to b with the rolling ball, as FilletPicks does where step 10's
// construction doesn't take an edge, for tests on edges it does take,
// whose results are known.
func RollingBallAlong(sh *srf.SShell, a, b geom.Vector, r float64, flat bool) error {
	t, err := Build(sh)
	if err != nil {
		return err
	}
	e, err := t.wholeEdge(a, b)
	if err != nil {
		return err
	}
	return rollingBallFillet(sh, t, e, r, 1<<30, flat)
}

// FilletPicksSetback rounds the picks as FilletPicks does, but with a
// setback at every corner of three rounded edges, even of one radius,
// where FilletPicks puts a sphere.
func FilletPicksSetback(sh *srf.SShell, picks []FilletPick, r float64, origin uint32) error {
	return filletPicksSetback(sh, picks, r, origin, true)
}
