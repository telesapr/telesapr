//go:build js && wasm

// Command telesapr-wasm is the browser spike (ROADMAP F8,
// doc/notes/browser-wasm-spike.md): the whole application on the
// headless platform, its frames drawn by the software renderer and
// handed to a canvas. It builds no editor.
//
// Under node (scripts/wasm-spike.sh) it is given a model and times
// loading it, building its solids and drawing frames:
//
//	telesapr-wasm <model.tlspr> [frames]
//
// In a browser (web/index.html) it sets the functions telesaprOpen,
// telesaprFrame and telesaprRun on the page and waits for calls.
package main

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"os"
	"strconv"
	"syscall/js"
	"time"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/platform"
	"github.com/telesapr/telesapr/pkg/platform/headless"
	"github.com/telesapr/telesapr/pkg/script"
	"github.com/telesapr/telesapr/pkg/ui"
)

// app is the application and the window its frames are drawn in.
type app struct {
	ss *ui.Application
	w  *headless.Window
}

func newApp() *app {
	p := headless.New()
	// A browser has no font folders to walk: text falls back to the
	// font pkg/ttf embeds.
	ss := ui.New(p)
	ss.Init()
	a := &app{ss: ss}
	for _, w := range p.Windows {
		if w.Kind == platform.WindowToplevel {
			a.w = w
			break
		}
	}
	return a
}

// open loads path, which in a browser is a file the page put in its
// in-memory file system, and builds its solids.
func (a *app) open(path string, width, height float64) (time.Duration, error) {
	start := time.Now()
	if err := a.ss.LoadFromFile(path); err != nil {
		return 0, err
	}
	a.ss.AfterNewFile()
	a.w.Width, a.w.Height = width, height
	a.turn(0)
	a.ss.GW.ZoomToFit(false, false)
	return time.Since(start), nil
}

// turn looks at the model from 30° above, turned by angle about z.
func (a *app) turn(angle float64) {
	s, c := math.Sincos(angle)
	right := geom.Vector{X: c, Y: s}
	toward := geom.Vector{X: s * 0.866, Y: -c * 0.866, Z: 0.5}
	up := toward.Cross(right).WithMagnitude(1)
	a.ss.GW.SetProjection(right, up)
}

// frame draws the model turned by angle and returns how long it took.
func (a *app) frame(angle float64) time.Duration {
	start := time.Now()
	a.turn(angle)
	a.w.Render()
	return time.Since(start)
}

// run evaluates Go statements on the model, as the script console
// does, and returns what they printed.
func (a *app) run(src string) (string, error) {
	var out bytes.Buffer
	e, err := script.New(script.Options{Host: a.ss.ScriptHost(), Stdout: &out, Stderr: &out})
	if err != nil {
		return "", err
	}
	shown, err := e.Run(context.Background(), "console", src)
	return out.String() + shown, err
}

func main() {
	if len(os.Args) > 1 {
		os.Exit(bench(os.Args[1:]))
	}
	serve()
}

// bench times one model under node.
func bench(args []string) int {
	frames := 20
	if len(args) > 1 {
		frames, _ = strconv.Atoi(args[1])
	}
	start := time.Now()
	a := newApp()
	fmt.Printf("init %v\n", time.Since(start))
	d, err := a.open(args[0], 1280, 800)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Printf("load and regenerate %v\n", d)
	var total, worst time.Duration
	for i := range frames {
		f := a.frame(float64(i) * 2 * math.Pi / float64(frames))
		total += f
		worst = max(worst, f)
	}
	mean := total / time.Duration(max(frames, 1))
	fmt.Printf("%d frames 1280x800: mean %v (%.1f fps), worst %v\n", frames, mean, 1/mean.Seconds(), worst)
	return 0
}

// serve sets the page's functions and waits for calls.
func serve() {
	a := newApp()
	js.Global().Set("telesaprOpen", js.FuncOf(func(_ js.Value, args []js.Value) any {
		d, err := a.open(args[0].String(), args[1].Float(), args[2].Float())
		if err != nil {
			return map[string]any{"error": err.Error()}
		}
		return map[string]any{"ms": d.Seconds() * 1000}
	}))
	js.Global().Set("telesaprFrame", js.FuncOf(func(_ js.Value, args []js.Value) any {
		d := a.frame(args[0].Float())
		pix := args[1]
		if a.w.Frame != nil && pix.Length() == len(a.w.Frame.Pix) {
			js.CopyBytesToJS(pix, a.w.Frame.Pix)
		}
		return d.Seconds() * 1000
	}))
	js.Global().Set("telesaprRun", js.FuncOf(func(_ js.Value, args []js.Value) any {
		out, err := a.run(args[0].String())
		if err != nil {
			out += err.Error()
		}
		return out
	}))
	js.Global().Call("telesaprReady")
	select {}
}
