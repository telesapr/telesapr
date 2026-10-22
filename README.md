# TeleSAPR

TeleSAPR is a parametric 2D/3D CAD program written in Go. It began as a port of
[SolveSpace](https://solvespace.com) and is now a separate project: it no longer tracks
SolveSpace's behaviour or file format. It saves its own text format, `.tlspr`, and imports
SolveSpace's `.slvs`.

> **Warning:** TeleSAPR is experimental. It may have bugs, and it may change in ways that break
> your files, scripts and workflow. It is not recommended for sensitive or critical work.

It is built and tested on Linux. See [`ROADMAP.md`](ROADMAP.md) for where it is going.

## What it adds to SolveSpace

- **Solid features**: fillets and chamfers (rounded and filled edges, corners, edges of their own
  radius, edges between curved faces by a rolling ball), shell, draft, ruled and smooth lofts
  (guided, closed into a ring, along a solid's sides), sweeps along open and closed paths,
  patterns along a path, mirror, a sketch wrapped or embossed onto a cylinder or cone, and screw
  threads edited as one, with run-outs, drawn by the drafting convention on drawings.
- **Edges by name**: any edge of a solid can be picked on the model, including edges no sketch
  entity draws, and is saved by a name that holds when the model upstream changes.
- **Parameters**: named values and expressions that drive dimensions and feature values
  (extrusion depth, revolve angle, radii), and dimensions read by name; the document's own unit
  of length.
- **Assemblies**: linked parts that take a configuration and parameter values, relinked to
  another file; named datums on entities or any face or edge of a part, mated by name; a screw
  mate; density and mass properties per part; clearance between parts and wall thickness.
- **History**: groups moved in the history, suppressed with what uses them, and what a deletion
  took listed.
- **Sketch feedback**: entities coloured by how free they are, conflicting constraints marked,
  measuring without a constraint, and a search over every command.
- **Section view**, an OpenGL renderer, and **Check Solid**, which says whether a solid is sound.
- **STEP files** linked as reference meshes, and STEP export with one surface per face; 3MF
  and glTF (`.glb`, for Blender and web viewers) export of the mesh with its colours. STL export has quality presets (draft, standard, fine)
  and an angular tolerance, also as `telesapr-cli export-mesh --quality`.
- **Scripting in Go**, with forms that ask for a script's values and a library folder of your
  own packages; a model written back as the script that builds it; and **drawing with an AI
  model** through MCP.

The [user's guide](doc/guide/README.md) has a page for each, built on an example script.

## Download

Each [release](https://github.com/telesapr/telesapr/releases) carries `telesapr` (the window) and
`telesapr-cli` (the command line) for:

- **Linux x86_64**: a `.deb` (`sudo apt install ./telesapr_*.deb`), an `.rpm`
  (`sudo dnf install ./telesapr-*.rpm`), or a `.tar.gz` to unpack anywhere, with the desktop
  entry, icons and MIME type under its `share/`. The window needs an X server with OpenGL;
  without OpenGL it draws in software.
- **Windows x64 and arm64**: a `-setup.exe` installer (Start Menu entry, `.tlspr` association,
  and PATH for `telesapr-cli` if asked), or a `.zip` to run from any folder. The window draws in
  software.
- **macOS 11 or later, Apple silicon and Intel**: a `.dmg` with `TeleSAPR.app`; drag it to
  Applications. The app is not notarized, so the first time Control-click it and choose Open.
  `telesapr-cli` is inside, at `TeleSAPR.app/Contents/MacOS/telesapr-cli`.

`SHA256SUMS` lists the checksums. The Windows and macOS packages are built by the release
workflow and not tested by it; TeleSAPR is tested on Linux. `doc/releasing.md` says how a
release is made.

## Build

TeleSAPR needs Go 1.27, X11 and OpenGL development files, and its windowing library
[takigo](https://github.com/takigo/takigo), which `go` fetches like any other module.

```bash
# Debian and Ubuntu; other distributions have the same libraries.
sudo apt-get install libx11-dev libxft-dev libfontconfig1-dev libgl-dev

git clone https://github.com/telesapr/telesapr
cd telesapr
make build                                      # bin/telesapr and bin/telesapr-cli
sudo make install                               # with the desktop entry, icons and MIME type
```

`make help` lists the other targets. `make test-fast` runs the quick tests, `make test` all of
them. `make ci` runs what the GitHub workflow does on a push to master: formatting, the
generated-symbol check, the build, every test (the GUI ones under Xvfb), the race
detector on the engine packages, the linter and govulncheck. On a branch the workflow runs the
quick tests in place of all of them and leaves out the race detector; both run on master and on
request. `make coverage` (every test with the 85% rule) is slow and runs in the nightly workflow,
along with the full robustness sweeps, the fuzz targets, the benchmarks and the quick tests on
Windows and macOS.

## Using it

`telesapr [file.tlspr]` opens the application. Solids are made off the window's thread: while a
change is being made into the solid, the window draws the solid as it was, and Esc stops it
(`TELESAPR_BACKGROUND=off` makes them on the thread, as before). `telesapr-cli` works without a window:
`telesapr-cli --help` lists its commands, which export views, drawings, meshes, surfaces and the
model as a script, convert `.slvs` files (`telesapr-cli convert`), and run scripts.

### Scripting

A sketch can be read and changed from Go scripts, run by an embedded interpreter
([yaegi](https://github.com/traefik/yaegi)):

- In the application, Script > Show Console opens a console where Go is typed a line at a time
  and run on the drawing: `doc` is the sketch, what a line draws shows at once, and each line
  is one step of undo. Script > Run File... and `telesapr --script file.go` run a file.
- Without a window, `telesapr-cli run [model.tlspr] script.go [args]` runs a file,
  `telesapr-cli repl` reads lines from standard input and `telesapr-cli eval` takes them from
  the command line.
- A script that declares an `api.Form` asks for its values in a dialog when the window runs
  it, and takes them as `--set name=value` on the command line.
- Packages of your own go in the script library folder (the configuration screen's, or
  `--lib` on the command line; by default `telesapr/library` in the user's configuration
  folder): `import "gears"` reads `<library>/gears`. See [Scripts](doc/guide/scripts.md).

```go
r := doc.Sketch.Rect(V(0, 0, 0), V(40, 25, 0)) // mm, in the active workplane
doc.Constrain.Length(r[0], 40)
hole := doc.Sketch.Circle(V(20, 12, 0), 4)
doc.Constrain.Diameter(hole, 8)
plate := doc.Features.Extrude(6, "plate")
fmt.Println(plate.Analyze().Volume())
doc.File.Export("plate.stl")
```

What a script sees of the sketch is `pkg/script/api` (`go doc ./pkg/script/api`). Its calls sit
where the window's commands do: `doc.Sketch` holds what the Sketch menu does, `doc.Features` the
Features menu, and so on, and what only one kind of group has is on a view of it
(`g.AsFillet().SetRadius(3)`). In a console `help()` lists the most used of it, `help(x)` says
what a value is and what methods it has, `help("Features.Extrude")` or `help("api")` looks a name
up, and `help("Features > Fillet Edges")` says what a script calls in place of a menu entry. A script file is a Go program that
imports that package and starts from `api.Current()`, so it can also be compiled as it is; see
[`examples/scripts`](examples/scripts), each of which a test runs.

### Drawing with an AI model

An AI model can draw in the window you have open from what you say in plain English. TeleSAPR
offers the drawing to any Model Context Protocol client, which brings the model; it has none of
its own and needs no API key.

- In the application, turn on Script > Allow AI Connections.
- Tell the client to start `telesapr-cli mcp`, for Claude Code with
  `claude mcp add telesapr -- telesapr-cli mcp`.

The model writes scripts against `pkg/script/api`, reads the drawing in words and looks at
pictures of it. Each script shows in the script console and is one step of undo.
`telesapr-cli mcp --headless` does the same without a window. See [`doc/ai.md`](doc/ai.md).

## Project structure

```
cmd/telesapr/          The application
cmd/telesapr-cli/      The command-line tool
dist/freedesktop/      Desktop entry, icons and MIME type
doc/                   The user's guide, the .tlspr format, the AI guide, the audit,
                       decision records (adr/) and working notes (notes/)
examples/scripts/      Example scripts, one or more per feature
pkg/
├── check/             Invariants of a sound solid: the test oracle, Analyze > Check Solid
├── core/              Handles, entities, constraints, expressions
├── dxf/               DXF writing
├── export/            File > Export: vector, mesh and STEP files
├── geom/              Geometric primitives and algorithms
├── internal/          The constrained Delaunay triangulator, C's printf, UTF-8 as the
│                      fonts read it, goldens, parallel loops and robustness sweeps
├── libm/              Bit-exact libm functions, as glibc computes them
├── mcpserver/         The drawing as tools for an MCP client (telesapr-cli mcp)
├── persist/
│   ├── tlspr/         TeleSAPR's file format, .tlspr
│   ├── slvs/          SolveSpace's .slvs, read for import
│   ├── convert/       .slvs files and what they link imported as .tlspr
│   ├── dxf/           DXF drawings imported into a sketch
│   ├── link/          Loading what link groups link: models, STL and STEP files
│   └── step/          STEP (ISO 10303-21) reading and tessellation
├── platform/          Windows, menus and dialogs (takigo, headless)
├── render/            Canvas, the software renderer (soft) and the OpenGL one (gl)
├── res/               Bundled icons and fonts
├── script/            Go scripts on a sketch: the interpreter, the api they use, and
│                      agent, for programs that draw by writing scripts
├── sketch/            Sketch, groups and their regeneration, the solid features
├── solver/            Constraint solver (sparse QR, COLAMD)
├── srf/               Surfaces, curves, Booleans, meshing
├── thread/            Screw thread standards and profiles
├── topo/              Topology of a solid: faces, edges, names, local edits
├── ttf/               TrueType and other font loading, text outlines
├── ui/                Graphics and text windows, menus, screens
└── version/           TeleSAPR's version
testdata/              Fixtures and goldens
```

## Licence

TeleSAPR is free software under the GNU General Public License, version 3 or later, as
SolveSpace is; see [`LICENSE`](LICENSE). [`NOTICE`](NOTICE) credits the code it ports from other
projects.

## Origin

The code base was ported from SolveSpace's C++, by Jonathan Westhues and the other SolveSpace
authors: https://github.com/solvespace/solvespace
