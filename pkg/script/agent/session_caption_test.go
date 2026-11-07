package agent

import (
	"testing"

	"github.com/telesapr/telesapr/pkg/script/api"
)

// A caption's bounds read as get_drawing's do: three decimals, and a
// bound of -5e-16 is 0, not "-4.989e-16".
func TestShortPoint(t *testing.T) {
	t.Parallel()
	cases := []struct {
		p    api.Vec
		want string
	}{
		{api.Vec{X: 0, Y: 0, Z: -4.989e-16}, "(0, 0, 0)"},
		{api.Vec{X: 60, Y: 20.00049, Z: 4.0006}, "(60, 20, 4.001)"},
		{api.Vec{X: -1.2346, Y: 1e6, Z: 0.5}, "(-1.235, 1e+06, 0.5)"},
	}
	for _, c := range cases {
		if got := shortPoint(c.p); got != c.want {
			t.Errorf("shortPoint(%v) = %q, want %q", c.p, got, c.want)
		}
	}
}
