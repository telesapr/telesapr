package core

import (
	"github.com/spacesolve/spacesolve/pkg/geom"
	"math"
)

type BBox struct {
	MinP geom.Vector
	MaxP geom.Vector
}

func NewBBox(p0, p1 geom.Vector) BBox {
	return BBox{
		MinP: geom.Vector{
			X: math.Min(p0.X, p1.X),
			Y: math.Min(p0.Y, p1.Y),
			Z: math.Min(p0.Z, p1.Z),
		},
		MaxP: geom.Vector{
			X: math.Max(p0.X, p1.X),
			Y: math.Max(p0.Y, p1.Y),
			Z: math.Max(p0.Z, p1.Z),
		},
	}
}

func (b BBox) GetOrigin() geom.Vector {
	return b.MinP
}

func (b BBox) GetExtents() geom.Vector {
	return b.MaxP.Minus(b.MinP)
}

func (b *BBox) Include(v geom.Vector, r float64) {
	b.MinP.X = math.Min(b.MinP.X, v.X-r)
	b.MinP.Y = math.Min(b.MinP.Y, v.Y-r)
	b.MinP.Z = math.Min(b.MinP.Z, v.Z-r)

	b.MaxP.X = math.Max(b.MaxP.X, v.X+r)
	b.MaxP.Y = math.Max(b.MaxP.Y, v.Y+r)
	b.MaxP.Z = math.Max(b.MaxP.Z, v.Z+r)
}

func (b BBox) Overlaps(other BBox) bool {
	return !(b.MaxP.X < other.MinP.X || other.MaxP.X < b.MinP.X ||
		b.MaxP.Y < other.MinP.Y || other.MaxP.Y < b.MinP.Y ||
		b.MaxP.Z < other.MinP.Z || other.MaxP.Z < b.MinP.Z)
}
