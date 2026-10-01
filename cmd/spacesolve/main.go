// Command spacesolve is the SolveSpace GUI (C++ platform/entrygui.cpp).
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

	if args := os.Args[1:]; len(args) >= 1 {
		if len(args) > 1 {
			log.Print("Only the first file passed on command line will be opened.")
		}
		ss.Load(args[len(args)-1])
	}

	p.Run()
}
