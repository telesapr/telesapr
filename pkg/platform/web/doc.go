// Package web is the platform for a browser: TeleSAPR built for
// GOOS=js GOARCH=wasm draws into canvases of a page, shows its menus,
// dialogs and console as HTML, and reads and writes its files in the
// in-memory file system the page provides (cmd/telesapr-web/web/memfs.js).
// The design is in doc/dev-notes/browser-wasm-spike.md.
//
// The page calls into Go only through callbacks that enqueue: a Go
// function called from JavaScript that blocks would deadlock the page.
// One goroutine, the one that calls Platform.Run, takes the queue's
// functions one at a time and runs the whole user interface; a modal
// dialog runs the queue on that goroutine too, in a nested loop, until
// the page has answered it. Everything that needs no browser is in files
// without a build tag (the queue, key mapping, double clicks, scroll
// arithmetic, settings), so that it is tested natively; the rest builds
// for js/wasm only.
package web
