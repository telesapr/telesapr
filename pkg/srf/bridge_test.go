package srf

import (
	"testing"

	"github.com/spacesolve/spacesolve/pkg/geom"
)

func TestBridgeToContourSharedVertex(t *testing.T) {
	// Outer contour with a vertex at (2, 0); hole shares that vertex.
	outer := &SContour{}
	outer.AddPoint(geom.Vector{X: 0, Y: 0, Z: 0})
	outer.AddPoint(geom.Vector{X: 4, Y: 0, Z: 0})
	outer.AddPoint(geom.Vector{X: 4, Y: 4, Z: 0})
	outer.AddPoint(geom.Vector{X: 0, Y: 4, Z: 0})
	hole := &SContour{}
	hole.AddPoint(geom.Vector{X: 2, Y: 2, Z: 0})
	hole.AddPoint(geom.Vector{X: 2, Y: 0, Z: 0})
	hole.AddPoint(geom.Vector{X: 1, Y: 0, Z: 0})
	hole.AddPoint(geom.Vector{X: 1, Y: 2, Z: 0})

	el := &SEdgeList{}
	vl := &[]geom.Vector{}
	if !outer.BridgeToContour(hole, el, vl) {
		t.Fatalf("BridgeToContour failed to find shared vertex")
	}
	if len(outer.Points) != 8 {
		t.Errorf("merged contour points: got %d want 8", len(outer.Points))
	}
}

func TestBridgeToContourNonCrossingBridge(t *testing.T) {
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

	el := &SEdgeList{}
	vl := &[]geom.Vector{}
	if !outer.BridgeToContour(hole, el, vl) {
		t.Fatalf("BridgeToContour failed to find non-crossing bridge")
	}
	// With a non-shared-vertex bridge, merged has outer 4 + hole 4 = 8
	// points (the bridge endpoint replaces outer[thisp] once).
	if len(outer.Points) != 8 {
		t.Errorf("merged contour: got %d points, want 8", len(outer.Points))
	}
}

func TestBridgeToContourReturnsFalseOnEmptyContours(t *testing.T) {
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
