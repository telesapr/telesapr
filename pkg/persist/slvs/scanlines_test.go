package slvs

import (
	"strings"
	"testing"
)

// TestScanLinesLikeLoadFromFile pins scanLines to SolveSpace LoadFromFile's
// reading: NUL, CR and LF end a line, empty lines don't count, and a
// line it doesn't know is dropped and flagged.
func TestScanLinesLikeLoadFromFile(t *testing.T) {
	cases := []struct {
		name       string
		data       string
		lines      int
		empty, bad bool
	}{
		{"nothing", "", 0, true, false},
		{"blank lines", "\n\r\n\n", 0, true, false},
		{"NUL bytes only", strings.Repeat("\x00", 100), 0, true, false},
		{"NULs and newlines", "\x00\x00\n\x00\n", 0, true, false},
		{"junk", "hello\n", 0, false, true},
		{"known and junk", "AddGroup\nhello\nGroup.h.v=1\n", 2, false, true},
		{"version", versionString + "\r\n\r\n", 1, false, false},
		{"mesh lines", "Triangle 1 2\nAddSurface\n", 2, false, false},
		{"text after NUL is cut", "AddGroup\x00junk\n", 1, false, false},
		{"remap entries", "Group.remap={\n1 2 3\n4 5 6\n}\nAddGroup\n", 5, false, false},
	}
	for _, c := range cases {
		lines, empty, bad := scanLines([]byte(c.data))
		if len(lines) != c.lines || empty != c.empty || bad != c.bad {
			t.Errorf("%s: %d lines, empty %v, bad %v; want %d, %v, %v", c.name,
				len(lines), empty, bad, c.lines, c.empty, c.bad)
		}
	}
}
