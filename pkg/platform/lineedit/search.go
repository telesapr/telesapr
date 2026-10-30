package lineedit

import "strings"

// search is a search of the history under way: what is looked for,
// where the match is, and what was typed before it started.
type search struct {
	query  string
	at     int
	failed bool
	start  State
	shown  State
}

// StartSearch starts searching the history back from its end for what
// is typed next, as readline's Control+R does; again while searching,
// it looks for an older match.
func (e *Editor) StartSearch(s State) State {
	if e.search != nil {
		return e.find(e.search.at - 1)
	}
	e.search = &search{at: len(e.history), start: s, shown: s}
	return s
}

// Searching reports whether a search is under way.
func (e *Editor) Searching() bool { return e.search != nil }

// SearchPrompt is what a console shows in its prompt's place while a
// search is under way.
func (e *Editor) SearchPrompt() string {
	if e.search == nil {
		return ""
	}
	label := "(reverse-i-search)"
	if e.search.failed {
		label = "(failing reverse-i-search)"
	}
	return label + "'" + e.search.query + "': "
}

// SearchType adds text to what is looked for, and shows the newest
// entry from the match on back that has it.
func (e *Editor) SearchType(text string) State {
	e.search.query += text
	return e.find(min(e.search.at, len(e.history)-1))
}

// SearchDelete takes the last character off what is looked for, and
// looks again from the end.
func (e *Editor) SearchDelete() State {
	q := []rune(e.search.query)
	if len(q) > 0 {
		e.search.query = string(q[:len(q)-1])
	}
	if e.search.query == "" {
		e.search.at, e.search.failed, e.search.shown = len(e.history), false, e.search.start
		return e.search.start
	}
	return e.find(len(e.history) - 1)
}

// find shows the newest entry from from on back that has the query;
// with none, what is shown stays and the search says it failed.
func (e *Editor) find(from int) State {
	s := e.search
	for i := min(from, len(e.history)-1); i >= 0; i-- {
		if j := strings.LastIndex(e.history[i], s.query); j >= 0 && s.query != "" {
			s.at, s.failed = i, false
			s.shown = State{e.history[i], j}
			return s.shown
		}
	}
	s.failed = true
	return s.shown
}

// EndSearch ends the search: kept, what it shows is what is typed,
// and the history keys go on from its match; not kept, what was typed
// before it comes back.
func (e *Editor) EndSearch(keep bool) State {
	s := e.search
	e.search = nil
	if !keep {
		return s.start
	}
	if s.at < len(e.history) {
		if e.at == len(e.history) {
			e.draft = s.start.Text
		}
		e.at = s.at
	}
	return s.shown
}
