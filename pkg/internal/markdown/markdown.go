// Package markdown reads the Markdown the user's guide is written in into
// blocks a window can lay out. It knows the part of Markdown the guide
// uses (headings, paragraphs, lists, fenced and indented code, tables,
// quotes, pictures on a line of their own, links, emphasis, code spans,
// <kbd> keys and the guide's <video> elements) and no more: it is not a
// general CommonMark reader.
package markdown

import (
	"strings"
)

// Kind is what a block is.
type Kind int

// The kinds of block.
const (
	// Heading is a heading of Level 1 to 6.
	Heading Kind = iota
	// Paragraph is running text.
	Paragraph
	// Item is one entry of a list, at Level 1 for the outermost list.
	Item
	// Code is a block of code, kept as written in Text.
	Code
	// Table is a table: Header is its first row, Rows the rest.
	Table
	// Picture is an image on a line of its own: Src and Alt.
	Picture
	// Video is a video: Src is its first source.
	Video
	// Quote is a quoted paragraph.
	Quote
	// Rule is a horizontal line.
	Rule
)

// Block is one block of a page.
type Block struct {
	Kind Kind
	// Level is a heading's level or a list item's depth.
	Level int
	// Number is an ordered list item's number, 0 for a bullet and -1
	// for a further paragraph of the item before it.
	Number int
	// Spans is the text of a heading, paragraph, item or quote.
	Spans []Span
	// Text is the text of a code block.
	Text string
	// Header and Rows are a table's cells.
	Header [][]Span
	Rows   [][][]Span
	// Src and Alt are a picture's or a video's file and description.
	Src, Alt string
}

// Parse reads a page.
func Parse(src string) []Block {
	p := &parser{lines: strings.Split(strings.ReplaceAll(src, "\r\n", "\n"), "\n")}
	p.run()
	return p.blocks
}

type parser struct {
	lines  []string
	i      int
	blocks []Block
	// para gathers the lines of the paragraph, item or quote being read.
	para     []string
	paraKind Kind
	paraLvl  int
	paraNum  int
	// indents are the columns of the open lists' markers, outermost first.
	indents []int
}

func (p *parser) run() {
	for p.i < len(p.lines) {
		line := p.lines[p.i]
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == "":
			p.flush()
			p.i++
		case strings.HasPrefix(trimmed, "<!--"):
			p.flush()
			p.skipComment()
		case strings.HasPrefix(trimmed, "<video"):
			p.flush()
			p.video()
		case strings.HasPrefix(trimmed, "```"):
			p.flush()
			p.fence(line)
		case indentOf(line) >= 4 && len(p.para) == 0 && len(p.indents) == 0:
			p.indented()
		case strings.HasPrefix(trimmed, "#") && heading(trimmed) > 0:
			p.flush()
			n := heading(trimmed)
			p.add(Block{Kind: Heading, Level: n, Spans: Inline(strings.TrimSpace(trimmed[n:]))})
			p.indents = nil
			p.i++
		case strings.HasPrefix(trimmed, "|"):
			p.flush()
			p.indents = nil
			p.table()
		case trimmed == "---" || trimmed == "***" || trimmed == "___":
			p.flush()
			p.indents = nil
			p.add(Block{Kind: Rule})
			p.i++
		case strings.HasPrefix(trimmed, ">"):
			if p.paraKind != Quote {
				p.flush()
			}
			p.paraKind = Quote
			p.para = append(p.para, strings.TrimSpace(strings.TrimPrefix(trimmed, ">")))
			p.i++
		default:
			if col, num, rest, ok := listMarker(line); ok {
				p.flush()
				p.item(col, num, rest)
				p.i++
				continue
			}
			if alt, src, ok := picture(trimmed); ok && len(p.para) == 0 {
				p.add(Block{Kind: Picture, Src: src, Alt: alt})
				p.i++
				continue
			}
			if len(p.para) == 0 {
				p.paraKind = Paragraph
				if len(p.indents) > 0 && indentOf(line) > p.indents[0] {
					// A paragraph inside a list item, after a blank line.
					p.paraKind, p.paraLvl = Item, len(p.indents)
					p.paraNum = -1
				} else {
					p.indents = nil
				}
			}
			p.para = append(p.para, trimmed)
			p.i++
		}
	}
	p.flush()
}

// item opens a list item whose marker is at column col.
func (p *parser) item(col, num int, rest string) {
	for len(p.indents) > 0 && p.indents[len(p.indents)-1] > col {
		p.indents = p.indents[:len(p.indents)-1]
	}
	if len(p.indents) == 0 || p.indents[len(p.indents)-1] < col {
		p.indents = append(p.indents, col)
	}
	p.paraKind, p.paraLvl, p.paraNum = Item, len(p.indents), num
	p.para = append(p.para, rest)
}

// flush ends the paragraph, item or quote being read.
func (p *parser) flush() {
	if len(p.para) == 0 {
		return
	}
	b := Block{Kind: p.paraKind, Spans: Inline(strings.Join(p.para, " "))}
	if p.paraKind == Item {
		b.Level, b.Number = p.paraLvl, p.paraNum
	}
	p.add(b)
	p.para = nil
}

