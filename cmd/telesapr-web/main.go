//go:build js && wasm

// Command telesapr-web is the TeleSAPR editor in a browser: the same
// application as cmd/telesapr, on the platform in pkg/platform/web, built
// for js/wasm and shown by the page in web/ (scripts/wasm-build.sh,
// make wasm; doc/dev-notes/browser.md).
//
// Besides the editor it sets functions on the page, for the page's own
// scripts and for tests that drive it:
//
//	telesaprOpen(path)   opens a model of the in-memory file system; a promise of "" or the error
//	telesaprRun(source)  runs Go statements on the model as the console does; a promise of the output
//	telesaprStats()      how long the windows took to draw: {frames, lastMs, meanMs, maxMs}
//
// and calls telesaprReady() once the editor is up.
package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"syscall/js"
	"time"

	"github.com/telesapr/telesapr/pkg/platform/web"
	"github.com/telesapr/telesapr/pkg/render"
	"github.com/telesapr/telesapr/pkg/render/webgl"
	"github.com/telesapr/telesapr/pkg/script"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/ui"
)

func main() {
	srf.SetParallelModeFromEnv()
	p, err := web.New()
	if err != nil {
		fmt.Println("telesapr-web:", err)
		return
	}
	p.NewGLRenderer = func(canvas js.Value, width, height int) (render.ViewportCanvas, error) {
		r, err := webgl.New(canvas, width, height)
		if err != nil {
			return nil, err
		}
		return r, nil
	}

	ss := ui.New(p)
	// Solids are made on the event loop's goroutine. Tried on: true works,
	// but js/wasm runs every goroutine on one thread and has no
	// preemption, so a regeneration off the loop still keeps the page from
	// answering until it is done, and nothing is gained.
	ss.BackgroundShells = false
	ss.ScriptHistory = script.DefaultHistory()
	ss.Init()

	p.OnDropFiles = func(paths []string) { dropped(ss, paths) }
	exportToJS(p, ss)
	if ready := js.Global().Get("telesaprReady"); ready.Type() == js.TypeFunction {
		ready.Invoke()
	}
	p.Run()
}

// dropped opens a model dropped on the page, imports a drawing, and
// leaves any other file where the dialogs find it.
func dropped(ss *ui.Application, paths []string) {
	if len(paths) == 0 {
		return
	}
	file := paths[0]
	switch strings.ToLower(filepath.Ext(file)) {
	case ".tlspr", ".tlsprz":
		if ss.OkayToStartNewFile() {
			ss.Report(ss.Load(file))
		}
	case ".dxf":
		ss.ImportDxf(file)
	default:
		ss.Message("%s was put in the browser's storage as %s; choose it in the dialog of File > Open, "+
			"Group > Link or File > Import to use it.", filepath.Base(file), file)
	}
}

// exportToJS sets the page's functions.
func exportToJS(p *web.Platform, ss *ui.Application) {
	g := js.Global()
	g.Set("telesaprOpen", js.FuncOf(func(_ js.Value, args []js.Value) any {
		path := args[0].String()
		return promise(func(resolve func(any)) {
			p.PostToLoop(func() {
				if err := ss.Load(path); err != nil {
					resolve(err.Error())
					return
				}
				resolve("")
			})
		})
	}))
	g.Set("telesaprRun", js.FuncOf(func(_ js.Value, args []js.Value) any {
		src := args[0].String()
		return promise(func(resolve func(any)) {
			// The script runs on a goroutine of its own, as the
			// console's do; its calls into the application go to the
			// event loop.
			go func() {
				out, err := runScript(ss, src)
				if err != nil {
					out += err.Error()
				}
				resolve(out)
			}()
		})
	}))
	g.Set("telesaprStats", js.FuncOf(func(js.Value, []js.Value) any {
		st := p.Stats()
		ms := func(d time.Duration) float64 { return float64(d) / float64(time.Millisecond) }
		return map[string]any{"frames": st.Frames, "lastMs": ms(st.Last), "meanMs": ms(st.Mean()), "maxMs": ms(st.Max)}
	}))
}

// promise makes a JavaScript promise that start resolves, possibly later.
func promise(start func(resolve func(any))) js.Value {
	var executor js.Func
	executor = js.FuncOf(func(_ js.Value, args []js.Value) any {
		resolve := args[0]
		executor.Release()
		start(func(v any) { resolve.Invoke(v) })
		return nil
	})
	return js.Global().Get("Promise").New(executor)
}

// runScript evaluates Go statements on the model and returns what they
// printed.
func runScript(ss *ui.Application, src string) (string, error) {
	var out bytes.Buffer
	e, err := script.New(script.Options{Host: ss.ScriptHost(), Stdout: &out, Stderr: &out})
	if err != nil {
		return "", err
	}
	shown, err := e.Run(context.Background(), "page", src)
	if err != nil && errors.Is(err, ui.ErrLoopEnded) {
		return out.String(), err
	}
	return out.String() + shown, err
}
