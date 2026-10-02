package cdt

import (
	"math"
	"math/big"
)

// The error bounds of Shewchuk's adaptive predicates: when the
// floating-point determinant is larger than the bound times the
// permanent, its sign is right.
var (
	epsilon     = math.Ldexp(1, -53)
	ccwErrBound = (3 + 16*epsilon) * epsilon
	iccErrBound = (10 + 96*epsilon) * epsilon
)

// orient is positive when a, b, c turn counter-clockwise, negative
// when clockwise, and zero when they are on a line.
func orient(a, b, c Point) int {
	l := (a.X - c.X) * (b.Y - c.Y)
	r := (a.Y - c.Y) * (b.X - c.X)
	det := l - r
	if math.Abs(det) > ccwErrBound*(math.Abs(l)+math.Abs(r)) {
		return sign(det)
	}
	return orientExact(a, b, c)
}

func sign(x float64) int {
	switch {
	case x > 0:
		return 1
	case x < 0:
		return -1
	}
	return 0
}

// exact turns floating-point numbers into integers with one common
// power of two, so that sums and products of them are exact in
// big.Int arithmetic, which unlike big.Rat needs no reduction.
func exact(xs ...float64) []*big.Int {
	emin := math.MaxInt32
	mants := make([]int64, len(xs))
	exps := make([]int, len(xs))
	for i, x := range xs {
		if x == 0 {
			continue
		}
		fr, e := math.Frexp(x)
		mants[i] = int64(fr * (1 << 53))
		exps[i] = e - 53
		if exps[i] < emin {
			emin = exps[i]
		}
	}
	out := make([]*big.Int, len(xs))
	for i := range xs {
		v := big.NewInt(mants[i])
		if mants[i] != 0 {
			v.Lsh(v, uint(exps[i]-emin))
		}
		out[i] = v
	}
	return out
}

func isub(a, b *big.Int) *big.Int { return new(big.Int).Sub(a, b) }

func imul(a, b *big.Int) *big.Int { return new(big.Int).Mul(a, b) }

func orientExact(a, b, c Point) int {
	v := exact(a.X, a.Y, b.X, b.Y, c.X, c.Y)
	l := imul(isub(v[0], v[4]), isub(v[3], v[5]))
	r := imul(isub(v[1], v[5]), isub(v[2], v[4]))
	return l.Cmp(r)
}

// incircle is positive when d lies inside the circle through a, b, c
// (counter-clockwise), negative outside, zero on it.
func incircle(a, b, c, d Point) int {
	adx, ady := a.X-d.X, a.Y-d.Y
	bdx, bdy := b.X-d.X, b.Y-d.Y
	cdx, cdy := c.X-d.X, c.Y-d.Y
	bc := bdx*cdy - cdx*bdy
	ca := cdx*ady - adx*cdy
	ab := adx*bdy - bdx*ady
	al := adx*adx + ady*ady
	bl := bdx*bdx + bdy*bdy
	cl := cdx*cdx + cdy*cdy
	det := al*bc + bl*ca + cl*ab
	perm := (math.Abs(bdx*cdy)+math.Abs(cdx*bdy))*al +
		(math.Abs(cdx*ady)+math.Abs(adx*cdy))*bl +
		(math.Abs(adx*bdy)+math.Abs(bdx*ady))*cl
	if math.Abs(det) > iccErrBound*perm {
		return sign(det)
	}
	return incircleExact(a, b, c, d)
}

func incircleExact(a, b, c, d Point) int {
	v := exact(a.X, a.Y, b.X, b.Y, c.X, c.Y, d.X, d.Y)
	adx, ady := isub(v[0], v[6]), isub(v[1], v[7])
	bdx, bdy := isub(v[2], v[6]), isub(v[3], v[7])
	cdx, cdy := isub(v[4], v[6]), isub(v[5], v[7])
	bc := isub(imul(bdx, cdy), imul(cdx, bdy))
	ca := isub(imul(cdx, ady), imul(adx, cdy))
	ab := isub(imul(adx, bdy), imul(bdx, ady))
	al := new(big.Int).Add(imul(adx, adx), imul(ady, ady))
	bl := new(big.Int).Add(imul(bdx, bdx), imul(bdy, bdy))
	cl := new(big.Int).Add(imul(cdx, cdx), imul(cdy, cdy))
	det := new(big.Int).Add(imul(al, bc), imul(bl, ca))
	det.Add(det, imul(cl, ab))
	return det.Sign()
}
