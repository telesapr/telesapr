package api

import (
	"slices"
	"strings"
)

// MenuCalls says, for each entry of the window's main menu, by its path
// ("Features/Extrude"), the calls a script makes instead, as help()
// names them ("Features.Extrude"; several separated by spaces), or,
// after "- ", why the entry has none: most of those are the window's
// own, its view and its panels, which a script has none of. A test of
// the package holds it to the menu, both ways.
var MenuCalls = map[string]string{
	"File/New":         "- a script works on the drawing it is given; File.Load replaces it",
	"File/Open...":     "File.Load",
	"File/Open Recent": "- the window's list of files; File.Load takes any file",
	"File/Save":        "File.Save",
	"File/Save As...":  "File.Save",
	// A SolveSpace model (.slvs) is converted by api.Import, a function of the
	// package rather than of the document, which it opens.
	"File/Import...":              "File.ImportDXF",
	"File/Export/Image (.png)...": "File.Render",
	"File/Export/2d View (.pdf, .eps, .svg, .dxf, .plt, .ngc)...":       "File.Export File.ExportAs",
	"File/Export/2d Section (.pdf, .eps, .svg, .dxf, .plt, .ngc)...":    "File.ExportSection File.ExportAs",
	"File/Export/3d Wireframe (.step, .dxf)...":                         "File.Export File.ExportAs",
	"File/Export/Triangle Mesh (.stl, .3mf, .obj, .html, .js, .wrl)...": "File.Export File.ExportAs File.SetMeshQuality",
	"File/Export/Surfaces (.step)...":                                   "File.Export File.ExportAs",
	"File/Export/Drawing (.pdf, .eps, .svg, .dxf, .plt, .ngc)...":       "Drawing.Export",
	"File/Exit": "- the window's",

	"Edit/Undo":                   "Edit.Undo",
	"Edit/Redo":                   "Edit.Redo",
	"Edit/Regenerate All":         "Edit.Regen",
	"Edit/Cut":                    "Edit.Copy Entity.Delete",
	"Edit/Copy":                   "Edit.Copy",
	"Edit/Paste":                  "Edit.Paste",
	"Edit/Paste Transformed...":   "Edit.Paste",
	"Edit/Delete":                 "Entity.Delete Constraint.Delete Group.Delete",
	"Edit/Select Edge Chain":      "- a selection made with the mouse; a script names the entities, Edit.Select",
	"Edit/Select All":             "Edit.Select",
	"Edit/Unselect All":           "Edit.Select",
	"Edit/Snap Selection to Grid": "- the grid is the view's; Entity.MoveTo puts a point where it is asked",
	"Edit/Rotate Imported 90°":    "LinkGroup.Rotate90 LinkGroup.Rotate LinkGroup.SetOrientation LinkGroup.Move",
	"Edit/Find Command...":        "Edit.RunCommand",
	"Edit/Parameters...":          "Params.List Params.Set",
	"Edit/Line Styles...": "Doc.Styles Doc.Style Doc.NewStyle Style.SetName Style.Delete " +
		"Style.SetColor Style.SetFillColor Style.SetFilled Style.SetWidth Style.SetWidthAs " +
		"Style.SetStipple Style.SetStippleScale Style.SetTextHeight Style.SetTextHeightAs Style.SetTextAlign Style.SetTextAngle Doc.ResetStyles " +
		"Style.SetZIndex Style.SetVisible Style.SetExportable Request.SetStyle Constraint.SetStyle",
	"Edit/View Projection...": "- the window's view; File.Render takes its own",
	"Edit/Preferences...":     "Edit.SetChordTolerance Edit.SetMaxSegments Edit.SetCheckClosedContour File.ExportOptions File.SetExportOptions View.SetUnits",

	"View/Zoom In":                                       "- the window's view",
	"View/Zoom Out":                                      "- the window's view",
	"View/Zoom To Fit":                                   "- the window's view; File.Render fits the model",
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
	"View/Show/Reference Dimensions":                     "- what the window shows",
	"View/Show/Comments":                                 "- what the window shows",
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
	"View/Section View":                                  "View.SetSection View.ClearSection",
	"View/Show Drawing Sheet":                            "Drawing.Show",
	"View/Drawing Sheet...":                              "Drawing.Set Drawing.Get",
	"View/Dimension Units/Dimensions in Millimeters":     "View.SetUnits",
	"View/Dimension Units/Dimensions in Meters":          "View.SetUnits",
	"View/Dimension Units/Dimensions in Inches":          "View.SetUnits",
	"View/Dimension Units/Dimensions in Feet and Inches": "View.SetUnits",
	"View/Show Toolbar":                                  "- the window's",
	"View/Show Property Browser":                         "- the window's",
	"View/Full Screen":                                   "- the window's",

	"Sketch/In Workplane":                 "Sketch.InWorkplane",
	"Sketch/Anywhere In 3d":               "Sketch.Anywhere3D",
	"Sketch/Datum Point":                  "Sketch.Point",
	"Sketch/Workplane":                    "Sketch.NewWorkplane",
	"Sketch/Line Segment":                 "Sketch.Line Sketch.Polyline",
	"Sketch/Construction Line Segment":    "Sketch.Line Entity.SetConstruction",
	"Sketch/Rectangle":                    "Sketch.Rect",
	"Sketch/Circle":                       "Sketch.Circle",
	"Sketch/Arc of a Circle":              "Sketch.Arc",
	"Sketch/Bezier Cubic Spline":          "Sketch.Cubic",
	"Sketch/Text in TrueType Font":        "Sketch.Text",
	"Sketch/Image":                        "Sketch.Image",
	"Sketch/Toggle Construction":          "Entity.SetConstruction Request.SetConstruction",
	"Sketch/Tangent Arc at Point":         "Sketch.TangentArc",
	"Sketch/Split Curves at Intersection": "Sketch.Split",

	"Constrain/Distance / Diameter":       "Constrain.Distance Constrain.ProjectedDistance Constrain.Length Constrain.Diameter Constrain.Radius",
	"Constrain/Reference Dimension":       "Constraint.SetReference",
	"Constrain/Minimum Dimension":         "Constrain.MinDistance Constraint.SetLimit",
	"Constrain/Maximum Dimension":         "Constrain.MaxDistance Constraint.SetLimit",
	"Constrain/Angle / Equal Angle":       "Constrain.Angle Constrain.EqualAngle",
	"Constrain/Reference Angle":           "Constraint.SetReference",
	"Constrain/Other Supplementary Angle": "Constraint.SetOther",
	"Constrain/Toggle Reference Dim":      "Constraint.SetReference",
	"Constrain/Horizontal":                "Constrain.Horizontal",
	"Constrain/Vertical":                  "Constrain.Vertical",
	"Constrain/On Point / Curve / Plane":  "Constrain.On Constrain.Coincident",
	"Constrain/On Segment":                "Constrain.OnSegment",
	"Constrain/Equal Length / Radius":     "Constrain.Equal",
	"Constrain/Length / Arc Ratio":        "Constrain.Ratio",
	"Constrain/Length / Arc Difference":   "Constrain.Difference",
	"Constrain/At Midpoint":               "Constrain.Midpoint",
	"Constrain/Symmetric":                 "Constrain.Symmetric",
	"Constrain/Parallel / Tangent":        "Constrain.Parallel Constrain.Tangent",
	"Constrain/Perpendicular":             "Constrain.Perpendicular",
	"Constrain/Same Orientation":          "Constrain.SameOrientation",
	"Constrain/Lock Point Where Dragged":  "Constrain.Lock",
	"Constrain/Comment":                   "Constrain.Comment",

	"Features/Sketch In 3d":                  "Features.Sketch3D",
	"Features/Sketch In New Workplane":       "Features.SketchIn Features.SketchAt Features.SketchOnLines Features.SketchOnNormal",
	"Features/Sketch On Face":                "Features.SketchOnFace",
	"Features/Step Translating":              "Features.Translate",
	"Features/Step Rotating":                 "Features.Rotate",
	"Features/Extrude":                       "Features.Extrude",
	"Features/Helix":                         "Features.Helix Features.HelixAbout",
	"Features/Lathe":                         "Features.Lathe Features.LatheAbout",
	"Features/Revolve":                       "Features.Revolve Features.RevolveAbout",
	"Features/Screw Thread...":               "Features.Thread",
	"Features/Sweep Along Path":              "Features.Sweep",
	"Features/Mirror":                        "Features.Mirror",
	"Features/Chamfer Edges":                 "Features.Chamfer",
	"Features/Fillet Edges":                  "Features.Fillet",
	"Features/Loft To Selected Profile":      "Features.Loft",
	"Features/Shell Solid":                   "Features.Shell",
	"Features/Wrap Sketch Onto Face":         "Features.Emboss",
	"Features/Pattern Along Path":            "Features.PatternAlongPath",
	"Features/Draft Selected Sides":          "ExtrudeGroup.SetDraftFaces",
	"Features/Guide Loft by Selected Curves": "LoftGroup.SetGuides",
	"Features/Move Group Earlier":            "GroupHistory.MoveBefore",
	"Features/Move Group Later":              "GroupHistory.MoveBefore",
	"Features/Move Group to End":             "GroupHistory.MoveToEnd GroupHistory.MoveToEndWithSources",
	"Features/Broken References...":          "Features.BrokenReferences Features.ClearBrokenReferences",

	"Assembly/Link / Assemble...":     "Assembly.Link",
	"Assembly/Link Recent":            "- links a file from the recent list; Assembly.Link takes any file",
	"Assembly/Show Interfering Parts": "Assembly.Interferences",
	"Assembly/Bill of Materials":      "Assembly.BillOfMaterials Assembly.ExportBOM",
	"Assembly/Motion Study...":        "Assembly.MotionStudy",
	"Assembly/Mate Concentric":        "Assembly.Concentric",
	"Assembly/Mate Flush":             "Assembly.Flush",
	"Assembly/Mate Hinge":             "Assembly.Hinge",

	"Analyze/Measure Selection":            "Analyze.Measure",
	"Analyze/Measure Volume":               "GroupAnalysis.Volume",
	"Analyze/Measure Area":                 "GroupAnalysis.Area Analyze.FacesArea",
	"Analyze/Measure Perimeter":            "Analyze.CurveLength",
	"Analyze/Show Center of Mass":          "GroupAnalysis.CenterOfMass",
	"Analyze/Check Solid":                  "GroupAnalysis.Check",
	"Analyze/Group Dependencies":           "GroupHistory.DependsOn GroupHistory.UsedBy GroupHistory.OwnSources",
	"Analyze/Ray-Cast Check":               "GroupAnalysis.RayCheck GroupAnalysis.Probe",
	"Analyze/Show Naked Edges":             "GroupAnalysis.Check",
	"Analyze/Show Underconstrained Points": "Entity.Free Group.Dof",
	"Analyze/Trace Point":                  "- draws a point's path in the window; Assembly.MotionStudy reports positions",
	"Analyze/Stop Tracing...":              "- draws a point's path in the window; Assembly.MotionStudy reports positions",
	"Analyze/Step Dimension...":            "Constraint.Set",

	"Script/Show Console":         "- the window's console",
	"Script/Run File...":          "- a script runs from telesapr-cli run or telesapr --script",
	"Script/Stop":                 "- the window's console",
	"Script/Allow AI Connections": "- the window's setting; telesapr-cli mcp serves a drawing without one",

	"Help/User's Guide": "- the window's",
	"Help/Go to GitHub": "- the window's",
	"Help/About":        "- the window's",
}

