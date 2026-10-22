# TeleSAPR: how the code is written

Read this before changing anything. `README.md` says what the program is; `ROADMAP.md` says
what is being built; `doc/adr/` says why the big decisions were taken. This file is the
conventions, for people and for coding agents alike (`CLAUDE.md` is the same file).

## Layout

- `cmd/telesapr` is the window, `cmd/telesapr-cli` the command-line tool. Both are thin.
- `pkg/` holds the packages (a recorded deviation from the usual root layout: see
  `doc/adr/0003-pkg-directory.md`; do not move them). Private helpers go under `pkg/internal/`
  or a package's own `internal/`. There is no `util`, `common` or `helpers` package.
- The layers, each importing only the ones above it: `geom`, `libm`, `ttf`, `dxf` → `core` →
  `srf`, `solver`, `thread` → `topo` → `sketch` → `persist/*`, `export`, `check`, `render` →
  `script/*`, `mcpserver`, `platform/*` → `ui` → `cmd/*`. The font and DXF libraries know nothing
  of the kernel: text outlines go through `ttf.OutlineSink`, which `srf.OutlineInto` takes into
  Béziers. `pkg/sketch` is the model: every way of changing a model (window, script, file)
  reaches the same entry point there.
- One concern per file, named for what it holds (`group_generate.go`, not `util.go`). A function
  longer than a screen is split.

## Go

- Go 1.27 (`go.mod`). Use the language as it is: `range n`, `slices`, `maps`, `min`/`max`,
  `errors.Is`/`errors.As`, `any`. No `interface{}`, no `sort.Slice`, no hand-rolled loops where
  the standard library has the function.
- Errors are values: a function that can fail returns `error`, last. Wrap with
  `fmt.Errorf("doing x: %w", err)`; sentinel errors for what callers branch on. No `bool` for
  success, no panic for anything a file or a user can cause; `panic` is for a programmer error
  only. Error strings are lowercase fragments; the user-facing sentence is made where the dialog
  is (`pkg/ui`).
- `srf.CxxSort` and `pkg/libm` stay as they are: the order of a mesh's triangles and the last
  bits of the kernel's arithmetic are what the goldens and the exports record (a stable sort in
  CxxSort's place moves nine of them; tried 2026-10-07).
- No process-wide mutable state. What a computation needs, it is given (a `Sketch`, an options
  struct). Package-level `var` is for tables and sentinels that nothing writes after
  initialisation. `init()` only for `runtime.LockOSThread` in `cmd/telesapr` and generated code.
  The tolerance a solid is followed at is `srf.Tolerance`, carried by every shell and surface
  and computed by the sketch per regeneration (`Sketch.Tol`); `srf.DefaultTolerance` is what a
  shell with none resolves to, and only tests set it. `TELESAPR_PARALLEL` is read once at
  startup by the binaries (`srf.SetParallelModeFromEnv`) and nowhere else.
- `context.Context` first where a call can be cancelled (scripts, agent, background
  regeneration). Every goroutine has an owner that waits for it; fan-out goes through
  `pkg/internal/par`, which bounds it.
- Naming: `MixedCaps`, initialisms kept whole (`ID`, `RGBA`, `STEP`), no `Get` prefix on an
  accessor, receivers one or two letters and the same on every method of a type. Exported
  identifiers have a doc comment that starts with their name; every package has a package
  comment.
- Comments say why, in terms of this code. They do not describe the code by reference to
  SolveSpace's C++ (`doc/adr/0001-separate-from-solvespace.md`).
- Lint is `golangci-lint` with `.golangci.yml`; the tree is clean. A `//nolint` carries a reason.
  `golangci-lint run --fix` is not run over `examples/`: the scripts there run in yaegi, which
  has no range over an integer or a function, so they keep the older forms. A range loop over a
  slice fixes its bound once; a loop that walks a slice while appending to it keeps its counting
  form, with a `//nolint:intrange` saying so.

## Tests and the definition of done

A feature is done when it is tested, has a user interface and has scripting support, and all
three reach the same `pkg/sketch` entry point. `doc/done-audit.md` keeps the table.

- Tests of a package whose tests touch no process-wide state call `t.Parallel()`; a test that
  sets `srf.DefaultTolerance`, `ttf.DefaultFonts`, an environment variable or the working
  directory cannot, and says why in a comment where it is not obvious.
- Tests sit beside the code. A new package holds at least 85% statement coverage and no exported
  function at 0% (`scripts/coverage-packages.txt`, `make coverage`). Solids are checked with the
  oracle `pkg/check` (watertight, no naked edges, volume where closed-form).
- Goldens: a `.tlspr` fixture per feature variant under `testdata/`; `.goui` text-window goldens
  under `testdata/ui/golden`; PNGs for the renderers. A golden changes only in a commit of its
  own that says why, made with the `scripts/regen-*.sh` script for it.
- Every creating and editing action is one undo step and comes back identical on redo. Bad
  input gives a diagnostic on the group, never a panic.
