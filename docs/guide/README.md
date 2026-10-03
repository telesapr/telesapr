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

| Page | What it covers | Example |
|---|---|---|
| [Parameters and units](parameters-and-units.md) | Named values that drive dimensions; dimensions read by name; the document's unit of length; measuring | `plate`, `dimensions`, `feedback` |
| [History](history.md) | Moving a group, suppressing one with what uses it, broken references | `history` |
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
| [STEP links](step-links.md) | A STEP file, or an STL or OBJ mesh, linked as a reference | `step` |
| [Bill of materials](bill-of-materials.md) | The parts an assembly links, counted, to a CSV file | `bom` |
| [Check Solid](check-solid.md) | Whether a solid is sound, and its volume | every example |

Files are saved as `.tlspr`, a text format described in [`docs/tlspr-format.md`](../tlspr-format.md).
SolveSpace's `.slvs` files are imported (File > Import, or `telesapr-cli convert`) and never
written. Drawing with an AI model is in [`docs/ai.md`](../ai.md).
