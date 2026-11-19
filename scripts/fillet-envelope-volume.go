//go:build ignore

// Prints the volume a rolling-ball fillet of radius r takes from the
// y+ rim edge of curvedPocket in the dish with both walls leaning
// (pkg/topo, fillet_pivot_curved_test.go), integrated straight from the
// ball's envelope, not from any mesh or B-rep: a point is taken when it
// lies in the solid, between the x- wall's plane and the x+ wall's
// cylinder carried on past the pocket, and in the spandrel of the
// ball whose centre's normal plane along the spine holds it. The spine
// is where the cone of the top offset r into the solid meets the y+
// wall's cylinder grown by r. TestFilletEndLeaningWalls checks our
// volumes against these. Run with
//
//	go run scripts/fillet-envelope-volume.go -r 1
//
// With -chamfer it is the chamfer of distance r instead, as pkg/topo's
// chamferSection makes it: in each normal plane of the edge, the line
// between the top's and the wall's points r from the edge
// (TestFilletEndLeaningWallsChamfer). From 2.25 the chamfer's foot on
// the wall runs below the wall's foot on the pocket's floor, z = 4,
// where the rim dips lowest; the floor's plane then cuts the chamfer,
// and -floor 4 takes only what lies above it
// (TestFilletEndLeaningWallsChamferPastFloor).
//
// With -pts file, it instead reads lines "P face x y z" (points sampled
// on another program's fillet faces) and prints how far each face's
// points lie off the envelope, the ball's surface.
package main

import (
	"flag"
	"fmt"
	"math"
	"os"
	"strings"
	"sync"
)

type vec struct{ x, y, z float64 }

func (a vec) add(b vec) vec     { return vec{a.x + b.x, a.y + b.y, a.z + b.z} }
func (a vec) sub(b vec) vec     { return vec{a.x - b.x, a.y - b.y, a.z - b.z} }
func (a vec) mul(s float64) vec { return vec{a.x * s, a.y * s, a.z * s} }
func (a vec) dot(b vec) float64 { return a.x*b.x + a.y*b.y + a.z*b.z }
func (a vec) norm() float64     { return math.Sqrt(a.dot(a)) }
func (a vec) unit() vec         { return a.mul(1 / a.norm()) }
func (a vec) String() string    { return fmt.Sprintf("(%.4f, %.4f, %.4f)", a.x, a.y, a.z) }

// The dish's top, z = 5 + k ρ, and the walls' cylinders of radius 50
// about axes at height 40: the y+ wall's across at cy, the x+ wall's at
// x = -28.
const k = 0.375

var (
	sk = math.Sqrt(1 + k*k)
	cy = 8 - math.Sqrt(2500-400)
)

// spine is the ball's centre at x, r below the top and r outside the y+
// wall's cylinder, by Newton's steps from guess.
func spine(r, x float64, guess vec) vec {
	y, z := guess.y, guess.z
	for range 60 {
		h := math.Hypot(y-cy, z-40)
		f1 := h - (50 + r)
		a, b := (y-cy)/h, (z-40)/h
		rho := math.Hypot(x, y)
		f2 := 5 + k*rho - z - r*sk
		c, d := k*y/rho, -1.0
		det := a*d - b*c
		dy, dz := (f1*d-b*f2)/det, (a*f2-c*f1)/det
		y, z = y-dy, z-dz
		if math.Abs(dy)+math.Abs(dz) < 1e-14 {
			break
		}
	}
	return vec{x, y, z}
}

// centreFor is the spine's point whose normal plane holds p.
func centreFor(r float64, p, guess vec) vec {
	xs, g := p.x, guess
	const eps, step = 1e-6, 1e-4
	tangent := func(x float64, c vec) vec {
		return spine(r, x+eps, c).sub(spine(r, x-eps, c)).mul(1 / (2 * eps))
	}
	for range 60 {
		c := spine(r, xs, g)
		g = c
		f := p.sub(c).dot(tangent(xs, c))
		c2 := spine(r, xs+step, c)
		df := (p.sub(c2).dot(tangent(xs+step, c2)) - f) / step
		dx := f / df
		xs -= dx
		if math.Abs(dx) < 1e-13 {
			break
		}
	}
	return spine(r, xs, g)
}

// inSolid is whether p lies in the dish, outside the pocket's y+ wall,
// and between the x- wall's plane and the x+ wall's cylinder.
func inSolid(p vec) bool {
	return p.z <= 5+k*math.Hypot(p.x, p.y) && math.Hypot(p.y-cy, p.z-40) >= 50 &&
		p.x >= -12 && math.Hypot(p.x+28, p.z-40) <= 50
}

// floorZ is the height of the plane below which nothing is taken
// (-floor): the pocket's floor, which cuts a fillet or chamfer whose
// foot on the wall runs below the wall's foot.
var floorZ = math.Inf(-1)

// taken is whether the fillet takes p: in the solid, past the ball and
// between the ball's radii to its feet on the top and the wall; with
// chamfer, the chamfer of distance r (chamferTaken).
func taken(r float64, chamfer bool, p, guess vec) bool {
	if !inSolid(p) || p.z < floorZ {
		return false
	}
	if chamfer {
		return chamferTaken(r, p, guess)
	}
	c := centreFor(r, p, guess)
	d := p.sub(c)
	if d.norm() <= r {
		return false
	}
	rho := math.Hypot(c.x, c.y)
	u0 := vec{-k * c.x / rho, -k * c.y / rho, 1}.unit()
	u1 := vec{0, -(c.y - cy), -(c.z - 40)}.unit()
	a11, a12, a22 := u0.dot(u0), u0.dot(u1), u1.dot(u1)
	b1, b2 := u0.dot(d), u1.dot(d)
	det := a11*a22 - a12*a12
	return (a22*b1-a12*b2)/det >= 0 && (a11*b2-a12*b1)/det >= 0
}

