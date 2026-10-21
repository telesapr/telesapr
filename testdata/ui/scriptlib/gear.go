//go:build telesapr

// The script of the ext_script_library scenario: it imports gears from
// the script library.
package main

import (
	"fmt"

	"github.com/telesapr/telesapr/pkg/script/api"

	"gears"
)

func main() {
	fmt.Println("radius", gears.Disc(api.Current(), 12).Radius())
}
