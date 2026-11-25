package tkgui

import (
	"fmt"
	"slices"
	"strings"

	"github.com/takigo/takigo/font"

	"github.com/telesapr/telesapr/pkg/internal/markdown"
)

// The guide window wears the look of the guide's site
// (doc/guide/stylesheets/guide.css, from the landing page's
// site/style.css): drawing paper and ink, headings lettered in a drafting
// hand, tables ruled all round, and a blueprint in the dark scheme. What
// richtext's CSS cannot say (letter spacing, text-transform) is left out,
// or done to the text itself, as the capitals of the headings are.

// guidePalette is the site's colours of one scheme.
type guidePalette struct {
	paper, sheet, ink, faded, hair, red, link string
}

// The site's two schemes: paper and ink, and the blueprint.
var (
	guidePaper = guidePalette{paper: "#ece4cf", sheet: "#f6f0df", ink: "#1a1915", faded: "#575246",
		hair: "#8f8773", red: "#b3231b", link: "#24378a"}
	guideBlueprint = guidePalette{paper: "#0f2c57", sheet: "#143567", ink: "#e6eef8", faded: "#a9bdd8",
		hair: "#5f7ea8", red: "#ffb38a", link: "#cfe0ff"}
)

// The site's font lists: its text, its typed text and its lettering.
// richtext takes only the first family of a CSS list, so the window picks
// the first one installed. The site's own drafting font (osifont) is a web
// font a text widget cannot load; the narrow sans it falls back to on the
// site is used in its place.
var (
	guideTextFamilies   = []string{"PT Serif", "Liberation Serif", "Noto Serif", "DejaVu Serif", "Georgia", "Times New Roman"}
	guideTypedFamilies  = []string{"PT Mono", "Cascadia Mono", "DejaVu Sans Mono", "Menlo", "Consolas", "Liberation Mono", "Courier New"}
	guideLetterFamilies = []string{"osifont", "Arial Narrow", "Liberation Sans Narrow", "Nimbus Sans Narrow", "DejaVu Sans Condensed"}
)

// guideStyle is the look of the guide window: its colours, and the fonts
// found for the site's lists.
type guideStyle struct {
	guidePalette
	text, typed, letter string
}

func newGuideStyle(installed []string, dark bool) guideStyle {
	s := guideStyle{guidePalette: guidePaper}
	if dark {
		s.guidePalette = guideBlueprint
	}
	s.text = firstInstalled(installed, guideTextFamilies, "serif")
	s.typed = firstInstalled(installed, guideTypedFamilies, "monospace")
	s.letter = firstInstalled(installed, guideLetterFamilies, "sans-serif")
	return s
}

// firstInstalled is the first of the families wanted that is installed,
// or the generic family when none is.
func firstInstalled(installed, wanted []string, generic string) string {
	for _, w := range wanted {
		if slices.ContainsFunc(installed, func(f string) bool { return strings.EqualFold(f, w) }) {
			return w
		}
	}
	return generic
}

// font is the font of the page's text, from which ems and the headings'
// sizes follow.
func (s guideStyle) font() font.Attributes {
	return font.Attributes{Family: s.text, Size: 12, Weight: font.WeightNormal, Slant: font.SlantRoman}
}

// css is the stylesheet over richtext's own.
func (s guideStyle) css() string {
	return fmt.Sprintf(`
h1, h2, h3, h4, h5, h6 { font-family: "%[3]s"; font-weight: normal; color: %[1]s; }
h1 { font-size: 2.3em; border-bottom: 2px solid %[1]s; padding-bottom: 0.2em; margin: 0.4em 0 0.9em; }
h2 { font-size: 1.55em; border-bottom: 1px solid %[1]s; padding-bottom: 0.15em; margin: 1.2em 0 0.6em; }
h3 { font-size: 1.25em; }
h4 { font-size: 1.05em; }
h5, h6 { color: %[4]s; }
a { color: %[5]s; text-decoration: underline; }
a:hover { color: %[6]s; }
code, kbd, pre { font-family: "%[2]s"; font-size: 0.9em; }
code, kbd { background: %[7]s; }
pre { background: %[7]s; border: 1px solid %[1]s; border-left: 4px solid %[1]s; border-radius: 0; }
pre code { background: transparent; }
blockquote { color: %[4]s; border-left: 4px solid %[8]s; }
hr { border-top: 1px solid %[8]s; }
table { background: %[7]s; border: 2px solid %[1]s; }
th, td { border: 1px solid %[1]s; }
th { font-family: "%[3]s"; font-weight: normal; background: transparent; border-bottom: 2px solid %[1]s; }
p.picture { text-align: center; margin: 0.4em 0 1.2em; }
.caption { font-style: italic; color: %[4]s; }
`, s.ink, s.typed, s.letter, s.faded, s.link, s.red, s.sheet, s.hair)
}

// lettered writes the headings of the first two levels and the tables'
// headers in capitals, as the site's stylesheet does with text-transform,
// which richtext does not read. Code in them is left as it is typed.
func lettered(blocks []markdown.Block) []markdown.Block {
	out := slices.Clone(blocks)
	for k, b := range out {
		switch {
		case b.Kind == markdown.Heading && b.Level <= 2:
			out[k].Spans = capitals(b.Spans)
		case b.Kind == markdown.Table:
			header := make([][]markdown.Span, len(b.Header))
			for c, cell := range b.Header {
				header[c] = capitals(cell)
			}
			out[k].Header = header
		}
	}
	return out
}

func capitals(spans []markdown.Span) []markdown.Span {
	out := slices.Clone(spans)
	for k, s := range out {
		if s.Style&(markdown.Mono|markdown.Key) == 0 {
			out[k].Text = strings.ToUpper(s.Text)
		}
	}
	return out
}
