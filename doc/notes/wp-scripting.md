# WP scripting: A4 library, B5 forms, D3 file guard

Branch `wp-scripting`, cut from devel at 38cc68e1. Not merged; devel has moved since
(glTF + MCP tools in `pkg/mcpserver/server.go`, protocol v2), so the merge has a conflict there:
keep both the `form` argument of run_script and the new tools.

## State

- A4 (script library folder): done. `script.Options.Library`, `Engine.SetLibrary`,
  `DefaultLibrary` (`<config>/telesapr/library`); yaegi `GoPath` "." with a `libraryFS` that maps
  `src/<path>` to `<library>/<path>`; build tag `telesapr` for scripts the go tool can't build.
  `--lib` on run/repl/eval/`mcp --headless`; configuration screen row; example `library`.
- B5 (script forms): done. `api.Form`/`api.Field`, `script.ReadForm`/`ApplyForm` (values written
  into the source text in place of the literals, lines kept), `Options.Form`, `run --set`,
  run_script `form`, Script > Run File shows `platform.FormDialog`. Example `form`.
- D3 (file guard): done. `script.Files` (Read, Write, Folder func, Temp); engine replaces os and
  filepath calls that name files, `check.ReadBaseline`, api `Open`/`New`; removes sketch,
  persist/tlspr, persist/convert and `api.NewDoc`/`NewSketchHost`/`SetCurrent`/`Import`/`SketchHost`;
  api calls ask `api.FileGuard`; `Doc.Host()` panics for a guarded doc (use `Engine.Host()`).
  Env is yaegi's own, empty when guarded. `agent.GuardFiles` is the AI policy; on by default in
  the window (separate interpreter for AI scripts) and in `mcp --headless` (`--allow`,
  `--unguarded`). `TestGuardCoversFiles` makes every string-taking call classified.

## Traps

- yaegi replaces os.Getenv/Setenv/Environ with its own env map: they are not process-wide.
- `typedFloats` returns the shared generated table when nothing changes: copy (`own`) before
  editing a package's exports.
- Text window clicks: "row N" in a dump is the row's top in half-rows; a row spans y in
  [10N, 10N+20] and the earlier row wins at the boundary. Click at 10N+5.
- Do not `pkill -f ui.test`: it kills other worktrees' test runs (it did once here).
- New devel code that reads or writes a file from package api (relink, glTF export, MCP
  export/open/list_files tools) must call `d.checkFile`, or `TestGuardCoversFiles` /
  `TestGuardedScript` will not know; MCP tools that bypass scripts should respect the guard too.

## Next

- `make ci` on the branch; then the merge into devel resolves server.go.