// menuPath is a menu entry's path as MenuCalls keys it, from one
// written with " > ", "›" or "/" between the menus, with or without
// the "..." of an entry that opens a dialog.
func menuPath(entry string) (string, bool) {
	parts := strings.FieldsFunc(entry, func(r rune) bool { return r == '/' || r == '>' || r == '›' })
	for i, p := range parts {
		parts[i] = strings.TrimSpace(p)
	}
	path := strings.Join(parts, "/")
	for _, key := range []string{path, path + "...", strings.TrimSuffix(path, "...")} {
		if _, ok := MenuCalls[key]; ok {
			return key, true
		}
	}
	for key := range MenuCalls {
		if strings.EqualFold(key, path) || strings.EqualFold(strings.TrimSuffix(key, "..."), path) {
			return key, true
		}
	}
	return "", false
}

// MenuEntryCalls is what a script calls in place of a menu entry,
// written "Features > Fillet Edges" or "Features/Fillet Edges": the
// calls, or why there are none. It reports false when no entry has
// that path.
func MenuEntryCalls(entry string) (path string, calls []string, why string, ok bool) {
	path, ok = menuPath(entry)
	if !ok {
		return "", nil, "", false
	}
	v := MenuCalls[path]
	if r, none := strings.CutPrefix(v, "- "); none {
		return path, nil, r, true
	}
	return path, strings.Fields(v), "", true
}

// MenuEntriesOf lists the menu entries a call stands for, as
// "Features > Extrude", in order.
func MenuEntriesOf(call string) []string {
	var out []string
	for path, v := range MenuCalls {
		if strings.HasPrefix(v, "- ") {
			continue
		}
		if slices.Contains(strings.Fields(v), call) {
			out = append(out, strings.ReplaceAll(path, "/", " > "))
		}
	}
	slices.Sort(out)
	return out
}
