package api_test

// The script API must reach everything the interface makes
// (doc/done-audit.md, "Definition of done"): these tests fail when a
// group type or a menu command has no API method and no reason to have
// none, or when an exported API name has no doc comment, which help()
// shows. That every example runs is tested where telesapr-cli run is,
// in cmd/telesapr-cli.

import (
	"go/ast"
	"go/doc"
	"go/parser"
	"go/token"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/script/api"
	"github.com/telesapr/telesapr/pkg/ui"
)

// apiType is the type of package api that help() names name: a type
// of the package, or a field of Doc, which holds a menu's calls.
func apiType(name string) (reflect.Type, bool) {
	if f, ok := reflect.TypeFor[api.Doc]().FieldByName(name); ok && f.IsExported() {
		return f.Type, true
	}
	t, ok := apiTypes[name]
	return t, ok
}

// apiTypes are the API's types by name, for MenuCalls.
var apiTypes = map[string]reflect.Type{
	"Doc":           reflect.TypeFor[*api.Doc](),
	"Group":         reflect.TypeFor[api.Group](),
	"Entity":        reflect.TypeFor[api.Entity](),
	"Request":       reflect.TypeFor[api.Request](),
	"Constraint":    reflect.TypeFor[api.Constraint](),
	"GroupHistory":  reflect.TypeFor[api.GroupHistory](),
	"GroupAnalysis": reflect.TypeFor[api.GroupAnalysis](),
	"ExtrudeGroup":  reflect.TypeFor[api.ExtrudeGroup](),
	"LoftGroup":     reflect.TypeFor[api.LoftGroup](),
	"RevolveGroup":  reflect.TypeFor[api.RevolveGroup](),
	"ThreadGroup":   reflect.TypeFor[api.ThreadGroup](),
	"LinkGroup":     reflect.TypeFor[api.LinkGroup](),
	"Style":         reflect.TypeFor[api.Style](),
	"Param":         reflect.TypeFor[api.Param](),
	"RepeatGroup":   reflect.TypeFor[api.RepeatGroup](),
	"SweepGroup":    reflect.TypeFor[api.SweepGroup](),
	"MirrorGroup":   reflect.TypeFor[api.MirrorGroup](),
	"ChamferGroup":  reflect.TypeFor[api.ChamferGroup](),
	"FilletGroup":   reflect.TypeFor[api.FilletGroup](),
	"ShellGroup":    reflect.TypeFor[api.ShellGroup](),
	"Solid":         reflect.TypeFor[api.Solid](),
	"SolidFace":     reflect.TypeFor[api.SolidFace](),
	"SolidEdge":     reflect.TypeFor[api.SolidEdge](),
	"SolidVertex":   reflect.TypeFor[api.SolidVertex](),
	"ExportOptions": reflect.TypeFor[api.ExportOptions](),
}

// hasMethod reports whether "Type.Method" exists.
func hasMethod(name string) bool {
	typ, method, ok := strings.Cut(name, ".")
	if !ok {
		return false
	}
	t, ok := apiType(typ)
	if !ok {
		return false
	}
	_, ok = t.MethodByName(method)
	return ok
}

