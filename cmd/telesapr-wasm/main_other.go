//go:build !(js && wasm)

// Command telesapr-wasm is the browser spike; it is built for js/wasm
// only (scripts/wasm-spike.sh).
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "telesapr-wasm runs only as js/wasm: see scripts/wasm-spike.sh")
	os.Exit(2)
}
