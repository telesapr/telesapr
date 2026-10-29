package srf

import (
	"errors"
	"fmt"
	"math"
	"sync"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/internal/sweep"
)

// pathPart is a piece of a test path in the xz plane, turned about y:
// a line of length l, or an arc of radius r turning by turn, after a
// corner turning by corner. Turns are about +y by the right-hand rule.
type pathPart struct {
	l, r, turn, corner float64
	arc                bool
}

// piece2 is a piece of a path in the xz plane, swept by a profile
// reaching w either side of it: a line from p along the unit d of
// length l, or an arc about c of radius r from r0, the start's radial
// vector, turning by an about +y when sg is 1, -y when -1.
type piece2 struct {
	arc  bool
	p, d [2]float64
	l    float64
	c    [2]float64
	r0   [2]float64
	r    float64
	an   float64
	sg   float64
}

// holds says whether x lies in the piece swept by w, carried on from
// s0 to s1 of its length or turn (0 to l, or 0 to its angle, unless
// carried on).
func (pc piece2) holds(x [2]float64, w, s0, s1 float64) bool {
	if !pc.arc {
		v := [2]float64{x[0] - pc.p[0], x[1] - pc.p[1]}
		s := v[0]*pc.d[0] + v[1]*pc.d[1]
		t := v[0]*pc.d[1] - v[1]*pc.d[0]
		return s >= s0 && s <= s1 && math.Abs(t) <= w
	}
	v := [2]float64{x[0] - pc.c[0], x[1] - pc.c[1]}
	// A turn about +y is clockwise seen with x right and z up. From a
	// quarter turn before the start: arcs of the tests turn by half a
	// turn at most.
	a := -pc.sg * math.Atan2(pc.r0[0]*v[1]-pc.r0[1]*v[0], pc.r0[0]*v[0]+pc.r0[1]*v[1])
	a = math.Mod(a+math.Pi/2+4*math.Pi, 2*math.Pi) - math.Pi/2
	return math.Abs(math.Hypot(v[0], v[1])-pc.r) <= w && a >= s0 && a <= s1
}

// testPath is the steps along parts from the origin, setting out along
// +z, and the same pieces in the xz plane.
func testPath(parts []pathPart) ([]SweepStep, []piece2) {
	var steps []SweepStep
	var pieces []piece2
	y := geom.Vector{Y: 1}
	p, alpha := [2]float64{}, 0.0
	at := func() geom.Vector { return geom.Vector{X: p[0], Z: p[1]} }
	for _, pt := range parts {
		if pt.corner != 0 {
			steps = append(steps, SweepStep{Corner: true, Center: at(), Axis: y.ScaledBy(math.Copysign(1, pt.corner)),
				Angle: math.Abs(pt.corner)})
			alpha += pt.corner
		}
		d := [2]float64{math.Sin(alpha), math.Cos(alpha)}
		if !pt.arc {
			pieces = append(pieces, piece2{p: p, d: d, l: pt.l})
			steps = append(steps, SweepStep{Along: geom.Vector{X: d[0], Z: d[1]}.ScaledBy(pt.l)})
			p = [2]float64{p[0] + pt.l*d[0], p[1] + pt.l*d[1]}
			continue
		}
		sg := math.Copysign(1, pt.turn)
		// The centre is to the side the turn goes: about +y, (cos, -sin)
		// of the way.
		c := [2]float64{p[0] + sg*pt.r*math.Cos(alpha), p[1] - sg*pt.r*math.Sin(alpha)}
		r0 := [2]float64{p[0] - c[0], p[1] - c[1]}
		pieces = append(pieces, piece2{arc: true, c: c, r0: r0, r: pt.r, an: math.Abs(pt.turn), sg: sg})
		steps = append(steps, SweepStep{Arc: true, Center: geom.Vector{X: c[0], Z: c[1]}, Axis: y.ScaledBy(sg),
			Angle: math.Abs(pt.turn)})
		end := geom.Vector{X: p[0], Z: p[1]}.RotatedAbout(geom.Vector{X: c[0], Z: c[1]}, y, pt.turn)
		p = [2]float64{end.X, end.Z}
		alpha += pt.turn
	}
	return steps, pieces
}

