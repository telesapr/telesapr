package topo_test

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/internal/sweep"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/topo"
)

// bandPiece is a piece of a meridian for wallMoment: a line from a to
// b, or with arc an arc about centre from a to b the short way, and into
// the side its material lies on (for an arc, true when inside its
// circle).
type bandPiece struct {
	a, b, centre [2]float64
	arc          bool
	into         [2]float64
	inCircle     bool
}

// meridianPolygon is the profile of pts and arcs (as meridian takes
// them) as a polygon, each arc in many chords.
func meridianPolygon(pts [][2]float64, arcs map[int][2]float64) [][2]float64 {
	var poly [][2]float64
	for i, p := range pts {
		q := pts[(i+1)%len(pts)]
		c, ok := arcs[i]
		if !ok {
			poly = append(poly, p)
			continue
		}
		a0 := math.Atan2(p[1]-c[1], p[0]-c[0])
		a1 := math.Atan2(q[1]-c[1], q[0]-c[0])
		d := math.Remainder(a1-a0, 2*math.Pi)
		r := math.Hypot(p[0]-c[0], p[1]-c[1])
		for k := range 512 {
			a := a0 + d*float64(k)/512
			poly = append(poly, [2]float64{c[0] + r*math.Cos(a), c[1] + r*math.Sin(a)})
		}
	}
	return poly
}

func inPolygon(p [2]float64, poly [][2]float64) bool {
	in := false
	for i, a := range poly {
		b := poly[(i+1)%len(poly)]
		if (a[1] > p[1]) != (b[1] > p[1]) && p[0] < a[0]+(p[1]-a[1])*(b[0]-a[0])/(b[1]-a[1]) {
			in = !in
		}
	}
	return in
}

