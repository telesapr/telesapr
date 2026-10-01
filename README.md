# spacesolve (Go Port)

This is a Go port of spacesolve, a parametric 2D/3D CAD tool.

## Project Structure

```
pkg/
├── core/            # Handles, entities, constraints, expressions
├── geom/            # Geometric primitives and algorithms
├── libm/            # Bit-exact libm functions matching C++ output
├── persist/slvs/    # .slvs file load/save
├── sketch/          # Sketch, groups, requests, export
├── solver/          # Constraint solver (sparse QR, COLAMD)
├── srf/             # Surfaces, curves, booleans, meshing
└── ttf/             # TrueType/FreeType font loading and text outlines
```

`cmd/spacesolve` is the application, `cmd/spacesolve-cli` the command line
tool (`solvespace-cli`'s counterpart): `spacesolve-cli --help` lists its
commands. See `PORTING.md` for parity status.

## Build

```bash
go build ./...
```

## Original C++ Source

This port is based on the C++ version at: https://github.com/solvespace/solvespace
