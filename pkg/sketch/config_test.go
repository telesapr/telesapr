package sketch_test

import (
	"math"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/script/api"
)

// Configurations (roadmap 16.2): a plate small and large from one
// table. Adding one takes the table; while one is active, the table's
// changes are its; activating the other writes its values back and the
// model follows. Renames and deletions of parameters reach every
// configuration; undo, redo and a round trip keep them.
func TestConfigurations(t *testing.T) {
	t.Parallel()
	d := api.New()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(d.Params.Set("w", "20"))
	must(d.Params.Set("h", "w/2"))
	sides := d.Sketch.Rect(api.V(0, 0, 0), api.V(20, 10, 0))
	must(d.Constrain.Length(sides[0], 20).SetExpr("w"))
	must(d.Constrain.Length(sides[1], 10).SetExpr("h"))
	plate := d.Features.Extrude(5, "")
	volume := func() float64 { return plate.Analyze().Check().Volume }

	must(d.Params.AddConfiguration("small"))
	must(d.Params.AddConfiguration("large"))
	must(d.Params.Set("w", "40"))
	if v := volume(); math.Abs(v-40*20*5) > 1e-6 {
		t.Fatalf("large: volume %g", v)
	}
	step(d)
	before := saved(t, d)
	must(d.Params.ActivateConfiguration("small"))
	if v := volume(); math.Abs(v-20*10*5) > 1e-6 || d.Params.ActiveConfiguration() != "small" {
		t.Fatalf("small: volume %g, active %q", v, d.Params.ActiveConfiguration())
	}
	undoRedo(t, d, before)
	back := roundTrip(t, d)
	if got := strings.Join(back.Params.Configurations(), " "); got != "small large" || back.Params.ActiveConfiguration() != "small" {
		t.Errorf("reopened: %q, active %q", got, back.Params.ActiveConfiguration())
	}
	must(back.Params.ActivateConfiguration("large"))
	if p, _ := back.Params.Get("w"); p.Value != 40 {
		t.Errorf("the reopened large: w = %g", p.Value)
	}

	// A rename reaches the inactive configuration; a parameter that an
	// inactive configuration uses can't be deleted.
	must(d.Params.Rename("w", "width"))
	must(d.Params.ActivateConfiguration("large"))
	if p, _ := d.Params.Get("width"); p.Value != 40 {
		t.Errorf("large after the rename: width = %g", p.Value)
	}
	must(d.Params.Set("k", "3"))
	must(d.Params.ActivateConfiguration("small"))
	must(d.Params.Set("h", "k*3"))
	must(d.Params.ActivateConfiguration("large"))
	if err := d.Params.Delete("k"); err == nil || !strings.Contains(err.Error(), "configuration small") {
		t.Errorf("deleting what small uses: %v", err)
	}
	// None active: the table changes alone.
	must(d.Params.ActivateConfiguration(""))
	must(d.Params.Set("width", "60"))
	must(d.Params.ActivateConfiguration("large"))
	if p, _ := d.Params.Get("width"); p.Value != 40 {
		t.Errorf("large changed with none active: width = %g", p.Value)
	}
	for name, err := range map[string]error{
		"a second large":  d.Params.AddConfiguration("large"),
		"no such":         d.Params.ActivateConfiguration("medium"),
		"a blank name":    d.Params.AddConfiguration(" "),
		"rename to small": d.Params.RenameConfiguration("large", "small"),
	} {
		if err == nil {
			t.Errorf("%s: no error", name)
		}
	}
	must(d.Params.RenameConfiguration("large", "big"))
	must(d.Params.DeleteConfiguration("big"))
	if d.Params.ActiveConfiguration() != "" || len(d.Params.Configurations()) != 1 {
		t.Errorf("after the deletion: active %q, %v", d.Params.ActiveConfiguration(), d.Params.Configurations())
	}
}
