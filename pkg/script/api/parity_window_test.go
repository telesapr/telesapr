package api_test

// The window changes the model by more than its menu: links and edit boxes
// on the text screens, the context menu, drags. completeness_test.go holds
// the menu to the script API; these tests hold the rest, both ways:
//
//   - every exported method of the API's object types (Group, Entity,
//     Style, the typed groups ...) names the window action it matches, or
//     says why it has none (TestEveryObjectMethodHasAWindowAction);
//   - every link, edit box, context-menu entry and drag of the window names
//     the script call that does the same, or says why there is none
//     (TestEveryWindowActionHasAScriptCall).
//
// Both tables are checked against the code they describe, so neither can
// name something that is gone. A value that starts "GAP:" is a window edit
// of the model that no script call reaches: it is a to-do, and `grep GAP:`
// finds them.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/script/api"
	"github.com/telesapr/telesapr/pkg/ui"
)

// uiDir is the interface package's source.
var uiDir = filepath.Join("..", "..", "ui")

// windowItems are the window's actions that change or could change the
// model, found in the source of package ui.
type windowItems struct {
	// screens maps a link handler (a textWindow method named Screen...)
	// to the letters of its "switch link", none for a handler without.
	screens map[string][]string
	edits   []string // the Edit constants but editNothing
	context map[string]bool
	drags   map[string]bool
}

// parseUI reads package ui's non-test files.
func parseUI(t *testing.T) []*ast.File {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(uiDir, "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	var files []*ast.File
	for _, path := range paths {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, f)
	}
	return files
}

// receiverName is the type name of a method's receiver.
func receiverName(fd *ast.FuncDecl) string {
	if fd.Recv == nil || len(fd.Recv.List) != 1 {
		return ""
	}
	x := fd.Recv.List[0].Type
	if s, ok := x.(*ast.StarExpr); ok {
		x = s.X
	}
	if id, ok := x.(*ast.Ident); ok {
		return id.Name
	}
	return ""
}

// stringArg is the string literal a call's first argument is, if it is.
func stringArg(c *ast.CallExpr) (string, bool) {
	if len(c.Args) == 0 {
		return "", false
	}
	bl, ok := c.Args[0].(*ast.BasicLit)
	if !ok || bl.Kind != token.STRING {
		return "", false
	}
	s, err := strconv.Unquote(bl.Value)
	return s, err == nil
}

// scanUI lists the window's actions from the source.
func scanUI(t *testing.T) windowItems {
	t.Helper()
	w := windowItems{screens: map[string][]string{}, context: map[string]bool{}, drags: map[string]bool{}}
	files := parseUI(t)
	for _, f := range files {
		for _, decl := range f.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok || fd.Body == nil {
				continue
			}
			name, recv := fd.Name.Name, receiverName(fd)
			if recv == "textWindow" && strings.HasPrefix(name, "Screen") {
				var letters []string
				ast.Inspect(fd.Body, func(n ast.Node) bool {
					sw, ok := n.(*ast.SwitchStmt)
					if !ok {
						return true
					}
					if id, ok := sw.Tag.(*ast.Ident); !ok || id.Name != "link" {
						return true
					}
					for _, c := range sw.Body.List {
						for _, e := range c.(*ast.CaseClause).List {
							if bl, ok := e.(*ast.BasicLit); ok && bl.Kind == token.CHAR {
								letters = append(letters, strings.Trim(bl.Value, "'"))
							}
						}
					}
					return true
				})
				w.screens[name] = letters
			}
			// A drag is a graphicsWindow method dragXxx of the mouse.
			if recv == "graphicsWindow" && len(name) > 4 && strings.HasPrefix(name, "drag") &&
				name[4] >= 'A' && name[4] <= 'Z' {
				w.drags[name] = true
			}
		}
	}
	// The Edit constants.
	w.edits = slices.DeleteFunc(constsOfType(t, uiDir, "Edit"), func(s string) bool { return s == "editNothing" })

	// The context menu's entries: string literals given to AddItem and
	// AddSubMenu, and the labels addGroupMenu picks between.
	path := filepath.Join(uiDir, "contextmenu.go")
	f, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, decl := range f.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok || fd.Body == nil {
			continue
		}
		ast.Inspect(fd.Body, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.CallExpr:
				if sel, ok := n.Fun.(*ast.SelectorExpr); ok && (sel.Sel.Name == "AddItem" || sel.Sel.Name == "AddSubMenu") {
					if s, ok := stringArg(n); ok {
						w.context[s] = true
					}
				}
			case *ast.AssignStmt:
				if fd.Name.Name == "addGroupMenu" {
					for _, r := range n.Rhs {
						if bl, ok := r.(*ast.BasicLit); ok && bl.Kind == token.STRING {
							s, _ := strconv.Unquote(bl.Value)
							w.context[s] = true
						}
					}
				}
			}
			return true
		})
	}
	return w
}

// isReason reports whether a table's value is a reason rather than a
// list of actions or calls.
func isReason(v string) bool {
	return strings.HasPrefix(v, "- ") || strings.HasPrefix(v, "GAP:")
}