- An interface scenario per feature under `testdata/ui/scripts`, run on the headless platform,
  and a path-equivalence test that builds the same model through the interface and a script.
- Scripting: a method on `pkg/script/api` per feature with a doc comment (`help()` and
  `docs.json` come from it), `make generate` run and `make generate-check` passing, an example
  under `examples/scripts` that a test executes.
- Three tiers. `make test-fast` (`go test -short`) is the check before a commit: a test that
  takes more than a few seconds skips itself under `testing.Short()`, saying what it is.
  `make test` runs every test, and `make ci` is what the GitHub workflow runs on `master`
  (checks, test, race, lint, vuln); on a branch the workflow runs `make test-fast` in
  place of `make test` and leaves out the race detector, both of which run on `master` and on
  request (workflow_dispatch), so `make ci` before the merge is what catches the rest. The nightly workflow, on
  `master`, runs what is too slow or costs too many minutes for a push: `make coverage` (the 85%
  rule, several times slower than `make test`), `make test-sweeps`,
  `make fuzz`, `make bench`, govulncheck as a failing job, the race detector over `pkg/ui`
  and `pkg/script/api`, and the quick tests on Windows and macOS. `make lint-new` lints what a branch changed.
- `scripts/go-test.sh` runs the slow packages (`SHARDED`) as several processes, their heaviest
  tests (`HEAVY`) one each; every test target goes through it. A new test of a minute or more in
  one of those packages goes into `HEAVY`.
- On Windows and macOS the workflows run `scripts/test-os.sh`: the quick tests without those
  `scripts/os-skip/<GOOS>.txt` lists, each with why it fails there. A fix takes its test off the
  list; a new test that fails there for a reason of the platform goes on it, with the reason.
- golangci-lint is pinned in `tools/lint/go.mod`, a module of its own, and `make lint` builds it
  into `bin/`; govulncheck is pinned in `go.mod`'s tool block. Neither comes from `PATH`.

### Environment variables the tests and tools read

| Variable | Effect |
|---|---|
| `TELESAPR_GUI_TESTS=1` | Runs the tests that open windows (`pkg/platform/tkgui`, `pkg/render/gl`); `make test-gui` sets it under Xvfb. They are skipped without it, and without `DISPLAY`. |
| `TELESAPR_REGEN_GOLDEN=1` | The golden tests rewrite their goldens instead of comparing; set by `scripts/regen-golden.sh`. |
| `UPDATE_GOLDENS=1` | The same for the software renderer's PNGs (`pkg/render/soft`). |
| `TELESAPR_REGEN_INVARIANTS=1` | Rewrites `testdata/invariants.txt` (`pkg/persist/slvs`). |
| `TELESAPR_SWEEP=1` | Robustness sweeps run their whole grid instead of a sample (`pkg/internal/sweep`). |
| `TELESAPR_STEP_CORPUS=dir` | `TestExternalCorpus` reads the STEP files `external.txt` lists from `dir`. |
| `TELESAPR_STEP_OUT=dir`, `TELESAPR_KEEP_STEP=1`, `TELESAPR_RENDER_DIR=dir`, `TELESAPR_PATHEQUIV_DIR=dir`, `TELESAPR_GL_DUMP=dir` | Tests in `pkg/ui` and `pkg/render/gl` keep what they produced there, for a diff. |
| `TELESAPR_UI_SCRIPT=name` | `TestUIScript` prints what the interface shows for one script of `testdata/ui/scripts`. |
| `TELESAPR_CACHE_DIR=dir` | Where tessellations of STEP files are kept; tests keep none unless it is set. |
| `TELESAPR_SCRIPT_OUT=dir` | `TestScriptRoundTrip` (`pkg/script`) writes there the script of each fixture that fails to come back. |
| `TELESAPR_BENCH_MODEL=file` | `BenchmarkRegenerateModel` (`pkg/persist/tlspr`) regenerates that model, one too large for `testdata` (`doc/notes/performance.md`). |
| `TELESAPR_CLI=file` | The `telesapr-cli` that `scripts/perf-fixture-hashes.sh` runs, in place of `bin/telesapr-cli`. |
| `TELESAPR_SOLVESPACE_SRC=dir` | A SolveSpace source tree, for `scripts/gen-dxf-cptables.py`, which reads its vendored libdxfrw. Not needed to build or test. |

At run time, `TELESAPR_PARALLEL=off` generates solids on one goroutine and
`TELESAPR_BACKGROUND=off` on the window's thread.

## Branches and commits

- One branch per work package or step, cut from `devel` and named for it. `make test` passes
  after each commit, `make ci` before the merge; the branch is merged into `devel` and deleted.
  `devel` is merged into `master` when it is to be pushed or released.
- A commit does one thing and says why in its message. Regenerated goldens are a commit of their
  own.
- takigo is a public module, required in `go.mod` at a tagged version.
- A release is `make release` on `master` (`doc/releasing.md`): the tag is `v` +
  `version.Version`, bumped by hand in a commit of its own.
