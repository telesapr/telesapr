package libm

import "math"

// This is sysdeps/ieee754/dbl-64/e_asin.c (IBM Accurate Mathematical
// Library), as e_asin-fma.c compiles it.

// uasncs.h.
const (
	af1 = 1.66666666666664110590506577996662e-01
	af2 = 7.50000000026122686814431784722623e-02
	af3 = 4.46428561421059750978517350006940e-02
	af4 = 3.03821268582119319911193410625235e-02
	af5 = 2.23551211026525610742786300334557e-02
	af6 = 1.81382903404565056280372531963613e-02
	t24 = 16777216.0
	t27 = 134217728.0
	rt0 = 9.99999999859990725855365213134618e-01
	rt1 = 4.99999999495955425917856814202739e-01
	rt2 = 3.75017500867345182581453026130850e-01
	rt3 = 3.12523626554518656309172508769531e-01
)

// asinSmallPoly is ((((f6*x2 + f5)*x2 + f4)*x2 + f3)*x2 + f2)*x2 + f1.
func asinSmallPoly(x2 float64) float64 {
	return math.FMA(math.FMA(math.FMA(math.FMA(math.FMA(af6, x2, af5), x2, af4), x2, af3), x2, af2), x2, af1)
}

// asincosTable finds the row of asncs for 0.125 <= |x| < 0.96875 and
// returns the arcsine of |x| as res0 + t, res0 = asncs[n+terms+2].
func asincosTable(k, m int32, x float64) (res0, t float64) {
	var n, terms int32
	switch {
	case k < 0x3fe00000:
		// 0.125 <= |x| < 0.5.
		if k < 0x3fd00000 {
			n = 11 * ((k & 0x000fffff) >> 15)
		} else {
			n = 11*((k&0x000fffff)>>14) + 352
		}
		terms = 5
	case k < 0x3fe80000:
		// 0.5 <= |x| < 0.75.
		n = 1056 + ((k&0x000fe000)>>11)*3
		terms = 6
	case k < 0x3fed8000:
		// 0.75 <= |x| < 0.921875.
		n = 992 + ((k&0x000fe000)>>13)*13
		terms = 7
	case k < 0x3fee8000:
		// 0.921875 <= |x| < 0.953125.
		n = 884 + ((k&0x000fe000)>>13)*14
		terms = 8
	default:
		// 0.953125 <= |x| < 0.96875.
		n = 768 + ((k&0x000fe000)>>13)*15
		terms = 9
	}
	a := asncs[n:]
	var xx float64
	if m > 0 {
		xx = x - a[0]
	} else {
		xx = -x - a[0]
	}
	// p = xx*xx*(a2 + xx*(a3 + ... + xx*a[terms+1])) + a[terms+2]
	q := a[terms+1]
	for j := terms; j >= 2; j-- {
		q = math.FMA(xx, q, a[j])
	}
	p := math.FMA(xx*xx, q, a[terms+2])
	t = math.FMA(a[1], xx, p)
	return a[terms+3], t
}

// sqrtNear1 is the square root of z = (1 - |x|)/2 for 0.96875 <= |x| < 1,
// as asin and acos compute it: y + cc, and the polynomial p in z.
func sqrtNear1(z float64, asin bool) (y, cc, p float64) {
	k := hi(z)
	t := inroot[(k&0x001fffff)>>14] * powtwo[511-(k>>21)]
	r := math.FMA(-(t * t), z, 1.0)
	t *= math.FMA(r, math.FMA(r, math.FMA(r, rt3, rt2), rt1), rt0)
	c := t * z
	// t = c*(1.5 - 0.5*t*c) is only used in t + y, so it's fused there.
	h := math.FMA(-(0.5 * t), c, 1.5)
	if asin {
		y = (c + t24) - t24
	} else {
		y = math.FMA(-t27, c, math.FMA(t27, c, c))
	}
	cc = math.FMA(-y, y, z) / math.FMA(c, h, y)
	p = math.FMA(math.FMA(math.FMA(math.FMA(math.FMA(af6, z, af5), z, af4), z, af3), z, af2), z, af1) * z
	return y, cc, p
}

// Asin is glibc's asin.
func Asin(x float64) float64 {
	m := hi(x)
	k := 0x7fffffff & m
	switch {
	case k < 0x3e500000:
		return x
	case k < 0x3fc00000:
		// 2^-26 <= |x| < 2^-3.
		x2 := x * x
		return math.FMA(asinSmallPoly(x2), x2*x, x)
	case k < 0x3fef0000:
		// 0.125 <= |x| < 0.96875.
		res0, t := asincosTable(k, m, x)
		res := res0 + t
		if m > 0 {
			return res
		}
		return -res
	case k < 0x3ff00000:
		// 0.96875 <= |x| < 1.
		var z float64
		if m > 0 {
			z = 0.5 * (1.0 - x)
		} else {
			z = 0.5 * (1.0 + x)
		}
		y, cc, p := sqrtNear1(z, true)
		cor := math.FMA(-(2.0 * (y + cc)), p, math.FMA(-2.0, cc, hp1))
		res1 := math.FMA(-2.0, y, hp0)
		res := res1 + cor
		if m > 0 {
			return res
		}
		return -res
	case k == 0x3ff00000 && lo(x) == 0:
		if m > 0 {
			return hp0
		}
		return -hp0
	}
	return (x - x) / (x - x)
}

// Acos is glibc's acos.
func Acos(x float64) float64 {
	m := hi(x)
	k := 0x7fffffff & m
	switch {
	case k < 0x3c880000:
		return hp0
	case k < 0x3fc00000:
		// 2.77556*10^-17 <= |x| < 2^-3.
		x2 := x * x
		r := hp0 - x
		cor := math.FMA(-asinSmallPoly(x2), x2*x, ((hp0-r)-x)+hp1)
		return r + cor
	case k < 0x3fef0000:
		// 0.125 <= |x| < 0.96875.
		res0, t := asincosTable(k, m, x)
		var y float64
		if m > 0 {
			y = hp0 - res0
			t = hp1 - t
		} else {
			y = hp0 + res0
			t = hp1 + t
		}
		return y + t
	case k < 0x3ff00000:
		// 0.96875 <= |x| < 1.
		var z float64
		if m > 0 {
			z = 0.5 * (1.0 - x)
		} else {
			z = 0.5 * (1.0 + x)
		}
		y, cc, p := sqrtNear1(z, false)
		// GCC computes p*(y + cc) once, before the branch, so it isn't
		// fused in either.
		pyc := p * (y + cc)
		if m < 0 {
			cor := (hp1 - cc) - pyc
			res1 := hp0 - y
			res := res1 + cor
			return res + res
		}
		cor := cc + pyc
		res := y + cor
		return res + res
	case k == 0x3ff00000 && lo(x) == 0:
		if m > 0 {
			return 0
		}
		return 2.0 * hp0
	}
	return (x - x) / (x - x)
}
