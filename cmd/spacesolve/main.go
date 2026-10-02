// Command spacesolve is the SolveSpace GUI (C++ platform/entrygui.cpp).
// Unlike C++, it can run a Go script on the sketch at startup; see
// parseArgs.
package main

import (
	"log"
	"os"

	"github.com/spacesolve/spacesolve/pkg/core"
	"github.com/spacesolve/spacesolve/pkg/platform"
	"github.com/spacesolve/spacesolve/pkg/platform/tkgui"
	"github.com/spacesolve/spacesolve/pkg/srf"
	"github.com/spacesolve/spacesolve/pkg/ui"
)

func main() {
	log.SetFlags(0)
	srf.SetParallelModeFromEnv()
	p, err := tkgui.New(platform.DefaultSettingsPath())
	if err != nil {
		log.Fatal(err)
	}
	core.SetSettings(p.GetSettings())

	ss := ui.New(p)
	ss.Init()

	files, scriptFile, scriptArgs := parseArgs(os.Args[1:])
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

// parseArgs splits the command line into the sketch files to open and,
// after --script, a Go script to run on the sketch with its arguments:
//
//	spacesolve [sketch.spcs] [--script script.go [argument...]]
func parseArgs(args []string) (files []string, scriptFile string, scriptArgs []string) {
	for i, arg := range args {
		if arg == "--script" {
			if i+1 < len(args) {
				scriptFile, scriptArgs = args[i+1], args[i+2:]
			} else {
				log.Print("--script needs the script to run.")
			}
			break
		}
		files = append(files, arg)
	}
	return files, scriptFile, scriptArgs
}