// windowActions says, for each action of the window that is not a menu
// command, which script calls do the same ("Type.Method", spaces apart),
// or, after "- ", why there is none; "GAP:" marks a model edit that no
// script call reaches. Keys: a link handler "ScreenX", or "ScreenX/c" for
// the letter c of its switch (a handler with letters is listed by letter);
// an edit box "editX"; a context-menu entry "ctx:Label"; a drag "dragX".
var windowActions = map[string]string{
	// The group list and group screen.
	"ScreenHome":                "- navigation",
	"ScreenSelectGroup":         "- navigation: shows a group's screen",
	"ScreenToggleGroupShown":    "Group.SetVisible",
	"ScreenShowGroupsSpecial/s": "Group.SetVisible",
	"ScreenShowGroupsSpecial/c": "Group.SetVisible Group.Dof",
	"ScreenShowGroupsSpecial/h": "Group.SetVisible",
	"ScreenActivateGroup":       "Doc.Activate",
	"ScreenHowGroupSolved":      "Group.Conflicts Group.Solved",
	"ScreenShowConfiguration":   "- navigation",
	"ScreenShowEditView":        "- navigation",
	"ScreenGoToWebsite":         "- opens the project's page in a browser",
	"ScreenHoverGroupWorkplane": "- hover highlight, changes nothing",
	"ScreenHoverEntity":         "- hover highlight, changes nothing",
	"ScreenHoverRequest":        "- hover highlight, changes nothing",
	"ScreenHoverConstraint":     "- hover highlight, changes nothing",
	"ScreenSelectEntity":        "Edit.Select",
	"ScreenSelectRequest":       "Edit.Select",
	"ScreenSelectConstraint":    "Edit.Select",
	"ScreenShowConstraint":      "- shows where the constraint is in the sketch; changes nothing",
	"ScreenUnselectAll":         "Edit.Select",

	"ScreenChangeGroupOption/s": "Group.SetTwoSided",
	"ScreenChangeGroupOption/S": "Group.SetTwoSided",
	"ScreenChangeGroupOption/w": "ExtrudeGroup.SetSkewed",
	"ScreenChangeGroupOption/k": "RepeatGroup.SetSkipFirst MirrorGroup.SetKeepOriginal",
	"ScreenChangeGroupOption/K": "RepeatGroup.SetSkipFirst MirrorGroup.SetKeepOriginal",
	"ScreenChangeGroupOption/m": "MirrorGroup.SetMirrorSolid",
	"ScreenChangeGroupOption/c": "Group.SetCombine",
	"ScreenChangeGroupOption/P": "Group.SetSuppress",
	"ScreenChangeGroupOption/A": "Group.SetSuppress",
	"ScreenChangeGroupOption/r": "Group.SetRelaxConstraints",
	"ScreenChangeGroupOption/e": "Group.SetAllowRedundant",
	"ScreenChangeGroupOption/D": "Group.SetSuppressDof",
	"ScreenChangeGroupOption/v": "Group.SetVisible",
	"ScreenChangeGroupOption/d": "Group.SetAllDimsReference",
	"ScreenChangeGroupOption/f": "Group.SetForceToMesh",

	"ScreenChangeLoftTurn":            "LoftGroup.SetTurn",
	"ScreenColor":                     "Group.SetColor",
	"ScreenOpacity":                   "Group.SetOpacity",
	"ScreenDensity":                   "Group.SetDensity",
	"ScreenChangeExprA":               "RepeatGroup.SetCopies Group.SetExpr",
	"ScreenChangeGroupName":           "Group.SetName",
	"ScreenChangeGroupScale":          "Group.SetExpr",
	"ScreenChangeHelixPitch":          "Group.SetExpr FilletGroup.SetRadius ChamferGroup.SetDistance ShellGroup.SetThickness SweepGroup.SetCornerRadius ExtrudeGroup.SetDraftNeutral",
	"ScreenChangeLinkAsSolid":         "LinkGroup.SetAsSolid",
	"ScreenRelink":                    "LinkGroup.SetFile",
	"ScreenChangeLinkConfiguration":   "LinkGroup.SetConfiguration",
	"ScreenChangeLinkParam":           "LinkGroup.SetParam",
	"ScreenChangeSweepUp/x":           "SweepGroup.SetSweepUp",
	"ScreenChangeSweepUp/y":           "SweepGroup.SetSweepUp",
	"ScreenChangeSweepUp/z":           "SweepGroup.SetSweepUp",
	"ScreenChangeGroupDraft":          "ExtrudeGroup.SetDraft SweepGroup.SetTwist LoftGroup.SetTangency Group.SetExpr",
	"ScreenChangeGroupExtent":         "ExtrudeGroup.SetDepth RevolveGroup.SetAngle Group.SetExpr",
	"ScreenChangeExtentHeld":          "ExtrudeGroup.SetDepth ExtrudeGroup.FreeDepth RevolveGroup.SetAngle RevolveGroup.FreeAngle",
	"ScreenChangeChamferDistance2":    "ChamferGroup.SetSecondDistance",
	"ScreenSwapChamferDistances":      "ChamferGroup.SwapDistances",
	"ScreenSameChamferDistances":      "ChamferGroup.SetSecondDistance",
	"ScreenLoftNoGuides":              "LoftGroup.SetGuides",
	"ScreenLoftAlongSides":            "LoftGroup.SetAlongSides",
	"ScreenLoftClosed":                "LoftGroup.SetClosed",
	"ScreenChangeLoftProfileTangency": "LoftGroup.SetProfileTangency",
	"ScreenDraftAllSides":             "ExtrudeGroup.SetDraftFaces",
	"ScreenChangeLoftStart":           "LoftGroup.SetTangency",
	"ScreenChangePitchOption":         "Group.SetExpr",
	"ScreenDeleteGroup":               "Group.Delete",
	"ScreenChangeEdgeRadius":          "FilletGroup.SetEdgeRadius FilletGroup.SetEdgeRadiusExpr",
	"ScreenResetEdgeRadius":           "FilletGroup.SetEdgeRadiusExpr",
	"ScreenAllowRedundant":            "Group.SetAllowRedundant",
	"ScreenStepDimFinish":             "Constraint.Set",
	"ScreenStepDimSteps":              "Constraint.Set",
	"ScreenStepDimGo":                 "Constraint.Set",
	"ScreenChangeTangentArc/r":        "Sketch.TangentArc",
	"ScreenChangeTangentArc/a":        "Sketch.TangentArc",
	"ScreenChangeTangentArc/m":        "Sketch.TangentArc",
	"ScreenChangeAppearance":          "Group.SetAppearance",
	"ScreenMoveGroup":                 "GroupHistory.MoveBefore GroupHistory.MoveToEnd",
	"ScreenShowBrokenReferences":      "- navigation; Features.BrokenReferences lists them",
	"ScreenClearBrokenReferences":     "Features.ClearBrokenReferences",
	"ScreenExportBOM":                 "Assembly.ExportBOM",
	"ScreenChangeCommandQuery":        "- the search box of Edit > Find Command; the script names the command",
	"ScreenRunCommand":                "Edit.RunCommand",
	"ScreenChangeThread/d":            "Features.Thread ThreadGroup.Edit",
	"ScreenChangeThread/l":            "Features.Thread ThreadGroup.Edit",
	"ScreenChangeThread/c":            "Features.Thread ThreadGroup.Edit",
	"ScreenChangeThread/e":            "Features.Thread ThreadGroup.Edit",
	"ScreenChangeThread/i":            "Features.Thread ThreadGroup.Edit",
	"ScreenChangeThread/t":            "Features.Thread ThreadGroup.Edit Features.ThreadOnFace",
	"ScreenChangeThread/r":            "Features.Thread ThreadGroup.Edit Features.ThreadOnFace",
	"ScreenChangeThread/n":            "Features.Thread ThreadGroup.Edit",
	"ScreenChangeThread/u":            "Features.Thread ThreadGroup.Edit",
	"ScreenChangeThread/o":            "Features.Thread ThreadGroup.Edit",
	"ScreenChangeThread/b":            "Features.Thread ThreadGroup.Edit",
	"ScreenChangeThread/s":            "Features.Thread ThreadGroup.Edit",
	"ScreenChangeThread/h":            "Features.Thread ThreadGroup.Edit",
	"ScreenChangeThread/k":            "Features.Thread ThreadGroup.Edit",
	"ScreenChangeThread/y":            "Features.Thread ThreadGroup.Edit Features.ThreadOnFace",
	"ScreenChangeThread/z":            "Features.Thread ThreadGroup.Edit Features.ThreadOnFace",
	"ScreenChangeThread/x":            "Features.Thread ThreadGroup.Edit Features.ThreadOnFace",
	"ScreenChangeThread/m":            "- navigation: shows the sizes to choose from, which ThreadSizes and ThreadPitches give a script",
	"ScreenPickThread/S":              "- navigation: shows another series' sizes, which ThreadSizes gives a script",
	"ScreenPickThread/Z":              "Features.Thread ThreadGroup.Edit",
	"ScreenPickThread/P":              "Features.Thread ThreadGroup.Edit",
	"ScreenPickThread/B":              "- navigation",
	"ScreenChangeThread/f":            "Features.Thread ThreadGroup.Edit",
	"ScreenChangeThread/q":            "- navigation: shows the classes to choose from, which ThreadClasses gives a script",
	"ScreenPickThreadClass/C":         "Features.Thread ThreadGroup.Edit",
	"ScreenPickThreadClass/N":         "Features.Thread ThreadGroup.Edit",
	"ScreenPickThreadClass/B":         "- navigation",
	"ScreenMakeThread":                "Features.Thread ThreadGroup.Edit",
	"ScreenEditThread":                "ThreadGroup.Edit",
	"ScreenThreadAction/o":            "- navigation: shows a thread's groups in the group list or folds them, which ThreadGroup.Groups gives a script",
	"ScreenThreadAction/v":            "ThreadGroup.SetVisible",
	"ScreenThreadAction/u":            "ThreadGroup.MoveBefore",
	"ScreenThreadAction/d":            "ThreadGroup.MoveBefore",
	"ScreenThreadAction/s":            "ThreadGroup.SetSuppressed",
	"ScreenThreadAction/x":            "ThreadGroup.Delete",
	"ScreenChangeMotion/n":            "Assembly.MotionStudy",
	"ScreenChangeMotion/f":            "Assembly.MotionStudy",
	"ScreenChangeMotion/t":            "Assembly.MotionStudy",
	"ScreenChangeMotion/s":            "Assembly.MotionStudy",
	"ScreenChangeMotion/r":            "Assembly.MotionStudy",
	"ScreenChangeMotion/R":            "Assembly.MotionStudy",
	"ScreenChangeClearance/g":         "Assembly.Clearances",
	"ScreenChangeClearance/w":         "GroupAnalysis.WallThickness",
	"ScreenChangeClearance/c":         "Assembly.Clearances",
	"ScreenChangeClearance/t":         "GroupAnalysis.WallThickness",
	"ScreenChangeClearance/s":         "Assembly.ClearancesWithin",
	"ScreenChangePasteTransformed/t":  "Edit.Paste",
	"ScreenChangePasteTransformed/r":  "Edit.Paste",
	"ScreenChangePasteTransformed/s":  "Edit.Paste",
	"ScreenChangePasteTransformed/f":  "Edit.Paste",
	"ScreenPasteTransformed/o":        "Edit.Paste",
	"ScreenPasteTransformed/t":        "Edit.Paste",
	"ScreenPasteTransformed/g":        "Edit.Paste",

	// The describe and constraint screens.
	"ScreenEditTtfText":               "Entity.SetText",
	"ScreenToggleTtfKerning":          "Entity.SetKerning",
	"ScreenSetTtfFont":                "Entity.SetFont",
	"ScreenConstraintToggleReference": "Constraint.SetReference",
	"ScreenConstraintShowAsRadius":    "Constraint.SetShowAsRadius",

	// Parameters and configurations.
	"ScreenShowParameters":        "- navigation",
	"ScreenChangeParameter/e":     "Params.Set",
	"ScreenChangeParameter/n":     "Params.Rename",
	"ScreenChangeParameter/x":     "Params.Delete",
	"ScreenChangeParameter/a":     "Params.Set",
	"ScreenChangeDatum/a":         "Datums.Name",
	"ScreenChangeDatum/n":         "Datums.Name",
	"ScreenChangeDatum/x":         "Datums.Delete",
	"ScreenChangeDatum/s":         "Datums.Get",
	"ScreenChangeDatum/f":         "Datums.NameFace Datums.NameEdge",
	"ScreenPickDatumForMate":      "Datums.Get",
	"ScreenMateDatums":            "Assembly.Flush Assembly.Concentric Assembly.Hinge",
	"ScreenSelectLinkedDatum":     "Datums.Get",
	"ScreenChangeConfiguration/c": "Params.ActivateConfiguration",
	"ScreenChangeConfiguration/r": "Params.RenameConfiguration",
	"ScreenChangeConfiguration/x": "Params.DeleteConfiguration",
	"ScreenChangeConfiguration/A": "Params.AddConfiguration",

	// The section view.
	"ScreenChangeSectionOffset": "View.SetSection",
	"ScreenChangeSectionFlip":   "View.SetSection",
	"ScreenSectionOff":          "View.ClearSection",

	// The drawing.
	"ScreenChangeDrawing/w": "Drawing.Show",
	"ScreenChangeDrawing/s": "Drawing.Set",
	"ScreenChangeDrawing/o": "Drawing.Set",
	"ScreenChangeDrawing/O": "Drawing.Set",
	"ScreenChangeDrawing/t": "Drawing.Set",
	"ScreenChangeDrawing/f": "Drawing.Set",
	"ScreenChangeDrawing/h": "Drawing.Set",
	"ScreenChangeDrawing/T": "Drawing.Set",
	"ScreenChangeDrawing/c": "Drawing.Set",
	"ScreenChangeDrawing/V": "Drawing.Set",
	"ScreenChangeDrawing/N": "Drawing.Set",
	"ScreenChangeDrawing/v": "Drawing.Set",
	"ScreenChangeDrawing/M": "Drawing.Set",
	"ScreenChangeDrawing/y": "Drawing.Set",
	"ScreenChangeDrawing/Y": "Drawing.Set",
	"ScreenChangeDrawing/x": "Drawing.Set",
	"ScreenChangeDrawing/X": "Drawing.Set",
	"ScreenChangeDrawing/r": "Drawing.Set",
	"ScreenChangeDrawing/m": "Drawing.AddDimension Drawing.AddFaceDimension Drawing.AddEdgeFaceDistance",
	"ScreenChangeDrawing/a": "Drawing.Set",
	"ScreenChangeDrawing/d": "Drawing.Set",
	"ScreenChangeDrawing/1": "Drawing.Set",
	"ScreenChangeDrawing/2": "Drawing.Set",
	"ScreenChangeDrawing/3": "Drawing.Set",
	"ScreenChangeDrawing/4": "Drawing.Set",
	"ScreenChangeDrawing/5": "Drawing.Set",
	"ScreenChangeDrawing/6": "Drawing.Set",

	// Styles.
	"ScreenShowListOfStyles":         "- navigation",
	"ScreenShowStyleInfo":            "- navigation",
	"ScreenLoadFactoryDefaultStyles": "Doc.ResetStyles",
	"ScreenCreateCustomStyle":        "Doc.NewStyle",
	"ScreenChangeBackgroundColor":    "- the window's background colour, an application setting that is not in the file",
	"ScreenChangeStyleName":          "Style.SetName",
	"ScreenDeleteStyle":              "Style.Delete",
	"ScreenChangeStylePatternType":   "Style.SetStipple",
	"ScreenChangeStyleMetric/t":      "Style.SetTextHeight",
	"ScreenChangeStyleMetric/s":      "Style.SetStippleScale",
	"ScreenChangeStyleMetric/w":      "Style.SetWidth",
	"ScreenChangeStyleMetric/W":      "Style.SetWidth",
	"ScreenChangeStyleTextAngle":     "Style.SetTextAngle",
	"ScreenChangeStyleZIndex":        "Style.SetZIndex",
	"ScreenChangeStyleColor/s":       "Style.SetColor",
	"ScreenChangeStyleColor/f":       "Style.SetFillColor",
	"ScreenChangeStyleYesNo/w":       "Style.SetWidthAs",
	"ScreenChangeStyleYesNo/W":       "Style.SetWidthAs",
	"ScreenChangeStyleYesNo/g":       "Style.SetTextHeightAs",
	"ScreenChangeStyleYesNo/G":       "Style.SetTextHeightAs",
	"ScreenChangeStyleYesNo/e":       "Style.SetExportable",
	"ScreenChangeStyleYesNo/v":       "Style.SetVisible",
	"ScreenChangeStyleYesNo/f":       "Style.SetFilled",
	"ScreenChangeStyleYesNo/L":       "Style.SetTextAlign",
	"ScreenChangeStyleYesNo/H":       "Style.SetTextAlign",
	"ScreenChangeStyleYesNo/R":       "Style.SetTextAlign",
	"ScreenChangeStyleYesNo/B":       "Style.SetTextAlign",
	"ScreenChangeStyleYesNo/V":       "Style.SetTextAlign",
	"ScreenChangeStyleYesNo/T":       "Style.SetTextAlign",
	"ScreenAssignSelectionToStyle":   "Request.SetStyle Constraint.SetStyle",

	// The configuration screen: what is kept with the application's
	// settings, not in the file, has no call; the export options and the
	// tolerances have.
	"ScreenChangeColor":                    "- the model colours of the window, an application setting",
	"ScreenChangeChordTolerance":           "Edit.SetChordTolerance",
	"ScreenChangeMaxSegments":              "Edit.SetMaxSegments",
	"ScreenChangeExportChordTolerance":     "File.SetExportOptions",
	"ScreenChangeExportMaxSegments":        "File.SetExportOptions",
	"ScreenChangeGridSpacing":              "- the snap grid is the window's view",
	"ScreenChangeDigitsAfterDecimal":       "- how the window writes numbers; an application setting",
	"ScreenChangeDigitsAfterDecimalDegree": "- how the window writes numbers; an application setting",
	"ScreenChangeUseSIPrefixes":            "- how the window writes numbers; an application setting",
	"ScreenChangeExportScale":              "File.SetExportOptions",
	"ScreenChangeExportOffset":             "File.SetExportOptions",
	"ScreenChangeArcDimDefault":            "- which dimension the Distance / Diameter command makes on an arc; Constrain.Diameter and Constrain.Radius say which",
	"ScreenChangeShowFullFilePath":         "- the window's title; an application setting",
	"ScreenChangeFixExportColors":          "File.SetExportOptions",
	"ScreenChangeExportBackgroundColor":    "File.SetExportOptions",
	"ScreenChangeBackFaces":                "- how the window draws; an application setting",
	"ScreenChangeTurntableNav":             "- how the mouse turns the view",
	"ScreenChangeUseOpenGL":                "- the window's renderer; File.Render takes its own",
	"ScreenChangeDragSolidsLate":           "- how the window draws during a drag; an application setting",
	"ScreenChangeAgentConfirm":             "- whether the window asks before a connected program's script runs",
	"ScreenChangeCameraNav":                "- how the mouse turns the view",
	"ScreenChangeImmediatelyEditDimension": "- whether a new dimension opens its edit box; a script gives the value",
	"ScreenChangeShowContourAreas":         "- how the window draws; an application setting",
	"ScreenChangeCheckClosedContour":       "Edit.SetCheckClosedContour",
	"ScreenChangeColorByFreedom":           "- how the window draws; Entity.Free reads the same",
	"ScreenChangeShaded":                   "- how the window draws; an application setting",
	"ScreenChangeAutomaticLineConstraints": "Constrain.Auto",
	"ScreenChangeShadedTriangles":          "File.SetExportOptions",
	"ScreenChangePwlCurves":                "File.SetExportOptions",
	"ScreenChangeCanvasSizeAuto":           "File.SetExportOptions",
	"ScreenChangeCanvasSize":               "File.SetExportOptions",
	"ScreenChangeGCodeParameter/d":         "File.SetExportOptions",
	"ScreenChangeGCodeParameter/h":         "File.SetExportOptions",
	"ScreenChangeGCodeParameter/s":         "File.SetExportOptions",
	"ScreenChangeGCodeParameter/F":         "File.SetExportOptions",
	"ScreenChangeGCodeParameter/P":         "File.SetExportOptions",
	"ScreenChangeAutosaveInterval":         "- how often the window saves a backup; an application setting",
	"ScreenChangeAgentGuarded":             "- whether a connected program's scripts are kept to the drawing's folder; an application setting, --unguarded on the command line",
	"ScreenChangeAgentFolders":             "- the folders a connected program's scripts may use; an application setting, --allow on the command line",
	"ScreenChangeScriptLibrary":            "- where the window's scripts import their own packages from; an application setting, --lib on the command line",
	"ScreenChangeFindConstraintTimeout":    "- how long the window looks for the constraints to remove; an application setting",
	"ScreenChangeAnimationSpeed":           "- how fast the window turns the view; an application setting",

	// The view screen.
	"ScreenChangeViewScale":       "- the window's view; File.Render takes its own",
	"ScreenChangeViewToFullScale": "- the window's view",
	"ScreenChangeViewOrigin":      "- the window's view",
	"ScreenChangeViewProjection":  "- the window's view",
	"ScreenChangeLightDirection":  "- the window's lights; File.Render has its own",
	"ScreenChangeLightIntensity":  "- the window's lights; File.Render has its own",
	"ScreenChangeLightAmbient":    "- the window's lights; File.Render has its own",
	"ScreenChangeCameraTangent":   "- the window's perspective",
	"ScreenChangeExplodeDistance": "- the exploded view is the window's view",

	// The edit boxes: what EditControlDone does with the text.
	"editTimesRepeated":            "RepeatGroup.SetCopies Group.SetExpr",
	"editGroupName":                "Group.SetName",
	"editGroupScale":               "Group.SetExpr",
	"editLinkParam":                "LinkGroup.SetParam",
	"editGroupColor":               "Group.SetColor",
	"editGroupOpacity":             "Group.SetOpacity",
	"editGroupDensity":             "Group.SetDensity",
	"editLightDirection":           "- the window's lights",
	"editLightIntensity":           "- the window's lights",
	"editColor":                    "- the model colours of the window",
	"editChordTolerance":           "Edit.SetChordTolerance File.SetExportOptions",
	"editMaxSegments":              "Edit.SetMaxSegments File.SetExportOptions",
	"editCameraTangent":            "- the window's perspective",
	"editGridSpacing":              "- the snap grid is the window's view",
	"editDigitsAfterDecimal":       "- how the window writes numbers",
	"editDigitsAfterDecimalDegree": "- how the window writes numbers",
	"editExportScale":              "File.SetExportOptions",
	"editExportOffset":             "File.SetExportOptions",
	"editCanvasSize":               "File.SetExportOptions",
	"editGCodeDepth":               "File.SetExportOptions",
	"editGCodeSafeHeight":          "File.SetExportOptions",
	"editGCodePasses":              "File.SetExportOptions",
	"editGCodeFeed":                "File.SetExportOptions",
	"editGCodePlungeFeed":          "File.SetExportOptions",
	"editAutosaveInterval":         "- an application setting",
	"editScriptLibrary":            "- an application setting",
	"editAgentFolders":             "- an application setting",
	"editLightAmbient":             "- the window's lights",
	"editFindConstraintTimeout":    "- an application setting",
	"editExplodeDistance":          "- the exploded view is the window's view",
	"editAnimationSpeed":           "- an application setting",
	"editTTFText":                  "Entity.SetText",
	"editStepDimFinish":            "Constraint.Set",
	"editStepDimSteps":             "Constraint.Set",
	"editStyleWidth":               "Style.SetWidth",
	"editStyleTextHeight":          "Style.SetTextHeight",
	"editStyleTextAngle":           "Style.SetTextAngle",
	"editStyleColor":               "Style.SetColor",
	"editStyleFillColor":           "Style.SetFillColor",
	"editStyleName":                "Style.SetName",
	"editBackgroundColor":          "- the window's background colour, an application setting",
	"editStyleStipplePeriod":       "Style.SetStippleScale",
	"editStyleZIndex":              "Style.SetZIndex",
	"editPasteTimesRepeated":       "Edit.Paste",
	"editPasteAngle":               "Edit.Paste",
	"editPasteScale":               "Edit.Paste",
	"editViewScale":                "- the window's view",
	"editViewOrigin":               "- the window's view",
	"editViewProjRight":            "- the window's view",
	"editViewProjUp":               "- the window's view",
	"editTangentArcRadius":         "Sketch.TangentArc",
	"editHelixPitch":               "Group.SetExpr FilletGroup.SetRadius ChamferGroup.SetDistance ShellGroup.SetThickness SweepGroup.SetCornerRadius LoftGroup.SetTangency",
	"editEdgeRadius":               "FilletGroup.SetEdgeRadius FilletGroup.SetEdgeRadiusExpr",
	"editGroupDraft":               "ExtrudeGroup.SetDraft SweepGroup.SetTwist LoftGroup.SetTangency Group.SetExpr",
	"editLoftProfileTangency":      "LoftGroup.SetProfileTangency",
	"editGroupExtent":              "ExtrudeGroup.SetDepth RevolveGroup.SetAngle Group.SetExpr",
	"editChamferDistance2":         "ChamferGroup.SetSecondDistance",
	"editThreadDesignation":        "Features.Thread ThreadGroup.Edit",
	"editThreadLength":             "Features.Thread ThreadGroup.Edit",
	"editThreadClearance":          "Features.Thread ThreadGroup.Edit",
	"editThreadPhase":              "Features.Thread ThreadGroup.Edit",
	"editThreadRunOut":             "Features.Thread ThreadGroup.Edit",
	"editParameterAdd":             "Params.Set",
	"editParameterExpr":            "Params.Set",
	"editParameterName":            "Params.Rename",
	"editDatumAdd":                 "Datums.Name",
	"editDatumName":                "Datums.Name",
	"editDatumSolid":               "Datums.NameFace Datums.NameEdge",
	"editConfigurationAdd":         "Params.AddConfiguration",
	"editConfigurationName":        "Params.RenameConfiguration",
	"editMotionParameter":          "Assembly.MotionStudy",
	"editMotionFrom":               "Assembly.MotionStudy",
	"editMotionTo":                 "Assembly.MotionStudy",
	"editMotionSteps":              "Assembly.MotionStudy",
	"editClearanceGap":             "Assembly.Clearances",
	"editClearanceWall":            "GroupAnalysis.WallThickness",
	"editDrawingScale":             "Drawing.Set",
	"editDrawingField":             "Drawing.Set",
	"editDrawingSection":           "Drawing.Set",
	"editDrawingDetail":            "Drawing.Set",
	"editDrawingDimension":         "Drawing.AddDimension Drawing.AddFaceDimension Drawing.AddEdgeFaceDistance",
	"editDrawingPlace":             "Drawing.Set",
	"editCommandQuery":             "- the search box of Edit > Find Command",
	"editSectionOffset":            "View.SetSection",

	// The context menu: most entries run the menu entry of the same name.
	"ctx:Assign to Style":                    "Request.SetStyle Constraint.SetStyle",
	"ctx:Screw Thread...":                    "Features.Thread Features.ThreadOnFace Features.ThreadOnEdge",
	"ctx:No Style":                           "Request.SetStyle Constraint.SetStyle",
	"ctx:Newly Created Custom Style...":      "Doc.NewStyle Request.SetStyle",
	"ctx:Group Info":                         "- shows the group's screen",
	"ctx:Style Info":                         "- shows the style's screen",
	"ctx:Select Edge Chain":                  "- a selection made with the mouse; a script names the entities, Edit.Select",
	"ctx:Toggle Reference Dimension":         "Constraint.SetReference",
	"ctx:Other Supplementary Angle":          "Constraint.SetOther",
	"ctx:Snap to Grid":                       "Entity.MoveTo",
	"ctx:Remove Spline Point":                "Entity.RemoveSplinePoint",
	"ctx:Add Spline Point":                   "Entity.AddSplinePoint",
	"ctx:Toggle Construction":                "Entity.SetConstruction Request.SetConstruction",
	"ctx:Delete Point-Coincident Constraint": "Entity.DeleteCoincidences",
	"ctx:Cut":                                "Edit.Copy Entity.Delete",
	"ctx:Copy":                               "Edit.Copy",
	"ctx:Select All":                         "Edit.Select",
	"ctx:Paste":                              "Edit.Paste",
	"ctx:Paste Transformed...":               "Edit.Paste",
	"ctx:Delete":                             "Entity.Delete Constraint.Delete Group.Delete",
	"ctx:Unselect All":                       "Edit.Select",
	"ctx:Unselect Hovered":                   "Edit.Select",
	"ctx:Zoom to Fit":                        "- the window's view",
	"ctx:Fillet Edges":                       "Features.FilletSolid",
	"ctx:Chamfer Edges":                      "Features.ChamferSolid",
	"ctx:Sweep Along Edges":                  "Features.SweepEdges",
	"ctx:Shell Solid Open Here":              "Features.ShellSolid",
	"ctx:Wrap Sketch Onto This Face":         "Features.Emboss",
	"ctx:Draft Selected Sides":               "ExtrudeGroup.SetDraftFaces",
	"ctx:Mirror in This Plane":               "Features.Mirror",
	"ctx:Select Face Edges":                  "Edit.SelectFaceEdges",
	"ctx:Sketch On Face":                     "Features.SketchOnFace",
	"ctx:Appearance":                         "View.SetFaceAppearance",
	"ctx:The group's own":                    "View.SetFaceAppearance",
	"ctx:Measure Selection":                  "Analyze.Measure",
	"ctx:Edit Value...":                      "Constraint.Set Constraint.SetExpr",
	"ctx:Group":                              "- a submenu of the group's own entries below",
	"ctx:Activate":                           "Doc.Activate",
	"ctx:Hide":                               "Group.SetVisible",
	"ctx:Show":                               "Group.SetVisible",
	"ctx:Suppress with What Uses It":         "Group.SetSuppress",
	"ctx:Bring Back":                         "Group.SetSuppress",
	"ctx:Delete Group":                       "Group.Delete",

	// Drags (mouse.go).
	"dragPoints":        "Entity.MoveTo Entity.MoveBy",
	"dragMarquee":       "- a selection made with the mouse; a script names the entities, Edit.Select",
	"dragConstraint":    "Constraint.SetLabelOffset",
	"dragNewPoint":      "Sketch.Point Sketch.Line Sketch.Rect Sketch.Circle",
	"dragNewCubicPoint": "Sketch.Cubic",
	"dragNewArcPoint":   "Sketch.Arc",
	"dragRadius":        "Entity.SetRadius",
	"dragNormal":        "Entity.SetOrientation LinkGroup.SetOrientation",
}

