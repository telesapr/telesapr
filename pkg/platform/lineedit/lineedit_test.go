package lineedit

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

// at is a State written with a | where the cursor is.
func at(s string) State {
	i := strings.IndexByte(s, '|')
	return State{s[:i] + s[i+1:], i}
}

// show writes a State as at reads it.
func show(s State) string { return s.Text[:s.Cursor] + "|" + s.Text[s.Cursor:] }

// TestEditing moves and edits as readline's keys do.
func TestEditing(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		op   func(State) State
		in   string
		want string
	}{
		{"word back", WordBack, "doc.Ske|tch", "doc.|Sketch"},
		{"word back over dots", WordBack, "doc.|Sketch", "|doc.Sketch"},
		{"word back at start", WordBack, "|x", "|x"},
		{"word forward", WordForward, "d|oc.Sketch", "doc|.Sketch"},
		{"word forward over dots", WordForward, "doc|.Sketch", "doc.Sketch|"},
		{"delete back", DeleteBack, "añ|b", "a|b"},
		{"delete back at start", DeleteBack, "|ab", "|ab"},
		{"transpose", Transpose, "a|bc", "ba|c"},
		{"transpose at end", Transpose, "abc|", "acb|"},
		{"transpose at start", Transpose, "|abc", "|abc"},
		{"transpose a line's end", Transpose, "ab\n|cd", "ab\n|cd"},
		{"newline", Newline, "\tx := 1|", "\tx := 1\n\t|"},
		{"newline after a brace", Newline, "  if a {|}", "  if a {\n  \t|}"},
		{"newline on blank", Newline, "|", "\n|"},
	} {
		if got := show(c.op(at(c.in))); got != c.want {
			t.Errorf("%s: %q gives %q, want %q", c.name, c.in, got, c.want)
		}
	}
}

// TestKills kills and yanks back, a kill after a kill adding to it.
func TestKills(t *testing.T) {
	t.Parallel()
	var e Editor
	steps := []struct {
		op   func(State) State
		want string
	}{
		{e.KillToEnd, "a := |"},
		{e.Yank, "a := doc.Groups()|"},
		{e.KillSpaceWord, "a := |"},
		{e.KillSpaceWord, "a |"},
		{e.Yank, "a := doc.Groups()|"},
		{WordBack, "a := doc.|Groups()"},
		{e.KillWord, "a := doc.|()"},
		{e.KillWordBack, "a := |()"},
		{e.Yank, "a := doc.Groups|()"},
		{e.KillToStart, "|()"},
	}
	s := at("a := |doc.Groups()")
	for i, step := range steps {
		s = step.op(s)
		if show(s) != step.want {
			t.Fatalf("step %d: %q, want %q", i, show(s), step.want)
		}
	}
	// At a line's end Control+K takes the line's end.
	if got := show(e.KillToEnd(at("a|\nb"))); got != "a|b" {
		t.Errorf("killing a line's end: %q", got)
	}
	if got := show(e.KillToEnd(at("ab|"))); got != "ab|" {
		t.Errorf("killing at the end: %q", got)
	}
}

// TestHistory goes back and forth through what was entered, keeping
// what is typed, and leaves Up and Down to the text inside several
// lines.
func TestHistory(t *testing.T) {
	t.Parallel()
	var e Editor
	e.SetHistory([]string{"one", "two\nlines"})
	e.Add("three")
	e.Add("three")
	e.Add("  ")
	if !slices.Equal(e.History(), []string{"one", "two\nlines", "three"}) {
		t.Fatalf("history %q", e.History())
	}
	s := at("draft|")
	var shown []string
	prev := func() {
		var ok bool
		if s, ok = e.Previous(s); !ok {
			shown = append(shown, "(text)")
			s.Cursor = 0
			return
		}
		shown = append(shown, s.Text)
	}
	next := func() {
		var ok bool
		if s, ok = e.Next(s); !ok {
			shown = append(shown, "(text)")
			s.Cursor = len(s.Text)
			return
		}
		shown = append(shown, s.Text)
	}
	prev() // three
	prev() // two, at its end: on the second line
	prev() // Up moves to the first line
	prev() // one
	prev() // stays
	next() // two, at its end
	next() // three
	next() // draft
	next() // stays
	want := []string{"three", "two\nlines", "(text)", "one", "one", "two\nlines", "three", "draft", "draft"}
	if !slices.Equal(shown, want) {
		t.Errorf("shown %q, want %q", shown, want)
	}
	if s, ok := e.Next(at("a|\nb")); ok || show(s) != "a|\nb" {
		t.Errorf("Down above a line: %q %v", show(s), ok)
	}
}

