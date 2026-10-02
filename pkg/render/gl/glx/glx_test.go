//go:build linux && cgo && !nogl

package glx

import (
	"os"
	"runtime"
	"testing"
)

func TestOffscreen(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if os.Getenv("DISPLAY") == "" {
		t.Skip("no X display")
	}
	if _, err := NewOffscreen(0, 10); err == nil {
		t.Error("a pixel buffer of no width")
	}
	c, err := NewOffscreen(64, 32)
	if err != nil {
		t.Skipf("no GLX: %v", err)
	}
	if err := c.MakeCurrent(); err != nil {
		t.Fatal(err)
	}
	// Current already: nothing to do.
	if err := c.MakeCurrent(); err != nil {
		t.Fatal(err)
	}
	if w, h := c.Size(); w != 64 || h != 32 {
		t.Errorf("size %dx%d", w, h)
	}
	if c.DoubleBuffered() {
		t.Error("a pixel buffer is double-buffered")
	}
	c.SwapBuffers()
	c.Destroy()
	c.Destroy()
	c.SwapBuffers()
	if err := c.MakeCurrent(); err == nil {
		t.Error("a destroyed context made current")
	}
}

// A window that doesn't exist gives an error, not Xlib's exit.
func TestNoWindow(t *testing.T) {
	if os.Getenv("DISPLAY") == "" {
		t.Skip("no X display")
	}
	if c, err := NewForWindow(0x7ffffff0); err == nil {
		c.Destroy()
		t.Error("a context on a window that doesn't exist")
	}
}

// No X server: an error.
func TestNoDisplay(t *testing.T) {
	t.Setenv("DISPLAY", ":9999")
	if _, err := NewOffscreen(8, 8); err == nil {
		t.Error("a pixel buffer with no X server")
	}
	if _, err := NewForWindow(1); err == nil {
		t.Error("a window context with no X server")
	}
}

// A pixel buffer larger than the server allows is refused.
func TestHugeOffscreen(t *testing.T) {
	if os.Getenv("DISPLAY") == "" {
		t.Skip("no X display")
	}
	if c, err := NewOffscreen(1<<20, 1<<20); err == nil {
		c.Destroy()
		t.Skip("the server made a pixel buffer of 2^20 by 2^20")
	}
}
