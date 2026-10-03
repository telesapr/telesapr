package sketch_test

import (
	"math"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/script/api"
)

// A group made with an earlier group active goes into the history after
// that group (roadmap 16.4): a chamfer of a block made after a hole was
// cut through it comes before the cut, the cut still cuts, every group
// refers only to groups before it, and the inserted group can't move
// before what it uses, nor what it uses after it. Undo, redo and a
// round trip keep the history, and the file the active group. Nothing earlier may refer to the
// inserted group, or to anything after the active group.
func TestInsertGroup(t *testing.T) {
	d := api.New()
	rect := d.Active()
	d.Rect(api.V(0, 0, 0), api.V(20, 10, 0))
	block := d.Extrude(10, "block")
	d.SketchIn(d.Workplane("XY"), "hole-sketch")
	centre := d.Circle(api.V(5, 5, 0), 2)
	cut := d.Extrude(10, "cut")
	cut.SetCombine(api.Difference)
	hole := math.Pi * 4 * 10

	// Making a group active is not a step to undo, as in the window.
	d.Activate(block)
	step(d)
	before := saved(t, d)
	ch := d.Chamfer([]api.Entity{lineAlong(t, block, api.V(20, 0, 10), api.V(20, 10, 10))}, 2, "")
	var names []string
	sk := d.Host().Sketch()
	for _, hg := range sk.GroupOrder {
		names = append(names, sk.GetGroup(hg).Name)
	}
	if got := strings.Join(names, " "); got != "#references sketch-in-plane block chamfer hole-sketch cut" {
		t.Fatalf("the history: %s", got)
	}
	if v := cut.Volume(); math.Abs(v-(2000-20-hole)) > 0.01*hole {
		t.Errorf("the cut after the chamfer leaves %g", v)
	}
	for _, hg := range sk.GroupOrder {
		if dep, later := sk.LaterReference(hg); later {
			t.Errorf("%s refers to %s, after it", sk.GroupName(hg), sk.GroupName(dep.Group))
		}
	}
	if err := ch.MoveBefore(block); err == nil || !strings.Contains(err.Error(), "which it uses") {
		t.Errorf("the chamfer before its block: %v", err)
	}
	if err := block.MoveBefore(cut); err == nil || !strings.Contains(err.Error(), "which uses it") {
		t.Errorf("the block after the chamfer: %v", err)
	}
	undoRedo(t, d, before)
	// The file keeps the active group, which what is shown follows.
	if back := roundTrip(t, d); back.Active().Name() != "chamfer" {
		t.Errorf("reopened with %s active", back.Active().Name())
	}
	if !strings.Contains(string(saved(t, d)), "\nactive 0x6\n") {
		t.Error("the file doesn't name the active group")
	}

	// The rectangle's sketch, active, can't be tied to the hole's
	// circle, which comes after it.
	d.Activate(rect)
	var corner api.Entity
	for _, e := range rect.Entities() {
		if e.Kind() == "point" {
			corner = e
			break
		}
	}
	for name, c := range map[string]struct {
		fn   func()
		want string
	}{
		"a constraint on a later entity": {func() { d.Coincident(corner, centre.Center()) }, "comes after the active group"},
		"a group on a later face": {func() {
			d.Activate(block)
			var face api.Entity
			for _, e := range cut.Entities() {
				if e.Kind() == "face" {
					face = e
				}
			}
			d.Mirror(face, "")
		}, "would refer to"},
	} {
		func() {
			defer func() {
				err, ok := recover().(error)
				if !ok || !strings.Contains(err.Error(), c.want) {
					t.Errorf("%s: %v, want an error saying %q", name, err, c.want)
				}
			}()
			c.fn()
		}()
	}
	// The refused group left nothing behind.
	if got := len(d.Host().Sketch().GroupOrder); got != 6 {
		t.Errorf("%d groups after the refusals", got)
	}
}
