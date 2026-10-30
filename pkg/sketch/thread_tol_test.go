package sketch_test

import (
	"fmt"
	"testing"

	"github.com/telesapr/telesapr/pkg/sketch"
)

// TestThreadNutFootChords makes nuts whose ridges' foot lies past the
// bore by less than the chord tolerance, and requires them sound, their
// helices swept again finer only where a chord of the foot dips into
// the bore: a 2-8 BUTT nut at 0.2% and 0.5% (774 and 758 naked edges
// when it was swept at the sketch's tolerance), and at 0.05%, where
// none dips.
func TestThreadNutFootChords(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("thread solids")
	}
	nut := sketch.ThreadOptions{Length: 40.64, Through: true, Internal: true}
	for _, c := range []struct {
		tol   float64
		finer bool
	}{{0.2, true}, {0.5, true}, {0.05, false}} {
		tc := threadCase{"2-8 BUTT nut", 50.8, 40.64, "2-8 BUTT", nut}
		t.Run(fmt.Sprint(c.tol), func(t *testing.T) {
			t.Parallel()
			sk, _, res, err := tc.tryBuild(t, c.tol)
			if err != nil {
				t.Fatal(err)
			}
			if !res.Sound {
				t.Errorf("at %g%%: not sound", c.tol)
			}
			n := 0
			for i := range sk.Groups {
				g := &sk.Groups[i]
				if g.Type != sketch.GroupHelix {
					continue
				}
				n++
				if finer := g.ThisShell.Tol.ChordTolMm() < sk.Tol().ChordTolMm(); finer != c.finer {
					t.Errorf("at %g%%: helix %s swept at %g mm, the sketch's %g", c.tol, g.Name,
						g.ThisShell.Tol.ChordTolMm(), sk.Tol().ChordTolMm())
				}
			}
			if n != 1 {
				t.Errorf("%d helices, want 1", n)
			}
		})
	}
}
