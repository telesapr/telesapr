# TeleSAPR is a separate project, not a SolveSpace port

**Status:** Accepted
**Date:** 2026-10-01

## Context

TeleSAPR began as a Go port of SolveSpace, held bit-identical with the C++: the same solver
results, the same `.slvs` bytes on save, the same libm, the same `std::sort` order. That gave a
testable port, but every feature beyond SolveSpace's had to be squeezed past the parity harness,
and the file format could not grow.

## Decision

From 2026-10-01 TeleSAPR tracks neither SolveSpace's behaviour nor its file format. The kernel
is developed as Go code in its own right; `.slvs` files are imported, never written
(see [0002](0002-tlspr-file-format.md)); the C++ comparison tests were frozen and then removed.
The provenance stays in `NOTICE`. Comments describe the Go code on its own terms rather than by
reference to the C++ (decision of 2026-10-07; the rewrite is work package 9 of the review plan).

## Consequences

Features no longer wait on parity, and goldens change when the kernel improves, in commits that
say why. The bit-exact scaffolding (`pkg/libm`, `CxxSort`, the FMA build rules) is kept only
where a golden depends on it, and may be replaced call site by call site when the goldens allow.
A bug can no longer be settled by diffing against the original; the SolveSpace tree is kept
outside the repository (`TELESAPR_SOLVESPACE_SRC`) for the two scripts that still build against
its vendored Eigen.
