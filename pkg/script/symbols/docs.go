package symbols

import (
	_ "embed"
	"encoding/json"
	"sync"
)

// Doc is something a package declares, as help() shows it.
type Doc struct {
	// Decl is its declaration: "func (g Group) Bounds() (min, max Vec)".
	Decl string `json:"decl"`
	// Text is its comment, or the first sentence of it.
	Text string `json:"text,omitempty"`
}

// TypeDoc is a type of a package.
type TypeDoc struct {
	Text string `json:"text,omitempty"`
	// Kind is what the type is declared as: "struct", "int", ...
	Kind    string         `json:"kind,omitempty"`
	Methods map[string]Doc `json:"methods,omitempty"`
	// Makers are the package's functions that return the type, and
	// Values its constants and variables of the type.
	Makers []string `json:"makers,omitempty"`
	Values []string `json:"values,omitempty"`
}

// PackageDoc is what a package exports.
type PackageDoc struct {
	Name   string              `json:"name"`
	Text   string              `json:"text,omitempty"`
	Types  map[string]*TypeDoc `json:"types,omitempty"`
	Funcs  map[string]Doc      `json:"funcs,omitempty"`
	Values map[string]Doc      `json:"values,omitempty"`
}

//go:embed docs.json
var docsJSON []byte

var docs = sync.OnceValue(func() map[string]*PackageDoc {
	var d map[string]*PackageDoc
	if err := json.Unmarshal(docsJSON, &d); err != nil {
		panic("symbols: docs.json: " + err.Error())
	}
	return d
})

// Docs is what the packages a script can import declare, by import
// path; it is generated from their sources, see gendoc.
func Docs() map[string]*PackageDoc { return docs() }
