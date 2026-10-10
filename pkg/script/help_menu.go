package script

import (
	"fmt"
	"io"
	"reflect"
	"strings"
	"sync"

	"github.com/telesapr/telesapr/pkg/script/api"
	"github.com/telesapr/telesapr/pkg/script/symbols"
)

// This file ties help to the window's menus: the fields of api.Doc
// hold one menu's calls each, so help("Sketch.Line") is the method
// Line of the type of doc.Sketch, and help("Features > Fillet Edges")
// names what a script calls in place of that entry.

// menuFields maps a field of api.Doc ("Sketch") to the name of its
// type ("SketchMenu"), and the type's name back to the field.
var menuFields = sync.OnceValue(func() map[string]string {
	m := map[string]string{}
	for f := range reflect.TypeFor[api.Doc]().Fields() {
		if f.IsExported() {
			m[f.Name] = f.Type.Name()
			m[f.Type.Name()] = f.Name
		}
	}
	return m
})

// apiTypeName is the type of package api that help names name: the
// type of a field of Doc for the field's name, else name itself.
func apiTypeName(name string) string {
	if t, ok := menuFields()[name]; ok && !strings.HasSuffix(name, "Menu") {
		return t
	}
	return name
}

// isMenuPath reports whether name is written as a menu entry is,
// "Features > Fillet Edges", rather than as a name of Go.
func isMenuPath(name string) bool {
	if strings.ContainsAny(name, ">›") {
		return true
	}
	if strings.Contains(name, " ") {
		return true
	}
	_, _, _, ok := api.MenuEntryCalls(name)
	return ok && strings.Contains(name, "/") && len(packagesNamed(name)) == 0
}

// helpMenuEntry writes what a script calls in place of a menu entry,
// and each call in full. It is false if no entry has that path.
func helpMenuEntry(w io.Writer, name string) bool {
	path, calls, why, ok := api.MenuEntryCalls(name)
	if !ok {
		return false
	}
	entry := strings.ReplaceAll(path, "/", " > ")
	if len(calls) == 0 {
		fmt.Fprintf(w, "%s has no call of its own: %s.\n", entry, why)
		return true
	}
	fmt.Fprintf(w, "%s is, in a script:\n", entry)
	for _, c := range calls {
		fmt.Fprintln(w)
		if !helpName(w, c) {
			fmt.Fprintln(w, c)
		}
	}
	return true
}

// menuLine writes the menu entries a method of package api stands
// for, if it stands for any.
func menuLine(w io.Writer, typeName, method string) {
	owner := typeName
	if field, ok := menuFields()[typeName]; ok && strings.HasSuffix(typeName, "Menu") {
		owner = field
	}
	if entries := api.MenuEntriesOf(owner + "." + method); len(entries) > 0 {
		fmt.Fprintf(w, "\nMenu: %s\n", strings.Join(entries, "; "))
	}
}

// symbolType is the reflect.Type of a type a script can import, or nil.
func symbolType(importPath, pkgName, name string) reflect.Type {
	v, ok := symbols.Symbols[importPath+"/"+pkgName][name]
	if !ok || v.Kind() != reflect.Pointer {
		return nil
	}
	return v.Type().Elem()
}
