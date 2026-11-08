# TeleSAPR: how the code is written

Read this before changing anything. `README.md` says what the program is, `ROADMAP.md` what is
open. This file is the conventions, for people and coding agents alike; `CLAUDE.md` imports it.
Rules for one area load with that area's files (`.claude/rules/`, listed at the end), and the
multi-step procedures are skills (`.claude/skills/`). `make checks` runs `scripts/agents-check.sh`,
which fails when any of these files names a make target, path, test, variable or identifier that
does not exist: keep them true, in the same commit as the change.

## Layout

- `cmd/telesapr` is the window, `cmd/telesapr-cli` the command line (also the MCP server and
  the script runner), `cmd/telesapr-web` the same window built for js/wasm. All three are thin.
- `pkg/` holds the packages, a deliberate deviation from the usual root layout: do not move
  them. `github.com/telesapr/telesapr/pkg/script/api` is imported by every user script, the
  examples and the generated symbol tables. Private helpers go under `pkg/internal/` (`par`
  bounded fan-out, `sweep` sampled grids, `fixtures`, `golden`, `cdt`) or a package's own
  `internal/`. There is no `util`, `common` or `helpers` package.
- The layers, each importing only what is left of it:
  `libm` → `geom` → `core` → `srf`, `solver` → `topo` → `sketch` → `check`, `export`, `render`,
  `persist/*` → `script/*`, `mcpserver`, `platform/*` → `ui` → `cmd/*`.
  `ttf`, `dxf`, `thread` and `res` (embedded resources) are leaves that `sketch`, `export` and
  `render` use; `persist/step` sits with `topo`, since `persist/link` reads STEP. The font and
  DXF libraries know nothing of the kernel: text outlines go through `ttf.OutlineSink`, which
  `srf.OutlineInto` takes into Béziers. `pkg/sketch` is the model: every way of changing a model
  (window, script, file) reaches the same entry point there.
- One concern per file, named for what it holds (`group_generate.go`, not `util.go`). A function
  longer than a screen is split.
- `doc/guide` is the user's guide (a page per feature, built on `examples/scripts`),
  `doc/guide/reference` the menus and script calls, `doc/dev-notes` how things are made,
  `doc/dev-notes/done-audit.md` the definition of done and its table, `doc/tlspr-format.md` the
  file format, `doc/releasing.md` the release.

## Go

- Go 1.27 (`go.mod`). Use the language as it is: `range n`, `slices`, `maps`, `min`/`max`,
  `errors.Is`/`errors.As`, `any`. No `interface{}`, no `sort.Slice`, no hand-rolled loops where
  the standard library has the function.
- Errors are values: a function that can fail returns `error`, last. Wrap with
  `fmt.Errorf("doing x: %w", err)`; sentinel errors for what callers branch on. No `bool` for
  success, no panic for anything a file or a user can cause; `panic` is for a programmer error
  only. Error strings are lowercase fragments; the user-facing sentence is made where the dialog
  is (`pkg/ui`).
- No process-wide mutable state. What a computation needs, it is given (a `Sketch`, an options
  struct). Package-level `var` is for tables and sentinels that nothing writes after
  initialisation. `init()` is for `runtime.LockOSThread` in `cmd/telesapr`, generated code,
  tables computed once (`pkg/ttf`) and a format registering its readers (`pkg/persist/tlspr`).
  `TELESAPR_PARALLEL` is read once at startup by the binaries (`srf.SetParallelModeFromEnv`)
  and nowhere else.
- `context.Context` first where a call can be cancelled (scripts, agent, background
  regeneration). Every goroutine has an owner that waits for it; fan-out goes through
  `pkg/internal/par`, which bounds it.
- Naming: `MixedCaps`, initialisms kept whole (`ID`, `RGBA`, `STEP`), no `Get` prefix on an
  accessor, receivers one or two letters and the same on every method of a type. Exported
  identifiers have a doc comment that starts with their name; every package has a package
  comment.
