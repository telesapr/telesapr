package libm

import "math"

// This is sysdeps/ieee754/dbl-64/s_atan.c and e_atan2.c (IBM Accurate
// Mathematical Library), as s_atan-fma.c and e_atan2-fma.c compile them.

// atnat.h and atnat2.h.
var (
	ad3    = fromBits(0xbfd5555555555555)
	ad5    = fromBits(0x3fc99999999997fd)
	ad7    = fromBits(0xbfc24924923f7603)
	ad9    = fromBits(0x3fbc71c6e5129a3b)
	ad11   = fromBits(0xbfb7458022b13c25)
	ad13   = fromBits(0x3fb375f08b31cbce)
	atanA  = fromBits(0x3e4bb67a00000000)
	atanB  = fromBits(0x3fb0000000000000)
	atanC  = fromBits(0x3ff0000000000000)
	atanD  = fromBits(0x4030000000000000)
	atanE  = fromBits(0x43349ff200000000)
	hpi    = fromBits(0x3ff921fb54442d18)
	mhpi   = fromBits(0xbff921fb54442d18)
	hpi1   = fromBits(0x3c91a62633145c07)
	inv16  = fromBits(0x3fb0000000000000)
	opi    = fromBits(0x400921fb54442d18)
	opi1   = fromBits(0x3ca1a62633145c07)
	mopi   = fromBits(0xc00921fb54442d18)
	qpi    = fromBits(0x3fe921fb54442d18)
	mqpi   = fromBits(0xbfe921fb54442d18)
	tqpi   = fromBits(0x4002d97c7f3321d2)
	mtqpi  = fromBits(0xc002d97c7f3321d2)
	two500 = fromBits(0x5f30000000000000)
	twom5h = fromBits(0x20b0000000000000)
)

const two52 = 0x1.0p52

// atanPoly is d3 + v*(d5 + v*(d7 + v*(d9 + v*(d11 + v*d13)))).
func atanPoly(v float64) float64 {
	yy := math.FMA(v, ad13, ad11)
	yy = math.FMA(v, yy, ad9)
	yy = math.FMA(v, yy, ad7)
	yy = math.FMA(v, yy, ad5)
	return math.FMA(v, yy, ad3)
}

// cijPoly is c2 + z*(c3 + z*(c4 + z*(c5 + z*c6))) for row i of cij.
func cijPoly(i int, z float64) float64 {
	c := cij[7*i:]
	yy := math.FMA(z, c[6], c[5])
	yy = math.FMA(z, yy, c[4])
	yy = math.FMA(z, yy, c[3])
	return math.FMA(z, yy, c[2])
}

// cijIndex is the row of cij for u in [1/16, 1).
func cijIndex(u float64) int {
	return int((two52+256*u)-two52) - 16
}

// Atan is glibc's atan.
func Atan(x float64) float64 {
	ux := hi(x)
	dx := lo(x)
	if ux&0x7ff00000 == 0x7ff00000 && (ux&0x000fffff)|dx != 0 {
		return x + x
	}

	u := x
	if x < 0 {
		u = -x
	}
	if u < atanC {
		if u < atanB {
			if u < atanA {
				return x
			}
			// A <= u < B.
			v := x * x
			return math.FMA(atanPoly(v), x*v, x)
		}
		// B <= u < C.
		i := cijIndex(u)
		z := u - cij[7*i]
		y := math.FMA(cijPoly(i, z), z, cij[7*i+1])
		return math.Copysign(y, x)
	}
	if u < atanD {
		// C <= u < D.
		w := 1 / u
		t1 := w * u
		t2 := math.FMA(w, u, -t1)
		i := cijIndex(w)
		z := math.FMA(w, (1-t1)-t2, w-cij[7*i])
		yy := math.FMA(-z, cijPoly(i, z), hpi1)
		t1 = hpi - cij[7*i+1]
		return math.Copysign(t1+yy, x)
	}
	if u < atanE {
		// D <= u < E.
		w := 1 / u
		v := w * w
		t1 := w * u
		t2 := math.FMA(w, u, -t1)
		t3, cor := esub(hpi, w)
		yy := math.FMA(-atanPoly(v), w*v, math.FMA(-w, (1-t1)-t2, hpi1+cor))
		return math.Copysign(t3+yy, x)
	}
	// u >= E.
	if x > 0 {
		return hpi
	}
	return mhpi
}

