# Screw threads: where they stand, November 2026

What TeleSAPR's thread feature covers, how it compares with FreeCAD and build123d
(`bd_warehouse.thread`), and what is still missing. The first version of this note was a
hands-on evaluation, written 2026-10-23 on `devel` at `ef635b21`. It found five defects and
ranked eight recommendations. The five defects and recommendations 2 to 7 were built in
`4787a6b3`, and the buttress forms, the tables' classes and the full-length follow came after.
This version, of 2026-11-03 at `c4e18d1a`, keeps only what is still true and what is still open.
For what was fixed, see `CHANGELOG.md`, the thread rows of `doc/dev-notes/done-audit.md`, and
`git show 4787a6b3:doc/notes/thread-evaluation.md` for the evaluation as first written.

## What TeleSAPR has

- **Forms and series.** ISO 68-1 60° (metric and unified inch), ISO 2904 trapezoidal, Whitworth
  (BSW, BSF) and ISO 228-1 parallel pipe (G), ASME B1.5 ACME, and buttress: DIN 513 (`S20x4`,
  3°/30°) and ASME B1.9 (`1-12 BUTT`, 7°/45°, `PUSH-BUTT`/`PULL-BUTT`, rounded roots on request).
  The tables cover ISO 261 coarse and fine pitches and ISO 2902 trapezoidal pitches.
  `ThreadSizes`, `ThreadPitches` and `ThreadClasses` list them. Multi-start threads are written
  as `M16xPh3P1.5`, with level starts if wanted. Left-hand is a choice, or `-LH` in the
  designation.
- **Where it starts.** A sketch circle or arc, a cylindrical face, or a circular edge
  (`ThreadOnFace`, `ThreadOnEdge`). On a face the end and the direction are worked out from the
  face. `FullLength` runs the thread the face's whole length and follows the face when the part
  changes.
- **Options.** Internal or external. The length is a number or a parameter expression, or
  through the far end. The crest is standard, truncated or rounded (for buttress threads too).
  Other options: a run-out, a 45° chamfer at the free end, a phase, the load flank turned back,
  and a clearance or a tolerance class. Classes come from the standards' tables: ISO 965-1,
  ISO 2903, ASME B1.1, B1.5 (2G–5G), B1.9 (2A/3A, 2B/3B) and DIN 513-3.
- **Cosmetic or modelled.** A cosmetic thread has no helix: the part is left at its nominal
  size, marked in 3D, and drawn by the convention.
- **Fit.** `Assembly.Screw` fits a nut's ridges between a bolt's. `Assembly.Interferences`
  reports threads that collide.
- **In the window.** Features > Screw Thread... (Shift+E), a toolbar button, the context menu and
  command search. A series, size and pitch picker; words with units; the thread's own summary.
  The whole thread is one row of the history, which shows, suppresses, moves and deletes it as
  one. `[edit thread]` edits it in one undo step.
- **Drawings.** The ISO 6410 convention: a clean ¾ root arc, ISO 128 line weights in
  SVG/DXF/PDF/EPS, and the run-out's end line. "Threads as modelled" is the alternative.
- **Scripts and AI clients.** `Features.Thread` / `ThreadOnFace` / `ThreadOnEdge` with
  `ThreadOptions`, `AsThread().Edit`, and `ThreadGroup.Finished` (the group whose solid is the
  whole thread). MCP `run_script` returns what the script printed (`Output`, `Value`).
- **Refusals.** A thread longer than its part, a thread run off its part, or a rod thinner than
  its root is refused. A part made shorter later becomes a problem on the thread.

Rechecked 2026-11-03: two of the first pass's sharper findings no longer reproduce.

- An M10 nut whose Ø10 hole is drawn before it is threaded now comes out sound, at clearance 0
  and at 0.2.
- An M10 × 30 bolt now passes `export-mesh --strict` at the CLI's default 1 mm chord.

## Comparison

