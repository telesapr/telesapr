package topo_test

import (
	"reflect"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/topo"
)

// TestFilletPaced checks that a fillet of a shell given a pace
// (srf.SShell.PaceBy) calls it while the rolling ball runs and its
// result is checked, and makes the solid made without one.
func TestFilletPaced(t *testing.T) {
	t.Parallel()
	picks := []topo.FilletPick{{A: v(0, 0, 30), B: v(10, 0, 30)}}

	want := box(srf.Tolerance{})
	if err := topo.FilletPicks(want, picks, 2, 1<<30, false); err != nil {
		t.Fatal(err)
	}

	got := box(srf.Tolerance{})
	paces := 0
	got.PaceBy(func() { paces++ })
	if err := topo.FilletPicks(got, picks, 2, 1<<30, false); err != nil {
		t.Fatal(err)
	}
	if paces == 0 {
		t.Fatal("the fillet never paced")
	}
	got.PaceBy(nil)
	if !reflect.DeepEqual(got, want) {
		t.Error("paced, the fillet made another solid")
	}
}

// TestLoftAndSweepPaced checks that a closed loft and a fitted sweep,
// which pkg/topo builds from nothing, call the pace they are given
// while they are made, and make the solids made without one, the pace
// not kept on them.
func TestLoftAndSweepPaced(t *testing.T) {
	t.Parallel()
	col := core.NewRGBA(100, 100, 100, 255)
	tol := srf.Tol(0.01, 30)
	loops := ringProfiles(6, 25, func(int) []srf.SBezier { return circleAt(3, 0, 0, 0) })
	want, err := topo.MakeClosedLoftHeld(loops, nil, col, 9, tol, nil)
	if err != nil {
		t.Fatal(err)
	}
	paces := 0
	got, err := topo.MakeClosedLoftHeld(loops, nil, col, 9, tol, func() { paces++ })
	if err != nil {
		t.Fatal(err)
	}
	if paces == 0 {
		t.Error("the loft never paced")
	}
	if got.Pace() != nil {
		t.Error("the loft's shell kept the pace")
	}
	if !reflect.DeepEqual(got, want) {
		t.Error("paced, the loft made another solid")
	}

	path := helixPath(20, 15, 1.5, 10)
	profile, _ := profileAcross(path, 2)
	sweep := func(pace func()) *srf.SShell {
		sp, err := topo.NewSweepPath(path, path[0].TangentAt(0).NormalDir(0), topo.PathSweepOptions{Tol: 5e-4})
		if err != nil {
			t.Fatal(err)
		}
		sp.PaceBy(pace)
		sh, err := topo.MakePathSweep(sp, profile, col, 1<<30, 1<<30+1, 1<<30+2, tol)
		if err != nil {
			t.Fatal(err)
		}
		return sh
	}
	wantSweep := sweep(nil)
	paces = 0
	gotSweep := sweep(func() { paces++ })
	if paces == 0 {
		t.Error("the sweep never paced")
	}
	if !reflect.DeepEqual(gotSweep, wantSweep) {
		t.Error("paced, the sweep made another solid")
	}
}
