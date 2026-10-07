package core

type RGBA struct {
	Red   uint8
	Green uint8
	Blue  uint8
	Alpha uint8
}

func NewRGBA(r, g, b, a uint8) RGBA {
	return RGBA{Red: r, Green: g, Blue: b, Alpha: a}
}

func RGBAFromFloat(r, g, b, a float32) RGBA {
	return RGBA{
		Red:   uint8(255.1 * r),
		Green: uint8(255.1 * g),
		Blue:  uint8(255.1 * b),
		Alpha: uint8(255.1 * a),
	}
}

func (c RGBA) RedF() float32   { return float32(c.Red) / 255.0 }
func (c RGBA) GreenF() float32 { return float32(c.Green) / 255.0 }
func (c RGBA) BlueF() float32  { return float32(c.Blue) / 255.0 }
func (c RGBA) AlphaF() float32 { return float32(c.Alpha) / 255.0 }

func (c RGBA) IsEmpty() bool {
	return c.Alpha == 0
}

func (c RGBA) Equals(other RGBA) bool {
	return c.Red == other.Red &&
		c.Green == other.Green &&
		c.Blue == other.Blue &&
		c.Alpha == other.Alpha
}

func (c RGBA) WithAlpha(alpha uint8) RGBA {
	return RGBA{c.Red, c.Green, c.Blue, alpha}
}

func (c RGBA) ToPackedInt() uint32 {
	return uint32(c.Red) |
		(uint32(c.Green) << 8) |
		(uint32(c.Blue) << 16) |
		(uint32(255-c.Alpha) << 24)
}

func (c RGBA) ToARGB32() uint32 {
	return uint32(c.Blue) |
		(uint32(c.Green) << 8) |
		(uint32(c.Red) << 16) |
		(uint32(c.Alpha) << 24)
}

func RGBAFromPackedInt(rgba uint32) RGBA {
	return RGBA{
		Red:   uint8(rgba & 0xff),
		Green: uint8((rgba >> 8) & 0xff),
		Blue:  uint8((rgba >> 16) & 0xff),
		Alpha: uint8(255 - ((rgba >> 24) & 0xff)),
	}
}
