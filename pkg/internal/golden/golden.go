// Package golden compares text dumps with goldens made by the C++
// harnesses under scripts/.
package golden

import (
	"bufio"
	"strconv"
	"strings"
	"testing"
)

// CompareDump compares two dumps line by line and field by field; a
// field that differs as text is read as a float, since C's %a and
// Go's %x spell hex floats' exponents differently.
func CompareDump(t testing.TB, want, got string) {
	t.Helper()
	ws := bufio.NewScanner(strings.NewReader(want))
	gs := bufio.NewScanner(strings.NewReader(got))
	ws.Buffer(nil, 1<<24)
	gs.Buffer(nil, 1<<24)
	for line := 1; ; line++ {
		wok, gok := ws.Scan(), gs.Scan()
		if !wok || !gok {
			if wok != gok {
				t.Fatalf("line %d: dump lengths differ (C++ has more: %v)", line, wok)
			}
			return
		}
		wf, gf := strings.Fields(ws.Text()), strings.Fields(gs.Text())
		if len(wf) != len(gf) {
			t.Fatalf("line %d:\n C++: %s\n  Go: %s", line, ws.Text(), gs.Text())
		}
		for i := range wf {
			if wf[i] == gf[i] {
				continue
			}
			wv, werr := strconv.ParseFloat(wf[i], 64)
			gv, gerr := strconv.ParseFloat(gf[i], 64)
			if werr != nil || gerr != nil || wv != gv {
				t.Fatalf("line %d field %d:\n C++: %s\n  Go: %s", line, i, ws.Text(), gs.Text())
			}
		}
	}
}
