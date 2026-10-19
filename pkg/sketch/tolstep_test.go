package sketch

import "testing"

// TestTolStep puts tolerances on the scale: never above what was asked
// for, at most a quarter below, the same for sizes between two steps.
func TestTolStep(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ in, want float64 }{
		{1, 1}, {0.118, 0.109375}, {0.106, 0.09375}, {0.11, 0.109375}, {3, 3}, {0.875, 0.875}, {0.99, 0.875},
	} {
		if got := tolStep(c.in); got != c.want {
			t.Errorf("tolStep(%g) = %g, want %g", c.in, got, c.want)
		}
	}
	for x := 0.001; x < 1000; x *= 1.07 {
		if s := tolStep(x); s > x || s < 0.75*x {
			t.Fatalf("tolStep(%g) = %g", x, s)
		}
	}
	if tolStep(0) != 0 {
		t.Error("no tolerance became one")
	}
}
