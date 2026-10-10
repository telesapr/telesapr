# Script API renames in 0.5

The calls of `pkg/script/api` that change a drawing moved from `Doc` to a field of it for each
menu of the window, and what only one kind of group has moved to a view of the group
(`docs/adr/0004-scripting-with-yaegi.md`, addendum). A script written for 0.4 is brought up to
date with this table; `help("Features > Fillet Edges")` names the call of any menu entry.
Calls not listed here kept their names: `doc.Group`, `doc.Active`, `g.Solved`, `e.Pos` and the
rest of `Doc`'s lookups, `Group`'s common methods, `Entity`, `Constraint` and the `Solid` types.

New in 0.5: `doc.Constrain.Ratio`, `doc.Constrain.Difference`, `doc.Constrain.SameOrientation`.

## doc.File

| 0.4 | 0.5 |
|---|---|
| `doc.Load(…)` | `doc.File.Load(…)` |
| `doc.Save(…)` | `doc.File.Save(…)` |
| `doc.Export(…)` | `doc.File.Export(…)` |
| `doc.ExportAs(…)` | `doc.File.ExportAs(…)` |
| `doc.Render(…)` | `doc.File.Render(…)` |
| `doc.Unsaved(…)` | `doc.File.Unsaved(…)` |
| `doc.ForgetChanges(…)` | `doc.File.ForgetChanges(…)` |

## doc.Edit

| 0.4 | 0.5 |
|---|---|
| `doc.Undo(…)` | `doc.Edit.Undo(…)` |
| `doc.Redo(…)` | `doc.Edit.Redo(…)` |
| `doc.Regen(…)` | `doc.Edit.Regen(…)` |
| `doc.Copy(…)` | `doc.Edit.Copy(…)` |
| `doc.Paste(…)` | `doc.Edit.Paste(…)` |
| `doc.Select(…)` | `doc.Edit.Select(…)` |
| `doc.SelectEdges(…)` | `doc.Edit.SelectEdges(…)` |
| `doc.SelectFaces(…)` | `doc.Edit.SelectFaces(…)` |
| `doc.Selection(…)` | `doc.Edit.Selection(…)` |
| `doc.SelectedConstraints(…)` | `doc.Edit.SelectedConstraints(…)` |
| `doc.SelectedEdges(…)` | `doc.Edit.SelectedEdges(…)` |
| `doc.SelectedFaces(…)` | `doc.Edit.SelectedFaces(…)` |
| `doc.RunCommand(…)` | `doc.Edit.RunCommand(…)` |
| `doc.SetChordTolerance(…)` | `doc.Edit.SetChordTolerance(…)` |
| `doc.ChordTolerance(…)` | `doc.Edit.ChordTolerance(…)` |

## doc.Params

| 0.4 | 0.5 |
|---|---|
| `doc.AddConfiguration(…)` | `doc.Params.AddConfiguration(…)` |
| `doc.ActivateConfiguration(…)` | `doc.Params.ActivateConfiguration(…)` |
| `doc.ActiveConfiguration(…)` | `doc.Params.ActiveConfiguration(…)` |
| `doc.Configurations(…)` | `doc.Params.Configurations(…)` |
| `doc.RenameConfiguration(…)` | `doc.Params.RenameConfiguration(…)` |
| `doc.DeleteConfiguration(…)` | `doc.Params.DeleteConfiguration(…)` |
| `doc.Parameters(…)` | `doc.Params.List(…)` |
| `doc.Parameter(…)` | `doc.Params.Get(…)` |
| `doc.SetParameter(…)` | `doc.Params.Set(…)` |

## doc.View

| 0.4 | 0.5 |
|---|---|
| `doc.SetSection(…)` | `doc.View.SetSection(…)` |
| `doc.ClearSection(…)` | `doc.View.ClearSection(…)` |
| `doc.Section(…)` | `doc.View.Section(…)` |
| `doc.Units(…)` | `doc.View.Units(…)` |
| `doc.SetUnits(…)` | `doc.View.SetUnits(…)` |
| `doc.MmPerUnit(…)` | `doc.View.MmPerUnit(…)` |
| `doc.SetFaceAppearance(…)` | `doc.View.SetFaceAppearance(…)` |
| `doc.FaceAppearance(…)` | `doc.View.FaceAppearance(…)` |

