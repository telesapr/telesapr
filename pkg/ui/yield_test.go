package ui

import (
	"testing"

	"github.com/telesapr/telesapr/pkg/platform/headless"
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
