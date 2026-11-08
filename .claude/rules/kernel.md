---
paths:
  - "pkg/srf/**"
  - "pkg/topo/**"
  - "pkg/geom/**"
  - "pkg/libm/**"
  - "pkg/core/**"
  - "pkg/solver/**"
  - "pkg/thread/**"
  - "pkg/check/**"
---

# The kernel

- `srf.CxxSort` and `pkg/libm` stay as they are: the order of a mesh's triangles and the last
  bits of the kernel's arithmetic are what the goldens and the exports record (a stable sort in
  CxxSort's place moves nine of them; tried 2026-10-07).
- The tolerance a solid is followed at is `srf.Tolerance`, carried by every shell and surface
  and computed by the sketch per regeneration (`Sketch.Tol`); `srf.DefaultTolerance` is what a
  shell with none resolves to, and nothing sets it: a test builds its shells at the tolerance it
  needs.
- Fan-out goes through `pkg/internal/par`, which bounds it; `TELESAPR_PARALLEL=off` must give
  the same solid as the parallel run.
- A feature that cannot make its solid returns an error, which `pkg/sketch` records as a
  diagnostic on the group: never silently wrong geometry. Every solid a test builds goes through
  `pkg/check`; a feature's robustness sweep fixes its allowed failure rate before the work starts.
- Before taking up a kernel item of `ROADMAP.md`, read its dated "Done" and "Tried" notes in
  `git show af848717:ROADMAP.md`, so as not to try again what failed; a kernel item is sized
  only after a spike on its hardest fixture. OpenCASCADE (FreeCAD) is the oracle for fillet and
  chamfer volumes; where it refuses or deviates, the test says so and checks an integral.
- Comments say why in terms of this code, not by reference to SolveSpace's C++.
