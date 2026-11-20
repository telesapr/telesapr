package script

import (
	"regexp"
	"slices"
	"testing"

	"github.com/telesapr/telesapr/pkg/script/symbols"
)

// The ways a comment of package api names another declaration.
var (
	// SetDrawing, AddDrawingDimension: a name of two humps or more, not
	// after a dot or in a word or a quotation.
	commentCamel = regexp.MustCompile(`(?:^|[^\w."/])([A-Z][a-z0-9]+(?:[A-Z][a-z0-9]*)+)\b`)
	// Drawing.Get, doc.Params.Set, Group.Analyze
	commentDotted = regexp.MustCompile(`(?:^|[^\w.])(doc\.)?([A-Z]\w*)\.([A-Z]\w*)\b`)
)

// commentWords are words of two humps that are no declaration.
var commentWords = []string{"SolveSpace", "TeleSAPR", "TrueType", "OpenType", "OpenCASCADE", "FreeCAD", "PostScript"}

// TestAPICommentsNameRealCalls checks that the names the comments of
// package api mention, which help shows, are of declarations that
// exist: a call renamed leaves its old name in the comments of others
// (SetDrawing for Drawing.Set, AddDrawingDimension for AddDimension).
func TestAPICommentsNameRealCalls(t *testing.T) {
	t.Parallel()
	docs := symbols.Docs()
	known := map[string]bool{}
	for _, p := range docs {
		for name, td := range p.Types {
			known[name] = true
			for m := range td.Methods {
				known[m] = true
			}
			for _, f := range td.Fields {
				known[f.Name] = true
			}
		}
		for name := range p.Funcs {
			known[name] = true
		}
		for name := range p.Values {
			known[name] = true
		}
	}
	api := docs[apiPath]
	// has reports whether a struct of package api has a method or a field
	// called name, and whether the type is one to check.
	has := func(typ, name string) (found, checked bool) {
		td := api.Types[typ]
		if td == nil || td.Kind != "struct" {
			return false, false
		}
		_, ok := td.Methods[name]
		_, field := fieldOf(td, name)
		return ok || field, true
	}
	doc := api.Types["Doc"]

	var bad []string
	check := func(where, text string) {
		for _, m := range commentCamel.FindAllStringSubmatch(text, -1) {
			if !known[m[1]] && !slices.Contains(commentWords, m[1]) {
				bad = append(bad, where+": "+m[1])
			}
		}
		for _, m := range commentDotted.FindAllStringSubmatch(text, -1) {
			typ := m[2]
			if f, ok := fieldOf(doc, typ); ok && m[1] != "" {
				typ = f.Type
			}
			found, checked := has(typ, m[3])
			menuFound, menuChecked := has(typ+"Menu", m[3])
			if (checked || menuChecked) && !found && !menuFound {
				bad = append(bad, where+": "+m[1]+m[2]+"."+m[3])
			}
		}
	}
	check("package api", api.Text)
	for name, td := range api.Types {
		check(name, td.Text)
		for m, d := range td.Methods {
			check(name+"."+m, d.Text)
		}
		for _, f := range td.Fields {
			check(name+"."+f.Name, f.Text)
		}
	}
	for name, d := range api.Funcs {
		check(name, d.Text)
	}
	for name, d := range api.Values {
		check(name, d.Text)
	}
	slices.Sort(bad)
	for _, b := range bad {
		t.Errorf("a comment names what doesn't exist: %s", b)
	}
}
