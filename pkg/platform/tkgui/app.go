// Package tkgui is the platform on the takigo Tk port: windows whose
// content is a frame the application renders in software, Tk menus,
// timers and dialogs. It ports platform/guigtk.cpp.
package tkgui

import (
	"fmt"
	"os/exec"
	"runtime"
	"time"

	"github.com/takigo/takigo"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/platform"
)

// Platform is a platform.Platform on takigo. It must be used from the
// goroutine that runs the event loop.
type Platform struct {
	app      *takigo.App
	settings core.Settings
	mainBar  *MenuBar
	rootUsed bool
	names    int

	// guide is the user's guide's window, made when first shown.
	guide *guideWindow

	// glReported is set once a GL failure has been logged.
	glReported bool

	// The pointer's last position on the screen, where a popup menu
	// opens.
	pointerX, pointerY int

	// The progress window up, if one is, and what waits for it to
	// close (progress.go).
	progress *progressWindow
	held     []func()
	// windows are the windows made, whose GL frames a progress window
	// keeps copies of.
	windows []*Window
}

var _ platform.Platform = (*Platform)(nil)

// New opens the display. Settings are kept in settingsPath (see
// platform.DefaultSettingsPath); empty keeps them in memory only.
func New(settingsPath string) (*Platform, error) {
	app, err := takigo.NewApp(takigo.Title("TeleSAPR"), takigo.Size(910, 680))
	if err != nil {
		return nil, err
	}
	var s core.Settings = platform.NewMemorySettings()
	if settingsPath != "" {
		s = platform.OpenJSONSettings(settingsPath)
	}
	return &Platform{app: app, settings: s}, nil
}

// App is the takigo application.
func (p *Platform) App() *takigo.App { return p.app }

func (p *Platform) newName(prefix string) string {
	p.names++
	return fmt.Sprintf("%s%d", prefix, p.names)
}

// Settings returns the settings store.
func (p *Platform) Settings() core.Settings { return p.settings }

// Run runs the event loop until Exit.
func (p *Platform) Run() { p.app.Run() }

// Exit stops the event loop.
func (p *Platform) Exit() { p.app.Quit() }

// OpenInBrowser opens url with the desktop's handler.
func (p *Platform) OpenInBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}

// FontFiles lists the font files in the usual font directories, as
// the GTK build lists the ones fontconfig knows.
func (p *Platform) FontFiles() []string { return platform.SystemFontFiles() }

// CreateTimer returns a timer on the event loop.
func (p *Platform) CreateTimer() platform.Timer { return &Timer{p: p} }

// Timer is a single-shot timer on the event loop.
type Timer struct {
	p         *Platform
	onTimeout func()
	cancel    func() bool
	// gen counts the starts, so that a timeout held past a restart
	// doesn't fire as well as the restarted one.
	gen int
}

// SetOnTimeout sets what runs when the timer fires.
func (t *Timer) SetOnTimeout(fn func()) { t.onTimeout = fn }

// RunAfter (re)starts the timer. A timeout while a progress window is
// up waits for it to close, unless the timer is started again first.
func (t *Timer) RunAfter(ms int) {
	if t.cancel != nil {
		t.cancel()
	}
	t.gen++
	gen := t.gen
	t.cancel = t.p.app.After(time.Duration(ms)*time.Millisecond, func() {
		t.p.runOrHold(func() {
			if gen != t.gen {
				return
			}
			t.cancel = nil
			if t.onTimeout != nil {
				t.onTimeout()
			}
		})
	})
}

// RunAfterNextFrame fires after a frame's time.
func (t *Timer) RunAfterNextFrame() { t.RunAfter(1) }

// RunAfterProcessingEvents fires once pending events are handled.
func (t *Timer) RunAfterProcessingEvents() { t.RunAfter(0) }
