package sketch

import (
	"cmp"
	"errors"
	"fmt"
	"math"
	"slices"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// A thread is made on a part: its ridges' feet go into the part, and
// a ridge past the part's end is a coil on nothing, which the Boolean
// cannot join to it (a 20 mm thread on a 10 mm rod came out with 314
// naked edges). So the part is measured along the thread before a
// thread is made or edited, and again whenever it regenerates, since a
// parameter may shorten the part under it.

// ErrThreadPastPart is returned, wrapped with how far the part
// reaches, for a thread longer than the part it is made on.
var ErrThreadPastPart = errors.New("thread is longer than its part")

// ErrThreadThinPart is returned for a screw's thread on a part
// thinner than its root, where its ridges would stand on nothing.
var ErrThreadThinPart = errors.New("part is thinner than the thread's root")

// ErrThreadNoPart is returned for a thread whose circle has no part
// along it: no solid at all, or a solid the other way.
var ErrThreadNoPart = errors.New("no part along the thread from its circle")

// threadPartProbes are the angles, from the circle's u direction, of
// the lines along the axis the part is measured on: off the quarter
// turns, where the faces of a turned or extruded part join and a line
// could run along an edge.
var threadPartProbes = [...]float64{0.37, 0.37 + math.Pi/2, 0.37 + math.Pi, 0.37 + 3*math.Pi/2}

// threadPartSlack is how far, in millimetres, a thread may reach past
// its part: the part's end face is where a thread as long as the part
// ends, to within what a plane's intersection makes of it.
const threadPartSlack = 1e-3

// threadAxis is where a thread on the circle he lies: its centre, the
// way it runs, and two directions across it.
func (sk *Sketch) threadAxis(he core.HEntity, o ThreadOptions) (origin, along, u, v geom.Vector) {
	circle := sk.MustEntity(he)
	origin = sk.MustEntity(circle.Point[0]).PointGetNum(sk)
	q := sk.MustEntity(circle.Normal).NormalGetNum(sk)
	along = q.RotationN()
	if o.Reverse {
		along = along.ScaledBy(-1)
	}
	return origin, along, q.RotationU(), q.RotationV()
}

// checkThreadPart says whether the thread o on the circle he fits on
// the solid of part, the group whose solid so far the thread is made
// on: ErrThreadNoPart, ErrThreadThinPart, or ErrThreadPastPart wrapped
// with how far the part reaches.
func (sk *Sketch) checkThreadPart(he core.HEntity, o ThreadOptions, l threadLayout, part *Group) error {
	var probe partProbe
	if part != nil {
		probe = newPartProbe(&part.RunningShell, &part.RunningMesh)
	}
	origin, along, u, v := sk.threadAxis(he, o)
	reach := probe.reach(origin, along, u, v, l.baseR, o.Length)
	switch {
	case reach <= 0 && !o.Internal && probe.reach(origin, along, u, v, l.baseR/2, o.Length) > 0:
		return ErrThreadThinPart
	case reach <= 0:
		return ErrThreadNoPart
	case reach < o.Length-threadPartSlack:
		return fmt.Errorf("%w, which reaches %.4g mm along it", ErrThreadPastPart, reach)
	}
	return nil
}

// partProbe measures a solid along lines: a shell's, or a mesh's when
// the shell is empty. Its zero value is no solid.
type partProbe struct {
	hits func(a, b geom.Vector) []threadHit
	span float64
}

// newPartProbe is the probe of the solid; the shell is copied, to make
// its trims' BSPs on.
func newPartProbe(shell *srf.SShell, mesh *srf.SMesh) partProbe {
	p := partProbe{span: solidSpan(shell, mesh)}
	switch {
	case len(shell.Surface) > 0:
		sh := shell.Clone()
		sh.MakeClassifyingBsps(nil)
		p.hits = func(a, b geom.Vector) []threadHit { return shellHits(&sh, a, b) }
	case len(mesh.Triangles) > 0:
		tree := srf.NewMeshTree(mesh)
		p.hits = func(a, b geom.Vector) []threadHit { return meshHits(tree, a, b) }
	}
	return p
}

// reach is how far the solid reaches from the circle's plane along the
// thread, at radius r, where the ridges' feet go into it: on each of
// the probes, a line along the axis, to where the line first leaves the
// solid; the most of those, since a cross hole may cut one. It is 0
// where no line starts in the solid, and past length where the solid
// goes on beyond.
func (p partProbe) reach(origin, along, u, v geom.Vector, r, length float64) float64 {
	if p.hits == nil {
		return 0
	}
	// The lines start well behind the part, so that what they cross
	// before the circle's plane says whether they start inside.
	behind := length + p.span
	reach := 0.0
	for _, angle := range threadPartProbes {
		at := origin.Plus(u.ScaledBy(r * math.Cos(angle))).Plus(v.ScaledBy(r * math.Sin(angle)))
		a, b := at.Minus(along.ScaledBy(behind)), at.Plus(along.ScaledBy(length+1))
		reach = max(reach, reachFrom(p.hits(a, b), along, behind))
	}
	return reach
}

// threadHit is where a line meets a solid's boundary: how far along
// the line, and the solid's outward normal there.
type threadHit struct {
	t      float64
	normal geom.Vector
}

// reachFrom is, of the hits along a line in the direction along, how
// far past at the solid goes on from at without a break: 0 when the
// line is outside there, by the last hit before it.
func reachFrom(hits []threadHit, along geom.Vector, at float64) float64 {
	slices.SortFunc(hits, func(x, y threadHit) int { return cmp.Compare(x.t, y.t) })
	inside := false
	// A plane through the circle bounds a part that starts there: a hit
	// that close counts as before the circle's plane, the side it was
	// drawn from.
	const onPlane = 1e-6
	for _, h := range hits {
		leaving := h.normal.Dot(along) > 0
		if h.t <= at+onPlane {
			inside = !leaving
			continue
		}
		if !inside {
			return 0
		}
		if leaving {
			return h.t - at
		}
	}
	if !inside {
		return 0
	}
	return math.Inf(1)
}

// shellHits are where the segment (a, b) meets the shell's trimmed
// surfaces, which have their trims' BSPs made.
func shellHits(sh *srf.SShell, a, b geom.Vector) []threadHit {
	var inters []srf.SInter
	sh.AllPointsIntersecting(a, b, &inters, true, true, false)
	ab := b.Minus(a)
	out := make([]threadHit, 0, len(inters))
	for _, in := range inters {
		// A hit on an edge is the same crossing as its neighbour's.
		t := in.P.Minus(a).Dot(ab) / ab.Magnitude()
		if slices.ContainsFunc(out, func(h threadHit) bool { return math.Abs(h.t-t) < geom.LengthEps }) {
			continue
		}
		out = append(out, threadHit{t: t, normal: in.SurfNormal})
	}
	return out
}

// meshHits are where the segment (a, b) meets the mesh's triangles.
func meshHits(tree *srf.MeshTree, a, b geom.Vector) []threadHit {
	ab := b.Minus(a)
	n := ab.Magnitude()
	dir := ab.ScaledBy(1 / n)
	var out []threadHit
	tmin, skip := 0.0, -1
	for {
		hit, ok := tree.Ray(a, dir, tmin, skip)
		if !ok || hit.T > n {
			return out
		}
		normal := dir
		if hit.Cos < 0 {
			normal = dir.ScaledBy(-1)
		}
		if len(out) == 0 || hit.T-out[len(out)-1].t > geom.LengthEps {
			out = append(out, threadHit{t: hit.T, normal: normal})
		}
		tmin, skip = hit.T, hit.Tri
	}
}

// solidSpan is the diagonal of the box about the solid, which a line
// that starts that far behind a point starts outside it from.
func solidSpan(shell *srf.SShell, mesh *srf.SMesh) float64 {
	lo := geom.Vector{X: math.Inf(1), Y: math.Inf(1), Z: math.Inf(1)}
	hi := lo.ScaledBy(-1)
	for i := range shell.Surface {
		var smax, smin geom.Vector
		shell.Surface[i].AxisAlignedBounding(&smax, &smin)
		smax.MakeMaxMin(&hi, &lo)
		smin.MakeMaxMin(&hi, &lo)
	}
	for _, tr := range mesh.Triangles {
		for _, p := range [...]geom.Vector{tr.A, tr.B, tr.C} {
			p.MakeMaxMin(&hi, &lo)
		}
	}
	if len(shell.Surface) == 0 && len(mesh.Triangles) == 0 {
		return 1
	}
	return hi.Minus(lo).Magnitude() + 1
}

// reportThreadPart puts a problem on the group that keeps a thread when the
// part it is made on no longer holds it, as when a parameter has made
// the part shorter than the thread since, or when the thread could
// not follow its face (thread_follow.go).
func (g *Group) reportThreadPart(sk *Sketch) {
	if !g.Thread.IsThread() {
		return
	}
	if g.threadProblem != "" {
		g.setProblem(nil, "%s", g.threadProblem)
	}
	tg, err := sk.threadGroupsOf(g.H)
	circle := sk.Entity(g.Thread.Circle)
	if err != nil || circle == nil || !circle.IsCircle() {
		return
	}
	l, err := g.Thread.layout(circle.CircleGetRadiusNum(sk))
	if err != nil {
		return
	}
	if err := sk.checkThreadPart(g.Thread.Circle, g.Thread.ThreadOptions, l, sk.solidAt(tg.blank)); err != nil {
		g.setProblem(nil, "%v", err)
	}
}
