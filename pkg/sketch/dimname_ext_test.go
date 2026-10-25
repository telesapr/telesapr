package sketch_test

import (
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/script/api"
)

// A named dimension keeps its name through undo, redo and a round trip,
// and a script reads and changes it by the name (roadmap 16.1).
func TestDimensionNameFile(t *testing.T) {
	t.Parallel()
	d := api.New()
	sides := d.Sketch.Rect(api.V(0, 0, 0), api.V(30, 20, 0))
	w := d.Constrain.Length(sides[0], 30)
	h := d.Constrain.Length(sides[1], 20)
	step(d)
	before := saved(t, d)
	w.SetName("width")
	undoRedo(t, d, before)
	if err := h.SetExpr("width/3"); err != nil {
		t.Fatal(err)
	}
	back := roundTrip(t, d)
	if c := back.Dimension("width"); c.Value() != 30 || c.Name() != "width" {
		t.Errorf("reopened: %s named %q", c, c.Name())
	}
	if !strings.Contains(string(saved(t, d)), `name "width"`) {
		t.Error("the file doesn't name the dimension")
	}
	func() {
		defer func() {
			if err, ok := recover().(error); !ok || !strings.Contains(err.Error(), "no dimension") {
				t.Errorf("an unknown name: %v", err)
			}
		}()
		d.Dimension("depth")
	}()
}
