# STEP exchange

Refreshed 2026-11-24 on branch `step-edge-merge`, from devel 3ecef233, after the writer joined an
edge's pieces (first written 2026-11-23, after the writer was moved to AP242). How TeleSAPR's STEP
reader (`pkg/persist/step`, `pkg/persist/link`) and writer (`pkg/export/step.go`,
`step_analytic.go`, `step_analytic_curve.go`, `step_bodies.go`, `step_edge_merge.go`,
`step_validation.go`) stand against the
current standard and its Recommended Practices, how an exchange with other programs is checked,
and what that check found.

## The standard

The current edition is AP242 edition 4, ISO 10303-242:2025 (August 2025). The ISO text is sold;
this note works from what is free:

- STEP Tools' notes on the editions (`steptools.com/docs/stp_aim/notes_ap242e4.html`): edition 4
  changes no attribute of an entity edition 3 had; it adds PMI tolerance modifiers, a tessellated
  B-rep, UUIDs and data-quality entities. The schema name is the same in every edition,
  `AP242_MANAGED_MODEL_BASED_3D_ENGINEERING_MIM_LF`, the edition told by its object identifier's
  fifth number. STEP Tools gives 1, 3, 4 and 5 for the four editions; NIST's SFA takes 1 for the
  first, 2 or 3 the second, 4 the third and 5 to 7 the fourth, and asks for
  `{ 1 0 10303 442 7 1 4 }` (5 and 6 were drafts): we write 7. OpenCASCADE 7.9 still writes 1.
- The MBx-IF (formerly CAx-IF) Recommended Practices (`mbx-if.org/home/?p=194`), of which these
  bear on a mechanical B-rep: Geometric and Assembly Validation Properties v4.6 (2023), Model
  Styling and Organization v1.10 (2025), the General Guidelines v2.1 (2024), and the PDM Usage
  Guide 4.3 for the product structure.
- NIST's STEP File Analyzer and Viewer (SFA, `github.com/usnistgov/SFA`, 5.54 of 2026-11-02),
  which the MBx-IF runs on every test file: syntax against the schema, and the Recommended
  Practices where a file carries what they govern.

### Where we stand

| Rule | Reader | Writer |
|---|---|---|
| Schema | AP203, AP203 e2, AP214, AP242, AP209 named; none refused | AP242 edition 4, `{ 1 0 10303 442 7 1 4 }`; was `CONFIG_CONTROL_DESIGN`, AP203 edition 1 |
| Header | not read | the file's name, the time (`Options.StepTime`; the zero time writes 1970-01-01, for the goldens), 'TeleSAPR'; was `'output_file'` of 2009 |
| Product | assemblies by NAUO and CDSR, mapped items, mirrored placements | named after the file, in AP242's application context with its protocol definition, every product in the category 'part' (PDM Usage Guide); an assembly's parts named after their groups |
| Units | SI with prefixes, conversion-based (inch, degree) | mm, radian, steradian, uncertainty 2 × LengthEps |
| Analytic surfaces | plane, cylinder, cone, sphere, torus, extrusion, revolution, offset | plane, cylinder, cone, sphere and torus where every point of a 5 × 5 grid on the patch (on every patch of a face written as one) lies on it within 1e-7 of its size; lines and circles for an edge's straight and circular curves; the rest rational B-splines. A wireframe's free curves stay B-splines, a LINE or CIRCLE being unbounded |
| Edges | the EDGE_CURVEs of each EDGE_LOOP, their 3D curves | an EDGE_CURVE per run of a face's trim pieces between real vertices, shared by both faces (`step_edge_merge.go`); see "Edges across patches" below |
| Bodies | each MANIFOLD_SOLID_BREP, BREP_WITH_VOIDS, shell model, tessellated shape | a MANIFOLD_SOLID_BREP of its own CLOSED_SHELL per body (`srf.SShell.Pieces`); a shell with a cavity, a piece whose mesh encloses a negative volume, still one CLOSED_SHELL: no BREP_WITH_VOIDS yet |
| PCURVE | not read; trims projected from the 3D curves | not written |
| Colours (Styling 1.10) | STYLED_ITEM and OVER_RIDING_STYLED_ITEM, RGB and named colours | a STYLED_ITEM per face, its group's colour |
| Validation properties (GVP 4.6) | not read | volume, surface area and centre per part, at the part's level, combined in one REPRESENTATION (4.12), measured on the shell meshed at a twentieth of its tolerance |
| PMI, layers, tessellated B-rep, UUIDs | not read | not written |

## The exchange check

`scripts/step-exchange.sh [out-dir]` exports a set of models from the tree (fillets, chamfers,
lofts, sweeps, shells, a mirror and a pattern of separate bodies, wrapped text, threads, two
assemblies, one of them coloured; `scripts/step-exchange/colours.go`) with `telesapr-cli run`
and `scripts/step-exchange/export.go`, which prints our volume, centre and box. Then:

