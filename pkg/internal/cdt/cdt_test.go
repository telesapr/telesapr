package cdt

import (
	"math"
	"math/rand"
	"testing"
)

// loop adds a closed polygon's points and segments.
func loop(pts *[]Point, segs *[][2]int, poly []Point) {
	base := len(*pts)
	*pts = append(*pts, poly...)
	for i := range poly {
		*segs = append(*segs, [2]int{base + i, base + (i+1)%len(poly)})
	}
}

func area(pts []Point, tris [][3]int) float64 {
	a := 0.0
	for _, t := range tris {
		p, q, r := pts[t[0]], pts[t[1]], pts[t[2]]
		a += ((q.X-p.X)*(r.Y-p.Y) - (q.Y-p.Y)*(r.X-p.X)) / 2
	}
	return a
}

func checkCCW(t *testing.T, pts []Point, tris [][3]int) {
	t.Helper()
	for _, tr := range tris {
		if orient(pts[tr[0]], pts[tr[1]], pts[tr[2]]) <= 0 {
			t.Fatalf("triangle %v is not counter-clockwise", tr)
		}
	}
}

func circle(cx, cy, r float64, n int, cw bool) []Point {
	out := make([]Point, n)
	for i := range out {
		a := 2 * math.Pi * float64(i) / float64(n)
		if cw {
			a = -a
		}
		out[i] = Point{cx + r*math.Cos(a), cy + r*math.Sin(a)}
	}
	return out
}

func TestSquareWithHole(t *testing.T) {
	var pts []Point
	var segs [][2]int
	loop(&pts, &segs, []Point{{0, 0}, {10, 0}, {10, 10}, {0, 10}})
	loop(&pts, &segs, circle(5, 5, 2, 64, false))
	tris, err := Triangulate(pts, segs)
	if err != nil {
		t.Fatal(err)
	}
	checkCCW(t, pts, tris)
	hole := 0.0
	c := circle(5, 5, 2, 64, false)
	for i := range c {
		j := (i + 1) % len(c)
		hole += (c[i].X*c[j].Y - c[j].X*c[i].Y) / 2
	}
	if got, want := area(pts, tris), 100-hole; math.Abs(got-want) > 1e-9 {
		t.Errorf("area %g, want %g", got, want)
	}
}

func TestOrientationDoesNotMatter(t *testing.T) {
	// Even-odd: a clockwise outer loop and a hole either way round.
	for _, cw := range []bool{false, true} {
		var pts []Point
		var segs [][2]int
		loop(&pts, &segs, circle(0, 0, 10, 50, cw))
		loop(&pts, &segs, circle(0, 0, 3, 30, !cw))
		tris, err := Triangulate(pts, segs)
		if err != nil {
			t.Fatal(err)
		}
		checkCCW(t, pts, tris)
		a := area(pts, tris)
		want := 50*100*math.Sin(2*math.Pi/50)/2 - 30*9*math.Sin(2*math.Pi/30)/2
		if math.Abs(a-want) > 1e-9 {
			t.Errorf("cw %v: area %g, want %g", cw, a, want)
		}
	}
}

func TestSteinerPointsAndCollinear(t *testing.T) {
	var pts []Point
	var segs [][2]int
	// A square whose sides carry collinear points, filled with a grid.
	var sq []Point
	for i := 0; i < 10; i++ {
		sq = append(sq, Point{float64(i), 0})
	}
	for i := 0; i < 10; i++ {
		sq = append(sq, Point{10, float64(i)})
	}
	for i := 10; i > 0; i-- {
		sq = append(sq, Point{float64(i), 10})
	}
	for i := 10; i > 0; i-- {
		sq = append(sq, Point{0, float64(i)})
	}
	loop(&pts, &segs, sq)
	nb := len(pts)
	for i := 1; i < 10; i++ {
		for j := 1; j < 10; j++ {
			pts = append(pts, Point{float64(i) + 0.5*0, float64(j)})
		}
	}
	tris, err := Triangulate(pts, segs)
	if err != nil {
		t.Fatal(err)
	}
	checkCCW(t, pts, tris)
	if a := area(pts, tris); math.Abs(a-100) > 1e-9 {
		t.Errorf("area %g", a)
	}
	if want := 2 * 100; len(tris) != want {
		t.Errorf("%d triangles, want %d", len(tris), want)
	}
	used := map[int]bool{}
	for _, tr := range tris {
		for _, v := range tr {
			used[v] = true
		}
	}
	if len(used) != len(pts) {
		t.Errorf("%d of %d points used (%d on the boundary)", len(used), len(pts), nb)
	}
}

