package core

import "github.com/spacesolve/spacesolve/pkg/geom"

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

func (b BBox) GetOrigin() geom.Vector {
	return b.MinP
}

func (b BBox) GetExtents() geom.Vector {
	return b.MaxP.Minus(b.MinP)
}

func (b *BBox) Include(v geom.Vector, r float64) {
	b.MinP.X = geom.Min(b.MinP.X, v.X-r)
	b.MinP.Y = geom.Min(b.MinP.Y, v.Y-r)
	b.MinP.Z = geom.Min(b.MinP.Z, v.Z-r)

	b.MaxP.X = geom.Max(b.MaxP.X, v.X+r)
	b.MaxP.Y = geom.Max(b.MaxP.Y, v.Y+r)
	b.MaxP.Z = geom.Max(b.MaxP.Z, v.Z+r)
}

func (b BBox) Overlaps(other BBox) bool {
	return !(b.MaxP.X < other.MinP.X || other.MaxP.X < b.MinP.X ||
		b.MaxP.Y < other.MinP.Y || other.MaxP.Y < b.MinP.Y ||
		b.MaxP.Z < other.MinP.Z || other.MaxP.Z < b.MinP.Z)
}