func (p *parser) add(b Block) { p.blocks = append(p.blocks, b) }

func (p *parser) skipComment() {
	for p.i < len(p.lines) {
		done := strings.Contains(p.lines[p.i], "-->")
		p.i++
		if done {
			return
		}
	}
}

func (p *parser) video() {
	b := Block{Kind: Video}
	for p.i < len(p.lines) {
		line := p.lines[p.i]
		p.i++
		if b.Src == "" {
			b.Src = attr(line, "src")
		}
		if strings.Contains(line, "</video>") {
			break
		}
	}
	p.add(b)
}

func (p *parser) fence(open string) {
	indent := indentOf(open)
	p.i++
	var code []string
	for p.i < len(p.lines) {
		line := p.lines[p.i]
		p.i++
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			break
		}
		code = append(code, trimIndent(line, indent))
	}
	p.add(Block{Kind: Code, Text: strings.Join(code, "\n")})
}

func (p *parser) indented() {
	var code []string
	for p.i < len(p.lines) {
		line := p.lines[p.i]
		if strings.TrimSpace(line) != "" && indentOf(line) < 4 {
			break
		}
		code = append(code, trimIndent(line, 4))
		p.i++
	}
	for len(code) > 0 && strings.TrimSpace(code[len(code)-1]) == "" {
		code = code[:len(code)-1]
	}
	p.add(Block{Kind: Code, Text: strings.Join(code, "\n")})
}

func (p *parser) table() {
	b := Block{Kind: Table}
	first := true
	for p.i < len(p.lines) {
		line := strings.TrimSpace(p.lines[p.i])
		if !strings.HasPrefix(line, "|") {
			break
		}
		p.i++
		cells := splitRow(line)
		if separator(cells) {
			continue
		}
		row := make([][]Span, len(cells))
		for k, c := range cells {
			row[k] = Inline(c)
		}
		if first {
			b.Header, first = row, false
		} else {
			b.Rows = append(b.Rows, row)
		}
	}
	p.add(b)
}

// splitRow splits a table row at the bars not in a code span or escaped.
func splitRow(line string) []string {
	line = strings.TrimPrefix(line, "|")
	line = strings.TrimSuffix(line, "|")
	var cells []string
	var cur strings.Builder
	inCode := false
	for k := 0; k < len(line); k++ {
		c := line[k]
		switch {
		case c == '\\' && k+1 < len(line) && line[k+1] == '|':
			cur.WriteByte('|')
			k++
		case c == '`':
			inCode = !inCode
			cur.WriteByte(c)
		case c == '|' && !inCode:
			cells = append(cells, strings.TrimSpace(cur.String()))
			cur.Reset()
		default:
			cur.WriteByte(c)
		}
	}
	return append(cells, strings.TrimSpace(cur.String()))
}

func separator(cells []string) bool {
	for _, c := range cells {
		if strings.Trim(c, ":-") != "" || !strings.Contains(c, "-") {
			return false
		}
	}
	return true
}

// heading is the level of a heading line, 0 if it is not one.
func heading(s string) int {
	n := 0
	for n < len(s) && s[n] == '#' {
		n++
	}
	if n == 0 || n > 6 || (n < len(s) && s[n] != ' ') {
		return 0
	}
	return n
}

// listMarker reads a list item's marker: its column, its number (0 for a
// bullet) and the text after it.
func listMarker(line string) (col, num int, rest string, ok bool) {
	col = indentOf(line)
	s := line[col:]
	if len(s) >= 2 && (s[0] == '-' || s[0] == '*' || s[0] == '+') && s[1] == ' ' {
		return col, 0, strings.TrimSpace(s[2:]), true
	}
	k := 0
	for k < len(s) && s[k] >= '0' && s[k] <= '9' {
		num = num*10 + int(s[k]-'0')
		k++
	}
	if k > 0 && k < 10 && k+1 < len(s) && (s[k] == '.' || s[k] == ')') && s[k+1] == ' ' {
		return col, num, strings.TrimSpace(s[k+2:]), true
	}
	return 0, 0, "", false
}

// picture reads a line that is only an image.
func picture(s string) (alt, src string, ok bool) {
	if !strings.HasPrefix(s, "![") || !strings.HasSuffix(s, ")") {
		return "", "", false
	}
	mid := strings.Index(s, "](")
	if mid < 0 {
		return "", "", false
	}
	return s[2:mid], s[mid+2 : len(s)-1], true
}

// attr reads the value of an HTML attribute on a line.
func attr(line, name string) string {
	k := strings.Index(line, name+`="`)
	if k < 0 {
		return ""
	}
	v, _, found := strings.Cut(line[k+len(name)+2:], `"`)
	if !found {
		return ""
	}
	return v
}

func indentOf(line string) int {
	n := 0
	for _, c := range line {
		switch c {
		case ' ':
			n++
		case '\t':
			n += 4
		default:
			return n
		}
	}
	return n
}

func trimIndent(line string, n int) string {
	k := 0
	for k < len(line) && k < n && line[k] == ' ' {
		k++
	}
	return line[k:]
}
