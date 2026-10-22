package script

import (
	"reflect"
	"strings"

	"github.com/telesapr/telesapr/pkg/script/symbols"
)

// takesString reports whether a function of type t takes a string, or
// a list of them, after its first skip arguments.
func takesString(t reflect.Type, skip int) bool {
	for i := skip; i < t.NumIn(); i++ {
		in := t.In(i)
		if in.Kind() == reflect.String || (in.Kind() == reflect.Slice && in.Elem().Kind() == reflect.String) {
			return true
		}
	}
	return false
}

// stringTaking lists what a symbol of an exported table offers that
// takes a string: the symbol, if it is such a function, or the methods
// of the type it is that do, as "Type.Method". Interfaces are left
// out: what a script calls through one is a value of a type listed
// with its methods, or one this package makes.
func stringTaking(name string, v reflect.Value) []string {
	if strings.HasPrefix(name, "_") {
		// The interpreter's wrappers of interfaces.
		return nil
	}
	switch {
	case v.Kind() == reflect.Func:
		if takesString(v.Type(), 0) {
			return []string{name}
		}
	case v.Kind() == reflect.Pointer && v.IsNil():
		typ := v.Type().Elem()
		if typ.Kind() == reflect.Interface {
			return nil
		}
		var out []string
		seen := map[string]bool{}
		for _, tt := range []reflect.Type{typ, reflect.PointerTo(typ)} {
			for m := range tt.Methods() {
				if !seen[m.Name] && takesString(m.Type, 1) {
					seen[m.Name] = true
					out = append(out, name+"."+m.Name)
				}
			}
		}
		return out
	}
	return nil
}

// pureFilePackages are packages a guarded script has whose calls name
// no files: what strings they take are text, numbers, patterns and
// names of other things.
var pureFilePackages = map[string]bool{
	"bufio/bufio": true, "bytes/bytes": true, "cmp/cmp": true, "context/context": true,
	"encoding/csv/csv": true, "encoding/json/json": true, "errors/errors": true,
	// flag parses os.Args.
	"flag/flag": true, "fmt/fmt": true, "io/io": true,
	// What io/fs reads, it reads through an fs.FS, which os.DirFS gives
	// a guarded script checked.
	"io/fs/fs":  true,
	"math/math": true, "math/bits/bits": true, "math/cmplx/cmplx": true, "math/rand/rand": true,
	"path/path": true, "regexp/regexp": true, "sort/sort": true, "strconv/strconv": true,
	"strings/strings": true, "sync/sync": true, "text/tabwriter/tabwriter": true,
	// time.LoadLocation reads the system's zone files by a zone's name,
	// which it refuses with "..".
	"time/time": true, "unicode/unicode": true, "unicode/utf8/utf8": true,
	"github.com/telesapr/telesapr/pkg/geom/geom": true,
	"github.com/telesapr/telesapr/pkg/srf/srf":   true,
	showPath + "/console":                        true,
}

// fileNamingSymbols lists what a guarded engine's scripts can call
// that takes a string, as "<package> <symbol>", but for the packages
// of pureFilePackages; a symbol the engine replaced (guardExports) is
// marked "guarded".
func fileNamingSymbols(e *Engine) []string {
	var out []string
	for path, pkg := range e.exports(0) {
		if pureFilePackages[path] {
			continue
		}
		for name, v := range pkg {
			for _, s := range stringTaking(name, v) {
				entry := path + " " + s
				if orig, ok := symbols.Symbols[path][name]; v.Kind() == reflect.Func && ok && orig.Pointer() != v.Pointer() {
					entry = "guarded " + entry
				}
				out = append(out, entry)
			}
		}
	}
	return out
}

// Why what takes a string in fileless doesn't take a file's name.
const (
	inRoot    = "inside a root, which os.OpenRoot gives a guarded script only for a folder it may write"
	lexical   = "works on the path's text and reads nothing"
	groupName = "the new group's name"
	ofDrawing = "a name, an expression or text of the drawing"
	interpEnv = "the interpreter's own environment, which it replaces, empty for a guarded script"
)