- Comments say why, in terms of this code. They do not describe the code by reference to
  SolveSpace's C++: since 2026-10-01 TeleSAPR tracks neither SolveSpace's behaviour nor its
  file format, and the provenance is in `NOTICE`.
- Lint is `golangci-lint` with `.golangci.yml`, pinned in `tools/lint/go.mod`; the tree is
  clean. A `//nolint` carries a reason. `examples/` keeps the older loop forms (yaegi has no
  range over an integer or a function) and is not run through `--fix`.
- takigo is a public module, required in `go.mod` at a tagged version.

## Definition of done

A feature is done when it is tested, has a user interface and has scripting support, all three
reaching the same `pkg/sketch` entry point, and the documents describe it as it is.
`doc/dev-notes/done-audit.md` keeps the table; the `new-feature` skill walks the order. In
short: entry point and undo state in `pkg/sketch`; save and load; a menu entry and text-window
screen; a method on `pkg/script/api` with a doc comment, `make generate` run; an example under
`examples/scripts`; a `.tlspr` golden per variant; an interface scenario under
`testdata/ui/scripts` and a path-equivalence test; the guide page, reference, done-audit row,
`ROADMAP.md` row (taken out when done) and `CHANGELOG.md` line.

## Tests

- `make test-fast` (`go test -short`, no `HEAVY` test) is the check before a commit, under a
  minute. `make test` is every test but the heavy ones, `make test-heavy` only those, `make ci`
  what CI runs on `master` plus the race detector: run it before a merge. Details, the sweeps,
  the coverage rule and the environment variables are in `.claude/rules/testing.md`.
- Tests sit beside the code and call `t.Parallel()` unless they set something of the process's,
  and then say so. Solids are checked with the oracle `pkg/check`. Bad input gives a diagnostic
  on the group, never a panic. Every creating and editing action is one undo step and comes
  back identical on redo.
- A golden changes only in a commit of its own that says why, made with its `scripts/regen-*.sh`
  script (the `regen-goldens` skill).

## Branches and commits

- One branch per work package or step, cut from `devel` and named for it. `make test` passes
  after each commit, `make ci` before the merge; the branch is merged into `devel` and deleted
  (the `finish-branch` skill). `devel` is merged into `master` to push or release.
- A commit does one thing and says why in its message. Regenerated goldens are a commit of their
  own. Never `git stash` in this repository: the user keeps stash entries of their own.
- A change a user would notice (a feature, a changed behaviour, a fix, a removal) adds a line to
  `## [Unreleased]` in `CHANGELOG.md`, in the same commit. Internal work does not.
- A change in code brings the documents that describe it up to date in the same commit:
  `.claude/rules/docs.md` says which document holds what. After a rename or a removal, grep
  `doc/` and `ROADMAP.md` for the old name.
- A release is `make release` on `master` (`doc/releasing.md`); the version is bumped by hand in
  a commit of its own.

## Where the rest is

| Working on | Read |
|---|---|
| `pkg/srf`, `pkg/topo`, `pkg/geom`, `pkg/libm`, `pkg/core`, `pkg/solver`, `pkg/thread`, `pkg/check` | `.claude/rules/kernel.md` |
| `pkg/sketch`, `pkg/persist/*`, `pkg/export`, `testdata/*.tlspr` | `.claude/rules/model.md` |
| `pkg/ui`, `pkg/platform/*`, `pkg/render/*`, `testdata/ui` | `.claude/rules/ui.md` |
| `pkg/script/*`, `pkg/mcpserver`, `cmd/telesapr-cli`, `examples/` | `.claude/rules/scripting.md` |
| tests, `testdata/`, `scripts/`, `Makefile`, `.github/` | `.claude/rules/testing.md` |
| `doc/`, `ROADMAP.md`, `CHANGELOG.md`, `README.md` | `.claude/rules/docs.md` |

Claude Code loads each when a matching file is read or edited; another agent reads the row for
the files it is about to touch.
