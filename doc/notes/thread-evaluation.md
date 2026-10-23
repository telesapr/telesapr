# Screw threads: an evaluation, October 2026

What TeleSAPR's thread feature covers, how it is to work with from the window, a script and an AI
client, and how it stands against FreeCAD and build123d (`bd_warehouse.thread`). Written
2026-10-23 on `devel` at `ef635b21`; the plan was `doc/notes/thread-evaluation-plan.md`. No code
was changed; the scratch scripts and outputs of the hands-on pass were kept outside the repository.

## Since

Worked on branch `thread-improvements` the same day: the five defects fixed (an interference
false negative, run-outs off the grid, a thread longer than its part refused, the drawing's
convention and weights, `run_script`'s output), and recommendations 2 to 7 built: threads on a
face or edge, a size and pitch picker with ISO 261 fine pitches, Whitworth, pipe and ACME forms,
cosmetic threads, a chamfered end, classes, phase and screw mates that fit a nut to its bolt,
clearer words, the thread's own summary and one row in the history. Not built: buttress, taper,
a fastener library. What is left is in ROADMAP's "Thread leftovers". The text below is the
evaluation as written.

## Summary

- **The geometry is the strong part.** ISO metric (coarse, fine, multi-start, left-hand), unified
  inch and ISO trapezoidal threads, internal and external, with shaped crests, a die run-out and
  level starts, built as sound B-rep solids in 0.15–0.4 s, exported to STEP as closed solids, and
  edited as one feature in one undo step. Neither FreeCAD nor bd_warehouse has a run-out, and
  neither makes a modelled thread a fused, closed solid as dependably.
- **The coverage is narrower than both.** No Whitworth/BSP/NPT, no ACME, no buttress or bottle
  thread, no taper, no end chamfer, no fit class, no fastener library, and no cosmetic thread:
  every thread is modelled.
- **Working with it is where it loses most.** The thread is started from a sketch circle only (not
  a cylinder face), the size is typed with no list to pick from, the screen's words are terse,
  and from a script the circle has to be found by looping over entities.
- **The hands-on pass found five defects** worth fixing before anything new: unsound run-outs
  outside the tested grid, no check of a thread longer than its part, a false negative in
  `Assembly.Interferences`, a gap in the drawing's convention, and the MCP `run_script` result
  losing what the script printed (all under "Defects found").

## Method

| Pass | How | Output |
|---|---|---|
| Window | `TELESAPR_UI_SCRIPT=<name> go test ./pkg/ui -run TestUIScript` for `ext_thread`, `ext_thread_edit`, `ext_thread_runout`, `ext_drawing_thread`; the messages read from `pkg/ui/thread.go`, `pkg/sketch/thread.go`, `pkg/thread/designation.go` | all four pass, 0.35–1.8 s each |
| Script | a fresh `telesapr-cli`, the `bolt`, `threadedit`, `threadrunout` examples, and scratch scripts over 25 cases, a bolt-and-nut fit, exports and a drawing | below |
| AI client | the `telesapr` MCP server on a window started with `--agent`: a rod and an M10 thread by `run_script`, a sweep of length × run-out by `ThreadGroup.Edit`, `render` with a section, `measure` | below |
| Others | the bd_warehouse 0.4.0 source and docs, FreeCAD's `FeatureHole.cpp`, its wiki (through the FreeCAD-documentation mirror; the wiki itself refuses a fetch), release notes and PRs, the Fasteners workbench source, the Fusion, Onshape and SolidWorks help | Sources |

## What TeleSAPR has

- **Forms.** ISO 68-1 60° (metric and unified inch share the profile) and ISO 2904 trapezoidal
  30°. **Tables:** ISO 261 coarse M1–M64, Tr8–Tr60 at the preferred pitch, UNC/UNF #0–2".
  Fine pitch is typed (`M8x1`), not tabled; multi-start as `M16xPh3P1.5` or `(2 starts)`;
  left-hand by `-LH`.
- **Options** (`ThreadOptions`): internal/external, length as a number or a parameter expression,
  through the far end, against the circle's normal, clearance, crest standard/truncated/rounded,
  run-out, level starts.
- **How it is built.** A sketch and a lathe that turn the part down or bore it, a ridge sketch and
  a helix that unions or cuts it: four groups, plus two per further level start. The helix group
  keeps the options (`GroupThread`); `[edit thread]` and `ThreadGroup.Edit` rebuild them in one
  undo step, keeping handles so later groups stay attached. Saved as a `thread {}` block,
  exported to a script as one call.
- **Drawings.** ISO 6410 convention by default (crest outline, thin root lines, a ¾ root circle
  end-on, a nut's root hidden), with "threads as modelled" as the alternative.
- **Tests.** Analytic volume, a 400-case chord-tolerance sweep, a 72-case run-out sweep
  (`TestThreadRunOutSweep`, 8 sampled unless `TELESAPR_SWEEP=1`), goldens, the four UI
  scenarios, path equivalence, `BenchmarkBuildThread`.

### Hands-on results

Every case below was made by `telesapr-cli` scripts; every success was a sound solid at 0 DOF.

| Case | Outcome | Time |
|---|---|---|
| M8 on a rod (baseline) | 1376.61 mm³ | 230 ms |
| M8x1 | 1402.22 mm³ | 303 ms |
| 1/4-20 UNC, clearance 0.1 | 670.45 mm³ (690.55 with none) | 200 ms |
| Tr20x4 internal through (a nut) | 14620.7 mm³ | 156 ms |
| M8-LH | same volume as right-hand; "M8 LH" is accepted as `M8-LH` | 230 ms |
| M16xPh3P1.5, one helix / level starts | 7498.3 / 7523.6 mm³ | 371 / 352 ms |
| M10 internal through, in a Ø20 block | 2448.29 mm³ | 192 ms |
| crest rounded / truncated | 1375.58 / 1368.49 mm³ | 283 / 227 ms |
| M24 × 100 | 47188.8 mm³, 8176 triangles | 818 ms |
| `M7.5` | "M7.5 isn't a standard size; give its pitch, as in M7.5x1" | – |
| `Tr21` | "Tr21 isn't a standard size; give its pitch, as in Tr21x1" (1 mm is a poor hint for a trapezoidal thread) | – |
| `M0.5` | suggests `M0.5x1`, which is itself refused as too coarse | – |
| length 1.5 on M8 | "thread length must be more than 2.332 mm for this pitch" | – |
| run-out with through | "a through thread has no shoulder to run out at" | – |
| a cylinder face as the target | "a thread is made on a circle or an arc, and face of g003-rod isn't one" | – |
| **thread of 20 on a 10 mm rod** | **accepted; unsound, 314 naked edges** | 265 ms |

| Step (M8, in `eval`) | Time |
|---|---|
| make | 168 ms (+50 ms check) |
| edit to M8x1, length 20 | 239 ms |
| undo / redo of the edit | 186 / 289 ms |
| undo / redo of the make | 1 / 3 ms (+50 ms regeneration) |
| `regenerate` of the bolt example | 0.11 s |

Undo takes a whole thread away (four to eight groups) in one step, in the window and in a script.

**Exports** (M10 × 30 bolt). STL: 1700 triangles at the CLI's default 1 mm chord, which has 122
naked edges and is refused by `--strict`; `standard` (2330 triangles, 117 KB) and finer pass it.
STEP: 338 KB, one closed shell of 60 faces (20 B-spline); read back as a solid it is sound and
within 0.01% in volume, as are a run-out rod and a nut. The "open thread ends in STEP" row of the
roadmap is about imported files: none of our own exports had them.

**Bolt and nut.** An M10 nut fits the M10 rod only when it sits a half pitch (or an odd multiple)
along; at a whole pitch the ridges collide, and nothing in the feature or the API helps with the
phase. With clearance 0.2 the radial gap measured by ray probes is exactly 0.200 mm, and
`Assembly.Clearances` gives 0.094 mm, the flank gap (0.2·sin 30°). Drawing a Ø10 hole in the nut
first and then threading it gives an unsound solid ("where two parts of the solid touch"); with
clearance 0.2 or without the hole it is sound.

## Defects found

1. **Run-outs outside the swept grid fail.** An M10 × 20 thread with a 2 mm run-out (1.33 pitches)
   came out unsound by MCP: 24 naked edges, "the Boolean with the solid so far failed". A sweep of
   length {14, 16, 18, 20, 20.5, 22, 24} × run-out {1, 1.5, 2, 3} gave 2 of 28 unsound, both at
   run-out 2 (lengths 14 and 20); their build took twice as long, so the retry over other ridge
   planes ran and did not help. `doc/guide/threads.md` says every run-out of the sweep is sound,
   which is true of its 72 cases but reads as a general promise. The grid has no M10 and no
   run-out near 4/3 pitch.
2. **A thread longer than its part is accepted.** On a 10 mm rod a 20 mm thread (with or without
   through) gives an unsound solid with no diagnostic before it. It should be a group problem
   ("the thread is longer than the part, N mm") or be clipped to the part.
3. **`Assembly.Interferences` misses a collision.** The nut at a whole pitch along, clearance 0,
   reports no interference, while ray probes find 0.81 mm penetration on 3655 rays; at z = 6.3 it
   reports 31.76 mm³. The exact-contact case (z = 6.75) took 15.4 s. This is the analysis, not the
   thread, but threads are where coincident helical faces make it fail.
4. **The drawing's convention leaves a gap.** In the TOP view of an M10 bolt the threaded length
   is drawn about 1.5 mm apart from the shank, with stray dashed ridge-end fragments; the end view
   shows a full root circle with fragments rather than a clean ¾ arc. Crest and root lines share
   one 0.1 mm style in the SVG, where ISO 6410 wants the root thin and the crest thick.
5. **MCP `run_script` loses what a script prints when it succeeds.** The tool returns both a text
   and a structured result (`RunOutput`), and the structured one has no output or value fields;
   Claude Code shows the structured one, so a successful script's `fmt.Printf` never reaches the
   model (on an error the text is shown, with the output). To read a sweep's numbers the script
   had to end with `panic`. Adding `Output` and `Value` to `RunOutput` (`pkg/mcpserver/server.go`)
   fixes it for every feature, not only threads.

Smaller: the drawing screen says "File > Export Drawing" where the menu is File > Export >
Drawing; `telesapr-cli eval` exits 0 after an error; an error in a `run` script is reported near
the top of `main` rather than at the failing call; size names are normalised quietly (`M8x1.25`
becomes `M8`, `M7x1` becomes `M7`), which is right but could say so.

## Usability

### Window

Features > Screw Thread... (Shift+E), command search; no toolbar button and no context-menu
entry, though the toolbar has `helix`. The screen, from a Ø20 circle:

```
SCREW THREAD
   thread    M20 [change]
             2.50 pitch, right-hand
   length    30.00 [change]
   clearance 0.00 [change]    run-out 0.00 [change]
    external   internal    crest  standard  truncated  rounded
     through the far end
     against the circle's normal
 make the thread now
```

What is good: the nearest standard size is offered, the pitch and hand are spelled out under the
designation, the length takes an expression, and errors are specific ("M7.5 isn't a standard
size; give its pitch, as in M7.5x1"). What is awkward, against what a Fusion, SolidWorks or
Onshape user expects:

| Expected | TeleSAPR | Suggestion |
|---|---|---|
| Pick the cylinder face | a sketch circle or arc only; a face or a solid's edge is "Bad selection" | take a cylindrical face (and its end circle), the way a user points at a shaft |
| A size list, then a pitch list | a free-text `[change]`; the accepted forms are in a doc comment | a list of the table's sizes and, for the chosen size, its pitches (coarse and fine) |
| Full length, or offset + length | length from the circle's plane, "through the far end" | a "full length" toggle that follows the face; an offset |
| Hand | `-LH` typed into the designation | a right/left choice beside it |
| Class / fit (6g/6H, 2A/2B) | clearance in mm | a class that sets the clearance |
| Cosmetic by default, modelled on request | always modelled | a cosmetic thread: a face attribute drawn by the convention and written to STEP, the helix built only when asked |
| End chamfer | none; the ridge stops half a pitch short, blunt | a chamfer end (Onshape and bd_warehouse have one) |
| Clear words | "against the circle's normal", "through the far end", unitless clearance/run-out | "direction: flip", "to the end of the part", units on the fields |
| Confirmation | a sound make says nothing; the screen jumps to the helix group, whose raw `pitch`/`angle` rows can be edited and fight the thread, with `[edit thread]` on row 20 | show the thread's own summary first, hide or lock the raw helix rows |
| One feature in the history | four to eight groups | fold them under one row in the group list |

### Script

```go
var end api.Entity
for _, e := range rod.Entities() {
	if e.Kind() == "circle" && e.Center().Pos().Z > length/2 {
		end = e
	}
}
thread := doc.Features.Thread(end, api.ThreadOptions{Designation: "M10", Length: 20, Reverse: true})
```

The call itself is short and the options struct reads well; `AsThread().Options()`/`Edit` make a
parametric edit two lines. The cost is around it: the circle is found by a loop (a `FaceAt` face
can't be used), `Reverse` has to be guessed from which way the circle's normal points, and with
level starts the group returned holds only the first start, so the finished solid is
`last(t.Groups())`. bd_warehouse's `IsoThread(major_diameter=10, pitch=1.5, length=20)` needs no
selection at all, though it then needs a placement and a fuse.

### AI client

The MCP server is a real advantage: `api_help "Features > Screw Thread"` and `"ThreadOptions"`
explain the call, a rod and a thread came in one `run_script`, `render` with a ZX section and
the top view shows the profile and the run-out cone plainly, and `measure` says sound or not,
naked edges included. Against it: defect 5 (the printed output lost), the dimension the thread
adds to the active sketch (`c01e-proj-pt-pt-distance = 17.2`) drawn across every render, and the
section view's naming (a ZX section seen "front" is end-on; "top" is the one that shows the
profile), which took a try.

## Comparison

| | TeleSAPR | FreeCAD 1.0/1.1 | build123d / bd_warehouse 0.4.0 |
|---|---|---|---|
| Forms | ISO 60°, unified, ISO trapezoidal | Hole: ISO metric, fine, UNC/UNF/UNEF, NPT, BSP, BSW, BSF, ISO tyre; external only by helix + sketch by hand, or Fasteners WB | ISO 60°, Whitworth 55°, BSPP, ACME 29°, metric trapezoidal 30°, plastic bottle (ASTM D2911, PCO1881), any profile through `Thread` |
| Standard tables | ISO 261 coarse, Tr preferred, UNC/UNF | per type in the Hole; Fasteners WB sizes | BSPP and metric trapezoidal only; `IsoThread` takes diameter and pitch |
| Internal / external | both, one feature | internal in the Hole; external by hand | both |
| Hand | `-LH` | Right/Left | `hand` |
| Multi-start | yes, level starts too | no | `TrapezoidalThread` only |
| Taper | no | Hole `Tapered` (ISO 7-1 / ASME B1.20.1 angle set in 1.1) | `taper_angle` raises "not currently supported" |
| End finish | blunt half pitch short | none beyond DIN 76 depth | raw / fade / square / chamfer |
| Run-out | die run-out cone | no | no (fade is the nearest) |
| Crest | standard / truncated / rounded | standard | apex width/radius free |
| Fit | clearance in mm | ThreadClass, ThreadFit, custom clearance | `interference`, `manufacturing_compensation` |
| Editing | one feature, one undo step, expressions | Hole properties, expressions | re-run the code |
| Drawing | ISO 6410 convention or modelled, tied to the feature | TechDraw thread line tools by hand, not linked to the Hole | none (no drawings) |
| Cosmetic | no | Hole `CosmeticThread`; thread texture after 1.1 (PR 22573, 26.3) | no; `simple=True` fasteners leave a plain cylinder |
| STEP / STL | closed solid; STL sound at `standard` | through OCCT | through OCCT; threads a Compound unless fused |
| Robustness | sound in the tested grids; 2/28 run-outs off-grid failed, no length check | "Model Thread works only if Reversed is not set"; the helix wiki calls coincident surfaces "fragile" | #23 internal thread loses a loop on fuse (open), #34 threaded hole "very difficult for the OCCT CAD kernel" (open), #63, #69 open |
| Speed | 0.15–0.4 s, 0.8 s for M24 × 100 | "will increase recomputation time significantly"; 0.20 "UI becomes quite unresponsive" (1.1 faster) | raw 0.018 s, fade 0.087 s, square 0.370 s, chamfer 1.641 s (docs' table) |
| Fastener library | no | Fasteners WB (`Thread` "Generate real thread", off by default), Hole | `Screw`, `Nut`, `ClearanceHole`, `TapHole`, `ThreadedHole`, `InsertHole`, many head types |
| Scripting | `Features.Thread(circle, ThreadOptions)`, `AsThread().Edit` | Hole properties from Python | classes, no selection needed |
| AI / MCP | built in: help, run, render with sections, measure | an MCP server exists outside the project (not tested) | none built in; an agent writes Python |

The two are aimed differently. bd_warehouse is a parts library for code-first users: wide in forms
and end finishes, fast because it rarely fuses, and its maintainer steers users away from fused
threads in holes. FreeCAD puts threads in the Hole, where most threads in parts are, with the
standards and classes a machinist wants, and leaves external threads to the user. TeleSAPR is the
only one of the three whose modelled threads are dependably fused sound solids, with a run-out
and in-place editing, but it is narrower than either and asks more of the user to start one.

## Gaps and recommendations, ranked

1. **Fix the defects above**: the run-out failures (widen the sweep to M10 and run-outs near
   4/3 pitch, then fix), a group problem for a thread longer than its part, the interference
   false negative, the drawing gap and line weights, and `Output`/`Value` in MCP `RunOutput`.
2. **Start from a cylinder face.** Take a cylindrical face or a solid's circular edge; work out the
   end, the diameter and the direction (no more guessing `Reverse`). In scripts, accept a
   `SolidFace`.
3. **A size and pitch picker.** A list of the table's sizes and pitches, a fine-pitch table
   (ISO 261 fine), hand as a choice, and better hints for unknown sizes (a trapezoidal pitch for
   `Tr21`).
4. **A cosmetic thread.** The thread as an attribute of a face: drawn by the convention, written
   to STEP, built as a helix only when asked. It is what every commercial system does by default,
   and it would make threads cheap in assemblies.
5. **End finishes.** A chamfer at the free end (the usual 45° to the root), as bd_warehouse and
   Onshape have.
6. **Fit.** A tolerance class that sets the clearance, and a way to phase a nut to its bolt (a
   mate that aligns the helices, or a "fits on" option for an internal thread).
7. **One row in the history**, the raw helix rows hidden behind `[edit thread]`, a toolbar button
   and a context-menu entry.
8. **More forms**: Whitworth/BSP and NPT (taper first), ACME, buttress, and a fastener library
   (bolt, nut, tapped hole) built on the feature.

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
- ThreadProfile addon: https://github.com/mwganson/ThreadProfile (from search results only)
- Fasteners WB: https://github.com/shaise/FreeCAD_FastenersWB/blob/master/FastenersCmd.py
- TechDraw thread lines: https://github.com/FreeCAD/FreeCAD-documentation/blob/main/wiki/TechDraw_ExtensionThreadHoleSide.md

Commercial:
- Fusion Thread: https://help.autodesk.com/cloudhelp/ENU/Fusion-Model/files/GUID-C37E8172-DE94-4EB0-AFE3-9D3172328441.htm,
  API defaults https://help.autodesk.com/cloudhelp/ENU/Fusion-360-API/files/ThreadFeatureInput.htm
- Fusion thread data XML (community, no official how-to found): https://www.cadforum.cz/en/how-to-add-special-thread-types-to-fusion-for-3d-printing-din477-tip15110
- Onshape External Thread (from search snippets): https://cad.onshape.com/help/Content/PartStudio/external_thread.htm
- SolidWorks Thread and Cosmetic Thread (unverified, the help pages load by script):
  https://help.solidworks.com/2019/English/SolidWorks/sldworks/r_thread_propertymanager.htm,
  https://help.solidworks.com/2012/English/SolidWorks/sldworks/HIDD_DVE_CTHREAD.htm
- thread 95.9% of a simple part's rebuild (0.38 of 0.39 s, SW 2016/17): https://www.goengineer.com/blog/solidworks-performance-physical-cosmetic-threads