// TestSearch searches the history back, as Control+R does.
func TestSearch(t *testing.T) {
	t.Parallel()
	var e Editor
	e.SetHistory([]string{"c := doc.Sketch.Circle(V(0, 0, 0), 2)", "doc.Groups()", "c.Radius()"})
	if e.Searching() || e.SearchPrompt() != "" {
		t.Fatal("searching before a search")
	}
	start := at("typed|")
	s := e.StartSearch(start)
	if s != start || e.SearchPrompt() != "(reverse-i-search)'': " {
		t.Fatalf("a search starts with %q, %q", show(s), e.SearchPrompt())
	}
	s = e.SearchType("c")
	if s.Text != "c.Radius()" || s.Cursor != 0 {
		t.Errorf("c found %q", show(s))
	}
	s = e.SearchType(".")
	if s.Text != "c.Radius()" {
		t.Errorf("c. found %q", show(s))
	}
	for _, want := range []string{"doc.Groups()", "c := doc.Sketch.Circle(V(0, 0, 0), 2)"} {
		if s = e.StartSearch(s); s.Text != want || show(s)[s.Cursor+1:s.Cursor+3] != "c." {
			t.Errorf("searching again found %q, want %q", show(s), want)
		}
	}
	if s = e.StartSearch(s); !strings.HasPrefix(e.SearchPrompt(), "(failing") || s.Text != "c := doc.Sketch.Circle(V(0, 0, 0), 2)" {
		t.Errorf("searching again past the oldest: %q %q", show(s), e.SearchPrompt())
	}
	s = e.SearchDelete()
	if s.Text != "c.Radius()" || e.SearchPrompt() != "(reverse-i-search)'c': " {
		t.Errorf("deleting found %q, %q", show(s), e.SearchPrompt())
	}
	if s = e.SearchDelete(); s != start {
		t.Errorf("deleting all of it shows %q", show(s))
	}
	e.SearchDelete()
	e.SearchType("Groups")
	if s = e.EndSearch(true); s.Text != "doc.Groups()" || e.Searching() {
		t.Errorf("kept %q", show(s))
	}
	// Up goes on from the match, Down back to what was typed.
	if p, _ := e.Previous(atEnd(s.Text)); p.Text != "c := doc.Sketch.Circle(V(0, 0, 0), 2)" {
		t.Errorf("Up after the search: %q", p.Text)
	}
	e.Next(State{})
	if n, _ := e.Next(State{}); n.Text != "c.Radius()" {
		t.Errorf("Down after the search: %q", n.Text)
	}
	if n, _ := e.Next(State{}); n.Text != "typed" {
		t.Errorf("Down to the end: %q", n.Text)
	}

	e.StartSearch(start)
	e.SearchType("nothing")
	if s = e.EndSearch(false); s != start {
		t.Errorf("cancelled, %q", show(s))
	}
	e.StartSearch(start)
	if s = e.EndSearch(true); s != start {
		t.Errorf("kept with nothing found, %q", show(s))
	}
}

