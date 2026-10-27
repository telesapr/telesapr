# Changelog

What changed in each release of TeleSAPR, newest first. The text of a version's section is the
body of its GitHub release (`make release`, `doc/releasing.md`).

Write an entry in the commit that makes the change, under `## [Unreleased]`, in the group it
belongs to: `Added`, `Changed`, `Fixed` or `Removed`. At the version bump, rename `Unreleased`
to `## [X.Y.Z] - YYYY-MM-DD` and put a new empty `## [Unreleased]` above it.

## [Unreleased]

### Added
- On a group's screen, an edge of the solid named by the faces it lies between (a chamfer's, a fillet's or a sweep's picked on the model) and a face named by its identity (a shell's, a sketch on a face's, a wrap's) are hovered and selected from their rows, as an entity's row does, on the solid before the group, where the edge still is.
- What a feature is built from can be changed after it is made, without making it again: a fillet's or chamfer's edges, a shell's faces, a sweep's or pattern's path, a loft's profiles and a mirror's plane. On the group's screen, [change] takes the selection as the new inputs (with nothing selected it activates the group before the feature, whose solid the inputs are picked on, and the screen stays while they are) and [remove] on a row takes that one away; in scripts, `SetEdges`, `SetFaces`, `SetPath`, `SetProfiles` and `SetPlane` on the typed group. A change the kernel can't make is refused and nothing moves; a later group built on what the change takes away reports it, as when a group is deleted.

### Removed
- Scripting: `srf.SetChordTolMm`, `srf.SetMaxSegments` and `srf.SetMesher`, which set the tolerance of shells built with none and were meant for tests; a document's chord tolerance and segments are set with `Edit.SetChordTolerance` and `Edit.SetMaxSegments`.

### Fixed
- A screw thread made on a face over its full length follows the face when the part changes afterwards: a rod made longer by a parameter, or a hole drilled deeper, carries a thread as long as it is at the next regeneration, where before the thread kept its old length until it was edited. A face come to reach the part's end, or to stop short of it, or too short for the thread, is reported on the thread's group, and the thread takes it up when edited.
- A menu opened from the keyboard (Alt and a letter, F10, Left and Right) on a very busy machine keeps its first entry active, so Return runs it, instead of sometimes closing without running anything.
- Text drawn in the vector font on two threads at once, as a drawing exported by a script while the window draws, no longer races on the font's glyphs.

## [0.5.3] - 2026-10-25

### Changed
- Scripting: `Style.SetWidthAs` converts the width and the stipple width to the new unit, as the styles screen does, so the lines keep their look; a script sets the unit before the numbers in it.
- Colouring by freedom colours only lines in the active group's own style, and points: a construction line keeps the construction colour, and a line given a style keeps its style's colour.

### Fixed
- Line styles: freehand lines are drawn as a wave and zigzag lines as straight runs broken by a zigzag, as the styles screen shows them, instead of as short dashes and not at all. A dashed line whose stipple width is zero or too small for its dashes to be a pixel apart is drawn continuous instead of not at all.
- Line styles: a negative stipple width, from a script or a file, no longer freezes the window; a script that sets a negative width or stipple width is stopped with an error.
- Line styles: the dots of the dot, dash-dot and dash-dot-dot patterns are as wide as the line, rather than a fraction of a pixel and nearly invisible.
- Deleting a line style on the styles screen redraws what was drawn in it at once.

## [0.5.2] - 2026-10-24

### Fixed
- The Windows and macOS packages build again: 0.5.0 and 0.5.1 were tagged, but their packages were not made because tests of the library and of path handling failed on those systems. Nothing in the program changed since 0.5.1.

## [0.5.1] - 2026-10-24

### Fixed
- Windows: a script can import packages from the script library (the interpreter asked for `src\gears`, which the library did not answer).

## [0.5.0] - 2026-10-24

### Added
- Wrap Sketch Onto Face: emboss or engrave a sketch (text, shapes) on a cylinder, cone or turned face.
- Closed lofts, lofts held square to a middle profile, and lofts that leave a profile along the sides of the solid it is drawn on.
- Datums: named datums on any face or edge of a solid, found through links as `group.name`, and mates on them.
- Mate Screw (a screw mate by a turn about an axis), Analyze > Clearance and Walls (clearance between parts, wall thickness), density per group and mass properties per part.
- Thread run-out and level starts, threads edited as one block on their helix group, and screw threads drawn by the drafting convention in drawings.
- Shell for lathes and revolves; extrusion depth and revolve angle held by a value; closed sweeps.
- glTF (`.glb`) export, File > Export > Script (the model as the script that builds it), and relink for linked parts.
- Scripting: forms a script declares, filled in a dialog, by `--set` or by MCP; a script library folder; file guard for AI programs; new MCP tools (measure, export, open, list_files).
- Chamfers and fillets onto curved faces by the rolling ball, with fitted end sections.

### Changed
- Faster solving and regeneration: systems solved per connected component and kept between solves, solids kept by what they were made from, a faster software renderer.
- A setting to draw a drag's solids late, off the thread.
- DXF import: arcs in 3D get the normal of their plane.

### Fixed
- Deep wraps onto cones join without hair vertices; a section through a turned solid's axis closes.
- Several fillet, chamfer and sweep cases that failed under the race detector or at twisted patches.

## [0.4.13] - 2026-10-21

Earlier releases are listed on the [releases page](https://github.com/telesapr/telesapr/releases).