// wallMoment is ∫ r dA over the wall a shell of thickness thick leaves
// of the profile pts, arcs with the pieces holding a point of open taken
// away: every point of the profile within thick of a kept piece along
// its normal, the band each kept face leaves; the wall stops square to
// a kept face where it meets one taken away. Pieces on the axis are no
// faces. It is measured on a grid of step h, independently of the
// shell's construction.
func wallMoment(pts [][2]float64, arcs map[int][2]float64, open [][2]float64, thick, h float64) float64 {
	poly := meridianPolygon(pts, arcs)
	var bands []bandPiece
	// ring is every piece of the profile in order, faces or not.
	type ringPiece struct {
		bp         bandPiece
		open, face bool
	}
	var ring []ringPiece
	onPiece := func(bp bandPiece, p [2]float64) bool {
		if !bp.arc {
			dx, dy := bp.b[0]-bp.a[0], bp.b[1]-bp.a[1]
			l := math.Hypot(dx, dy)
			s := ((p[0]-bp.a[0])*dx + (p[1]-bp.a[1])*dy) / l
			return math.Abs((p[0]-bp.a[0])*dy-(p[1]-bp.a[1])*dx)/l < 1e-6 && s > -1e-6 && s < l+1e-6
		}
		r := math.Hypot(bp.a[0]-bp.centre[0], bp.a[1]-bp.centre[1])
		return math.Abs(math.Hypot(p[0]-bp.centre[0], p[1]-bp.centre[1])-r) < 1e-6
	}
	for i, p := range pts {
		q := pts[(i+1)%len(pts)]
		if p[0] == 0 && q[0] == 0 {
			ring = append(ring, ringPiece{bandPiece{a: p, b: q}, false, false})
			continue
		}
		bp := bandPiece{a: p, b: q}
		if c, ok := arcs[i]; ok {
			bp.arc, bp.centre = true, c
			a0 := math.Atan2(p[1]-c[1], p[0]-c[0])
			a1 := math.Atan2(q[1]-c[1], q[0]-c[0])
			am := a0 + math.Remainder(a1-a0, 2*math.Pi)/2
			r := math.Hypot(p[0]-c[0], p[1]-c[1])
			toward := [2]float64{c[0] + (r-1e-4)*math.Cos(am), c[1] + (r-1e-4)*math.Sin(am)}
			bp.inCircle = inPolygon(toward, poly)
		} else {
			mid := [2]float64{(p[0] + q[0]) / 2, (p[1] + q[1]) / 2}
			dx, dy := q[0]-p[0], q[1]-p[1]
			l := math.Hypot(dx, dy)
			bp.into = [2]float64{-dy / l, dx / l}
			if !inPolygon([2]float64{mid[0] + 1e-4*bp.into[0], mid[1] + 1e-4*bp.into[1]}, poly) {
				bp.into = [2]float64{dy / l, -dx / l}
			}
		}
		isOpen := false
		for _, o := range open {
			if onPiece(bp, o) {
				isOpen = true
			}
		}
		if !isOpen {
			bands = append(bands, bp)
		}
		ring = append(ring, ringPiece{bp, isOpen, true})
	}
	// Corners between pieces, each a line where it matters: where two
	// faces kept meet at an inside corner (the material's angle more than
	// a half turn), the walls' inner faces run on to meet, holding the
	// quadrilateral between the walls' ends and where they meet; where a face kept
	// meets one taken away at an outside corner, its wall runs on past
	// its end, under the face taken away.
	var extra []func([2]float64) bool
	for i, cur := range ring {
		prev := ring[(i+len(ring)-1)%len(ring)]
		if !prev.face || !cur.face {
			continue
		}
		c := cur.bp.a
		var d2 [2]float64
		if cur.bp.arc {
			a0 := math.Atan2(c[1]-cur.bp.centre[1], c[0]-cur.bp.centre[0])
			r := math.Hypot(c[0]-cur.bp.centre[0], c[1]-cur.bp.centre[1])
			a1 := math.Atan2(cur.bp.b[1]-cur.bp.centre[1], cur.bp.b[0]-cur.bp.centre[0])
			am := a0 + math.Remainder(a1-a0, 2*math.Pi)*1e-3
			d2 = [2]float64{cur.bp.centre[0] + r*math.Cos(am) - c[0], cur.bp.centre[1] + r*math.Sin(am) - c[1]}
		} else {
			d2 = [2]float64{cur.bp.b[0] - c[0], cur.bp.b[1] - c[1]}
		}
		if prev.bp.arc {
			continue
		}
		in1 := prev.bp.into
		inside := d2[0]*in1[0]+d2[1]*in1[1] < 0
		dot := func(p [2]float64, u [2]float64) float64 { return (p[0]-c[0])*u[0] + (p[1]-c[1])*u[1] }
		switch {
		case inside && !prev.open && !cur.open && !cur.bp.arc:
			// The quadrilateral between the two walls' ends at c and the
			// point q where their inner faces meet.
			in2 := cur.bp.into
			det := in1[0]*in2[1] - in1[1]*in2[0]
			q := [2]float64{c[0] + thick*(in2[1]-in1[1])/det, c[1] + thick*(in1[0]-in2[0])/det}
			quad := [][2]float64{c, {c[0] + thick*in1[0], c[1] + thick*in1[1]}, q, {c[0] + thick*in2[0], c[1] + thick*in2[1]}}
			extra = append(extra, func(p [2]float64) bool { return inPolygon(p, quad) })
		case !inside && !prev.open && cur.open:
			// prev runs into c: past its end, along its direction.
			dir := [2]float64{c[0] - prev.bp.a[0], c[1] - prev.bp.a[1]}
			l := math.Hypot(dir[0], dir[1])
			dir = [2]float64{dir[0] / l, dir[1] / l}
			extra = append(extra, func(p [2]float64) bool {
				s, d := dot(p, dir), dot(p, in1)
				return s >= 0 && s <= 3*thick && d >= 0 && d <= thick
			})
		case !inside && prev.open && !cur.open && !cur.bp.arc:
			// cur runs out of c: before its start, against its direction.
			in2 := cur.bp.into
			l := math.Hypot(d2[0], d2[1])
			back := [2]float64{-d2[0] / l, -d2[1] / l}
			extra = append(extra, func(p [2]float64) bool {
				s, d := dot(p, back), dot(p, in2)
				return s >= 0 && s <= 3*thick && d >= 0 && d <= thick
			})
		}
	}
	inBand := func(bp bandPiece, p [2]float64) bool {
		if !bp.arc {
			dx, dy := bp.b[0]-bp.a[0], bp.b[1]-bp.a[1]
			l := math.Hypot(dx, dy)
			s := ((p[0]-bp.a[0])*dx + (p[1]-bp.a[1])*dy) / l
			d := (p[0]-bp.a[0])*bp.into[0] + (p[1]-bp.a[1])*bp.into[1]
			return s >= 0 && s <= l && d >= 0 && d <= thick
		}
		c := bp.centre
		r := math.Hypot(bp.a[0]-c[0], bp.a[1]-c[1])
		dist := math.Hypot(p[0]-c[0], p[1]-c[1])
		d := dist - r
		if bp.inCircle {
			d = r - dist
		}
		if d < 0 || d > thick {
			return false
		}
		a0 := math.Atan2(bp.a[1]-c[1], bp.a[0]-c[0])
		span := math.Remainder(math.Atan2(bp.b[1]-c[1], bp.b[0]-c[0])-a0, 2*math.Pi)
		at := math.Remainder(math.Atan2(p[1]-c[1], p[0]-c[0])-a0, 2*math.Pi)
		if span < 0 {
			span, at = -span, -at
		}
		return at >= 0 && at <= span
	}
	lo, hi := [2]float64{math.Inf(1), math.Inf(1)}, [2]float64{math.Inf(-1), math.Inf(-1)}
	for _, p := range poly {
		lo = [2]float64{math.Min(lo[0], p[0]), math.Min(lo[1], p[1])}
		hi = [2]float64{math.Max(hi[0], p[0]), math.Max(hi[1], p[1])}
	}
	var m float64
	for r := lo[0] + h/2; r < hi[0]; r += h {
		for z := lo[1] + h/2; z < hi[1]; z += h {
			p := [2]float64{r, z}
			in := false
			for _, bp := range bands {
				if inBand(bp, p) {
					in = true
					break
				}
			}
			for _, e := range extra {
				if !in && e(p) {
					in = true
				}
			}
			if in && inPolygon(p, poly) {
				m += r * h * h
			}
		}
	}
	return m
}

