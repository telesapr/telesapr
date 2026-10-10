# TeleSAPR user's guide

TeleSAPR draws as SolveSpace does: sketches in workplanes, held by constraints, turned into
solids by groups that each add a step to the model's history. SolveSpace's own
[tutorials](https://solvespace.com/tutorial.pl) teach that part, and hold for TeleSAPR.

These pages cover what TeleSAPR adds. Each says how to use a feature from the window, and from a
script, and ends with what it does not do yet. Each is built on an example under
[`examples/scripts`](../../examples/scripts), which runs without a window:

    telesapr-cli run examples/scripts/fillet/fillet.go

or in the application, from Script > Run File, or with `telesapr --script <file>`. Every
feature can be scripted; the console (Script > Show Console) takes the same Go, a line at a
time, and `help()` there lists what a script can call.

The menus run in the order of the work. Sketch draws and Constrain holds what is drawn; Features
makes the history's groups (sketches, then the solids that create, modify and pattern, then
moving a group in the history); Assembly links files, mates them and checks them against each
other; Analyze measures and checks. View > Show shows and hides what the icons at the top of the
property browser do. An entry with nothing to work on, as Paste with an empty clipboard, is greyed
and its key does nothing. Edit > Find Command (Ctrl+K) finds any entry by words of its name, and
lists last, with the reason, those that can't run now. A right click offers what applies to what
is under the pointer: on an edge of the solid, Fillet, Chamfer and Sweep Along Edges; on a face,
Shell, Mirror and Sketch On Face; on a dimension, Edit Value; and on anything, a Group submenu to activate, hide,
suppress or delete the group it belongs to. The arrow keys move what is selected by a pixel of the view, ten with
Shift, as a drag does (`Entity.MoveBy` in a script). The toolbar at the left has buttons for the features that
take a selection (fillet, chamfer, shell, sweep, loft, mirror, pattern) and for measuring and the
section view; it takes three columns when the window is too short for two. The property browser's
first line says where its screen is ("home › g004-extrude"), and a group's screen offers, beside
rename and delete, to make it active, hide it, or move it earlier or later. Every entry of the
menus has a script call, or is the window's own (its view and its panels): Doc.Copy and Doc.Paste,
Doc.CurveLength, Group.Area, Doc.FacesArea and Group.CenterOfMass are the Edit and Analyze
entries' (`examples/scripts/measure`). A script
names an entry as the command search lists it, `doc.RunCommand("Features > Extrude")`; the names
the entries had before the menus took this order are still taken.

| Page | What it covers | Example |
|---|---|---|
| [Parameters and units](parameters-and-units.md) | Named values that drive dimensions; configurations; a motion study; dimensions read by name; the document's unit of length; measuring | `plate`, `configurations`, `motion`, `dimensions`, `feedback` |
| [Constraints while drawing](drawing.md) | Tangent, perpendicular, parallel and equal inferred as a line is drawn | (none) |
| [Minimum and maximum dimensions](limits.md) | Distances, lengths, radii and angles bounded from one side; a point kept between a segment's ends | `limits` |
| [History](history.md) | Moving a group, suppressing one with what uses it, broken references | `history` |
| [Appearance and shading](appearance.md) | Matte, plastic, glossy and metal faces, lit by a shader | `appearance` |
| [Sketch on a face](sketch-on-face.md) | A sketch in a flat face of the solid, following it | `pedestal` |
| [Section view](section-view.md) | Cutting the model open to look inside | `section` |
| [Mirror](mirror.md) | A group reflected in a plane | `mirror` |
| [Chamfer](chamfer.md) | Edges cut at a distance | `chamfer`, `edges` |
| [Fillet](fillet.md) | Edges rounded, or filled when they go in | `fillet`, `roundbox`, `tray` |
| [Shell](shell.md) | A solid hollowed to walls | `shell`, `pot` |
| [Draft](draft.md) | An extrusion's sides leaned for a mould | `draft` |
| [Loft](loft.md) | Two profiles joined by ruled faces | `loft` |
| [Sweep](sweep.md) | A profile moved along a path | `sweep` |
| [Pattern along a path](pattern.md) | A solid repeated along a path | `pattern` |
| [Screw threads](threads.md) | Metric, unified and trapezoidal threads | `bolt` |
| [Drawings](drawings.md) | A sheet of the standard views, sections and details with a title block, to PDF and DXF | `drawing` |
| [STEP links](step-links.md) | A STEP file, or an STL or OBJ mesh, linked as a reference | `step` |
| [Assemblies](bill-of-materials.md) | Mates; the parts counted, to a CSV file; interfering parts by their shells; STEP with its structure | `mates`, `bom` |
| [Check Solid](check-solid.md) | Whether a solid is sound, and its volume | every example |

Files are saved as `.tlspr`, a text format described in [`docs/tlspr-format.md`](../tlspr-format.md).
SolveSpace's `.slvs` files are imported (File > Import, or `telesapr-cli convert`) and never
written. Drawing with an AI model is in [`docs/ai.md`](../ai.md).
