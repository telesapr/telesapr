//go:build cgo && !nogl && !linux && !darwin

package gl

import "testing"

// offscreen skips the test: wgl has an offscreen context, but no test
// of the renderer uses it yet (ROADMAP "OpenGL leftovers").
func offscreen(t *testing.T, _, _ int) offscreenContext {
	t.Helper()
	t.Skip("no offscreen context wired for this system")
	return nil
}
