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

// setParallelMode takes SPACESOLVE_PARALLEL: "exact" (the default) has
// the solids generated on all the processors where that gives the bits
// of one; "fast" on all of them throughout, for other last bits; "off"
// on one (see srf.ParallelMode).
func setParallelMode() {
	switch v := os.Getenv("SPACESOLVE_PARALLEL"); v {
	case "", "exact":
	case "fast":
		srf.SetParallelMode(srf.ParallelFast)
	case "off":
		srf.SetParallelMode(srf.ParallelOff)
	default:
		log.Printf("SPACESOLVE_PARALLEL is '%s'; it can be exact, fast or off.", v)
	}
}

func main() {
	log.SetFlags(0)
	setParallelMode()
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
