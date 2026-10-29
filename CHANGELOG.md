# Changelog

What changed in each release of TeleSAPR, newest first. The text of a version's section is the
body of its GitHub release (`make release`, `doc/releasing.md`).

Write an entry in the commit that makes the change, under `## [Unreleased]`, in the group it
belongs to: `Added`, `Changed`, `Fixed` or `Removed`. At the version bump, rename `Unreleased`
to `## [X.Y.Z] - YYYY-MM-DD` and put a new empty `## [Unreleased]` above it.

## [Unreleased]

### Added
- A modelled screw thread can have a phase and a run-out together: the ridges are moved along the thread by the phase and still fade out into the run-out's cone. Before, a thread with a run-out refused a phase ("a thread with a run-out can't be phased").
- A chamfer's edges can have distances of their own: each edge's row on the chamfer's screen has [own distance] and [own second distance], and scripts have `ChamferGroup.SetEdgeDistance` and `SetEdgeSecondDistance`. An edge with none takes the chamfer's, and swapping the chamfer's distances swaps each edge's too. Before, a chamfer had one distance and one second distance for all its edges.
- A fillet or chamfer whose end runs off its face onto a curved face, such as a band's rounded top, is made: the curved face is carried on past its edge, cuts the blend and grows over its end. A fillet ending on a curved face that a side's face goes on past, as a cone past a band's rounded top, is made the same way. Before, both were refused.
- Two chamfered edges meet at a corner whose third edge stays sharp with any distances: two distances on each, or a different distance on each edge. The chamfers meet along the line where their planes cross, and the one that reaches further down the sharp edge goes on past it. This works where the chamfers cut (a block's top corner) and where they fill (the foot of a block standing on a plate, whose side face grows out to the mitre), as OpenCASCADE's makeChamfer does.
- A datum on a face neither flat nor round, or on an edge neither straight nor round, is taken where the face or edge was clicked, and follows that place as the model changes, rather than at its middle, keeping its end of an edge when an edit turns the edge round. Scripts name such a place with `Datums.NameFaceAt` and `Datums.NameEdgeAt`.
- A revolve of more than half a turn is hollowed with one or both of its ends kept as walls: the inside keeps the thickness from each end and is rounded about the axis, where the ends meet. Before, such a shell was refused unless both ends were taken away.
- Solids made of lathes about one axis joined to each other, as a cup with a flange round its foot or a cylinder with a groove turned out of it, are hollowed by their profiles joined, so the walls follow every lathe exactly. Before, a shell of a lathe with any other solid was refused as a curved solid.
- Buttress threads: DIN 513's metric S threads (`S20x4`, flanks at 3° and 30°) and ASME B1.9's buttress inch threads (`1-12 BUTT`, 7° and 45°), typed or chosen from two new series, `buttress` and `BUTT`. The ridge leans, its load flank facing along the thread on a screw and back in a nut; **load flank back** on the thread's screen (`LoadFlankBack` in scripts) turns it about, and the screw mate refuses two buttress threads whose load flanks face the same way. A buttress crest can be truncated but not rounded, and buttress threads have no tolerance classes.
- A rib can be rounded all over in one edit: its top's edges and its foot picked together. The top is rounded first and the foot's fillet goes on round onto the top's fillets where they reach the floor. A closed foot left by fillets made before is filled whole too, from its edges or one of them. Before, such picks were refused ("a convex edge's fillet meets a concave one's here", or "pick it alone").
- Chamfers meet at any corner where every edge is chamfered, as OpenCASCADE's do: chamfers of two distances at a corner, three edges whose faces don't meet square, and four or more edges at a pyramid's apex. One chamfer ends across between where its lines cross the others', the others are cut square to their edges, and the corner is filled by a plane or a four-sided patch. Before, such corners were refused by the edit and cut by wedges into a point, or refused ("the edge ends where 4 edges meet"). Refusals at a corner of chamfers now say "chamfer" where they said "fillet".
- Fillets of different radii meet in mitres where their edges meet at a sharp corner of a curved wall, or at a corner whose third edge stays sharp: the smaller fillet ends against the larger one on its own wall, and the larger one's end turns onto that wall down to the corner, as other CAD programs make it. Before, edges of different radii meeting so were refused ("pick it alone").
- On a group's screen, an edge of the solid named by the faces it lies between (a chamfer's, a fillet's or a sweep's picked on the model) and a face named by its identity (a shell's, a sketch on a face's, a wrap's) are hovered and selected from their rows, as an entity's row does, on the solid before the group, where the edge still is.
- What a feature is built from can be changed after it is made, without making it again: a fillet's or chamfer's edges, a shell's faces, a sweep's or pattern's path, a loft's profiles and a mirror's plane. On the group's screen, [change] takes the selection as the new inputs (with nothing selected it activates the group before the feature, whose solid the inputs are picked on, and the screen stays while they are) and [remove] on a row takes that one away; in scripts, `SetEdges`, `SetFaces`, `SetPath`, `SetProfiles` and `SetPlane` on the typed group. A change the kernel can't make is refused and nothing moves; a later group built on what the change takes away reports it, as when a group is deleted.

- `telesapr-cli export-section` writes the section of the solid by the plane `--plane` names, `z=15` for a plane square to an axis or six numbers for a point and a normal, or by the workplane active when the model was saved, in any 2d vector format.
- A closed loft can be held square to its profiles between the first and the last, as an open loft is: the ring runs straight through each held profile. Before, a closed loft with a profile held square was refused.
- A closed loft can be held by guide curves, each going round the ring through every profile and closing on itself: Features > Guide Loft by Selected Curves on a closed loft, or `LoftGroup.SetGuides`. Before, a closed loft with guides was refused.

### Changed
- A loft closed into a ring keeps its profiles' size between them: its sections there are turned with the ring, as a closed sweep's are, so a ring of four circles is a torus within 0.2% of its volume where it was 5% short and thinner between its profiles. A ring of three profiles, a third of a turn apart, is made; before, profiles more than a quarter turn apart were refused. Rings have more patches than before.
- A modelled thread with a rounded crest can be chamfered, at either end or both, as a standard or truncated one; before, it was refused ("a rounded crest can't be chamfered"). Scripts lose `sketch.ErrChamferRounded` and gain `sketch.ErrChamferTooFine`, the refusal of a chamfer on a screw so coarse for its size that the cone would reach its axis.
- Thread tolerance classes follow the standards' tables: ISO 965-1's for metric threads and ISO 2903's for trapezoidal ones, looked up by diameter range rather than computed from the size, and ASME B1.1's length of engagement and rounding for unified ones. A 1/4-20 UNC 2A screw is now 0.0037 inch on its pitch diameter, as the table has it, not 0.0040, and an M10 6H nut 180 µm, not 175; a thread made with a class keeps its old clearance until it is edited.
- An ACME thread finer than 10 threads per inch has half the crest clearance, 0.005 inch a side, as ASME B1.5 gives it.

### Removed
- Scripting: `srf.SetChordTolMm`, `srf.SetMaxSegments` and `srf.SetMesher`, which set the tolerance of shells built with none and were meant for tests; a document's chord tolerance and segments are set with `Edit.SetChordTolerance` and `Edit.SetMaxSegments`.

### Fixed
- A chamfered thread whose chamfer's cone meets a ridge just past its crest, as on an M8 stud 30 mm long chamfered at both ends, is sound; before, it came out with open edges at every way of placing its ridges. Where an intersection leaves an edge of a face at a grazing angle, the kernel now follows it the other way when the first way finds nothing.
- File > Export > 2d Section through a plane that cuts the solid, rather than lying along one of its faces, writes where the plane cuts it, where before it wrote an empty file; a plane that misses the solid says so and writes nothing. The section is of the solid as it is at the active group, a sketch drawn after it included.
- A corner where three chamfered edges of one distance meet is cut by a plane through the points where the chamfers' lines cross, as other CAD programs cut it, where before it was a patch of a sphere through those points, bulging out of the plane; and at such a corner whose faces don't all meet square, one chamfer ends between where its lines cross the others' and the other two are cut square to their edges, the corner filled by a four-sided patch, as OpenCASCADE's chamfers are, where before the chamfers ended where a sphere would touch the faces.
- Chamfers of edges that meet at a sharp corner of a curved wall, as a rim's edges between the corners of a profile of curves, or round to their own start on a wall with one crease, or an edge with the one it meets at a corner whose third edge stays sharp, are mitred there as fillets are, where before they were refused as "not mitred". A chamfer that ends on a curved face beside its edge's end is cut at its distance from where the faces meet, where it was up to 0.05 mm off; and chamfers along curved faces at fine chord tolerances are no longer refused as "can't be fitted" where the surface was right.
- Fillets of some of a rim's edges that turn at sharp corners of a curved wall, two or three of four cubics' top edges picked together, are made, mitred where the picked edges meet and ending on the unpicked edges' walls, where before they were refused as "can't be fitted" at fine chord tolerances.
- A screw thread made on a face over its full length follows the face when the part changes afterwards: a rod made longer by a parameter, or a hole drilled deeper, carries a thread as long as it is at the next regeneration, where before the thread kept its old length until it was edited. A face come to reach the part's end, or to stop short of it, or too short for the thread, is reported on the thread's group, and the thread takes it up when edited.
- A menu opened from the keyboard (Alt and a letter, F10, Left and Right) on a very busy machine keeps its first entry active, so Return runs it, instead of sometimes closing without running anything.
- Text drawn in the vector font on two threads at once, as a drawing exported by a script while the window draws, no longer races on the font's glyphs.
- Scripting: `Solid.FaceAt` finds the face of a point on the seam between two quarters of a round face, as on a hole's wall at 0 or 90 degrees, instead of failing with "no face of the solid passes through".

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
