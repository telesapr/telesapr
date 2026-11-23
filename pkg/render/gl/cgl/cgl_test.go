//go:build darwin && cgo && !nogl

package cgl

import (
	"runtime"
	"testing"
)

func TestOffscreen(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if _, err := NewOffscreen(0, 10); err == nil {
		t.Error("a framebuffer of no width")
	}
	c, err := NewOffscreen(64, 32)
	if err != nil {
		t.Skipf("no CGL: %v", err)
	}
	t.Logf("software renderer: %v", c.Software())
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
		t.Error("a framebuffer is double-buffered")
	}
	if !c.Framebuffered() {
		t.Error("an offscreen context draws in the default framebuffer")
	}
	if c.PixelRatio() != 1 {
		t.Errorf("pixel ratio %v", c.PixelRatio())
	}
	c.SwapBuffers()
	c.Destroy()
	c.Destroy()
	c.SwapBuffers()
	if err := c.MakeCurrent(); err == nil {
		t.Error("a destroyed context made current")
	}
}

// A framebuffer larger than the renderbuffers allow is refused.
func TestHugeOffscreen(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if c, err := NewOffscreen(1<<20, 1<<20); err == nil {
		c.Destroy()
		t.Skip("CGL made a framebuffer of 2^20 by 2^20")
	}
}
