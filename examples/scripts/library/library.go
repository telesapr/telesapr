//go:build telesapr

// A gear blank drawn by a package of a script library.
//
// Without a window:
//
//	telesapr-cli run --lib examples/scripts/library/lib examples/scripts/library/library.go
//
// In the application: set "script library folder" in the
// configuration screen to examples/scripts/library/lib, then
// Script > Run File.
//
// The package gears is in the library, not in TeleSAPR: an import the
// program has no package for is looked for in the library folder. The
// go tool can't find it there, so this file carries the build tag
// "telesapr", which the scripts' interpreter has and go build and
// go vet don't. It writes gear.tlspr where it is run, unless it is
// given "-".
package main

import (
	"fmt"
	"os"

	"github.com/telesapr/telesapr/pkg/script/api"

	"gears"
)

func main() {
	doc := api.Current()
	gear := gears.Blank(doc, 24, 2, 8, 6, "gear")
	fmt.Printf("%s: volume %.0f mm³\n", gear, gear.Analyze().Volume())
	if len(os.Args) > 1 && os.Args[1] == "-" {
		return
	}
	if err := doc.File.Save("gear.tlspr"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}