// joinedArea is the area, integrated by rows across a square about the
// pieces, of the region the profile
// reaching w either side sweeps along the pieces, each meeting the next
// at a corner: each piece, and at each corner where the two pieces
// carried on past it overlap, which fills its outside.
func joinedArea(pieces []piece2, w float64, rows int) float64 {
	size := func(pc piece2) float64 {
		if pc.arc {
			return pc.an
		}
		return pc.l
	}
	carry := func(pc piece2) float64 {
		if pc.arc {
			return math.Pi / 2
		}
		return 2 * pc.l
	}
	held := func(x [2]float64) bool {
		for k, pc := range pieces {
			if pc.holds(x, w, 0, size(pc)) {
				return true
			}
			if k > 0 {
				a := pieces[k-1]
				if a.holds(x, w, size(a), size(a)+carry(a)) && pc.holds(x, w, -carry(pc), 0) {
					return true
				}
			}
		}
		return false
	}
	// Rows run at a slant, so that no cap lies along them: a row's
	// length jumps across one.
	sn, cs := math.Sincos(0.37)
	// A square about the pieces, turned with the rows.
	bmin, bmax := [2]float64{math.Inf(1), math.Inf(1)}, [2]float64{math.Inf(-1), math.Inf(-1)}
	grow := func(x [2]float64, r float64) {
		for i := range 2 {
			bmin[i], bmax[i] = math.Min(bmin[i], x[i]-r), math.Max(bmax[i], x[i]+r)
		}
	}
	for _, pc := range pieces {
		if pc.arc {
			grow(pc.c, pc.r+w)
		} else {
			grow(pc.p, w)
			grow([2]float64{pc.p[0] + pc.l*pc.d[0], pc.p[1] + pc.l*pc.d[1]}, w)
		}
	}
	mid := [2]float64{(bmin[0] + bmax[0]) / 2, (bmin[1] + bmax[1]) / 2}
	half := math.Max(bmax[0]-bmin[0], bmax[1]-bmin[1])/2*math.Sqrt2 + 1
	inside := func(y [2]float64) bool {
		return held([2]float64{mid[0] + cs*y[0] - sn*y[1], mid[1] + sn*y[0] + cs*y[1]})
	}
	lo, hi := -half, half
	cols := 2 * rows
	h := (hi - lo) / float64(rows)
	dx := (hi - lo) / float64(cols)
	area := 0.0
	for i := range rows {
		z := lo + (float64(i)+0.5)*h
		prev := inside([2]float64{lo, z})
		for j := 1; j <= cols; j++ {
			x := lo + float64(j)*dx
			in := inside([2]float64{x, z})
			if in == prev {
				continue
			}
			// The crossing, by halving.
			l, r := x-dx, x
			for range 40 {
				m := (l + r) / 2
				if inside([2]float64{m, z}) == prev {
					l = m
				} else {
					r = m
				}
			}
			if in {
				area -= l * h
			} else {
				area += l * h
			}
			prev = in
		}
	}
	return area
}

func deg(a float64) float64 { return a * math.Pi / 180 }

// Paths with corners beside arcs: an arc then a line, a line then an
// arc, an arc into an arc turning the other way, and an arc between two
// lines, each corner turning either way.
func jointPaths() map[string][]pathPart {
	paths := map[string][]pathPart{}
	for _, phi := range []float64{30, -30, 60, -60, 90, -90} {
		name := func(s string) string { return fmt.Sprint(s, " ", phi) }
		paths[name("arc line")] = []pathPart{{arc: true, r: 10, turn: deg(60)}, {l: 8, corner: deg(phi)}}
		paths[name("line arc")] = []pathPart{{l: 8}, {arc: true, r: 10, turn: deg(60), corner: deg(phi)}}
	}
	paths["arc arc"] = []pathPart{{arc: true, r: 10, turn: deg(45)}, {arc: true, r: 6, turn: deg(-60), corner: deg(40)}}
	paths["line arc line"] = []pathPart{{l: 6}, {arc: true, r: 8, turn: deg(70), corner: deg(-35)}, {l: 6, corner: deg(50)}}
	return paths
}

