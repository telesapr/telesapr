# STEP exchange

Refreshed 2026-11-27 on branch `step-apex-faces`, from devel 97f62b67, after a face of several
patches came to run its bounds the STEP way and the reader to read faces through a pole (first
written 2026-11-23, after the writer was moved to AP242; refreshed on `step-edge-merge` when the
writer joined an edge's pieces, on `step-link-checks` when the reader came to read validation
properties and check each solid's mesh, and on `step-voids` when the writer came to write a
cavity as a void). How TeleSAPR's STEP
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
| Bodies | each MANIFOLD_SOLID_BREP, BREP_WITH_VOIDS, shell model, tessellated shape | a MANIFOLD_SOLID_BREP of its own CLOSED_SHELL per body (`srf.SShell.Pieces`); a body with cavities a BREP_WITH_VOIDS, each cavity an ORIENTED_CLOSED_SHELL `.F.` of its faces turned out (`step_bodies.go`); see "Voids" below |
| PCURVE | not read; trims projected from the 3D curves | not written |
| Colours (Styling 1.10) | STYLED_ITEM and OVER_RIDING_STYLED_ITEM, RGB and named colours | a STYLED_ITEM per face, its group's colour |
| Validation properties (GVP 4.6) | a product's volume, area and centre, a property each or combined, compared with what was read (see "Reading validation properties" below) | volume, surface area and centre per part, at the part's level, combined in one REPRESENTATION (4.12), measured on the shell meshed at a twentieth of its tolerance |
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
(their far ends and middles, either way round: a sheet's loops ran turned round before
2026-11-27, its edges the same way on both faces, below), and none of them is a LINE or CIRCLE
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

## Reading validation properties, 2026-11-24

`pkg/persist/step/validation.go` reads every PROPERTY_DEFINITION named 'geometric validation
property' of a PRODUCT_DEFINITION_SHAPE (or of a PRODUCT_DEFINITION) through its
PROPERTY_DEFINITION_REPRESENTATION: a MEASURE_REPRESENTATION_ITEM's VOLUME_MEASURE or
AREA_MEASURE, simple or complex, in its DERIVED_UNIT of a length unit (else the representation's
length unit to the power), and a CARTESIAN_POINT, the centre, in the representation's context.
Ours put the three in one representation, other programs one each; both read. A product's shapes
are those in the spaces its SHAPE_DEFINITION_REPRESENTATIONs reach (the assembly's union-find,
`assembly.repsOfPD`), measured in their own space as a part's properties are; a product holding no
shape that is no other's child, an assembly's root, is measured as every placement. A product used
in an assembly with no shapes of its own is left out.

The tolerances: 1% of the volume and of the area, the Recommended Practices' "good", and 1% of the
part's box diagonal for the centre. The link's mesh is measured first; a product that differs is
tessellated again at a twentieth of the chord tolerance (`validationFiner`) before it is said,
since a thin rod's chords take more than 1% of its volume at the link's tolerance. A product that
agrees counts in `Report.Validated` and the link's screen says "volume, area and centre as the
file declares for n parts"; one that differs is a sentence in `Report.Mismatches`, which
`Problem` joins. The cost is a pass over the triangles; the finer tessellation only on a
difference.

Checked by `TestValidationProperties`: OpenCASCADE's eight exchange fixtures with their
`reference.txt` values added as other programs write them, in millimetres and in metres, agree;
5% more volume and the centre 5 mm away are said, and the area is not blamed. Our own exports
(`testdata/export/*/*/*/surfaces.step`) agree.

## Checking a mesh link, 2026-11-24

The box fused with a ball whose pole lay on its edge came in as a mesh with 254 naked edges and
409 crossing ones and nothing said (fixed 2026-11-23). Each solid's mesh
(MANIFOLD_SOLID_BREP, BREP_WITH_VOIDS, FACETED_BREP, TESSELLATED_SOLID; not shells, which may be
open) is now searched as Check Solid searches a group's (`meshcheck.go`): in its own space, so an
assembly's parts touching don't count, after its T-junctions are snapped, and not when faces were
left out, which the report already says. The problem reads "the mesh of 1 solid has 3 naked edges,
so it is not closed". A link read as a solid drops it: the solid is checked as any group's.

Triangles lying on one another count as touching, not crossing (`coplanarIsInter` false): with
them counted, NIST_MBE_PMI_6's pole slivers gave two crossing edges and the knife-edge Boolean's
export (`testdata/export/group/boolean_knife_edge`) one, both where faces only touch. With it
false the corpus, the exchange fixtures and our exports say nothing.

Cost, on the corpus read without the cache: 0.50 s of 2.47 s (NIST_MBE_PMI_3: 135 ms of 278 ms,
2136 triangles; the edge search is parallel, `badEdgesInto`), paid once, since the tessellation
and its report are kept (`stepDiskVersion` 3; 4 since voids are turned over whole, below). Tested by `TestMeshCheck` (a tessellated cube
whole, open and pierced; an open shell says nothing) and `TestLinkStepOpenMesh`.

## Voids, 2026-11-24

A shell's pieces (`srf.SShell.Pieces`) are each meshed: one enclosing a positive volume is a body,
one enclosing a negative volume a cavity, its faces facing into the hollow. Each cavity goes to the
smallest body whose mesh winds round a point of the cavity's surface (`stepBodies`,
`SMesh.WindingNumber`), so a body standing in another's cavity is a solid of its own and its own
cavities its own. A cavity in no body, or a shell with no body, is written as before, one
CLOSED_SHELL of every face.