func TestDoubledSegmentsCancel(t *testing.T) {
	// A square with a slit into it along a doubled segment: the slit
	// leaves the area whole.
	pts := []Point{{0, 0}, {4, 0}, {4, 4}, {0, 4}, {2, 2}}
	segs := [][2]int{{0, 1}, {1, 2}, {2, 3}, {3, 0}, {0, 4}, {4, 0}}
	tris, err := Triangulate(pts, segs)
	if err != nil {
		t.Fatal(err)
	}
	if a := area(pts, tris); math.Abs(a-16) > 1e-12 {
		t.Errorf("area %g, want 16", a)
	}
}

func TestDuplicatePointsMerge(t *testing.T) {
	pts := []Point{{0, 0}, {1, 0}, {1, 1}, {0, 1}, {1, 1}}
	segs := [][2]int{{0, 1}, {1, 4}, {2, 3}, {3, 0}}
	tris, err := Triangulate(pts, segs)
	if err != nil {
		t.Fatal(err)
	}
	if a := area(pts, tris); math.Abs(a-1) > 1e-12 {
		t.Errorf("area %g", a)
	}
}

func TestDelaunayWithoutSegments(t *testing.T) {
	// Random points inside a triangle that is the only boundary: every
	// triangle's circumcircle is empty, as no inner segment constrains
	// it.
	r := rand.New(rand.NewSource(1))
	pts := []Point{{0, 0}, {1, 0}, {0, 1}}
	for len(pts) < 400 {
		p := Point{r.Float64(), r.Float64()}
		if p.X+p.Y < 0.98 && p.X > 0.01 && p.Y > 0.01 {
			pts = append(pts, p)
		}
	}
	tris, err := Triangulate(pts, [][2]int{{0, 1}, {1, 2}, {2, 0}})
	if err != nil {
		t.Fatal(err)
	}
	checkCCW(t, pts, tris)
	if a := area(pts, tris); math.Abs(a-0.5) > 1e-12 {
		t.Errorf("area %g", a)
	}
	for _, tr := range tris {
		for i, p := range pts {
			if i == tr[0] || i == tr[1] || i == tr[2] {
				continue
			}
			if incircle(pts[tr[0]], pts[tr[1]], pts[tr[2]], p) > 0 {
				t.Fatalf("point %d lies in the circumcircle of %v", i, tr)
			}
		}
	}
}

func TestRandomPolygons(t *testing.T) {
	// Star-shaped polygons with many vertices and a grid inside: the
	// area must be the polygon's.
	r := rand.New(rand.NewSource(7))
	for k := 0; k < 50; k++ {
		n := 3 + r.Intn(200)
		poly := make([]Point, n)
		for i := range poly {
			a := 2 * math.Pi * float64(i) / float64(n)
			rad := 1 + 4*r.Float64()
			poly[i] = Point{rad * math.Cos(a), rad * math.Sin(a)}
		}
		var pts []Point
		var segs [][2]int
		loop(&pts, &segs, poly)
		for x := -5.0; x <= 5; x += 0.37 {
			for y := -5.0; y <= 5; y += 0.41 {
				pts = append(pts, Point{x, y})
			}
		}
		tris, err := Triangulate(pts, segs)
		if err != nil {
			t.Fatalf("polygon %d: %v", k, err)
		}
		checkCCW(t, pts, tris)
		want := 0.0
		for i := range poly {
			j := (i + 1) % n
			want += (poly[i].X*poly[j].Y - poly[j].X*poly[i].Y) / 2
		}
		if got := area(pts, tris); math.Abs(got-want) > 1e-9*want {
			t.Fatalf("polygon %d: area %g, want %g", k, got, want)
		}
	}
}

