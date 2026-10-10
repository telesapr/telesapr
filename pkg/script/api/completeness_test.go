package api_test

// The script API must reach everything the interface makes
// (docs/done-audit.md, "Definition of done"): these tests fail when a
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

// menuMethods says, for each entry of the main menu, the API method a
// script uses instead ("Type.Method"; several separated by spaces),
// or, after "-", why the entry needs none: most of those are the
// window's own, its view and its panels, which a script has none of.
var menuMethods = map[string]string{
	"File/New":                                           "- a script works on the drawing it is given; Doc.Load replaces it",
	"File/Open...":                                       "Doc.Load",
	"File/Open Recent":                                   "- the window's list of files; Doc.Load takes any file",
	"File/Save":                                          "Doc.Save",
	"File/Save As...":                                    "Doc.Save",
	"File/Import...":                                     "- a SolveSpace file is converted by telesapr-cli convert, then Doc.Load",
	"File/Export/Image...":                               "Doc.Render",
	"File/Export/2d View...":                             "Doc.Export Doc.ExportAs",
	"File/Export/2d Section...":                          "Doc.Export Doc.ExportAs",
	"File/Export/3d Wireframe...":                        "Doc.Export Doc.ExportAs",
	"File/Export/Triangle Mesh...":                       "Doc.Export Doc.ExportAs",
	"File/Export/Surfaces...":                            "Doc.Export Doc.ExportAs",
	"File/Export/Drawing...":                             "Doc.ExportDrawing",
	"File/Exit":                                          "- the window's",
	"Edit/Undo":                                          "Doc.Undo",
	"Edit/Redo":                                          "Doc.Redo",
	"Edit/Regenerate All":                                "Doc.Regen",
	"Edit/Cut":                                           "Doc.Copy Entity.Delete",
	"Edit/Copy":                                          "Doc.Copy",
	"Edit/Paste":                                         "Doc.Paste",
	"Edit/Paste Transformed...":                          "Doc.Paste",
	"Edit/Delete":                                        "Entity.Delete Constraint.Delete Group.Delete",
	"Edit/Select Edge Chain":                             "- a selection made with the mouse; a script names the entities, Doc.Select",
	"Edit/Select All":                                    "Doc.Select",
	"Edit/Unselect All":                                  "Doc.Select",
	"Edit/Snap Selection to Grid":                        "- the grid is the view's; Entity.MoveTo puts a point where it is asked",
	"Edit/Rotate Imported 90°":                           "- turns a linked part a quarter in the view; Doc.Link places it",
	"Edit/Find Command...":                               "Doc.RunCommand",
	"Edit/Parameters...":                                 "Doc.Parameters Doc.SetParameter",
	"Edit/Line Styles...":                                "- line styles are not scripted yet",
	"Edit/View Projection...":                            "- the window's view; Doc.Render takes its own",
	"Edit/Preferences...":                                "Doc.SetChordTolerance Doc.SetUnits",
	"View/Zoom In":                                       "- the window's view",
	"View/Zoom Out":                                      "- the window's view",
	"View/Zoom To Fit":                                   "- the window's view; Doc.Render fits the model",
	"View/Align View to Workplane":                       "- the window's view",
	"View/Nearest Ortho View":                            "- the window's view",
	"View/Nearest Isometric View":                        "- the window's view",
	"View/Center View At Point":                          "- the window's view",
	"View/Show/Workplanes of Inactive Groups":            "- what the window shows",
	"View/Show/Normals":                                  "- what the window shows",
	"View/Show/Points":                                   "- what the window shows",
	"View/Show/Construction Entities":                    "- what the window shows",
	"View/Show/Constraints and Dimensions":               "- what the window shows",
	"View/Show/Dimensions Only":                          "- what the window shows",
	"View/Show/No Constraints":                           "- what the window shows",
	"View/Show/Shaded Solid":                             "- what the window shows",
	"View/Show/Selectable Faces":                         "- what the window shows",
	"View/Show/Solid Edges":                              "- what the window shows",
	"View/Show/Solid Outline":                            "- what the window shows",
	"View/Show/Triangle Mesh":                            "- what the window shows",
	"View/Show/Hidden Lines Invisible":                   "- what the window shows",
	"View/Show/Hidden Lines Stippled":                    "- what the window shows",
	"View/Show/Hidden Lines Visible":                     "- what the window shows",
	"View/Show Snap Grid":                                "- what the window shows",
	"View/Darken Inactive Solids":                        "- what the window shows",
	"View/Use Perspective Projection":                    "- the window's view",
	"View/Show Exploded View":                            "- what the window shows",
	"View/Section View":                                  "Doc.SetSection Doc.ClearSection",
	"View/Show Drawing Sheet":                            "Doc.ShowDrawingSheet",
	"View/Drawing Sheet...":                              "Doc.SetDrawing Doc.Drawing",
	"View/Dimension Units/Dimensions in Millimeters":     "Doc.SetUnits",
	"View/Dimension Units/Dimensions in Meters":          "Doc.SetUnits",
	"View/Dimension Units/Dimensions in Inches":          "Doc.SetUnits",
	"View/Dimension Units/Dimensions in Feet and Inches": "Doc.SetUnits",
	"View/Show Toolbar":                                  "- the window's",
	"View/Show Property Browser":                         "- the window's",
	"View/Full Screen":                                   "- the window's",
	"Analyze/Measure Selection":                          "Doc.Measure",
	"Analyze/Measure Volume":                             "Group.Volume",
	"Analyze/Measure Area":                               "Group.Area Doc.FacesArea",
	"Analyze/Measure Perimeter":                          "Doc.CurveLength",
	"Analyze/Show Center of Mass":                        "Group.CenterOfMass",
	"Analyze/Check Solid":                                "Group.Check",
	"Analyze/Ray-Cast Check":                             "Group.RayCheck Group.Probe",
	"Analyze/Show Naked Edges":                           "Group.Check",
	"Analyze/Show Underconstrained Points":               "Entity.Free Group.Dof",
	"Analyze/Trace Point":                                "- draws a point's path in the window; Doc.MotionStudy reports positions",
	"Analyze/Stop Tracing...":                            "- draws a point's path in the window; Doc.MotionStudy reports positions",
	"Analyze/Step Dimension...":                          "Doc.MotionStudy",
	"Script/Show Console":                                "- the window's console",
	"Script/Run File...":                                 "- a script runs from telesapr-cli run or telesapr --script",
	"Script/Stop":                                        "- the window's console",
	"Script/Allow AI Connections":                        "- the window's setting; telesapr-cli mcp serves a drawing without one",
	"Help/User's Guide":                                  "- the window's",
	"Help/Go to GitHub":                                  "- the window's",
	"Help/About":                                         "- the window's",
	"Features/Sketch In 3d":                              "Doc.Sketch3D",
	"Features/Sketch In New Workplane":                   "Doc.SketchIn Doc.SketchAt",
	"Features/Sketch On Face":                            "Doc.SketchOnFace",
	"Features/Step Translating":                          "Doc.Translate",
	"Features/Step Rotating":                             "Doc.Rotate",
	"Features/Extrude":                                   "Doc.Extrude",
	"Features/Helix":                                     "Doc.Helix",
	"Features/Lathe":                                     "Doc.Lathe",
	"Features/Revolve":                                   "Doc.Revolve",
	"Assembly/Link / Assemble...":                        "Doc.Link",
	"Assembly/Link Recent":                               "- links a file from the recent list; Doc.Link takes any file",
	"Features/Screw Thread...":                           "Doc.Thread",
	"Features/Sweep Along Path":                          "Doc.Sweep",
	"Features/Mirror":                                    "Doc.Mirror",
	"Features/Chamfer Edges":                             "Doc.Chamfer",
	"Features/Fillet Edges":                              "Doc.Fillet",
	"Features/Loft To Selected Profile":                  "Doc.Loft",
	"Features/Shell Solid":                               "Doc.Shell",
	"Features/Pattern Along Path":                        "Doc.PatternAlongPath",
	"Features/Draft Selected Sides":                      "Group.SetDraftFaces",
	"Features/Guide Loft by Selected Curves":             "Group.SetGuides",
	"Features/Move Group Earlier":                        "Group.MoveBefore",
	"Features/Move Group Later":                          "Group.MoveBefore",
	"Features/Move Group to End":                         "Group.MoveToEnd Group.MoveToEndWithSources",
	"Features/Broken References...":                      "Doc.BrokenReferences Doc.ClearBrokenReferences",
	"Assembly/Show Interfering Parts":                    "Doc.Interferences",
	"Assembly/Bill of Materials":                         "Doc.BillOfMaterials Doc.ExportBOM",
	"Assembly/Motion Study...":                           "Doc.MotionStudy",
	"Sketch/In Workplane":                                "Doc.SetWorkplane",
	"Sketch/Anywhere In 3d":                              "Doc.Free3D",
	"Sketch/Datum Point":                                 "Doc.Point",
	"Sketch/Workplane":                                   "Doc.NewWorkplane",
	"Sketch/Line Segment":                                "Doc.Line Doc.Polyline",
	"Sketch/Construction Line Segment":                   "Doc.Line Entity.SetConstruction",
	"Sketch/Rectangle":                                   "Doc.Rect",
	"Sketch/Circle":                                      "Doc.Circle",
	"Sketch/Arc of a Circle":                             "Doc.Arc",
	"Sketch/Bezier Cubic Spline":                         "Doc.Cubic",
	"Sketch/Text in TrueType Font":                       "Doc.Text",
	"Sketch/Image":                                       "Doc.Image",
	"Sketch/Toggle Construction":                         "Entity.SetConstruction Request.SetConstruction",
	"Sketch/Tangent Arc at Point":                        "Doc.TangentArc",
	"Sketch/Split Curves at Intersection":                "Doc.Split",
	"Constrain/Distance / Diameter":                      "Doc.Distance Doc.Length Doc.Diameter Doc.Radius",
	"Constrain/Reference Dimension":                      "Constraint.SetReference",
	"Constrain/Minimum Distance":                         "Doc.MinDistance",
	"Constrain/Maximum Distance":                         "Doc.MaxDistance",
	"Constrain/Angle / Equal Angle":                      "Doc.Angle Doc.Constrain",
	"Constrain/Reference Angle":                          "Constraint.SetReference",
	"Constrain/Other Supplementary Angle":                "Constraint.SetOther",
	"Constrain/Toggle Reference Dim":                     "Constraint.SetReference",
	"Constrain/Horizontal":                               "Doc.Horizontal",
	"Assembly/Mate Concentric":                           "Doc.Concentric",
	"Assembly/Mate Flush":                                "Doc.Flush",
	"Assembly/Mate Hinge":                                "Doc.Hinge",
	"Constrain/Vertical":                                 "Doc.Vertical",
	"Constrain/On Point / Curve / Plane":                 "Doc.On Doc.Coincident",
	"Constrain/On Segment":                               "Doc.OnSegment",
	"Constrain/Equal Length / Radius":                    "Doc.Equal",
	"Constrain/Length / Arc Ratio":                       "Doc.Constrain",
	"Constrain/Length / Arc Difference":                  "Doc.Constrain",
	"Constrain/At Midpoint":                              "Doc.Midpoint",
	"Constrain/Symmetric":                                "Doc.Symmetric",
	"Constrain/Parallel / Tangent":                       "Doc.Parallel Doc.Tangent",
	"Constrain/Perpendicular":                            "Doc.Perpendicular",
	"Constrain/Same Orientation":                         "Doc.Constrain",
	"Constrain/Lock Point Where Dragged":                 "Doc.Lock",
	"Constrain/Comment":                                  "Doc.Comment",
}

// apiTypes are the API's types by name, for menuMethods.
var apiTypes = map[string]reflect.Type{
	"Doc":        reflect.TypeFor[*api.Doc](),
	"Group":      reflect.TypeFor[api.Group](),
	"Entity":     reflect.TypeFor[api.Entity](),
	"Request":    reflect.TypeFor[api.Request](),
	"Constraint": reflect.TypeFor[api.Constraint](),
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
		key := strings.Join(item.Path, "/")
		seen[key] = true
		methods, ok := menuMethods[key]
		switch {
		case !ok:
			t.Errorf("the menu entry %q has no script API method; add one, or say in menuMethods why it needs none", key)
		case strings.HasPrefix(methods, "- "):
		default:
			for m := range strings.FieldsSeq(methods) {
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
	"GroupLoft":             "Doc.Loft",
	"GroupShell":            "Doc.Shell",
	"GroupPathPattern":      "Doc.PatternAlongPath",
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
