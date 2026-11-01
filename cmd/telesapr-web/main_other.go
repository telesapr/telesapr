//go:build !(js && wasm)

// Command telesapr-web is the browser version of the editor; it is
// built for js/wasm only (scripts/wasm-build.sh, make wasm).
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "telesapr-web runs only as js/wasm: see scripts/wasm-build.sh")
	os.Exit(2)
}
