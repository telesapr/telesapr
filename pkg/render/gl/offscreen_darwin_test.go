//go:build darwin && cgo && !nogl

package gl

import (
	"testing"

	"github.com/telesapr/telesapr/pkg/render/gl/cgl"
)

// offscreen makes a CGL context on a framebuffer of the given size,
// on the GPU or, where the machine has none, the software renderer;
// it skips the test where it can make neither. Nothing here needs a
// display or a window server session beyond what CGL itself asks.
func offscreen(t *testing.T, width, height int) offscreenContext {
	t.Helper()
	ctx, err := cgl.NewOffscreen(width, height)
	if err != nil {
		t.Skipf("no GL: %v", err)
	}
	if ctx.Software() {
		t.Log("offscreen context on the software renderer")
	}
	return ctx
}
