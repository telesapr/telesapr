package agent

import (
	"fmt"
	"reflect"
	"regexp"
	"slices"

	"github.com/telesapr/telesapr/pkg/script/api"
	"github.com/telesapr/telesapr/pkg/script/symbols"
)

// The ways the hand-written texts name a call of package api.
var (
	// doc.Features.Extrude, doc.Params.Set
	callDocField = regexp.MustCompile(`\bdoc\.([A-Z]\w*)\.([A-Z]\w*)`)
	// doc.Describe, doc.Group; a field of Doc is not a method
	callDoc = regexp.MustCompile(`\bdoc\.([A-Z]\w*)\b`)
	// g.Analyze, g.AsFillet().SetRadius, g.Solid().FaceAt
	callGroup = regexp.MustCompile(`\bg\.([A-Z]\w*)(?:\(\)\.([A-Z]\w*))?`)
	// a.Volume, with a := g.Analyze()
	callAnalysis = regexp.MustCompile(`\ba\.([A-Z]\w*)\(`)
	// Features.Extrude, Group.SetCombine
	callTyped = regexp.MustCompile(`"([A-Z]\w*)\.([A-Z]\w*)"`)
	// Features > Fillet Edges
	callMenu = regexp.MustCompile(`\b[A-Z][a-z]+(?: > [A-Z]\w*(?: (?:and|[A-Z]\w*))*(?: \(\.\w+\))?)+`)
)

// UnknownCalls are the calls of package api that text names and that
// do not exist, in the forms the primer and the tool descriptions use:
// doc.Features.Extrude, doc.Describe, g.Analyze(), g.Solid().FaceAt,
// a.Volume() (a being a group's analysis), "Group.SetCombine" and menu
// entries as "Features > Fillet Edges". It is for a test that keeps
// the hand-written texts true to the interface.
func UnknownCalls(text string) []string {
	p := symbols.Docs()[apiPath]
	if p == nil {
		return []string{"(no documentation of " + apiPath + ")"}
	}
	hasMethod := func(typ, method string) bool {
		td := p.Types[typ]
		if td == nil {
			return false
		}
		_, ok := td.Methods[method]
		return ok
	}
	docType := reflect.TypeFor[api.Doc]()
	fieldType := func(name string) string {
		if f, ok := docType.FieldByName(name); ok && f.IsExported() {
			return f.Type.Name()
		}
		return ""
	}
	views := map[string]string{}
	for _, v := range groupViews() {
		views[v[0]] = v[1]
	}
	views["Solid"] = "Solid"
	analysis, _ := reflect.TypeFor[api.Group]().MethodByName("Analyze")

	var bad []string
	check := func(name string, ok bool) {
		if !ok && !slices.Contains(bad, name) {
			bad = append(bad, name)
		}
	}
	for _, m := range callDocField.FindAllStringSubmatch(text, -1) {
		t := fieldType(m[1])
		check(m[0], t != "" && hasMethod(t, m[2]))
	}
	for _, m := range callDoc.FindAllStringSubmatch(text, -1) {
		if fieldType(m[1]) == "" {
			check(m[0], hasMethod("Doc", m[1]))
		}
	}
	for _, m := range callGroup.FindAllStringSubmatch(text, -1) {
		check("g."+m[1], hasMethod("Group", m[1]))
		if m[2] != "" {
			t := views[m[1]]
			check(fmt.Sprintf("g.%s().%s", m[1], m[2]), t != "" && hasMethod(t, m[2]))
		}
	}
	for _, m := range callAnalysis.FindAllStringSubmatch(text, -1) {
		check("a."+m[1], hasMethod(analysis.Type.Out(0).Name(), m[1]))
	}
	for _, m := range callTyped.FindAllStringSubmatch(text, -1) {
		t := m[1]
		if ft := fieldType(t); ft != "" {
			t = ft
		}
		// Not every "Name.Name" is a call: a package's function, as
		// "math.Hypot", is lower case and does not match.
		check(m[0], hasMethod(t, m[2]))
	}
	for _, m := range callMenu.FindAllString(text, -1) {
		_, _, _, ok := api.MenuEntryCalls(m)
		check(m, ok)
	}
	return bad
}
