//go:build !linux

package gl

// NewWindowContext returns ErrUnavailable: this system has no context
// backend yet, and callers draw in software.
func NewWindowContext(uint64) (WindowContext, error) { return nil, ErrUnavailable }