// A revolve's curved face taken away where it meets a sloping or curved
// face kept, at an inside corner: a cylinder standing on a cone or on a
// torus. The inside leaves the corner into the empty wedge outside it,
// so the wall under the kept face keeps its thickness and ends square to
// it at the corner. The volume is Pappus's over the wall, measured on a
// grid apart from the shell (wallMoment).
func TestShellRevolvedInsideCornerSloping(t *testing.T) {
	t.Parallel()
	fine := srf.Tol(0.002, 20)
	type shellCase struct {
		name string
		pts  [][2]float64
		arcs map[int][2]float64
		open [][2]float64
	}
	var cases []shellCase
	// A cylinder of radius r0 above a cone from the outside's radius 10
	// at h1 to r0 at 5, rising or falling, its top taken away too. In the
	// ordinary run each radius with each height once, either way up, so
	// half of the 16; all under TELESAPR_SWEEP=1.
	for i, r0 := range []float64{4, 6} {
		for j, h1 := range []float64{2, 3.5, 6.5, 8} {
			if sweep.Full() || (i+j)%2 == 0 {
				cases = append(cases, shellCase{fmt.Sprintf("a cylinder of %g on a cone from %g", r0, h1),
					[][2]float64{{0, 0}, {10, 0}, {10, h1}, {r0, 5}, {r0, 10}, {0, 10}}, nil,
					[][2]float64{{r0, 8}, {r0 / 2, 10}}})
			}
			// Upside down: the cylinder below the cone, the corner at the
			// curved face's end, its bottom taken away.
			if sweep.Full() || (i+j)%2 == 1 {
				cases = append(cases, shellCase{fmt.Sprintf("a cylinder of %g below a cone to %g", r0, 10-h1),
					[][2]float64{{0, 0}, {r0, 0}, {r0, 5}, {10, 10 - h1}, {10, 10}, {0, 10}}, nil,
					[][2]float64{{r0, 2}, {r0 / 2, 0}}})
			}
		}
	}
	// A torus beside the cylinder: a cove, its material outside its
	// circle, and a round, its material inside.
	cases = append(cases,
		shellCase{"a cylinder on a cove", [][2]float64{{0, 0}, {10, 0}, {10, 3}, {6, 5}, {6, 10}, {0, 10}},
			map[int][2]float64{2: {9.5, 7}}, [][2]float64{{6, 8}, {3, 10}}},
		shellCase{"a cylinder on a round", [][2]float64{{0, 0}, {10, 0}, {10, 3}, {6, 5}, {6, 10}, {0, 10}},
			map[int][2]float64{2: {7, 2}}, [][2]float64{{6, 8}, {3, 10}}},
		// A step half the thickness wide on a cone: the run-out along the
		// step would pass its far end, so the inside leaves the corner
		// into the empty wedge above.
		shellCase{"a cylinder on a narrow step", [][2]float64{{0, 0}, {10, 0}, {10, 3}, {6.5, 5}, {6, 5}, {6, 10}, {0, 10}},
			nil, [][2]float64{{6, 8}, {3, 10}}},
		// A sphere taken away beside a cone: a dome on a sloping shoulder.
		shellCase{"a dome on a cone", [][2]float64{{0, 0}, {10, 0}, {10, 3}, {6, 5}, {0, 11}},
			map[int][2]float64{3: {0, 5}}, [][2]float64{{6 * math.Cos(0.7), 5 + 6*math.Sin(0.7)}}},
	)
	if testing.Short() {
		cases = cases[:3]
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			sh, rev := turned(meridian(t, c.pts, c.arcs, fine), 0, 2*math.Pi, fine)
			tp := build(t, sh)
			var open []int
			for _, rz := range c.open {
				for k := range 4 {
					a := math.Pi/4 + float64(k)*math.Pi/2
					f, d := tp.FaceAt(v(rz[0]*math.Cos(a), rz[0]*math.Sin(a), rz[1]))
					if f < 0 || d > 1e-6 {
						t.Fatalf("no face at %v", rz)
					}
					open = append(open, f)
				}
			}
			out, err := topo.ShellRevolved(sh, rev, open, 1, 1<<30)
			if err != nil {
				t.Fatal(err)
			}
			want := 2 * math.Pi * wallMoment(c.pts, c.arcs, c.open, 1, 0.005)
			if got := sound(t, out); math.Abs(got-want) > 0.003*want {
				t.Errorf("volume %g, want %g", got, want)
			}
		})
	}
}