// fileless are what a guarded script can call that takes a string
// which is not a file's name, with why. TestGuardCoversFiles requires
// every such call to be here, guarded by the engine, or in apiGuarded:
// a new one has to be put in one of them.
var fileless = map[string]string{
	"github.com/telesapr/telesapr/pkg/check/check BaselineKey":                         "compares lines of a baseline, which ReadBaseline reads guarded",
	"github.com/telesapr/telesapr/pkg/check/check CompareBaseline":                     "compares lines of a baseline, which ReadBaseline reads guarded",
	"github.com/telesapr/telesapr/pkg/core/core CnfColor":                              "a setting's name, in a store the script was given",
	"github.com/telesapr/telesapr/pkg/core/core CnfExportable":                         "a setting's name, in a store the script was given",
	"github.com/telesapr/telesapr/pkg/core/core CnfPrefixToName":                       "a setting's name, in a store the script was given",
	"github.com/telesapr/telesapr/pkg/core/core CnfStippleScale":                       "a setting's name, in a store the script was given",
	"github.com/telesapr/telesapr/pkg/core/core CnfStippleType":                        "a setting's name, in a store the script was given",
	"github.com/telesapr/telesapr/pkg/core/core CnfTextHeight":                         "a setting's name, in a store the script was given",
	"github.com/telesapr/telesapr/pkg/core/core CnfWidth":                              "a setting's name, in a store the script was given",
	"github.com/telesapr/telesapr/pkg/core/core FreezeBool":                            "a setting's name, in a store the script was given",
	"github.com/telesapr/telesapr/pkg/core/core FreezeColor":                           "a setting's name, in a store the script was given",
	"github.com/telesapr/telesapr/pkg/core/core NewFromString":                         "a variable's name, an expression or a stipple pattern",
	"github.com/telesapr/telesapr/pkg/core/core NewVariable":                           "a variable's name, an expression or a stipple pattern",
	"github.com/telesapr/telesapr/pkg/core/core NoSettings.FreezeFloat":                "a setting's name, in a store the script was given",
	"github.com/telesapr/telesapr/pkg/core/core NoSettings.FreezeInt":                  "a setting's name, in a store the script was given",
	"github.com/telesapr/telesapr/pkg/core/core NoSettings.FreezeString":               "a setting's name, in a store the script was given",
	"github.com/telesapr/telesapr/pkg/core/core NoSettings.ThawFloat":                  "a setting's name, in a store the script was given",
	"github.com/telesapr/telesapr/pkg/core/core NoSettings.ThawInt":                    "a setting's name, in a store the script was given",
	"github.com/telesapr/telesapr/pkg/core/core NoSettings.ThawString":                 "a setting's name, in a store the script was given",
	"github.com/telesapr/telesapr/pkg/core/core RenameVariable":                        "a variable's name, an expression or a stipple pattern",
	"github.com/telesapr/telesapr/pkg/core/core StipplePatternFromString":              "a variable's name, an expression or a stipple pattern",
	"github.com/telesapr/telesapr/pkg/core/core ThawBool":                              "a setting's name, in a store the script was given",
	"github.com/telesapr/telesapr/pkg/core/core ThawColor":                             "a setting's name, in a store the script was given",
	"github.com/telesapr/telesapr/pkg/core/core ValidVariableName":                     "a variable's name, an expression or a stipple pattern",
	"github.com/telesapr/telesapr/pkg/script/api/api ConstrainMenu.Comment":            ofDrawing,
	"github.com/telesapr/telesapr/pkg/script/api/api ConstrainMenu.CommentOn":          ofDrawing,
	"github.com/telesapr/telesapr/pkg/script/api/api Constraint.SetComment":            ofDrawing,
	"github.com/telesapr/telesapr/pkg/script/api/api Constraint.SetExpr":               ofDrawing,
	"github.com/telesapr/telesapr/pkg/script/api/api Constraint.SetName":               ofDrawing,
	"github.com/telesapr/telesapr/pkg/script/api/api Doc.Constraint":                   ofDrawing,
	"github.com/telesapr/telesapr/pkg/script/api/api Doc.Dimension":                    ofDrawing,
	"github.com/telesapr/telesapr/pkg/script/api/api Doc.Group":                        ofDrawing,
	"github.com/telesapr/telesapr/pkg/script/api/api Doc.Request":                      ofDrawing,
	"github.com/telesapr/telesapr/pkg/script/api/api Doc.Style":                        ofDrawing,
	"github.com/telesapr/telesapr/pkg/script/api/api Doc.Workplane":                    ofDrawing,
	"github.com/telesapr/telesapr/pkg/script/api/api Drawing.DimensionView":            "a view's name or a dimension's kind",
	"github.com/telesapr/telesapr/pkg/script/api/api Drawing.PlaceView":                "a view's name or a dimension's kind",
	"github.com/telesapr/telesapr/pkg/script/api/api Drawing.RemoveView":               "a view's name or a dimension's kind",
	"github.com/telesapr/telesapr/pkg/script/api/api Drawing.ViewIndex":                "a view's name or a dimension's kind",
	"github.com/telesapr/telesapr/pkg/script/api/api DrawingMenu.AddDimension":         "a view's name or a dimension's kind",
	"github.com/telesapr/telesapr/pkg/script/api/api DrawingMenu.AddEdgeFaceDistance":  "a view's name or a dimension's kind",
	"github.com/telesapr/telesapr/pkg/script/api/api DrawingMenu.AddFaceDimension":     "a view's name or a dimension's kind",
	"github.com/telesapr/telesapr/pkg/script/api/api EditMenu.RunCommand":              "a menu command: one that reads or writes a file asks the user for it in a dialog, or uses the document's own",
	"github.com/telesapr/telesapr/pkg/script/api/api Entity.SetFont":                   "a font by name, from the fonts the program knows",
	"github.com/telesapr/telesapr/pkg/script/api/api Entity.SetText":                   ofDrawing,
	"github.com/telesapr/telesapr/pkg/script/api/api FeaturesMenu.Chamfer":             groupName,
	"github.com/telesapr/telesapr/pkg/script/api/api FeaturesMenu.ChamferSolid":        groupName,
	"github.com/telesapr/telesapr/pkg/script/api/api FeaturesMenu.Emboss":              groupName,
	"github.com/telesapr/telesapr/pkg/script/api/api FeaturesMenu.Engrave":             groupName,
	"github.com/telesapr/telesapr/pkg/script/api/api FeaturesMenu.Extrude":             groupName,
	"github.com/telesapr/telesapr/pkg/script/api/api FeaturesMenu.Fillet":              groupName,
	"github.com/telesapr/telesapr/pkg/script/api/api FeaturesMenu.FilletSolid":         groupName,
	"github.com/telesapr/telesapr/pkg/script/api/api FeaturesMenu.Helix":               groupName,
	"github.com/telesapr/telesapr/pkg/script/api/api FeaturesMenu.HelixAbout":          groupName,
	"github.com/telesapr/telesapr/pkg/script/api/api FeaturesMenu.Lathe":               groupName,
	"github.com/telesapr/telesapr/pkg/script/api/api FeaturesMenu.LatheAbout":          groupName,
	"github.com/telesapr/telesapr/pkg/script/api/api FeaturesMenu.Loft":                groupName,
	"github.com/telesapr/telesapr/pkg/script/api/api FeaturesMenu.LoftThrough":         groupName,
	"github.com/telesapr/telesapr/pkg/script/api/api FeaturesMenu.Mirror":              groupName,
	"github.com/telesapr/telesapr/pkg/script/api/api FeaturesMenu.PatternAlongPath":    groupName,
	"github.com/telesapr/telesapr/pkg/script/api/api FeaturesMenu.Revolve":             groupName,
	"github.com/telesapr/telesapr/pkg/script/api/api FeaturesMenu.RevolveAbout":        groupName,
	"github.com/telesapr/telesapr/pkg/script/api/api FeaturesMenu.Rotate":              groupName,
	"github.com/telesapr/telesapr/pkg/script/api/api FeaturesMenu.Shell":               groupName,
	"github.com/telesapr/telesapr/pkg/script/api/api FeaturesMenu.ShellSolid":          groupName,
	"github.com/telesapr/telesapr/pkg/script/api/api FeaturesMenu.Sketch3D":            groupName,
	"github.com/telesapr/telesapr/pkg/script/api/api FeaturesMenu.SketchAt":            groupName,
	"github.com/telesapr/telesapr/pkg/script/api/api FeaturesMenu.SketchIn":            groupName,
	"github.com/telesapr/telesapr/pkg/script/api/api FeaturesMenu.SketchOnFace":        groupName,
	"github.com/telesapr/telesapr/pkg/script/api/api FeaturesMenu.SketchOnLines":       groupName,
	"github.com/telesapr/telesapr/pkg/script/api/api FeaturesMenu.SketchOnNormal":      groupName,
	"github.com/telesapr/telesapr/pkg/script/api/api FeaturesMenu.Sweep":               groupName,
	"github.com/telesapr/telesapr/pkg/script/api/api FeaturesMenu.SweepEdges":          groupName,
	"github.com/telesapr/telesapr/pkg/script/api/api FeaturesMenu.Translate":           groupName,
	"github.com/telesapr/telesapr/pkg/script/api/api FilletGroup.SetEdgeRadiusExpr":    ofDrawing,
	"github.com/telesapr/telesapr/pkg/script/api/api Form.Bool":                        "a field's name",
	"github.com/telesapr/telesapr/pkg/script/api/api Form.Float":                       "a field's name",
	"github.com/telesapr/telesapr/pkg/script/api/api Form.Int":                         "a field's name",
	"github.com/telesapr/telesapr/pkg/script/api/api Form.Text":                        "a field's name",
	"github.com/telesapr/telesapr/pkg/script/api/api Group.SetAppearance":              ofDrawing,
	"github.com/telesapr/telesapr/pkg/script/api/api Group.SetExpr":                    ofDrawing,
	"github.com/telesapr/telesapr/pkg/script/api/api Group.SetName":                    ofDrawing,
	"github.com/telesapr/telesapr/pkg/script/api/api MenuEntriesOf":                    "a menu entry or a call, as help takes them",
	"github.com/telesapr/telesapr/pkg/script/api/api MenuEntryCalls":                   "a menu entry or a call, as help takes them",
	"github.com/telesapr/telesapr/pkg/script/api/api DatumsMenu.Delete":                "a datum's name",
	"github.com/telesapr/telesapr/pkg/script/api/api DatumsMenu.Get":                   "a datum's name",
	"github.com/telesapr/telesapr/pkg/script/api/api DatumsMenu.Name":                  "a datum's name",
	"github.com/telesapr/telesapr/pkg/script/api/api LinkGroup.Param":                  "a part's parameter's name",
	"github.com/telesapr/telesapr/pkg/script/api/api LinkGroup.SetConfiguration":       "a part's configuration's name",
	"github.com/telesapr/telesapr/pkg/script/api/api LinkGroup.SetParam":               "a part's parameter's name and an expression",
	"github.com/telesapr/telesapr/pkg/script/api/api ParamsMenu.ActivateConfiguration": "a parameter's or a configuration's name, or an expression",
	"github.com/telesapr/telesapr/pkg/script/api/api ParamsMenu.AddConfiguration":      "a parameter's or a configuration's name, or an expression",
	"github.com/telesapr/telesapr/pkg/script/api/api ParamsMenu.Delete":                "a parameter's or a configuration's name, or an expression",
	"github.com/telesapr/telesapr/pkg/script/api/api ParamsMenu.DeleteConfiguration":   "a parameter's or a configuration's name, or an expression",
	"github.com/telesapr/telesapr/pkg/script/api/api ParamsMenu.Get":                   "a parameter's or a configuration's name, or an expression",
	"github.com/telesapr/telesapr/pkg/script/api/api ParamsMenu.Rename":                "a parameter's or a configuration's name, or an expression",
	"github.com/telesapr/telesapr/pkg/script/api/api ParamsMenu.RenameConfiguration":   "a parameter's or a configuration's name, or an expression",
	"github.com/telesapr/telesapr/pkg/script/api/api ParamsMenu.Set":                   "a parameter's or a configuration's name, or an expression",
	"github.com/telesapr/telesapr/pkg/script/api/api ParamsMenu.Users":                 "a parameter's or a configuration's name, or an expression",
	"github.com/telesapr/telesapr/pkg/script/api/api SketchMenu.Text":                  "a font by name, from the fonts the program knows",
	"github.com/telesapr/telesapr/pkg/script/api/api Solid.Edge":                       ofDrawing,
	"github.com/telesapr/telesapr/pkg/script/api/api Style.SetName":                    ofDrawing,
	"github.com/telesapr/telesapr/pkg/script/api/api Style.SetTextAlign":               ofDrawing,
	"github.com/telesapr/telesapr/pkg/script/api/api ViewMenu.SetFaceAppearance":       ofDrawing,
	"github.com/telesapr/telesapr/pkg/script/api/api ViewMenu.SetSection":              ofDrawing,
	"os/os Expand":                      "expands with the script's own mapping",
	"os/os ExpandEnv":                   interpEnv,
	"os/os Getenv":                      interpEnv,
	"os/os LookupEnv":                   interpEnv,
	"os/os Setenv":                      interpEnv,
	"os/os Unsetenv":                    interpEnv,
	"os/os File.WriteString":            "writes to a file already open",
	"os/os NewSyscallError":             "the name of a system call",
	"os/os Root.Chmod":                  inRoot,
	"os/os Root.Chown":                  inRoot,
	"os/os Root.Chtimes":                inRoot,
	"os/os Root.Create":                 inRoot,
	"os/os Root.Lchown":                 inRoot,
	"os/os Root.Link":                   inRoot,
	"os/os Root.Lstat":                  inRoot,
	"os/os Root.Mkdir":                  inRoot,
	"os/os Root.MkdirAll":               inRoot,
	"os/os Root.Open":                   inRoot,
	"os/os Root.OpenFile":               inRoot,
	"os/os Root.OpenRoot":               inRoot,
	"os/os Root.ReadFile":               inRoot,
	"os/os Root.Readlink":               inRoot,
	"os/os Root.Remove":                 inRoot,
	"os/os Root.RemoveAll":              inRoot,
	"os/os Root.Rename":                 inRoot,
	"os/os Root.Stat":                   inRoot,
	"os/os Root.Symlink":                inRoot,
	"os/os Root.WriteFile":              inRoot,
	"path/filepath/filepath Abs":        lexical,
	"path/filepath/filepath Base":       lexical,
	"path/filepath/filepath Clean":      lexical,
	"path/filepath/filepath Dir":        lexical,
	"path/filepath/filepath Ext":        lexical,
	"path/filepath/filepath FromSlash":  lexical,
	"path/filepath/filepath HasPrefix":  lexical,
	"path/filepath/filepath IsAbs":      lexical,
	"path/filepath/filepath IsLocal":    lexical,
	"path/filepath/filepath Join":       lexical,
	"path/filepath/filepath Localize":   lexical,
	"path/filepath/filepath Match":      lexical,
	"path/filepath/filepath Rel":        lexical,
	"path/filepath/filepath Split":      lexical,
	"path/filepath/filepath SplitList":  lexical,
	"path/filepath/filepath ToSlash":    lexical,
	"path/filepath/filepath VolumeName": lexical,
}

// apiGuarded are the calls of package api that take a file's name and
// ask the document's host (api.FileGuard) before they use it;
// TestGuardedScript tries each.
var apiGuarded = []string{
	"AssemblyMenu.ExportBOM",
	"AssemblyMenu.Link",
	"AssemblyMenu.LinkStep",
	"AssemblyMenu.MotionStudy",
	"DrawingMenu.Export",
	"FileMenu.Export",
	"FileMenu.ExportAs",
	"FileMenu.ExportScript",
	"FileMenu.ExportSection",
	"FileMenu.ImportDXF",
	"FileMenu.Load",
	"FileMenu.Render",
	"FileMenu.Save",
	"LinkGroup.SetFile",
	"SketchMenu.Image",
}
