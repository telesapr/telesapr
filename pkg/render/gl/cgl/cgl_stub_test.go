//go:build !darwin || !cgo || nogl

package cgl

import (
	"errors"
	"testing"
)

// Without macOS, cgo or with nogl no context can be made, and the
// callers draw in software.
func TestUnavailable(t *testing.T) {
	t.Parallel()
	if _, err := NewForWindow(1); !errors.Is(err, ErrUnavailable) {
		t.Errorf("NewForWindow: %v", err)
	}
	c, err := NewOffscreen(8, 8)
	if !errors.Is(err, ErrUnavailable) {
		t.Errorf("NewOffscreen: %v", err)
	}
	if c != nil {
		t.Error("a context in a build without CGL")
	}
	c = &Context{}
	if err := c.MakeCurrent(); !errors.Is(err, ErrUnavailable) {
		t.Errorf("MakeCurrent: %v", err)
	}
	c.SwapBuffers()
	c.Destroy()
	if c.DoubleBuffered() || c.Software() || c.Framebuffered() || c.PixelRatio() != 1 {
		t.Error("a stub context claims to draw")
	}
	if w, h := c.Size(); w != 0 || h != 0 {
		t.Errorf("size %dx%d", w, h)
	}
}
