# Browser WASM spike: TeleSAPR in a browser, October 2026

Whether a browser version of TeleSAPR is possible. The route is **Go to WebAssembly**
(`GOOS=js GOARCH=wasm`), not a hand-written JS port. The plan below was written on 2026-10-21;
the spike ran on 2026-10-29 (branch `rd6-wasm-spike`) and its results come first.

## Results (2026-10-29)

**Go.** Every risk the plan named cleared on the first day: the whole program but the takigo
window builds for js/wasm unchanged, yaegi evaluates api scripts in node and in Chrome, a model's
files come out byte for byte as the native build's, and fonts need nothing new. What the spike
found against is the software renderer's speed: frames at 1280×800 take 56 to 155 ms in wasm (6 to 18 fps), mostly under
the 15 fps gate, so a browser version needs a WebGL renderer from the start.

### What builds

`GOOS=js GOARCH=wasm go build` succeeds for every package but two, with no source change and no
build tag added:

- built: `geom libm ttf dxf core srf solver thread topo sketch persist/... export/... check render
  render/soft script script/api script/symbols ui mcpserver platform/headless`, and
  `cmd/telesapr-cli` and every example under `examples/scripts`;
- not built: `cmd/telesapr` and `pkg/platform/tkgui`, through takigo (`takigo/font` has no
  `namedFontDefs` for js). They are reached only from `cmd/telesapr`, so nothing needs a tag.

`pkg/render/gl` keeps itself out by its `cgo && !nogl` tag, as the plan expected.
`pkg/script/agent`'s socket compiles, but a browser has no sockets: the agent is off there.

### What runs

- **The CLI, under node** (`go_js_wasm_exec`): `telesapr-cli thumbnail` of fillet-tee, the M8×30
  bolt and the guided loft ring writes PNGs **byte-identical** to the native build's. `telesapr-cli
  run examples/scripts/bracket/bracket.go` (yaegi) writes `bracket.tlspr`, `.stl`, `.step` and
  `.png` byte-identical to native. `pkg/libm`'s bit-exact arithmetic carries over: wasm has
  IEEE doubles and `math.FMA` is software there, with the same results.
- **Package tests under js/wasm** (`go test` through `go_js_wasm_exec`): `pkg/geom`,
  `pkg/render/soft` and `pkg/srf` pass in `-short` (srf takes 213 s).
  `pkg/script`: see below.
- **In Chrome** (headless, `cmd/telesapr-wasm/web/index.html`): the application on the headless
  platform loads a model from an in-memory file system, regenerates it, draws frames into a 2D
  canvas through `putImageData`, and runs a console script through yaegi.

`wasm_exec` refuses an environment over 12 KB ("total length of command line and environment
variables exceeds limit"); run it under `env -i`.

### Timings

The same probe (`cmd/telesapr-wasm`: `ui.New` on `platform/headless`, `LoadFromFile`, zoom to fit,
then 20 frames turning round the model at 1280×800), on one machine (x86-64, node v26.8.1, Chrome 156.0.8078.47
headless, no GPU; in Chrome 40 frames, drawn into the canvas):

| Model | Load + regenerate: native parallel / one thread / wasm node / Chrome | Frame mean: native parallel / one thread / wasm node / Chrome |
|---|---|---|
| fillet-tee | 0.14 s / 0.15 s / 0.77 s / 1.5 s | 15 ms / 37 ms / 118 ms / 153 ms |
| loft-ring-guide | 1.15 s / 1.24 s / 3.2 s / 3.3 s | 71 ms / 67 ms / 138 ms / 145 ms |
| bolt_m8x30 | 1.04 s / 1.91 s / 5.0 s / 5.0 s | 40 ms / 31 ms / 94 ms / 56 ms |

- Wasm is **2.5 to 5 times slower than one native thread**, and loses the parallel speed-up on
  top (js/wasm runs goroutines on one thread; `TELESAPR_PARALLEL` changes nothing there).
- Start-up: the module is 45 MB (`-s -trimpath` 44 MB), **12 MB gzipped**; node takes about 1.9 s
  to compile it, Chrome 0.2 to 0.6 s with `instantiateStreaming` (cached on the second load).
  `ui.New` plus `Init` takes 35 to 130 ms.
- Even natively the software renderer draws 1280×800 at 15 to 65 fps; in wasm 6 to 18.
  Smaller canvases were not measured.

### Fonts

Nothing to add. `pkg/ttf` embeds Bitstream Vera Sans (`ttf.EmbeddedFont`) and `pkg/res` embeds the
vector and bitmap fonts and icons (2.5 MB). `platform.SystemFontFiles` walks folders that a
browser doesn't have and returns none, without error, so text requests fall back to the embedded
font, as on a machine without fonts. A font of the user's would be a file put into the in-memory
file system and listed by the web platform's `FontFiles`.

### Files

Go's js port calls `globalThis.fs` in the shape of node's callback API (`syscall/fs_js.go`), and
`wasm_exec.js` keeps a `fs` already there. `cmd/telesapr-wasm/web/memfs.js` (about 100 lines) is
an in-memory one, with the `path.resolve` and `process` that `syscall` also reaches for. With it
`os.Open`, `os.Stat`, `os.WriteFile` and `filepath.WalkDir` work unchanged, so `tlspr.LoadFile`,
links between models, exports and autosave need no browser code. A file comes in by
`memfs.put` (from `<input type=file>` or a drop) and goes out by `memfs.get` and a download link;
persistence across visits would put the in-memory files in IndexedDB.

## What a `web` platform needs (sized)

The probe uses `platform/headless`; the editor needs a `pkg/platform/web` beside `tkgui`
(2730 lines) and `headless` (710), implementing the 79 methods of `pkg/platform`.

1. **The event loop on its own goroutine.** The platform's dialogs are synchronous
   (`MessageDialog.RunModal() Response`, `FormDialog.RunModal`, `FileDialog.RunModal`), and a Go
   function called from JS that blocks deadlocks the page. So JS events (mouse, keys, resize,
   timers, `requestAnimationFrame`) are queued onto one Go goroutine that runs `pkg/ui`, and a modal
   dialog blocks that goroutine on a channel while the page shows the dialog and answers it. That
   keeps `pkg/ui` as it is. **S** to prove, part of the platform below.
2. **WebGL renderer.** The 15 fps gate failed for the software renderer, so a WebGL 2 backend
   of `render.ViewportCanvas` (`pkg/render/webgl`) is needed for the editor to be usable. It can
   follow `pkg/render/gl`'s structure and GLSL (`render.Shade`), through `syscall/js` calls in place of
   go-gl. The soft renderer stays the fallback and draws the text window and thumbnails. **M** (two to
   three weeks), the largest piece.
3. **`pkg/platform/web`**: windows as canvases (the graphics window and the text window side by
   side, tool windows as floating panels), the menu bar and context menus in HTML, message and form
   dialogs in HTML, file dialogs through `<input type=file>` and downloads over memfs, timers on
   `setTimeout`, settings in `localStorage`, the cursor and tooltips, the text window's inline
   editor as an `<input>`, keyboard events mapped to `platform.KeyboardEvent`. **M** (two to three
   weeks), with the headless backend as the template of its semantics and `testdata/ui/scripts` run
   against it in node for the parts without a canvas.
4. **`cmd/telesapr-web`** (the probe grown up), the page, memfs with IndexedDB persistence, and a
   `make wasm` target. **S**.
5. **Size and speed.** 12 MB gzipped is a slow first visit, not a blocker; `pkg/script/symbols`
   and `pkg/res`' icons are the large parts to look at. Regeneration 2.5 to 5 times slower than
   native means models taking a second natively take several in the browser; that wants regeneration
   off the event goroutine with a progress indicator, which `ShellJob` already shapes. **S** to
   measure, open-ended to improve.

Altogether **L** (five to eight weeks) for an editor at parity with the window, minus the agent.
Not in it: threads in the browser (wasm threads need SharedArrayBuffer, cross-origin isolation, and
Go's js port does not use them), and the MCP agent (no sockets; a postMessage bridge would be its own
item).

## Reproducing

- `scripts/wasm-spike.sh [out-dir [model.tlspr]]` builds `cmd/telesapr-wasm` and the page into
  `out-dir` (default `bin/wasm`). Serve it (`python3 -m http.server -d bin/wasm 8080`) and open
  `http://localhost:8080/?frames=60` (`model=` names another file in the folder). The log is under
  the canvas and on the console.
