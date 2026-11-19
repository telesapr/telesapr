package topo_test

import (
	"bytes"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/telesapr/telesapr/pkg/export"
	"github.com/telesapr/telesapr/pkg/persist/step"
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/topo"
)

// stepRoundTrip writes sh as STEP, one surface per face when one, and
// reads it back as a solid with the STEP reader: the file's faces and
// the solid read, nil when unread. With TELESAPR_STEP_OUT set the file
// is kept there as name.step, for scripts/fillet-pivot-seam-fixtures.py
// to read.
func stepRoundTrip(t *testing.T, sh *srf.SShell, unread, one bool, name string) (faces int, read *srf.SShell) {
	t.Helper()
	sk := sketch.NewSketch()
	sk.NewFile()
	sk.GenerateAll()
	g := &sk.Groups[len(sk.Groups)-1]
	g.RunningShell = *sh
	var buf bytes.Buffer
	if err := export.WriteStep(&buf, sk, g, export.Options{Scale: 1, StepOneSurfacePerFace: one}); err != nil {
		t.Fatal(err)
	}
	if dir := os.Getenv("TELESAPR_STEP_OUT"); dir != "" {
		if err := os.WriteFile(filepath.Join(dir, name+".step"), buf.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	f, err := step.Parse(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	m := step.ReadModel(f)
	if len(m.Shapes) != 1 {
		t.Fatalf("%d shapes read, want one", len(m.Shapes))
	}
	if unread {
		return len(m.Shapes[0].Faces), nil
	}
	read, err = m.ShellOf(0, 0, sh.Tol)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	return len(m.Shapes[0].Faces), read
}

// The pocket's corner rounded on the seam of the dish's lathed sheet,
// and the same pocket turned off the seam into one quarter, written as
// STEP and read back: one closed solid of every face, the volume ours.
// The corner's walls were left out of the file before 2026-11-19: the
// crease's line, which the fillets' feet end on, passes a few
// millionths off its trims' ends, and the walls' loops of the line's
// pieces didn't close (snapSectionEnds). Turned into a quarter, the
// pocket's outline is a hole in that quarter of the cone, whose
// outline in its parameters cut a corner off at the apex, and the
// hole was written as a face of its own (uvOffPoles): read back as a
// solid and an open shell. One surface per face or one per patch, as
// the window and a script write them. (Chamfers of 1 and 1.5 written
// one surface per face, which OpenCASCADE reads as one valid solid,
// our reader doesn't close: a piece of the chamfer's foot it lays on a
// line of the cone's grid moves off the vertex by a thousandth; their
// faces are counted only.)
func TestFilletPivotSeamStep(t *testing.T) {
	t.Parallel()
	cases := []struct {
		spin, ra, rb float64
		flat         bool
	}{
		{0, 1, 1, false},
		{45, 1, 1, false},
		{0, 1, 1.5, true},
		{45, 1, 1.5, true},
	}
	for _, c := range cases {
		for _, one := range []bool{false, true} {
			if testing.Short() && (one || c.flat) {
				continue
			}
			// The names scripts/fillet-pivot-seam-fixtures.py reads.
			kind, prefix, suffix := "fillet", "", ""
			if c.flat {
				kind = "chamfer"
			}
			if c.spin != 0 {
				prefix = "spun_"
			}
			if one {
				suffix = "_one"
			}
			name := fmt.Sprintf("%sseam_0.1_0_%g_%g_%s%s", prefix, c.ra, c.rb, kind, suffix)
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				sh, picks := seamPocket(0.1, 0, c.spin, fine(0.05))
				if err := topo.FilletPicks(sh, picks(c.ra, c.rb), 1, 1<<30, c.flat); err != nil {
					t.Fatal(err)
				}
				want := surfaceVolume(t, sh)
				faces, read := stepRoundTrip(t, sh, one && c.flat, one, name)
				tp, err := topo.Build(sh)
				if err != nil {
					t.Fatal(err)
				}
				// Each face of ours is one of the file's, but where one
				// surface is written for a sheet's patches.
				if n := len(tp.Faces); !one && faces != n {
					t.Errorf("%d faces written, ours %d", faces, n)
				}
				if one && faces >= len(tp.Faces) {
					t.Errorf("%d faces written, ours %d: no sheet written as one", faces, len(tp.Faces))
				}
				if read == nil {
					return
				}
				sound(t, read)
				if got := surfaceVolume(t, read); math.Abs(got-want) > 2e-4*want {
					t.Errorf("volume read %.4f, ours %.4f", got, want)
				}
			})
		}
	}
}
