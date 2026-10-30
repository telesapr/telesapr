package lineedit

import "strings"

// menu is a completion that found several candidates, which Tab then
// goes through: each in turn takes the word's place, and after the last
// what was typed comes back.
type menu struct {
	// start is where the word starts, typed what it was before Tab
	// went through the candidates, at the candidate shown, -1 for typed.
	start      int
	typed      string
	candidates []string
	at         int
	// shown is the State the last step left: Tab goes on through the
	// candidates only from it, and anything else starts anew.
	shown State
}

// Complete completes the word before the cursor, as Tab does. With one
// candidate it completes to it. With several it completes as far as
// they agree and returns them, to be shown; Tab again from there puts
// each in the word's place in turn, and after the last the word as it
// was. With nothing but space before the cursor on its line it inserts
// a tab.
func (e *Editor) Complete(s State, complete Completer) (State, []string) {
	return e.complete(s, complete, 1)
}

// CompleteBack is Complete going through the candidates backwards, as
// Shift+Tab does.
func (e *Editor) CompleteBack(s State, complete Completer) (State, []string) {
	return e.complete(s, complete, -1)
}

func (e *Editor) complete(s State, complete Completer, step int) (State, []string) {
	if m := e.menu; m != nil && m.shown == s {
		// The positions are -1, the word as typed, and the candidates.
		n := len(m.candidates) + 1
		m.at = ((m.at+1+step)%n+n)%n - 1
		word := m.typed
		if m.at >= 0 {
			word = m.candidates[m.at]
		}
		m.shown = State{s.Text[:m.start] + word + s.after(), m.start + len(word)}
		return m.shown, nil
	}
	e.menu = nil
	if strings.TrimSpace(s.Text[s.lineStart():s.Cursor]) == "" {
		return Insert(s, "\t"), nil
	}
	start, candidates := complete(s.Text, s.Cursor)
	if len(candidates) == 0 || start < 0 || start > s.Cursor {
		return s, nil
	}
	common := candidates[0]
	for _, c := range candidates[1:] {
		common = commonPrefix(common, c)
	}
	out := State{s.Text[:start] + common + s.after(), start + len(common)}
	if len(candidates) == 1 {
		return out, nil
	}
	e.menu = &menu{start: start, typed: common, candidates: candidates, at: -1, shown: out}
	return out, candidates
}

// Choice is which candidate Tab shows now: its number from 1, how many
// there are, and the candidate. i is 0 when what was typed is shown, or
// when Tab isn't going through candidates.
func (e *Editor) Choice() (i, n int, candidate string) {
	m := e.menu
	if m == nil {
		return 0, 0, ""
	}
	if m.at < 0 {
		return 0, len(m.candidates), ""
	}
	return m.at + 1, len(m.candidates), m.candidates[m.at]
}

// EndCompletion ends going through candidates, as any key but Tab
// does.
func (e *Editor) EndCompletion() { e.menu = nil }
