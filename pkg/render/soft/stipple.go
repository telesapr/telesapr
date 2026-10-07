package soft

// stippleA and stippleB are the two complementary checkerboards of
// ssglFillPattern, 32 rows of 32 bits from the bottom, the leftmost
// pixel in the top bit.
var stippleA, stippleB = makeStipples()

func makeStipples() (a, b [32]uint32) {
	for x := range 32 {
		for y := range 32 {
			ym, xm := y%4, x%4
			for k := range 2 {
				if xm >= 1 && xm <= 2 && ym >= 1 && ym <= 2 {
					if k == 0 {
						b[y] |= 0x80000000 >> uint(x)
					} else {
						a[y] |= 0x80000000 >> uint(x)
					}
				}
				ym = (ym + 2) % 4
				xm = (xm + 2) % 4
			}
		}
	}
	return
}
