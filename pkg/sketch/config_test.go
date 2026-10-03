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
	d := api.New()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(d.SetParameter("w", "20"))
	must(d.SetParameter("h", "w/2"))
	sides := d.Rect(api.V(0, 0, 0), api.V(20, 10, 0))
	must(d.Length(sides[0], 20).SetExpr("w"))
	must(d.Length(sides[1], 10).SetExpr("h"))
	plate := d.Extrude(5, "")
	volume := func() float64 { return plate.Check().Volume }

	must(d.AddConfiguration("small"))
	must(d.AddConfiguration("large"))
	must(d.SetParameter("w", "40"))
	if v := volume(); math.Abs(v-40*20*5) > 1e-6 {
		t.Fatalf("large: volume %g", v)
	}
	step(d)
	before := saved(t, d)
	must(d.ActivateConfiguration("small"))
	if v := volume(); math.Abs(v-20*10*5) > 1e-6 || d.ActiveConfiguration() != "small" {
		t.Fatalf("small: volume %g, active %q", v, d.ActiveConfiguration())
	}
	undoRedo(t, d, before)
	back := roundTrip(t, d)
	if got := strings.Join(back.Configurations(), " "); got != "small large" || back.ActiveConfiguration() != "small" {
		t.Errorf("reopened: %q, active %q", got, back.ActiveConfiguration())
	}
	must(back.ActivateConfiguration("large"))
	if p, _ := back.Parameter("w"); p.Value != 40 {
		t.Errorf("the reopened large: w = %g", p.Value)
	}

	// A rename reaches the inactive configuration; a parameter that an
	// inactive configuration uses can't be deleted.
	must(d.RenameParameter("w", "width"))
	must(d.ActivateConfiguration("large"))
	if p, _ := d.Parameter("width"); p.Value != 40 {
		t.Errorf("large after the rename: width = %g", p.Value)
	}
	must(d.SetParameter("k", "3"))
	must(d.ActivateConfiguration("small"))
	must(d.SetParameter("h", "k*3"))
	must(d.ActivateConfiguration("large"))
	if err := d.DeleteParameter("k"); err == nil || !strings.Contains(err.Error(), "configuration small") {
		t.Errorf("deleting what small uses: %v", err)
	}
	// None active: the table changes alone.
	must(d.ActivateConfiguration(""))
	must(d.SetParameter("width", "60"))
	must(d.ActivateConfiguration("large"))
	if p, _ := d.Parameter("width"); p.Value != 40 {
		t.Errorf("large changed with none active: width = %g", p.Value)
	}
	for name, err := range map[string]error{
		"a second large":  d.AddConfiguration("large"),
		"no such":         d.ActivateConfiguration("medium"),
		"a blank name":    d.AddConfiguration(" "),
		"rename to small": d.RenameConfiguration("large", "small"),
	} {
		if err == nil {
			t.Errorf("%s: no error", name)
		}
	}
	must(d.RenameConfiguration("large", "big"))
	must(d.DeleteConfiguration("big"))
	if d.ActiveConfiguration() != "" || len(d.Configurations()) != 1 {
		t.Errorf("after the deletion: active %q, %v", d.ActiveConfiguration(), d.Configurations())
	}
}