func TestCrossingSegments(t *testing.T) {
	// A bow tie: its segments cross, which is reported.
	pts := []Point{{0, 0}, {2, 2}, {2, 0}, {0, 2}}
	segs := [][2]int{{0, 1}, {1, 2}, {2, 3}, {3, 0}}
	if _, err := Triangulate(pts, segs); err != ErrCrossing {
		t.Errorf("error %v, want ErrCrossing", err)
	}
}

func TestDegenerateInput(t *testing.T) {
	if tris, err := Triangulate(nil, nil); tris != nil || err != nil {
		t.Error("empty input")
	}
	pts := []Point{{0, 0}, {0, 0}, {0, 0}}
	if tris, _ := Triangulate(pts, [][2]int{{0, 1}, {1, 2}, {2, 0}}); len(tris) != 0 {
		t.Error("one point made triangles")
	}
	pts = []Point{{0, 0}, {1, 0}, {math.NaN(), 0}}
	if _, err := Triangulate(pts, [][2]int{{0, 1}, {1, 2}, {2, 0}}); err == nil {
		t.Error("NaN accepted")
	}
	pts = []Point{{0, 0}, {1, 0}, {0, 1}}
	if _, err := Triangulate(pts, [][2]int{{0, 1}, {1, 2}, {2, 7}}); err == nil {
		t.Error("bad index accepted")
	}
}

func TestExactPredicates(t *testing.T) {
	// Points on a line, off by less than rounding.
	a, b := Point{0.1, 0.1}, Point{0.3, 0.3}
	c := Point{0.2, 0.2}
	if o := orient(a, b, c); o != orientExact(a, b, c) {
		t.Errorf("orient %d, exact %d", o, orientExact(a, b, c))
	}
	// Four points on a circle.
	p := []Point{{1, 0}, {0, 1}, {-1, 0}, {0, -1}}
	if s := incircle(p[0], p[1], p[2], p[3]); s != 0 {
		t.Errorf("incircle of cocircular points %d", s)
	}
}

func BenchmarkGrid(b *testing.B) {
	var pts []Point
	var segs [][2]int
	loop(&pts, &segs, circle(0, 0, 10, 400, false))
	for x := -10.0; x <= 10; x += 0.2 {
		for y := -10.0; y <= 10; y += 0.2 {
			if x*x+y*y < 9.5*9.5 {
				pts = append(pts, Point{x, y})
			}
		}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := Triangulate(pts, segs); err != nil {
			b.Fatal(err)
		}
	}
}

func TestSegmentThroughVertex(t *testing.T) {
	// A diagonal segment, twice so it cancels, runs through an inner
	// point: it is split there and the area stays whole.
	pts := []Point{{0, 0}, {4, 0}, {4, 4}, {0, 4}, {2, 2}, {1, 3}, {3, 1}}
	segs := [][2]int{{0, 1}, {1, 2}, {2, 3}, {3, 0}, {0, 2}, {2, 0}}
	tris, err := Triangulate(pts, segs)
	if err != nil {
		t.Fatal(err)
	}
	checkCCW(t, pts, tris)
	if a := area(pts, tris); math.Abs(a-16) > 1e-12 {
		t.Errorf("area %g", a)
	}
	edges := map[[2]int]bool{}
	for _, tr := range tris {
		for k := 0; k < 3; k++ {
			edges[[2]int{tr[k], tr[(k+1)%3]}] = true
		}
	}
	if !(edges[[2]int{0, 4}] || edges[[2]int{4, 0}]) || !(edges[[2]int{4, 2}] || edges[[2]int{2, 4}]) {
		t.Error("the diagonal is not split at the middle point")
	}
}