## doc.Drawing

| 0.4 | 0.5 |
|---|---|
| `doc.Drawing(…)` | `doc.Drawing.Get(…)` |
| `doc.SetDrawing(…)` | `doc.Drawing.Set(…)` |
| `doc.ShowDrawingSheet(…)` | `doc.Drawing.Show(…)` |

## doc.Sketch

| 0.4 | 0.5 |
|---|---|
| `doc.Point(…)` | `doc.Sketch.Point(…)` |
| `doc.Line(…)` | `doc.Sketch.Line(…)` |
| `doc.Circle(…)` | `doc.Sketch.Circle(…)` |
| `doc.Arc(…)` | `doc.Sketch.Arc(…)` |
| `doc.Cubic(…)` | `doc.Sketch.Cubic(…)` |
| `doc.NewWorkplane(…)` | `doc.Sketch.NewWorkplane(…)` |
| `doc.Polyline(…)` | `doc.Sketch.Polyline(…)` |
| `doc.Rect(…)` | `doc.Sketch.Rect(…)` |
| `doc.Text(…)` | `doc.Sketch.Text(…)` |
| `doc.Image(…)` | `doc.Sketch.Image(…)` |
| `doc.TangentArc(…)` | `doc.Sketch.TangentArc(…)` |
| `doc.Split(…)` | `doc.Sketch.Split(…)` |
| `doc.ActiveWorkplane(…)` | `doc.Sketch.ActiveWorkplane(…)` |
| `doc.UV(…)` | `doc.Sketch.UV(…)` |
| `doc.SetWorkplane(…)` | `doc.Sketch.InWorkplane(…)` |
| `doc.Free3D(…)` | `doc.Sketch.Anywhere3D(…)` |

## doc.Constrain

| 0.4 | 0.5 |
|---|---|
| `doc.Coincident(…)` | `doc.Constrain.Coincident(…)` |
| `doc.Distance(…)` | `doc.Constrain.Distance(…)` |
| `doc.Length(…)` | `doc.Constrain.Length(…)` |
| `doc.MinDistance(…)` | `doc.Constrain.MinDistance(…)` |
| `doc.MaxDistance(…)` | `doc.Constrain.MaxDistance(…)` |
| `doc.OnSegment(…)` | `doc.Constrain.OnSegment(…)` |
| `doc.Diameter(…)` | `doc.Constrain.Diameter(…)` |
| `doc.Radius(…)` | `doc.Constrain.Radius(…)` |
| `doc.Horizontal(…)` | `doc.Constrain.Horizontal(…)` |
| `doc.Vertical(…)` | `doc.Constrain.Vertical(…)` |
| `doc.Equal(…)` | `doc.Constrain.Equal(…)` |
| `doc.Parallel(…)` | `doc.Constrain.Parallel(…)` |
| `doc.Perpendicular(…)` | `doc.Constrain.Perpendicular(…)` |
| `doc.Angle(…)` | `doc.Constrain.Angle(…)` |
| `doc.Tangent(…)` | `doc.Constrain.Tangent(…)` |
| `doc.On(…)` | `doc.Constrain.On(…)` |
| `doc.Midpoint(…)` | `doc.Constrain.Midpoint(…)` |
| `doc.Symmetric(…)` | `doc.Constrain.Symmetric(…)` |
| `doc.Lock(…)` | `doc.Constrain.Lock(…)` |
| `doc.Comment(…)` | `doc.Constrain.Comment(…)` |
| `doc.AutoConstrain(…)` | `doc.Constrain.Auto(…)` |
| `doc.Constrain(…)` | `doc.Constrain.Raw(…)` |

## doc.Features

