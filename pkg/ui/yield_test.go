package ui

import (
	"testing"

	"github.com/telesapr/telesapr/pkg/platform/headless"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// yieldingPlatform is the headless platform as a browser page's is: one
// thread, which the application gives back between groups.
type yieldingPlatform struct {
	*headless.Platform
	yields int
}

func (p *yieldingPlatform) Yield() { p.yields++ }

// TestRegenerationYields checks that a regeneration on the interface's
// thread lets a platform that has no other (platform.Yielder) have it
// back between groups, and that opening a model is such a regeneration.
func TestRegenerationYields(t *testing.T) {
	t.Parallel()
	p := &yieldingPlatform{Platform: headless.New()}
	ss := New(p)
	ss.Init()
	p.yields = 0
	if err := ss.Load("../../testdata/solid/chamfer-corner.tlspr"); err != nil {
		t.Fatal(err)
	}
	groups := len(ss.SK.GroupOrder)
	if groups < 2 {
		t.Fatalf("the model has %d groups", groups)
	}
	if p.yields < groups {
		t.Errorf("opening a model of %d groups yielded %d times; want one a group at least", groups, p.yields)
	}
}

// TestKernelPaceOnYielder checks that the sketch's Hooks give its
// Booleans the platform's Yield to call inside a group where the
// platform has one thread, and nothing where it has threads of its own,
// so that the Booleans there still run at once.
func TestKernelPaceOnYielder(t *testing.T) {
	t.Parallel()
	p := &yieldingPlatform{Platform: headless.New()}
	ss := New(p)
	ss.Init()
	pacer, ok := ss.SK.Hooks.(sketch.KernelPacer)
	if !ok {
		t.Fatal("the application's Hooks are no KernelPacer")
	}
	pace := pacer.KernelPace()
	if pace == nil {
		t.Fatal("no pace on a platform with one thread")
	}
	p.yields = 0
	pace()
	if p.yields != 1 {
		t.Errorf("the pace yielded %d times; want 1", p.yields)
	}

	other := New(headless.New())
	other.Init()
	if pace := other.SK.Hooks.(sketch.KernelPacer).KernelPace(); pace != nil {
		t.Error("a pace on a platform with threads of its own")
	}
}
