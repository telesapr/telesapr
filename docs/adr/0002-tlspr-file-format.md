# A text file format of its own, `.tlspr`

**Status:** Accepted
**Date:** 2026-10-02

## Context

SolveSpace's `.slvs` format is a flat list of `Key=Value` lines with no room for named
parameters, expressions, document units, drawings, appearances or packed assemblies, and its
numbers are printed by C's `printf`. Keeping it would have meant a dialect SolveSpace could not
read anyway.

## Decision

TeleSAPR saves `.tlspr`, a line-oriented text format with a version number in its header,
written in a stable order so that a save is reproducible and a change is a small diff
(`docs/tlspr-format.md`). The reader and writer live in `pkg/persist/tlspr`, split into a syntax
layer that knows nothing of sketches and a model layer that maps the tree to `sketch.Sketch`.
A packed model, `.tlsprz`, is a zip of the model with what it links. `.slvs` is imported only,
through `pkg/persist/slvs` and `telesapr-cli convert`. Files with the old `spacesolve` header are
not read. A file of a newer major version is refused with its version named.

## Consequences

Every feature adds its fields behind the version number, with a fixture of the previous version
that must still load. Two saves of one model are the same bytes, which the path-equivalence
tests rely on. The format is TeleSAPR's to change, and nothing else reads it.
