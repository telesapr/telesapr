//go:build !cgo || nogl

package gl

import "github.com/spacesolve/spacesolve/pkg/render"

// New returns ErrUnavailable: this build has no OpenGL.
func New(Context) (render.ViewportCanvas, error) { return nil, ErrUnavailable }
