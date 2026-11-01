//go:build js && wasm

package web

import (
	_ "embed"
	"errors"
	"fmt"
	"sync"
	"syscall/js"
	"time"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/platform"
	"github.com/telesapr/telesapr/pkg/render"
)

//go:embed web.js
var pageScript string

//go:embed web.css
var pageStyle string

// Platform is a platform.Platform in a browser page. It is used from
// the goroutine that calls Run, and through PostToLoop from the others.
type Platform struct {
	// NewGLRenderer, if set, makes the WebGL renderer the graphics
	// window draws with when the application asks for OpenGL; it draws
	// into the canvas it is given, whose size is in device pixels. Unset,
	// or when it fails, the windows draw in software.
	NewGLRenderer func(canvas js.Value, width, height int) (render.ViewportCanvas, error)
	// OnDropFiles, if set, is called on the event loop with the paths,
	// in the in-memory file system, of files dropped on the page.
	OnDropFiles func(paths []string)

	q        *eventQueue
	w        js.Value // the object web.js returns
	settings core.Settings
	done     chan struct{}
	exit     sync.Once

	// modal counts the dialogs and popup menus running: while one does,
	// the windows get no mouse and keyboard events.
	modal int

	mainWin *Window
	bar     *MenuBar
	focused *Window
	windows []*Window
	// The pointer's last place in the page, where a popup menu opens.
	pointerX, pointerY float64
	// keep holds the functions given to the page for good.
	keep []js.Func

	glReported bool
	stats      RenderStats
}

var (
	_ platform.Platform    = (*Platform)(nil)
	_ platform.LoopPoster  = (*Platform)(nil)
	_ platform.ConsoleHost = (*Platform)(nil)
)

// RenderStats are the times the windows took to draw.
type RenderStats struct {
	Frames int
	Last   time.Duration
	Total  time.Duration
	Max    time.Duration
}

// Mean is the mean time of a frame.
func (s RenderStats) Mean() time.Duration {
	if s.Frames == 0 {
		return 0
	}
	return s.Total / time.Duration(s.Frames)
}

// New builds the page: the layout, the menu bar's place and the key
// listeners. It fails outside a browser.
func New() (*Platform, error) {
	doc := js.Global().Get("document")
	if doc.IsUndefined() || doc.IsNull() {
		return nil, errors.New("no document: not running in a browser")
	}
	style := doc.Call("createElement", "style")
	style.Set("textContent", pageStyle)
	doc.Get("head").Call("appendChild", style)
	w := js.Global().Get("Function").New(pageScript).Invoke()
	p := &Platform{q: newEventQueue(), w: w, done: make(chan struct{})}
	p.settings = kvSettings{s: localStore{js.Global().Get("localStorage")}}
	w.Call("init")
	w.Call("onKey", p.fn(p.keyEvent))
	w.Set("onDropFiles", p.fn(p.dropFiles))
	return p, nil
}

// fn makes a function for the page that stays for as long as the
// platform does.
func (p *Platform) fn(f func(args []js.Value) any) js.Func {
	jf := js.FuncOf(func(_ js.Value, args []js.Value) any { return f(args) })
	p.keep = append(p.keep, jf)
	return jf
}

// Settings returns the settings, kept in the page's localStorage.
func (p *Platform) Settings() core.Settings { return p.settings }

// PostToLoop has fn run on the event loop's goroutine; it may be called
// from any goroutine.
func (p *Platform) PostToLoop(fn func()) { p.q.post(fn) }

// LoopDone is closed once the event loop has ended.
func (p *Platform) LoopDone() <-chan struct{} { return p.done }

// Run serves the event loop until Exit. The page's events and the
// timers queue their work for it; it never waits on the page.
func (p *Platform) Run() {
	defer close(p.done)
	p.q.run(func() bool { return false })
}

// Exit ends the event loop and tells the user so.
func (p *Platform) Exit() {
	p.exit.Do(func() {
		p.q.close()
		p.w.Call("exited")
	})
}

// Stats are the times the windows took to draw so far.
func (p *Platform) Stats() RenderStats { return p.stats }

// runModal runs the event queue, on the loop's goroutine, until the
// page has ended the modal state: show is given the function the page's
// callback calls to end it, which takes what to do on the loop before
// that (store the answer). The input events of the windows are dropped
// meanwhile.
func (p *Platform) runModal(show func(end func(apply func()))) {
	p.modal++
	defer func() { p.modal-- }()
	finished := false
	show(func(apply func()) {
		p.q.post(func() {
			if apply != nil {
				apply()
			}
			finished = true
		})
	})
	p.q.run(func() bool { return finished })
}

