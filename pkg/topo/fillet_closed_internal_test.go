package topo

import (
	"errors"
	"testing"

	"github.com/telesapr/telesapr/pkg/geom"
)

// The refusal of a blend left not closed is told apart from the others,
// wrapped or not, and names the blend.
func TestNotClosedRefusal(t *testing.T) {
	t.Parallel()
	err := filletErr(geom.Vector{}, notClosedMsg, "chamfer")
	if !NotClosedRefusal(err) {
		t.Errorf("%q not told as not closed", err)
	}
	if !NotClosedRefusal(errors.Join(errors.New("group"), err)) {
		t.Error("a wrapped refusal not told as not closed")
	}
	if NotClosedRefusal(filletErr(geom.Vector{}, curvedEndMsg)) || NotClosedRefusal(errors.New(notClosedPrefix)) {
		t.Error("another refusal, or a plain error, told as not closed")
	}
	if got := err.Error(); got != notClosedPrefix+" (the chamfer's mesh comes out open at this size): make it a little smaller or larger" {
		t.Errorf("message %q", got)
	}
}
