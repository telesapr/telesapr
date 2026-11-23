//go:build linux && cgo && !nogl

package gl

import (
	"os"
	"testing"

	"github.com/telesapr/telesapr/pkg/render/gl/glx"
)

// offscreen makes a GLX pixel-buffer context of the given size, or
// skips the test where there is no X display with GLX.
func offscreen(t *testing.T, width, height int) offscreenContext {
	t.Helper()
	// Read here so that go test's cache knows the result depends on
	// it: Xlib reads it in C, which the cache doesn't see.
	if os.Getenv("DISPLAY") == "" {
		t.Skip("no X display")
	}
	ctx, err := glx.NewOffscreen(width, height)
	if err != nil {
		t.Skipf("no GL: %v", err)
	}
	return ctx
}
