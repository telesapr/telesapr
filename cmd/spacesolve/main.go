// Command spacesolve is the SolveSpace GUI (C++ platform/entrygui.cpp).
// Unlike C++, it can run a Go script on the sketch at startup; see
// parseArgs.
package main

import (
	"log"
	"os"
	"runtime"
	"strings"

	"github.com/spacesolve/spacesolve/pkg/core"
	"github.com/spacesolve/spacesolve/pkg/platform"
	"github.com/spacesolve/spacesolve/pkg/platform/tkgui"
	"github.com/spacesolve/spacesolve/pkg/srf"
	"github.com/spacesolve/spacesolve/pkg/ui"
)

// OpenGL contexts belong to a thread, and the windows draw from the
// event loop, which runs on the main goroutine: keep it on one thread.
func init() { runtime.LockOSThread() }

func main() {
	log.SetFlags(0)
	srf.SetParallelModeFromEnv()
	p, err := tkgui.New(platform.DefaultSettingsPath())
	if err != nil {
		log.Fatal(err)
	}
	core.SetSettings(p.GetSettings())

	opts := parseArgs(os.Args[1:])
	files, scriptFile, scriptArgs := opts.files, opts.scriptFile, opts.scriptArgs

	ss := ui.New(p)
	ss.RendererOverride = opts.renderer
	ss.Init()

	if len(files) >= 1 {
		if len(files) > 1 {
			log.Print("Only the first file passed on command line will be opened.")
		}
		ss.Load(files[len(files)-1])
	}
	if scriptFile != "" {
		// It runs once the event loop does, on the sketch just opened.
		console := ss.Console()
		console.SetVisible(true)
		console.RunFile(scriptFile, scriptArgs...)
	}

	p.Run()
}

// options is what the command line asks for.
type options struct {
	files      []string
	renderer   string
	scriptFile string
	scriptArgs []string
}

// parseArgs splits the command line into the sketch files to open, the
// renderer to draw with for this run, and, after --script, a Go script
// to run on the sketch with its arguments:
//
//	spacesolve [--renderer opengl|software] [sketch.spcs] [--script script.go [argument...]]
func parseArgs(args []string) (o options) {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--script":
			if i+1 < len(args) {
				o.scriptFile, o.scriptArgs = args[i+1], args[i+2:]
			} else {
				log.Print("--script needs the script to run.")
			}
			return o
		case arg == "--renderer" || strings.HasPrefix(arg, "--renderer="):
			value, ok := strings.CutPrefix(arg, "--renderer=")
			if !ok {
				if i+1 >= len(args) {
					log.Print("--renderer needs opengl or software.")
					continue
				}
				i++
				value = args[i]
			}
			if value != "opengl" && value != "software" {
				log.Printf("--renderer %s: it is opengl or software.", value)
				continue
			}
			o.renderer = value
		default:
			o.files = append(o.files, arg)
		}
	}
	return o
}
