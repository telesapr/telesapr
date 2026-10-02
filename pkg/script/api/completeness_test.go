package api_test

// The script API must reach everything the interface makes
// (ROADMAP-JUMP.md, "Definition of done"): these tests fail when a
// group type or a creating menu command has no API method, or when an
// exported API name has no doc comment, which help() shows. That every
// example runs is tested where spacesolve-cli run is, in
// cmd/spacesolve-cli.

import (
	"go/ast"
	"go/doc"
	"go/parser"
	"go/token"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/spacesolve/spacesolve/pkg/script/api"
	"github.com/spacesolve/spacesolve/pkg/ui"
)

// creatingMenus are the menus whose entries make or change the model.
// Every entry in them must be in menuMethods.
var creatingMenus = []string{"New Group", "Sketch", "Constrain"}

// menuMethods says, for each entry of creatingMenus, the API method a
// script uses instead ("Type.Method"; several separated by spaces),
// or, after "-", why the entry needs none.
var menuMethods = map[string]string{
	"New Group/Sketch In 3d":              "Doc.Sketch3D",
	"New Group/Sketch In New Workplane":   "Doc.SketchIn Doc.SketchAt",
	"New Group/Step Translating":          "Doc.Translate",
	"New Group/Step Rotating":             "Doc.Rotate",
	"New Group/Extrude":                   "Doc.Extrude",
	"New Group/Helix":                     "Doc.Helix",
	"New Group/Lathe":                     "Doc.Lathe",
	"New Group/Revolve":                   "Doc.Revolve",
	"New Group/Link / Assemble...":        "Doc.Link",
	"New Group/Link Recent":               "- links a file from the recent list; Doc.Link takes any file",
	"New Group/Screw Thread...":           "Doc.Thread",
	"New Group/Sweep Along Path":          "Doc.Sweep",
	"New Group/Mirror":                    "Doc.Mirror",
	"New Group/Chamfer Edges":             "Doc.Chamfer",
	"New Group/Fillet Edges":              "Doc.Fillet",
	"Sketch/In Workplane":                 "Doc.SetWorkplane",
	"Sketch/Anywhere In 3d":               "Doc.Free3D",
	"Sketch/Datum Point":                  "Doc.Point",
	"Sketch/Workplane":                    "Doc.NewWorkplane",
	"Sketch/Line Segment":                 "Doc.Line Doc.Polyline",
	"Sketch/Construction Line Segment":    "Doc.Line Entity.SetConstruction",
	"Sketch/Rectangle":                    "Doc.Rect",
	"Sketch/Circle":                       "Doc.Circle",
	"Sketch/Arc of a Circle":              "Doc.Arc",
	"Sketch/Bezier Cubic Spline":          "Doc.Cubic",
	"Sketch/Text in TrueType Font":        "Doc.Text",
	"Sketch/Image":                        "Doc.Image",
	"Sketch/Toggle Construction":          "Entity.SetConstruction Request.SetConstruction",
	"Sketch/Tangent Arc at Point":         "Doc.TangentArc",
	"Sketch/Split Curves at Intersection": "Doc.Split",
	"Constrain/Distance / Diameter":       "Doc.Distance Doc.Length Doc.Diameter Doc.Radius",
	"Constrain/Reference Dimension":       "Constraint.SetReference",
	"Constrain/Angle / Equal Angle":       "Doc.Angle Doc.Constrain",
	"Constrain/Reference Angle":           "Constraint.SetReference",
	"Constrain/Other Supplementary Angle": "Constraint.SetOther",
	"Constrain/Toggle Reference Dim":      "Constraint.SetReference",
	"Constrain/Horizontal":                "Doc.Horizontal",
	"Constrain/Vertical":                  "Doc.Vertical",
	"Constrain/On Point / Curve / Plane":  "Doc.On Doc.Coincident",
	"Constrain/Equal Length / Radius":     "Doc.Equal",
	"Constrain/Length / Arc Ratio":        "Doc.Constrain",
	"Constrain/Length / Arc Difference":   "Doc.Constrain",
	"Constrain/At Midpoint":               "Doc.Midpoint",
	"Constrain/Symmetric":                 "Doc.Symmetric",
	"Constrain/Parallel / Tangent":        "Doc.Parallel Doc.Tangent",
	"Constrain/Perpendicular":             "Doc.Perpendicular",
	"Constrain/Same Orientation":          "Doc.Constrain",
	"Constrain/Lock Point Where Dragged":  "Doc.Lock",
	"Constrain/Comment":                   "Doc.Comment",
}

// apiTypes are the API's types by name, for menuMethods.
var apiTypes = map[string]reflect.Type{
	"Doc":        reflect.TypeOf(&api.Doc{}),
	"Group":      reflect.TypeOf(api.Group{}),
	"Entity":     reflect.TypeOf(api.Entity{}),
	"Request":    reflect.TypeOf(api.Request{}),
	"Constraint": reflect.TypeOf(api.Constraint{}),
}

// hasMethod reports whether "Type.Method" exists.
func hasMethod(name string) bool {
	typ, method, ok := strings.Cut(name, ".")
	if !ok {
		return false
	}
	t, ok := apiTypes[typ]
	if !ok {
		return false
	}
	_, ok = t.MethodByName(method)
	return ok
}

func TestEveryMenuCommandHasAMethod(t *testing.T) {
	seen := map[string]bool{}
	for _, item := range ui.MenuItems() {
		creating := false
		for _, m := range creatingMenus {
			creating = creating || item.Path[0] == m
		}
		if !creating {
			continue
		}
		key := strings.Join(item.Path, "/")
		seen[key] = true
		methods, ok := menuMethods[key]
		switch {
		case !ok:
			t.Errorf("the menu entry %q has no script API method; add one, or say in menuMethods why it needs none", key)
		case strings.HasPrefix(methods, "- "):
		default:
			for _, m := range strings.Fields(methods) {
				if !hasMethod(m) {
					t.Errorf("the menu entry %q names %s, which doesn't exist", key, m)
				}
			}
		}
	}
	for key := range menuMethods {
		if !seen[key] {
			t.Errorf("menuMethods names %q, which is no menu entry any more", key)
		}
	}
}

// groupMethods says which method makes each group type.
var groupMethods = map[string]string{
	"GroupDrawing3D":        "Doc.Sketch3D",
	"GroupDrawingWorkplane": "Doc.SketchIn",
	"GroupExtrude":          "Doc.Extrude",
	"GroupLathe":            "Doc.Lathe",
	"GroupRevolve":          "Doc.Revolve",
	"GroupHelix":            "Doc.Helix",
	"GroupMirror":           "Doc.Mirror",
	"GroupChamfer":          "Doc.Chamfer",
	"GroupFillet":           "Doc.Fillet",
	"GroupSweep":            "Doc.Sweep",
	"GroupRotate":           "Doc.Rotate",
	"GroupTranslate":        "Doc.Translate",
	"GroupLinked":           "Doc.Link",
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
	sort.Strings(names)
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
	p, err := doc.NewFromFiles(fset, files, "github.com/spacesolve/spacesolve/pkg/script/api")
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
