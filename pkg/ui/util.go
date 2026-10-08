package ui

import (
	"math"
	"strconv"
	"time"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/sketch"
)

func vec3(x, y, z float64) geom.Vector { return geom.Vector{X: x, Y: y, Z: z} }

// getMilliseconds is a monotonic clock in milliseconds.
func getMilliseconds() int64 { return time.Since(processStart).Milliseconds() }

var processStart = time.Now()

// evalExpr evaluates an expression the user typed.
func (ss *Application) evalExpr(e *core.Expr) float64 {
	return e.Eval(sketch.NewRuntime(ss.SK), nil)
}

// cSpecial is how C's printf writes NaN and infinities, which Go writes
// as "NaN" and "+Inf".
func cSpecial(v float64) (string, bool) {
	switch {
	case math.IsNaN(v):
		if math.Signbit(v) {
			return "-nan", true
		}
		return "nan", true
	case math.IsInf(v, 1):
		return "inf", true
	case math.IsInf(v, -1):
		return "-inf", true
	}
	return "", false
}

// cFixed is C's "%.*f".
func cFixed(prec int, v float64) string {
	if s, ok := cSpecial(v); ok {
		return s
	}
	return strconv.FormatFloat(v, 'f', prec, 64)
}
