package srf

import "github.com/telesapr/telesapr/pkg/geom"

// OutlineInto takes the curves of plotted text (ttf.OutlineSink) into
// a list of Béziers.
type OutlineInto struct{ List *SBezierList }

func (o OutlineInto) Line(a, b geom.Vector)         { o.List.Add(SBezierFrom2(a, b)) }
func (o OutlineInto) Quadratic(a, b, c geom.Vector) { o.List.Add(SBezierFrom3(a, b, c)) }
func (o OutlineInto) Cubic(a, b, c, d geom.Vector)  { o.List.Add(SBezierFrom4(a, b, c, d)) }
