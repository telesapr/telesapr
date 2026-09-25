package geom

type Point2d struct {
	U, V float64
}

func NewPoint2d(u, v float64) Point2d {
	return Point2d{U: u, V: v}
}

func (p Point2d) Plus(b Point2d) Point2d {
	return Point2d{U: p.U + b.U, V: p.V + b.V}
}

func (p Point2d) Minus(b Point2d) Point2d {
	return Point2d{U: p.U - b.U, V: p.V - b.V}
}

func (p Point2d) ScaledBy(s float64) Point2d {
	return Point2d{U: p.U * s, V: p.V * s}
}

func (p Point2d) Equals(b Point2d) bool {
	return p == b
}
