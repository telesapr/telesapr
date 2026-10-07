# Packages stay under `pkg/`

**Status:** Accepted
**Date:** 2026-10-07

## Context

The Go conventions this project follows (the `go-ultimate` skill, which synthesises the Google
and Uber style guides) say library code lives at the module root and a `pkg/` directory is an
anti-pattern. TeleSAPR has 46 packages under `pkg/`, and the scripting interface is imported as
`github.com/telesapr/telesapr/pkg/script/api` by every user script, by the examples under
`examples/scripts`, by the user's guide, and by the generated symbol tables that the interpreter
loads. Moving the packages would break every script written so far for a benefit that is only
cosmetic from inside the repository.

## Decision

The packages stay under `pkg/`. Private helpers go under `pkg/internal/` (or a package's own
`internal/`). This is a recorded deviation from the convention, not an oversight; a new package
follows the same rule.

## Consequences

The import path of the scripting interface is stable. A reader used to the root layout finds
the code one directory deeper. Should the module ever be split, the scripting interface moves
last, with an alias period.
