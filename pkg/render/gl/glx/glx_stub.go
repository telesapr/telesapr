//go:build !linux || !cgo || nogl

package glx

// Context is a GLX context; in this build there are none.
type Context struct{}

// NewForWindow returns ErrUnavailable.
func NewForWindow(uint64) (*Context, error) { return nil, ErrUnavailable }

// NewOffscreen returns ErrUnavailable.
func NewOffscreen(int, int) (*Context, error) { return nil, ErrUnavailable }

// MakeCurrent returns ErrUnavailable.
func (c *Context) MakeCurrent() error { return ErrUnavailable }

// DoubleBuffered is false.
func (c *Context) DoubleBuffered() bool { return false }

// SwapBuffers does nothing.
func (c *Context) SwapBuffers() {}

// Size is zero.
func (c *Context) Size() (width, height int) { return 0, 0 }

// Destroy does nothing.
func (c *Context) Destroy() {}
