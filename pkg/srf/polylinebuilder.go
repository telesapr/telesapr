package srf

import "github.com/spacesolve/spacesolve/pkg/geom"

// PolylineBuilder accumulates line segments and emits them as
// either a single open polyline or one polyline per closed contour.
// Used by SSurface::MakeTrimEdgesInto to walk a SCurve's polyline
// representation and produce edges for shell trimming.
type PolylineBuilder struct {
	Points    []geom.Vector
	Open      bool
	EdgeTags  []int
	MinPoint  geom.Vector
	MaxPoint  geom.Vector
	HasBounds bool
}

func (b *PolylineBuilder) Clear() {
	b.Points = b.Points[:0]
	b.Open = false
	b.EdgeTags = b.EdgeTags[:0]
	b.MinPoint = geom.Vector{}
	b.MaxPoint = geom.Vector{}
	b.HasBounds = false
}

// Start begins a new polyline at p.
func (b *PolylineBuilder) Start(p geom.Vector) {
	b.Points = append(b.Points[:0], p)
	b.EdgeTags = b.EdgeTags[:0]
	b.HasBounds = true
	b.MinPoint = p
	b.MaxPoint = p
}

// Extend appends a point to the current polyline.
func (b *PolylineBuilder) Extend(p geom.Vector, tag int) {
	b.Points = append(b.Points, p)
	b.EdgeTags = append(b.EdgeTags, tag)
	if p.X < b.MinPoint.X {
		b.MinPoint.X = p.X
	}
	if p.Y < b.MinPoint.Y {
		b.MinPoint.Y = p.Y
	}
	if p.Z < b.MinPoint.Z {
		b.MinPoint.Z = p.Z
	}
	if p.X > b.MaxPoint.X {
		b.MaxPoint.X = p.X
	}
	if p.Y > b.MaxPoint.Y {
		b.MaxPoint.Y = p.Y
	}
	if p.Z > b.MaxPoint.Z {
		b.MaxPoint.Z = p.Z
	}
}

// GenerateOutlines converts the builder's points into SEdges, with
// closeLoop controlling whether the last point connects back to the
// first.
func (b *PolylineBuilder) GenerateOutlines(el *SEdgeList, closeLoop bool) {
	for i := 0; i+1 < len(b.Points); i++ {
		el.AddEdge(b.Points[i], b.Points[i+1], 0, 0, b.EdgeTags[i])
	}
	if closeLoop && len(b.Points) > 2 {
		el.AddEdge(b.Points[len(b.Points)-1], b.Points[0], 0, 0, b.EdgeTags[len(b.EdgeTags)-1])
	}
}

// MakeFromOutlines consumes an SOutlineList and produces a
// PolylineBuilder for each connected chain of outlines. Not a
// general-purpose chain extractor — Phase 1 implementation is
// minimal; full version in Phase 3 once SSurface is available.
func MakeFromOutlines(sol *SOutlineList) []PolylineBuilder {
	var out []PolylineBuilder
	if len(sol.Outlines) == 0 {
		return out
	}
	used := make([]bool, len(sol.Outlines))
	for i := range sol.Outlines {
		if used[i] {
			continue
		}
		used[i] = true
		pb := PolylineBuilder{}
		pb.Start(sol.Outlines[i].A)
		pb.Extend(sol.Outlines[i].B, sol.Outlines[i].Tag)
		out = append(out, pb)
	}
	return out
}
