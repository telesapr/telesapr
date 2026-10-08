// Command telesapr is the TeleSAPR GUI. It can run a Go script on the
// sketch at startup; see parseArgs.
package main

import (
	"fmt"
	"log/slog"
	"os"
	"runtime"
	"strings"

	"github.com/telesapr/telesapr/pkg/platform"
	"github.com/telesapr/telesapr/pkg/platform/tkgui"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/ui"
)

// OpenGL contexts belong to a thread, and the windows draw from the
// event loop, which runs on the main goroutine: keep it on one thread.
func init() { runtime.LockOSThread() }

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				return slog.Attr{}
			}
			return a
		},
	}))
	cfg, problems := parseArgs(os.Args[1:])
	for _, p := range problems {
		logger.Warn(p)
	}
	if err := run(cfg, logger); err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}
}

// run opens the window on the platform, loads the file the command line
// names and runs its script, and serves the event loop until the
// window closes.
func run(cfg options, logger *slog.Logger) error {
	srf.SetParallelModeFromEnv()
	p, err := tkgui.New(platform.DefaultSettingsPath())
	if err != nil {
		return err
	}

	ss := ui.New(p)
	ss.RendererOverride = cfg.renderer
	// The solids are made off the interface's thread unless
	// TELESAPR_BACKGROUND is off (roadmap 17.3).
	ss.BackgroundShells = os.Getenv("TELESAPR_BACKGROUND") != "off"
	ss.Init()

	if len(cfg.files) >= 1 {
		if len(cfg.files) > 1 {
			logger.Warn("only the first file passed on the command line will be opened")
		}
		ss.Report(ss.Load(cfg.files[len(cfg.files)-1]))
	}
	if cfg.agent {
		if err := ss.AllowAgent(true); err != nil {
			logger.Warn("--agent: " + err.Error())
		}
	}
	if cfg.scriptFile != "" {
		// It runs once the event loop does, on the sketch just opened.
		console := ss.Console()
		console.SetVisible(true)
		console.RunFile(cfg.scriptFile, cfg.scriptArgs...)
	}

	p.Run()
	return nil
}

// options is what the command line asks for.
type options struct {
	files      []string
	renderer   string
	scriptFile string
	scriptArgs []string
	// agent lets AI programs connect for this run, as Script > Allow
	// AI Connections does.
	agent bool
}

// parseArgs splits the command line into the sketch files to open, the
// renderer to draw with for this run, whether AI programs may connect
// (telesapr-cli mcp), and, after --script, a Go script to run on the
// sketch with its arguments, saying what it could not read:
//
//	telesapr [--renderer opengl|software] [--agent] [sketch.tlspr] [--script script.go [argument...]]
func parseArgs(args []string) (o options, problems []string) {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--script":
			if i+1 < len(args) {
				o.scriptFile, o.scriptArgs = args[i+1], args[i+2:]
			} else {
				problems = append(problems, "--script needs the script to run")
			}
			return o, problems
		case arg == "--agent":
			o.agent = true
		case arg == "--renderer" || strings.HasPrefix(arg, "--renderer="):
			value, ok := strings.CutPrefix(arg, "--renderer=")
			if !ok {
				if i+1 >= len(args) {
					problems = append(problems, "--renderer needs opengl or software")
					continue
				}
				i++
				value = args[i]
			}
			if value != "opengl" && value != "software" {
				problems = append(problems, fmt.Sprintf("--renderer %s: it is opengl or software", value))
				continue
			}
			o.renderer = value
		default:
			o.files = append(o.files, arg)
		}
	}
	return o, problems
}
