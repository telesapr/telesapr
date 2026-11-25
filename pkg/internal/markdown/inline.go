package markdown

import "strings"

// Style is how a span is set; the bits combine.
type Style uint8

// The styles.
const (
	Bold Style = 1 << iota
	Italic
	// Mono is a code span.
	Mono
	// Key is a key of the keyboard, <kbd>.
	Key
)

// Span is a run of text in one style, a link when Link is set.
type Span struct {
	Text  string
	Style Style
	Link  string
}

// Inline reads the text of a block into spans.
func Inline(s string) []Span {
	r := &inline{}
	r.read(s)
	r.emit()
	return r.spans
}

type inline struct {
	spans []Span
	buf   strings.Builder
	style Style
	link  string
}

func (r *inline) emit() {
	if r.buf.Len() == 0 {
		return
	}
	t := r.buf.String()
	r.buf.Reset()
	if n := len(r.spans); n > 0 && r.spans[n-1].Style == r.style && r.spans[n-1].Link == r.link {
		r.spans[n-1].Text += t
		return
	}
	r.spans = append(r.spans, Span{Text: t, Style: r.style, Link: r.link})
}

// with reads s in the style st added to the current one, and in the link
// if one is given.
func (r *inline) with(s string, st Style, link string) {
	r.emit()
	oldStyle, oldLink := r.style, r.link
	r.style |= st
	if link != "" {
		r.link = link
	}
	r.read(s)
	r.emit()
	r.style, r.link = oldStyle, oldLink
}

func (r *inline) read(s string) {
	for k := 0; k < len(s); {
		c := s[k]
		switch {
		case c == '\\' && k+1 < len(s) && strings.IndexByte("\\`*_[]()#+-.!|<>", s[k+1]) >= 0:
			r.buf.WriteByte(s[k+1])
			k += 2
		case c == '`':
			n := run(s[k:], '`')
			if e := strings.Index(s[k+n:], strings.Repeat("`", n)); e >= 0 {
				r.emit()
				old := r.style
				r.style |= Mono
				r.buf.WriteString(strings.TrimSpace(s[k+n : k+n+e]))
				r.emit()
				r.style = old
				k += n + e + n
			} else {
				r.buf.WriteString(s[k : k+n])
				k += n
			}
		case strings.HasPrefix(s[k:], "<kbd>"):
			if e := strings.Index(s[k:], "</kbd>"); e >= 0 {
				r.with(s[k+5:k+e], Key, "")
				k += e + 6
			} else {
				r.buf.WriteByte(c)
				k++
			}
		case c == '!' && k+1 < len(s) && s[k+1] == '[':
			// A picture within text shows its description.
			if text, _, n, ok := bracketed(s[k+1:]); ok {
				r.buf.WriteString(text)
				k += 1 + n
			} else {
				r.buf.WriteByte(c)
				k++
			}
		case c == '[':
			if text, href, n, ok := bracketed(s[k:]); ok {
				r.with(text, 0, href)
				k += n
			} else {
				r.buf.WriteByte(c)
				k++
			}
		case c == '*' || c == '_':
			n := min(run(s[k:], c), 2)
			if c == '_' && k > 0 && wordByte(s[k-1]) {
				r.buf.WriteByte(c)
				k++
				continue
			}
			delim := strings.Repeat(string(c), n)
			e := closing(s[k+n:], delim, c)
			if e <= 0 {
				r.buf.WriteString(delim)
				k += n
				continue
			}
			st := Italic
			if n == 2 {
				st = Bold
			}
			r.with(s[k+n:k+n+e], st, "")
			k += n + e + n
		default:
			r.buf.WriteByte(c)
			k++
		}
	}
}

// closing finds the delimiter that closes an emphasis, -1 if none.
func closing(s, delim string, c byte) int {
	for k := 0; k < len(s); k++ {
		switch {
		case s[k] == '`':
			// Skip code spans: their stars are literal.
			if e := strings.IndexByte(s[k+1:], '`'); e >= 0 {
				k += e + 1
			}
		case strings.HasPrefix(s[k:], delim) && k > 0 && s[k-1] != ' ':
			if len(delim) == 1 && k+1 < len(s) && s[k+1] == c {
				// The first of a pair: a nested bold.
				k++
				continue
			}
			if c == '_' && k+len(delim) < len(s) && wordByte(s[k+len(delim)]) {
				continue
			}
			return k
		}
	}
	return -1
}

// bracketed reads "[text](href)" at the start of s and its length.
func bracketed(s string) (text, href string, n int, ok bool) {
	depth := 0
	for k := 0; k < len(s); k++ {
		switch s[k] {
		case '`':
			if e := strings.IndexByte(s[k+1:], '`'); e >= 0 {
				k += e + 1
			}
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				if k+1 >= len(s) || s[k+1] != '(' {
					return "", "", 0, false
				}
				e := strings.IndexByte(s[k+2:], ')')
				if e < 0 {
					return "", "", 0, false
				}
				return s[1:k], s[k+2 : k+2+e], k + 3 + e, true
			}
		}
	}
	return "", "", 0, false
}

func run(s string, c byte) int {
	n := 0
	for n < len(s) && s[n] == c {
		n++
	}
	return n
}

func wordByte(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}

// Plain is the text of spans without their styles.
func Plain(spans []Span) string {
	var b strings.Builder
	for _, s := range spans {
		b.WriteString(s.Text)
	}
	return b.String()
}

// Anchor is the name a heading is linked to by: its text in lower case,
// spaces turned into hyphens and other punctuation dropped, as the
// guide's site makes them.
func Anchor(spans []Span) string {
	var b strings.Builder
	for _, c := range strings.ToLower(Plain(spans)) {
		switch {
		case c == ' ' || c == '-':
			b.WriteByte('-')
		case c == '_' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c > 127:
			b.WriteRune(c)
		}
	}
	return b.String()
}
