package libm

import "math"

// glibc's hypot (sysdeps/ieee754/dbl-64/e_hypot.c), the kernel built
// without FMA, as x86-64 builds it.

const (
	hypotScale    = 0x1p-600
	hypotLargeVal = 0x1p+511
	hypotTinyVal  = 0x1p-459
	hypotEps      = 0x1p-54
)

// hypotKernel needs ax >= ay >= 0, and ax, ay and ax - ay that square
// without overflow or underflow.
func hypotKernel(ax, ay float64) float64 {
	var t1, t2 float64
	h := math.Sqrt(ax*ax + ay*ay)
	if h <= 2.0*ay {
		delta := h - ay
		t1 = ax * (2.0*delta - ax)
		t2 = (delta - 2.0*(ax-ay)) * delta
	} else {
		delta := h - ax
		t1 = 2.0 * delta * (ax - 2.0*ay)
		t2 = (4.0*delta-ay)*ay + delta*delta
	}
	h -= (t1 + t2) / (2.0 * h)
	return h
}

// Hypot is glibc's hypot: sqrt(x*x + y*y), without spurious overflow or
// underflow, with an error below 1 ulp.
func Hypot(x, y float64) float64 {
	if math.IsInf(x, 0) || math.IsInf(y, 0) || math.IsNaN(x) || math.IsNaN(y) {
		if math.IsInf(x, 0) || math.IsInf(y, 0) {
			return math.Inf(1)
		}
		return x + y
	}
	x = math.Abs(x)
	y = math.Abs(y)
	ax, ay := x, y
	if x < y {
		ax, ay = y, x
	}

	// If ax is huge, scale both inputs down.
	if ax > hypotLargeVal {
		if ay <= ax*hypotEps {
			return ax + ay
		}
		return hypotKernel(ax*hypotScale, ay*hypotScale) / hypotScale
	}

	// If ay is tiny, scale both inputs up.
	if ay < hypotTinyVal {
		if ax >= ay/hypotEps {
			return ax + ay
		}
		return hypotKernel(ax/hypotScale, ay/hypotScale) * hypotScale
	}

	// Common case: ax is not huge and ay is not tiny.
	if ay <= ax*hypotEps {
		return ax + ay
	}
	return hypotKernel(ax, ay)
}
