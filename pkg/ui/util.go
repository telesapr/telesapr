package ui

import (
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/spacesolve/spacesolve/pkg/core"
	"github.com/spacesolve/spacesolve/pkg/geom"
	"github.com/spacesolve/spacesolve/pkg/sketch"
)

func sprintf(format string, args ...any) string { return fmt.Sprintf(format, args...) }

func vec3(x, y, z float64) geom.Vector { return geom.Vector{X: x, Y: y, Z: z} }

// getMilliseconds is a monotonic clock in milliseconds (C++
// GetMilliseconds).
func getMilliseconds() int64 { return time.Since(processStart).Milliseconds() }

var processStart = time.Now()

// evalExpr evaluates an expression the user typed (C++ Expr::Eval).
func (ss *SolveSpaceUI) evalExpr(e *core.Expr) float64 {
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
