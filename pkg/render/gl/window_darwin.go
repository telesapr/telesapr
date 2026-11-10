package gl

import "github.com/telesapr/telesapr/pkg/render/gl/cgl"

// NewWindowContext makes a context on the window with the platform
// handle id (an NSView here). Without cgo in the build it returns
// cgl.ErrUnavailable, and callers draw in software.
func NewWindowContext(id uint64) (WindowContext, error) {
	c, err := cgl.NewForWindow(id)
	if err != nil {
		return nil, err
	}
	return c, nil
}
