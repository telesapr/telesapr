package core

import (
	"math"

	"github.com/telesapr/telesapr/pkg/geom"
)

type BBox struct {
	MinP geom.Vector
	MaxP geom.Vector
}

func NewBBox(p0, p1 geom.Vector) BBox {
	return BBox{
		MinP: geom.Vector{
			X: geom.Min(p0.X, p1.X),
			Y: geom.Min(p0.Y, p1.Y),
			Z: geom.Min(p0.Z, p1.Z),
		},
		MaxP: geom.Vector{
			X: geom.Max(p0.X, p1.X),
			Y: geom.Max(p0.Y, p1.Y),
			Z: geom.Max(p0.Z, p1.Z),
		},
	}
}

// Origin is the box's centre (C++ BBox::Origin).
func (b BBox) Origin() geom.Vector { return b.MinP.Plus(b.MaxP.Minus(b.MinP).ScaledBy(0.5)) }

// Extents is half the box's size (C++ BBox::Extents).
func (b BBox) Extents() geom.Vector { return b.MaxP.Minus(b.MinP).ScaledBy(0.5) }

func (b *BBox) Include(v geom.Vector, r float64) {
	b.MinP.X = geom.Min(b.MinP.X, v.X-r)
	b.MinP.Y = geom.Min(b.MinP.Y, v.Y-r)
	b.MinP.Z = geom.Min(b.MinP.Z, v.Z-r)

	b.MaxP.X = geom.Max(b.MaxP.X, v.X+r)
	b.MaxP.Y = geom.Max(b.MaxP.Y, v.Y+r)
	b.MaxP.Z = geom.Max(b.MaxP.Z, v.Z+r)
}

// Overlaps reports whether the boxes' interiors overlap (C++
// BBox::Overlaps).
func (b BBox) Overlaps(b1 BBox) bool {
	t := b1.Origin().Minus(b.Origin())
	e := b1.Extents().Plus(b.Extents())

	return math.Abs(t.X) < e.X && math.Abs(t.Y) < e.Y && math.Abs(t.Z) < e.Z
}

// Contains reports whether p, in x and y, lies within r of the box
// (C++ BBox::Contains).
func (b BBox) Contains(p geom.Point2d, r float64) bool {
	return p.U >= (b.MinP.X-r) &&
		p.V >= (b.MinP.Y-r) &&
		p.U <= (b.MaxP.X+r) &&
		p.V <= (b.MaxP.Y+r)
}
