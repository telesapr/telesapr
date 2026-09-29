# SolveSpace→Go Core Engine Porting Status

This document tracks the current state of the Go port vs the C++
solvespace core engine. It is the source of truth for the
"reach true parity" goal's success criterion #4.

## Categories

- **OK** — file ported (or split across multiple Go files).
- **PARTIAL** — most methods ported but some gaps remain; see notes.
- **MISSING** — file not ported at all.
- **UNAUDITED** — ported, but not yet compared line by line against the C++.
- **OUT-OF-SCOPE** — UI / export / render work, not part of "core engine".

## Core engine files

| C++ file | Lines | Status | Go file(s) | Notes |
|---|---|---|---|---|
| `expr.cpp` | 963 | OK | `pkg/core/expr.go`, `pkg/core/expr_parse.go` | `ExprQuaternion::Rotate`/`RotationU/V/N`/`Times` use the C++ formulas, which differ from a rotation for non-unit quaternions. `PartialWrt` (`Derivative`), `FoldConstants` (including its `Tol` of 0.001 for the x+0, x*1, x*0 rules) and `IsZeroConst` are faithful |
| `entity.cpp` | 997 | OK | `pkg/core/entity.go`, `pkg/core/entity_base.go` | Every `EntityBase` accessor, numeric and symbolic, for all point/normal/face/distance types, plus `GenerateEquations` and `Entity::CalculateNumerical`. The sketch is passed in as a `core.EntitySketch` instead of the global `SK` |
| `constraint.cpp` | 994 | OK | `pkg/core/constraint.go`, `pkg/sketch/constraint.go` | Audited: `DescriptionString`, `DeleteAllConstraintsFor`, `AddConstraint` (as `AddConstraintRemember`), `Constrain`, `TryConstrain`, `ConstrainCoincident` and the three tangent helpers, which return C++'s error message instead of showing it. `MenuConstrain` is user interface |
| `system.cpp` | 660 | OK | `pkg/solver/system.go`, `pkg/solver/qr.go`, `pkg/solver/colamd.go` | `Solve`, `SolveRank`, `SolveBySubstitution`, `WriteJacobian`, `NewtonSolve`, `SolveLeastSquares` (the QR-of-A' form of this checkout), `TestRank`, `FindWhichToRemoveToFixJacobian`, `MarkParamsFree`. The QR is the vendored Eigen 3.4's `SparseQR` with `COLAMDOrdering` (`pkg/solver/colamd.go`), down to its storage and summation order |
| `group.cpp` | 1228 | OK | `pkg/sketch/group.go`, `pkg/sketch/group_generate.go` | `Generate` (all group types; new params seeded from `Sketch.View`, the GUI's view, or `DefaultView` without one), `Clear`, `CopyEntity`, `Remap` (as `RemapEntity`), the `Make*` helpers, `GenerateEquations` and `TransformImportedBy`. LINKED groups copy the `ImpEntity` the loader reads from the linked file. Menu and UI code are out of scope |
| `generate.cpp` | 598 | OK | `pkg/sketch/generate_all.go`, `pkg/sketch/sketch.go` | `GenerateAll` (the `genForBBox` pass, regenerating entities and params, prune-and-retry), `SolveGroup` with `WriteEqSystemForGroup` and `WriteEquationsExceptFor`, `ForceReferences`, `CalculateEntityBBox`. `GenerateAll`'s DIRTY and UNTIL_ACTIVE ranges (the active group is `Sketch.Active`, else the one C++ picks on load), `TestRankForGroup`, `MarkGroupDirty`, the `deleted` counts and their message (through `Sketch.Message`), and `ScheduleGenerateAll`, which `Refresh` serves as C++'s event loop does. Dragged params aren't marked |
| `groupmesh.cpp` | 803 | OK | `pkg/sketch/group_pipeline.go` | `AssembleLoops`/`GenerateLoops`, `GenerateShellAndMesh` (every group type, step and repeat, the face entities, the running-shell combine, and the forced-to-mesh combine with `SnapToMesh`), `GenerateForStepAndRepeat`, `GenerateForBoolean`, `RemapFaces`, `GenerateDisplayItems` (without the render-side polyline join) are faithful; `group/force_to_mesh` saves match C++ byte for byte |
| `drawentity.cpp` (`GenerateBezierCurves`) | — | OK | `pkg/sketch/entity_bezier.go` | Lines, arcs/circles, `ComputeInterpolatingSpline` (C2 cubics, periodic too), TTF text (through `ttf.Fonts.PlotString`) |
| `request.cpp` | 250 | OK | `pkg/sketch/request.go`, `pkg/persist/slvs/linked.go` | Audited: `Generate` (an image request takes its aspect ratio from the image file, loaded as `ReloadLinkedImage` does, and a TTF request from its font), `DescriptionString`, `IndexOfPoint`, the entity/request table |
| `style.cpp` | 948 | OK | `pkg/core/style.go`, `pkg/core/settings.go`, `pkg/sketch/style.go` | Audited: the default styles, `FillDefaultStyle` (from the user's settings, `core.GetSettings`, which like the headless C++ build's return the defaults unless the caller sets a store), `LoadFactoryDefaults` and `FreezeDefaultStyles`, `Get` (which creates a missing style, so an entity naming one that the file lacks gets a saved "new-custom-style", as in C++), `ForEntity`, the colour, width, text height, exportable and stipple getters (widths in mm take the view scale), `DescriptionString`. Loading starts each style from the defaults. The style screens are user interface |
| `clipboard.cpp` | 527 | OK | `pkg/sketch/clipboard.go`, `pkg/sketch/edit.go` | Audited: the clipboard (`NewEntityFor` was off by one), `CopySelection`, `PasteClipboard` and `DeleteSelection`, taking the selection as an argument, with `GraphicsWindow::AddRequest` and `DeleteTaggedRequests`. The menus and the paste-transformed screen are user interface |
| `srf/surface.cpp` | 494 | OK | `pkg/srf/surface.go`, `pkg/srf/shell.go`, `pkg/srf/raycast_surface.go` | Audited throughout: `FromPlane`, `FromExtrusionOf`, `IsExtrusion`, `IsCylinder`, `FromRevolutionOf`, `FromTransformationOf`, `GetAxisAlignedBounding`, `LineEntirelyOutsideBbox`, `MakeEdgesInto`/`MakeTrimEdgesInto`, `ExactSurfaceTangentAt`, `MakeSectionEdgesInto`, `TriangulateInto`, `Reverse`, `ScaleSelfBy`, `Clear` (trims only), `CoincidentWithPlane`, `SShell::MakeFromCopyOf`/`MakeFromTransformationOf` |
| `srf/shell.cpp` | 617 | OK | `pkg/srf/shell.go` | `MakeFromExtrusionOf`, `MakeFromRevolutionOf`, `MakeFromHelicalRevolutionOf` (including their face entities, through the `ShellGroup` the caller passes in place of C++'s `Group`), `CheckNormalAxisRelationship`, `MakeFirstOrderRevolvedSurfaces`, `MakeSectionEdgesInto` faithful; checked against a C++ harness in `pkg/srf/revolve_test.go` |
| `srf/boolean.cpp` | 1100 | OK | `pkg/srf/boolean.go`, `pkg/srf/boolean_csg.go`, `pkg/srf/boolean_trim.go`, `pkg/srf/bsp_uv.go` | `SBspUv`, `MakeFromBoolean`, curve splitting, `MakeCopyTrimAgainst`, `TrimFromEdgeList`, `EdgeNormalsWithinSurface` |
| `srf/polygon.cpp` / `polygon.cpp` | 924 | OK | `pkg/srf/polygon.go`, `pkg/srf/contour.go`, `pkg/srf/edgelist.go`, `pkg/srf/merge.go` | `AssemblePolygon`/`AssembleContour`, contour orientation (C++ sign convention), `FixContourDirections`, `EdgeCrosses`, `CullExtraneousEdges`, `SContour::AddPoint`/`CopyInto`/`FindPointWithMinX`/`MakeEdgesInto`, `OffsetInto` (contour and polygon; the Go one was a Minkowski stand-in on the first contour only), `ComputeNormal` and `SignedArea` (which had stored and computed the normal as side effects), and `STriangle` (`pkg/srf/triangle.go`: `MinAltitude`, `ContainsPoint`, `Raytrace`, `IsDegenerate`, `Transform`; `From` leaves the vertex normals zero) |
| `srf/surfinter.cpp` | 641 | OK | `pkg/srf/surfinter.go`, `pkg/srf/surface.go`, `pkg/srf/merge.go` | `AddExactIntersectionCurve`, every `IntersectAgainst` case (plane-plane, plane-extrusion, coaxial extrusions, exact curves in a plane, the marching general case), `CoincidentWith`, `ContainsPlaneCurve`, `MakeCoincidentEdgesInto`. The branches no fixture reaches are pinned to a C++ harness by `TestIntersectionBranchesVsCpp` |
| `srf/curve.cpp` | 886 | OK | `pkg/srf/curve.go`, `pkg/srf/bezier*.go`, `pkg/srf/boolean_csg.go` | Audited throughout: the `SBezier` constructors (through `Vector4`, so unused weights are 0), `Start`/`Finish`, `Reverse`, `ScaleSelfBy`, `GetBoundingProjd`, `TransformedBy`, `IsInPlane`/`IsRational` (with `LENGTH_EPS`), `IsCircle`, `InPerspective`, `Equals`, `CullIdenticalBeziers` (was a stand-in), `AllIntersectionsWith`, `GetPlaneContainingBeziers`, loop assembly (`FromCurves`, `SBezierLoopSet::From`, `FindOuterFacesFrom`), `IsClosed`, the loop set's lazy `SignedArea`, `AddOpenPath`, `SCurve::FromTransformationOf`, `GetSurfaceA/B`, `RemoveShortSegments`, `STrimBy::EntireCurve`. The loop set keeps its outer loop and holes apart, in C++'s order |
| `srf/triangulate.cpp` | 738 | OK | `pkg/srf/triangulate.go` | Audited throughout: hole bridging (`BridgeToContour`), `IsEmptyTriangle`, `IsEar` with `RayIsInside`, `ClipEarInto`, the fan pass and ear clipping, `ChordToleranceForEdge`, the adaptive grid (`MakeTriangulationGridInto`, `UvGridTriangulateInto`), `TriangulateInto` |
| `srf/ratpoly.cpp` | 790 | OK | `pkg/srf/bezier.go`, `pkg/srf/pwl.go`, `pkg/srf/surface.go` | Adaptive pwl (`MakePwlInto`), `PointOnSurfaces`, `PointOnCurve`, `ClosestPointOnThisAndSurface`, `SSurface::ClosestPointTo`/`ClosestPointNewton`, `TangentsAt`, `NormalAt` (unnormalized, as in C++), `SSurface::PointAt` (C++'s product order, so the same bits), `SBezier::PointAt`/`TangentAt`, `SBezier::ClosestPointTo` (re-ported: it was a clamped Newton stand-in, which put split points a few ulps off), `SplitAt` (C++'s `Vector4::Blend`), `PointIntersectingLine`, `EdgeCurveIntersection`. Callers pass `mustConverge`/`retry` as the C++ call sites do |
| `srf/raycast.cpp` | 807 | OK | `pkg/srf/raycast.go`, `pkg/srf/raycast_surface.go`, `pkg/srf/surface.go` | Re-audited: `DepartureFromCoplanar` and `SplitInHalf` (with the weighting and row helpers) were stand-ins; the old `SplitInHalf` was wrong, and cancelled out a `PointIntersectingLine` difference on `boolean_tangent_crossing`. `ClassifyEdge` now treats a surface without a BSP as C++ does (outside). `AllPointsIntersecting`, `ClassifyRegion` checked |
| `srf/merge.cpp` | 130 | OK | `pkg/srf/merge.go` | `MergeCoincidentSurfaces`, with `CoincidentWith` (audited) |
| `bsp.cpp` | 733 | OK | `pkg/srf/bsp.go` | `SBsp3` (`FromMesh` with glibc's `rand()` shuffle, `Insert` with the triangle and quad splits, `InsertConvex`, `InsertInPlane`, `GenerateInPaintOrder`) and `SBsp2`, re-ported: the earlier port pushed straddling triangles down both sides unsplit |
| `constrainteq.cpp` | 1062 | OK | `pkg/core/constraint_eq.go`, `pkg/sketch/group_generate.go` | `GenerateEquations` for every type, `ModifyToSatisfy`, `HasLabel`, `Generate` and the static helpers (`Distance`, `PointLineDistance`, `PointPlaneDistance`, `DirectionCosine`, `PointInThreeSpace`, `VectorsParallel3d`) |
| `modify.cpp` | 751 | OK | `pkg/sketch/edit.go`, `pkg/sketch/modify.go` | `ReplacePointInConstraints`, `RemoveConstraintsForPointBeingDeleted`, `FixConstraintsForRequestBeingDeleted`/`PointBeingDeleted`, and the editing tools: `MakeTangentArc` (with `ParametricCurve`; the tangent arc settings are passed in), `SplitLine`, `SplitCircle`, `SplitCubic`, `SplitEntity` and `SplitLinesOrCurves`, which take the selection, and the mouse position in the default view, as arguments. Checked against a C++ harness by `TestModifyToolsVsCpp` |
| `solvespace.cpp` | 1236 | OK | `pkg/sketch/sketch.go`, `pkg/sketch/group.go` | `Refresh`, the unsaved flag; the `Add*` helpers assign handles as `IdList::AddAndAssignId` does (largest plus one; they used the count). Observer notifications are out-of-scope (UI). |
| `undoredo.cpp` | 157 | OK | `pkg/sketch/undo.go` | Re-ported: states hold what defines the sketch (groups without their generated data, group order, requests, constraints, params, styles), stacks are `MAX_UNDO` (100) deep, and a pop reloads linked files and regenerates. The active group is derived from the group order; the unsaved flag and menus are the caller's |
| `mesh.cpp` | 1221 | OK | `pkg/srf/mesh.go`, `pkg/srf/kdnode.go`, `pkg/srf/outline.go` | `SMesh` (the BSP Booleans via `AddAgainstBsp` and `Simplify`, `MakeFromTransformationOf`, `MakeEdgesInPlaneInto`, `CalculateVolume`, `GetCenterOfMass`, ...), `SKdNode` (`From` copies the triangles and shuffles them with glibc's `rand()`; `SnapToMesh`, `FindEdgeOn`, `MakeCertainEdgesInto`, `MakeOutlinesInto`, `OcclusionTestLine`) and `SOutline`. `FirstIntersectionWith` takes the ray, which C++ makes from the view. `PrecomputeTransparency` sorts with a port of libstdc++'s `std::sort` (`pkg/srf/cxxsort.go`, pinned by `TestCxxSortVsLibstdcxx`), since the display mesh's order is the order it exports in |
| `file.cpp` | 1030 | OK | `pkg/persist/slvs/slvs.go`, `pkg/persist/slvs/linked.go`, `pkg/sketch/upgrade.go` | Loading follows `LoadFromFile`: the saved entities are dropped, `ReloadAllLinked` reads each linked file's entities, mesh and shell (`LoadEntitiesFromFile`: an IDF board, an STL mesh, or a .slvs file's entities, styles, mesh and shell), loads the images it uses, and forces a mesh-only link to a mesh, and `UpgradeLegacyData` regenerates the sketch (REGEN) and seeds legacy constraint params. Saving follows `SaveToFile` (C++'s header, whatever the loaded file had; regenerate, then each entity's export numerics and `Entity::IsVisible` with the default view, the custom styles only, then the last group's running mesh and shell), and matches current C++ byte for byte on the fixtures, shells included. A missing linked file or image counts as declined (C++ asks the user to find it). `NewFile`, `ClearExisting` and `CreateDefaultDrawingGroup` are `pkg/sketch/newfile.go`; a new file saves as C++'s does (`TestNewFileVsCppSave`) |
| `importmesh.cpp` | 258 | OK | `pkg/persist/slvs/stl.go` | `LinkStl`: a binary STL's triangles, colours, and the origin, bounding box and edge-vertex entities; `group/link_stl` saves match C++ byte for byte |
| `importidf.cpp` | 536 | OK | `pkg/persist/slvs/idf.go` | `LinkIDF`: the board outline, cutouts, routing keepouts and drilled holes as entities, and the board extruded (`std::stof`'s float rounding included); `group/link_idf` saves match C++ byte for byte |
| `ttf.cpp` | 449 | OK | `pkg/ttf` | `TtfFontList` (`LoadAll`, sorted with `CxxSort` and deduplicated by name; the font files are `ttf.FontFiles`, set with `SetFontFiles`, as the headless build's `fontFiles`; the builtin Bitstream Vera is embedded), `LoadFont`, `PlotString` (its float factor included), `AspectRatio`, `ExtractTTFData`, and `ReadUTF8`'s decoding. Under it, FreeType 2.14.3 as C++ calls it: opening a face (a collection's first; WOFF and WOFF2 files, with brotli and WOFF2's glyf/loca/hmtx transforms), the names, the Unicode charmap (formats 0, 2, 4, 6, 8, 10, 12, 13, 14, validated; synthesized from `post` glyph names or the CFF charset without one), `FT_Request_Size`, the kern table, `FT_Outline_Decompose`. The TrueType driver's unhinted glyph loading (composites, phantom points, `FT_MulFix`/`FT_DivFix`/`FT_Hypot`), and for "tricky" fonts (detected by family name or `cvt`/`fpgm`/`prep` checksums) the hinted loading they need: the v40 bytecode interpreter (`ttinterp.c`), `fpgm`/`prep`, hinted metrics, `hdmx`. The CFF driver: CFF and CFF2 in OpenType (`OTTO`, collections) and bare CFF files, CID-keyed fonts, the Adobe charstring engine (`psaux/cf2*`, unhinted), the font matrix, CFF2 blends at the default instance. The Type 1 driver: PFA (hex or binary `eexec`) and PFB files, the PostScript dictionary parser (`psobjs.c`, `psconv.c`), Subrs (hashed when sparse) and CharStrings, encodings, multiple-master fonts at their `WeightVector`, and the charstrings through the Adobe engine's Type 1 mode (flex, hint replacement, `seac`, `sbw`, `div`, othersubrs). The Type 42 driver: the font dictionary (hex or binary `sfnts` strings, `/name` and LilyPond `(name) cvn` CharStrings, `.notdef` swapped to glyph 0, the encodings), the embedded TrueType font through the TrueType driver (tricky fonts hinted), no kerning. The PFR driver: the header, logical and physical font records (extra items, the auxiliary names and metrics), its charmap, simple and compound glyph programs (with `pfr_glyph_close_contour`'s check against the previous contour's last point), kerning items and their binary search. FreeType's drivers are tried in its module order |
| `polyline.cpp` | 247 | OK | `pkg/srf/polylinebuilder.go` | `PolylineBuilder`: vertices looked up as C++'s `unordered_map` with `VectorHash` (its `y`-for-`z` term included) and `VectorPred`, edges keyed by ordered vertex pair, `Generate`'s two passes (open chains, then loops) and its plane-following `GetNext`, `MakeFromEdges`/`MakeFromOutlines`, `GenerateEdges`/`GenerateOutlines`. `Group::GenerateDisplayItems` joins the display outlines through it when edges or outlines are shown; `TestDisplayOutlinesVsCpp` compares every group's outlines bit for bit with C++ |
| glibc `libm` (2.44) | — | OK | `pkg/libm` | `sin`, `cos`, `tan`, `asin`, `acos`, `atan`, `atan2` and the huge-argument reduction, as glibc's x86-64 FMA variants compile them (tables read from `libm-2.44.a`); the engine's trigonometry goes through it |
| `util.cpp` (geometry helpers) | — | OK | `pkg/geom` | `Vector::Equals` uses `LENGTH_EPS`, `OnLineSegment`, `BoundingBoxIntersectsLine`, `BandedMatrix`, plane intersections, `AtIntersectionOfLines`, `DistanceToLine`/`DistanceToPlane`, `DotInToCsys`/`ScaleOutOfCsys`, `WithMagnitude`, `RotatedAbout` (both overloads; Go's went through a quaternion), `DirectionCosineWith`, `ClosestPointOnLine`, `DivProjected`, `ClampWithin`, `Vector4` (`From(w, v)` premultiplies, `Blend`), `Point2d` (`Equals` with `LENGTH_EPS`, `Angle`, `FromPolar`, `DistanceToLineSigned`), `BBox` (`GetOrigin` is the centre; `Overlaps`, `Contains`), `Quaternion::From(axis, angle)`, `Times`, `Inverse`, `Mirror` (was wrong), `ToThe`, `Quaternion::Rotate`/`RotationU/V/N`/`From(u, v)`; `geom.Max`/`Min` are `std::max`/`min` |

An **OK** row means the functions listed in its notes (or the whole
file, when none are listed) were compared line by line against the C++.

## GUI files

The GUI runs on takigo (`github.com/msorc/takigo`, a pure-Go Tk) through
the `pkg/platform` interfaces, and draws with a software renderer. The
rows below follow the same rule as the engine's: **OK** means compared
line by line with the C++.

| C++ file | Lines | Status | Go file(s) | Notes |
|---|---|---|---|---|
| `render/render.h`, `render.cpp` | 867 | OK | `pkg/render/{camera,canvas,uicanvas,picker}.go` | `Camera`, `Lighting`, the canvas interfaces, `UiCanvas`, `ObjectPicker`; `TestCameraVsCpp` |
| `render/rendergl1.cpp` | 857 | OK | `pkg/render/soft/*.go` | The GL1 renderer's semantics on a CPU z-buffer: depth ranges per layer, zIndex offsets, stipple, fat lines, lighting. Pixels are Go-only goldens (`testdata/render/soft`). A mesh is rasterised in horizontal bands at once; each pixel still meets its triangles in draw order, so the frame is the same as on one thread (`TestPaintParallelSameAsSerial`) |
| `resource.cpp` | 1587 | OK | `pkg/res/*.go` | Embedded resources, `Pixmap`, `BitmapFont`, `VectorFont`; `TestFontsVsCpp`. Translations are not ported |
| `platform/gui.h`, `gui.cpp` | 536 | OK | `pkg/platform/*.go` | Events, accelerators, file filters, JSON settings |
| `platform/guinone.cpp` | 145 | OK | `pkg/platform/headless` | Also records windows, menus and dialogs, and answers dialogs from scripts |
| `platform/guigtk.cpp` | 1683 | OK | `pkg/platform/tkgui` | takigo backend; no takigo changes. Fullscreen through `_NET_WM_STATE`, tooltips drawn on the canvas, the edit overlay selects its text as GTK's entry does. Every window gets the 48x48 icon through `_NET_WM_ICON`, as GTK's `set_icon` sets it. takigo quirks worked around here: its packer resizes a toplevel to its content and ignores a size set through `wm geometry`, so a restored size is set again once layout has run, and it does not follow a toplevel's moves, so `FreezePosition` asks the server and subtracts `_NET_FRAME_EXTENTS` |
| `platform/entrygui.cpp` | 42 | OK | `cmd/spacesolve` | `make install` puts the desktop entry (`dist/freedesktop`), the icons and the MIME type in place, as `res/CMakeLists.txt` does |
| `graphicswin.cpp` | 1481 | OK | `pkg/ui/graphicswin.go`, `menu_table.go` | |
| `mouse.cpp` | 1600 | OK | `pkg/ui/mouse.go` | |
| `draw.cpp` | 961 | OK | `pkg/ui/draw.go` | `TestDrawCallsVsCpp`: every canvas call for 100 fixtures, two views |
| `drawentity.cpp` | 867 | OK | `pkg/ui/drawentity.go` | |
| `drawconstraint.cpp` | 1383 | OK | `pkg/ui/drawconstraint.go` | |
| `toolbar.cpp` | 252 | OK | `pkg/ui/toolbar.go` | |
| `textwin.cpp` | 1222 | OK | `pkg/ui/textwin.go`, `textbuttons.go` | `Printf` compares link functions by their code pointers, as C++ compares function pointers. `TestTextWindowVsCpp`: every row, cell colour and link of every screen for 31 fixtures |
| `textscreens.cpp` | 984 | OK | `pkg/ui/textscreens.go` | |
| `describescreen.cpp` | 589 | OK | `pkg/ui/describescreen.go` | |
| `confscreen.cpp` | 592 | OK | `pkg/ui/confscreen.go` | |
| `view.cpp` | 174 | OK | `pkg/ui/viewscreen.go` | |
| `style.cpp` (UI portion) | — | OK | `pkg/ui/stylescreens.go` | |
| `clipboard.cpp` (UI portion) | — | OK | `pkg/ui/clipboard.go` | |
| `solvespace.cpp` (UI portion) | — | OK | `pkg/ui/{solvespace,app,menufile,menuanalyze}.go` | Settings, units, load/save, autosave, recent files, the File, Analyze and Help menus. The engine keeps linked paths as the file wrote them; saving to another folder rewrites them relative to it, as C++ writes its `P` fields |
| `group.cpp`, `constraint.cpp`, `undoredo.cpp` (menus) | — | OK | `pkg/ui/{menugroup,menuconstrain,undomenu}.go` | |
| `export.cpp` | 1272 | OK | `pkg/ui/export.go` | 2d view, wireframe and section export with hidden line removal and cutter-radius offset; mesh export as STL, OBJ, three.js and VRML; PNG export through the next repaint |
| `exportvector.cpp` | 1365 | OK | `pkg/ui/exportvector.go`, `exportdxf.go`, `pkg/dxf` | EPS, PDF, SVG, HPGL, G code, STEP 2d and DXF. `pkg/dxf` ports the part of libdxfrw's ASCII writer SolveSpace uses (AC1021); its HEADER section, which depends on nothing SolveSpace sets, is captured from C++ (`scripts/regen-export-golden.sh`) |
| `exportstep.cpp` | 721 | OK | `pkg/ui/exportstep.go` | Surfaces and wireframes, with the alias tables. As in C++, exporting surfaces scales the shell's own surfaces by the export scale |
| `importdxf.cpp` | 1199 | OK | `pkg/ui/importdxf.go`, `pkg/dxf/dxfread` | `pkg/dxf/dxfread` ports the part of libdxfrw's reader SolveSpace uses: ASCII and binary files, the header's version and code page, every code page's text decoding (the tables generated by `scripts/gen-dxf-cptables.py`), the entities' parsing and extrusions, and libdxfrw's quirks (a block's first entity loses its first record, SEQEND's records land in its polyline, dimension code 42 also sets the extrusion's x, a dimension kept in a block is replayed as unknown, a diametric one as radial). Differences: C++ reads an off layer's negative colour index out of the palette's bounds (here: the colour it negates); C++ crashes on an INSERT of a missing block, a spline with too few control points or an empty polyline (here: they are skipped). DWG import reports that it isn't supported |

Not ported: translations, 3Dconnexion, the GL3 and Cairo renderers,
DWG import, and the Windows, macOS and HTML backends.

## Parity checks

481 tests (336 top-level) across the 8 engine packages (`pkg/core`,
`pkg/geom`, `pkg/libm`, `pkg/persist/slvs`, `pkg/sketch`, `pkg/solver`,
`pkg/srf`, `pkg/ttf`); no TODO markers. The tests that pin Go to C++:

- **Saves.** `TestSLVSLoadRoundTrip`/`TestPhase7AllFixturesRoundTrip`:
  every fixture, loaded and generated as C++ `AfterNewFile` does, saves
  to the bytes current C++ writes (boolean shells included); the
  fixtures older versions wrote (`roundTripExclusions`) are compared
  with C++'s save of them instead (`TestExcludedFixturesVsCppSave`,
  `testdata/cppsave`). `TestModifyToolsVsCpp`: the tangent arc and split
  tools, and `TestExportStlVsCpp`: STL export, byte for byte. TrueType
  text uses `testdata/Gentium-R.ttf`, as C++'s test harness does;
  `request/ttf_text/extrude*.slvs` extrude text, so their shells come
  from glyph outlines.
- **Solving.** `TestParityGroupSolveVsCpp`: every group solves with
  C++'s degrees of freedom and face entities. `TestSparseQRVsEigen`:
  rank and least-squares solution bit for bit on 500 systems.
- **Fonts.** `TestTTFVsCpp`: font names, cap heights, aspect ratios and
  `PlotString`'s Beziers bit for bit, for Gentium and the builtin font;
  `TestSynthFontsVsCpp`: the same for 39 generated fonts
  (`scripts/gen-ttf-testfonts.py`: CFF/CFF2 charstring operators, CID
  and bare CFF, WOFF/WOFF2, cmap formats 2/8/10, `post` names, tricky
  fonts whose bytecode exercises the interpreter, Type 1 PFA/PFB and
  multiple-master fonts, Type 42 fonts, and PFR fonts from the script's
  own PFR assembler).
- **Numerics.** `TestLibmVsGlibc`: 36,000 inputs over every branch,
  `hypot` included;
  `TestNoFusedMultiplyAdd`: the build doesn't fuse `x*y + z`;
  `TestCxxSortVsLibstdcxx`: `std::sort`.
- **Export.** `TestExportVsCpp`: 15 fixtures exported in every format
  (2d views rotated and straight on, wireframe, section, mesh,
  surfaces, and the export options that are off by default), byte for
  byte against `scripts/cpp-export-harness.cpp`. `pkg/internal/cfmt`
  formats as glibc's printf (`TestSprintfVsGlibc`).
- **Import.** `TestImportDxfVsCpp`: 32 DXF files (made with ezdxf to
  cover every entity, blocks, bulges, extrusions, texts, dimensions,
  versions, code pages including the double-byte ones, and binary DXF;
  ezdxf's examples; and our own exports) imported into a new sketch, in
  its workplane and free in 3d, saved byte for byte as C++ saves them.
- **GUI.** `TestDrawCallsVsCpp` (every canvas call of
  `GraphicsWindow::Draw`), `TestTextWindowVsCpp` (every text window
  screen), `TestDisplayOutlinesVsCpp`, `TestNewFileVsCppSave` and
  `TestSaveNewFileVsCpp` (a new file saved through File > Save),
  `TestCameraVsCpp`, `TestFontsVsCpp`; headless tests drive the menus,
  dialogs and text window links.
- **Booleans.** `TestParityBooleanVolumeVsCppGolden`: every boolean
  fixture's volume within the C++ testsuite's 1e-6;
  `TestParityBooleanFixtures`: watertight meshes;
  `TestIntersectionBranchesVsCpp`: the intersection cases no fixture
  reaches.

Golden data is regenerated by the `scripts/regen-*` scripts (C++
harnesses built against `solvespace/build` and its vendored Eigen, or
glibc); `scripts/libmcheck` compares `pkg/libm` with the system libm on
~10^9 inputs, and `scripts/regen-ttf-golden.sh` with font files compares
`pkg/ttf` with C++ on them (all 1,513 system font files matched:
TrueType, OpenType CFF and CFF2, collections, Type 1 and
multiple-master Type 1, bitmap-only).

## Definition of done

Every row above is **OK**, `go vet ./...` and `go test ./...` pass on
the whole repo (the parity tests above included), and no engine package
has TODO markers. Met.

## Lessons

Parity came from replacing simplified stand-ins with faithful ports,
never from tuning; the Go-only heuristics that had masked them are
gone. A faithful port can break parity by removing a bug that cancelled
another; port the other side too. Bit-level details that mattered:
`List::AddToBeginning` doesn't insert; `std::max`/`min` keep the first
argument for zeros of either sign; identity transforms still turn `-0`
into `+0`; product order changes bits; Go folds constant expressions exactly, so
`30*math.Pi/180` isn't C++'s `30*PI/180` (use `geom.Pi`); GCC evaluates
a call's arguments right to left, so nested calls that add requests run
last argument first; C++ links the
vendored Eigen 3.4,
not the system's; GCC's fused multiply-adds in glibc's FMA variants;
and C++ solves once more on open (`AfterNewFile`) than on load.

## Remaining work

No known stand-ins are left, but `SBezier::ClosestPointTo` turned out
to be one after its row was audited, so an "audited" row may still
hide one.

**Known gaps:**

- The fonts a sketch can use are the ones the caller lists with
  `ttf.SetFontFiles`, as in C++'s headless build (C++'s GUI builds list
  the system's fonts through fontconfig).
- FreeType's CFF and Type 1 drivers seed the charstring `random`
  operator from memory addresses, so a glyph using it can't match C++.
  CID-keyed Type 1 fonts (FreeType's t1cid driver, not ported) get no
  Unicode charmap, so C++ never lists them either; bitmap-only fonts
  fail in both.
- Undefined behaviour in C++ that Go can't reproduce: for a character a
  font lacks, ttf.cpp loads the glyph whose index is the code point, and
  FreeType's Type 42 driver reads past its CharStrings for an index
  beyond them (C++ crashes; Go stops the string, as for a glyph that
  fails to load). A PFR font without a style name (or family name and
  font ID) has C++ make a `std::string` from NULL. No test font does
  either. There are no Type 42 or PFR fonts on this system, so those
  drivers are pinned by generated fonts only (and a Type 42 wrapping of
  Gentium, which matched C++ up to its crash).
- The bits match C++ as built here: x86-64 with FMA and AVX2 (glibc
  2.44 picks the FMA variants of its trigonometry) and the vendored
  Eigen built without FMA. Elsewhere C++ itself computes other last
  bits. Go has to be built for `GOAMD64=v1` (the default) or `v2`: for
  `v3`, and on arm64, Go fuses `x*y + z` itself.