- OpenCASCADE (`DRAWEXE`, `scripts/step-exchange/occ-read.tcl`) reads each: volume and centre by
  `vprops` at 1e-9, solid by solid; `checkshape`; solids and faces; names, colours and
  validation properties by its document reader (`XStat`).
- FreeCAD (`FreeCADCmd`, `scripts/step-exchange/freecad-read.py`) reads each: valid, shells
  closed, solids, faces, the tight box (`optimalBoundingBox`) and each face's surface kind.
- OpenCASCADE writes each again as AP242 (`occ-rewrite.tcl`) and TeleSAPR links that as a solid
  (`link.go`): the round trip.
- SFA's syntax check, where `SFA_CL` names `sfa-cl.exe`, under wine. `sfa-cl.exe` runs with
  `WINEPREFIX` of its own; its first run installs the IFCsvr toolkit and stops, the second runs.
- `scripts/step-exchange/compare.py` prints a line per model against the GVP practice's
  production thresholds (4.13.3): volume within 0.5%, centre and box corners within 0.02 mm plus
  0.1% of the box's diagonal.

The other way, `scripts/step-exchange/occ-fixtures.tcl` has OpenCASCADE build a box, a cylinder,
a cone, a ball, a ring, a filleted box, a holed block and the holed block with a ball fused at a
corner of its top, and write them to `testdata/step/exchange`, each as AP242 and the last two as
AP203 and AP214 too, with its own measures of the shapes it built (`reference.txt`).
`TestExchangeFixtures` reads every file as a mesh and as a solid against them; it needs neither
program.

## Results, 2026-11-23

Before the writer changed: 21 of 21 models within the thresholds by OpenCASCADE and FreeCAD,
every file valid and closed in both, SFA clean on all, the round trip within 0.006%. Every face
came over as a B-spline (FreeCAD: `BSplineSurface` for all 383 faces); the colours of the
coloured assembly came over, two.

After: 21 of 21 still, SFA clean on all, the round trip within 0.007%; of the 383 faces 104 are
B-splines (the lofts' 72, the flanks of threads, the fillets' blends, wrapped letters, the
cubic sweeps), the rest planes, cylinders, cones, spheres and tori. FreeCAD finds the knob's side
a cylinder of radius 15 with 17 circles and 2 lines for edges. The volume each file declares is
within 0.03% of OpenCASCADE's of the solid it read (the bolt's thread and the twisted sweep the
most, their trims' chords). OpenCASCADE reads the product names and the declared properties
(`XStat`); SFA lists them as properties. The holed block exported, read back by OpenCASCADE, is
π·25·10 mm³ to 1e-13, where the B-spline file gave 785.39814.

The others to TeleSAPR: every OpenCASCADE file read right as a solid. As a mesh, the way a link
is read unless it is set to read as a solid, the ball fused at the block's corner came in with
254 naked and 409 self-intersecting edges, 9% short, and no problem said: its face goes round the
ball's lower pole, which lies on the block's upright edge, between two of its rims, and then up
the ball's seam, which the tessellator put at the pole's borrowed u, a whole turn off
(`faceSurface.loop`; fixed, `box_ball_pole_on_edge` in `testdata/step/shapes`).

## Edges across patches, 2026-11-24

A face's trims come as pieces, a Bezier each, cut wherever the trim crosses a patch of a sheet or
was cut by an intersection, and the writer made each piece an EDGE_CURVE with its own B-spline
and vertices. A pen's barrel, a chamfer of many small patches along wrapped mouldings, had 213
faces and 6015 edges on 634 pairs of faces (149 on one pair); its edge curves and their points
were 3 MB of the 4.9 MB file.

