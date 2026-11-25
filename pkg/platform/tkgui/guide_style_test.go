package tkgui

import (
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/internal/markdown"
)

func TestGuideStyleFindsInstalledFonts(t *testing.T) {
	t.Parallel()
	s := newGuideStyle([]string{"DejaVu Sans Mono", "liberation serif", "Nimbus Sans Narrow"}, false)
	if s.text != "Liberation Serif" || s.typed != "DejaVu Sans Mono" || s.letter != "Nimbus Sans Narrow" {
		t.Errorf("fonts %q %q %q", s.text, s.typed, s.letter)
	}
	none := newGuideStyle(nil, true)
	if none.text != "serif" || none.typed != "monospace" || none.letter != "sans-serif" {
		t.Errorf("fonts with none installed %q %q %q", none.text, none.typed, none.letter)
	}
	if none.paper != guideBlueprint.paper || !strings.Contains(none.css(), guideBlueprint.link) {
		t.Errorf("dark style is not the blueprint: %+v", none.guidePalette)
	}
}

func TestLetteredCapitalisesHeadingsNotCode(t *testing.T) {
	t.Parallel()
	blocks := markdown.Parse("## Use `doc.Pad` here\n\n### Small\n\n| Entry | Keys |\n|---|---|\n| a | b |\n")
	got := lettered(blocks)
	if p := markdown.Plain(got[0].Spans); p != "USE doc.Pad HERE" {
		t.Errorf("heading %q", p)
	}
	if markdown.Anchor(got[0].Spans) != markdown.Anchor(blocks[0].Spans) {
		t.Error("capitals changed the heading's anchor")
	}
	if p := markdown.Plain(got[1].Spans); p != "Small" {
		t.Errorf("third-level heading %q", p)
	}
	if p := markdown.Plain(got[2].Header[0]); p != "ENTRY" || markdown.Plain(got[2].Rows[0][0]) != "a" {
		t.Errorf("table %q %q", p, markdown.Plain(got[2].Rows[0][0]))
	}
	if markdown.Plain(blocks[0].Spans) != "Use doc.Pad here" {
		t.Error("lettered changed the blocks it was given")
	}
}
