# Running in the browser

The editor builds for `GOOS=js GOARCH=wasm` (`cmd/telesapr-web`) on its own platform,
`pkg/platform/web`. The design and its measurements are in `browser-wasm-spike.md`.

## Build and run

    make wasm                                  # scripts/wasm-build.sh bin/web
    scripts/wasm-serve.py bin/web 8080         # then open http://localhost:8080/

`?model=<url>` opens a model once the editor is up. The program is about 43 MB, 40 MB after wasm-opt. The build
shrinks it with `wasm-opt` (binaryen) when that is on PATH or in `$WASM_OPT`. It writes no
compressed copy: SourceForge's project web, where `scripts/deploy-site.sh` puts it, ignores
`.htaccess` and so could not send one, and the Cloudflare in front of it compresses the `.wasm`
by itself, with brotli, to about 11 MB. `scripts/wasm-serve.py` sends it uncompressed.
The build leaves out WOFF2 fonts (`pkg/ttf/woff2_nobrotli_js.go`): the Brotli decoder imports
`net/http`, which cost 2 MB. Without GPU access WebGL runs in the browser's software rasteriser and
is slow; the software renderer is used when WebGL cannot be had (or when `UseOpenGL` is off).

## How it is put together

- `pkg/platform/web` is a `platform.Platform`. JavaScript events only enqueue work; one goroutine
  (`Platform.Run`) runs `pkg/ui`. A modal dialog or popup menu (`runModal`) runs the same queue in a
  nested loop until the page answers, so `RunModal` keeps its synchronous shape. Timers are
  `time.AfterFunc` posting to the queue; `PostToLoop` is the queue's `post`.
- The DOM is `web.js` and `web.css`, embedded in the package: windows (canvases; the text window and
  other tool windows are panels in a column at the right, resizable), the menu bar and context
  menus, message/form/file dialogs, the script console, tooltips, the editor `<input>`, the
  scrollbar.
- Pure logic builds natively and is tested there (`queue.go`, `keys.go`, `mouse.go`, `scroll.go`,
  `settings.go`, `filters.go`); the rest is `js && wasm`.
- Files: `memfs.js` is the file system Go's `os` sees. What is under `/home/` is kept in IndexedDB.
  File > Open takes a file from the user (`<input type=file>` or a drop) into `/home/user`; a save
  dialog asks for a name in `/home/user` and, once the application has written the file, offers it
  as a download. Saving without a dialog (Ctrl+S) writes `/home/user` only; the **Files** button in
  the menu bar lists, downloads and deletes what is stored.
- Settings are in `localStorage`. Fonts: the embedded one, plus any in `/home/user/.local/share/fonts`.

## Page functions (tests, scripts)

`telesaprOpen(path)`, `telesaprRun(src)` (both promises) and `telesaprStats()`; see
`cmd/telesapr-web/main.go`.

## Not done / known limits

- Regeneration runs on the event goroutine (a background goroutine gains nothing on one thread
  without preemption); a big model freezes the page while it builds.
- No agent (MCP) connections; no `testdata/ui/scripts` run against this platform (they use the
  headless platform's recorded dialogs); no overwrite prompt in the save dialog; "Exit" shows a
  notice, the page has to be reloaded; Alt-key shortcuts and keys the browser reserves
  (Ctrl+N/T/W) cannot be taken.
- Driving it headlessly: Chrome with `--remote-debugging-port` and plain CDP; `DOM.setFileInputFiles`
  fills the open dialog, `Browser.setDownloadBehavior` catches downloads.
