// Command gendoc writes docs.json, what help() in a script console
// says about the packages a script can import: their types, functions
// and methods as declared, with their comments. It is run by go
// generate in pkg/script/symbols:
//
//	gendoc [-full package,...] package...
//
// The comments of the packages after -full are kept whole, those of
// the others down to their first sentence.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"go/ast"
	"go/build"
	"go/doc"
	"go/parser"
	"go/printer"
	"go/token"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/telesapr/telesapr/pkg/script/symbols"
)

func main() {
	log.SetFlags(0)
	full := flag.String("full", "", "packages whose comments are kept whole")
	out := flag.String("o", "docs.json", "the file to write")
	flag.Parse()
	whole := map[string]bool{}
	for p := range strings.SplitSeq(*full, ",") {
		whole[p] = true
	}

	docs := map[string]*symbols.PackageDoc{}
	for _, path := range flag.Args() {
		pkg, err := build.Import(path, ".", 0)
		if err != nil {
			log.Fatalf("gendoc: %v", err)
		}
		docs[path] = describe(pkg, whole[path])
	}
	data, err := json.MarshalIndent(docs, "", "\t")
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(*out, append(data, '\n'), 0o644); err != nil {
		log.Fatal(err)
	}
}

// describe reads a package's sources for what it exports.
func describe(pkg *build.Package, whole bool) *symbols.PackageDoc {
	fset := token.NewFileSet()
	var files []*ast.File
	for _, name := range pkg.GoFiles {
		f, err := parser.ParseFile(fset, filepath.Join(pkg.Dir, name), nil, parser.ParseComments)
		if err != nil {
			log.Fatalf("gendoc: %v", err)
		}
		files = append(files, f)
	}
	d, err := doc.NewFromFiles(fset, files, pkg.ImportPath)
	if err != nil {
		log.Fatalf("gendoc: %s: %v", pkg.ImportPath, err)
	}

	text := func(s string) string {
		if whole {
			return strings.TrimSpace(s)
		}
		return d.Synopsis(s)
	}
	function := func(f *doc.Func) symbols.Doc {
		decl := *f.Decl
		decl.Doc, decl.Body = nil, nil
		var b bytes.Buffer
		if err := printer.Fprint(&b, fset, &decl); err != nil {
			log.Fatalf("gendoc: %s: %v", f.Name, err)
		}
		return symbols.Doc{Decl: strings.Join(strings.Fields(b.String()), " "), Text: text(f.Doc)}
	}
	values := func(into map[string]symbols.Doc, groups []*doc.Value, kind string) (names []string) {
		for _, v := range groups {
			for _, name := range v.Names {
				if !ast.IsExported(name) {
					continue
				}
				into[name] = symbols.Doc{Decl: kind + " " + name, Text: text(v.Doc)}
				names = append(names, name)
			}
		}
		return names
	}

	p := &symbols.PackageDoc{
		Name:   d.Name,
		Text:   text(d.Doc),
		Types:  map[string]*symbols.TypeDoc{},
		Funcs:  map[string]symbols.Doc{},
		Values: map[string]symbols.Doc{},
	}
	values(p.Values, d.Consts, "const")
	values(p.Values, d.Vars, "var")
	for _, f := range d.Funcs {
		p.Funcs[f.Name] = function(f)
	}
	for _, t := range d.Types {
		td := &symbols.TypeDoc{Text: text(t.Doc), Kind: kindOf(t), Methods: map[string]symbols.Doc{}}
		for _, m := range t.Methods {
			td.Methods[m.Name] = function(m)
		}
		// What makes one, and the constants of the type.
		for _, f := range t.Funcs {
			p.Funcs[f.Name] = function(f)
			td.Makers = append(td.Makers, f.Name)
		}
		td.Values = append(values(p.Values, t.Consts, "const"), values(p.Values, t.Vars, "var")...)
		sort.Strings(td.Makers)
		p.Types[t.Name] = td
	}
	return p
}

// kindOf says what a type is declared as: "struct", "interface", or
// the type it is made from.
func kindOf(t *doc.Type) string {
	for _, spec := range t.Decl.Specs {
		ts, ok := spec.(*ast.TypeSpec)
		if !ok || ts.Name.Name != t.Name {
			continue
		}
		switch x := ts.Type.(type) {
		case *ast.StructType:
			return "struct"
		case *ast.InterfaceType:
			return "interface"
		case *ast.Ident:
			return x.Name
		case *ast.FuncType:
			return "func"
		default:
			var b bytes.Buffer
			_ = printer.Fprint(&b, token.NewFileSet(), ts.Type)
			return strings.Join(strings.Fields(b.String()), " ")
		}
	}
	return ""
}
