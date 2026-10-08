# What leaves `pkg/ui`, and what stays

**Status:** Accepted
**Date:** 2026-10-08

## Context

`pkg/ui` was one package of 33,000 lines: the window and menus, but also the file-format
writers of File > Export (vector, DXF, STEP and mesh formats), the DXF importer, the text
window's screens and the scripting console and host. The writers and the importer read a few
settings and style helpers from the application and were otherwise independent of it; the
screens and the scripting glue poke most of the application (83 and 57 distinct members).

## Decision

The format writers are `pkg/export`, behind an `Options` value (scale, tolerance, canvas,
colours, G-code, lights) and a `Constraints` interface for what a DXF file's dimensions ask of
the interface. The DXF importer is `pkg/persist/dxf`, with `Import` and `Has3D`; the dialogs
around it stay in `ui`. Both have their own tests at the 85% gate: the importer's golden test
runs without the application and matches the application's goldens byte for byte.

The text window's screens and the scripting console and host stay in `pkg/ui`. An interface
wide enough to move them would be the application itself, and they are the user interface;
splitting them would add a package boundary without taking away a dependency.

## Consequences

Exports can be written and tested without a window; the command-line tool and the scripting
host go through the same `Options`. The next cut of `pkg/ui`, if one is wanted, is the
`Application` struct itself (the configuration is already `Config`), not a package line.
