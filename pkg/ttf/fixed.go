package ttf

import "math/bits"

// FreeType's fixed-point arithmetic (ftcalc.c, fttrigon.c), as its
// 64-bit build computes it. FT_Long, FT_Pos and FT_Fixed are int64.

// mulFix is FT_MulFix: (a*b) / 0x10000, rounded.
func mulFix(a, b int64) int64 {
	ab := a * b
	ab += 0x8000 + (ab >> 63)
	return ab >> 16
}

// moveSign is FT_MOVE_SIGN: x's magnitude, flipping s for a negative x.
func moveSign(x int64, s int) (uint64, int) {
	if x < 0 {
		return 0 - uint64(x), -s
	}
	return uint64(x), s
}

// divFix is FT_DivFix: (a*0x10000) / b, rounded.
func divFix(a_, b_ int64) int64 {
	s := 1
	a, s := moveSign(a_, s)
	b, s := moveSign(b_, s)
	var q uint64 = 0x7FFFFFFF
	if b > 0 {
		q = ((a << 16) + (b >> 1)) / b
	}
	if s < 0 {
		return -int64(q)
	}
	return int64(q)
}

// mulDiv is FT_MulDiv: (a*b) / c, rounded.
func mulDiv(a_, b_, c_ int64) int64 {
	s := 1
	a, s := moveSign(a_, s)
	b, s := moveSign(b_, s)
	c, s := moveSign(c_, s)
	var d uint64 = 0x7FFFFFFF
	if c > 0 {
		d = (a*b + (c >> 1)) / c
	}
	if s < 0 {
		return -int64(d)
	}
	return int64(d)
}

// pixRound is FT_PIX_ROUND.
func pixRound(x int64) int64 { return (x + 32) &^ 63 }

const (
	trigScale   = 0xDBD95B16 // the CORDIC shrink factor 0.858785336480436 * 2^32
	trigSafeMsb = 29
	trigMaxIter = 23

	anglePI  = 180 << 16
	anglePI2 = anglePI / 2
)

var trigArctanTable = [...]int64{
	1740967, 919879, 466945, 234379, 117304, 58666, 29335,
	14668, 7334, 3667, 1833, 917, 458, 229, 115,
	57, 29, 14, 7, 4, 2, 1,
}

// trigDownscale is ft_trig_downscale.
func trigDownscale(val int64) int64 {
	s := 1
	if val < 0 {
		val = -val
		s = -1
	}
	val = int64((uint64(val)*trigScale + 0x40000000) >> 32)
	if s < 0 {
		return -val
	}
	return val
}

func abs64(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}

// trigPrenorm is ft_trig_prenorm.
func trigPrenorm(x, y int64) (int64, int64, int) {
	shift := 31 - bits.LeadingZeros32(uint32(abs64(x)|abs64(y)))
	if shift <= trigSafeMsb {
		shift = trigSafeMsb - shift
		return int64(uint64(x) << shift), int64(uint64(y) << shift), shift
	}
	shift -= trigSafeMsb
	return x >> shift, y >> shift, -shift
}

// trigPseudoPolarize is ft_trig_pseudo_polarize.
func trigPseudoPolarize(x, y int64) (int64, int64) {
	var theta int64
	if y > x {
		if y > -x {
			theta = anglePI2
			x, y = y, -x
		} else {
			if y > 0 {
				theta = anglePI
			} else {
				theta = -anglePI
			}
			x, y = -x, -y
		}
	} else {
		if y < -x {
			theta = -anglePI2
			x, y = -y, x
		} else {
			theta = 0
		}
	}

	b := int64(1)
	for i, k := 1, 0; i < trigMaxIter; i, b, k = i+1, b<<1, k+1 {
		if y > 0 {
			xtemp := x + ((y + b) >> i)
			y -= (x + b) >> i
			x = xtemp
			theta += trigArctanTable[k]
		} else {
			xtemp := x - ((y + b) >> i)
			y += (x + b) >> i
			x = xtemp
			theta -= trigArctanTable[k]
		}
	}

	// FT_PAD_ROUND(theta, 16)
	if theta >= 0 {
		theta = (theta + 8) &^ 15
	} else {
		theta = -((-theta + 8) &^ 15)
	}
	return x, theta
}

// hypot is FT_Hypot (FT_Vector_Length).
func hypot(x, y int64) int64 {
	if x == 0 {
		return abs64(y)
	} else if y == 0 {
		return abs64(x)
	}
	x, y, shift := trigPrenorm(x, y)
	x, _ = trigPseudoPolarize(x, y)
	x = trigDownscale(x)
	if shift > 0 {
		return (x + (1 << (shift - 1))) >> shift
	}
	return int64(uint32(x) << -shift)
}
