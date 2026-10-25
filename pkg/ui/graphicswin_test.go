package ui

import (
	"path/filepath"
	"testing"
)

// A file names its active group only when that is not the last one, so
// a file saved with the last group active names none and the sketch
// comes back with no active group at all. The window asks the sketch,
// which answers with the last group, rather than handing on a handle no
// group has, which would leave everything that draws the active group
// drawing nothing and the text window reading a nil group. GW.Init sets
// the sketch's own field a moment later; the window is drawn between
// the read and that, which is what is asked here.
func TestActiveGroupOfAReadFile(t *testing.T) {
	t.Parallel()
	ss, _ := newTestApp(t)
	if err := ss.LoadFromFile(filepath.Join("..", "..", "testdata", "topo", "slot.tlspr")); err != nil {
		t.Fatal(err)
	}
	if ss.SK.Active.V != 0 {
		t.Fatalf("the file named its active group as %v, so this tests nothing",
			ss.SK.Group(ss.SK.Active).Name)
	}

	hg := ss.GW.activeGroupH()
	if want := ss.SK.GroupOrder[len(ss.SK.GroupOrder)-1]; hg != want {
		t.Errorf("the active group is %v, want the last group %v", hg, want)
	}
	if g := ss.SK.Group(hg); g == nil {
		t.Errorf("the active group %v names no group", hg)
	}
}