// Where the solid comes back within the thickness outside a curved face
// taken away, as across a slot narrower than the thickness, the run-out
// past the face is shortened to clear it; and where two curved faces
// taken away meet at an inside corner, both run out and their moved
// pieces meet outside the corner. The volume is Pappus's over the wall,
// measured apart from the shell (wallMoment).
func TestShellRevolvedInsideCornerRunOut(t *testing.T) {
	t.Parallel()
	fine := srf.Tol(0.002, 20)
	type shellCase struct {
		name string
		pts  [][2]float64
		arcs map[int][2]float64
		open [][2]float64
	}
	var cases []shellCase
	// A slot of width w beside a cylinder of radius 6 taken away, from
	// z 5 to the top of the outside part at 8, its bottom flat or a cone
	// rising to the slot's far side.
	for _, w := range []float64{0.3, 0.5, 0.8} {
		cases = append(cases,
			shellCase{fmt.Sprintf("a slot %g wide, flat", w),
				[][2]float64{{0, 0}, {10, 0}, {10, 8}, {6 + w, 8}, {6 + w, 5}, {6, 5}, {6, 10}, {0, 10}}, nil,
				[][2]float64{{6, 8}, {3, 10}}},
			shellCase{fmt.Sprintf("a slot %g wide, a cone", w),
				[][2]float64{{0, 0}, {10, 0}, {10, 8}, {6 + w, 8}, {6 + w, 5 + w}, {6, 5}, {6, 10}, {0, 10}}, nil,
				[][2]float64{{6, 8}, {3, 10}}})
	}
	// A cylinder on a cone, both taken away, and on a cove.
	for _, h1 := range []float64{2, 3.5} {
		cases = append(cases, shellCase{fmt.Sprintf("a cylinder on a cone from %g, both open", h1),
			[][2]float64{{0, 0}, {10, 0}, {10, h1}, {6, 5}, {6, 10}, {0, 10}}, nil,
			[][2]float64{{6, 8}, {8, (h1 + 5) / 2}, {3, 10}}})
	}
	cases = append(cases, shellCase{"a cylinder on a cove, both open",
		[][2]float64{{0, 0}, {10, 0}, {10, 3}, {6, 5}, {6, 10}, {0, 10}},
		map[int][2]float64{2: {9.5, 7}}, [][2]float64{{6, 8}, {9.5 - math.Sqrt(0.25+16)*math.Cos(0.6), 7 - math.Sqrt(0.25+16)*math.Sin(0.6)}, {3, 10}}})
	if testing.Short() {
		cases = cases[:2]
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			sh, rev := turned(meridian(t, c.pts, c.arcs, fine), 0, 2*math.Pi, fine)
			tp := build(t, sh)
			var open []int
			for _, rz := range c.open {
				for k := range 4 {
					a := math.Pi/4 + float64(k)*math.Pi/2
					f, d := tp.FaceAt(v(rz[0]*math.Cos(a), rz[0]*math.Sin(a), rz[1]))
					if f < 0 || d > 1e-6 {
						t.Fatalf("no face at %v", rz)
					}
					open = append(open, f)
				}
			}
			out, err := topo.ShellRevolved(sh, rev, open, 1, 1<<30)
			if err != nil {
				t.Fatal(err)
			}
			want := 2 * math.Pi * wallMoment(c.pts, c.arcs, c.open, 1, 0.005)
			if got := sound(t, out); math.Abs(got-want) > 0.003*want {
				t.Errorf("volume %g, want %g", got, want)
			}
		})
	}
}

// Where the face beside the inside corner is so steep that the way out
// of the corner runs back through the solid, refused, saying so.
func TestShellRevolvedInsideCornerRefused(t *testing.T) {
	t.Parallel()
	refused := func(name string, pts [][2]float64, at [2]float64, says string) {
		sh, rev := turned(meridian(t, pts, nil, srf.Tolerance{}), 0, 2*math.Pi, srf.Tolerance{})
		tp := build(t, sh)
		var open []int
		for k := range 4 {
			a := math.Pi/4 + float64(k)*math.Pi/2
			f, _ := tp.FaceAt(v(at[0]*math.Cos(a), at[0]*math.Sin(a), at[1]))
			open = append(open, f)
		}
		_, err := topo.ShellRevolved(sh, rev, open, 1, 1<<30)
		if err == nil || !strings.Contains(err.Error(), says) {
			t.Errorf("%s: %v", name, err)
		}
	}
	// A slot a twentieth of the thickness wide: no run-out the shell
	// tries clears it.
	refused("beside a slot too narrow", [][2]float64{{0, 0}, {10, 0}, {10, 8}, {6.05, 8}, {6.05, 5}, {6, 5}, {6, 10}, {0, 10}},
		[2]float64{6, 8}, "nearer than an eighth")
}