// chamferTaken is whether the chamfer of distance r takes p, a point of
// the solid: in the edge's normal plane that holds p, on the edge's side
// of the line between the feet, the points of the top's and the wall's
// curves in that plane r from the edge, as pkg/topo's chamferSection
// puts them.
func chamferTaken(r float64, p, guess vec) bool {
	e := centreFor(0, p, guess)
	const eps = 1e-6
	tn := spine(0, e.x+eps, e).sub(spine(0, e.x-eps, e)).unit()
	// The plane's axes: a across the top, b square to it and to tn.
	a := vec{0, 1, 0}
	a = a.sub(tn.mul(a.dot(tn))).unit()
	b := vec{tn.y*a.z - tn.z*a.y, tn.z*a.x - tn.x*a.z, tn.x*a.y - tn.y*a.x}
	at := func(phi float64) vec {
		return e.add(a.mul(r * math.Cos(phi))).add(b.mul(r * math.Sin(phi)))
	}
	top := func(q vec) float64 { return 5 + k*math.Hypot(q.x, q.y) - q.z }
	wall := func(q vec) float64 { return math.Hypot(q.y-cy, q.z-40) - 50 }
	// The foot on the top is on the wall's outside, the wall's below the
	// top: each the root of its surface round the circle about e.
	f0, ok0 := rootOn(at, top, func(q vec) bool { return wall(q) > 0 })
	f1, ok1 := rootOn(at, wall, func(q vec) bool { return top(q) > 0 })
	if !ok0 || !ok1 {
		return false
	}
	ch := f1.sub(f0)
	n := e.sub(f0)
	n = n.sub(ch.mul(n.dot(ch) / ch.dot(ch)))
	d := p.sub(e)
	return d.norm() <= 2*r && p.sub(f0).dot(n) >= 0
}

// rootOn is the point of the circle at(φ) where f changes sign and keep
// holds.
func rootOn(at func(float64) vec, f func(vec) float64, keep func(vec) bool) (vec, bool) {
	const n = 180
	for i := range n {
		lo, hi := 2*math.Pi*float64(i)/n, 2*math.Pi*float64(i+1)/n
		if f(at(lo))*f(at(hi)) > 0 {
			continue
		}
		for range 60 {
			m := (lo + hi) / 2
			if f(at(lo))*f(at(m)) <= 0 {
				hi = m
			} else {
				lo = m
			}
		}
		if q := at((lo + hi) / 2); keep(q) {
			return q, true
		}
	}
	return vec{}, false
}

func main() {
	r := flag.Float64("r", 1, "the fillet's radius")
	n := flag.Int("n", 200, "grid points across a radius")
	dx := flag.Float64("dx", 0.01, "the step between slices along x")
	chamfer := flag.Bool("chamfer", false, "the chamfer between the ball's feet, not the fillet")
	pts := flag.String("pts", "", "a file of points to check against the envelope instead")
	flag.Float64Var(&floorZ, "floor", math.Inf(-1), "the height of the pocket's floor, below which nothing is taken")
	flag.Parse()
	if *pts != "" {
		checkPoints(*r, *pts)
		return
	}
	h := *r / float64(*n)
	side := 2.2
	if *chamfer {
		side = 4.4
	}
	cells := int(side * float64(*n))
	var mu sync.Mutex
	var total, below float64
	var wg sync.WaitGroup
	sem := make(chan struct{}, 16)
	for i := range int(40 / *dx) {
		x := -12 + (float64(i)+0.5)**dx
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			// The slice's square about the edge, from the spine's point.
			c := spine(*r, x, vec{x, 3, 10})
			y0, z0 := c.y-1.6**r, c.z-0.3**r
			if *chamfer {
				// About the edge, the feet r from it either way.
				c = spine(0, x, vec{x, 3, 10})
				y0, z0 = c.y-2.2**r, c.z-2.2**r
			}
			area := 0.0
			for iy := range cells {
				for iz := range cells {
					p := vec{x, y0 + (float64(iy)+0.5)*h, z0 + (float64(iz)+0.5)*h}
					if taken(*r, *chamfer, p, c) {
						area += h * h
					}
				}
			}
			mu.Lock()
			total += area * *dx
			if x < 0 {
				below += area * *dx
			}
			mu.Unlock()
		}()
	}
	wg.Wait()
	fmt.Printf("r %g: volume %.6f (x < 0 %.6f, x > 0 %.6f)\n", *r, total, below, total-below)
}

// checkPoints prints, for each face of the points in file, the furthest
// its points lie off the ball's surface.
func checkPoints(r float64, file string) {
	data, err := os.ReadFile(file)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	worst, where := map[string]float64{}, map[string]vec{}
	for _, ln := range strings.Split(string(data), "\n") {
		var f string
		var p vec
		if n, _ := fmt.Sscanf(ln, "P %s %g %g %g", &f, &p.x, &p.y, &p.z); n != 4 {
			continue
		}
		c := centreFor(r, p, spine(r, p.x, vec{p.x, 3, 10}))
		if d := math.Abs(p.sub(c).norm() - r); d >= worst[f] {
			worst[f], where[f] = d, p
		}
	}
	for f, w := range worst {
		fmt.Printf("face %s: %.6f mm off at %v\n", f, w, where[f])
	}
}
