//go:build !cgo || nogl

package gl

import "github.com/telesapr/telesapr/pkg/render"

// New returns ErrUnavailable: this build has no OpenGL.
func New(Context) (render.ViewportCanvas, error) { return nil, ErrUnavailable }
