package main

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/telesapr/telesapr/pkg/geom"
)

// sectionPlane is the plane --plane names: a point on it and its normal.
type sectionPlane struct {
	origin, normal geom.Vector
}

// errPlaneSyntax is what parsePlane says of a plane it can't read.
var errPlaneSyntax = errors.New(`want "x=<d>", "y=<d>", "z=<d>" or six numbers "<x>,<y>,<z>,<nx>,<ny>,<nz>"`)

// parsePlane reads a plane square to an axis at a distance from the
// origin ("z=15"), or through a point with a normal (six numbers).
func parsePlane(spec string) (*sectionPlane, error) {
	if axis, d, ok := strings.Cut(spec, "="); ok {
		at, err := strconv.ParseFloat(strings.TrimSpace(d), 64)
		if err != nil {
			return nil, errPlaneSyntax
		}
		var n geom.Vector
		switch strings.ToLower(strings.TrimSpace(axis)) {
		case "x":
			n = geom.Vector{X: 1}
		case "y":
			n = geom.Vector{Y: 1}
		case "z":
			n = geom.Vector{Z: 1}
		default:
			return nil, errPlaneSyntax
		}
		return &sectionPlane{origin: n.ScaledBy(at), normal: n}, nil
	}
	fields := strings.Split(spec, ",")
	if len(fields) != 6 {
		return nil, errPlaneSyntax
	}
	var f [6]float64
	for i, s := range fields {
		v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
		if err != nil {
			return nil, errPlaneSyntax
		}
		f[i] = v
	}
	p := &sectionPlane{
		origin: geom.Vector{X: f[0], Y: f[1], Z: f[2]},
		normal: geom.Vector{X: f[3], Y: f[4], Z: f[5]},
	}
	if p.normal.Magnitude() == 0 {
		return nil, fmt.Errorf("the normal (%g, %g, %g) has no length", f[3], f[4], f[5])
	}
	return p, nil
}
