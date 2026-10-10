package export

import (
	"math"

	"github.com/telesapr/telesapr/pkg/geom"
)

// The alias lists of a STEP file grow with every point and edge of the
// solid, and were searched from the start for each one written: the
// export of a 16 by 16 step and repeat spent longer on that than on
// regenerating the model. The indexes below find what those searches
// found, the first entry of a list that matches, without the search.

// aliasIndex finds, for an entity number, the first entry of an alias
// list whose aliases have it.
type aliasIndex map[int]int

// add records that entry's aliases have number.
func (x *aliasIndex) add(number, entry int) {
	if *x == nil {
		*x = make(aliasIndex)
	}
	if e, ok := (*x)[number]; !ok || entry < e {
		(*x)[number] = entry
	}
}

// find is the first entry whose aliases have number.
func (x aliasIndex) find(number int) (entry int, ok bool) {
	entry, ok = x[number]
	return entry, ok
}

// pointCells finds the points of the point alias list near a point:
// a grid of cells as wide as the tolerance points are merged at, so
// that a point within it of another is in the same cell or one of the
// 26 around it.
type pointCells struct {
	cells map[[3]int64][]int
	far   []int // entries too far out for a cell, searched as before
}

// pointCellReach is the largest coordinate, in cells, that pointCells
// files by cell; past it a coordinate's cell would not fit an int64.
const pointCellReach = 1 << 52

// cellOf is p's cell, or false when p has no cell.
func cellOf(p geom.Vector) ([3]int64, bool) {
	var c [3]int64
	for k, x := range [3]float64{p.X, p.Y, p.Z} {
		f := math.Floor(x / stepPrecision)
		if !(math.Abs(f) < pointCellReach) { // NaN too
			return c, false
		}
		c[k] = int64(f)
	}
	return c, true
}

// add files entry, whose point is p.
func (pc *pointCells) add(p geom.Vector, entry int) {
	c, ok := cellOf(p)
	if !ok {
		pc.far = append(pc.far, entry)
		return
	}
	if pc.cells == nil {
		pc.cells = make(map[[3]int64][]int)
	}
	pc.cells[c] = append(pc.cells[c], entry)
}

// first is the first entry of points whose point EqualsTol v at
// stepPrecision, or -1.
func (pc *pointCells) first(points []stepPointAlias, v geom.Vector) int {
	best := -1
	try := func(entries []int) {
		for _, e := range entries {
			if (best < 0 || e < best) && points[e].v.EqualsTol(v, stepPrecision) {
				best = e
			}
		}
	}
	c, ok := cellOf(v)
	if !ok {
		// v is too far out for a cell: search them all.
		for e := range points {
			if points[e].v.EqualsTol(v, stepPrecision) {
				return e
			}
		}
		return -1
	}
	try(pc.far)
	for dx := int64(-1); dx <= 1; dx++ {
		for dy := int64(-1); dy <= 1; dy++ {
			for dz := int64(-1); dz <= 1; dz++ {
				try(pc.cells[[3]int64{c[0] + dx, c[1] + dy, c[2] + dz}])
			}
		}
	}
	return best
}

// resetAliases empties the alias lists and their indexes, for the next
// part of an assembly.
func (w *stepFileWriter) resetAliases() {
	w.pointAliases, w.edgeCurveAliases, w.curveAliases, w.orientedEdgeAliases = nil, nil, nil, nil
	w.pointByNumber, w.vertexByNumber, w.orientedEdgeByNumber = nil, nil, nil
	w.pointCells = pointCells{}
}