func TestEveryWindowActionHasAScriptCall(t *testing.T) {
	t.Parallel()
	w := scanUI(t)
	if len(w.screens) < 100 || len(w.edits) < 60 || len(w.context) < 30 || len(w.drags) < 6 {
		t.Fatalf("found only %d link handlers, %d edit kinds, %d context entries, %d drags", len(w.screens), len(w.edits), len(w.context), len(w.drags))
	}
	want := map[string]bool{}
	for name, letters := range w.screens {
		if len(letters) == 0 {
			want[name] = true
		}
		for _, l := range letters {
			want[name+"/"+l] = true
		}
	}
	for _, e := range w.edits {
		want[e] = true
	}
	for c := range w.context {
		want["ctx:"+c] = true
	}
	for d := range w.drags {
		want[d] = true
	}

	for key := range want {
		if _, ok := windowActions[key]; !ok {
			t.Errorf("the window's %s has no script call and no reason in windowActions", key)
		}
	}
	for key, v := range windowActions {
		if !want[key] {
			// ScreenChangeParameter and ScreenChangeConfiguration test
			// the letter of an add link before their switch.
			if fn, _, ok := strings.Cut(key, "/"); !ok || len(w.screens[fn]) == 0 {
				t.Errorf("windowActions names %s, which the window doesn't have", key)
			}
		}
		if strings.HasPrefix(v, "GAP:") {
			t.Logf("open gap: %s: %s", key, v)
		}
		if isReason(v) {
			continue
		}
		for call := range strings.FieldsSeq(v) {
			if !hasMethod(call) {
				t.Errorf("windowActions[%s] names %s, which doesn't exist", key, call)
			}
		}
	}
}

