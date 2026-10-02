# spacesolve

spacesolve is a parametric 2D/3D CAD tool written in Go. It started as a
port of SolveSpace and is now a separate project: it no longer tracks
SolveSpace's behaviour or file format.

## Project Structure

```
cmd/spacesolve/      # The GUI
cmd/spacesolve-cli/  # The command-line tool
pkg/
├── check/           # Invariants of a sound solid: the test oracle, Analyze > Check Solid
├── core/            # Handles, entities, constraints, expressions
├── geom/            # Geometric primitives and algorithms
├── libm/            # Bit-exact libm functions matching C++ output
├── persist/slvs/    # .slvs file load/save
├── platform/        # Windows, menus and dialogs (takigo, headless)
├── render/          # Canvas and the software renderer
├── sketch/          # Sketch, groups, requests, export, the screw thread
├── script/          # Go scripts on a sketch: interpreter, and the api they use
├── solver/          # Constraint solver (sparse QR, COLAMD)
├── srf/             # Surfaces, curves, booleans, meshing
├── thread/          # Screw thread standards and profiles
├── ttf/             # TrueType/FreeType font loading and text outlines
└── ui/              # Graphics and text windows, menus, screens
```

`cmd/spacesolve` is the application, `cmd/spacesolve-cli` the command line
tool (`solvespace-cli`'s counterpart): `spacesolve-cli --help` lists its
commands. See `ROADMAP-JUMP.md` for where the project is going.

## Scripting

A sketch can be read and changed from Go scripts, run by an embedded
interpreter ([yaegi](https://github.com/traefik/yaegi)):

- In the application, `Script > Show Console` opens a console where Go
  is typed a line at a time and run on the drawing: `doc` is the sketch,
  what a line draws shows at once, and each line is one step of undo.
  `Script > Run File...` and `spacesolve --script file.go` run a file.
- Without a window, `spacesolve-cli run [sketch.slvs] script.go [args]`
  runs a file, `spacesolve-cli repl` reads lines from standard input and
  `spacesolve-cli eval` takes them from the command line.

```go
r := doc.Rect(V(0, 0, 0), V(40, 25, 0)) // mm, in the active workplane
doc.Length(r[0], 40)
hole := doc.Circle(V(20, 12, 0), 4)
doc.Diameter(hole, 8)
plate := doc.Extrude(6, "plate")
fmt.Println(plate.Volume())
doc.Export("plate.stl")
```

What a script sees of the sketch is `pkg/script/api` (`go doc
./pkg/script/api`). In a console `help()` lists the most used of it,
`help(x)` says what a value is and what methods it has, and
`help("Group.Bounds")` or `help("api")` looks a name up.
A script file is a Go program that imports that package and starts
from `api.Current()`, so it can also be compiled as it is; see
`examples/scripts`. `pkg/script` is the interpreter around it, and
`pkg/script/symbols` the generated tables of what scripts can import
(`make generate` after changing what those packages export).

## Build

```bash
go build ./...
```

## Origin

The code base was ported from SolveSpace's C++: https://github.com/solvespace/solvespace
