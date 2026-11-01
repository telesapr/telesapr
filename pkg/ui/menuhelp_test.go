package ui

import (
	"slices"
	"testing"

	"github.com/telesapr/telesapr/pkg/version"
)

// Help > User's Guide opens the guide on the project's site, where it
// has its pictures and search, and Go to GitHub the repository.
func TestHelpMenuLinks(t *testing.T) {
	t.Parallel()
	_, p := newTestApp(t)
	p.MainMenuBar.Find("Help", "User's Guide").OnTrigger()
	p.MainMenuBar.Find("Help", "Go to GitHub").OnTrigger()
	want := []string{"https://telesapr.sourceforge.io/guide/", version.Repository}
	if !slices.Equal(p.OpenedURLs, want) {
		t.Fatalf("opened %q, want %q", p.OpenedURLs, want)
	}
}
