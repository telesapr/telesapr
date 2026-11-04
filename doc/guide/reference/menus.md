<!-- Written by TestReferencePages (pkg/ui/docref_test.go) from the program; regenerate with scripts/regen-golden.sh docref, don't edit. -->

# Menu reference

Every entry of the main menu, its keys, and what a script calls to do the same (see [Scripts](../scripts.md)). An entry marked *on/off* is ticked while it is on; one marked *choice* is one of a set. Edit > Find Command (<kbd>Ctrl</kbd>+<kbd>K</kbd>) finds an entry by words of its name.

## File

| Entry | Keys | In a script |
|---|---|---|
| New | <kbd>Ctrl</kbd>+<kbd>N</kbd> | none: a script works on the drawing it is given; File.Load replaces it |
| Open... | <kbd>Ctrl</kbd>+<kbd>O</kbd> | `File.Load` |
| Save | <kbd>Ctrl</kbd>+<kbd>S</kbd> | `File.Save` |
| Save As... | <kbd>Ctrl</kbd>+<kbd>Shift</kbd>+<kbd>S</kbd> | `File.Save` |
| Import... |  | `File.ImportDXF` |
| Export > Image (.png)... |  | `File.Render` |
| Export > 2d View (.pdf, .eps, .svg, .dxf, .plt, .ngc)... |  | `File.Export`, `File.ExportAs` |
| Export > 2d Section (.pdf, .eps, .svg, .dxf, .plt, .ngc)... |  | `File.ExportSection`, `File.ExportAs` |
| Export > 3d Wireframe (.step, .dxf)... |  | `File.Export`, `File.ExportAs` |
| Export > Triangle Mesh (.stl, .3mf, .glb, .obj, .html, .js, .wrl)... |  | `File.Export`, `File.ExportAs`, `File.SetMeshQuality` |
| Export > Surfaces (.step)... |  | `File.Export`, `File.ExportAs` |
| Export > Script (.go)... |  | `File.ExportScript`, `Doc.Script` |
| Export > Drawing (.pdf, .eps, .svg, .dxf, .plt, .ngc)... |  | `Drawing.Export` |
| Exit | <kbd>Ctrl</kbd>+<kbd>Q</kbd> | none: the window's |

## Edit

| Entry | Keys | In a script |
|---|---|---|
| Undo | <kbd>Ctrl</kbd>+<kbd>Z</kbd> | `Edit.Undo` |
| Redo | <kbd>Ctrl</kbd>+<kbd>Y</kbd> | `Edit.Redo` |
| Regenerate All | <kbd>Space</kbd> | `Edit.Regen` |
| Cut | <kbd>Ctrl</kbd>+<kbd>X</kbd> | `Edit.Copy`, `Entity.Delete` |
| Copy | <kbd>Ctrl</kbd>+<kbd>C</kbd> | `Edit.Copy` |
| Paste | <kbd>Ctrl</kbd>+<kbd>V</kbd> | `Edit.Paste` |
| Paste Transformed... | <kbd>Ctrl</kbd>+<kbd>T</kbd> | `Edit.Paste` |
| Delete | <kbd>Del</kbd> | `Entity.Delete`, `Constraint.Delete`, `Group.Delete` |
| Select Edge Chain | <kbd>Ctrl</kbd>+<kbd>E</kbd> | none: a selection made with the mouse; a script names the entities, Edit.Select |
| Select All | <kbd>Ctrl</kbd>+<kbd>A</kbd> | `Edit.Select` |
| Unselect All | <kbd>Esc</kbd> | `Edit.Select` |
| Snap Selection to Grid | <kbd>.</kbd> | none: the grid is the view's; Entity.MoveTo puts a point where it is asked |
| Rotate Imported 90° | <kbd>9</kbd> | `LinkGroup.Rotate90`, `LinkGroup.Rotate`, `LinkGroup.SetOrientation`, `LinkGroup.Move` |
| Find Command... | <kbd>Ctrl</kbd>+<kbd>K</kbd> | `Edit.RunCommand` |
| Parameters... | <kbd>Ctrl</kbd>+<kbd>Shift</kbd>+<kbd>M</kbd> | `Params.List`, `Params.Set` |
| Datums... |  | `Datums.List`, `Datums.Names`, `Datums.Name`, `Datums.NameFace`, `Datums.NameEdge`, `Datums.NameFaceAt`, `Datums.NameEdgeAt`, `Datums.Delete`, `Datums.Get` |
| Line Styles... |  | `Doc.Styles`, `Doc.Style`, `Doc.NewStyle`, `Style.SetName`, `Style.Delete`, `Style.SetColor`, `Style.SetFillColor`, `Style.SetFilled`, `Style.SetWidth`, `Style.SetWidthAs`, `Style.SetStipple`, `Style.SetStippleScale`, `Style.SetTextHeight`, `Style.SetTextHeightAs`, `Style.SetTextAlign`, `Style.SetTextAngle`, `Doc.ResetStyles`, `Style.SetZIndex`, `Style.SetVisible`, `Style.SetExportable`, `Request.SetStyle`, `Constraint.SetStyle` |
| View Projection... |  | none: the window's view; File.Render takes its own |
| Preferences... |  | `Edit.SetChordTolerance`, `Edit.SetMaxSegments`, `Edit.SetCheckClosedContour`, `File.ExportOptions`, `File.SetExportOptions`, `View.SetUnits` |