// noMenuEntry says why a call of a menu's namespace is named by no
// entry of the menu: what reads the drawing, which the window shows
// rather than does.
var noMenuEntry = map[string]string{
	"File.Unsaved":                 "the window's title says it",
	"File.ForgetChanges":           "the window asks instead",
	"Edit.Selection":               "the window shows the selection",
	"Edit.SelectedConstraints":     "the window shows the selection",
	"Edit.SelectedEdges":           "the window shows the selection",
	"Edit.SelectedFaces":           "the window shows the selection",
	"Edit.SelectEdges":             "picking edges with the mouse",
	"Edit.SelectFaces":             "picking faces with the mouse",
	"Edit.SelectFaceEdges":         "Select Face Edges in the context menu of a face, which is no entry of the main menu",
	"Edit.ChordTolerance":          "Edit > Preferences shows it",
	"Edit.MaxSegments":             "Edit > Preferences shows it",
	"Edit.CheckClosedContour":      "Edit > Preferences shows it",
	"Params.Get":                   "Edit > Parameters shows it",
	"Params.Rename":                "Edit > Parameters edits it",
	"Params.Delete":                "Edit > Parameters edits it",
	"Params.Users":                 "Edit > Parameters lists what uses each parameter",
	"Params.Errors":                "Edit > Parameters shows it",
	"Params.AddConfiguration":      "Edit > Parameters edits it",
	"Params.ActivateConfiguration": "Edit > Parameters edits it",
	"Params.ActiveConfiguration":   "Edit > Parameters shows it",
	"Params.Configurations":        "Edit > Parameters shows it",
	"Params.RenameConfiguration":   "Edit > Parameters edits it",
	"Params.DeleteConfiguration":   "Edit > Parameters edits it",
	"View.Section":                 "the window shows it",
	"View.Units":                   "the window shows it",
	"View.MmPerUnit":               "the window shows it",
	"View.SetFaceAppearance":       "the Appearance entry of a face's context menu",
	"View.FaceAppearance":          "a face is shaded in the appearance it has",
	"Drawing.Shown":                "the window shows it",
	"Drawing.Dimensions":           "the drawing sheet shows them",
	"Drawing.AddDimension":         "dimensioning a view of the drawing sheet with the mouse",
	"Drawing.AddFaceDimension":     "dimensioning a view of the drawing sheet with the mouse",
	"Drawing.AddEdgeFaceDistance":  "dimensioning a view of the drawing sheet with the mouse",
	"Sketch.ActiveWorkplane":       "the window shows it",
	"Sketch.UV":                    "a point of the workplane, where the mouse puts one",
	"Constrain.Auto":               "the window constrains as a sketch is drawn",
	"Sketch.Fonts":                 "a query: the fonts the text tool and the text window offer",
	"Constrain.CommentOn":          "Constrain > Comment with a point selected, which Constrain.Comment (at a place) does not",
	"Constrain.Raw":                "an escape hatch for a constraint the window makes some other way",
	"Features.FilletSolid":         "Features > Fillet Edges on edges of the solid",
	"Features.ChamferSolid":        "Features > Chamfer Edges on edges of the solid",
	"Features.ShellSolid":          "Features > Shell Solid on faces of the solid",
	"Features.SweepEdges":          "Features > Sweep Along Path on edges of the solid",
	"Features.LoftThrough":         "Features > Loft To Selected Profile with several selected",
	"Assembly.LinkStep":            "Assembly > Link / Assemble with a STEP file",
}

func TestEveryMenuCallHasAnEntry(t *testing.T) {
	named := map[string]bool{}
	for _, v := range api.MenuCalls {
		for c := range strings.FieldsSeq(v) {
			named[c] = true
		}
	}
	doc := reflect.TypeFor[api.Doc]()
	for f := range doc.Fields() {
		if !f.IsExported() {
			continue
		}
		for m := range f.Type.Methods() {
			call := f.Name + "." + m.Name
			if named[call] {
				if _, ok := noMenuEntry[call]; ok {
					t.Errorf("%s is in MenuCalls and in noMenuEntry", call)
				}
				continue
			}
			if _, ok := noMenuEntry[call]; !ok {
				t.Errorf("no menu entry names %s; add it to MenuCalls, or say in noMenuEntry why there is none", call)
			}
		}
	}
	for call := range noMenuEntry {
		if !hasMethod(call) {
			t.Errorf("noMenuEntry names %s, which doesn't exist", call)
		}
	}
}

func TestEveryMenuCommandHasAMethod(t *testing.T) {
	seen := map[string]bool{}
	for _, item := range ui.MenuItems() {
		key := strings.Join(item.Path, "/")
		seen[key] = true
		methods, ok := api.MenuCalls[key]
		switch {
		case !ok:
			t.Errorf("the menu entry %q has no script API method; add one, or say in api.MenuCalls why it needs none", key)
		case strings.HasPrefix(methods, "- "):
		default:
			for m := range strings.FieldsSeq(methods) {
				if !hasMethod(m) {
					t.Errorf("the menu entry %q names %s, which doesn't exist", key, m)
				}
			}
		}
	}
	for key := range api.MenuCalls {
		if !seen[key] {
			t.Errorf("MenuCalls names %q, which is no menu entry any more", key)
		}
	}
}

