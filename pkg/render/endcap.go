package render

// EndcapCircle is cos and sin of (pi*i/10 + pi/2), for i from 0 to 10:
// the half circle a round line end is drawn with.
var EndcapCircle = [11][2]float64{
	{0.0000, 1.0000},
	{-0.3090, 0.9511},
	{-0.5878, 0.8090},
	{-0.8090, 0.5878},
	{-0.9511, 0.3090},
	{-1.0000, 0.0000},
	{-0.9511, -0.3090},
	{-0.8090, -0.5878},
	{-0.5878, -0.8090},
	{-0.3090, -0.9511},
	{0.0000, -1.0000},
}
