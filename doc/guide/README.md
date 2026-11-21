# TeleSAPR user's guide

TeleSAPR draws the way SolveSpace does. You sketch in a workplane, hold the sketch with
constraints, and turn it into a solid with groups. Each group is one step of the model's
history. SolveSpace's own [tutorials](https://solvespace.com/tutorial.pl) teach that part, and
everything in them holds for TeleSAPR.

These pages cover what TeleSAPR adds. Each page says how to use a feature from the window and
from a script, and ends with what the feature does not do yet. Each page is built on an example
under [`examples/scripts`](../../examples/scripts). An example runs without a window:

    telesapr-cli run examples/scripts/fillet/fillet.go

or in the application, from Script > Run File, or with `telesapr --script <file>`.

New to TeleSAPR? Start with [Getting started](getting-started.md), which builds a first part
step by step, and [The window](interface.md), a tour of the screen. The
[menu reference](reference/menus.md), [keyboard shortcuts](reference/shortcuts.md) and
[toolbar](reference/toolbar.md) pages are written from the program itself, so they list every
entry as it is. `make guide-site` builds these pages as a web site in `site/guide`.

Every feature can be scripted. The console (Script > Show Console, Ctrl+Shift+K) takes the same
Go, one line at a time. `help()` there lists what a script can call.

## Finding your way around the window

[The window](interface.md) has the detail. In short:

- **The menus** run in the order of the work: **Sketch** draws, **Constrain** holds what is
  drawn, **Features** makes the groups of the history, **Assembly** links and mates files,
  **Analyze** measures and checks.
- An entry with nothing to work on is greyed out, and its key does nothing.
- **Edit > Find Command** (Ctrl+K) finds any entry by words of its name.
- **A right click** offers what applies to what is under the pointer: Fillet Edges on an edge,
  Sketch On Face on a face, Edit Value on a dimension, and a Group submenu everywhere.
- **The toolbar** down the left has a button for each common entry.
- **The property browser** on the right shows the groups, and the screen of the group or
  selection you click.

**Changing what a feature is built from.** A fillet, chamfer, shell, sweep, pattern, loft or
mirror can be given other edges, faces, a path, profiles or a plane after it is made. On the
group's screen, [change] takes the current selection. From a script, the typed group has
`SetEdges`, `SetFaces`, `SetPath`, `SetProfiles` and `SetPlane`. See
[`examples/scripts/rework`](../../examples/scripts/rework/rework.go).

**Menu entries from a script.** Every menu entry has a script call, unless it belongs to the
window itself (its view and its panels). The [menu reference](reference/menus.md) names the
call beside each entry; [`examples/scripts/measure`](../../examples/scripts/measure) uses the
Edit and Analyze ones. A script can also run an entry by its name in the command search:
`doc.Edit.RunCommand("Features > Extrude")`. The names entries had before the menus were
reordered still work.

## The pages