// A square swept along paths with corners beside arcs: sound, and the
// volume of the pieces and the fills between them carried on, slice by
// slice across the arcs' axis.
func TestSweepJointedCorners(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("the volumes are integrated by rows: a few seconds")
	}
	for name, parts := range jointPaths() {
		steps, pieces := testPath(parts)
		if err := CheckSweepCorners(squareAt(0, 0, 2), steps, false); err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		want := 2 * joinedArea(pieces, 1, 1500)
		sweepCheck(t, name, squareAt(0, 0, 2), steps, want, Tol(1e-4, 100))
	}
}

// A round bar swept along a corner either way beside an arc: the volume
// is the joined areas of its slices across the arc's axis.
func TestSweepJointedCornersRound(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("the volumes are integrated by slices and rows: a few seconds")
	}
	z := geom.Vector{Z: 1}
	round := sketchLoop(circleArcs(geom.Vector{}, z, 1), z)
	for _, name := range []string{"arc line 60", "line arc -60", "arc arc"} {
		steps, pieces := testPath(jointPaths()[name])
		// Across y = sin α the bar reaches cos α either side of the path.
		const n = 8
		want := 0.0
		for k := range n {
			al := (float64(k) + 0.5) * math.Pi / 2 / n
			want += 2 * joinedArea(pieces, math.Cos(al), 600) * math.Cos(al) * math.Pi / 2 / n
		}
		sweepCheck(t, name, round, steps, want, fineTol())
	}
}

// Profiles of slanted sides, whose joints on the arc's cones bow off
// their chords, and with holes, swept along corners beside arcs.
func TestSweepJointedCornersProfiles(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("the volumes are integrated by slices and rows: a few seconds")
	}
	diamond := PolygonLoopSet([]geom.Vector{{X: 1}, {Y: 1}, {X: -1}, {Y: -1}}, geom.Vector{Z: 1})
	ring := squareAt(0, 0, 2)
	hole := squareAt(0, 0, 1).Outer
	hole.Reverse()
	ring.Holes = append(ring.Holes, hole)
	for _, name := range []string{"arc line 60", "line arc -60", "line arc line"} {
		steps, pieces := testPath(jointPaths()[name])
		// At height y the diamond reaches 1 - |y| either side.
		const n = 16
		want := 0.0
		for k := range n {
			y := (float64(k) + 0.5) / n
			want += 2 * joinedArea(pieces, 1-y, 600) / n
		}
		sweepCheck(t, "diamond "+name, diamond, steps, want, fineTol())
		want = 2*joinedArea(pieces, 1, 1000) - joinedArea(pieces, 0.5, 1000)
		sweepCheck(t, "ring "+name, ring, steps, want, fineTol())
	}
}

// Corners beside arcs the pieces can't be joined at are refused by
// name, at the corner.
func TestCheckSweepCornersRefused(t *testing.T) {
	t.Parallel()
	for name, c := range map[string]struct {
		parts []pathPart
		want  error
	}{
		"a line too short":     {[]pathPart{{arc: true, r: 10, turn: deg(60)}, {l: 0.3, corner: deg(90)}}, ErrCornerTooLong},
		"an arc too short":     {[]pathPart{{l: 8}, {arc: true, r: 10, turn: deg(3), corner: deg(-90)}}, ErrCornerTooLong},
		"nearly turning back":  {[]pathPart{{arc: true, r: 10, turn: deg(60)}, {l: 8, corner: deg(170)}}, ErrCornerNoJoint},
		"sharp on a tight arc": {[]pathPart{{arc: true, r: 3, turn: deg(60)}, {l: 8, corner: deg(-150)}}, ErrCornerNoJoint},
	} {
		steps, _ := testPath(c.parts)
		err := CheckSweepCorners(squareAt(0, 0, 2), steps, false)
		if _, ok := errors.AsType[*CornerError](err); !errors.Is(err, c.want) || !ok {
			t.Errorf("%s: %v, want %v", name, err, c.want)
		}
	}
	// A corner about x after an arc about y.
	steps, _ := testPath([]pathPart{{arc: true, r: 10, turn: deg(60)}, {l: 8}})
	at := SweepEnd(steps[:1]).Apply(geom.Vector{})
	steps = []SweepStep{steps[0], {Corner: true, Center: at, Axis: geom.Vector{X: 1}, Angle: deg(30)}, steps[1]}
	if err := CheckSweepCorners(squareAt(0, 0, 2), steps, false); !errors.Is(err, ErrCornerOutOfPlane) {
		t.Errorf("out of plane: %v", err)
	}
}

