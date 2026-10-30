// Package lineedit is the editing of what is typed in a console, as
// readline does it in a terminal: killing and yanking, moving by words,
// the history and searching it, and completion. It works on the text
// and the cursor alone, so that a console of any toolkit can use it.
package lineedit

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// State is what is typed and where the cursor is in it, as a byte
// offset.
type State struct {
	Text   string
	Cursor int
}

// before and after are the text either side of the cursor.
func (s State) before() string { return s.Text[:s.Cursor] }
func (s State) after() string  { return s.Text[s.Cursor:] }

// Completer says what the word that ends at cursor in text can become:
// the candidates, which replace text[start:cursor].
type Completer func(text string, cursor int) (start int, candidates []string)

// Editor is what a console keeps between keys: what was entered, what
// was killed, and a search under way.
type Editor struct {
	// history is what was entered, oldest first; at is where the
	// history keys are in it, len(history) at what is being typed,
	// which draft keeps.
	history []string
	at      int
	draft   string

	// killed is what the last kills took, which Yank puts back; a kill
	// right after one, at the State it left, adds to it.
	killed   string
	killedAt State
	kills    bool

	search *search
	menu   *menu
}

// SetHistory replaces what was entered, oldest first.
func (e *Editor) SetHistory(entries []string) {
	e.history = append([]string(nil), entries...)
	e.at, e.draft = len(e.history), ""
}

// History is what was entered, oldest first.
func (e *Editor) History() []string { return e.history }

// Add adds an entry to the history, unless it is blank or the same as
// the last, and goes back to its end.
func (e *Editor) Add(entry string) {
	if strings.TrimSpace(entry) != "" && (len(e.history) == 0 || e.history[len(e.history)-1] != entry) {
		e.history = append(e.history, entry)
	}
	e.at, e.draft = len(e.history), ""
}

// atEnd is a State with the cursor after text.
func atEnd(text string) State { return State{text, len(text)} }

// Previous brings back the entry before the one shown, keeping what is
// typed to come back to. In text of several lines it does nothing but
// on the first line, where a toolkit's Up would leave the text: ok is
// false when the key is left to the toolkit.
func (e *Editor) Previous(s State) (State, bool) {
	if strings.Contains(s.before(), "\n") {
		return s, false
	}
	if e.at == 0 {
		return s, true
	}
	if e.at == len(e.history) {
		e.draft = s.Text
	}
	e.at--
	return atEnd(e.history[e.at]), true
}

// Next is Previous the other way, from the last line, and ends at what
// was being typed.
func (e *Editor) Next(s State) (State, bool) {
	if strings.Contains(s.after(), "\n") {
		return s, false
	}
	if e.at == len(e.history) {
		return s, true
	}
	e.at++
	if e.at == len(e.history) {
		return atEnd(e.draft), true
	}
	return atEnd(e.history[e.at]), true
}

// kill takes text[from:to] out, adding it to what the last kill took
// if this one comes right after it.
func (e *Editor) kill(s State, from, to int) State {
	if from == to {
		return s
	}
	taken := s.Text[from:to]
	switch {
	case !e.kills || e.killedAt != s:
		e.killed = taken
	case from < s.Cursor:
		e.killed = taken + e.killed
	default:
		e.killed += taken
	}
	out := State{s.Text[:from] + s.Text[to:], from}
	e.killedAt, e.kills = out, true
	return out
}

// lineStart and lineEnd are where the line the cursor is on starts and
// ends.
func (s State) lineStart() int { return strings.LastIndexByte(s.before(), '\n') + 1 }
func (s State) lineEnd() int {
	if i := strings.IndexByte(s.after(), '\n'); i >= 0 {
		return s.Cursor + i
	}
	return len(s.Text)
}

// KillToEnd kills from the cursor to the end of its line, or the line's
// end itself if the cursor is there.
func (e *Editor) KillToEnd(s State) State {
	end := s.lineEnd()
	if end == s.Cursor && end < len(s.Text) {
		end++
	}
	return e.kill(s, s.Cursor, end)
}

// KillToStart kills from the start of the cursor's line to the cursor.
func (e *Editor) KillToStart(s State) State { return e.kill(s, s.lineStart(), s.Cursor) }

// KillSpaceWord kills from the cursor back to where the word of
// anything but space before it starts.
func (e *Editor) KillSpaceWord(s State) State {
	t := strings.TrimRightFunc(s.before(), unicode.IsSpace)
	from := strings.LastIndexFunc(t, unicode.IsSpace) + 1
	return e.kill(s, from, s.Cursor)
}