| 0.4 | 0.5 |
|---|---|
| `doc.Sketch3D(…)` | `doc.Features.Sketch3D(…)` |
| `doc.SketchIn(…)` | `doc.Features.SketchIn(…)` |
| `doc.SketchAt(…)` | `doc.Features.SketchAt(…)` |
| `doc.SketchOnFace(…)` | `doc.Features.SketchOnFace(…)` |
| `doc.Extrude(…)` | `doc.Features.Extrude(…)` |
| `doc.Revolve(…)` | `doc.Features.Revolve(…)` |
| `doc.Lathe(…)` | `doc.Features.Lathe(…)` |
| `doc.Helix(…)` | `doc.Features.Helix(…)` |
| `doc.Sweep(…)` | `doc.Features.Sweep(…)` |
| `doc.SweepEdges(…)` | `doc.Features.SweepEdges(…)` |
| `doc.Loft(…)` | `doc.Features.Loft(…)` |
| `doc.LoftThrough(…)` | `doc.Features.LoftThrough(…)` |
| `doc.Thread(…)` | `doc.Features.Thread(…)` |
| `doc.Fillet(…)` | `doc.Features.Fillet(…)` |
| `doc.FilletSolid(…)` | `doc.Features.FilletSolid(…)` |
| `doc.Chamfer(…)` | `doc.Features.Chamfer(…)` |
| `doc.ChamferSolid(…)` | `doc.Features.ChamferSolid(…)` |
| `doc.Shell(…)` | `doc.Features.Shell(…)` |
| `doc.ShellSolid(…)` | `doc.Features.ShellSolid(…)` |
| `doc.Mirror(…)` | `doc.Features.Mirror(…)` |
| `doc.Translate(…)` | `doc.Features.Translate(…)` |
| `doc.Rotate(…)` | `doc.Features.Rotate(…)` |
| `doc.PatternAlongPath(…)` | `doc.Features.PatternAlongPath(…)` |
| `doc.BrokenReferences(…)` | `doc.Features.BrokenReferences(…)` |
| `doc.ClearBrokenReferences(…)` | `doc.Features.ClearBrokenReferences(…)` |

## doc.Assembly

| 0.4 | 0.5 |
|---|---|
| `doc.Link(…)` | `doc.Assembly.Link(…)` |
| `doc.LinkStep(…)` | `doc.Assembly.LinkStep(…)` |
| `doc.Concentric(…)` | `doc.Assembly.Concentric(…)` |
| `doc.Flush(…)` | `doc.Assembly.Flush(…)` |
| `doc.Hinge(…)` | `doc.Assembly.Hinge(…)` |
| `doc.Interferences(…)` | `doc.Assembly.Interferences(…)` |
| `doc.BillOfMaterials(…)` | `doc.Assembly.BillOfMaterials(…)` |
| `doc.ExportBOM(…)` | `doc.Assembly.ExportBOM(…)` |
| `doc.MotionStudy(…)` | `doc.Assembly.MotionStudy(…)` |

## doc.Analyze

| 0.4 | 0.5 |
|---|---|
| `doc.Measure(…)` | `doc.Analyze.Measure(…)` |
| `doc.CurveLength(…)` | `doc.Analyze.CurveLength(…)` |
| `doc.FacesArea(…)` | `doc.Analyze.FacesArea(…)` |

## g.History()

| 0.4 | 0.5 |
|---|---|
| `g.DependsOn(…)` | `g.History().DependsOn(…)` |
| `g.UsedBy(…)` | `g.History().UsedBy(…)` |
| `g.MoveBefore(…)` | `g.History().MoveBefore(…)` |
| `g.MoveToEnd(…)` | `g.History().MoveToEnd(…)` |
| `g.OwnSources(…)` | `g.History().OwnSources(…)` |
| `g.MoveToEndWithSources(…)` | `g.History().MoveToEndWithSources(…)` |
| `g.MoveWithSourcesBefore(…)` | `g.History().MoveWithSourcesBefore(…)` |

## g.Analyze()

| 0.4 | 0.5 |
|---|---|
| `g.Volume(…)` | `g.Analyze().Volume(…)` |
| `g.Area(…)` | `g.Analyze().Area(…)` |
| `g.CenterOfMass(…)` | `g.Analyze().CenterOfMass(…)` |
| `g.Bounds(…)` | `g.Analyze().Bounds(…)` |
| `g.Check(…)` | `g.Analyze().Check(…)` |
| `g.Mesh(…)` | `g.Analyze().Mesh(…)` |
| `g.Shell(…)` | `g.Analyze().Shell(…)` |
| `g.Triangles(…)` | `g.Analyze().Triangles(…)` |
| `g.RayCheck(…)` | `g.Analyze().RayCheck(…)` |
| `g.Probe(…)` | `g.Analyze().Probe(…)` |

## g.AsExtrude()