// objectMethods says, for each exported method of the object types of the
// API (those apiTypes lists, but the menus' namespaces, which MenuCalls
// holds), the action of the window it matches: tokens apart by ";" of
// a link handler or an edit box (the keys of windowActions), "menu:" and a
// menu path, "ctx:" and a context-menu entry, or a drag. "-q" stands for
// "a query: the window shows the value", "-raw" for a script-only escape
// hatch; any other "- " gives its own reason; "GAP:" marks a model edit
// of the window that the script cannot make.
var objectMethods = map[string]string{
	"ExportOptions": `
Quality  -q
`,
	"Doc": `
Dimension      -q
Describe       -q
Host           -raw
With           -raw
Batch          -script-only: groups many calls into one step; the window's steps are its own
Groups         -q
Active         -q
Activate       ScreenActivateGroup; ctx:Activate
Group          -q
Request        -q
Constraint     -q
Entity         -q
Workplane      -q
Origin         -q
String         -q
Styles         -q
Style          -q
ResetStyles    ScreenLoadFactoryDefaultStyles
NewStyle       ScreenCreateCustomStyle; ctx:Newly Created Custom Style...
Script         menu:File/Export/Script (.go)...
`,
	"Group": `
Appearance            -q
SetAppearance         ScreenChangeAppearance
Describe              -q
String                -q
Valid                 -q
Name                  -q
SetName               ScreenChangeGroupName; editGroupName
Type                  -q
Kind                  -q
Solved                -q
Dof                   -q
Requests              -q
Entities              -q
Made                  -q
Constraints           -q
Param                 -raw
Workplane             -q
Visible               -q
SetVisible            ScreenToggleGroupShown; ScreenChangeGroupOption/v; ScreenShowGroupsSpecial/s; ctx:Hide; ctx:Show
RelaxConstraints      -q
SetRelaxConstraints   ScreenChangeGroupOption/r
AllowRedundant        -q
SetAllowRedundant     ScreenChangeGroupOption/e; ScreenAllowRedundant
SuppressDof           -q
SetSuppressDof        ScreenChangeGroupOption/D
AllDimsReference      -q
SetAllDimsReference   ScreenChangeGroupOption/d
ForceToMesh           -q
SetForceToMesh        ScreenChangeGroupOption/f
SetSuppress           ScreenChangeGroupOption/P; ScreenChangeGroupOption/A; ctx:Suppress with What Uses It; ctx:Bring Back
Suppression           -q
LeftOut               -q
Combine               -q
SetCombine            ScreenChangeGroupOption/c
Color                 -q
SetColor              ScreenColor; editGroupColor
Opacity               -q
SetOpacity            ScreenOpacity; editGroupOpacity
Density               -q
SetDensity            ScreenDensity; editGroupDensity
Delete                ScreenDeleteGroup; ctx:Delete Group; menu:Edit/Delete
Face                  -q
Conflicts             ScreenHowGroupSolved
History               -an accessor for GroupHistory
Analyze               -an accessor for GroupAnalysis
AsExtrude             -an accessor for the typed group
AsSweep               -an accessor for the typed group
AsRepeat              -an accessor for the typed group
AsMirror              -an accessor for the typed group
AsChamfer             -an accessor for the typed group
AsFillet              -an accessor for the typed group
AsShell               -an accessor for the typed group
AsWrap                -an accessor for the typed group
AsLoft                -an accessor for the typed group
AsLink                -an accessor for the typed group
AsRevolve             -an accessor for the typed group
AsThread              -an accessor for the typed group
IsThread              -q
ThreadOf              -q
SetExpr               ScreenChangeExprA; ScreenChangeGroupScale; ScreenChangeHelixPitch; ScreenChangeGroupDraft; ScreenChangeLoftStart; ScreenChangePitchOption; ScreenChangeGroupExtent
Expr                  -q
Value                 -q
Faces                 -q
Problem               -q
Edges                 -q
Solid                 -q
SolidEdges            -q
TwoSided              -q
SetTwoSided           ScreenChangeGroupOption/s; ScreenChangeGroupOption/S
`,
	"GroupHistory": `
DependsOn               menu:Analyze/Group Dependencies
UsedBy                  menu:Analyze/Group Dependencies
OwnSources              menu:Analyze/Group Dependencies
MoveBefore              ScreenMoveGroup; menu:Features/Move Group Earlier; menu:Features/Move Group Later
MoveToEnd               menu:Features/Move Group to End
MoveToEndWithSources    menu:Features/Move Group to End
MoveWithSourcesBefore   ScreenMoveGroup
`,
	"GroupAnalysis": `
Area            menu:Analyze/Measure Area
CenterOfMass    menu:Analyze/Show Center of Mass
Mesh            -script-only: the triangles of the group's solid
Shell           -script-only: the group's shell
Volume          menu:Analyze/Measure Volume
MassProperties  menu:Analyze/Measure Volume
Triangles       -script-only: the triangles of the group's solid
Bounds          menu:Analyze/Group Dependencies
Check           menu:Analyze/Check Solid
RayCheck        menu:Analyze/Ray-Cast Check
WallThickness   menu:Analyze/Clearance and Walls...
Clearances      menu:Analyze/Clearance and Walls...
ClearancesWithin menu:Analyze/Clearance and Walls...
Probe           -script-only: a ray cast at the group's solid
`,
	"Entity": `
String           -q
Valid            -q
Kind             -q
Type             -q
Group            -q
Request          -q
Pos              -q
MoveTo           dragPoints; ctx:Snap to Grid
MoveBy           dragPoints
Points           -q
Part             -q
Overlap          -q
Start            -q
End              -q
Center           -q
Normal           -q
Radius           -q
SetRadius        dragRadius
Orientation      -q
SetOrientation   dragNormal
Length           -q
Dir              -q
FaceNormal       -q
FacePoint        -q
Visible          -q
IsCoincident     -q
Construction     -q
SetConstruction  ctx:Toggle Construction; menu:Sketch/Toggle Construction
Delete           menu:Edit/Delete; ctx:Delete
AddSplinePoint   ctx:Add Spline Point
MakePeriodic     -the mouse closes a spline by ending it on its first point (mouse.go), which no menu or link does
RemoveSplinePoint ctx:Remove Spline Point
DeleteCoincidences ctx:Delete Point-Coincident Constraint
Free             -q
Text             -q
SetText          ScreenEditTtfText; editTTFText
Font             -q
SetFont          ScreenSetTtfFont
Kerning          -q
SetKerning       ScreenToggleTtfKerning
`,
	"Request": `
String           -q
Valid            -q
Type             -q
Group            -q
Entity           -q
Construction     -q
SetConstruction  ctx:Toggle Construction; menu:Sketch/Toggle Construction
Style            -q
SetStyle         ScreenAssignSelectionToStyle; ctx:Assign to Style
Delete           menu:Edit/Delete; ctx:Delete
`,
	"Param": `
Valid   -raw
Val     -raw
Set     -raw
String  -raw
`,
	"Constraint": `
String          -q
Valid           -q
Type            -q
Group           -q
Value           -q
Set             ctx:Edit Value...; ScreenStepDimGo; menu:Analyze/Step Dimension...
SetExpr         ctx:Edit Value...
Expr            -q
Style           -q
SetStyle        ScreenAssignSelectionToStyle; ctx:Assign to Style
Name            -q
SetName         ctx:Edit Value...
Reference       -q
SetReference    ScreenConstraintToggleReference; ctx:Toggle Reference Dimension; menu:Constrain/Toggle Reference Dim
Other           -q
SetOther        ctx:Other Supplementary Angle; menu:Constrain/Other Supplementary Angle
SetShowAsRadius ScreenConstraintShowAsRadius
LabelOffset     -q
SetLabelOffset  dragConstraint
Comment         -q
SetComment      ctx:Edit Value...
Delete          menu:Edit/Delete; ctx:Delete
SetLimit        menu:Constrain/Minimum Dimension; menu:Constrain/Maximum Dimension
Limit           -q
`,
	"Style": `
String          -q
Valid           -q
Custom          -q
Name            -q
SetName         ScreenChangeStyleName; editStyleName
Color           -q
SetColor        ScreenChangeStyleColor/s; editStyleColor
FillColor       -q
SetFillColor    ScreenChangeStyleColor/f; editStyleFillColor
Filled          -q
SetFilled       ScreenChangeStyleYesNo/f
Width           -q
SetWidth        ScreenChangeStyleMetric/w; editStyleWidth
WidthAs         -q
SetWidthAs      ScreenChangeStyleYesNo/w; ScreenChangeStyleYesNo/W
Stipple         -q
SetStipple      ScreenChangeStylePatternType
StippleScale    -q
SetStippleScale ScreenChangeStyleMetric/s; editStyleStipplePeriod
TextHeight      -q
SetTextHeight   ScreenChangeStyleMetric/t; editStyleTextHeight
TextHeightAs    -q
SetTextHeightAs ScreenChangeStyleYesNo/g
TextAlign       -q
SetTextAlign    ScreenChangeStyleYesNo/L
TextAngle       -q
SetTextAngle    ScreenChangeStyleTextAngle; editStyleTextAngle
ZIndex          -q
SetZIndex       ScreenChangeStyleZIndex; editStyleZIndex
Visible         -q
SetVisible      ScreenChangeStyleYesNo/v
Exportable      -q
SetExportable   ScreenChangeStyleYesNo/e
Requests        -q
Constraints     -q
Delete          ScreenDeleteStyle
`,
	"ExtrudeGroup": `
Skewed           -q
SetSkewed        ScreenChangeGroupOption/w
Draft            -q
SetDraft         ScreenChangeGroupDraft; editGroupDraft
DraftFaces       -q
SetDraftFaces    ScreenDraftAllSides; ctx:Draft Selected Sides; menu:Features/Draft Selected Sides
DraftNeutral     -q
SetDraftNeutral  -script-only: the window shows no row for the neutral plane of a draft
Depth            -q
DepthHeld        -q
SetDepth         ScreenChangeExtentHeld; ScreenChangeGroupExtent; editGroupExtent
FreeDepth        ScreenChangeExtentHeld
`,
	"ThreadGroup": `
Options  -q
Groups   -q
Finished -q
Edit     ScreenEditThread; ScreenMakeThread
Group    -q
Visible  -q
SetVisible  ScreenThreadAction/v
Suppressed  -q
SetSuppressed  ScreenThreadAction/s
MoveBefore  ScreenThreadAction/u; ScreenThreadAction/d
Delete   ScreenThreadAction/x
`,
	"RevolveGroup": `
Angle            -q
AngleHeld        -q
SetAngle         ScreenChangeExtentHeld; ScreenChangeGroupExtent; editGroupExtent
FreeAngle        ScreenChangeExtentHeld
`,
	"SweepGroup": `
Twist            -q
SetTwist         ScreenChangeGroupDraft; editGroupDraft
CornerRadius     -q
SetCornerRadius  ScreenChangeHelixPitch; editHelixPitch
SweepUp          -q
SetSweepUp       ScreenChangeSweepUp/x; ScreenChangeSweepUp/y; ScreenChangeSweepUp/z
`,
	"RepeatGroup": `
Copies        -q
SkipFirst     -q
SetSkipFirst  ScreenChangeGroupOption/k; ScreenChangeGroupOption/K
SetCopies     ScreenChangeExprA; editTimesRepeated
`,
	"MirrorGroup": `
KeepOriginal     -q
SetKeepOriginal  ScreenChangeGroupOption/k; ScreenChangeGroupOption/K
MirrorSolid      -q
SetMirrorSolid   ScreenChangeGroupOption/m
Plane            -q
`,
	"ChamferGroup": `
Distance           -q
SetDistance        ScreenChangeHelixPitch; editHelixPitch
SecondDistance     -q
SetSecondDistance  ScreenChangeChamferDistance2; editChamferDistance2; ScreenSameChamferDistances
SwapDistances      ScreenSwapChamferDistances
`,
	"FilletGroup": `
Radius               -q
SetRadius            ScreenChangeHelixPitch; editHelixPitch
EdgeRadius           -q
SetEdgeRadius        ScreenChangeEdgeRadius; editEdgeRadius
SetEdgeRadiusExpr    ScreenChangeEdgeRadius; ScreenResetEdgeRadius
`,
	"ShellGroup": `
Thickness     -q
SetThickness  ScreenChangeHelixPitch; editHelixPitch
`,
	"WrapGroup": `
Height     -q
SetHeight  ScreenChangeHelixPitch; editHelixPitch
Engraved   -q
`,
	"LoftGroup": `
Profiles     -q
Turn         -q
SetTurn      ScreenChangeLoftTurn
Tangency     -q
SetTangency  ScreenChangeLoftStart; ScreenChangeGroupDraft
AlongSides     -q
SetAlongSides  ScreenLoftAlongSides
Guides       -q
SetGuides    ScreenLoftNoGuides; menu:Features/Guide Loft by Selected Curves
Closed       -q
SetClosed    ScreenLoftClosed
ProfileTangency     -q
SetProfileTangency  ScreenChangeLoftProfileTangency; editLoftProfileTangency
`,
	"LinkGroup": `
AsSolid         -q
SetAsSolid      ScreenChangeLinkAsSolid
File            -q
SetFile         ScreenRelink
Configuration   -q
SetConfiguration  ScreenChangeLinkConfiguration
Param           -q
SetParam        ScreenChangeLinkParam; editLinkParam
ModelParams     -q
ModelConfigurations  -q
Info            -q
Problem         -q
Offset          -q
Move            menu:Edit/Rotate Imported 90°; dragPoints
Orientation     -q
SetOrientation  dragNormal
Rotate          menu:Edit/Rotate Imported 90°
Rotate90        menu:Edit/Rotate Imported 90°
`,
	"Solid": `
Faces          -q
Edges          -q
Vertices       -q
Edge           -q
Face           -q
EdgeAt         -q
FaceAt         -q
EdgesBetween   -q
EdgesAlong     -q
Problems       -q
Euler          -q
`,
	"SolidFace": `
Patches     -q
Name        -q
ID          -q
Entity      -q
Edges       -q
Neighbours  -q
`,
	"SolidEdge": `
Name      -q
Describe  -q
Faces     -q
Ends      -q
Points    -q
Length    -q
Pieces    -q
Straight  -q
Smooth    -q
`,
	"SolidVertex": `
Pos    -q
Edges  -q
`,
}