// KillWordBack kills from the cursor back to the start of a word.
func (e *Editor) KillWordBack(s State) State { return e.kill(s, wordBack(s), s.Cursor) }

// KillWord kills from the cursor to the end of a word.
func (e *Editor) KillWord(s State) State { return e.kill(s, s.Cursor, wordForward(s)) }

// Yank puts what the last kills took at the cursor.
func (e *Editor) Yank(s State) State {
	e.kills = false
	return Insert(s, e.killed)
}

// Insert puts text at the cursor and the cursor after it.
func Insert(s State, text string) State {
	return State{s.before() + text + s.after(), s.Cursor + len(text)}
}

// isWord reports whether r is in a word, for moving by words.
func isWord(r rune) bool { return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r) }

// wordBack is where the word before the cursor, or the one it is in,
// starts.
func wordBack(s State) int {
	i := s.Cursor
	for i > 0 {
		r, n := utf8.DecodeLastRuneInString(s.Text[:i])
		if isWord(r) {
			break
		}
		i -= n
	}
	for i > 0 {
		r, n := utf8.DecodeLastRuneInString(s.Text[:i])
		if !isWord(r) {
			break
		}
		i -= n
	}
	return i
}

// wordForward is where the word after the cursor, or the one it is in,
// ends.
func wordForward(s State) int {
	i := s.Cursor
	for i < len(s.Text) {
		r, n := utf8.DecodeRuneInString(s.Text[i:])
		if isWord(r) {
			break
		}
		i += n
	}
	for i < len(s.Text) {
		r, n := utf8.DecodeRuneInString(s.Text[i:])
		if !isWord(r) {
			break
		}
		i += n
	}
	return i
}

// WordBack moves the cursor to the start of a word.
func WordBack(s State) State { return State{s.Text, wordBack(s)} }

// WordForward moves the cursor to the end of a word.
func WordForward(s State) State { return State{s.Text, wordForward(s)} }

// DeleteBack deletes the character before the cursor.
func DeleteBack(s State) State {
	if s.Cursor == 0 {
		return s
	}
	_, n := utf8.DecodeLastRuneInString(s.before())
	return State{s.Text[:s.Cursor-n] + s.after(), s.Cursor - n}
}

// Transpose swaps the characters either side of the cursor, or the two
// before it at the end of a line, and moves the cursor past them.
func Transpose(s State) State {
	if s.Cursor == s.lineEnd() {
		_, n := utf8.DecodeLastRuneInString(s.before())
		s.Cursor -= n
	}
	if s.Cursor <= s.lineStart() || s.Cursor >= s.lineEnd() {
		return s
	}
	a, an := utf8.DecodeLastRuneInString(s.before())
	b, bn := utf8.DecodeRuneInString(s.after())
	start := s.Cursor - an
	return State{s.Text[:start] + string(b) + string(a) + s.Text[s.Cursor+bn:], s.Cursor + bn}
}

// Newline starts another line at the cursor, indented as the cursor's
// line is, and by a tab more after an opening bracket.
func Newline(s State) State {
	line := s.Text[s.lineStart():s.Cursor]
	indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
	if t := strings.TrimSpace(line); t != "" && strings.ContainsAny(t[len(t)-1:], "{([") {
		indent += "\t"
	}
	return Insert(s, "\n"+indent)
}

// commonPrefix is what a and b start with, whole characters only.
func commonPrefix(a, b string) string {
	n := 0
	for n < len(a) && n < len(b) {
		r, size := utf8.DecodeRuneInString(a[n:])
		if q, _ := utf8.DecodeRuneInString(b[n:]); q != r {
			break
		}
		n += size
	}
	return a[:n]
}

// Columns lays words out in columns across width characters, as a
// shell lists what a word can be completed to, down each column first;
// the text ends with a new line.
func Columns(words []string, width int) string {
	if len(words) == 0 {
		return ""
	}
	wide := 0
	for _, w := range words {
		wide = max(wide, utf8.RuneCountInString(w))
	}
	wide += 2
	cols := max(width/wide, 1)
	rows := (len(words) + cols - 1) / cols
	var b strings.Builder
	for r := range rows {
		var line strings.Builder
		for col := range cols {
			i := col*rows + r
			if i >= len(words) {
				break
			}
			line.WriteString(words[i] + strings.Repeat(" ", wide-utf8.RuneCountInString(words[i])))
		}
		b.WriteString(strings.TrimRight(line.String(), " "))
		b.WriteByte('\n')
	}
	return b.String()
}