| 0.4 | 0.5 |
|---|---|
| `g.Draft(…)` | `g.AsExtrude().Draft(…)` |
| `g.SetDraft(…)` | `g.AsExtrude().SetDraft(…)` |
| `g.DraftFaces(…)` | `g.AsExtrude().DraftFaces(…)` |
| `g.SetDraftFaces(…)` | `g.AsExtrude().SetDraftFaces(…)` |
| `g.DraftNeutral(…)` | `g.AsExtrude().DraftNeutral(…)` |
| `g.SetDraftNeutral(…)` | `g.AsExtrude().SetDraftNeutral(…)` |

## g.AsSweep()

| 0.4 | 0.5 |
|---|---|
| `g.Twist(…)` | `g.AsSweep().Twist(…)` |
| `g.SetTwist(…)` | `g.AsSweep().SetTwist(…)` |
| `g.CornerRadius(…)` | `g.AsSweep().CornerRadius(…)` |
| `g.SetCornerRadius(…)` | `g.AsSweep().SetCornerRadius(…)` |
| `g.SweepUp(…)` | `g.AsSweep().SweepUp(…)` |
| `g.SetSweepUp(…)` | `g.AsSweep().SetSweepUp(…)` |

## g.AsRepeat()

| 0.4 | 0.5 |
|---|---|
| `g.Copies(…)` | `g.AsRepeat().Copies(…)` |
| `g.SetCopies(…)` | `g.AsRepeat().SetCopies(…)` |

## g.AsMirror()

| 0.4 | 0.5 |
|---|---|
| `g.KeepOriginal(…)` | `g.AsMirror().KeepOriginal(…)` |
| `g.SetKeepOriginal(…)` | `g.AsMirror().SetKeepOriginal(…)` |
| `g.MirrorSolid(…)` | `g.AsMirror().MirrorSolid(…)` |
| `g.SetMirrorSolid(…)` | `g.AsMirror().SetMirrorSolid(…)` |
| `g.Plane(…)` | `g.AsMirror().Plane(…)` |

## g.AsChamfer()

| 0.4 | 0.5 |
|---|---|
| `g.Distance(…)` | `g.AsChamfer().Distance(…)` |
| `g.SetDistance(…)` | `g.AsChamfer().SetDistance(…)` |
| `g.SecondDistance(…)` | `g.AsChamfer().SecondDistance(…)` |
| `g.SetSecondDistance(…)` | `g.AsChamfer().SetSecondDistance(…)` |
| `g.SwapDistances(…)` | `g.AsChamfer().SwapDistances(…)` |

## g.AsFillet()

| 0.4 | 0.5 |
|---|---|
| `g.Radius(…)` | `g.AsFillet().Radius(…)` |
| `g.SetRadius(…)` | `g.AsFillet().SetRadius(…)` |
| `g.EdgeRadius(…)` | `g.AsFillet().EdgeRadius(…)` |
| `g.SetEdgeRadius(…)` | `g.AsFillet().SetEdgeRadius(…)` |
| `g.SetEdgeRadiusExpr(…)` | `g.AsFillet().SetEdgeRadiusExpr(…)` |

## g.AsShell()

| 0.4 | 0.5 |
|---|---|
| `g.Thickness(…)` | `g.AsShell().Thickness(…)` |
| `g.SetThickness(…)` | `g.AsShell().SetThickness(…)` |

## g.AsLoft()

| 0.4 | 0.5 |
|---|---|
| `g.Profiles(…)` | `g.AsLoft().Profiles(…)` |
| `g.Turn(…)` | `g.AsLoft().Turn(…)` |
| `g.SetTurn(…)` | `g.AsLoft().SetTurn(…)` |
| `g.Tangency(…)` | `g.AsLoft().Tangency(…)` |
| `g.SetTangency(…)` | `g.AsLoft().SetTangency(…)` |
| `g.Guides(…)` | `g.AsLoft().Guides(…)` |
| `g.SetGuides(…)` | `g.AsLoft().SetGuides(…)` |

## g.AsLink()

| 0.4 | 0.5 |
|---|---|
| `g.LinkAsSolid(…)` | `g.AsLink().AsSolid(…)` |
| `g.SetLinkAsSolid(…)` | `g.AsLink().SetAsSolid(…)` |
| `g.LinkInfo(…)` | `g.AsLink().Info(…)` |
| `g.LinkProblem(…)` | `g.AsLink().Problem(…)` |