// CreateTimer returns a timer on the event loop.
func (p *Platform) CreateTimer() platform.Timer { return &Timer{p: p} }

// Timer is a single-shot timer; it fires on the event loop.
type Timer struct {
	p         *Platform
	onTimeout func()
	gen       int
	timer     *time.Timer
}

// SetOnTimeout sets what runs when the timer fires.
func (t *Timer) SetOnTimeout(fn func()) { t.onTimeout = fn }

// RunAfter (re)starts the timer.
func (t *Timer) RunAfter(ms int) {
	t.cancel()
	gen := t.gen
	fire := func() {
		t.p.q.post(func() {
			if t.gen != gen {
				return
			}
			if t.onTimeout != nil {
				t.onTimeout()
			}
		})
	}
	if ms <= 0 {
		fire()
		return
	}
	t.timer = time.AfterFunc(time.Duration(ms)*time.Millisecond, fire)
}

func (t *Timer) cancel() {
	t.gen++
	if t.timer != nil {
		t.timer.Stop()
		t.timer = nil
	}
}

// RunAfterNextFrame fires when the browser is about to draw a frame.
func (t *Timer) RunAfterNextFrame() {
	t.cancel()
	gen := t.gen
	var raf js.Func
	raf = js.FuncOf(func(js.Value, []js.Value) any {
		raf.Release()
		t.p.q.post(func() {
			if t.gen == gen && t.onTimeout != nil {
				t.onTimeout()
			}
		})
		return nil
	})
	js.Global().Call("requestAnimationFrame", raf)
}

// RunAfterProcessingEvents fires once the events queued now are handled.
func (t *Timer) RunAfterProcessingEvents() { t.RunAfter(0) }

// OpenInBrowser opens url in a new tab.
func (p *Platform) OpenInBrowser(url string) {
	js.Global().Call("open", url, "_blank", "noopener")
}

// FontFiles lists the fonts in the usual font folders of the in-memory
// file system, which hold what the user put there: a browser has no
// fonts of its own to offer, and the text falls back to the font the
// program embeds.
func (p *Platform) FontFiles() []string { return platform.SystemFontFiles() }

// offerDownload downloads the file at path once the application has
// written it, which it does after the dialog returns: the file is
// offered when it exists and its size has stopped changing.
func (p *Platform) offerDownload(path string) {
	last := int64(-1)
	var check func(n int)
	check = func(n int) {
		size := p.fileSize(path)
		switch {
		case size >= 0 && size == last:
			p.w.Call("download", path)
			return
		case n >= 100:
			return
		}
		last = size
		time.AfterFunc(100*time.Millisecond, func() { p.q.post(func() { check(n + 1) }) })
	}
	p.q.post(func() { check(0) })
}

// fileSize is the size of a file of the in-memory file system, -1 if
// there is none.
func (p *Platform) fileSize(path string) int64 {
	b := js.Global().Get("memfs").Call("get", path)
	if b.IsUndefined() || b.IsNull() {
		return -1
	}
	return int64(b.Get("length").Int())
}

// dropFiles is called by the page with the paths of files dropped on it.
func (p *Platform) dropFiles(args []js.Value) any {
	var paths []string
	if err := jsonUnmarshal(args[0].String(), &paths); err != nil {
		return nil
	}
	p.q.post(func() {
		if p.OnDropFiles != nil && p.modal == 0 {
			p.OnDropFiles(paths)
		}
	})
	return nil
}

// keyEvent is the page's key listener: it queues the key for the window
// in focus and says whether the browser is to leave the key alone.
func (p *Platform) keyEvent(args []js.Value) any {
	typ, key, code := args[0].Int(), args[1].String(), args[2].String()
	shift, ctrl, alt, meta := args[3].Bool(), args[4].Bool(), args[5].Bool(), args[6].Bool()
	cands := keyCandidates(key, code, shift, ctrl, alt, meta)
	if len(cands) == 0 {
		return false
	}
	p.q.post(func() {
		if p.modal > 0 {
			return
		}
		w := p.focused
		if w == nil {
			w = p.mainWin
		}
		if w != nil {
			w.deliverKey(platform.KeyboardEventType(typ), cands)
		}
	})
	return takesKey(key, code, shift, ctrl, alt, meta)
}

// logOnce reports a failure to make the WebGL renderer, the first time.
func (p *Platform) glFailed(err error) {
	if !p.glReported {
		p.glReported = true
		js.Global().Get("console").Call("warn", fmt.Sprintf("WebGL renderer unavailable, drawing in software: %v", err))
	}
}