## View

| Entry | Keys | In a script |
|---|---|---|
| Zoom In | <kbd>+</kbd> | none: the window's view |
| Zoom Out | <kbd>-</kbd> | none: the window's view |
| Zoom To Fit | <kbd>F</kbd> | none: the window's view; File.Render fits the model |
| Align View to Workplane | <kbd>W</kbd> | none: the window's view |
| Nearest Ortho View | <kbd>F2</kbd> | none: the window's view |
| Nearest Isometric View | <kbd>F3</kbd> | none: the window's view |
| Center View At Point | <kbd>F4</kbd> | none: the window's view |
| Show > Workplanes of Inactive Groups *(on/off)* |  | none: what the window shows |
| Show > Normals *(on/off)* |  | none: what the window shows |
| Show > Points *(on/off)* |  | none: what the window shows |
| Show > Construction Entities *(on/off)* |  | none: what the window shows |
| Show > Constraints and Dimensions *(choice)* |  | none: what the window shows |
| Show > Dimensions Only *(choice)* |  | none: what the window shows |
| Show > No Constraints *(choice)* |  | none: what the window shows |
| Show > Reference Dimensions *(on/off)* |  | none: what the window shows |
| Show > Comments *(on/off)* |  | none: what the window shows |
| Show > Shaded Solid *(on/off)* |  | none: what the window shows |
| Show > Selectable Faces *(on/off)* |  | none: what the window shows |
| Show > Solid Edges *(on/off)* |  | none: what the window shows |
| Show > Solid Outline *(on/off)* |  | none: what the window shows |
| Show > Triangle Mesh *(on/off)* |  | none: what the window shows |
| Show > Hidden Lines Invisible *(choice)* |  | none: what the window shows |
| Show > Hidden Lines Stippled *(choice)* |  | none: what the window shows |
| Show > Hidden Lines Visible *(choice)* |  | none: what the window shows |
| Show Snap Grid *(on/off)* | <kbd>></kbd> | none: what the window shows |
| Darken Inactive Solids *(on/off)* |  | none: what the window shows |
| Use Perspective Projection *(on/off)* | <kbd>`</kbd> | none: the window's view |
| Show Exploded View *(on/off)* | <kbd>\</kbd> | none: what the window shows |
| Section View *(on/off)* | <kbd>Ctrl</kbd>+<kbd>Shift</kbd>+<kbd>X</kbd> | `View.SetSection`, `View.ClearSection` |
| Show Drawing Sheet *(on/off)* |  | `Drawing.Show` |
| Drawing Sheet... |  | `Drawing.Set`, `Drawing.Get`, `Drawing.AddSection`, `Drawing.AddDetail`, `Drawing.RemoveSection`, `Drawing.RemoveDetail` |
| Dimension Units > Dimensions in Millimeters *(choice)* |  | `View.SetUnits` |
| Dimension Units > Dimensions in Meters *(choice)* |  | `View.SetUnits` |
| Dimension Units > Dimensions in Inches *(choice)* |  | `View.SetUnits` |
| Dimension Units > Dimensions in Feet and Inches *(choice)* |  | `View.SetUnits` |
| Show Toolbar *(on/off)* | <kbd>Ctrl</kbd>+<kbd>Tab</kbd> | none: the window's |
| Show Property Browser *(on/off)* | <kbd>Tab</kbd> | none: the window's |
| Full Screen *(on/off)* | <kbd>Ctrl</kbd>+<kbd>F11</kbd> | none: the window's |

## Sketch

| Entry | Keys | In a script |
|---|---|---|
| In Workplane *(choice)* | <kbd>2</kbd> | `Sketch.InWorkplane` |
| Anywhere In 3d *(choice)* | <kbd>3</kbd> | `Sketch.Anywhere3D` |
| Datum Point | <kbd>P</kbd> | `Sketch.Point` |
| Workplane |  | `Sketch.NewWorkplane` |
| Line Segment | <kbd>S</kbd> | `Sketch.Line`, `Sketch.Polyline` |
| Construction Line Segment | <kbd>Shift</kbd>+<kbd>S</kbd> | `Sketch.Line`, `Entity.SetConstruction` |
| Rectangle | <kbd>R</kbd> | `Sketch.Rect` |
| Circle | <kbd>C</kbd> | `Sketch.Circle` |
| Arc of a Circle | <kbd>A</kbd> | `Sketch.Arc` |
| Bezier Cubic Spline | <kbd>B</kbd> | `Sketch.Cubic` |
| Text in TrueType Font | <kbd>T</kbd> | `Sketch.Text` |
| Image |  | `Sketch.Image` |
| Toggle Construction | <kbd>G</kbd> | `Entity.SetConstruction`, `Request.SetConstruction` |
| Tangent Arc at Point | <kbd>Shift</kbd>+<kbd>A</kbd> | `Sketch.TangentArc` |
| Split Curves at Intersection | <kbd>I</kbd> | `Sketch.Split` |

## Constrain

| Entry | Keys | In a script |
|---|---|---|
| Distance / Diameter | <kbd>D</kbd> | `Constrain.Distance`, `Constrain.ProjectedDistance`, `Constrain.Length`, `Constrain.Diameter`, `Constrain.Radius` |
| Reference Dimension | <kbd>Shift</kbd>+<kbd>D</kbd> | `Constraint.SetReference` |
| Minimum Dimension |  | `Constrain.MinDistance`, `Constraint.SetLimit` |
| Maximum Dimension |  | `Constrain.MaxDistance`, `Constraint.SetLimit` |
| Angle / Equal Angle | <kbd>N</kbd> | `Constrain.Angle`, `Constrain.EqualAngle` |
| Reference Angle | <kbd>Shift</kbd>+<kbd>N</kbd> | `Constraint.SetReference` |
| Other Supplementary Angle | <kbd>U</kbd> | `Constraint.SetOther` |
| Toggle Reference Dim | <kbd>E</kbd> | `Constraint.SetReference` |
| Horizontal | <kbd>H</kbd> | `Constrain.Horizontal` |
| Vertical | <kbd>V</kbd> | `Constrain.Vertical` |
| On Point / Curve / Plane | <kbd>O</kbd> | `Constrain.On`, `Constrain.Coincident` |
| On Segment |  | `Constrain.OnSegment` |
| Equal Length / Radius | <kbd>Q</kbd> | `Constrain.Equal` |
| Length / Arc Ratio | <kbd>Z</kbd> | `Constrain.Ratio` |
| Length / Arc Difference | <kbd>J</kbd> | `Constrain.Difference` |
| At Midpoint | <kbd>M</kbd> | `Constrain.Midpoint` |
| Symmetric | <kbd>Y</kbd> | `Constrain.Symmetric` |
| Parallel / Tangent | <kbd>L</kbd> | `Constrain.Parallel`, `Constrain.Tangent` |
| Perpendicular | <kbd>[</kbd> | `Constrain.Perpendicular` |
| Same Orientation | <kbd>X</kbd> | `Constrain.SameOrientation` |
| Lock Point Where Dragged | <kbd>]</kbd> | `Constrain.Lock` |
| Comment | <kbd>;</kbd> | `Constrain.Comment` |

## Features

| Entry | Keys | In a script |
|---|---|---|
| Sketch In 3d | <kbd>Shift</kbd>+<kbd>3</kbd> | `Features.Sketch3D` |
| Sketch In New Workplane | <kbd>Shift</kbd>+<kbd>W</kbd> | `Features.SketchIn`, `Features.SketchAt`, `Features.SketchOnLines`, `Features.SketchOnNormal` |
| Sketch On Face | <kbd>Shift</kbd>+<kbd>B</kbd> | `Features.SketchOnFace` |
| Extrude | <kbd>Shift</kbd>+<kbd>X</kbd> | `Features.Extrude` |
| Revolve | <kbd>Shift</kbd>+<kbd>V</kbd> | `Features.Revolve`, `Features.RevolveAbout` |
| Lathe | <kbd>Shift</kbd>+<kbd>L</kbd> | `Features.Lathe`, `Features.LatheAbout` |
| Helix | <kbd>Shift</kbd>+<kbd>H</kbd> | `Features.Helix`, `Features.HelixAbout` |
| Sweep Along Path | <kbd>Shift</kbd>+<kbd>U</kbd> | `Features.Sweep` |
| Loft To Selected Profile | <kbd>Shift</kbd>+<kbd>O</kbd> | `Features.Loft` |
| Guide Loft by Selected Curves |  | `LoftGroup.SetGuides` |
| Screw Thread... | <kbd>Shift</kbd>+<kbd>E</kbd> | `Features.Thread`, `Features.ThreadOnFace`, `Features.ThreadOnEdge` |
| Wrap Sketch Onto Face |  | `Features.Emboss` |
| Fillet Edges | <kbd>Shift</kbd>+<kbd>F</kbd> | `Features.Fillet` |
| Chamfer Edges | <kbd>Shift</kbd>+<kbd>Q</kbd> | `Features.Chamfer` |
| Shell Solid | <kbd>Shift</kbd>+<kbd>G</kbd> | `Features.Shell` |
| Draft Selected Sides |  | `ExtrudeGroup.SetDraftFaces` |
| Mirror | <kbd>Shift</kbd>+<kbd>Y</kbd> | `Features.Mirror` |
| Step Translating | <kbd>Shift</kbd>+<kbd>T</kbd> | `Features.Translate` |
| Step Rotating | <kbd>Shift</kbd>+<kbd>R</kbd> | `Features.Rotate` |
| Pattern Along Path |  | `Features.PatternAlongPath` |
| Move Group Earlier |  | `GroupHistory.MoveBefore` |
| Move Group Later |  | `GroupHistory.MoveBefore` |
| Move Group to End |  | `GroupHistory.MoveToEnd`, `GroupHistory.MoveToEndWithSources` |
| Broken References... |  | `Features.BrokenReferences`, `Features.Mended`, `Features.LeftOut`, `Features.LeftOutMissing`, `Features.MendLeftOut`, `Features.DiscardLeftOut`, `Features.ClearBrokenReferences` |

## Assembly

| Entry | Keys | In a script |
|---|---|---|
| Link / Assemble... | <kbd>Shift</kbd>+<kbd>I</kbd> | `Assembly.Link` |
| Mate Concentric |  | `Assembly.Concentric` |
| Mate Flush |  | `Assembly.Flush` |
| Mate Hinge |  | `Assembly.Hinge` |
| Mate Screw |  | `Assembly.Screw` |
| Show Interfering Parts | <kbd>Ctrl</kbd>+<kbd>Shift</kbd>+<kbd>I</kbd> | `Assembly.Interferences` |
| Bill of Materials |  | `Assembly.BillOfMaterials`, `Assembly.ExportBOM` |
| Motion Study... |  | `Assembly.MotionStudy` |

## Analyze

| Entry | Keys | In a script |
|---|---|---|
| Measure Selection *(on/off)* | <kbd>Ctrl</kbd>+<kbd>Shift</kbd>+<kbd>E</kbd> | `Analyze.Measure` |
| Measure Volume | <kbd>Ctrl</kbd>+<kbd>Shift</kbd>+<kbd>V</kbd> | `GroupAnalysis.Volume`, `GroupAnalysis.MassProperties` |
| Measure Area | <kbd>Ctrl</kbd>+<kbd>Shift</kbd>+<kbd>A</kbd> | `GroupAnalysis.Area`, `Analyze.FacesArea` |
| Measure Perimeter | <kbd>Ctrl</kbd>+<kbd>Shift</kbd>+<kbd>P</kbd> | `Analyze.CurveLength` |
| Show Center of Mass | <kbd>Ctrl</kbd>+<kbd>Shift</kbd>+<kbd>C</kbd> | `GroupAnalysis.CenterOfMass` |
| Check Solid | <kbd>Ctrl</kbd>+<kbd>Shift</kbd>+<kbd>O</kbd> | `GroupAnalysis.Check` |
| Group Dependencies |  | `GroupHistory.DependsOn`, `GroupHistory.UsedBy`, `GroupHistory.OwnSources` |
| Ray-Cast Check |  | `GroupAnalysis.RayCheck`, `GroupAnalysis.Probe` |
| Clearance and Walls... |  | `Assembly.Clearances`, `Assembly.ClearancesWithin`, `GroupAnalysis.WallThickness` |
| Show Naked Edges | <kbd>Ctrl</kbd>+<kbd>Shift</kbd>+<kbd>N</kbd> | `GroupAnalysis.Check` |
| Show Underconstrained Points | <kbd>Ctrl</kbd>+<kbd>Shift</kbd>+<kbd>F</kbd> | `Entity.Free`, `Group.Dof` |
| Trace Point | <kbd>Ctrl</kbd>+<kbd>Shift</kbd>+<kbd>T</kbd> | none: draws a point's path in the window; Assembly.MotionStudy reports positions |
| Stop Tracing... |  | none: draws a point's path in the window; Assembly.MotionStudy reports positions |
| Step Dimension... | <kbd>Ctrl</kbd>+<kbd>Shift</kbd>+<kbd>D</kbd> | `Constraint.Set` |

## Script

| Entry | Keys | In a script |
|---|---|---|
| Show Console *(on/off)* | <kbd>Ctrl</kbd>+<kbd>Shift</kbd>+<kbd>K</kbd> | none: the window's console |
| Run File... | <kbd>Ctrl</kbd>+<kbd>Shift</kbd>+<kbd>R</kbd> | none: a script runs from telesapr-cli run or telesapr --script |
| Stop |  | none: the window's console |
| Allow AI Connections *(on/off)* |  | none: the window's setting; telesapr-cli mcp serves a drawing without one |

## Help

| Entry | Keys | In a script |
|---|---|---|
| User's Guide |  | none: the window's |
| Go to GitHub |  | none: the window's |
| About |  | none: the window's |
