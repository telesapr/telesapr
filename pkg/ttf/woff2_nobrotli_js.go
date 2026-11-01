//go:build js

package ttf

import "errors"

// errWOFF2Unsupported is why a WOFF2 font cannot be opened in the
// browser build: the Brotli decoder imports net/http, which costs the
// WebAssembly program about 3 MB, and a browser's fonts come as TrueType
// or OpenType files.
var errWOFF2Unsupported = errors.New("ttf: WOFF2 fonts are not supported in the browser build")

// woff2Decompress always fails in the browser build.
func woff2Decompress(uint64, []byte) ([]byte, error) { return nil, errWOFF2Unsupported }