ISO 10303-42 has a void as an `oriented_closed_shell` used against the orientation of its closed
shell: the closed shell is the cavity as a solid of its own, faces out of the hollow, and `.F.`
turns them back. The writer writes the cavity's faces turned over (`stepFileWriter.turned`): the
ADVANCED_FACE's same sense and each FACE_BOUND's orientation both flipped, which is the
standard's `face_reverse`. OpenCASCADE reads each void's closed shell alone as the solid of the
cavity (2000 mm³ and 2144.66 for the hollow model's box and ball), and its own BREP_WITH_VOIDS
the same way (`hollow_AP242.step` in `testdata/step/exchange`).

Our reader turned a shell used against its orientation by flipping its faces' sense only, not
their bounds. A face whose bounds tell its orientation by their turn in the surface's parameters
was then read the wrong way: the hollow model's ball cavity, eight sphere patches, came in as a
mesh 3283 mm³ short with 80 naked edges, and as a solid "edge 26 is used by 6 co-edges". Planes
and OpenCASCADE's one-face sphere, bounded through its poles, read right either way. The reader
now turns such a face over whole (`reversedFace`); `stepDiskVersion` went to 4, as the CLI's
cache in the user's folder otherwise hands back the tessellation made before.

The hollow model (`scripts/step-exchange/hollow.go`, 60 × 30 × 30 with a box of 20 × 10 × 10
and a ball of radius 8 inside), by `scripts/step-exchange.sh`: before, one CLOSED_SHELL of 20
faces, OpenCASCADE 49855.339415 mm³ and FreeCAD valid, closed, one solid; after, a
BREP_WITH_VOIDS of the outer shell and two voids, OpenCASCADE 49855.339415 (the exact
60·30·30 − 2000 − 4/3·π·8³) and its centre the same, FreeCAD valid, closed, one solid, SFA clean,
our link of it 49857.66 as a mesh and 49857.42 as a solid (ours before 49857.63, the chords'
sag), its declared volume, area and centre matching what was read. The other 21 models as
before, 22 of 22 within the thresholds. SFA hung under wine at the sixth file from the end
(`shell-revolve`) on this run and was stopped; the 17 it checked, the hollow model among them,
are clean. `TestStepVoids` (`pkg/export`): a block with a box and a ball cavity, and a cube in the
box's cavity, is one BREP_WITH_VOIDS with two voids and one MANIFOLD_SOLID_BREP, read back as a
mesh and as solids to the model's volume, its properties validated.

## Faces through a pole, 2026-11-27

A dish's lathed cone written as one CONICAL_SURFACE face, a pocket at its seam reaching the apex,
read back in our reader with "the bounds in a cell cross", at every reference direction tried, and
the writer kept such faces (bounds reaching an apex or pole) B-splines (`reachesPole`). The cause
was the writer's: `sheetJobs` turned a sheet's loops round for the file, and `ExportCurveLoop`'s
FACE_BOUND `.F.` turned them again, so every face of several patches written as one surface ran
its bounds clockwise about its normal, each edge it shares run the same way by both faces (the
pivot-seam file: 11 of 31 edges; the knob: 8 of 19; 11 of the exchange's 22 models had such
edges, none since). Our reader set such faces right by the turn
of their bounds in the surface's parameters (`turnedLoops`), but took every bound through a pole
as the file gave it: the dish's face, its outer bound jumping at the apex by exactly half a turn
and its pocket's bound touching the apex up and down the seam, came in turned. OpenCASCADE and
FreeCAD fix wire orientation on reading and said nothing. A sheet's loops now run as a patch's
(`sheetJobs`), every edge is used once each way, `reachesPole` is gone, and the original reader
reads the corrected dish file. `scripts/step-exchange.sh` before and after: 22 of 22 models
within the thresholds, OpenCASCADE and FreeCAD valid, volumes the same.

The reader was made to read such files too, and faces of a sphere through or round a pole:

- A bound that jumps along a pole once, on a surface of one pole (a cone), is laid out from just
  past the jump and closed along the pole (`boundArea`, `fromJump`): the way round the rest of
  it takes fixes the jump's. So the old writer's dish file reads too.
- On a sphere (`onePole` false, closed one way) bounds are taken as the file gives them: a face
  may hold a pole all round, its bounds then enclosing its outside, and the turn of its bounds
  can't tell that from a face turned round. Taken by the turn, a ball with half of its cap cut off
  by a plane through its axis, one SPHERICAL_SURFACE face, read back 448 of 3736 mm³.
- A sphere's face whose bounds enclose less than nothing holds the poles they don't reach
  (`holdsPoles`), and its grid runs from pole to pole; it ran over the bounds' extent alone, and
  the half-capped ball came in open along the cut's parallel.

`TestStepSheetReachingPole` (`pkg/export`): the ball less half its cap and a cone less half its
tip, each cut by a block whose side runs down the axis, and a ball with a square hole through
it, written one surface per face: analytic faces, no B-spline, every edge once each way, read back
as solids within 0.1% of the same written a patch at a time. The kernel's own Booleans of such
blocks with a raw lathe's ball or cone are unsound at several turns of the block (300°, 120° for
the cone), and at 10° and 75° the ball's sheet is written a patch at a time (ROADMAP).

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
  solid, a dish's lathed cone with a pocket at its seam reaching the apex among them (see "Faces
  through a pole" above). The sheet's first control point may be that apex, so the reference
  direction is taken from a corner of the first patch off the axis (`seamOf`).
- A face's bounds that run the wrong way round pass OpenCASCADE's `checkshape` and FreeCAD's
  check as valid, with the right volume: their readers set the wires right. Count each edge's
  uses forwards and back in the file (`stepEdgeUses`) to see it.
- SFA writes its options file on a `csv` run and then fails a `syntax` run with "can't read
  gen(Excel)", after its report; the report above it stands.
