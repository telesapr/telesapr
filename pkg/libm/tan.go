package libm

import "math"

// This is sysdeps/ieee754/dbl-64/s_tan.c (IBM Accurate Mathematical
// Library), as s_tan-fma.c compiles it.

// utan.h.
var (
	td3     = fromBits(0x3FD5555555555555)
	td5     = fromBits(0x3FC11111111107C6)
	td7     = fromBits(0x3FABA1BA1CDB8745)
	td9     = fromBits(0x3F9664ED49CFC666)
	td11    = fromBits(0x3F82385A3CF2E4EA)
	te0     = fromBits(0x3FD5555555554DBD)
	te1     = fromBits(0x3FC11112E0A6B45F)
	mfftnhf = fromBits(0xc02f000000000000)
	tg1     = fromBits(0x3e4b096c00000000)
	tg2     = fromBits(0x3faf212d00000000)
	tg3     = fromBits(0x3fe92f1a00000000)
	tg4     = fromBits(0x4039000000000000)
	tg5     = fromBits(0x4197d78400000000)
	tgy2    = fromBits(0x3faf212d00000000)
	tmp1    = fromBits(0x3FF921FB58000000)
	tmp2    = fromBits(0xBE4DDE973C000000)
	tmp3    = fromBits(0xBC8CB3B399D747F2)
	tpp3    = fromBits(0xBC8CB3B398000000)
	tpp4    = fromBits(0xbacd747f23e32ed7)
	thpinv  = fromBits(0x3FE45F306DC9C883)
	ttoint  = fromBits(0x4338000000000000)
)

// eadd is EADD: z + zz = x + y exactly.
func eadd(x, y float64) (z, zz float64) {
	z = x + y
	if math.Abs(x) > math.Abs(y) {
		zz = (x - z) + y
	} else {
		zz = (y - z) + x
	}
	return z, zz
}

// esub is ESUB: z + zz = x - y exactly.
func esub(x, y float64) (z, zz float64) {
	z = x - y
	if math.Abs(x) > math.Abs(y) {
		zz = (x - z) - y
	} else {
		zz = x - (y + z)
	}
	return z, zz
}

// tanPoly is tan(a + da) for |a| <= 0.0608, as -cot when n is odd.
func tanPoly(a, da float64, n int32) float64 {
	a2 := a * a
	t2 := math.FMA(a2, td11, td9)
	t2 = math.FMA(a2, t2, td7)
	t2 = math.FMA(a2, t2, td5)
	t2 = math.FMA(a2, t2, td3)
	t2 = math.FMA(a*a2, t2, da)
	if n != 0 {
		// -cot
		b, db := eadd(a, t2)
		// DIV2 (1.0, 0.0, b, db, c, dc, t1, t2, t3, t4), with MUL12
		// by FMA.
		t1 := 1.0 / b
		t3 := t1 * b
		t4 := math.FMA(t1, b, -t3)
		t2 := math.FMA(-t1, db, ((1.0-t3)-t4)+0.0) / b
		c := t1 + t2
		dc := (t1 - c) + t2
		return -(c + dc)
	}
	// tan
	return a + t2
}

// tanTable is tan(ya + yya) for 0.0608 < |ya| <= 0.787 from the table,
// as -cot when n is odd; sy is the sign.
func tanTable(ya, yya, sy float64, n int32) float64 {
	i := int(mfftnhf + 256*ya)
	z0 := ya - xfg[4*i]
	z := z0 + yya
	z2 := z * z
	pz := math.FMA(z*z2, math.FMA(z2, te1, te0), z)
	fi := xfg[4*i+1]
	gi := xfg[4*i+2]
	if n != 0 {
		// -cot
		t2 := pz * (fi + gi) / (fi + pz)
		y := gi - t2
		return -sy * y
	}
	// tan
	t2 := pz * (gi + fi) / (gi - pz)
	y := fi + t2
	return sy * y
}

// Tan is glibc's tan.
func Tan(x float64) float64 {
	ux := hi(x)
	if ux&0x7ff00000 == 0x7ff00000 {
		return x - x
	}
	w := x
	if x < 0 {
		w = -x
	}

	// (I) abs(x) <= 1.259e-8.
	if w <= tg1 {
		return x
	}

	// (II) 1.259e-8 < abs(x) <= 0.0608.
	if w <= tg2 {
		x2 := x * x
		t2 := math.FMA(x2, td11, td9)
		t2 = math.FMA(x2, t2, td7)
		t2 = math.FMA(x2, t2, td5)
		t2 = math.FMA(x2, t2, td3)
		return math.FMA(t2, x*x2, x)
	}

	// (III) 0.0608 < abs(x) <= 0.787.
	if w <= tg3 {
		i := int(mfftnhf + 256*w)
		z := w - xfg[4*i]
		z2 := z * z
		s := 1.0
		if x < 0 {
			s = -1
		}
		pz := math.FMA(z*z2, math.FMA(z2, te1, te0), z)
		fi := xfg[4*i+1]
		gi := xfg[4*i+2]
		t2 := pz * (gi + fi) / (gi - pz)
		y := fi + t2
		return s * y
	}

	var a, da float64
	var n int32
	switch {
	case w <= tg4:
		// 0.787 < abs(x) <= 25: range reduction by algorithm i.
		t := math.FMA(x, thpinv, ttoint)
		xn := t - ttoint
		t1 := math.FMA(-xn, tmp2, math.FMA(-xn, tmp1, x))
		n = lo(t) & 1
		a = math.FMA(-xn, tmp3, t1)
		da = math.FMA(-xn, tmp3, t1-a)
	case w <= tg5:
		// 25 < abs(x) <= 1e8: range reduction by algorithm ii.
		t := math.FMA(x, thpinv, ttoint)
		xn := t - ttoint
		t1 := math.FMA(-xn, tmp2, math.FMA(-xn, tmp1, x))
		n = lo(t) & 1
		t = math.FMA(-xn, tpp3, t1)
		da = math.FMA(-xn, tpp3, t1-t)
		a = math.FMA(-xn, tpp4, t)
		da = math.FMA(-xn, tpp4, t-a) + da
		a, da = eadd(a, da)
	default:
		// 1e8 < abs(x) < 2**1024: range reduction by algorithm iii.
		var y0, y1 float64
		n, y0, y1 = remPio2(x)
		n &= 1
		a, da = eadd(y0, y1)
	}

	ya, yya, sy := a, da, 1.0
	if a < 0 {
		ya, yya, sy = -a, -da, -1
	}
	if ya <= tgy2 {
		return tanPoly(a, da, n)
	}
	return tanTable(ya, yya, sy, n)
}
