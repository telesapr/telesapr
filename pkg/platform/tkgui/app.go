// Package tkgui is the platform on the takigo Tk port: windows whose
// content is a frame the application renders in software, Tk menus,
// timers and dialogs. It ports platform/guigtk.cpp.
package tkgui

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/msorc/takigo"

	"github.com/spacesolve/spacesolve/pkg/core"
	"github.com/spacesolve/spacesolve/pkg/platform"
)

// Platform is a platform.Platform on takigo. It must be used from the
// goroutine that runs the event loop.
type Platform struct {
	app      *takigo.App
	settings core.Settings
	mainBar  *MenuBar
	rootUsed bool
	names    int

	// The pointer's last position on the screen, where a popup menu
	// opens.
	pointerX, pointerY int
}

var _ platform.Platform = (*Platform)(nil)

// New opens the display. Settings are kept in settingsPath (see
// platform.DefaultSettingsPath); empty keeps them in memory only.
func New(settingsPath string) (*Platform, error) {
	app, err := takigo.NewApp(takigo.Title("SolveSpace"), takigo.Size(910, 680))
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

// GetSettings returns the settings store.
func (p *Platform) GetSettings() core.Settings { return p.settings }

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

// GetFontFiles lists the font files in the usual font directories, as
// the GTK build lists the ones fontconfig knows.
func (p *Platform) GetFontFiles() []string {
	var dirs []string
	switch runtime.GOOS {
	case "windows":
		dirs = []string{filepath.Join(os.Getenv("WINDIR"), "Fonts")}
	case "darwin":
		dirs = []string{"/System/Library/Fonts", "/Library/Fonts"}
		if home, err := os.UserHomeDir(); err == nil {
			dirs = append(dirs, filepath.Join(home, "Library", "Fonts"))
		}
	default:
		dirs = []string{"/usr/share/fonts", "/usr/local/share/fonts"}
		if dataHome := os.Getenv("XDG_DATA_HOME"); dataHome != "" {
			dirs = append(dirs, filepath.Join(dataHome, "fonts"))
		}
		if home, err := os.UserHomeDir(); err == nil {
			dirs = append(dirs, filepath.Join(home, ".local", "share", "fonts"), filepath.Join(home, ".fonts"))
		}
	}
	var files []string
	for _, dir := range dirs {
		_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			switch strings.ToLower(filepath.Ext(path)) {
			case ".ttf", ".otf", ".ttc", ".pfa", ".pfb", ".woff", ".woff2":
				files = append(files, path)
			}
			return nil
		})
	}
	return files
}

// CreateTimer returns a timer on the event loop (C++ TimerImplGtk).
func (p *Platform) CreateTimer() platform.Timer { return &Timer{p: p} }

// Timer is a single-shot timer on the event loop.
type Timer struct {
	p         *Platform
	onTimeout func()
	cancel    func() bool
}

// SetOnTimeout sets what runs when the timer fires.
func (t *Timer) SetOnTimeout(fn func()) { t.onTimeout = fn }

// RunAfter (re)starts the timer.
func (t *Timer) RunAfter(ms int) {
	if t.cancel != nil {
		t.cancel()
	}
	t.cancel = t.p.app.After(time.Duration(ms)*time.Millisecond, func() {
		t.cancel = nil
		if t.onTimeout != nil {
			t.onTimeout()
		}
	})
}

// RunAfterNextFrame fires after a frame's time.
func (t *Timer) RunAfterNextFrame() { t.RunAfter(1) }

// RunAfterProcessingEvents fires once pending events are handled.
func (t *Timer) RunAfterProcessingEvents() { t.RunAfter(0) }
