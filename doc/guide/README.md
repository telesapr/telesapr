# TeleSAPR user's guide

TeleSAPR draws the way SolveSpace does. You sketch in a workplane, hold the sketch with
constraints, and turn it into a solid with groups. Each group adds one step to the model's
history. SolveSpace's own [tutorials](https://solvespace.com/tutorial.pl) teach that part, and
everything in them holds for TeleSAPR.

These pages cover what TeleSAPR adds. Each page says how to use a feature from the window and
from a script, and ends with what the feature does not do yet. Each page is built on an example
under [`examples/scripts`](../../examples/scripts). An example runs without a window:

    telesapr-cli run examples/scripts/fillet/fillet.go

or in the application, from Script > Run File, or with `telesapr --script <file>`.

New to TeleSAPR? Start with [Getting started](getting-started.md), which builds a first part
step by step, and [The window](interface.md), a tour of what is on the screen. The
[menu reference](reference/menus.md), [keyboard shortcuts](reference/shortcuts.md) and
[toolbar](reference/toolbar.md) pages are written from the program itself, so they list every
entry as it is. `make guide-site` builds these pages as a web site in `site/guide`.

Every feature can be scripted. The console (Script > Show Console, Ctrl+Shift+K) takes the same
Go, one line at a time, and `help()` there lists what a script can call.

## Finding your way around the window

**The menus** run in the order of the work:

- **Sketch** draws, and **Constrain** holds what is drawn.
- **Features** makes the groups of the history: sketches first, then the solids that create,
  change and repeat them, then the entries that move a group in the history.
- **Assembly** links files, mates them, and checks them against each other.
- **Analyze** measures and checks.
- **View > Show** shows and hides what the icons at the top of the property browser do. It also
  shows and hides reference dimensions and comments apart from the other constraints.

An entry with nothing to work on is greyed out, and its key does nothing. Paste with an empty
clipboard is one example.

**Edit > Find Command** (Ctrl+K) finds any entry by words of its name. Entries that can't run
right now are listed last, with the reason.

**A right click** offers what applies to what is under the pointer:

- on an edge of the solid: Fillet Edges, Chamfer Edges and Sweep Along Edges;
- on a face: Shell Solid Open Here, Select Face Edges, Sketch On Face and the face's appearance;
  Mirror in This Plane on a flat face an entity makes; Draft Selected Sides while a drafted
  extrusion is active; and Wrap Sketch Onto This Face while a sketch is active;
- on a cylindrical face or a circular edge: Screw Thread;
- on a dimension: Edit Value;
- on anything: a Group submenu to activate, hide, suppress or delete the group it belongs to.

**The keyboard and the mouse.** The arrow keys move what is selected by one pixel of the view,
or ten with Shift, as a drag does (`Entity.MoveBy` in a script). A rectangle dragged from left to
right selects what lies wholly inside it. Dragged from right to left, it is drawn in the
selection's colour and selects whatever it crosses.

**The toolbar** at the left has buttons for the features that take a selection (fillet, chamfer,
shell, sweep, loft, mirror, pattern), for measuring, and for the section view. It takes three
columns when the window is too short for two.

**The property browser's** first line says where its screen is, as in "home › g004-extrude". A
group's screen offers, beside rename and delete, to make the group active, hide it, or move it
earlier or later.

**Changing what a feature is built from.** A fillet, chamfer, shell, sweep, pattern, loft or
mirror can be given other edges, faces, a path, profiles or a plane after it is made. On the
group's screen, [change] takes the current selection. From a script, the typed group has
`SetEdges`, `SetFaces`, `SetPath`, `SetProfiles` and `SetPlane`. See
[`examples/scripts/rework`](../../examples/scripts/rework/rework.go).

**Menu entries from a script.** Every entry of the menus has a script call, unless it belongs to
the window itself (its view and its panels). The Edit and Analyze entries are `Edit.Copy`,
`Edit.Paste`, `Analyze.CurveLength`, `GroupAnalysis.Area`, `Analyze.FacesArea` and
`GroupAnalysis.CenterOfMass` (see [`examples/scripts/measure`](../../examples/scripts/measure)).
A script can also run an entry by the name the command search lists it under:
`doc.Edit.RunCommand("Features > Extrude")`. The names the entries had before the menus took
this order still work.

## The pages

