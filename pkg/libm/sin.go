package libm

import "math"

// This is sysdeps/ieee754/dbl-64/s_sin.c (IBM Accurate Mathematical
// Library), as s_sin-fma.c compiles it.

// usncs.h.
var (
	s1    = fromBits(0xbfc5555555555555)
	s2    = fromBits(0x3f81111111110ece)
	s3    = fromBits(0xbf2a01a019db08b8)
	s4    = fromBits(0x3ec71de27b9a7ed9)
	s5    = fromBits(0xbe5addffc2fcdf59)
	big   = fromBits(0x42c8000000000000)
	hp0   = fromBits(0x3ff921fb54442d18)
	hp1   = fromBits(0x3c91a62633145c07)
	mp1   = fromBits(0x3ff921fb58000000)
	mp2   = fromBits(0xbe4dde973c000000)
	pp3   = fromBits(0xbc8cb3b398000000)
	pp4   = fromBits(0xbacd747f23e32ed7)
	hpinv = fromBits(0x3fe45f306dc9c883)
	toint = fromBits(0x4338000000000000)
)

const (
	sn3 = -1.66666666666664880952546298448555e-01
	sn5 = 8.33333214285722277379541354343671e-03
	cs2 = 4.99999999999999999999950396842453e-01
	cs4 = -4.16666666666664434524222570944589e-02
	cs6 = 1.38888874007937613028114285595617e-03
)

// taylorSin is TAYLOR_SIN: sin(x + dx) for small x, xx = x*x.
func taylorSin(xx, x, dx float64) float64 {
	poly := math.FMA(math.FMA(math.FMA(math.FMA(s5, xx, s4), xx, s3), xx, s2), xx, s1)
	t := math.FMA(math.FMA(poly, x, -(0.5*dx)), xx, dx)
	return x + t
}

// doCos is do_cos: cos(x + dx) from the table.
func doCos(x, dx float64) float64 {
	if x < 0 {
		dx = -dx
	}
	u := big + math.Abs(x)
	x = math.Abs(x) - (u - big) + dx

	xx := x * x
	s := math.FMA(x*xx, math.FMA(xx, sn5, sn3), x)
	c := xx * math.FMA(xx, math.FMA(xx, cs6, cs4), cs2)
	k := lo(u) << 2
	sn, ssn, cs, ccs := sincostab[k], sincostab[k+1], sincostab[k+2], sincostab[k+3]
	cor := math.FMA(-sn, s, math.FMA(-cs, c, math.FMA(-s, ssn, ccs)))
	return cs + cor
}

// doSin is do_sin: sin(x + dx) from the table, or by Taylor series for
// small x.
func doSin(x, dx float64) float64 {
	xold := x
	if math.Abs(x) < 0.126 {
		return taylorSin(x*x, x, dx)
	}
	if x <= 0 {
		dx = -dx
	}
	u := big + math.Abs(x)
	x = math.Abs(x) - (u - big)

	xx := x * x
	s := x + math.FMA(x*xx, math.FMA(xx, sn5, sn3), dx)
	c := math.FMA(x, dx, xx*math.FMA(xx, math.FMA(xx, cs6, cs4), cs2))
	k := lo(u) << 2
	sn, ssn, cs, ccs := sincostab[k], sincostab[k+1], sincostab[k+2], sincostab[k+3]
	cor := math.FMA(cs, s, math.FMA(-sn, c, math.FMA(s, ccs, ssn)))
	return math.Copysign(sn+cor, xold)
}

// reduceSinCos is reduce_sincos: x = n pi/2 + a + da, for
// |x| < 105414350.
func reduceSinCos(x float64) (a, da float64, n int32) {
	t := math.FMA(x, hpinv, toint)
	xn := t - toint
	y := math.FMA(-xn, mp2, math.FMA(-xn, mp1, x))
	n = lo(t) & 3

	t2 := math.FMA(-xn, pp3, y)
	db := math.FMA(-xn, pp3, y-t2)

	b := math.FMA(-xn, pp4, t2)
	db += math.FMA(-xn, pp4, t2-b)
	return b, db, n
}

// doSinCos is do_sincos: sin or cos of a + da in quadrant n.
func doSinCos(a, da float64, n int32) float64 {
	var retval float64
	if n&1 != 0 {
		retval = doCos(a, da)
	} else {
		retval = doSin(a, da)
	}
	if n&2 != 0 {
		return -retval
	}
	return retval
}

// Sin is glibc's sin.
func Sin(x float64) float64 {
	k := hi(x) & 0x7fffffff
	switch {
	case k < 0x3e500000:
		return x
	case k < 0x3feb6000:
		return doSin(x, 0)
	case k < 0x400368fd:
		t := hp0 - math.Abs(x)
		return math.Copysign(doCos(t, hp1), x)
	case k < 0x419921FB:
		a, da, n := reduceSinCos(x)
		return doSinCos(a, da, n)
	case k < 0x7ff00000:
		n, y0, y1 := remPio2(x)
		return doSinCos(y0, y1, n)
	}
	return x / x
}

// Cos is glibc's cos.
func Cos(x float64) float64 {
	k := hi(x) & 0x7fffffff
	switch {
	case k < 0x3e400000:
		return 1
	case k < 0x3feb6000:
		return doCos(x, 0)
	case k < 0x400368fd:
		y := hp0 - math.Abs(x)
		a := y + hp1
		da := (y - a) + hp1
		return doSin(a, da)
	case k < 0x419921FB:
		a, da, n := reduceSinCos(x)
		return doSinCos(a, da, n+1)
	case k < 0x7ff00000:
		n, y0, y1 := remPio2(x)
		return doSinCos(y0, y1, n+1)
	}
	return x / x
}
