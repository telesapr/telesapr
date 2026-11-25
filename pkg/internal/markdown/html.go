package markdown

import (
	"fmt"
	"html"
	"strings"
)

// HTML writes a page's blocks as HTML for a reader that lays it out, as
// takigo's richtext does: headings carry their anchors as ids, lists
// nest, tables are tables and a picture stands with its description
// under it. A video, which such a reader cannot play, becomes a link to
// video, the address the caller gives it to be watched at.
func HTML(blocks []Block, video string) string {
	w := &htmlWriter{}
	for _, b := range blocks {
		if b.Kind != Item {
			w.closeLists(0)
		}
		switch b.Kind {
		case Heading:
			fmt.Fprintf(&w.b, "<h%d id=\"%s\">", b.Level, html.EscapeString(Anchor(b.Spans)))
			w.spans(b.Spans)
			fmt.Fprintf(&w.b, "</h%d>\n", b.Level)
		case Paragraph:
			w.para(b.Spans)
		case Item:
			w.item(b)
		case Quote:
			w.b.WriteString("<blockquote>")
			w.spans(b.Spans)
			w.b.WriteString("</blockquote>\n")
		case Code:
			w.b.WriteString("<pre>" + html.EscapeString(b.Text) + "</pre>\n")
		case Rule:
			w.b.WriteString("<hr>\n")
		case Picture:
			fmt.Fprintf(&w.b, "<p class=\"picture\"><img src=\"%s\" alt=\"%s\">",
				html.EscapeString(b.Src), html.EscapeString(b.Alt))
			if b.Alt != "" {
				w.b.WriteString("<br><span class=\"caption\">" + html.EscapeString(b.Alt) + "</span>")
			}
			w.b.WriteString("</p>\n")
		case Video:
			fmt.Fprintf(&w.b, "<p class=\"video\">▶ <a href=\"%s\">Watch the video of this</a></p>\n",
				html.EscapeString(video))
		case Table:
			w.table(b)
		}
	}
	w.closeLists(0)
	return w.b.String()
}

type htmlWriter struct {
	b strings.Builder
	// lists are the open lists, outermost first: the closing tag of each.
	lists []string
}

func (w *htmlWriter) para(spans []Span) {
	w.b.WriteString("<p>")
	w.spans(spans)
	w.b.WriteString("</p>\n")
}

// item writes a list item, opening and closing lists to reach its depth.
// An item is left open, so that a deeper list or a further paragraph goes
// inside it.
func (w *htmlWriter) item(b Block) {
	level := max(b.Level, 1)
	if b.Number < 0 {
		// A further paragraph of the item before, inside it.
		w.closeLists(level)
		w.para(b.Spans)
		return
	}
	tag := "ul"
	if b.Number > 0 {
		tag = "ol"
	}
	w.closeLists(level)
	if len(w.lists) == level {
		if w.lists[level-1] == "</li></"+tag+">\n" {
			w.b.WriteString("</li>\n")
		} else {
			w.closeLists(level - 1)
		}
	}
	for len(w.lists) < level {
		if tag == "ol" && b.Number != 1 {
			fmt.Fprintf(&w.b, "<ol start=\"%d\">\n", b.Number)
		} else {
			w.b.WriteString("<" + tag + ">\n")
		}
		w.lists = append(w.lists, "</li></"+tag+">\n")
	}
	w.b.WriteString("<li>")
	w.spans(b.Spans)
}

// closeLists closes the open lists deeper than depth.
func (w *htmlWriter) closeLists(depth int) {
	for len(w.lists) > depth {
		w.b.WriteString(w.lists[len(w.lists)-1])
		w.lists = w.lists[:len(w.lists)-1]
	}
}

func (w *htmlWriter) table(b Block) {
	w.b.WriteString("<table>\n")
	if len(b.Header) > 0 {
		w.row(b.Header, "th")
	}
	for _, r := range b.Rows {
		w.row(r, "td")
	}
	w.b.WriteString("</table>\n")
}

func (w *htmlWriter) row(cells [][]Span, tag string) {
	w.b.WriteString("<tr>")
	for _, c := range cells {
		w.b.WriteString("<" + tag + ">")
		w.spans(c)
		w.b.WriteString("</" + tag + ">")
	}
	w.b.WriteString("</tr>\n")
}

// spans writes text with its styles, a link around the rest.
func (w *htmlWriter) spans(spans []Span) {
	for _, s := range spans {
		var open, end []string
		add := func(tag string) {
			open = append(open, "<"+tag+">")
			end = append([]string{"</" + tag + ">"}, end...)
		}
		if s.Link != "" {
			open = append(open, "<a href=\""+html.EscapeString(s.Link)+"\">")
			end = append(end, "</a>")
		}
		if s.Style&Bold != 0 {
			add("b")
		}
		if s.Style&Italic != 0 {
			add("i")
		}
		switch {
		case s.Style&Key != 0:
			add("kbd")
		case s.Style&Mono != 0:
			add("code")
		}
		w.b.WriteString(strings.Join(open, ""))
		w.b.WriteString(html.EscapeString(s.Text))
		w.b.WriteString(strings.Join(end, ""))
	}
}
