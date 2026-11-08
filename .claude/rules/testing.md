---
paths:
  - "**/*_test.go"
  - "testdata/**"
  - "scripts/**"
  - "Makefile"
  - ".github/**"
  - "tools/**"
---

# Tests, tiers and tooling

## Three tiers

- `make test-fast` (`go test -short`, without the `HEAVY` tests) is the check before a commit,
  under a minute on a workstation: a test that takes more than a few seconds of processor skips
  itself under `testing.Short()`, saying what it is, or keeps a case or two. `make test-cost`
  lists what each of its tests costs and fails past `TEST_COST_LIMIT`.
- `make test` runs every test but the heavy ones, `make test-heavy` only those, and `make ci`
  what the GitHub workflow runs on `master` (checks, test, test-heavy, lint, vuln) and then the
  race detector over every test but the heavy ones (`make test-race`). On a branch the workflow
  runs `make test-fast` in place of test and test-heavy, so `make ci` before the merge is what
  catches the rest. The race detector in CI runs only on request (workflow_dispatch).
- The nightly workflow runs what is too slow for a push: `make coverage` (the 85% rule),
  `make test-sweeps`, `make fuzz`, `make bench`, govulncheck as a failing job, the race detector
  over `pkg/ui`, `pkg/script/api` and the heavy tests (`make test-race-heavy`), and the quick
  tests on Windows and macOS. It is switched off in the repository's settings, so
  `make test-sweeps` is run by hand after a change to what a sweep or a sampled table covers.
- `scripts/go-test.sh` runs the tests in one `go test`; every test target goes through it, and
  `TESTS=light|heavy|all` picks whether the `HEAVY` tests are left out, run alone or run with the
  rest. A new test of a minute or more of processor goes into `HEAVY`; a name there is defined
  in one package only (`TestHeavyNamesUnique`), since `-run` and `-skip` match it everywhere.
- `make lint-new` lints what a branch changed. golangci-lint is pinned in `tools/lint/go.mod`
  and built into `bin/`; govulncheck is pinned in `go.mod`'s tool block. Neither comes from `PATH`.
- On Windows and macOS the workflows run `scripts/test-os.sh`: the quick tests without the
  `scripts/os-skip/<GOOS>.txt` lists, each line with why it fails there. A fix takes its test
  off the list; a new test that fails there for a reason of the platform goes on it.

## Writing a test

- Tests of a package whose tests touch no process-wide state call `t.Parallel()`; a test that
  sets `srf.DefaultTolerance`, `ttf.DefaultFonts`, an environment variable or the working
  directory cannot, and says why in a comment where it is not obvious.
- A new package holds at least 85% statement coverage and no exported function at 0%
  (`scripts/coverage-packages.txt`, `make coverage`). Solids are checked with the oracle
  `pkg/check` (watertight, no naked edges, volume where closed-form).
- A grid of cases (sizes × angles × shapes, …) is a `sweep.Grid` with a `Sample`, and a table of
  cases or oracle lines goes through `sweep.Pick`: the ordinary run takes an even sample,
  `-short` and the race detector a fifth of it (`sweep.Thin`), `make test-sweeps` all of it.
  Cases other tests already cover are left out with `sweep.Without` (a test over every fixture
  keeps a few of the thread variants, `fixtures.ThreadVariant`). A table that is the mirror
  image of itself is checked on one case per mirror pair. Cases are parallel subtests, not a
  serial loop, unless they must run in order; a base solid all cases share is built once.
- Goldens: a `.tlspr` fixture per feature variant under `testdata/`; `.goui` text-window goldens
  under `testdata/ui/golden`; PNGs for the renderers; `testdata/tlspr/golden` for the imports.
  A golden changes only in a commit of its own that says why, made with the `scripts/regen-*.sh`
  script for it (the `regen-goldens` skill). `testdata/tlspr/v1` is never regenerated.

## Environment variables the tests and tools read

| Variable | Effect |
|---|---|
| `TELESAPR_GUI_TESTS=1` | Runs the tests that open windows (`pkg/platform/tkgui`, `pkg/render/gl`); `make test-gui` sets it under Xvfb. Skipped without it, and without `DISPLAY`. |
| `TELESAPR_REGEN_GOLDEN=1` | The golden tests rewrite their goldens instead of comparing; set by `scripts/regen-golden.sh`. |
| `TELESAPR_SWEEP=1` | Robustness sweeps run their whole grid instead of a sample, and sampled tables and fixture lists all their cases (`pkg/internal/sweep`); set by `make test-sweeps`. |
| `TELESAPR_STEP_CORPUS=dir` | `TestExternalCorpus` reads the STEP files `testdata/step/corpus/external.txt` lists from `dir`. |
| `TELESAPR_STEP_OUT=dir`, `TELESAPR_KEEP_STEP=1`, `TELESAPR_RENDER_DIR=dir`, `TELESAPR_PATHEQUIV_DIR=dir`, `TELESAPR_GL_DUMP=dir` | Tests in `pkg/ui` and `pkg/render/gl` keep what they produced there, for a diff. |
| `TELESAPR_DOC_SHOTS=dir`, `TELESAPR_DOC_VIDEOS=dir` | `TestDocScenarios` (`pkg/ui`) writes the guide's pictures and videos of `doc/guide/scenarios` there; set by `scripts/regen-doc-images.sh` and `scripts/regen-guide-videos.sh`. |
| `TELESAPR_DOC_SCENARIO=name` | `TestDocScenario` prints what one scenario shows. |
| `TELESAPR_DOC_WINDOW_SHOTS=dir` | `TestDocWindowShots` (`pkg/ui`) writes the guide's pictures of the real window's menus and dialogs there; needs `DISPLAY`, `scripts/regen-doc-window-images.sh` gives it an Xvfb. Skipped without it. |
| `TELESAPR_UI_SCRIPT=name` | `TestUIScript` prints what the interface shows for one script of `testdata/ui/scripts`. |
| `TELESAPR_CACHE_DIR=dir` | Where tessellations of STEP files are kept; tests keep none unless it is set. |
| `TELESAPR_SCRIPT_OUT=dir` | `TestScriptRoundTrip` (`pkg/script`) writes there the script of each fixture that fails to come back. |
| `TELESAPR_TEST_AS_CLI=1` | Makes the `cmd/telesapr-cli` test binary run as the command, so `TestExamplesRun` runs each example in a process of its own. |
| `TELESAPR_BENCH_MODEL=file` | `BenchmarkRegenerateModel` (`pkg/persist/tlspr`) regenerates that model, one too large for `testdata` (`doc/dev-notes/performance.md`). |
| `TELESAPR_CLI=file` | The `telesapr-cli` that `scripts/perf-fixture-hashes.sh` runs, in place of `bin/telesapr-cli`. |
| `TELESAPR_SOLVESPACE_SRC=dir` | A SolveSpace source tree, for `scripts/gen-dxf-cptables.py`. Not needed to build or test. |

At run time, `TELESAPR_PARALLEL=off` generates solids on one goroutine and
`TELESAPR_BACKGROUND=off` on the window's thread.
