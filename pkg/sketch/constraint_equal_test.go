package sketch

import (
	"errors"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
)

// EqualConstraints refuses what no equal constraint takes, with an
// error callers can branch on.
func TestEqualConstraintsErrors(t *testing.T) {
	t.Parallel()
	sk := NewSketch()
	for name, es := range map[string][]core.HEntity{
		"nothing":        nil,
		"missing entity": {{V: 12345}},
	} {
		if _, err := sk.EqualConstraints(core.Constraint{}, es); !errors.Is(err, ErrEqualSelection) {
			t.Errorf("%s gave %v, want ErrEqualSelection", name, err)
		}
	}
}
