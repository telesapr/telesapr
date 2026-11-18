package api

import (
	"regexp"
	"strings"
)

// tidy takes out of the script's main what nothing uses: a variable
// that no later statement reads is not declared. A call that makes
// something (a request, a group, a constraint) stays as a statement of
// its own; a line that only reads a name out of what a call made (a
// thread's group or request, the group a new drawing starts with) goes
// altogether. Taking one out can leave another unused, as a thread's
// group that only its requests' lines named, so it goes on until none
// is left. The variables are the ones the writer names after a handle
// (g, r, c and o), each declared once.
func tidy(src string) string {
	head, body, ok := strings.Cut(src, "func main() {\n")
	if !ok {
		return src
	}
	lines := strings.Split(body, "\n")
	for {
		uses := map[string]int{}
		for _, l := range lines {
			for _, id := range identifiers.FindAllString(stringLiteral.ReplaceAllString(l, `""`), -1) {
				uses[id]++
			}
		}
		changed := false
		kept := make([]string, 0, len(lines))
		for _, l := range lines {
			m := declaration.FindStringSubmatch(l)
			if m == nil || uses[m[1]] > 1 {
				kept = append(kept, l)
				continue
			}
			changed = true
			if !readsOnly.MatchString(m[2]) {
				kept = append(kept, "\t"+m[2])
			}
		}
		lines = kept
		if !changed {
			break
		}
	}
	// A group whose lines all went leaves its blank line beside the
	// next one.
	out := make([]string, 0, len(lines))
	for i, l := range lines {
		if l == "" && i < len(lines)-1 && (len(out) == 0 || out[len(out)-1] == "") {
			continue
		}
		out = append(out, l)
	}
	return head + "func main() {\n" + strings.Join(out, "\n")
}

var (
	identifiers   = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]*`)
	stringLiteral = regexp.MustCompile(`"(?:[^"\\]|\\.)*"`)
	// declaration is a line of main that declares one of the variables
	// the writer names: a letter and the number of a handle.
	declaration = regexp.MustCompile(`^\t([grco][0-9]+) := (.*)$`)
	// readsOnly is what a declaration reads and changes nothing by.
	readsOnly = regexp.MustCompile(`^(doc\.Active\(\)|\w+\.AsThread\(\)\.(Groups\(\)\[[0-9]+\]|Circle\(\)\.Group\(\))|\w+\.Requests\(\)\[[0-9]+\]\.Entity\(\))$`)
)
