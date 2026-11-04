# Running in the browser

Refreshed 2026-11-03 at c4e18d1a; regeneration progress, keys, save and Exit 2026-11-04. This note took in `browser-wasm-spike.md`, the spike's results
and plan (2026-10-21 to 10-29); the old text is `git show c4e18d1a:doc/dev-notes/browser-wasm-spike.md`.

The editor builds for `GOOS=js GOARCH=wasm` (`cmd/telesapr-web`, landed 7907a288) on its own
platform, `pkg/platform/web`, drawing with `pkg/render/webgl`. The site's "Try in browser" page
(`site/try.html`) runs it from `site/app`, which `scripts/deploy-site.sh` fills from `bin/web`.

## Build and run

    make wasm                                  # scripts/wasm-build.sh bin/web
    scripts/wasm-serve.py bin/web 8080         # then open http://localhost:8080/

`?model=<url>` opens a model once the editor is up. The program is about 40 MB after `wasm-opt`
(binaryen, used when on PATH or in `$WASM_OPT`). No compressed copy is written: SourceForge's
project web ignores `.htaccess` and could not send one, and the Cloudflare in front of it
compresses the `.wasm` with brotli, to about 11 MB. `scripts/wasm-serve.py` sends it uncompressed.

Everything but `cmd/telesapr` and `pkg/platform/tkgui` (through takigo) builds for js/wasm with no
build tag; `pkg/render/gl` keeps out by its `cgo && !nogl` tag. Output (saves, exports,
thumbnails) is byte-identical to the native build's: wasm has IEEE doubles, and `math.FMA` is
software there with the same results.

## How it is put together

- `pkg/platform/web` is a `platform.Platform`. A Go function called from JavaScript that blocks
  deadlocks the page, so JavaScript events only enqueue work, and one goroutine (`Platform.Run`)
  runs `pkg/ui`. A modal dialog or popup menu (`runModal`) runs the same queue in a nested loop
  until the page answers, so `RunModal` keeps its synchronous shape. Timers are `time.AfterFunc`
  posting to the queue; `PostToLoop` is the queue's `post`.
- The DOM is `web.js` and `web.css`, embedded in the package. Pure logic (`queue.go`, `keys.go`,
  `mouse.go`, `scroll.go`, `settings.go`, `filters.go`) builds natively and is tested there; the
  rest is `js && wasm`.
- Drawing: WebGL 2 where the page has it, else the software renderer (also when `UseOpenGL` is
  off). Without GPU access WebGL runs in the browser's software rasteriser and is slow.
- Files: `memfs.js` is the file system Go's `os` sees (Go's js port calls `globalThis.fs` in the
  shape of node's callback API), so loading, links, exports and autosave need no browser code.
  What is under `/home/` is kept in IndexedDB. Open takes a file (`<input type=file>` or a drop)
  into `/home/user`; a save dialog writes there and offers the file as a download; Ctrl+S writes
  `/home/user` only. The **Files** button lists, downloads and deletes what is stored.
- Settings are in `localStorage`. Fonts: the embedded ones, plus any in
  `/home/user/.local/share/fonts`; `platform.SystemFontFiles` finds no system folders and returns
  none.
- WOFF2 fonts are left out (`pkg/ttf/woff2_nobrotli_js.go`): the Brotli decoder imports
  `net/http`, which cost 2 MB.
- Page functions for tests and scripts: `telesaprOpen(path)`, `telesaprRun(src)` (promises) and
  `telesaprStats()`; see `cmd/telesapr-web/main.go`.

## Decisions

- Regeneration runs on the event goroutine (`BackgroundShells = false`). Tried off it: js/wasm runs
  every goroutine on one thread without preemption, so the page froze just the same.
- The page is given its thread back between a regeneration's groups instead (2026-11-04). The
  sketch asks its hooks `Cancel` between groups; the application's then calls the platform's
  `Yield` when the platform is a `platform.Yielder`, which only this one is. Every 100 ms
  (`busy.go`) `Yield` waits for a frame (`nextFrame` in `web.js`: `requestAnimationFrame`, or
  50 ms where a background tab draws none); the goroutine waiting leaves the thread to the
  browser. After 300 ms a box over the page says how long the work has taken, and takes the
  clicks and keys meanwhile; the event queue's `after` hook takes it away once the function
  that started the work returns. Every regeneration on the loop goes through it: an edit, Open,
  a script's. Measured (headless Chrome, CDP, machine loaded): opening `bolt_m8x30` took 15 s
  and drew 6 frames with the box counting, where before it drew none. One group is not split:
  the thread's Boolean holds the page for its length.
- The keys a browser keeps (Ctrl+N, Ctrl+T, Ctrl+W, with Shift too) are taken as Ctrl+Alt and the
  key (`browserCtrlKey`, `keys.go`); full screen, `navigator.keyboard.lock` gives the page the
  Ctrl keys themselves where the browser has it (Chromium). Alt alone still opens the menus by
  their mnemonics (`keyMenu`, `web.js`).
- A save to a name already stored asks first (`runSave`, `filedialog_js.go`); Exit offers Start
  Again, a reload: the program can't run twice in one page.
- WebGL from the start, not later: in the spike the software renderer drew 1280×800 at 6 to 18 fps
  in wasm, under the 15 fps gate.
