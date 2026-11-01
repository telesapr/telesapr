package ui

import (
	"slices"
	"strings"
	"testing"
)

// A file's name too long for the bill of materials' line breaks after
// a folder where one is in the line's second half, else at the width;
// no line is longer than the width and nothing is lost.
func TestWrapPath(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		path  string
		width int
		want  []string
	}{
		{"parts/bolt.tlspr", 20, []string{"parts/bolt.tlspr"}},
		{"assemblies/frame/parts/bolt.tlspr", 20, []string{"assemblies/frame/", "parts/bolt.tlspr"}},
		{"a/verylongfoldername/x.tlspr", 10, []string{"a/verylong", "foldername", "/x.tlspr"}},
		{strings.Repeat("x", 25), 10, []string{"xxxxxxxxxx", "xxxxxxxxxx", "xxxxx"}},
		{"", 10, []string{""}},
	} {
		got := wrapPath(c.path, c.width)
		if !slices.Equal(got, c.want) {
			t.Errorf("wrapPath(%q, %d) = %q, want %q", c.path, c.width, got, c.want)
		}
		if strings.Join(got, "") != c.path {
			t.Errorf("wrapPath(%q) lost text: %q", c.path, got)
		}
	}
}
