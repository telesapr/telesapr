// Package symbols holds what the interpreter can import: the packages
// of this program a script may use, and part of the standard library.
// The tables are generated; run "go generate" here, or "make generate",
// after changing what one of those packages exports.
package symbols

import "reflect"

//go:generate go tool yaegi extract -name symbols github.com/spacesolve/spacesolve/pkg/script/api github.com/spacesolve/spacesolve/pkg/check github.com/spacesolve/spacesolve/pkg/geom github.com/spacesolve/spacesolve/pkg/core github.com/spacesolve/spacesolve/pkg/sketch github.com/spacesolve/spacesolve/pkg/srf github.com/spacesolve/spacesolve/pkg/persist/spcs github.com/spacesolve/spacesolve/pkg/persist/convert
//go:generate go tool yaegi extract -name symbols bufio bytes cmp context encoding/csv errors flag fmt io io/fs math math/bits math/cmplx math/rand os path path/filepath regexp sort strconv strings sync text/tabwriter time unicode unicode/utf8

//
// Go 1.27's json.Options has a method no other package can name.
//go:generate go tool yaegi extract -name symbols -exclude ^Options$ encoding/json

//
// What help() says about all of those, from their sources.
//go:generate go run ./gendoc -full github.com/spacesolve/spacesolve/pkg/script/api github.com/spacesolve/spacesolve/pkg/script/api github.com/spacesolve/spacesolve/pkg/check github.com/spacesolve/spacesolve/pkg/geom github.com/spacesolve/spacesolve/pkg/core github.com/spacesolve/spacesolve/pkg/sketch github.com/spacesolve/spacesolve/pkg/srf github.com/spacesolve/spacesolve/pkg/persist/spcs github.com/spacesolve/spacesolve/pkg/persist/convert bufio bytes cmp context encoding/csv encoding/json errors flag fmt io io/fs math math/bits math/cmplx math/rand os path path/filepath regexp sort strconv strings sync text/tabwriter time unicode unicode/utf8

// Symbols maps an import path, with the package's name after it, to
// the package's exported symbols.
var Symbols = map[string]map[string]reflect.Value{}
