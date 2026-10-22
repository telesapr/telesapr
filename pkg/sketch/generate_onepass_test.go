package sketch_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/script/api"
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/srf"
)

// plate is a plate depth deep, regenerated.
func plate(depth float64) (*api.Doc, api.Group) {
	d := api.New()
	d.Sketch.Rect(api.V(0, 0, 0), api.V(10, 20, 0))
	ex := d.Features.Extrude(depth, "plate")
	step(d)
	return d, ex
}

// A regeneration in the window, after the first, solves each group and
// makes its solid once where the model's size keeps the tolerance; where
// an edit moves the size across a step, the solids are made again at the
// new tolerance, as a regeneration from nothing makes them.
func TestRegenerationOnePass(t *testing.T) {
	t.Parallel()
	d, ex := plate(5)
	d.With(func(sk *sketch.Sketch) {
		sk.Timings = &sketch.Timings{}
		sk.GenerateAll()
		for _, g := range sk.Timings.Groups() {
			if g.Solves > 1 || g.Solids > 1 {
				t.Errorf("group %v: %d solves, %d solids in one regeneration", g.Group, g.Solves, g.Solids)
			}
		}
		sk.Timings = nil
	})

	ex.AsExtrude().SetDepth(500)
	step(d)
	fresh, fex := plate(500)
	var got, want string
	var gotTol, wantTol srf.Tolerance
	d.With(func(sk *sketch.Sketch) {
		sk.GenerateAllTyped(sketch.GenerateDirty)
		got, gotTol = shellText(&sk.Group(ex.H).RunningShell), sk.Tol()
	})
	fresh.With(func(sk *sketch.Sketch) {
		sk.GenerateAllTyped(sketch.GenerateDirty)
		want, wantTol = shellText(&sk.Group(fex.H).RunningShell), sk.Tol()
	})
	if gotTol != wantTol || got != want {
		t.Errorf("deepened to 500: tolerance %v, from nothing %v; the same solid: %v", gotTol, wantTol, got == want)
	}
}

// shellText is what a shell's surfaces and curves are, as text: two
// shells made alike give the same.
func shellText(sh *srf.SShell) string {
	var b strings.Builder
	for _, s := range sh.Surface {
		fmt.Fprintf(&b, "s %d %d %v %v %d %v\n", s.DegM, s.DegN, s.Ctrl, s.Weight, len(s.Trim), s.Tol)
	}
	for _, c := range sh.Curve {
		fmt.Fprintf(&b, "c %v %d %v\n", c.Exact, len(c.Points), c.Points)
	}
	return b.String()
}
