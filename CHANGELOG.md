# Changelog

What changed in each release of TeleSAPR, newest first. The text of a version's section is the
body of its GitHub release (`make release`, `doc/releasing.md`).

Write an entry in the commit that makes the change, under `## [Unreleased]`, in the group it
belongs to: `Added`, `Changed`, `Fixed` or `Removed`. At the version bump, rename `Unreleased`
to `## [X.Y.Z] - YYYY-MM-DD` and put a new empty `## [Unreleased]` above it.

## [Unreleased]

### Changed
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