The FreeCAD and bd_warehouse columns are from the survey of October 2026 (FreeCAD 1.0/1.1,
bd_warehouse 0.4.0) and have not been rechecked.

| | TeleSAPR | FreeCAD 1.0/1.1 | build123d / bd_warehouse 0.4.0 |
|---|---|---|---|
| Forms | ISO 60°, unified, trapezoidal, BSW/BSF, G, ACME, DIN 513 and B1.9 buttress | Hole: ISO metric, fine, UNC/UNF/UNEF, NPT, BSP, BSW, BSF, ISO tyre; external only by helix + sketch by hand, or Fasteners WB | ISO 60°, Whitworth 55°, BSPP, ACME 29°, metric trapezoidal 30°, plastic bottle (ASTM D2911, PCO1881), any profile through `Thread` |
| Standard tables | ISO 261 coarse and fine, ISO 2902, UNC/UNF, BSW/BSF, G, ACME, buttress | per type in the Hole; Fasteners WB sizes | BSPP and metric trapezoidal only; `IsoThread` takes diameter and pitch |
| Started from | circle, cylindrical face, circular edge | the Hole's sketch | nothing (placed by code) |
| Internal / external | both, one feature | internal in the Hole; external by hand | both |
| Multi-start | yes, level starts too | no | `TrapezoidalThread` only |
| Taper | no | Hole `Tapered` (ISO 7-1 / ASME B1.20.1 angle set in 1.1) | `taper_angle` raises "not currently supported" |
| End finish | chamfer; run-out cone | none beyond DIN 76 depth | raw / fade / square / chamfer |
| Crest | standard / truncated / rounded; B1.9 rounded root | standard | apex width/radius free |
| Fit | classes from the standards' tables, or clearance in mm; phase; screw mate | ThreadClass, ThreadFit, custom clearance | `interference`, `manufacturing_compensation` |
| Editing | one feature, one row, one undo step, expressions, follows its face | Hole properties, expressions | re-run the code |
| Drawing | ISO 6410 convention or modelled, tied to the feature | TechDraw thread line tools by hand, not linked to the Hole | none (no drawings) |
| Cosmetic | yes | Hole `CosmeticThread`; thread texture after 1.1 (PR 22573, 26.3) | no; `simple=True` fasteners leave a plain cylinder |
| STEP / STL | closed solid; no thread attribute | through OCCT | through OCCT; threads a Compound unless fused |
| Robustness | sound in the tested grids, including off-grid run-outs; chamfered nuts at coarse tolerances fail (below) | "Model Thread works only if Reversed is not set"; the helix wiki calls coincident surfaces "fragile" | #23 internal thread loses a loop on fuse (open), #34 threaded hole "very difficult for the OCCT CAD kernel" (open), #63, #69 open |
| Speed (Oct 2026) | 0.15–0.4 s, 0.8 s for M24 × 100 | "will increase recomputation time significantly"; 0.20 "UI becomes quite unresponsive" (1.1 faster) | raw 0.018 s, fade 0.087 s, square 0.370 s, chamfer 1.641 s (docs' table) |
| Fastener library | no | Fasteners WB (`Thread` "Generate real thread", off by default), Hole | `Screw`, `Nut`, `ClearanceHole`, `TapHole`, `ThreadedHole`, `InsertHole`, many head types |
| Scripting | `Features.Thread` / `ThreadOnFace` / `ThreadOnEdge`, `AsThread().Edit`, size/pitch/class lists | Hole properties from Python | classes, no selection needed |
| AI / MCP | built in: help, run, render with sections, measure | an MCP server exists outside the project (not tested) | none built in; an agent writes Python |

TeleSAPR now matches or passes both in forms, classes and editing. Its modelled threads are
the ones dependably fused into sound solids. It still lacks what FreeCAD's Hole and
bd_warehouse's library have: tapers and ready-made fasteners.

## What is left

**In ROADMAP's "Thread leftovers"** (details and approaches there):

- DIN 513-3's nut deviation. 7H/8H/9H are taken as basic until DIN 513-1's axial play `a` is
  found.
- B1.9's preferred pitches and the default pitch per size, which wait for B1.9 Table 1.
- Chamfered nuts with B1.9 rounded roots, unsound at every chord tolerance for some sizes.
- Multi-start allowances for B1.5 and B1.9, which wait for B1.5 appendix B's K factor.

Chamfered nuts at coarse chord tolerances, unsound where the chamfer's cone met the ridges
(buttress nuts from 0.2%, ISO, UNC and trapezoidal nuts from 0.5–1%), were fixed in `a68c4c8b`: a
chamfered thread, and a buttress nut, is made at a tolerance of its own, a twentieth of the pitch
or a tenth of a rounded crest's radius (`threadSolidTol`, `pkg/sketch/thread_tol.go`).

**Against the others, parked in ROADMAP** ("Never asked for"):

- Tapered threads (NPT, BSPT), which FreeCAD's Hole has. This is a kernel question: a conical
  helix sweep.
- A fastener library (bolt, nut, tapped and clearance holes) built on the feature, as FreeCAD's
  Fasteners WB and bd_warehouse have.
- A thread attribute in STEP, so that a cosmetic thread reaches other CAD systems as one.
- Variable pitch.

**Not in the roadmap, from the comparison**, for whoever wants them:

- Other end finishes: bd_warehouse's fade and square ends.
- Bottle threads (ASTM D2911, PCO1881), or any user profile swept as a thread.

**Not rechecked since 2026-10-23**:

- `Assembly.Interferences` took 15.4 s on an exactly touching bolt and nut.
- In `render`'s section views, a ZX section seen "front" is end-on, and "top" is the view that
  shows the profile. That naming took an AI client a try to get right.
- The interface row in ROADMAP about a mirrored "M10-cosmetic" label in a guide picture is
  thread-related, but it is tracked there.

## Sources

build123d / bd_warehouse:
- bd_warehouse thread docs: https://bd-warehouse.readthedocs.io/en/latest/thread.html
- thread.py: https://github.com/gumyr/bd_warehouse/blob/main/src/bd_warehouse/thread.py
- releases (0.4.0): https://github.com/gumyr/bd_warehouse/releases
- issues: https://github.com/gumyr/bd_warehouse/issues/23, /34, /79 (closed 2026-07-21, b1ca1bb), /63, /69, /22
- fasteners: https://bd-warehouse.readthedocs.io/en/latest/fastener.html
- `Helix`: https://github.com/gumyr/build123d/blob/dev/src/build123d/objects_curve.py

FreeCAD:
- Hole feature: https://github.com/FreeCAD/FreeCAD/blob/main/src/Mod/PartDesign/App/FeatureHole.cpp
- Hole wiki (mirror): https://github.com/FreeCAD/FreeCAD-documentation/blob/main/wiki/PartDesign_Hole.md
- thread texture, PR 22573: https://github.com/FreeCAD/FreeCAD/pull/22573
- modelled threads, PR 4274: https://github.com/FreeCAD/FreeCAD/pull/4274
- 1.1 release notes: https://github.com/FreeCAD/FreeCAD-documentation/blob/main/wiki/Release_notes_1.1.md
- Additive Helix: https://github.com/FreeCAD/FreeCAD-documentation/blob/main/wiki/PartDesign_AdditiveHelix.md
- Fasteners WB: https://github.com/shaise/FreeCAD_FastenersWB/blob/master/FastenersCmd.py
- TechDraw thread lines: https://github.com/FreeCAD/FreeCAD-documentation/blob/main/wiki/TechDraw_ExtensionThreadHoleSide.md

Thread standards used for the classes: see ROADMAP's "Thread leftovers" (DIN 513-3:2020,
NBS H28 (1957) Part III, Machinery's Handbook, Engineers Edge's ACME rows).