// aliases are the short forms of a reason in objectMethods.
var aliases = map[string]string{
	"-q":   "- a query: the window shows the value, a script reads it",
	"-raw": "- script-only by design: a raw escape hatch the window has no row for",
}

// objectActions parses objectMethods into "Type.Method" -> value.
func objectActions(t *testing.T) map[string]string {
	t.Helper()
	out := map[string]string{}
	for typ, body := range objectMethods {
		for line := range strings.SplitSeq(body, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			name, v, ok := strings.Cut(line, " ")
			v = strings.TrimSpace(v)
			if !ok || v == "" {
				t.Errorf("objectMethods[%s]: %q has no window action or reason", typ, line)
				continue
			}
			if a, ok := aliases[v]; ok {
				v = a
			} else if rest, ok := strings.CutPrefix(v, "-"); ok {
				v = "- " + rest
			}
			key := typ + "." + name
			if _, dup := out[key]; dup {
				t.Errorf("objectMethods names %s twice", key)
			}
			out[key] = v
		}
	}
	return out
}

func TestEveryObjectMethodHasAWindowAction(t *testing.T) {
	t.Parallel()
	actions := objectActions(t)
	w := scanUI(t)
	menuPaths := map[string]bool{}
	for _, item := range ui.MenuItems() {
		menuPaths[strings.Join(item.Path, "/")] = true
	}
	// Every method of every object type.
	seen := map[string]bool{}
	for name, typ := range apiTypes {
		if isMenuType(name) {
			continue
		}
		for m := range typ.Methods() {
			key := name + "." + m.Name
			seen[key] = true
			if _, ok := actions[key]; !ok {
				t.Errorf("%s has no window action in objectMethods; name it, or say why there is none", key)
			}
		}
	}
	for key, v := range actions {
		if !seen[key] {
			t.Errorf("objectMethods names %s, which doesn't exist", key)
		}
		if isReason(v) {
			continue
		}
		for tok := range strings.SplitSeq(v, ";") {
			tok = strings.TrimSpace(tok)
			switch {
			case strings.HasPrefix(tok, "menu:"):
				if !menuPaths[strings.TrimPrefix(tok, "menu:")] {
					t.Errorf("objectMethods[%s]: %s is no menu entry", key, tok)
				}
			case strings.HasPrefix(tok, "ctx:"):
				if !w.context[strings.TrimPrefix(tok, "ctx:")] {
					t.Errorf("objectMethods[%s]: %s is no context-menu entry", key, tok)
				}
			case strings.HasPrefix(tok, "Screen"), strings.HasPrefix(tok, "edit"), strings.HasPrefix(tok, "drag"):
				if _, ok := windowActions[tok]; !ok {
					t.Errorf("objectMethods[%s]: %s is no link, edit box or drag of the window", key, tok)
				}
			default:
				t.Errorf("objectMethods[%s]: %q is neither a window action nor a reason", key, tok)
			}
		}
	}
	// A window action that names a script call and the call that
	// names it back must agree on its existence only (above); the
	// tables are two views of one pairing, kept by hand.
}

// isMenuType is true for the namespaces of Doc's menu fields, which
// TestEveryMenuCallHasAnEntry holds to the window's menu.
func isMenuType(name string) bool {
	f, ok := reflect.TypeFor[api.Doc]().FieldByName(name)
	return ok && f.IsExported()
}
