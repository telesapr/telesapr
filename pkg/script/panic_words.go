package script

import (
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"runtime"
	"strings"
)

// panicError is what a script's panic carried, as an error. A panic
// of Go's runtime, of reflect or of the interpreter is put in plain
// words: the interpreter runs a script's indexing and dereferencing
// through reflect, whose wording ("reflect: slice index out of range")
// says how the interpreter works rather than what the script did. An
// error of any other package, such as one of package api's calls that
// can't do what is asked, is the script's message and kept as it is.
func panicError(value any) error {
	switch v := value.(type) {
	case string:
		if plain, ok := plainPanic(v); ok {
			return errors.New(plain)
		}
		return fmt.Errorf("panic: %s", v)
	case runtime.Error:
		msg := strings.TrimPrefix(v.Error(), "runtime error: ")
		if plain, ok := plainPanic(msg); ok {
			return errors.New(plain)
		}
		return errors.New(msg)
	case *reflect.ValueError:
		if v.Kind == reflect.Invalid {
			// What the interpreter's reflect makes of a nil pointer or
			// a nil interface it looks into.
			return errors.New("nil pointer or nil interface")
		}
		return v
	case error:
		if t := reflect.TypeOf(v); t.Kind() == reflect.Pointer && t.Elem().PkgPath() == interpPath {
			// The interpreter puts the place of the check in its own
			// wrapping of the source before the message.
			msg := positionPrefix.ReplaceAllString(v.Error(), "")
			if plain, ok := plainPanic(msg); ok {
				return errors.New(plain)
			}
			return errors.New(msg)
		}
		return v
	}
	return fmt.Errorf("panic: %v", value)
}

// interpPath is the package of the interpreter's own errors.
const interpPath = "github.com/traefik/yaegi/interp"

// positionPrefix is a "line:col: " a message starts with.
var positionPrefix = regexp.MustCompile(`^\d+:\d+: `)

// panicWords are the panics put in other words, by what they start
// with; the rest of the message is kept after the new words.
var panicWords = []struct{ from, to string }{
	{"reflect: slice index out of range", "index out of range"},
	{"reflect: array index out of range", "index out of range"},
	{"reflect: string index out of range", "index out of range"},
	{"reflect.Value.Slice: slice index out of bounds", "slice bounds out of range"},
	{"reflect.Value.Slice3: slice index out of bounds", "slice bounds out of range"},
	{"invalid memory address or nil pointer dereference", "nil pointer"},
	{"integer divide by zero", "division by zero"},
	{"assignment to entry in nil map", "assignment to a nil map"},
	{"interface conversion: ", ""},
	// Index and slice errors of the runtime say the numbers, which are
	// kept: "index out of range [3] with length 0".
	{"index out of range", "index out of range"},
	{"slice bounds out of range", "slice bounds out of range"},
}

// plainPanic is msg, a panic of the runtime, reflect or the
// interpreter, in plain words, and whether it was one of those.
func plainPanic(msg string) (string, bool) {
	for _, w := range panicWords {
		if rest, ok := strings.CutPrefix(msg, w.from); ok {
			return w.to + rest, true
		}
	}
	return msg, false
}
