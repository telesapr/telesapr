# Dual contouring: what it would and would not add, October 2026

Where the Ju, Losasso, Schaefer and Warren paper (*Dual Contouring of Hermite Data*, Rice
University, 2002) fits this codebase, and what of it is worth taking. Written as an analysis
note; nothing was built. It follows the two performance studies' notes in `doc/notes/`.

## Summary

- **Dual contouring is a mesher for sampled data, not a modelling kernel.** It converts a
  signed grid or octree whose edges carry exact intersection points and normals (Hermite
  data) into a polygonal mesh. TeleSAPR's solids are exact B-rep shells; a mesh of them is
  made by tessellating the true surfaces, not by reconstructing a surface from samples.
- **It would not make drawing or regeneration faster.** The regeneration profile is Booleans,
  closest-point searches and the rolling ball, not polygonisation
  (`doc/notes/perf-core-study/kernel.md`); contouring a voxel field adds a sampling pass over
  the whole bounding box on top of the exact pipeline rather than replacing any of it.
- **It would make STL less exact, not more.** The export mesh samples surfaces within the
  chord tolerance; a dual contour approximates them from grid samples, so radii, tangency and
  feature positions become grid-dependent.
- **The paper's sharp-edge trick solves a problem this kernel does not have.** Dual
  contouring recovers sharp edges from normals because an implicit field does not store them;
  here sharp edges are topology (`Solid.Edges()`), already exact.
- **The borrowable part is the paper's second half, not its contouring.** Its quadratic-error
  octree simplification with the topology-safety test is a mesh LOD technique that could apply
  to the triangles this kernel already makes, for smaller STL and a lighter viewport on large
  models. That is quadric simplification; only loosely dual contouring.

## What the paper gives

- One vertex per cell whose sign changes, at the minimiser of a quadratic error function
  (QEF) built from the Hermite data, and a quad per sign-changing grid edge — no feature
  detection, no case tables.
- An octree variant: QEFs at interior nodes, leaves collapsed when the residual is within a
  tolerance, polygons generated from minimal edges so no restricted octree and no crack
  patching.
- A numerically stable QEF representation via QR (float storage, double accuracy), merged
  with Givens rotations.
- A three-check topology-safety test (manifold, quasi-manifold for multi-material, and
  sign-comparison equivalence on the 3×3×3 sub-grid) so simplification cannot merge or split
  regions.
- Material indices at grid points in place of signs, giving multi-material boundaries.

## Where TeleSAPR stands

- Viewport and export share one path: `RunningShell.TriangulateInto` into `DisplayMesh`
  (`pkg/sketch/group_pipeline.go`, `pkg/srf/shell.go`), trimmed-surface grids plus the ear
  clipper or CDT (`pkg/srf/triangulate.go`, `triangulate_cdt.go`) at the chord tolerance.
- Mesh quality is already a dial: chord tolerance, angular tolerance and segment limit, the
  `MeshDraft`/`MeshStandard`/`MeshFine` presets, `StrictMesh` for watertightness
  (`pkg/export/options.go`, `doc/done-audit.md`).
- Unchanged groups keep their display mesh (`group_pipeline.go` copies the previous group's
  `DisplayMesh`), so panning and rotating do not retessellate.
- The data contouring would need is available: ray casts against exact surfaces
  (`pkg/srf/raycast.go`, `raycast_tree.go`) and point-in-solid probes (`pkg/check`). The
  question is not feasibility but what it would buy.

## Why it does not fit here

- **Regeneration is not polygon-bound.** The kernel profile's top is `MakeFromBoolean`,
  `ClosestPointToUV`, `allPointsIntersecting` and `rollingBallFillet`; `TriangulateInto`
  appears once, at about a quarter of one sweep benchmark. A mesher swap leaves the profile's
  top alone.
- **A voxel pass is extra work.** Classifying a bounding box's grid points and building the
  octree costs ray casts or probes per cell, before any contouring. The paper's speed comes
  from replacing a uniform grid with an octree of an *implicit* model, which already has a
  field; this kernel does not.
- **STL loses exactness.** Fillet faces are exact cylinders, spheres and tori; the export
  follows them within the chord tolerance. A dual contour at any practical grid spacing puts
  vertices near, not on, those surfaces, and the deviation is not the chord tolerance but the
  sampling error of the field.
- **The one speed story the paper has — real-time CSG on a signed field — is not this
  kernel's CSG.** Exact Booleans here are not accelerated by a meshing change, and the
  `ForceToMesh` fallback already has its own path (mesh BSP).

## What is worth borrowing: QEF simplification for export and viewport LOD

The paper's §3–§4, applied to the `SMesh` this kernel already produces: build a QEF per
octree cell from the triangles' planes, collapse cells whose residual is within a tolerance,
and gate every collapse with the topology-safety test so thin walls and holes survive.

- **What it would buy.** Smaller `.stl`/`.3mf` for printing large step-and-repeat models; a
  display mesh for large assemblies whose triangle count follows a global error bound rather
  than the sketch's chord tolerance alone.
- **Where it would live.** A new file under `pkg/srf` beside `mesh.go` (the QEF and its QR
  representation) and an option in `pkg/export`'s `Options` for export; the viewport would
  take the LOD as `DisplayMesh`'s alternative. No voxelisation anywhere.
- **What it costs.** Every mesh golden that a simplified export writes; per `AGENTS.md`,
  goldens change in a commit of their own, with the regen script. The simplifier must keep the
  mesh watertight under `StrictMesh` and keep the check oracle (`pkg/check`) sound.
- **What to measure before deciding.** STL bytes and triangle counts for the large fixtures
  at equal max deviation against the unsimplified export; `pkg/ui`'s frame benchmark's
  triangle metric on an assembly fixture.
- **Honest scope.** This is Lindstrom-style quadric simplification with the paper's topology
  test; calling it dual contouring would be a stretch. It is the only part of the paper whose
  problem this codebase has.

## Rejected

- **A voxel or implicit modelling mode** (sculpting, lattices, organic shapes contoured with
  dual contouring): a new product direction that contradicts the exact-kernel identity, and a
  second representation to keep sound.
- **Dual contouring as the `ForceToMesh` fallback** when an exact Boolean fails: voxelising
  the operands to contour the result costs more than the mesh-BSP fallback and gives a worse
  mesh.

## References

- Tao Ju, Frank Losasso, Scott Schaefer, Joe Warren. *Dual Contouring of Hermite Data*. Rice
  University, 2002. https://www.cs.rice.edu/~jwarren/papers/dualcontour.pdf
- `doc/notes/perf-core-study/kernel.md` — where regeneration time actually goes.
- `doc/notes/k4-cdt-spike.md` — the tessellation quality/cost trade the CDT spike measured.
