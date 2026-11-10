package gl

import "github.com/telesapr/telesapr/pkg/render/gl/wgl"

// NewWindowContext makes a context on the window with the platform
// handle id (an HWND here). Without WGL in the build it returns
// wgl.ErrUnavailable, and callers draw in software.
func NewWindowContext(id uint64) (WindowContext, error) {
	c, err := wgl.NewForWindow(id)
	if err != nil {
		return nil, err
	}
	return c, nil
}
