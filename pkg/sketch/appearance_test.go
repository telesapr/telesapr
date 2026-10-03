package sketch_test

import (
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/script/api"
)

// Appearances (roadmap 19.2): a group's and a face's own, read back by
// the face, through undo, redo and a round trip; a name there isn't is
// refused.
func TestAppearance(t *testing.T) {
	d := api.New()
	ex := block(d, 0, 0, 20, 10, 10)
	top := ex.Solid().FaceAt(api.V(10, 5, 10))
	side := ex.Solid().FaceAt(api.V(10, 0, 5))
	step(d)
	before := saved(t, d)
	ex.SetAppearance("metal")
	undoRedo(t, d, before)
	d.SetFaceAppearance(top, "matte")
	if ex.Appearance() != "metal" || d.FaceAppearance(top) != "matte" || d.FaceAppearance(side) != "metal" {
		t.Errorf("group %q, top %q, side %q", ex.Appearance(), d.FaceAppearance(top), d.FaceAppearance(side))
	}
	back := roundTrip(t, d)
	g := back.Group(ex.Name())
	if g.Appearance() != "metal" || back.FaceAppearance(g.Solid().FaceAt(api.V(10, 5, 10))) != "matte" {
		t.Errorf("reopened: %q", g.Appearance())
	}
	d.SetFaceAppearance(top, "")
	if d.FaceAppearance(top) != "metal" {
		t.Errorf("the top's own taken away: %q", d.FaceAppearance(top))
	}
	func() {
		defer func() {
			if err, ok := recover().(error); !ok || !strings.Contains(err.Error(), "no appearance") {
				t.Errorf("a velvet solid: %v", err)
			}
		}()
		ex.SetAppearance("velvet")
	}()
}
