package webgl

import "github.com/telesapr/telesapr/pkg/render"

// projectionMatrix is the camera as a column-major matrix from model
// space to clip space: scale, perspective, rotation into the camera's
// axes and translation by the offset in one, as the GL renderer's
// glLoadMatrixd is. The depth scale makes a model about 30000 long
// fit the depth range; the w row is the perspective, 1 for an
// axonometric view.
func projectionMatrix(c *render.Camera) [16]float32 {
	sx := c.Scale * 2.0 / c.Width
	sy := c.Scale * 2.0 / c.Height
	sz := c.Scale * 1.0 / 30000
	clp := c.Tangent * c.Scale
	right, up := c.ProjRight, c.ProjUp
	n := up.Cross(right)
	off := c.Offset
	rows := [4][4]float64{
		{sx * right.X, sx * right.Y, sx * right.Z, sx * right.Dot(off)},
		{sy * up.X, sy * up.Y, sy * up.Z, sy * up.Dot(off)},
		{sz * n.X, sz * n.Y, sz * n.Z, sz * n.Dot(off)},
		{clp * n.X, clp * n.Y, clp * n.Z, clp*n.Dot(off) + 1},
	}
	var m [16]float32
	for row := range 4 {
		for col := range 4 {
			m[col*4+row] = float32(rows[row][col])
		}
	}
	return m
}