- Under node: `env -i PATH="$PATH" "$(go env GOROOT)/lib/wasm/go_js_wasm_exec" bin/wasm/telesapr.wasm
  testdata/solid/fillet-tee.tlspr 20` prints the timings.
- Headless Chrome: `google-chrome --headless=new --enable-logging=stderr --v=0 <url>`, the lines
  starting `spike:`. Not `--virtual-time-budget`: it stops the clock, and every time reads 0.
- The CLI: `GOOS=js GOARCH=wasm go build -o cli.wasm ./cmd/telesapr-cli`, then
  `env -i PATH="$PATH" go_js_wasm_exec cli.wasm thumbnail ...` as natively.

---

## The plan (2026-10-21), as it was

### Why the route is straightforward

The program splits along three clean seams (found 2026-10-21):

- **`pkg/ui` is platform-agnostic.** The whole editor (menus, dialogs, sketch editing, console) is
  written against the `pkg/platform` interfaces; it imports no takigo, no cgo, no GL.
- **`pkg/platform` already has two backends** (`tkgui`, `headless`); a third, `web`, slots in
  beside them, and `headless` is a near-complete template of the semantics to implement
  (`pkg/platform/headless/headless.go`).
- **`pkg/render/soft` is a pure-Go renderer**, already the no-OpenGL fallback; `PresentFrame`
  reads `image.RGBA` back (`pkg/platform/headless/headless.go`), which is exactly what is
  uploaded to a `<canvas>` or WebGL texture.
- **`cmd/telesapr/main.go` is thin**: build a `platform.Platform`, `ui.New(p)`, `p.Run()`. A
  browser main copies this shape.

### The three risks

1. **yaegi under js/wasm**: the scripting console runs a Go interpreter
   (`github.com/traefik/yaegi`); pure Go, but reflect/`unsafe`-heavy. Cleared.
2. **Single-threaded performance**: js/wasm has no parallel scheduling. Regeneration is usable;
   the software renderer is not fast enough for the editor (WebGL needed).
3. **File and font lifecycle**: `os` has no real filesystem under js/wasm, and
   `platform.FontFiles()` walks font folders. Cleared by memfs and the embedded fonts.

### The spike's steps

- S0.1 compile and render smoke test (the gate): passed, see "What builds".
- S0.2 yaegi under wasm: passed, in node and in Chrome.
- S0.3 frame loop and FPS in the browser: ran; 6 to 18 fps at 1280×800, mostly under the 15 to 20 fps
  gate, so the WebGL renderer is a required backend, not a follow-up.
- S0.4 fonts: settled, embedded.
