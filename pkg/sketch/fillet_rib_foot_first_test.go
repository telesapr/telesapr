package sketch_test

import (
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/internal/fixtures"
	"github.com/telesapr/telesapr/pkg/script/api"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// A rib on a plate, its foot filled in one fillet group and its top's
// five edges rounded in the next: the uprights and the slope's edges end
// on the foot's fillets, and each corner there is set back and filled by
// patches (pkg/topo's TestFilletRibFootFirstOracle, whose rib this is,
// raised onto a plate from z 0 to 3: 3651.930 mm³ at its fine tolerance).
func TestFilletRibTopAfterFoot(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("fills a rib's foot and rounds its top at a fine tolerance, about 30 s")
	}
	if fixtures.RaceDetector {
		// The race stage is near its limit; pkg/topo's
		// TestFilletRibFootFirstEach rounds the same corners there.
		t.Skip("a rib rounded at a fine tolerance, too slow under the race detector")
	}
	d := api.New()
	d.Edit.SetChordTolerance(0.001)
	d.Features.SketchIn(d.Workplane("XY"), "")
	d.Sketch.Rect(api.V(-10, -8, 0), api.V(28, 18, 0))
	d.Features.Extrude(3, "")
	d.Features.SketchIn(d.Workplane("ZX"), "")
	d.Sketch.Polyline(true, api.V(0, 0, 2), api.V(14, 0, 2), api.V(14, 0, 3), api.V(0, 0, 13))
	rib := d.Features.Extrude(10, "")
	d.Edit.Regen()
	s := rib.Solid()
	foot := d.Features.FilletSolid([]api.SolidEdge{
		s.EdgeAt(api.V(7, 0, 3)), s.EdgeAt(api.V(0, 5, 3)), s.EdgeAt(api.V(7, 10, 3)), s.EdgeAt(api.V(14, 5, 3)),
	}, 1, "")
	if p := foot.Problem(); p != "" {
		t.Fatal(p)
	}
	step(d)
	saveBefore := saved(t, d)
	s = foot.Solid()
	top := d.Features.FilletSolid([]api.SolidEdge{
		s.EdgeAt(api.V(0, 0, 8)), s.EdgeAt(api.V(0, 10, 8)), s.EdgeAt(api.V(7, 0, 8)), s.EdgeAt(api.V(7, 10, 8)),
		s.EdgeAt(api.V(0, 5, 13)),
	}, 1, "")
	if p := top.Problem(); p != "" {
		t.Fatal(p)
	}
	r := top.Analyze().Check()
	if want := 3651.930; !r.Sound() || math.Abs(r.Volume-want) > 0.1 {
		t.Fatalf("%s, want volume %g", r.String(), want)
	}
	undoRedo(t, d, saveBefore)
	// A file keeps the chord tolerance it was regenerated at only as
	// the default; at it, what is saved comes back the same.
	d.With(func(sk *sketch.Sketch) { sk.ChordTolPct = 0; sk.ScheduleGenerateAll() })
	if br := roundTrip(t, d).Group(top.Name()).Analyze().Check(); !br.Sound() {
		t.Fatalf("reopened: %s", br.String())
	}
}
