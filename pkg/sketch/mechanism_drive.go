package sketch

import (
	"maps"
	"math"
	"strconv"

	"github.com/telesapr/telesapr/pkg/core"
)

// pullDrive is a drive of a mechanism a pull may move: a parameter that
// is all of some dimension's expression (a driven hinge's angle, a
// driven concentric mate's travel), with a number for its own
// expression, so that a pull can write its new value in. by is what a
// unit of it moves the unknowns hs by, read by a central difference as
// the freedom count reads it (motions); bounds are the minimums and
// maximums beside the dimensions it drives, which stop the pull.
type pullDrive struct {
	name   string
	at     float64
	by     []float64
	dims   []*core.Constraint
	bounds []driveBound
	fs     *freedomSetting
}

// driveBound is a bound beside dimension dim, which a drive's value
// must keep dim within.
type driveBound struct {
	dim   *core.Constraint
	bound *core.Constraint
}

// pullDrives are the drives of the block a pull can move, at the
// params' values now, with what each moves unknowns hs by. A drive
// whose difference doesn't solve, or that moves nothing, is left out.
// The params and the dimensions' values are put back after.
func (sk *Sketch) pullDrives(block []core.HGroup, hs []core.HParam) []pullDrive {
	fs := sk.newFreedomSetting(block)
	defer fs.restore(sk)
	var out []pullDrive
	for _, name := range fs.names {
		if !sk.writableDrive(name) {
			continue
		}
		base := fs.vars[name]
		var ends [2][]float64
		ok := true
		for k, s := range []float64{-1, 1} {
			fs.setDrive(sk, name, base+s*driveDelta)
			ok = ok && sk.solveFreedom(fs)
			ends[k] = make([]float64, len(hs))
			for j, h := range hs {
				ends[k][j] = sk.MustParam(h).Val
			}
			fs.restoreParams(sk)
		}
		fs.setDrive(sk, name, base)
		if !ok {
			continue
		}
		by := make([]float64, len(hs))
		moves := false
		for j := range hs {
			by[j] = (ends[1][j] - ends[0][j]) / (2 * driveDelta)
			moves = moves || math.Abs(by[j]) > 1e-9
		}
		if !moves {
			continue
		}
		d := pullDrive{name: name, at: base, by: by, dims: fs.drives[name], fs: fs}
		for ci := range sk.Constraints {
			b := &sk.Constraints[ci]
			if !fs.bounds[b.H] || b.Limit == core.LimitNone {
				continue
			}
			for _, c := range d.dims {
				if sameMeasure(b, c) {
					d.bounds = append(d.bounds, driveBound{dim: c, bound: b})
				}
			}
		}
		out = append(out, d)
	}
	return out
}

// writableDrive reports whether parameter name's expression is a
// number, which a pull may write a new number over; a parameter worked
// out from others is not the pull's to change.
func (sk *Sketch) writableDrive(name string) bool {
	p := sk.Parameter(name)
	if p == nil {
		return false
	}
	e, err := core.NewFromString(p.Expr)
	return err == nil && len(e.Variables()) == 0
}

// sameMeasure reports whether constraints a and b measure the same
// thing, as a bound added beside a dimension (AddBoundOf) does.
func sameMeasure(a, b *core.Constraint) bool {
	return a.Type == b.Type && a.Group == b.Group && a.Workplane == b.Workplane &&
		a.PtA == b.PtA && a.PtB == b.PtB && a.EntityA == b.EntityA && a.EntityB == b.EntityB &&
		a.EntityC == b.EntityC && a.EntityD == b.EntityD
}

// within reports whether the drive at value v keeps each dimension it
// drives within the bounds beside it. A bound on a negative distance is
// on its size, as core.Constraint.Crossed takes it.
func (d *pullDrive) within(sk *Sketch, v float64) bool {
	vars := maps.Clone(d.fs.vars)
	vars[d.name] = v
	for _, db := range d.bounds {
		dv, err := evalNamed(db.dim.Expr, vars)
		if err != nil {
			continue
		}
		if !signedValue(db.dim.Type) && db.dim.Type != core.ConstraintAxisAngle {
			dv = math.Abs(dv)
		}
		if IsLengthValue(db.dim.Type) {
			dv *= sk.Units.MmPer()
		}
		b := db.bound.ValA
		if b < 0 && db.dim.Type != core.ConstraintAxisAngle {
			dv, b = math.Abs(dv), math.Abs(b)
		}
		const eps = 1e-9
		if db.bound.Limit == core.LimitMin && dv < b-eps || db.bound.Limit == core.LimitMax && dv > b+eps {
			return false
		}
	}
	return true
}

// room is how much of a change dz the drive can take within its
// bounds, from 0 to 1: all of it where it ends within them, or where it
// starts outside them already (the solve then fails, as for a value
// typed past a bound); else the part of it up to the bound, found by
// halving, since a dimension's expression need not be linear in it.
func (d *pullDrive) room(sk *Sketch, dz float64) float64 {
	if d.within(sk, d.at+dz) || !d.within(sk, d.at) {
		return 1
	}
	lo, hi := 0.0, 1.0
	for range 60 {
		mid := (lo + hi) / 2
		if d.within(sk, d.at+mid*dz) {
			lo = mid
		} else {
			hi = mid
		}
	}
	return lo
}

// writeDrive gives the drive's parameter the value v, rounded to a
// millionth so that the parameter table shows a number one can read,
// and the dimensions it drives their new values.
func (sk *Sketch) writeDrive(name string, v float64) {
	v = math.Round(v*1e6) / 1e6
	if v == 0 {
		v = 0 // no "-0"
	}
	// The expression is a number: it can't fail.
	_ = sk.SetParameterExpr(name, strconv.FormatFloat(v, 'g', -1, 64))
	sk.EvaluateParameters()
}