`exportShellFaces` now works out every face of a shell first (`shellJobs`: `surfaceJob` for a
patch, `sheetJobs` for a sheet written as one surface, each a closure that writes the surface
and the faces' loops) and writes them in the same order after. Between, `stepJoints` counts the
loop corners at each point, to `stepPrecision`. A joint goes when exactly two corners lie there
(an edge's two faces, or a seam's face twice), the curves either side of the two are the same
(their far ends and middles, either way round: a sheet's loops are turned round for the file,
so its edges may run the same way on both faces), and none of them is a LINE or CIRCLE
(`lineOrCircleOf`, which `writeLineOrCircle` writes). `edgeRuns` cuts each loop into runs
between the vertices kept; a run that ends where it starts (a ring between two faces, or a
ring that touches the rest of its face's loop at one vertex) is cut at the joint farthest from
that point, ties to the first in coordinate order, which both faces choose alike: a closed
curve's points are not found again as one curve's by `HasBSplineCurveAnAlias`, and the other
face wrote its own edge. A run of two or more pieces is one `B_SPLINE_CURVE_WITH_KNOTS`
(`joinedCurve`): each piece raised to the run's highest degree, its weights put in the form whose
end weights are 1 (`standardWeights`, a change of parameter, not of curve) so that they agree at
the joints, knots of multiplicity the degree at each joint spaced by the control polygons'
lengths. Each span is its piece exactly, whatever the knots. The other face's run is the same
pieces the other way, the same points, and the curve and edge aliases find it.

Written as patches (`StepOneSurfacePerFace` off) a joint is mostly where a third patch meets, and
little is joined. A loop with a T-junction, one face's piece running past the other's joint,
keeps its vertex: its sides don't match. Without that check the wrapped text fixture was read
back open by our reader.

| Model | faces | EDGE_CURVE | VERTEX_POINT | bytes |
|---|---|---|---|---|
| barrel (pen) | 213 | 6015 → 679 | 5772 → 436 | 4898640 → 2734135 |
| bolt_m8x30 | 82 | 488 → 240 | 410 → 162 | 400191 → 303057 |
| nut_m8 | 24 | 158 → 72 | 136 → 50 | 120901 → 87406 |
| lead_tr20 | 36 | 217 → 105 | 184 → 72 | 168655 → 125008 |
| sweep-twisted-arcs | 18 | 92 → 36 | 76 → 20 | 73755 → 51790 |
| wrap-cylinder-text-sunk | 29 | 118 → 74 | 94 → 50 | 108493 → 91297 |

The barrel read by OpenCASCADE: one valid solid of 213 faces, 679 edges and 436 vertices,
1770.99293 mm³ by `vprops` at 1e-9 where the file before gave 1770.99387; FreeCAD: valid,
closed, 213 faces of the same kinds; TeleSAPR linking it as a solid, 1771.541654 mm³ from either
file. `scripts/step-exchange.sh`: 21 of 21 within the
thresholds, as before; OpenCASCADE's volumes moved by under 5e-5 (the bolt's most,
2072.369 → 2072.454, its thread flanks' pcurves projected from the longer curves), ours and the
round trip's not at all. No golden under `testdata/export` moved: their solids are planes and
circles.

## Traps

- OpenCASCADE's and FreeCAD's `Shape.Volume`, and `vprops` without a tolerance, were 0.13% short
  on our knob, a full circle written as one rational B-spline, and put its centre 0.13 mm off its
  axis: `vprops shape 1e-9 -full` gives the volume to 1e-15. Without `-full` it prints six
  digits.
- `vprops` of a compound of two mirrored blocks put their centre 0.625 off; each solid's own was
  right. Measure solid by solid.
- OpenCASCADE reading its own file of the ball fused at the corner found 27455 mm³, where it had
  built 31644: it is no oracle for that face, and `reference.txt` holds the shapes as built.
- FreeCAD's `BoundBox` is the control points' box, far wider on a B-spline (the vase ±94 where it
  is ±34); `optimalBoundingBox` is the shape's.
- In `DRAWEXE`, `param write.step.schema` takes 3 (AP203), 4 (AP214) or 5 (AP242), and a
  `newmodel` before `stepwrite` for it to hold; the names (`AP242DIS`) are taken and ignored.
- Blender 5.2 has no STEP reader.
- A face of several patches written as one surface (`sheetJobs`) may mix surfaces: a swept
  pipe's one smooth face is a cylinder along the path's line and a torus round its arc. Taken from
  its first patch alone, the face came in invalid and 78% short; every patch is checked.
- SFA calls `(AREA_UNIT()DERIVED_UNIT(...))` wrongly mapped: AREA_UNIT and VOLUME_UNIT are
  subtypes of DERIVED_UNIT alone, written `AREA_UNIT((#e))`.
- The validation properties at the shell's own tolerance were 0.14% short on the knob: the
  surfaces are meshed at a twentieth of it (0.014%). The trims keep the points the shell was built
  with, so a thread's stays 0.17% short; a finer tolerance on the shell alone changed nothing.
- `TestFilletPivotSeamStep` (`pkg/topo`, not in `make test-fast`) reads our STEP back as a
  solid: a dish's lathed cone written as one CONICAL_SURFACE face, a pocket at its seam reaching
  the apex, failed in our reader ("the bounds in a cell cross") at every reference direction
  tried (0° to 270°), where OpenCASCADE read it; a face of several patches reaching its apex or
  pole is now written as a B-spline (`reachesPole`), and the reader's gap is a ROADMAP row. The
  sheet's first control point may be that apex, so the reference direction is taken from a
  corner of the first patch off the axis (`seamOf`).
- SFA writes its options file on a `csv` run and then fails a `syntax` run with "can't read
  gen(Excel)", after its report; the report above it stands.