// Corners beside arcs over profiles, the arc before or after, its
// radius and turn, and the corner's angle either way: every point either
// refused by CheckSweepCorners, at its corner, or swept sound, with a
// volume within 5% of the area times the path's length (the joint takes
// from one piece about what it gives the other; the tests above hold it
// to the slices'). 96 of the 240 points are refused, the corners of
// 150° and 170°: there the line's wall, offset from the path, passes
// outside the arc's wall the profile's same point turns along, so the
// arc's wall would leave through the line's start, which no mitre joins.
func TestSweepJointedCornerSweep(t *testing.T) {
	t.Parallel()
	diamond := PolygonLoopSet([]geom.Vector{{X: 1}, {Y: 1}, {X: -1}, {Y: -1}}, geom.Vector{Z: 1})
	z := geom.Vector{Z: 1}
	profiles := map[string]*SBezierLoopSet{
		"square":  squareAt(0, 0, 2),
		"round":   sketchLoop(circleArcs(geom.Vector{}, z, 1), z),
		"diamond": diamond,
	}
	areas := map[string]float64{"square": 4, "round": math.Pi, "diamond": 2}
	grid := sweep.Grid{
		Parallel: true,
		Axes: []sweep.Axis{
			sweep.Strings("profile", "square", "round", "diamond"),
			sweep.Strings("order", "arc line", "line arc"),
			sweep.Floats("r", 4, 12),
			sweep.Floats("turn", 30, 100),
			sweep.Floats("corner", -150, -90, -45, -20, 20, 45, 90, 150, 170, -170),
		},
		Sample: 24,
	}
	var mu sync.Mutex
	refused := 0
	grid.Run(func(p sweep.Point) error {
		arc := pathPart{arc: true, r: p.Float("r"), turn: deg(p.Float("turn"))}
		line := pathPart{l: 10}
		parts := []pathPart{arc, line}
		if p.String("order") == "line arc" {
			parts = []pathPart{line, arc}
		}
		parts[1].corner = deg(p.Float("corner"))
		steps, _ := testPath(parts)
		prof := profiles[p.String("profile")]
		if err := CheckSweepCorners(prof, steps, false); err != nil {
			if _, ok := errors.AsType[*CornerError](err); !ok {
				return fmt.Errorf("refused, not at a corner: %w", err)
			}
			if a := math.Abs(p.Float("corner")); a < 150 {
				return fmt.Errorf("a corner of %g° refused: %w", a, err)
			}
			mu.Lock()
			refused++
			mu.Unlock()
			return nil
		}
		s := SShell{Tol: fineTol()}
		s.MakeFromSweepOf(prof, steps, core.RGBA{}, 0, 0)
		if !meshIsSound(&s) {
			return errors.New("not watertight")
		}
		want := areas[p.String("profile")] * (10 + p.Float("r")*deg(p.Float("turn")))
		if v := shellVolume(&s); math.Abs(v-want) > 0.05*want {
			return fmt.Errorf("volume %g, want about %g", v, want)
		}
		return nil
	}).Require(t)
	t.Logf("refused %d", refused)
}
