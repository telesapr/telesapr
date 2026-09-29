// Package cfmt formats as C's printf does (glibc), for the file writers
// whose output must match the C++ program's byte for byte.
//
// It understands the directives the C++ code uses: flags "-+ #0", a
// width and a precision (either may be "*"), the length modifiers
// hh, h, l, ll, z, j, t and L (ignored), and the conversions d, i, u,
// x, X, o, c, s, f, F, e, E, g, G and %. Go's fmt differs from C on
// NaN and infinities ("NaN" for "nan") and on %g without a precision
// (Go prints the shortest representation, C six significant digits).
package cfmt

import (
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"
)

// Sprintf is C's sprintf.
func Sprintf(format string, args ...any) string {
	var b strings.Builder
	argi := 0
	next := func() any {
		if argi >= len(args) {
			panic("cfmt: too few arguments for " + strconv.Quote(format))
		}
		a := args[argi]
		argi++
		return a
	}

	for i := 0; i < len(format); i++ {
		c := format[i]
		if c != '%' {
			b.WriteByte(c)
			continue
		}
		i++
		if i >= len(format) {
			break
		}
		if format[i] == '%' {
			b.WriteByte('%')
			continue
		}

		// Flags.
		var flags []byte
		for i < len(format) && strings.IndexByte("-+ #0", format[i]) >= 0 {
			flags = append(flags, format[i])
			i++
		}
		// Width.
		width := -1
		if i < len(format) && format[i] == '*' {
			width = int(toInt(next()))
			if width < 0 {
				flags = append(flags, '-')
				width = -width
			}
			i++
		} else {
			start := i
			for i < len(format) && format[i] >= '0' && format[i] <= '9' {
				i++
			}
			if i > start {
				width, _ = strconv.Atoi(format[start:i])
			}
		}
		// Precision.
		prec := -1
		if i < len(format) && format[i] == '.' {
			i++
			if i < len(format) && format[i] == '*' {
				prec = int(toInt(next()))
				i++
			} else {
				start := i
				for i < len(format) && format[i] >= '0' && format[i] <= '9' {
					i++
				}
				prec, _ = strconv.Atoi(format[start:i])
			}
		}
		// Length modifiers.
		for i < len(format) && strings.IndexByte("hlLqjzt", format[i]) >= 0 {
			i++
		}
		if i >= len(format) {
			break
		}
		verb := format[i]
		b.WriteString(convert(verb, flags, width, prec, next()))
	}
	return b.String()
}

func has(flags []byte, f byte) bool {
	for _, c := range flags {
		if c == f {
			return true
		}
	}
	return false
}

// goSpec rebuilds the directive for Go's fmt.
func goSpec(flags []byte, width, prec int, verb byte) string {
	var s strings.Builder
	s.WriteByte('%')
	s.Write(flags)
	if width >= 0 {
		s.WriteString(strconv.Itoa(width))
	}
	if prec >= 0 {
		s.WriteByte('.')
		s.WriteString(strconv.Itoa(prec))
	}
	s.WriteByte(verb)
	return s.String()
}

// pad pads s to width as C does, without zero padding.
func pad(s string, flags []byte, width int) string {
	if width < 0 || len(s) >= width {
		return s
	}
	fill := strings.Repeat(" ", width-len(s))
	if has(flags, '-') {
		return s + fill
	}
	return fill + s
}

func convert(verb byte, flags []byte, width, prec int, arg any) string {
	switch verb {
	case 'd', 'i':
		v := toInt(arg)
		return fmt.Sprintf(goSpec(flags, width, prec, 'd'), v)
	case 'u':
		v := toUint(arg)
		return fmt.Sprintf(goSpec(flags, width, prec, 'd'), v)
	case 'x', 'X', 'o':
		// C's precision is a minimum digit count, as Go's is.
		return fmt.Sprintf(goSpec(flags, width, prec, verb), toUint(arg))
	case 'c':
		return pad(string([]byte{byte(toInt(arg))}), flags, width)
	case 's':
		s := fmt.Sprint(arg)
		if prec >= 0 && len(s) > prec {
			s = s[:prec]
		}
		return pad(s, flags, width)
	case 'f', 'F', 'e', 'E', 'g', 'G':
		v := toFloat(arg)
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return special(v, verb, flags, width)
		}
		if prec < 0 {
			prec = 6
		}
		if (verb == 'g' || verb == 'G') && prec == 0 {
			prec = 1
		}
		if verb == 'F' {
			verb = 'f'
		}
		return fmt.Sprintf(goSpec(flags, width, prec, verb), v)
	}
	panic(fmt.Sprintf("cfmt: unsupported conversion %%%c", verb))
}

// special is how glibc writes NaN and infinities.
func special(v float64, verb byte, flags []byte, width int) string {
	var s string
	if math.IsNaN(v) {
		s = "nan"
	} else {
		s = "inf"
	}
	switch {
	case math.Signbit(v):
		s = "-" + s
	case has(flags, '+'):
		s = "+" + s
	case has(flags, ' '):
		s = " " + s
	}
	if verb == 'F' || verb == 'E' || verb == 'G' {
		s = strings.ToUpper(s)
	}
	return pad(s, flags, width)
}

func toInt(a any) int64 {
	v := reflect.ValueOf(a)
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int()
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return int64(v.Uint())
	case reflect.Bool:
		if v.Bool() {
			return 1
		}
		return 0
	}
	panic(fmt.Sprintf("cfmt: %T is not an integer", a))
}

func toUint(a any) uint64 {
	v := reflect.ValueOf(a)
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		// C converts to unsigned int, of the argument's width.
		switch v.Kind() {
		case reflect.Int64:
			return uint64(v.Int())
		default:
			return uint64(uint32(v.Int()))
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return v.Uint()
	}
	panic(fmt.Sprintf("cfmt: %T is not an integer", a))
}

func toFloat(a any) float64 {
	switch v := a.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	}
	panic(fmt.Sprintf("cfmt: %T is not a float", a))
}
