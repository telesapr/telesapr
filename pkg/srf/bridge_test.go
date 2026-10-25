package srf

import (
	"testing"

	"github.com/telesapr/telesapr/pkg/geom"
)

func TestBridgeToContourSharedVertex(t *testing.T) {
	t.Parallel()
	// Outer contour with a vertex at (2, 0); hole shares that vertex.
	outer := &SContour{}
	outer.AddPoint(geom.Vector{X: 0, Y: 0, Z: 0})
	outer.AddPoint(geom.Vector{X: 2, Y: 0, Z: 0})
	outer.AddPoint(geom.Vector{X: 4, Y: 0, Z: 0})
	outer.AddPoint(geom.Vector{X: 4, Y: 4, Z: 0})
	outer.AddPoint(geom.Vector{X: 0, Y: 4, Z: 0})
	hole := &SContour{}
	hole.AddPoint(geom.Vector{X: 2, Y: 2, Z: 0})
	hole.AddPoint(geom.Vector{X: 2, Y: 0, Z: 0})
	hole.AddPoint(geom.Vector{X: 1, Y: 0, Z: 0})
	hole.AddPoint(geom.Vector{X: 1, Y: 2, Z: 0})
	hole.AddPoint(geom.Vector{X: 2, Y: 2, Z: 0}) // holes are stored closed

	el := &SEdgeList{}
	vl := &[]geom.Vector{}
	if !outer.BridgeToContour(hole, el, vl) {
		t.Fatalf("BridgeToContour failed to find shared vertex")
	}
	// Merged at the shared vertex without a bridge: the other 4 outer
	// points plus the closed hole's 5.
	if len(outer.Points) != 9 {
		t.Errorf("merged contour points: got %d want 9", len(outer.Points))
	}
}

func TestBridgeToContourNonCrossingBridge(t *testing.T) {
	t.Parallel()
	// Outer 4x4 square; hole 1x1 square fully inside, no shared vertex.
	outer := &SContour{}
	outer.AddPoint(geom.Vector{X: 0, Y: 0, Z: 0})
	outer.AddPoint(geom.Vector{X: 4, Y: 0, Z: 0})
	outer.AddPoint(geom.Vector{X: 4, Y: 4, Z: 0})
	outer.AddPoint(geom.Vector{X: 0, Y: 4, Z: 0})
	hole := &SContour{}
	hole.AddPoint(geom.Vector{X: 1.5, Y: 1.5, Z: 0})
	hole.AddPoint(geom.Vector{X: 2.5, Y: 1.5, Z: 0})
	hole.AddPoint(geom.Vector{X: 2.5, Y: 2.5, Z: 0})
	hole.AddPoint(geom.Vector{X: 1.5, Y: 2.5, Z: 0})
	hole.AddPoint(geom.Vector{X: 1.5, Y: 1.5, Z: 0}) // holes are stored closed

	el := &SEdgeList{}
	vl := &[]geom.Vector{}
	if !outer.BridgeToContour(hole, el, vl) {
		t.Fatalf("BridgeToContour failed to find non-crossing bridge")
	}
	// With a bridge, merged has the outer 4, the hole's 4 plus its
	// repeated join point, and the repeated outer join point: 10.
	if len(outer.Points) != 10 {
		t.Errorf("merged contour: got %d points, want 10", len(outer.Points))
	}
}

func TestBridgeToContourReturnsFalseOnEmptyContours(t *testing.T) {
	t.Parallel()
	outer := &SContour{}
	hole := &SContour{}
	el := &SEdgeList{}
	vl := &[]geom.Vector{}
	if outer.BridgeToContour(hole, el, vl) {
		t.Errorf("empty contours: expected false, got true")
	}
	if outer.BridgeToContour(nil, el, vl) {
		t.Errorf("nil hole: expected false, got true")
	}
}
