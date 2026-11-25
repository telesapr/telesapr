package markdown

import (
	"io/fs"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/doc/guide"
)

func TestParseBlocks(t *testing.T) {
	t.Parallel()
	src := "# Title\n\nSome **bold** and *it* with `code` and <kbd>Ctrl</kbd>+<kbd>K</kbd>.\n" +
		"See [the page](other.md#part).\n\nNext.\n\n- one\n  more\n   - nested\n2. two\n\n" +
		"| A | B |\n|---|---|\n| `x\\|y` | z |\n\n    indented code\n\n![alt](images/a.png)\n\n" +
		"<video controls>\n<source src=\"videos/v.webm\" type=\"video/webm\">\n</video>\n"
	b := Parse(src)
	kinds := []Kind{Heading, Paragraph, Paragraph, Item, Item, Item, Table, Code, Picture, Video}
	if len(b) != len(kinds) {
		t.Fatalf("got %d blocks, want %d: %+v", len(b), len(kinds), b)
	}
	for k, want := range kinds {
		if b[k].Kind != want {
			t.Errorf("block %d is %v, want %v", k, b[k].Kind, want)
		}
	}
	if got := Plain(b[1].Spans); got != "Some bold and it with code and Ctrl+K. See the page." {
		t.Errorf("paragraph text %q", got)
	}
	if b[1].Spans[len(b[1].Spans)-2].Link != "other.md#part" {
		t.Errorf("link %+v", b[1].Spans)
	}
	if Plain(b[3].Spans) != "one more" || b[4].Level != 2 || b[5].Number != 2 {
		t.Errorf("list %+v %+v %+v", b[3], b[4], b[5])
	}
	if Plain(b[6].Rows[0][0]) != "x|y" || b[6].Rows[0][0][0].Style != Mono {
		t.Errorf("table cell %+v", b[6].Rows[0][0])
	}
	if b[8].Src != "images/a.png" || b[9].Src != "videos/v.webm" {
		t.Errorf("picture %q video %q", b[8].Src, b[9].Src)
	}
	if Anchor(b[0].Spans) != "title" {
		t.Errorf("anchor %q", Anchor(b[0].Spans))
	}
}

// TestGuidePagesParse reads every page of the guide and checks that no
// Markdown is left in its text, which would show as written in the window.
func TestGuidePagesParse(t *testing.T) {
	t.Parallel()
	err := fs.WalkDir(guide.Pages, ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil || !strings.HasSuffix(name, ".md") {
			return err
		}
		src, err := fs.ReadFile(guide.Pages, name)
		if err != nil {
			return err
		}
		for _, b := range Parse(string(src)) {
			var texts [][]Span
			switch b.Kind {
			case Table:
				texts = append(texts, b.Header...)
				for _, r := range b.Rows {
					texts = append(texts, r...)
				}
			case Picture:
				if _, err := fs.Stat(guide.Pages, b.Src); err != nil {
					t.Errorf("%s: picture %s: %v", name, b.Src, err)
				}
			default:
				texts = append(texts, b.Spans)
			}
			for _, spans := range texts {
				for _, s := range spans {
					if s.Style&(Mono|Key) != 0 {
						continue
					}
					for _, mark := range []string{"**", "](", "<kbd>", "`"} {
						if strings.Contains(s.Text, mark) {
							t.Errorf("%s: %q left in %q", name, mark, s.Text)
						}
					}
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestGuideNavMatchesSite checks that the window's contents are the
// site's: doc/guide/nav.yml is a copy of the nav of mkdocs.yml, which
// the embedded pages cannot reach.
func TestGuideNavMatchesSite(t *testing.T) {
	t.Parallel()
	site, err := os.ReadFile("../../../mkdocs.yml")
	if err != nil {
		t.Fatal(err)
	}
	_, nav, ok := strings.Cut(string(site), "\nnav:\n")
	if !ok {
		t.Fatal("mkdocs.yml has no nav")
	}
	// The nav ends at the next top-level key.
	var lines []string
	for line := range strings.SplitSeq(nav, "\n") {
		if line != "" && line[0] != ' ' {
			break
		}
		lines = append(lines, line)
	}
	ours, err := fs.ReadFile(guide.Pages, "nav.yml")
	if err != nil {
		t.Fatal(err)
	}
	want, got := ParseNav(strings.Join(lines, "\n")), ParseNav(string(ours))
	if !reflect.DeepEqual(got, want) {
		t.Errorf("doc/guide/nav.yml is not the nav of mkdocs.yml:\n got %+v\nwant %+v", got, want)
	}
	if len(got) < 5 || got[0].Page != "README.md" || len(got[1].Children) == 0 {
		t.Errorf("contents read wrong: %+v", got)
	}
	var walk func([]Entry)
	walk = func(es []Entry) {
		for _, e := range es {
			if e.Page != "" {
				if _, err := fs.Stat(guide.Pages, e.Page); err != nil {
					t.Errorf("contents name %s: %v", e.Page, err)
				}
			}
			walk(e.Children)
		}
	}
	walk(got)
}

func TestHTML(t *testing.T) {
	t.Parallel()
	src := "## A *b* & c\n\nSee [**it**](x.md#y) and <kbd>Ctrl</kbd>.\n\n- one\n   - nested\n- two\n\n  more\n\n" +
		"3. three\n\n| H | K |\n|---|---|\n| `x` | y |\n\n![a picture](images/p.png)\n\n" +
		"<video>\n<source src=\"v.webm\">\n</video>\n"
	got := HTML(Parse(src), "https://example.org/page/")
	for _, want := range []string{
		`<h2 id="a-b--c">A <i>b</i> &amp; c</h2>`,
		`<a href="x.md#y"><b>it</b></a>`,
		`<kbd>Ctrl</kbd>`,
		"<ul>\n<li>one<ul>\n<li>nested</li></ul>\n</li>\n<li>two<p>more</p>\n</li></ul>\n",
		`<ol start="3">`,
		"<tr><th>H</th><th>K</th></tr>\n<tr><td><code>x</code></td><td>y</td></tr>",
		`<img src="images/p.png" alt="a picture"><br><span class="caption">a picture</span>`,
		`<a href="https://example.org/page/">Watch the video of this</a>`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("HTML lacks %q:\n%s", want, got)
		}
	}
	if strings.Count(got, "<ul>") != strings.Count(got, "</ul>") || strings.Count(got, "<li>") != strings.Count(got, "</li>") {
		t.Errorf("lists not closed:\n%s", got)
	}
}