| Page | What it covers | Example |
|---|---|---|
| [Parameters and units](parameters-and-units.md) | Named values that drive dimensions; an extrusion's depth and a revolve's angle held; configurations; a motion study; dimensions read by name; the document's unit of length; measuring | `plate`, `extent`, `configurations`, `motion`, `dimensions`, `feedback` |
| [Constraints while drawing](drawing.md) | Tangent, perpendicular, parallel and equal, added by themselves as you draw | `autoconstrain` |
| [Constraints](constraints.md) | Ratios and differences of lengths, equal distances and angles, distances along a direction, constraints on faces, what lies at a point, a text's font | `ratio`, `equal`, `text`, `overlap` |
| [Minimum and maximum dimensions](limits.md) | Distances, lengths, radii and angles bounded from one side; a point kept between a segment's ends | `limits` |
| [History](history.md) | Moving a group, suppressing a group with what uses it, broken references | `history` |
| [Appearance and shading](appearance.md) | Matte, plastic, glossy and metal faces, lit by a shader; glTF with the colours | `appearance`, `gltf` |
| [Sketch on a face](sketch-on-face.md) | A sketch in a flat face of the solid, which follows the face | `pedestal` |
| [Section view](section-view.md) | Cutting the model open to look inside | `section` |
| [Two-sided groups](two-sided.md) | An extrusion, revolve or repeat run both ways from its sketch | `spacer` |
| [Mirror](mirror.md) | A group reflected in a plane | `mirror` |
| [Chamfer](chamfer.md) | Edges cut at a distance | `chamfer`, `edges` |
| [Fillet](fillet.md) | Edges rounded, or filled where they go in | `fillet`, `roundbox`, `tray` |
| [Shell](shell.md) | A solid hollowed to walls | `shell`, `pot` |
| [Draft](draft.md) | An extrusion's sides leaned for a mould | `draft` |
| [Loft](loft.md) | Profiles joined by ruled faces or by one smooth surface | `loft` |
| [Sweep](sweep.md) | A profile moved along a path | `sweep` |
| [Pattern along a path](pattern.md) | A solid repeated along a path | `pattern` |
| [Screw threads](threads.md) | Metric, unified, trapezoidal, Whitworth, pipe, ACME and buttress threads | `bolt` |
| [Wrap and emboss](wrap.md) | A sketch rolled onto a cylinder or a cone, raised from it or sunk into it | `wrap` |
| [Drawings](drawings.md) | A sheet of the standard views, sections and details with a title block, to PDF and DXF | `drawing` |
| [STEP links](step-links.md) | A STEP file, or an STL or OBJ mesh, linked as a reference or as a solid | `step` |
| [Datums](datums.md) | Names for what a part offers, on entities or on its solid's faces and edges; mates by name | `datums`, `datums_solid` |
| [Assemblies](bill-of-materials.md) | Links relinked, or given a configuration and parameter values; mates, a screw among them; the parts counted, with their masses, to a CSV file; interfering parts; STEP with its structure | `partsizes`, `mates`, `screw`, `bom` |
| [Density and mass](mass.md) | A density for each part; the mass, centre of mass and inertia of each and of the whole | `mass` |
| [Scripts](scripts.md) | The console's keys; forms that ask for a script's values; packages of your own in a script library folder; a model written as a script | `form`, `library`, `exportscript`, `describe` |
| [Drawing with an AI model](ai.md) | Letting an AI program draw in your window over the Model Context Protocol; what it can do, what it may touch | (none) |
| [Clearance and walls](clearance.md) | Gaps between parts and walls thinner than a limit, before printing or machining | `clearance` |
| [Export and import](export-import.md) | Every export, mesh quality presets and strict meshes, sections, DXF import, the command line | `meshquality`, `dxfsection`, `gltf` |
| [Check Solid](check-solid.md) | Whether a solid is sound, and its volume | every example |

A few examples show things no page is about:
[`bracket`](../../examples/scripts/bracket/bracket.go), a plate with a row of holes that takes
its width and hole count from the command line;
[`workplanes`](../../examples/scripts/workplanes/workplanes.go), workplanes that follow their
geometry, and an axis given as a point and a direction;
[`styles`](../../examples/scripts/styles/styles.go), line styles of your own;
[`spline`](../../examples/scripts/spline/spline.go), a cubic spline's points added and taken
away; [`solve`](../../examples/scripts/solve/solve.go), how a group is solved and the options
its screen carries; and [`imported`](../../examples/scripts/imported/imported.go), what a DXF
import draws and the window does not.

## Files

Files are saved as `.tlspr`, a text format described in
[`doc/tlspr-format.md`](../tlspr-format.md). SolveSpace's `.slvs` files are imported (File >
Import, or `telesapr-cli convert`) and never written.
