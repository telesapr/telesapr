package srf

import "testing"

// TestSurfacePlanesSameAsEach holds the planes an index keeps for
// coplanarTrimCrossings to those worked out surface by surface, and
// has an index that no longer fits its shell give none.
func TestSurfacePlanesSameAsEach(t *testing.T) {
	t.Parallel()
	sh := lathedTube(10, 25, -15, 15, Tolerance{})
	for len(sh.Surface) < surfaceIndexMin {
		sh.Surface = append(sh.Surface, sh.Surface...)
	}
	sh.index = newSurfaceIndex(sh)
	planes := sh.index.planesOf(sh)
	if len(planes) != len(sh.Surface) {
		t.Fatalf("%d planes for %d surfaces", len(planes), len(sh.Surface))
	}
	flat := 0
	for i := range sh.Surface {
		if planes[i] != planeOf(&sh.Surface[i]) {
			t.Fatalf("surface %d: the index has plane %v, the surface %v", i, planes[i], planeOf(&sh.Surface[i]))
		}
		if planes[i].ok {
			flat++
		}
	}
	if flat == 0 || flat == len(sh.Surface) {
		t.Fatalf("%d of %d surfaces flat: the shell tests one kind only", flat, len(sh.Surface))
	}
	for i := range sh.Surface {
		if got := sh.index.planeOfHandle(sh, sh.Surface[i].H); got != planeOf(sh.FindSurface(sh.Surface[i].H)) {
			t.Fatalf("surface %d by handle: the index has plane %v, the surface %v", i, got, planeOf(&sh.Surface[i]))
		}
	}
	sh.Surface = sh.Surface[:len(sh.Surface)-1]
	if sh.index.planesOf(sh) != nil {
		t.Fatal("an index that no longer fits its shell gave planes")
	}
	var none *surfaceIndex
	if none.planesOf(sh) != nil {
		t.Fatal("a shell with no index gave planes")
	}
}