// Atan2 is glibc's atan2.
func Atan2(y, x float64) float64 {
	const ep, em = 59768832, -59768832

	ux, dx := hi(x), lo(x)
	if ux&0x7ff00000 == 0x7ff00000 && (ux&0x000fffff)|dx != 0 {
		return x + y
	}
	uy, dy := hi(y), lo(y)
	if uy&0x7ff00000 == 0x7ff00000 && (uy&0x000fffff)|dy != 0 {
		return y + y
	}

	// y = +-0.
	if uint32(uy) == 0x00000000 && dy == 0 {
		if ux >= 0 {
			return 0
		}
		return opi
	}
	if uint32(uy) == 0x80000000 && dy == 0 {
		if ux >= 0 {
			return math.Copysign(0, -1)
		}
		return mopi
	}

	// x = +-0.
	if x == 0 {
		if uy >= 0 {
			return hpi
		}
		return mhpi
	}

	// x = +-INF.
	if uint32(ux) == 0x7ff00000 && dx == 0 {
		switch {
		case uint32(uy) == 0x7ff00000:
			if dy == 0 {
				return qpi
			}
		case uint32(uy) == 0xfff00000:
			if dy == 0 {
				return mqpi
			}
		default:
			if uy >= 0 {
				return 0
			}
			return math.Copysign(0, -1)
		}
	} else if uint32(ux) == 0xfff00000 && dx == 0 {
		switch {
		case uint32(uy) == 0x7ff00000:
			if dy == 0 {
				return tqpi
			}
		case uint32(uy) == 0xfff00000:
			if dy == 0 {
				return mtqpi
			}
		default:
			if uy >= 0 {
				return opi
			}
			return mopi
		}
	}

	// y = +-INF.
	if uint32(uy) == 0x7ff00000 && dy == 0 {
		return hpi
	}
	if uint32(uy) == 0xfff00000 && dy == 0 {
		return mhpi
	}

	// Either x/y or y/x is very close to zero.
	ax := math.Abs(x)
	ay := math.Abs(y)
	de := (uy & 0x7ff00000) - (ux & 0x7ff00000)
	if de >= ep {
		if y > 0 {
			return hpi
		}
		return mhpi
	} else if de <= em {
		if x > 0 {
			return math.Copysign(ay/ax, y)
		}
		if y > 0 {
			return opi
		}
		return mopi
	}

	// If either x or y is extremely close to zero, scale abs(x), abs(y).
	if ax < twom5h || ay < twom5h {
		ax *= two500
		ay *= two500
	}
	// Likewise for large x and y.
	if ax > two500 || ay > two500 {
		ax *= twom5h
		ay *= twom5h
	}

	// x, y which are neither special nor extreme.
	var u, du float64
	if ay < ax {
		u = ay / ax
		v := ax * u
		vv := math.FMA(ax, u, -v)
		du = ((ay - v) - vv) / ax
	} else {
		u = ax / ay
		v := ay * u
		vv := math.FMA(ay, u, -v)
		du = ((ax - v) - vv) / ay
	}

	if x > 0 {
		// (i) x>0, abs(y)< abs(x): atan(ay/ax).
		if ay < ax {
			if u < inv16 {
				v := u * u
				zz := math.FMA(u*v, atanPoly(v), du)
				return math.Copysign(u+zz, y)
			}
			i := cijIndex(u)
			t3 := u - cij[7*i]
			v, dv := eadd(t3, du)
			c := cij[7*i:]
			t1, t2 := c[1], c[2]
			p := math.FMA(v, math.FMA(v, math.FMA(v, c[6], c[5]), c[4]), c[3])
			zz := math.FMA(v, t2, math.FMA(dv, t2, v*v*p))
			return math.Copysign(t1+zz, y)
		}

		// (ii) x>0, abs(x)<=abs(y): pi/2-atan(ax/ay).
		if u < inv16 {
			// Here and in (iii) and (iv), GCC computes zz =
			// u*v*poly before ESUB (or EADD) branches, and uses it
			// after, so it isn't fused.
			v := u * u
			t2, cor := esub(hpi, u)
			t3 := ((hpi1 + cor) - du) - u*v*atanPoly(v)
			return math.Copysign(t2+t3, y)
		}
		i := cijIndex(u)
		v := (u - cij[7*i]) + du
		zz := math.FMA(-v, cijPoly(i, v), hpi1)
		t1 := hpi - cij[7*i+1]
		return math.Copysign(t1+zz, y)
	}

	// (iii) x<0, abs(x)< abs(y): pi/2+atan(ax/ay).
	if ax < ay {
		if u < inv16 {
			v := u * u
			t2, cor := eadd(hpi, u)
			t3 := ((hpi1 + cor) + du) + u*v*atanPoly(v)
			return math.Copysign(t2+t3, y)
		}
		i := cijIndex(u)
		v := (u - cij[7*i]) + du
		zz := math.FMA(v, cijPoly(i, v), hpi1)
		t1 := hpi + cij[7*i+1]
		return math.Copysign(t1+zz, y)
	}

	// (iv) x<0, abs(y)<=abs(x): pi-atan(ax/ay).
	if u < inv16 {
		v := u * u
		t2, cor := esub(opi, u)
		t3 := ((opi1 + cor) - du) - u*v*atanPoly(v)
		return math.Copysign(t2+t3, y)
	}
	i := cijIndex(u)
	v := (u - cij[7*i]) + du
	zz := math.FMA(-v, cijPoly(i, v), opi1)
	t1 := opi - cij[7*i+1]
	return math.Copysign(t1+zz, y)
}