- No threads in the browser: wasm threads need SharedArrayBuffer and cross-origin isolation, and
  Go's js port does not use them. `TELESAPR_PARALLEL` changes nothing there.

## Speed (spike, 2026-10-29)

Load plus regeneration, one machine (x86-64, node v26.8.1, headless Chrome without GPU):

| Model | native, one thread | wasm (node / Chrome) |
|---|---|---|
| fillet-tee | 0.15 s | 0.77 s / 1.5 s |
| loft-ring-guide | 1.24 s | 3.2 s / 3.3 s |
| bolt_m8x30 | 1.91 s | 5.0 s / 5.0 s |

So wasm is 2.5 to 5 times slower than one native thread, and loses the parallel speed-up on top.
Chrome compiles the module in 0.2 to 0.6 s with `instantiateStreaming`.

## Size (2026-11-04)

What makes up the module, from the function bodies' sizes in an unstripped build (Go's `nm` does not
read wasm; the code section's bodies were summed by the package in their names): code 26 MB, data
17.5 MB (go1.27.3, before `wasm-opt`). The code by package, largest first: yaegi's interpreter 2.3,
`pkg/ui` 2.1, `pkg/topo` 2.0, `pkg/sketch` 2.0, `pkg/srf` 1.5, `pkg/script/api` 1.3, the runtime 1.2,
`pkg/ttf` 0.9, `pkg/script/symbols` 0.9 MB. The data is Go's tables (function and line tables, type
descriptors) but for 3.3 MB of embedded files: `pkg/res/data` 2.2 MB (the fonts, already gzipped;
the icons are 0.24 MB of PNG, not the large part the roadmap thought), `docs.json` 0.8 MB, the DXF
code pages 0.2 MB.

| Build | Raw | gzip -9 | brotli -q 5 |
|---|---|---|---|
| Before, `go build -ldflags=-s` | 43.62 MB | 11.76 MB | |
| Package `net` left out (below) | 42.26 MB | 11.51 MB | |
| Before, after `wasm-opt -Oz` (what `make wasm` ships) | 40.21 MB | 11.71 MB | 10.44 MB |
| After, after `wasm-opt -Oz` | 38.99 MB | 11.45 MB | 10.23 MB |

Brotli at its highest quality takes the first row to 8.92 MB; a host that compresses on the way
uses a faster level, which is nearer the column's.

- `net` came in only for the agent's Unix socket (`pkg/script/agent`, `pkg/ui/agent.go`), which a
  browser can't have: Go's js port answers it from an in-process stand-in that nothing outside the
  page can reach. The socket code is now `!js` (`socket_net.go`); the browser's `Listen` takes
  connections through the page's postMessage instead (`socket_js.go`, see Open).
- Tried, left: `testing` (imported by `pkg/persist/link` for `testing.Testing`) is 5 kB, `flag`
  being in already. `-ldflags=-w` changes nothing (no DWARF in wasm). `GOEXPERIMENT=nojsonv2`
  (encoding/json's old, smaller implementation; v2's packages are 0.8 MB of code) doesn't build:
  the scripts' symbol tables name v2's options.
- Left, each with a cost: `encoding/gob` (0.3 MB of code) is the STEP tessellation cache's format,
  which the browser keeps in IndexedDB; a format of its own would drop it. `docs.json` embedded
  gzipped would save 0.65 MB raw but nothing over the wire, and the generator would write a second
  file. yaegi with `go/build`, `go/doc`, `math/big` and `regexp` behind it, and the symbol tables,
  are the scripting console, which the browser has.

## The spike probe

The spike's timing probe (`cmd/telesapr-wasm`, `scripts/wasm-spike.sh`: the application on the
headless platform, timed under node) was removed on 2026-11-03; it is in `git show c4e18d1a`.

## Traps

- `go_js_wasm_exec` refuses an environment over 12 KB ("total length of command line and
  environment variables exceeds limit"): run it under `env -i`.
- Headless Chrome: not `--virtual-time-budget`, which stops the clock so every time reads 0.
  Drive it with `--remote-debugging-port` and plain CDP; `DOM.setFileInputFiles` fills the open
  dialog, `Browser.setDownloadBehavior` catches downloads.
- Package tests under js/wasm through `go_js_wasm_exec`: `pkg/geom`, `pkg/render/soft` and
  `pkg/srf` pass in `-short` (srf takes 213 s); `pkg/script`'s `-short` suite did not finish in
  55 minutes, and which test it was in is not known (`go test -v -run` per test is the next step).

## Open

- One slow group (a thread's Boolean) holds the page for its length: the yields are between groups.
- The agent's connection comes through postMessage (`pkg/script/agent/socket_js.go`,
  `msgconn.go`, `web/agent-client.js`), carrying the agent's own protocol, not MCP. Nothing yet
  carries MCP to it from a client outside the browser: the MCP server in the page would bring
  `net/http` back with the go-sdk (the 1.2 MB of Size), so a relay outside the page (an extension
  with native messaging to `telesapr-cli mcp`) is the likelier way. A site other than the page's
  is asked about in a dialog, which a headless browser can't answer, so only the page's own site
  was tried end to end.
- `testdata/ui/scripts` are not run against this platform (they use the headless platform's
  recorded dialogs). CI neither builds nor tests the js/wasm target, so a
  change can break it unseen; `make wasm` is the check.
- Module size: see Size; what is left to cut costs a feature or a format.
