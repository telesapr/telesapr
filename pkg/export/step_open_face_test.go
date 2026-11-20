package export

import (
	"bytes"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/srf"
)

// A face whose trims don't close is left out of a STEP file, and the
// user is told where, not handed an open shell in silence; a sound
// solid is written with no warning.
func TestStepWarnsOfFaceLeftOut(t *testing.T) {
	t.Parallel()
	for _, broken := range []bool{false, true} {
		sk := sketch.NewSketch()
		sk.NewFile()
		sk.GenerateAll()
		g := &sk.Groups[len(sk.Groups)-1]
		sh := srf.SShell{Tol: srf.Tol(0.1, 64)}
		sh.MakePrism([]geom.Vector{{}, {X: 10}, {X: 10, Y: 10}, {Y: 10}}, geom.Vector{}, geom.Vector{Z: 5},
			core.NewRGBA(100, 100, 100, 255))
		if broken {
			// One side of a face's outline gone: the loop is open.
			s := &sh.Surface[0]
			s.Trim = s.Trim[1:]
		}
		g.RunningShell = sh
		var warnings []string
		var buf bytes.Buffer
		if err := WriteStep(&buf, sk, g, options(&warnings)); err != nil {
			t.Fatal(err)
		}
		switch {
		case !broken && len(warnings) > 0:
			t.Errorf("sound solid: warned %q", warnings)
		case broken && (len(warnings) != 1 || !strings.Contains(warnings[0], "left out of the STEP file")):
			t.Errorf("open outline: warned %q, want one warning of a face left out", warnings)
		}
	}
}
