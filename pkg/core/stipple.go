package core

// stippleDashes are the dash patterns, in units of the stipple scale:
// alternating on and off lengths. Inkscape ignores all elements that
// are exactly zero instead of drawing them as dots, so dots are 1e-6.
var stippleDashes = [...][]float64{
	StippleContinuous: {},
	StippleShortDash:  {1.0, 2.0},
	StippleDash:       {1.0, 1.0},
	StippleDashDot:    {1.0, 0.5, 1e-6, 0.5},
	StippleDashDotDot: {1.0, 0.5, 1e-6, 0.5, 1e-6, 0.5},
	StippleDot:        {1e-6, 0.5},
	StippleLongDash:   {2.0, 0.5},
	StippleFreehand:   {1.0, 2.0},
	StippleZigzag:     {1.0, 2.0},
}

// StipplePatternDashes is the pattern's dash list.
func StipplePatternDashes(p StipplePattern) []float64 {
	return stippleDashes[p]
}

// StipplePatternLength is the length of one repetition of the pattern.
// The last pattern (zigzag) is left out of the table and so has length
// 0.
func StipplePatternLength(p StipplePattern) float64 {
	if p == StippleZigzag {
		return 0
	}
	length := 0.0
	for _, d := range stippleDashes[p] {
		length += d
	}
	return length
}