| Page | What it covers | Example |
|---|---|---|
| [Constraints while drawing](drawing.md) | Tangent, perpendicular, parallel and equal, added by themselves as you draw | `autoconstrain` |
| [Constraints](constraints.md) | Ratios and differences of lengths, equal distances and angles, distances along a direction, constraints on faces, what lies at a point, a text's font, large edits and drags | `ratio`, `equal`, `text`, `overlap` |
| [Parameters and units](parameters-and-units.md) | Named values that drive dimensions; an extrusion's depth and a revolve's angle held; configurations; a motion study; dimensions read by name; the document's unit of length; measuring | `plate`, `extent`, `configurations`, `motion`, `dimensions`, `feedback` |
| [Datums](datums.md) | Names for what a part offers, on entities or on its solid's faces and edges; mates by name | `datums`, `datums_solid`, `datums_freeform` |
| [Sketch on a face](sketch-on-face.md) | A sketch in a flat face of the solid, which follows the face | `pedestal` |
| [Minimum and maximum dimensions](limits.md) | Distances, lengths, radii and angles bounded from one side; a point kept between a segment's ends | `limits` |
| [Fillet](fillet.md) | Edges rounded, or filled where they go in | `fillet`, `roundbox`, `tray` |
| [Chamfer](chamfer.md) | Edges cut at a distance | `chamfer`, `edges` |
| [Draft](draft.md) | An extrusion's sides leaned for a mould | `draft` |
| [Shell](shell.md) | A solid hollowed to walls | `shell`, `pot` |
| [Two-sided groups](two-sided.md) | An extrusion, revolve or repeat run both ways from its sketch | `spacer` |
| [Mirror](mirror.md) | A group reflected in a plane | `mirror` |
| [Pattern along a path](pattern.md) | A solid repeated along a path | `pattern` |
| [Loft](loft.md) | Profiles joined by ruled faces or by one smooth surface | `loft` |
| [Sweep](sweep.md) | A profile moved along a path | `sweep` |
| [Wrap and emboss](wrap.md) | A sketch rolled onto a cylinder or a cone, raised from it or sunk into it | `wrap` |
| [Screw threads](threads.md) | Metric, unified, trapezoidal, Whitworth, pipe, ACME and buttress threads | `bolt` |
| [History](history.md) | Moving a group, suppressing a group with what uses it, broken references, damaged files | `history`, `damaged` |
| [Appearance and shading](appearance.md) | Matte, plastic, glossy and metal faces, lit by a shader; glTF with the colours | `appearance`, `gltf` |
| [Section view](section-view.md) | Cutting the model open to look inside | `section` |
| [Check Solid](check-solid.md) | Whether a solid is sound, and its volume | every example |
| [Density and mass](mass.md) | A density for each part; the mass, centre of mass and inertia of each and of the whole | `mass` |
| [Clearance and walls](clearance.md) | Gaps between parts and walls thinner than a limit, before printing or machining | `clearance` |
| [STEP links](step-links.md) | A STEP file, or an STL or OBJ mesh, linked as a reference or as a solid | `step` |
| [Assemblies](bill-of-materials.md) | Links relinked, or given a configuration and parameter values; mates, a screw among them; the parts counted, with their masses, to a CSV file; interfering parts; STEP with its structure | `partsizes`, `mates`, `screw`, `bom` |
| [Mechanisms](mechanisms.md) (experimental) | Linked parts that move together: a lid on its hinge pulled open in the window, an engine's crank and piston driven round through a motion study | `lid`, `engine` |
| [Drawings](drawings.md) | A sheet of the standard views, sections and details with a title block, to PDF and DXF | `drawing` |
| [Export and import](export-import.md) | Every export, mesh quality presets and strict meshes, sections, DXF import, the command line | `meshquality`, `dxfsection`, `gltf` |
| [Scripts](scripts.md) | The console's keys; forms that ask for a script's values; packages of your own in a script library folder; a model written as a script | `form`, `library`, `exportscript`, `describe` |
| [Drawing with an AI model](ai.md) | Letting an AI program draw in your window over the Model Context Protocol; what it can do, what it may touch | (none) |

A few examples show things no page is about:

- [`bracket`](../../examples/scripts/bracket/bracket.go): a plate with a row of holes that
  takes its width and hole count from the command line.
- [`workplanes`](../../examples/scripts/workplanes/workplanes.go): workplanes that follow their
  geometry, and an axis given as a point and a direction.
- [`styles`](../../examples/scripts/styles/styles.go): line styles of your own.
- [`spline`](../../examples/scripts/spline/spline.go): a cubic spline's points added and taken
  away.
- [`solve`](../../examples/scripts/solve/solve.go): how a group is solved, and the options its
  screen carries.
- [`imported`](../../examples/scripts/imported/imported.go): what a DXF import draws and the
  window does not.
- [`measure`](../../examples/scripts/measure): the Edit and Analyze menu entries from a script.
- [`rework`](../../examples/scripts/rework/rework.go): a feature given other edges, faces or a
  path after it is made.

## Files

Files are saved as `.tlspr`, a text format described in
[`doc/tlspr-format.md`](../tlspr-format.md). SolveSpace's `.slvs` files are imported (File >
Import, or `telesapr-cli convert`) and never written.

## In a browser

The project's site, [telesapr.sourceforge.io](https://telesapr.sourceforge.io), runs the same
window in a browser ("Try in browser"), with nothing installed. It differs from the desktop
program in a few ways:

- **Files** are kept in the browser's storage, on that computer only. Clearing the site's data
  removes them. A save dialog also offers the file as a download. Ctrl+S writes to the storage
  only. Saving under the name of a stored file asks before replacing it. The **Files** button
  lists, downloads and deletes what is stored.
- **A large model holds the page** while it regenerates: the window takes no clicks or keys
  until it is done. After a moment a box says how long it has taken, with a bar for how far it
  has got.
- **AI programs** can't connect through `telesapr-cli mcp` (`--agent`). Script > Allow AI
  Connections lets a program in the browser work on the drawing through the page instead (see
  [Drawing with an AI model](ai.md#in-a-browser)).
- **File > Exit** ends the program in the page. **Start Again** loads it afresh, with the files
  kept.
- **Keys.** The browser keeps Ctrl+N, Ctrl+T and Ctrl+W for itself. There, New is Ctrl+Alt+N,
  Paste Transformed Ctrl+Alt+T, Show Naked Edges Ctrl+Alt+Shift+N and Trace Point
  Ctrl+Alt+Shift+T. In full screen (View > Full Screen), Chrome and Edge give the page the Ctrl
  keys too. Menus open with Alt and their underlined letter, as on the desktop.
