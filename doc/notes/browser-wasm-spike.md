# Browser WASM spike plan: TeleSAPR in a browser, October 2026

Whether a browser version of TeleSAPR is possible, and the spike that proves it. The route is
**Go to WebAssembly** (`GOOS=js GOARCH=wasm`), not a hand-written JS port: the kernel is pure Go
and compiles to wasm as it is, and the UI layer is already written against a `platform.Platform`
interface with two backends. The open questions are whether the yaegi script engine runs under
js/wasm, whether the software renderer's frames reach a browser fast enough, and how fonts are
found there.

## Why the route is straightforward

The program splits along three clean seams (found 2026-10-21):

- **`pkg/ui` is platform-agnostic.** The whole editor (menus, dialogs, sketch editing, console) is
  written against the `pkg/platform` interfaces; it imports no takigo, no cgo, no GL.
- **`pkg/platform` already has two backends** (`tkgui`, `headless`); a third, `web`, slots in
  beside them, and `headless` is a near-complete template of the semantics to implement
  (`pkg/platform/headless/headless.go`).
- **`pkg/render/soft` is a pure-Go renderer**, already the no-OpenGL fallback; `PresentFrame`
  reads `image.RGBA` back (`pkg/platform/headless/headless.go:411`), which is exactly what is
  uploaded to a `<canvas>` or WebGL texture.
- **`cmd/telesapr/main.go` is thin**: build a `platform.Platform`, `ui.New(p)`, `p.Run()`. A
  browser main copies this shape.

Everything below the platform line — `geom, libm, srf, solver, thread, topo, sketch, core,
persist, export, check, ttf, dxf` — is pure Go, no cgo, no assembly (`pkg/libm` uses `math.FMA`
and Go's `math`, not glibc). cgo is walled off to `pkg/render/gl` (already tagged `cgo && !nogl`
with a stub), `pkg/render/gl/glx` (linux-only), and `pkg/platform/tkgui` (takigo/X11, imported
only by `cmd/telesapr`).

## The three risks

1. **yaegi under js/wasm** — the scripting console runs a Go interpreter
   (`github.com/traefik/yaegi`); pure Go, but reflect/`unsafe`-heavy. Highest risk to "scripting
   parity".
2. **Single-threaded performance** — js/wasm has no parallel scheduling, so `pkg/internal/par`
   solver/meshing fan-out and off-thread shells collapse to cooperative scheduling; heavy solids
   may render too slowly on the software renderer.
3. **File and font lifecycle** — `.tlspr` load/save and STL/STEP/3MF export use `os` (no real
   filesystem under js/wasm), and `platform.FontFiles()` discovers fonts through fontconfig.

## The spike

Fail-fast on the three risks before any UI is built. The spike builds no editor.

### S0.1 Wasm compile + render smoke test (the gate)

Prove the pure-Go kernel and `pkg/render/soft` cross-compile and produce pixels, with no
cgo/takigo/gl. A throwaway probe (a `cmd/telesapr-wasm` main, or a `_test.go` under
`//go:build js && wasm`) builds a trivial solid via `pkg/sketch` (`Features.Extrude` of a
rectangle), sets a `render.Camera`, calls `soft.New()`, draws the mesh, and reads
`ReadFrame() → *image.RGBA` — the exact path `headless.Window.PresentFrame` uses.

- Build with `GOOS=js GOARCH=wasm go build ./cmd/...`.
- Identify exactly which files need a `!js` (or `&& !nogl`) tag to keep `tkgui`, `gl` and `glx`
  out of the js/wasm import graph.
- **Go/no-go**: if the kernel or soft renderer fail to compile for js/wasm, the port stops here.

### S0.2 yaegi-under-wasm probe (highest risk)

Prove `pkg/script` evaluates a real sketch script under js/wasm, reusing the minimal `api.Host`
from `pkg/script`'s tests (`engine_test.go`, `run_test.go`), with source like
`doc.Sketch.Rect(...); doc.Features.Extrude(...); fmt.Println(...)`.

- The symbol tables are pre-generated (`pkg/script/symbols/*.go`, tagged `go1.27`), so no
  `yaegi extract` runs at wasm-build time; the wasm binary only binds the committed
  `reflect.Value` tables through `interp.Use` (`pkg/script/engine.go:206,229`). The risk is
  yaegi's runtime (reflect/`unsafe`/CFG lowering) under js/wasm, not the tables.
- Verified with `go test` under js/wasm through `go_js_wasm_exec` and node, so it is repeatable
  and CI-able.
- **Go/no-go**: if yaegi cannot Eval an api script in wasm, "scripting parity" is blocked; the
  fallback is a JS-side script bridge.

### S0.3 Frame loop + FPS (in the browser)

Feed the S0.1 render from a minimal `index.html` + `wasm_exec.js`: build and re-render a
mid-complexity model repeatedly, blit `ReadFrame()` into a `<canvas>` `ImageData`, and time FPS.

- **Go/no-go**: below about 15-20 fps for a representative solid single-threaded, the WebGL
  renderer (`pkg/render/gl` through the existing `render.ViewportCanvas` abstraction) becomes a
  required near-term backend, not an optional follow-up.

### S0.4 Font/text probe

Confirm `pkg/ttf` parses and outlines a bundled font wasm-side (pure-Go parsing — expected to
work), and decide the `platform.FontFiles()` substitute: `go:embed` a font set (there is already
`pkg/ttf/fonts/`) versus reusing `res.BitmapFont`.

## Build and test mechanics

- `GOOS=js GOARCH=wasm go build ./...`, excluding the tagged-out packages, for the compile gate.
- `GOOS=js GOARCH=wasm go test ./pkg/script/... ./pkg/render/soft/...` under node via
  `go_js_wasm_exec` and `$(go env GOROOT)/misc/wasm/wasm_exec.js`.
- Watch for `pkg/script`/`pkg/sketch` tests that touch process-wide state (environment,
  `srf.DefaultTolerance`) and need js/wasm-specific handling.

## Out of scope for the spike

`pkg/platform/web`, the file I/O shim, menus and dialogs, and any editor wiring — those are a
later phase and depend on these results.

## Decision points the spike feeds

1. yaegi clears wasm → plan the script-console parity; otherwise → JS-side script bridge.
2. Soft-render FPS adequacy → WebGL backend priority.
3. Font strategy locked.