// groupMethods says which method makes each group type.
var groupMethods = map[string]string{
	"GroupDrawing3D":        "Features.Sketch3D",
	"GroupDrawingWorkplane": "Features.SketchIn",
	"GroupExtrude":          "Features.Extrude",
	"GroupLathe":            "Features.Lathe",
	"GroupRevolve":          "Features.Revolve",
	"GroupHelix":            "Features.Helix",
	"GroupMirror":           "Features.Mirror",
	"GroupChamfer":          "Features.Chamfer",
	"GroupFillet":           "Features.Fillet",
	"GroupLoft":             "Features.Loft",
	"GroupShell":            "Features.Shell",
	"GroupPathPattern":      "Features.PatternAlongPath",
	"GroupSweep":            "Features.Sweep",
	"GroupRotate":           "Features.Rotate",
	"GroupTranslate":        "Features.Translate",
	"GroupLinked":           "Assembly.Link",
}

// constsOfType lists the constants of a named type declared in a
// package's directory, following a const block's implicit repetition.
func constsOfType(t *testing.T, dir, typ string) []string {
	fset := token.NewFileSet()
	files, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range f.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok || gd.Tok != token.CONST {
				continue
			}
			current := ""
			for _, spec := range gd.Specs {
				vs := spec.(*ast.ValueSpec)
				if vs.Type != nil {
					current = ""
					if id, ok := vs.Type.(*ast.Ident); ok {
						current = id.Name
					}
				} else if len(vs.Values) > 0 {
					current = ""
				}
				if current != typ {
					continue
				}
				for _, n := range vs.Names {
					if n.Name != "_" {
						names = append(names, n.Name)
					}
				}
			}
		}
	}
	slices.Sort(names)
	return names
}

func TestEveryGroupTypeHasAMethod(t *testing.T) {
	types := constsOfType(t, filepath.Join("..", "..", "sketch"), "GroupType")
	if len(types) < len(groupMethods) {
		t.Fatalf("found only the group types %v", types)
	}
	for _, name := range types {
		m, ok := groupMethods[name]
		if !ok {
			t.Errorf("the group type %s has no script API method that makes it; add one to groupMethods", name)
			continue
		}
		if !hasMethod(m) {
			t.Errorf("the group type %s is made by %s, which doesn't exist", name, m)
		}
	}
}

// TestEveryExportedNameHasADoc parses the API package as help() and
// docs.json read it, and requires a doc comment on every exported
// type, function, method, constant and variable.
func TestEveryExportedNameHasADoc(t *testing.T) {
	fset := token.NewFileSet()
	paths, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	var files []*ast.File
	for _, path := range paths {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, f)
	}
	p, err := doc.NewFromFiles(fset, files, "github.com/telesapr/telesapr/pkg/script/api")
	if err != nil {
		t.Fatal(err)
	}
	missing := func(what string, text string) {
		if strings.TrimSpace(text) == "" {
			t.Errorf("%s has no doc comment", what)
		}
	}
	values := func(vs []*doc.Value) {
		for _, v := range vs {
			if v.Doc != "" {
				continue
			}
			// A name in a group may have its own line comment.
			for _, spec := range v.Decl.Specs {
				vs := spec.(*ast.ValueSpec)
				for _, n := range vs.Names {
					if n.IsExported() && vs.Doc == nil && vs.Comment == nil {
						t.Errorf("%s has no doc comment", n.Name)
					}
				}
			}
		}
	}
	values(p.Consts)
	values(p.Vars)
	for _, f := range p.Funcs {
		missing(f.Name, f.Doc)
	}
	for _, typ := range p.Types {
		missing(typ.Name, typ.Doc)
		values(typ.Consts)
		values(typ.Vars)
		for _, f := range typ.Funcs {
			missing(f.Name, f.Doc)
		}
		for _, m := range typ.Methods {
			missing(typ.Name+"."+m.Name, m.Doc)
		}
	}
}
