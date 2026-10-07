package libm

import "math"

// This is sysdeps/ieee754/dbl-64/e_rem_pio2.c and k_rem_pio2.c (from
// fdlibm), the range reduction of sin, cos and tan for |x| >= 105414350.
// They are compiled without FMA.

var piO2 = []float64{
	1.57079625129699707031e+00,
	7.54978941586159635335e-08,
	5.39030252995776476554e-15,
	3.28200341580791294123e-22,
	1.27065575308067607349e-29,
	1.22933308981111328932e-36,
	2.73370053816464559624e-44,
	2.16741683877804819444e-51,
}

const (
	two24  = 1.67772160000000000000e+07
	twon24 = 5.96046447753906250000e-08
)

// remPio2 is __ieee754_rem_pio2: x = n pi/2 + y0 + y1.
func remPio2(x float64) (n int32, y0, y1 float64) {
	hx := hi(x)
	ix := hx & 0x7fffffff
	low := uint32(lo(x))
	e0 := (ix >> 20) - 1046
	z := math.Float64frombits(uint64(uint32(ix-(e0<<20)))<<32 | uint64(low))
	var tx [3]float64
	for i := range 2 {
		tx[i] = float64(int32(z))
		z = (z - tx[i]) * two24
	}
	tx[2] = z
	nx := 3
	for tx[nx-1] == 0 {
		nx--
	}
	var y [2]float64
	n = kernelRemPio2(tx[:], y[:], int(e0), nx, 2, twoOverPi)
	if hx < 0 {
		return -n, -y[0], -y[1]
	}
	return n, y[0], y[1]
}

// kernelRemPio2 is __kernel_rem_pio2 for prec 1 or 2.
func kernelRemPio2(x, y []float64, e0, nx, prec int, ipio2 []int32) int32 {
	initJk := []int{2, 3, 4, 6}
	var iq [20]int32
	var f, fq, q [20]float64

	jk := initJk[prec]
	jp := jk

	jx := nx - 1
	jv := max((e0-3)/24, 0)
	q0 := e0 - 24*(jv+1)

	j := jv - jx
	m := jx + jk
	for i := 0; i <= m; i, j = i+1, j+1 {
		if j < 0 {
			f[i] = 0
		} else {
			f[i] = float64(ipio2[j])
		}
	}

	for i := 0; i <= jk; i++ {
		fw := 0.0
		for j := 0; j <= jx; j++ {
			fw += x[j] * f[jx+i-j]
		}
		q[i] = fw
	}

	jz := jk
	var z float64
	var n, ih int32
	for {
		// Distill q[] into iq[] reversingly.
		i := 0
		j := jz
		z = q[jz]
		for ; j > 0; i, j = i+1, j-1 {
			fw := float64(int32(twon24 * z))
			iq[i] = int32(z - two24*fw)
			z = q[j-1] + fw
		}

		z = math.Ldexp(z, q0)
		z -= 8.0 * math.Floor(z*0.125)
		n = int32(z)
		z -= float64(n)
		ih = 0
		switch {
		case q0 > 0:
			i := iq[jz-1] >> (24 - q0)
			n += i
			iq[jz-1] -= i << (24 - q0)
			ih = iq[jz-1] >> (23 - q0)

		case q0 == 0:
			ih = iq[jz-1] >> 23

		case z >= 0.5:
			ih = 2
		}

		if ih > 0 {
			n++
			carry := int32(0)
			for i := range jz {
				j := iq[i]
				if carry == 0 {
					if j != 0 {
						carry = 1
						iq[i] = 0x1000000 - j
					}
				} else {
					iq[i] = 0xffffff - j
				}
			}
			if q0 > 0 {
				switch q0 {
				case 1:
					iq[jz-1] &= 0x7fffff
				case 2:
					iq[jz-1] &= 0x3fffff
				}
			}
			if ih == 2 {
				z = 1 - z
				if carry != 0 {
					z -= math.Ldexp(1, q0)
				}
			}
		}

		// Check if recomputation is needed.
		if z == 0 {
			j := int32(0)
			for i := jz - 1; i >= jk; i-- {
				j |= iq[i]
			}
			if j == 0 {
				k := 1
				for iq[jk-k] == 0 {
					k++
				}
				for i := jz + 1; i <= jz+k; i++ {
					f[jx+i] = float64(ipio2[jv+i])
					fw := 0.0
					for j := 0; j <= jx; j++ {
						fw += x[j] * f[jx+i-j]
					}
					q[i] = fw
				}
				jz += k
				continue
			}
		}
		break
	}

	// Chop off zero terms.
	if z == 0 {
		jz--
		q0 -= 24
		for iq[jz] == 0 {
			jz--
			q0 -= 24
		}
	} else {
		// Break z into 24-bit if necessary.
		z = math.Ldexp(z, -q0)
		if z >= two24 {
			fw := float64(int32(twon24 * z))
			iq[jz] = int32(z - two24*fw)
			jz++
			q0 += 24
			iq[jz] = int32(fw)
		} else {
			iq[jz] = int32(z)
		}
	}

	// Convert integer "bit" chunk to floating-point value.
	fw := math.Ldexp(1, q0)
	for i := jz; i >= 0; i-- {
		q[i] = fw * float64(iq[i])
		fw *= twon24
	}

	// Compute PIo2[0,...,jp]*q[jz,...,0].
	for i := jz; i >= 0; i-- {
		fw := 0.0
		for k := 0; k <= jp && k <= jz-i; k++ {
			fw += piO2[k] * q[i+k]
		}
		fq[jz-i] = fw
	}

	// Compress fq[] into y[].
	fv := 0.0
	for i := jz; i >= 0; i-- {
		fv += fq[i]
	}
	if ih == 0 {
		y[0] = fv
	} else {
		y[0] = -fv
	}
	fv = fq[0] - fv
	for i := 1; i <= jz; i++ {
		fv += fq[i]
	}
	if ih == 0 {
		y[1] = fv
	} else {
		y[1] = -fv
	}
	return n & 7
}
