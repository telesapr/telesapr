package markdown

import (
	"strings"
)

// Entry is an entry of the guide's contents: a page, or a section of
// entries when Page is empty.
type Entry struct {
	Title    string
	Page     string
	Children []Entry
}

// ParseNav reads the guide's contents, the nav of its site: lines of
// "- Title: page.md" and "- Section:", nested by indentation, with
// comments after '#'.
func ParseNav(src string) []Entry {
	type open struct {
		indent  int
		entries *[]Entry
	}
	var root []Entry
	stack := []open{{-1, &root}}
	for line := range strings.SplitSeq(src, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "- ") {
			continue
		}
		indent := indentOf(line)
		title, page, _ := strings.Cut(strings.TrimPrefix(trimmed, "- "), ":")
		for len(stack) > 1 && stack[len(stack)-1].indent >= indent {
			stack = stack[:len(stack)-1]
		}
		into := stack[len(stack)-1].entries
		*into = append(*into, Entry{Title: strings.TrimSpace(title), Page: strings.TrimSpace(page)})
		if page := strings.TrimSpace(page); page == "" {
			e := &(*into)[len(*into)-1]
			stack = append(stack, open{indent, &e.Children})
		}
	}
	return root
}
