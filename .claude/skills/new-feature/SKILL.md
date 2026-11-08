---
name: new-feature
description: Add a feature to TeleSAPR to the definition of done, in the order the project uses (entry point in pkg/sketch, file, interface, script API, tests, documents). Use when asked to add, build or implement a feature, a menu entry or a script call.
---

# Adding a feature

The definition of done is in `doc/dev-notes/done-audit.md`; this is its order. Work on a branch
cut from `devel`, named for the feature; `make test-fast` before each commit.

1. **Entry point.** A method on the model in `pkg/sketch` that makes the change, with its
   parameters in a struct; document state it adds goes into `UndoState` (`pkg/sketch/undo.go`).
   The kernel work, if any, is in `pkg/srf` or `pkg/topo`, refusing with an error where it
   cannot make the solid. Unit tests beside the code; every solid through `pkg/check`.
2. **File.** Save and load in `pkg/persist/tlspr`, described in `doc/tlspr-format.md`; a file
   of the previous version still loads. A `.tlspr` fixture per variant under `testdata/`.
3. **Interface.** A row in `pkg/ui/menu_table.go` (accelerator where free), selection-driven
   creation with a message when the selection does not fit, a text-window screen for every
   parameter, hover and selection highlighting for what the user picks, failure marked on the
   group. A scenario under `testdata/ui/scripts` with its `.goui` golden
   (`scripts/regen-goui.sh`).
4. **Script.** A method on `pkg/script/api` on the menu it belongs to, with a doc comment, to
   create, read, change and query; `make generate`; a test covering misuse too; an example under
   `examples/scripts/<name>` that `TestExamplesRun` runs.
5. **Path equivalence.** `testdata/ui/scripts/path_<name>.script` and `.go` building the same
   model, compared by `TestPathEquivalence`.
6. **Robustness.** For a solid feature, a `sweep.Grid` with its allowed failure rate fixed first;
   undo and redo identical; `go test -race` over the touched packages; a benchmark if it is on
   the regeneration path.
7. **Documents, same commit.** The guide page in `doc/guide` with its "Not yet", the reference
   pages, a row in `doc/dev-notes/done-audit.md`, the `ROADMAP.md` row taken out, a line under
   `## [Unreleased]` in `CHANGELOG.md`.
8. `make ci` before the merge; then the `finish-branch` skill.
