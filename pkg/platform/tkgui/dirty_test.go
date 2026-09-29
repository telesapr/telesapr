package tkgui

import "testing"

// TestChangedRect checks the rectangle present sends: the bounding box
// of the pixels that differ, or nothing.
func TestChangedRect(t *testing.T) {
	const w, h = 7, 5
	prev := make([]byte, 4*w*h)
	next := append([]byte(nil), prev...)
	if _, _, _, _, ok := changedRect(prev, next, w, h); ok {
		t.Fatal("equal frames differ")
	}
	set := func(x, y int, b byte) { next[4*(y*w+x)+1] = b }
	set(2, 1, 9)
	set(5, 3, 9)
	set(3, 2, 9)
	x0, y0, x1, y1, ok := changedRect(prev, next, w, h)
	if !ok || x0 != 2 || y0 != 1 || x1 != 6 || y1 != 4 {
		t.Fatalf("got %d,%d-%d,%d %v, want 2,1-6,4", x0, y0, x1, y1, ok)
	}
	next = append([]byte(nil), prev...)
	set(0, 4, 1)
	x0, y0, x1, y1, ok = changedRect(prev, next, w, h)
	if !ok || x0 != 0 || y0 != 4 || x1 != 1 || y1 != 5 {
		t.Fatalf("got %d,%d-%d,%d %v, want 0,4-1,5", x0, y0, x1, y1, ok)
	}
}