// TestComplete completes a word as Tab does.
func TestComplete(t *testing.T) {
	t.Parallel()
	words := []string{"Groups(", "Group(", "Active(", "Sketch", "Größe"}
	complete := func(text string, cursor int) (int, []string) {
		start := strings.LastIndexAny(text[:cursor], ". (") + 1
		var out []string
		for _, w := range words {
			if strings.HasPrefix(w, text[start:cursor]) {
				out = append(out, w)
			}
		}
		return start, out
	}
	for _, c := range []struct{ in, want, list string }{
		{"doc.Ac|", "doc.Active(|", ""},
		{"doc.G|", "doc.Gr|", "Groups( Group( Größe"},
		{"doc.Gro|", "doc.Group|", "Groups( Group("},
		{"doc.Group|", "doc.Group|", "Groups( Group("},
		{"doc.Grou|x", "doc.Group|x", "Groups( Group("},
		{"doc.Q|", "doc.Q|", ""},
		{"x := |", "x := |", "Groups( Group( Active( Sketch Größe"},
		{"\tx\n  |", "\tx\n  \t|", ""},
		{"|", "\t|", ""},
		{"doc.Grö|", "doc.Größe|", ""},
	} {
		var e Editor
		s, list := e.Complete(at(c.in), complete)
		if show(s) != c.want || strings.Join(list, " ") != c.list {
			t.Errorf("%q completes to %q, listing %q", c.in, show(s), list)
		}
	}
	bad := func(string, int) (int, []string) { return 99, []string{"x"} }
	var e Editor
	if s, _ := e.Complete(at("a|"), bad); show(s) != "a|" {
		t.Errorf("a bad start: %q", show(s))
	}
	if got := commonPrefix("Größe", "Grün"); got != "Gr" {
		t.Errorf("common prefix %q", got)
	}
}

// TestCompleteCycles goes through the candidates with Tab again, and
// back with Shift+Tab, to what was typed after the last; any other
// change starts anew.
func TestCompleteCycles(t *testing.T) {
	t.Parallel()
	complete := func(text string, cursor int) (int, []string) {
		start := strings.LastIndexByte(text[:cursor], '.') + 1
		var out []string
		for _, w := range []string{"Group(", "Groups("} {
			if strings.HasPrefix(w, text[start:cursor]) {
				out = append(out, w)
			}
		}
		return start, out
	}
	var e Editor
	s, list := e.Complete(at("doc.Gro|)"), complete)
	if show(s) != "doc.Group|)" || len(list) != 2 {
		t.Fatalf("the first Tab: %q, %q", show(s), list)
	}
	if i, n, _ := e.Choice(); i != 0 || n != 2 {
		t.Errorf("before going through them, choice %d of %d", i, n)
	}
	var shown []string
	for _, back := range []bool{false, false, false, true, true} {
		if back {
			s, list = e.CompleteBack(s, complete)
		} else {
			s, list = e.Complete(s, complete)
		}
		i, n, name := e.Choice()
		shown = append(shown, fmt.Sprintf("%s %d/%d %s", show(s), i, n, name))
		if list != nil {
			t.Errorf("the list again at %q", show(s))
		}
	}
	want := []string{"doc.Group(|) 1/2 Group(", "doc.Groups(|) 2/2 Groups(", "doc.Group|) 0/2 ",
		"doc.Groups(|) 2/2 Groups(", "doc.Group(|) 1/2 Group("}
	if !slices.Equal(shown, want) {
		t.Errorf("Tab went through\n%q\nwant\n%q", shown, want)
	}
	// Typed on, Tab completes anew.
	if s, list = e.Complete(Insert(s, "x"), complete); show(s) != "doc.Group(x|)" || list != nil {
		t.Errorf("after typing: %q %q", show(s), list)
	}
	e.Complete(at("doc.G|"), complete)
	e.EndCompletion()
	if i, n, _ := e.Choice(); i != 0 || n != 0 {
		t.Errorf("after ending, choice %d of %d", i, n)
	}
}

// TestColumns lists words down columns that fit.
func TestColumns(t *testing.T) {
	t.Parallel()
	words := []string{"a", "bb", "ccc", "d", "e"}
	if got, want := Columns(words, 12), "a    d\nbb   e\nccc\n"; got != want {
		t.Errorf("columns %q, want %q", got, want)
	}
	if got := Columns(words, 1); got != "a\nbb\nccc\nd\ne\n" {
		t.Errorf("one column %q", got)
	}
	if Columns(nil, 80) != "" {
		t.Error("columns of nothing")
	}
}
