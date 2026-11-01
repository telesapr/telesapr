package ui

import (
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/platform/headless"
)

// A separator is drawn only between entries: none first, none last,
// never two together.
func TestTidyMenuSeparators(t *testing.T) {
	t.Parallel()
	inner := headless.New().CreateMenu().(*headless.Menu)
	m := &tidyMenu{Menu: inner}
	m.AddSeparator()
	m.AddItem("a", nil, false)
	m.AddSeparator()
	m.AddSeparator()
	m.AddSubMenu("b")
	m.AddSeparator()
	m.AddItem("c", nil, false)
	m.AddSeparator()
	var got []string
	for _, it := range inner.Items {
		if it.Separator {
			got = append(got, "---")
		} else {
			got = append(got, it.Label)
		}
	}
	if want := "a --- b --- c"; strings.Join(got, " ") != want {
		t.Fatalf("menu %q, want %q", strings.Join(got, " "), want)
	}
}
