package sketch_test

import (
	"errors"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/script/api"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// OrientNormal turns the free normal of a workplane given by the
// workplane or by the normal, and refuses a normal that lies in a
// workplane, an entity with no normal and a quaternion of no size.
func TestOrientNormal(t *testing.T) {
	t.Parallel()
	d := api.New()
	d.Features.Sketch3D("free")
	wp := d.Sketch.NewWorkplane(api.V(0, 0, 0), api.V(1, 0, 0), api.V(0, 1, 0))
	pt := d.Sketch.Point(api.V(1, 2, 3))
	sk := d.Host().Sketch()

	q := geom.QuaternionFromBasis(api.V(0, 1, 0), api.V(0, 0, 1))
	if err := sk.OrientNormal(wp.H, q); err != nil {
		t.Fatal(err)
	}
	n := sk.Entity(sk.NormalOf(wp.H))
	if u := n.NormalU(sk); !nearPoint(u, api.V(0, 1, 0)) {
		t.Errorf("u %v after turning the workplane", u)
	}
	// The normal itself, a quaternion that isn't of size one.
	if err := sk.OrientNormal(sk.NormalOf(wp.H), geom.NewQuaternion(2, 0, 0, 0)); err != nil {
		t.Fatal(err)
	}
	if u := n.NormalU(sk); !nearPoint(u, api.V(1, 0, 0)) {
		t.Errorf("u %v after turning the normal", u)
	}

	before := n.NormalGetNum(sk)
	for name, fn := range map[string]func() error{
		"point": func() error { return sk.OrientNormal(pt.H, q) },
		"none":  func() error { return sk.OrientNormal(core.HEntity{}, q) },
		"zero":  func() error { return sk.OrientNormal(wp.H, geom.Quaternion{}) },
	} {
		if err := fn(); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
	if after := n.NormalGetNum(sk); after != before {
		t.Errorf("a refusal moved the normal: %v then %v", before, after)
	}

	// A circle drawn in a workplane has a normal that lies in it.
	d.Features.SketchIn(wp, "")
	c := d.Sketch.Circle(api.V(1, 1, 0), 2)
	sk = d.Host().Sketch()
	if err := sk.OrientNormal(c.H, q); !errors.Is(err, sketch.ErrNormalLocked) {
		t.Errorf("a circle in a workplane: %v", err)
	}
}
