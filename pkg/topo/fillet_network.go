package topo

import (
	"errors"
	"math"
	"slices"
	"strings"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// Fillets of several edges at once, with one radius, as one edit: every
// face along a rounded edge, or at its end, is built again from its own
// loops with the corners there replaced, and the rounded faces are put
// in. An edge is straight, between two flat faces, and rounded by a
// cylinder; or circular, where a flat face meets a cylinder square to it
// (a boss's top or foot, a hole's rim or floor), or a cone about that
// axis (a countersink's rim, a frustum's top), and rounded by a torus,
// the edge's cross section turned about the axis. A chamfer
// is the same edit with a flat cross section, which on a circular edge
// makes a cone.
//
// An edge sticks out (convex: the fillet takes material away) or goes in
// (concave: the fillet fills the corner). The faces are cut back along
// the same tangent curves either way, and the cross section is the same
// arc, whose control point is on the edge: only the centre of a corner's
// sphere lies outside the solid rather than inside.
//
// At a vertex, which must be a corner of three flat faces or a seam
// along a circular edge, the rounded edges meet in one of these ways:
//
//   - one edge: it ends on the third face, cut along the arc where the
//     rounded face meets it, as the end of a single fillet;
//   - two straight edges: their cylinders meet along an ellipse in the
//     plane that halves them, a mitre; when the faces don't lie alike
//     about them, the one whose section turns further goes on past the
//     ellipse, cut by the other's side face along a conic, to where its
//     tangent line meets the unrounded edge; where the unrounded edge
//     is of the other kind, as a rib's sharp edge above its filled foot,
//     the ellipse lies outside both side faces, and the face cut by the
//     conic grows to it instead;
//   - three straight edges: a sphere about the point r inside all three
//     faces, or outside them for concave edges (a face's tangent point
//     is that centre moved r back to the face along its normal), which
//     each cylinder meets along the arc where it ends;
//   - two edges that meet smoothly, as two pieces of one circle at a
//     seam of the cylinder, or a rounded outline's arc and line: their
//     rounded faces meet along the cross section there.

// FilletPick picks an edge to round: a straight edge by its ends, which
// must be a whole edge's; a circular one by a point on it, and with it
// the pieces of its circle it goes on into smoothly. R is its own
// radius, or 0 for the one all the picks are given.
type FilletPick struct {
	A, B geom.Vector
	At   geom.Vector
	Arc  bool
	R    float64
	// D2 is a chamfer's distance on its second face, or 0 for the same
	// as on the first (roadmap 28.1). The first face is the one whose
	// identity is First, or with First 0 the face of the lower identity,
	// the one an edge's name gives first.
	D2    float64
	First srf.FaceID
}

// filletEdge is one edge to round and what follows from its faces.
type filletEdge struct {
	e      int
	v      [2]int         // its vertices, from start to end
	f      [2]int         // its faces
	u      [2]geom.Vector // into each face, square to the edge (straight edges)
	dir    geom.Vector    // from v[0] to v[1] (straight edges)
	ss     [2]float64     // the tangent curves' distance from the edge on each face
	w      float64        // the arc's weight
	r      float64        // the radius, or a chamfer's distance
	origin uint32
	// sheet is the key the patches along one edge of the solid share,
	// the pieces of a circle: its first piece's origin, so the fillet of
	// a circle is one face, not four (roadmap 14.1).
	sheet uint32
	// concave: the edge goes in, and the fillet fills it.
	concave bool

	// A circular edge: its curve from v[0] to v[1], the cylinder's axis
	// through centre along axis (the flat face's outward normal), its
	// radius, which face is the flat one, and whether the flat face lies
	// outside the circle (a hole's rim, a boss's foot).
	circ   bool
	curve  srf.SBezier
	centre geom.Vector
	axis   geom.Vector
	radius float64
	flatK  int
	out    bool
	// gen is, for each face, the direction into it from the edge in the
	// half plane through the axis: along the radius and along the axis.
	gen [2][2]float64
}

// fits checks that a circular edge's tangent curves lie on their faces:
// that no setback reaches past the axis.
func (fe *filletEdge) fits(m geom.Vector) error {
	if !fe.circ {
		return nil
	}
	for k := range 2 {
		if fe.radius+fe.ss[k]*fe.gen[k][0] < geom.LengthEps {
			gen := fe.gen[k]
			if fe.flatK >= 0 {
				// The flat face is named by the curved one beside it.
				gen = fe.gen[1-fe.flatK]
			}
			return filletErr(m, "a radius this large is wider than the %s", fe.kindName(gen))
		}
	}
	return nil
}

// twoDistances gives a chamfer's edge distance d1 on its first face and
// d2 on the other (FilletPick.D2, First).
func (t *Topology) twoDistances(fe *filletEdge, d1, d2 float64, first srf.FaceID) error {
	m := t.Vertices[fe.v[0]].P.Plus(t.Vertices[fe.v[1]].P).ScaledBy(0.5)
	if fe.circ {
		m = fe.curve.PointAt(0.5)
	}
	if !(d2 > 0) || math.IsInf(d2, 0) {
		return filletErr(m, "a chamfer's second distance must be more than zero")
	}
	id0, id1 := t.Faces[fe.f[0]].ID, t.Faces[fe.f[1]].ID
	k := 0
	switch first {
	case 0:
		if id1 < id0 {
			k = 1
		}
	case id0:
	case id1:
		k = 1
	default:
		return filletErr(m, "the face the chamfer's first distance is on is not at the edge")
	}
	fe.ss[k], fe.ss[1-k] = d1, d2
	return fe.fits(m)
}

// FirstFaceNormal is the outward normal of the first face, as
// FilletPick.First of 0 takes it, at the straight edge through a and b:
// the flat face of the lower identity. It is false when no edge runs
// there or that face is curved.
func (t *Topology) FirstFaceNormal(a, b geom.Vector) (geom.Vector, bool) {
	e := t.edgeAtPoint(a.Plus(b).ScaledBy(0.5))
	if e < 0 {
		return geom.Vector{}, false
	}
	fs := t.EdgeFaces(e)
	if fs[0] < 0 || fs[1] < 0 {
		return geom.Vector{}, false
	}
	f := fs[0]
	if t.Faces[fs[1]].ID < t.Faces[fs[0]].ID {
		f = fs[1]
	}
	return t.planarNormal(f)
}

// even reports whether the edge is set back alike on both faces.
func (fe *filletEdge) even() bool { return math.Abs(fe.ss[0]-fe.ss[1]) <= geom.LengthEps }

// sign is 1 for a convex edge, whose fillet's centres lie inside the
// solid, and -1 for a concave one.
func (fe *filletEdge) sign() float64 {
	if fe.concave {
		return -1
	}
	return 1
}

// filletCorner is how the rounded edges meet at a vertex.
type filletCorner struct {
	kind int // the number of rounded edges there, or seam
	// at is the point an edge's end moves to in a face.
	at map[[2]int]geom.Vector
	// For one edge: the face it ends on.
	end int
	// For three: the sphere's centre; and its radius, the edges'.
	centre geom.Vector
	r      float64
	// For two: the mitre's plane, through centre.
	mitreN geom.Vector
	// For two whose faces don't lie alike: the edge whose strip turns
	// further, which goes on past the mitre; the curve the two strips
	// share, from the top's tangent point to where the other's strip
	// ends; and the conic where the longer strip meets the other's side
	// face, cutFace, from there to the third edge.
	long    int
	shared  srf.SBezier
	conic   srf.SBezier
	cutFace int
	// For a setback corner: each fillet's section where it is cut, by
	// edge; each face's curve between two fillets, by face; and the
	// patches that fill the hole.
	arcs       map[int]srf.SBezier
	faceCurves map[int]srf.SBezier
	quads      []srf.SSurface
	// For a corner of chamfers: the faces that fill it.
	plates []cornerPlate
}

// seam is the kind of a corner where two pieces of a circular edge meet.
const seam = 4

// FilletNetwork rounds the edges segs (each a whole straight edge
// between two flat faces) with radius r. The rounded
// faces get origins origin, origin+1, ..., and the spheres the origins
// after them.
func FilletNetwork(sh *srf.SShell, segs [][2]geom.Vector, r float64, origin uint32) error {
	picks := make([]FilletPick, len(segs))
	for i, sg := range segs {
		picks[i] = FilletPick{A: sg[0], B: sg[1]}
	}
	return FilletPicks(sh, picks, r, origin, false)
}

// FilletPicks rounds the picked edges with radius r, or with flat
// chamfers them at distance r along both faces; a chamfer here takes
// circular edges only. Where edges of different radii, or convex and
// concave ones, meet at a corner, they are rounded in stages
// (filletInStages).
func FilletPicks(sh *srf.SShell, picks []FilletPick, r float64, origin uint32, flat bool) error {
	err := filletPicks(sh, picks, r, origin, flat, nil)
	if curvedEndRefusal(err) && len(picks) == 1 {
		// An edge between flat faces that ends on a curved one: the
		// rolling ball, whose end follows the curved face (roadmap 22.5);
		// a chamfer's flat section follows it as a fillet's arc does.
		var cp srf.SShell
		cp.MakeFromCopyOf(sh)
		rerr := filletOnCurvedEnd(&cp, picks[0], r, origin, flat)
		if rerr == nil {
			*sh = cp
			return nil
		}
		if !errors.Is(rerr, errNotRolling) {
			return byRollingBall(rerr)
		}
	}
	if (curvedEndRefusal(err) || rollingAloneRefusal(err)) && len(picks) > 1 {
		// Picks that are the pieces of one closed chain, some of them
		// the rolling ball's: the chain at once.
		if cerr := filletOneChain(sh, picks, r, origin, flat, nil); !errors.Is(cerr, errNotOneChain) {
			return cerr
		}
		// Picks whose fillets don't meet, some of them the rolling
		// ball's: one after another.
		if aerr := filletApart(sh, picks, r, origin, flat); !errors.Is(aerr, errNotApart) {
			if curvedEndRefusal(err) {
				return byRollingBall(aerr)
			}
			return aerr
		}
	}
	if err == nil || !mixedCorner(err) && !setbackRefusal(err) {
		return err
	}
	// The corners are checked before the shell is changed, so it is
	// still the solid as it was.
	var serr error
	if mixedCorner(err) {
		staged, e := filletInStages(sh, picks, r, origin, flat)
		if e == nil {
			*sh = staged
			return nil
		}
		serr = e
	}
	// Else each corner the stages can't round gets a setback, the hole
	// filled by patches (roadmap 22.4), which says why where it can't. A
	// chamfer's corner of chamfered edges only took its setback already,
	// in the first stage (chamferCornerAt); one with an edge left sharp
	// has none.
	if !flat {
		berr := filletPicksSetback(sh, picks, r, origin, false)
		if berr == nil {
			return nil
		}
		return berr
	}
	if serr == nil {
		return err
	}
	var fx, sx *FilletError
	errors.As(err, &fx)
	why := serr.Error()
	if errors.As(serr, &sx) {
		why = sx.Msg
	}
	return filletErr(fx.At, "%s, and rounded one radius at a time they don't go on round each other's corners: %s", fx.Msg, why)
}

// curvedEndMsg is step 10's refusal of an edge that ends at a curved
// face.
const curvedEndMsg = "a face at the edge's end is curved"

// curvedEndRefusal reports whether err is step 10's refusal of an edge
// that ends at a curved face.
func curvedEndRefusal(err error) bool {
	var fx *FilletError
	return errors.As(err, &fx) && fx.Msg == curvedEndMsg
}

// rollingAloneRefusal reports whether err refuses a rolling-ball edge
// picked with others.
func rollingAloneRefusal(err error) bool {
	var fx *FilletError
	return errors.As(err, &fx) && strings.HasPrefix(fx.Msg, rollingAloneMsg)
}

// rollingAloneMsg begins the refusal of a rolling-ball edge picked with
// others whose fillets it meets.
const rollingAloneMsg = "this edge's fillet rolls a ball between curved faces, which is made on its own"

// filletOnCurvedEnd rounds the one picked edge p by the rolling ball.
func filletOnCurvedEnd(sh *srf.SShell, p FilletPick, r float64, origin uint32, flat bool) error {
	t, err := Build(sh)
	if err != nil {
		return err
	}
	var e int
	if p.Arc {
		if e = t.curvedEdgeAt(p.At); e < 0 {
			return filletErr(p.At, "no edge of the solid runs there")
		}
	} else if e, err = t.wholeEdge(p.A, p.B); err != nil {
		return err
	}
	if p.R != 0 {
		r = p.R
	}
	return rollingBallFillet(sh, t, e, r, origin, flat)
}

// filletPicks is FilletPicks in one stage. A rounded edge's chain does
// not go on into an edge forbid reports.
func filletPicks(sh *srf.SShell, picks []FilletPick, r float64, origin uint32, flat bool, forbid func(t *Topology, e int) bool) error {
	return filletPicksWith(sh, picks, r, origin, flat, forbid, false, false)
}

// filletPicksSetback is filletPicks with setback corners where no
// sphere fits.
func filletPicksSetback(sh *srf.SShell, picks []FilletPick, r float64, origin uint32, forceSetback bool) error {
	return filletPicksWith(sh, picks, r, origin, false, nil, true, forceSetback)
}

func filletPicksWith(sh *srf.SShell, picks []FilletPick, r float64, origin uint32, flat bool, forbid func(t *Topology, e int) bool, setback, forceSetback bool) error {
	what := "fillet"
	if flat {
		what = "chamfer"
	}
	if len(picks) == 0 {
		return filletErr(geom.Vector{}, "there are no edges to %s", what)
	}
	at0 := picks[0].A
	if picks[0].Arc {
		at0 = picks[0].At
	}
	radii := []float64{r}
	for _, p := range picks {
		if p.R != 0 {
			radii = append(radii, p.R)
		}
	}
	for _, x := range radii {
		if !(x > 0) || math.IsInf(x, 0) {
			if flat {
				return filletErr(at0, "a chamfer's distance must be more than zero")
			}
			return filletErr(at0, "a fillet's radius must be more than zero")
		}
	}
	t, err := Build(sh)
	if err != nil {
		return filletErr(at0, "the solid's topology is broken: %v", err)
	}
	// The rolling ball projects its every step on the faces it rolls on.
	sh.KeepClosestGuesses(true)
	defer sh.KeepClosestGuesses(false)
	var edges []filletEdge
	byEdge := map[int]int{}
	add := func(fe filletEdge) {
		if _, dup := byEdge[fe.e]; dup {
			// Two entities along one edge, as an extrusion's copies are.
			return
		}
		fe.origin = origin + uint32(len(edges))
		if fe.sheet == 0 {
			fe.sheet = fe.origin
		}
		byEdge[fe.e] = len(edges)
		edges = append(edges, fe)
	}
	for _, p := range picks {
		var e int
		if p.Arc {
			if e = t.curvedEdgeAt(p.At); e < 0 {
				return filletErr(p.At, "no edge of the solid runs there")
			}
		} else if e, err = t.wholeEdge(p.A, p.B); err != nil {
			return err
		}
		pr := r
		if p.R != 0 {
			pr = p.R
		}
		fe, err := t.filletEdgeOf(e, pr, flat)
		if err != nil {
			if t.generalBlendEdge(e) {
				// Between two faces step 10's construction doesn't take: the
				// rolling ball (roadmap 22.2), an edit of its own.
				if len(picks) > 1 {
					// Chains that meet in mitres, as a rim's runs between the
					// creases of its wall, go together; else each on its own.
					merr := filletMitredPicks(sh, t, picks, r, origin, flat)
					if !errors.Is(merr, errNotMitred) {
						return merr
					}
					return filletErr(midpoint(t.Edges[e].Points), rollingAloneMsg+
						": pick it alone, or with edges whose fillets don't meet its own (%v)", err)
				}
				rerr := rollingBallFillet(sh, t, e, pr, origin, flat)
				if errors.Is(rerr, errNotRolling) || rerr != nil && strings.Contains(err.Error(), "large") {
					// Step 10's reason, of size, says more.
					return err
				}
				return byRollingBall(rerr)
			}
			return err
		}
		if flat && p.D2 != 0 {
			if err := t.twoDistances(&fe, pr, p.D2, p.First); err != nil {
				return err
			}
		}
		add(fe)
	}
	// A rounded edge goes on into the edges it meets smoothly: the other
	// pieces of a circle, and the lines and arcs of a rounded outline.
	// Where the next edge can't take the radius, as a rounded corner
	// of a smaller radius, the chain stops, and the corner's check
	// gives that reason (roadmap 15.7).
	stopped := map[int]error{}
	for i := 0; i < len(edges); i++ { //nolint:intrange // edges grows while it is walked
		for _, v := range edges[i].v {
			for _, x := range t.Vertices[v].Edges {
				if _, ok := byEdge[x]; ok {
					continue
				}
				if forbid != nil && forbid(t, x) {
					continue
				}
				fe, err := t.filletEdgeOf(x, edges[i].r, flat)
				var fx *FilletError
				if err != nil && errors.As(err, &fx) && strings.HasPrefix(fx.Msg, "a radius this large is wider") {
					stopped[v] = err
				}
				if err == nil && !edges[i].even() {
					// The setbacks go on with the faces they lie on: try both
					// ways round, smoothJoin pairs the faces.
					var joined bool
					for _, sw := range []int{0, 1} {
						fe.ss = [2]float64{edges[i].ss[sw], edges[i].ss[1-sw]}
						if fe.fits(t.Vertices[v].P) == nil && t.smoothJoin(&edges[i], &fe, v) {
							joined = true
							break
						}
					}
					if !joined {
						continue
					}
				}
				if err == nil && t.smoothJoin(&edges[i], &fe, v) {
					if t.Lead(x) == t.Lead(edges[i].e) {
						fe.sheet = edges[i].sheet
					}
					add(fe)
				}
			}
		}
	}

	// How the edges meet at each vertex they touch.
	corners := map[int]*filletCorner{}
	for _, fe := range edges {
		for _, v := range fe.v {
			if corners[v] != nil {
				continue
			}
			var c *filletCorner
			var err error
			if flat && t.chamferCornerTakes(v, edges, byEdge) {
				c, err = t.chamferCornerAt(v, edges, byEdge)
				if err != nil {
					return err
				}
				corners[v] = c
				continue
			}
			c, err = t.filletCornerAt(v, edges, byEdge, flat)
			if setback && forceSetback && err == nil && c.kind == 3 {
				c, err = t.setbackCornerAt(v, edges, byEdge)
			}
			if err != nil && setback && setbackRefusal(err) {
				c, err = t.setbackCornerAt(v, edges, byEdge)
			}
			if err != nil {
				if why := stopped[v]; why != nil {
					var fx *FilletError
					errors.As(why, &fx)
					what := "fillet"
					if flat {
						what = "chamfer"
					}
					return filletErr(t.Vertices[v].P, "the %s stops at a rounded corner it can't go on round: %s; "+
						"make the %s smaller than the corner's radius", what, fx.Msg, map[bool]string{false: "radius", true: "distance"}[flat])
				}
				return err
			}
			corners[v] = c
		}
	}

	// The faces built again: those along the edges, and those the edges
	// end on.
	redo := map[int]bool{}
	for _, fe := range edges {
		redo[fe.f[0]], redo[fe.f[1]] = true, true
	}
	for _, c := range corners {
		if c.kind == 1 {
			redo[c.end] = true
		}
	}
	faces := make([]int, 0, len(redo))
	for f := range redo {
		faces = append(faces, f)
	}
	slices.Sort(faces)

	type rebuilt struct {
		s     srf.SSurface
		loops [][]srf.SBezier
	}
	var again []rebuilt
	for _, f := range faces {
		loops, err := t.filletLoops(f, edges, byEdge, corners, flat)
		if err != nil {
			return err
		}
		again = append(again, rebuilt{t.planeUnder(f, loops), loops})
	}
	color := t.Surface(edges[0].f[0]).Color

	// The rounded faces' loops and the spheres', while t still describes
	// the solid.
	type patch struct {
		s    srf.SSurface
		loop []srf.SBezier
		at   geom.Vector
	}
	var patches []patch
	for i := range edges {
		fe := &edges[i]
		a, b := t.Vertices[fe.v[0]].P, t.Vertices[fe.v[1]].P
		ca, cb := corners[fe.v[0]], corners[fe.v[1]]
		start := [2]geom.Vector{ca.at[[2]int{fe.e, fe.f[0]}], ca.at[[2]int{fe.e, fe.f[1]}]}
		end := [2]geom.Vector{cb.at[[2]int{fe.e, fe.f[0]}], cb.at[[2]int{fe.e, fe.f[1]}]}
		mid := a.Plus(b).ScaledBy(0.5)
		if fe.circ {
			mid = fe.curve.PointAt(0.5)
		} else {
			for k := range 2 {
				if end[k].Minus(start[k]).Dot(fe.dir) < 10*geom.LengthEps {
					return filletErr(mid, "the edge is too short for a %s this large", what)
				}
			}
		}
		side0 := fe.tangentCurve(0, start[0], end[0])
		side1 := fe.tangentCurve(1, start[1], end[1])
		side1.Reverse()
		loop := []srf.SBezier{side0}
		loop = append(loop, fe.connectors(t, fe.v[1], cb, end[0], end[1], flat)...)
		loop = append(loop, side1)
		loop = append(loop, fe.connectors(t, fe.v[0], ca, start[1], start[0], flat)...)
		s := fe.surface(a, b, flat)
		s.Color = color
		s.Origin = fe.origin
		s.Sheet = fe.sheet
		patches = append(patches, patch{s, loop, mid})
	}
	next := origin + uint32(len(edges))
	var vs []int
	for v := range corners {
		vs = append(vs, v)
	}
	slices.Sort(vs)
	for _, v := range vs {
		c := corners[v]
		if c.kind != 3 {
			continue
		}
		var loop []srf.SBezier
		for _, x := range t.Vertices[v].Edges {
			fe := &edges[byEdge[x]]
			loop = append(loop, fe.connector(t, v, c, c.at[[2]int{x, fe.f[0]}], c.at[[2]int{x, fe.f[1]}], flat))
		}
		loop, err := chainCurves(loop)
		if err != nil {
			return filletErr(t.Vertices[v].P, "the corner's arcs don't close: %v", err)
		}
		sphere := spherePatch(c.centre, c.r, loop)
		sphere.Color = color
		sphere.Origin = next
		next++
		patches = append(patches, patch{sphere, loop, t.Vertices[v].P})
	}

	for _, v := range vs {
		c := corners[v]
		if c.kind != setbackCorner {
			continue
		}
		for _, q := range c.quads {
			e0, e1, e2, e3 := q.EdgeCurve(0), q.EdgeCurve(1), q.EdgeCurve(2), q.EdgeCurve(3)
			e2.Reverse()
			e3.Reverse()
			q.Color = color
			q.Origin = next
			next++
			patches = append(patches, patch{q, []srf.SBezier{e0, e1, e2, e3}, t.Vertices[v].P})
		}
		for _, pl := range c.plates {
			pl.s.Color = color
			pl.s.Origin = next
			next++
			patches = append(patches, patch{pl.s, pl.loop, t.Vertices[v].P})
		}
	}

	RemoveFaces(sh, t, faces)
	for _, rb := range again {
		if _, err := AddFace(sh, rb.s, rb.loops); err != nil {
			return filletErr(rb.loops[0][0].Start(), "a face can't be built again: %v", err)
		}
	}
	for _, p := range patches {
		if _, err := AddFace(sh, p.s, [][]srf.SBezier{p.loop}); err != nil {
			return filletErr(p.at, "the %s's face can't be put in: %v", what, err)
		}
	}
	if t, err = Build(sh); err != nil {
		return filletErr(at0, "%v", err)
	}
	if probs := t.Validate(); len(probs) > 0 {
		return filletErr(probs[0].At, "the %s leaves the solid open: %s", what, probs[0].Msg)
	}
	return nil
}

// planeUnder is face f's surface for the loops it is built again from:
// its own, but for a flat face the loops reach past, as the face a
// concave fillet runs on to at its end, or the one that grows to a
// mitre outside it. There the plane is laid again under the loops, for
// a plane must cover its face (planeOver), or a later Boolean doesn't
// find what meets the part that grew.
func (t *Topology) planeUnder(f int, loops [][]srf.SBezier) srf.SSurface {
	s := *t.Surface(f)
	n, flat := t.planarNormal(f)
	if !flat || s.DegM != 1 || s.DegN != 1 {
		return s
	}
	o := s.Ctrl[0][0]
	eu, ev := s.Ctrl[0][1].Minus(o), s.Ctrl[1][0].Minus(o)
	uu, uv, vv := eu.Dot(eu), eu.Dot(ev), ev.Dot(ev)
	det := uu*vv - uv*uv
	if det < 1e-18 {
		return s
	}
	pts := []geom.Vector{s.Ctrl[0][0], s.Ctrl[0][1], s.Ctrl[1][0], s.Ctrl[1][1]}
	grown := false
	for _, loop := range loops {
		for _, sb := range loop {
			for k := range 9 {
				q := sb.PointAt(float64(k) / 8)
				d := q.Minus(o)
				a := (d.Dot(eu)*vv - d.Dot(ev)*uv) / det
				b := (d.Dot(ev)*uu - d.Dot(eu)*uv) / det
				if a < -1e-9 || a > 1+1e-9 || b < -1e-9 || b > 1+1e-9 {
					grown = true
				}
				pts = append(pts, q)
			}
		}
	}
	if !grown {
		return s
	}
	over := planeOver(pts, n)
	s.Ctrl, s.Weight = over.Ctrl, over.Weight
	return s
}

// curvedEdgeAt is the edge through p, a point on the curve the edge's
// points are chords of: within a twentieth of its longest chord, more
// than the sag of a chord that turns by less than a fifth of a turn.
func (t *Topology) curvedEdgeAt(p geom.Vector) int {
	best, bestD := -1, math.Inf(1)
	for e := range t.Edges {
		pts := t.Edges[e].Points
		longest := 0.0
		for i := 1; i < len(pts); i++ {
			longest = math.Max(longest, pts[i].Minus(pts[i-1]).Magnitude())
		}
		d := distanceToPolyline(p, pts)
		if d <= math.Max(1e-3, 0.05*longest) && d < bestD {
			best, bestD = e, d
		}
	}
	return best
}

// wholeEdge is the edge from a to b, which must be a whole edge.
func (t *Topology) wholeEdge(a, b geom.Vector) (int, error) {
	m := a.Plus(b).ScaledBy(0.5)
	e := t.edgeAtPoint(m)
	if e < 0 {
		return -1, filletErr(m, "no edge of the solid runs there")
	}
	va, vb := t.vertexNear(a), t.vertexNear(b)
	ed := &t.Edges[e]
	whole := ed.V[0] == va && ed.V[1] == vb || ed.V[0] == vb && ed.V[1] == va
	if va < 0 || vb < 0 || !whole {
		return -1, filletErr(m, "the segment is not a whole edge of the solid")
	}
	return e, nil
}

// filletEdgeOf checks edge e and works out its fillet, or with flat its
// chamfer.
func (t *Topology) filletEdgeOf(e int, r float64, flat bool) (filletEdge, error) {
	var fe filletEdge
	ed := &t.Edges[e]
	pa, pb := t.Vertices[ed.V[0]].P, t.Vertices[ed.V[1]].P
	m := midpoint(ed.Points)
	fs := t.EdgeFaces(e)
	if fs[0] < 0 || fs[1] < 0 || len(ed.CoEdges) != 2 {
		return fe, filletErr(m, "the edge is not between two faces")
	}
	if !t.Straight(e) {
		return t.circularEdgeOf(e, r, flat)
	}
	n1, ok1 := t.planarNormal(fs[0])
	n2, ok2 := t.planarNormal(fs[1])
	if !ok1 || !ok2 {
		return fe, filletErr(m, "a face at the edge is curved: a fillet here rounds edges between flat faces")
	}
	u1 := t.intoFace(fs[0], pa, pb, n1)
	u2 := t.intoFace(fs[1], pa, pb, n2)
	// Each face runs away from the other's outside for a convex edge,
	// towards it for a concave one.
	c1, c2 := u1.Dot(n2), u2.Dot(n1)
	if math.Abs(c1) < 1e-9 || math.Abs(c2) < 1e-9 || (c1 > 0) != (c2 > 0) {
		return fe, filletErr(m, "the edge is flat: a fillet rounds edges whose faces meet at an angle")
	}
	// The angle between the faces on the side the arc lies, outside the
	// solid for a concave edge.
	beta := math.Acos(math.Max(-1, math.Min(1, u1.Dot(u2))))
	fe = filletEdge{
		e: e, v: ed.V, f: fs, u: [2]geom.Vector{u1, u2}, r: r, concave: c1 > 0,
		dir: pb.Minus(pa).WithMagnitude(1), w: math.Sin(beta / 2),
	}
	fe.ss = [2]float64{r / math.Tan(beta/2), r / math.Tan(beta/2)}
	if flat {
		// A chamfer cuts at its distance along both faces.
		fe.ss = [2]float64{r, r}
	}
	return fe, nil
}

// circularEdgeOf checks a curved edge: an arc where a flat face meets a
// cylinder or a cone about the axis square to it through the arc's
// centre, the solid in the corner between them (convex) or round it
// (concave).
func (t *Topology) circularEdgeOf(e int, r float64, flat bool) (filletEdge, error) {
	var fe filletEdge
	ed := &t.Edges[e]
	m := midpoint(ed.Points)
	fs := t.EdgeFaces(e)
	what := "a fillet"
	if flat {
		what = "a chamfer"
	}
	flatK := -1
	var np geom.Vector
	for k, f := range fs {
		if n, ok := t.planarNormal(f); ok {
			flatK, np = k, n
		}
	}
	if flatK < 0 {
		// Two curved faces (roadmap 22.2): cylinders and cones about the
		// circle's axis, which its own plane gives.
		var ok bool
		if np, ok = planeNormalOf(ed.Points); !ok {
			return fe, filletErr(m, "the edge is curved, neither face at it is flat, and it is not a circle: %s rounds "+
				"edges of flat faces, cylinders and cones about one axis", what)
		}
		return t.coaxialEdgeOf(e, r, flat, np)
	}
	pa, pb := t.Vertices[ed.V[0]].P, t.Vertices[ed.V[1]].P
	var sb srf.SBezier
	c := t.shell.FindCurve(ed.Curve)
	switch {
	case c != nil && c.IsExact:
		sb = c.Exact
		switch {
		case sb.Start().Equals(pa) && sb.Finish().Equals(pb):
		case sb.Start().Equals(pb) && sb.Finish().Equals(pa):
			sb.Reverse()
		default:
			return fe, filletErr(m, "the edge is part of a longer curve")
		}
	default:
		// A Boolean's curve, its points on the faces' intersection: the
		// arc through them, when they lie on one.
		var ok bool
		if sb, ok = arcThrough(ed.Points, np); !ok {
			return fe, t.offAxisErr(m, fs[1-flatK], what)
		}
	}
	var cc geom.Vector
	var rad float64
	if !sb.IsCircle(np, &cc, &rad) {
		return fe, t.offAxisErr(m, fs[1-flatK], what)
	}
	fe = filletEdge{e: e, v: ed.V, f: fs, r: r, circ: true, curve: sb, centre: cc, axis: np, radius: rad, flatK: flatK}
	// The other face's generator: the line, in the half plane through the
	// axis, from the edge into the face; every point of its surface on
	// the line it turns about the axis.
	gen, ok := t.generator(&fe, fs[1-flatK])
	if !ok {
		return fe, t.offAxisErr(m, fs[1-flatK], what)
	}
	// The flat face on one side of the circle.
	step := 1e-3 * math.Min(rad, 1)
	radial := fe.radial(m)
	inside := t.inFace(fs[flatK], m.Minus(radial.ScaledBy(step)), np)
	outside := t.inFace(fs[flatK], m.Plus(radial.ScaledBy(step)), np)
	if inside == outside || math.Abs(gen[1]) < 1e-9 {
		return fe, filletErr(m, "the flat face and the %s don't meet as a corner at the edge", fe.kindName(gen))
	}
	fe.out = outside
	fe.gen[1-flatK] = gen
	fe.gen[flatK] = [2]float64{-1, 0}
	if fe.out {
		fe.gen[flatK][0] = 1
	}
	// Convex when the other face runs down from the flat one, under it,
	// as a boss's side from its top; concave when it rises, as from its
	// foot. The section's angle is between the two directions.
	fe.concave = gen[1] > 0
	beta := math.Acos(math.Max(-1, math.Min(1, fe.gen[0][0]*fe.gen[1][0]+fe.gen[0][1]*fe.gen[1][1])))
	fe.ss, fe.w = [2]float64{r / math.Tan(beta/2), r / math.Tan(beta/2)}, math.Sin(beta/2)
	if flat {
		// A chamfer cuts at its distance along both faces.
		fe.ss = [2]float64{r, r}
	}
	if err := fe.fits(m); err != nil {
		return fe, err
	}
	return fe, nil
}

// planeNormalOf is the normal of the plane the points lie in, when
// they do and don't lie on a line.
func planeNormalOf(pts []geom.Vector) (geom.Vector, bool) {
	if len(pts) < 3 {
		return geom.Vector{}, false
	}
	a, m, b := pts[0], pts[len(pts)/3], pts[2*len(pts)/3]
	n := m.Minus(a).Cross(b.Minus(a))
	if n.Magnitude() < 1e-12 {
		return geom.Vector{}, false
	}
	n = n.WithMagnitude(1)
	for _, p := range pts {
		if math.Abs(p.Minus(a).Dot(n)) > 10*geom.LengthEps {
			return geom.Vector{}, false
		}
	}
	return n, true
}

// coaxialEdgeOf checks a circular edge between two curved faces,
// cylinders or cones about the circle's axis n (roadmap 22.2): a cone
// on a shaft, a shaft from a cone's top. The fillet is the torus of the
// section's arc, as between a flat face and a cylinder; the edge is
// convex when each face runs away from the other's outside.
func (t *Topology) coaxialEdgeOf(e int, r float64, flat bool, n geom.Vector) (filletEdge, error) {
	var fe filletEdge
	ed := &t.Edges[e]
	m := midpoint(ed.Points)
	fs := t.EdgeFaces(e)
	pa, pb := t.Vertices[ed.V[0]].P, t.Vertices[ed.V[1]].P
	var sb srf.SBezier
	c := t.shell.FindCurve(ed.Curve)
	switch {
	case c != nil && c.IsExact:
		sb = c.Exact
		switch {
		case sb.Start().Equals(pa) && sb.Finish().Equals(pb):
		case sb.Start().Equals(pb) && sb.Finish().Equals(pa):
			sb.Reverse()
		default:
			return fe, filletErr(m, "the edge is part of a longer curve")
		}
	default:
		var ok bool
		if sb, ok = arcThrough(ed.Points, n); !ok {
			return fe, filletErr(m, "the edge has no exact curve, and its points are not on an arc of a circle")
		}
	}
	var cc geom.Vector
	var rad float64
	if !sb.IsCircle(n, &cc, &rad) {
		return fe, filletErr(m, "the edge is curved but not an arc of a circle")
	}
	fe = filletEdge{e: e, v: ed.V, f: fs, r: r, circ: true, curve: sb, centre: cc, axis: n, radius: rad, flatK: -1}
	// Each face's generator, and its outward normal in the half plane
	// through the axis.
	radial := fe.radial(m)
	var normal [2][2]float64
	for k := range 2 {
		gen, ok := t.generator(&fe, fs[k])
		if !ok {
			return fe, filletErr(m, "the edge is curved, and a face at it is neither flat nor a cylinder or a cone "+
				"about the circle's axis")
		}
		fe.gen[k] = gen
		s := t.Surface(fs[k])
		var u, v float64
		s.ClosestPointToUV(m, &u, &v, true)
		nk := s.NormalAt(u, v).WithMagnitude(1)
		normal[k] = [2]float64{nk.Dot(radial), nk.Dot(n)}
	}
	dot := func(a, b [2]float64) float64 { return a[0]*b[0] + a[1]*b[1] }
	c0, c1 := dot(fe.gen[0], normal[1]), dot(fe.gen[1], normal[0])
	if math.Abs(c0) < 1e-9 || math.Abs(c1) < 1e-9 || (c0 > 0) != (c1 > 0) {
		return fe, filletErr(m, "the faces don't meet as a corner at the edge: they go on smoothly into each other")
	}
	fe.concave = c0 > 0
	beta := math.Acos(math.Max(-1, math.Min(1, dot(fe.gen[0], fe.gen[1]))))
	fe.ss, fe.w = [2]float64{r / math.Tan(beta/2), r / math.Tan(beta/2)}, math.Sin(beta/2)
	if flat {
		fe.ss = [2]float64{r, r}
	}
	if err := fe.fits(m); err != nil {
		return fe, err
	}
	return fe, nil
}

// arcThrough is the rational quadratic arc through pts, from the first
// to the last, in a plane square to n, when they all lie on a circle
// and it turns by less than half a turn.
func arcThrough(pts []geom.Vector, n geom.Vector) (srf.SBezier, bool) {
	if len(pts) < 3 {
		return srf.SBezier{}, false
	}
	a, m, b := pts[0], pts[len(pts)/2], pts[len(pts)-1]
	// The centre: on the plane, as far from a as from m and b.
	n1, n2 := m.Minus(a), b.Minus(a)
	c := meet3(n, n.Dot(a), n1, n1.Dot(a.Plus(m).ScaledBy(0.5)), n2, n2.Dot(a.Plus(b).ScaledBy(0.5)))
	if math.Abs(n.Dot(n1.Cross(n2))) < 1e-12 {
		return srf.SBezier{}, false
	}
	rad := a.Minus(c).Magnitude()
	for _, p := range pts {
		if math.Abs(p.Minus(c).Magnitude()-rad) > 10*geom.LengthEps || math.Abs(p.Minus(a).Dot(n)) > 10*geom.LengthEps {
			return srf.SBezier{}, false
		}
	}
	da, db := a.Minus(c), b.Minus(c)
	theta := math.Acos(math.Max(-1, math.Min(1, da.Dot(db)/(rad*rad))))
	// The middle point's side: the arc through it.
	if da.Plus(db).Dot(m.Minus(c)) < 0 || theta > math.Pi-1e-6 || theta < 1e-9 {
		return srf.SBezier{}, false
	}
	mid := da.Plus(db).WithMagnitude(rad / math.Cos(theta/2))
	return arc(a, c.Plus(mid), b, math.Cos(theta/2)), true
}

// kindName names the surface a circular edge's generator gen turns out.
func (fe *filletEdge) kindName(gen [2]float64) string {
	if math.Abs(gen[0]) < 1e-9 {
		return "cylinder"
	}
	return "cone"
}

// generator is the direction, as (along the radius, along the axis), in
// which face f leaves the circular edge fe, when f's surface is a cone or
// cylinder about fe's axis through its circle: every point of it on the
// line through the circle with that direction, turned about the axis.
func (t *Topology) generator(fe *filletEdge, f int) ([2]float64, bool) {
	var none [2]float64
	at := func(p geom.Vector) [2]float64 {
		d := p.Minus(fe.centre)
		h := d.Dot(fe.axis)
		return [2]float64{d.Minus(fe.axis.ScaledBy(h)).Magnitude() - fe.radius, h}
	}
	// The point of the face's edges furthest from the circle gives the
	// direction.
	var g [2]float64
	far := 0.0
	var pts [][2]float64
	for _, x := range t.FaceEdges(f) {
		for _, p := range t.Edges[x].Points {
			q := at(p)
			pts = append(pts, q)
			if d := math.Hypot(q[0], q[1]); d > far {
				far, g = d, [2]float64{q[0] / d, q[1] / d}
			}
		}
	}
	if far < 10*geom.LengthEps {
		return none, false
	}
	s := t.Surface(f)
	if s == nil {
		return none, false
	}
	// The surface itself, over its whole patch, and the face's edges.
	for i := range 5 {
		for j := range 5 {
			pts = append(pts, at(s.PointAt(float64(i)/4, float64(j)/4)))
		}
	}
	for _, q := range pts {
		if math.Abs(q[0]*g[1]-q[1]*g[0]) > 10*geom.LengthEps {
			return none, false
		}
	}
	return g, true
}

// smoothJoin reports whether rounded edges a and b meet smoothly at
// vertex v: their cross sections there are one, so that their rounded
// faces go on into each other with no corner between, as at a seam of
// a cylinder or where a rounded outline's arc meets its line. Their
// faces pair off, the same face or two that go on into each other at
// the vertex, as a lathe's quarters of a disc do.
func (t *Topology) smoothJoin(a, b *filletEdge, v int) bool {
	if a.e == b.e || a.concave != b.concave || math.Abs(a.w-b.w) > 1e-9 {
		return false
	}
	if (a.v[0] != v && a.v[1] != v) || (b.v[0] != v && b.v[1] != v) {
		return false
	}
	p := t.Vertices[v].P
	for _, swap := range []int{0, 1} {
		ok := true
		for ka := range 2 {
			kb := ka ^ swap
			da, db := a.into(ka, p), b.into(kb, p)
			if !da.Equals(db) && da.Minus(db).Magnitude() > 1e-9 || math.Abs(a.ss[ka]-b.ss[kb]) > geom.LengthEps {
				ok = false
			}
		}
		if ok {
			return true
		}
	}
	return false
}

// radial is the direction from a circular edge's axis to p, square to
// the axis.
func (fe *filletEdge) radial(p geom.Vector) geom.Vector {
	d := p.Minus(fe.centre)
	return d.Minus(fe.axis.ScaledBy(d.Dot(fe.axis))).WithMagnitude(1)
}

// into is the direction at p, on the edge, square to it in face k and
// into that face: for a circular edge, along its generator there, in
// the half plane through the axis.
func (fe *filletEdge) into(k int, p geom.Vector) geom.Vector {
	if !fe.circ {
		return fe.u[k]
	}
	return fe.radial(p).ScaledBy(fe.gen[k][0]).Plus(fe.axis.ScaledBy(fe.gen[k][1]))
}

// tangentCurve is the curve along which the rounded face meets face k,
// from p (at the edge's start) to q (at its end): a line, or for a
// circular edge its arc moved along the face's generator: turned to a
// new radius and moved along the axis.
func (fe *filletEdge) tangentCurve(k int, p, q geom.Vector) srf.SBezier {
	if !fe.circ {
		return srf.SBezierFrom2(p, q)
	}
	sb := fe.curve
	scale := (fe.radius + fe.ss[k]*fe.gen[k][0]) / fe.radius
	lift := fe.axis.ScaledBy(fe.ss[k] * fe.gen[k][1])
	for i := 0; i <= sb.Degree; i++ {
		sb.Ctrl[i] = fe.centre.Plus(sb.Ctrl[i].Minus(fe.centre).ScaledBy(scale)).Plus(lift)
	}
	sb.Ctrl[0], sb.Ctrl[sb.Degree] = p, q
	return sb
}

// section is the cross section of the rounded face at p on the edge,
// from its point on face 0 to its point on face 1: an arc whose control
// point is p, or with flat a line.
func (fe *filletEdge) section(p geom.Vector, flat bool) srf.SBezier {
	a := p.Plus(fe.into(0, p).ScaledBy(fe.ss[0]))
	b := p.Plus(fe.into(1, p).ScaledBy(fe.ss[1]))
	if flat {
		return srf.SBezierFrom2(a, b)
	}
	return arc(a, p, b, fe.w)
}

// surface is the rounded face's surface: for a straight edge from a to
// b, the cross section swept along it, with a margin; for a circular
// one, the cross section at its start turned about the axis to its end.
func (fe *filletEdge) surface(a, b geom.Vector, flat bool) srf.SSurface {
	if !fe.circ {
		margin := math.Max(fe.ss[0], fe.ss[1]) + 2*fe.r
		l := b.Minus(a).Magnitude()
		p0 := a.Minus(fe.dir.ScaledBy(margin))
		section := fe.section(p0, flat)
		return srf.FromExtrusionOf(&section, geom.Vector{}, fe.dir.ScaledBy(l+2*margin))
	}
	section := fe.section(fe.curve.Start(), flat)
	// The angle from the arc's start to its end about the axis; an arc
	// of degree 2 turns by less than half a turn.
	ra, rb := fe.radial(fe.curve.Start()), fe.radial(fe.curve.Finish())
	theta := math.Atan2(ra.Cross(rb).Dot(fe.axis), ra.Dot(rb))
	// RotatedAroundAxis by a positive angle turns ra towards axis x ra,
	// or the other way.
	if ra.RotatedAroundAxis(fe.axis, 0.1).Dot(fe.axis.Cross(ra)) < 0 {
		theta = -theta
	}
	return srf.FromRevolutionOf(&section, fe.centre, fe.axis, 0, theta, 0, 0)
}

// facesAt are the faces round vertex v, each once.
func (t *Topology) facesAt(v int) []int {
	var out []int
	seen := map[int]bool{}
	for _, x := range t.Vertices[v].Edges {
		for _, f := range t.EdgeFaces(x) {
			if f >= 0 && !seen[f] {
				seen[f] = true
				out = append(out, f)
			}
		}
	}
	return out
}

// filletCornerAt works out how the rounded edges meet at vertex v, or
// with flat the chamfered ones, which its refusals name.
func (t *Topology) filletCornerAt(v int, edges []filletEdge, byEdge map[int]int, flat bool) (*filletCorner, error) {
	p := t.Vertices[v].P
	what := "fillet"
	if flat {
		what = "chamfer"
	}
	// The rounded edges here are all convex or all concave, of one radius.
	sg, r, n := 1.0, 0.0, 0
	for _, x := range t.Vertices[v].Edges {
		i, ok := byEdge[x]
		if !ok {
			continue
		}
		fe := &edges[i]
		if n > 0 && fe.sign() != sg {
			return nil, filletErr(p, "a convex edge's %s meets a concave one's here", what)
		}
		if n > 0 && math.Abs(fe.r-r) > geom.LengthEps {
			if flat {
				return nil, filletErr(p, "chamfers of distance %g and %g meet here above an edge left sharp", r, fe.r)
			}
			return nil, filletErr(p, "fillets of radius %g and %g meet here", r, fe.r)
		}
		sg, r, n = fe.sign(), fe.r, n+1
	}
	if n := len(t.Vertices[v].Edges); n != 3 || len(t.facesAt(v)) != 3 {
		// Four, where two rounded edges meet smoothly, each face of one
		// going on into a face of the other, as a lathe's quarters do.
		var round []*filletEdge
		for _, x := range t.Vertices[v].Edges {
			if i, ok := byEdge[x]; ok {
				round = append(round, &edges[i])
			}
		}
		if n != 4 || len(t.facesAt(v)) != 4 || len(round) != 2 || !t.smoothJoin(round[0], round[1], v) {
			return nil, filletErr(p, "the edge ends where %d edges meet; a %s's end needs a corner of three faces", n, what)
		}
	}
	var round, plain []int
	circular := 0
	for _, x := range t.Vertices[v].Edges {
		if i, ok := byEdge[x]; ok {
			round = append(round, x)
			if edges[i].circ {
				circular++
			}
		} else {
			plain = append(plain, x)
		}
	}
	c := &filletCorner{kind: len(round), at: map[[2]int]geom.Vector{}, long: -1, r: r}
	if c.kind == 2 && t.smoothJoin(&edges[byEdge[round[0]]], &edges[byEdge[round[1]]], v) {
		// A smooth join: both share the cross section, and the third edge,
		// between their other faces, is cut back to its point on them.
		a, b := &edges[byEdge[round[0]]], &edges[byEdge[round[1]]]
		c.kind = seam
		for _, fe := range []*filletEdge{a, b} {
			for k := range 2 {
				c.at[[2]int{fe.e, fe.f[k]}] = p.Plus(fe.into(k, p).ScaledBy(fe.ss[k]))
			}
		}
		// Each other edge is cut back to the tangent point on its faces,
		// which are faces of the rounded edges or go on from them.
		for _, x := range plain {
			fx := t.EdgeFaces(x)
			var q geom.Vector
			found := false
			for _, fe := range []*filletEdge{a, b} {
				for k := range 2 {
					if fe.f[k] == fx[0] || fe.f[k] == fx[1] {
						q, found = p.Plus(fe.into(k, p).ScaledBy(fe.ss[k])), true
					}
				}
			}
			if !found {
				return nil, filletErr(p, "an edge where two fillets meet smoothly lies between other faces")
			}
			for _, f := range fx {
				c.at[[2]int{x, f}] = q
			}
		}
		return c, nil
	}
	if c.kind >= 2 {
		for _, x := range round {
			if fe := &edges[byEdge[x]]; !fe.even() {
				return nil, filletErr(p, "a chamfer of two distances meets another chamfered edge here; "+
					"at a corner of chamfers the distances must be one")
			}
		}
	}
	if circular > 0 && c.kind == 2 {
		return nil, filletErr(p, "a circular edge's fillet meets another's at an angle; they must meet smoothly")
	}
	if circular > 0 && c.kind != 1 {
		return nil, filletErr(p, "a circular edge's fillet meets other fillets here")
	}
	if circular == 0 {
		for _, f := range t.facesAt(v) {
			if _, ok := t.planarNormal(f); !ok {
				return nil, filletErr(p, curvedEndMsg)
			}
		}
	}
	switch c.kind {
	case 1:
		fe := &edges[byEdge[round[0]]]
		along, end, err := t.endEdges(fe.e, v, fe.f)
		if err != nil {
			return nil, err
		}
		ne, ok := t.planarNormal(end)
		if !ok {
			return nil, filletErr(p, "the face at the edge's end is curved")
		}
		if fe.circ && (math.Abs(ne.Dot(fe.axis)) > 1e-9 || math.Abs(ne.Dot(fe.radial(p))) > 1e-9) {
			return nil, filletErr(p, "a circular edge's fillet ends on a face that is not square to it")
		}
		c.end = end
		for k := range 2 {
			q, err := t.pointAlong(along[k], v, fe.into(k, p), fe.ss[k], fe.concave)
			if err != nil {
				return nil, err
			}
			c.at[[2]int{fe.e, fe.f[k]}] = q
			c.at[[2]int{along[k], fe.f[k]}] = q
			c.at[[2]int{along[k], end}] = q
		}
	case 3:
		fs := t.facesAt(v)
		var n [3]geom.Vector
		var d [3]float64
		for i, f := range fs {
			n[i], _ = t.planarNormal(f)
			d[i] = n[i].Dot(p) - sg*r
		}
		if math.Abs(n[0].Dot(n[1].Cross(n[2]))) < 1e-9 {
			return nil, filletErr(p, "the faces at the corner don't meet in a point")
		}
		c.centre = meet3(n[0], d[0], n[1], d[1], n[2], d[2])
		for i, f := range fs {
			tp := c.centre.Plus(n[i].ScaledBy(sg * r))
			for _, x := range t.Vertices[v].Edges {
				efs := t.EdgeFaces(x)
				if efs[0] == f || efs[1] == f {
					c.at[[2]int{x, f}] = tp
				}
			}
		}
	case 2:
		a, b := &edges[byEdge[round[0]]], &edges[byEdge[round[1]]]
		z := plain[0]
		top := -1
		for _, f := range a.f {
			if f == b.f[0] || f == b.f[1] {
				top = f
			}
		}
		if top < 0 {
			return nil, filletErr(p, "two rounded edges meet here without a face between them")
		}
		// The tangent line of each edge on its other face reaches z.
		var q [2]geom.Vector
		var side [2]int
		for i, fe := range []*filletEdge{a, b} {
			k := 0
			if fe.f[0] == top {
				k = 1
			}
			side[i] = fe.f[k]
			var err error
			if q[i], err = t.pointAlong(z, v, fe.u[k], fe.ss[k], false); err != nil {
				return nil, err
			}
		}
		nt, _ := t.planarNormal(top)
		n1, _ := t.planarNormal(side[0])
		n2, _ := t.planarNormal(side[1])
		c.centre = meet3(nt, nt.Dot(p)-sg*r, n1, n1.Dot(p)-sg*r, n2, n2.Dot(p)-sg*r)
		tt := c.centre.Plus(nt.ScaledBy(sg * r))
		if !q[0].Equals(q[1]) {
			if err := t.unevenMitre(c, v, [2]*filletEdge{a, b}, top, side, q, sg*r); err != nil {
				return nil, err
			}
			return c, nil
		}
		// The plane through the centre that halves the two cylinders and
		// holds the mitre's ends.
		m := q[0].Minus(c.centre).Cross(tt.Minus(c.centre))
		if m.Magnitude() < 1e-12 {
			return nil, filletErr(p, "the mitre between the two fillets is degenerate")
		}
		c.mitreN = m.WithMagnitude(1)
		c.at[[2]int{a.e, top}], c.at[[2]int{b.e, top}] = tt, tt
		c.at[[2]int{a.e, side[0]}], c.at[[2]int{z, side[0]}] = q[0], q[0]
		c.at[[2]int{b.e, side[1]}], c.at[[2]int{z, side[1]}] = q[1], q[1]
	}
	return c, nil
}

// unevenMitre works out the corner where two fillets meet whose side
// faces don't lie alike about them, their sections turning by different
// angles. Their cylinders, of one radius, have axes that meet at the
// centre, so they still meet along an ellipse in the plane that halves
// the edges, through the centre and square to the top; but the strip
// that turns less, A, reaches its side face's tangent line there before
// the other, B, does. B goes on past the ellipse, cut by A's side face
// along a conic, to where its own tangent line meets the third edge.
// The third edge may be of the other kind than the two, as where the
// fillets at a rib's foot meet below its sharp edge: the ellipse then
// lies outside both side faces, and A's grows to it (roadmap 22.4).
// q are where each edge's tangent line on its side face meets the third
// edge; sr is the radius, negative for concave edges.
func (t *Topology) unevenMitre(c *filletCorner, v int, fes [2]*filletEdge, top int, side [2]int, q [2]geom.Vector, sr float64) error {
	p := t.Vertices[v].P
	ia, ib := 0, 1
	if fes[0].w < fes[1].w {
		ia, ib = 1, 0
	}
	A, B := fes[ia], fes[ib]
	away := func(fe *filletEdge) geom.Vector {
		if fe.v[1] == v {
			return fe.dir.ScaledBy(-1)
		}
		return fe.dir
	}
	nt, _ := t.planarNormal(top)
	na, _ := t.planarNormal(side[ia])
	nb, _ := t.planarNormal(side[ib])
	m := away(A).Plus(away(B)).Cross(nt)
	if m.Magnitude() < 1e-9 {
		return filletErr(p, "the mitre between the two fillets is degenerate")
	}
	c.mitreN = m.WithMagnitude(1)
	tt := c.centre.Plus(nt.ScaledBy(sr))
	// A's tangent line on its side meets the mitre's plane at qa, which
	// lies further from the vertex than where the line meets the third
	// edge.
	la := c.centre.Plus(na.ScaledBy(sr))
	qa := la.Plus(A.dir.ScaledBy(c.centre.Minus(la).Dot(c.mitreN) / A.dir.Dot(c.mitreN)))
	w := q[ib]
	// Where the third edge is of the fillets' kind, the mitre lies over
	// the two side faces, and A's side face is cut back to qa, short of
	// where its tangent line meets the third edge. Where it is of the
	// other kind, as the sharp edge of a rib whose foot is filled, the
	// mitre lies outside them both: qa is past the third edge, and A's
	// side face grows out to it, B's strip ending on the face along the
	// conic as before.
	inside := qa.Minus(q[ia]).Dot(away(A))
	if t.otherKind(c.plainEdge(t, v, A.e, B.e), A) {
		inside = -inside
	}
	if inside < 10*geom.LengthEps {
		return filletErr(p, "two fillets meet at a corner whose faces lie too unlike about them for a mitre")
	}
	// The conic in A's side face on B's cylinder, from qa to w: tangent at
	// qa to both, and at w to the third edge, where B's cylinder touches
	// its own side face.
	radial := func(x geom.Vector) geom.Vector {
		d := x.Minus(c.centre)
		return d.Minus(B.dir.ScaledBy(d.Dot(B.dir)))
	}
	t0 := na.Cross(radial(qa)).WithMagnitude(1)
	t1 := na.Cross(nb).WithMagnitude(1)
	ctrl, ok := lineMeet(qa, t0, w, t1)
	if !ok {
		return filletErr(p, "the mitre between the two fillets is degenerate")
	}
	// The weight from the conic's middle, on the line from the chord's
	// middle to the control point: B's cylinder there.
	mc := qa.Plus(w).ScaledBy(0.5)
	d0, d1 := radial(mc), radial(ctrl).Minus(radial(mc))
	qa2, qb2, qc2 := d1.Dot(d1), 2*d0.Dot(d1), d0.Dot(d0)-sr*sr
	disc := qb2*qb2 - 4*qa2*qc2
	if qa2 < 1e-18 || disc < 0 {
		return filletErr(p, "the mitre between the two fillets is degenerate")
	}
	k := -1.0
	for _, x := range []float64{(-qb2 - math.Sqrt(disc)) / (2 * qa2), (-qb2 + math.Sqrt(disc)) / (2 * qa2)} {
		if x > 1e-9 && x < 1-1e-9 {
			k = x
		}
	}
	if k < 0 {
		return filletErr(p, "the mitre between the two fillets is degenerate")
	}
	c.conic = arc(qa, ctrl, w, k/(1-k))
	c.long, c.cutFace = B.e, side[ia]
	c.at[[2]int{A.e, top}], c.at[[2]int{B.e, top}] = tt, tt
	c.at[[2]int{A.e, side[ia]}] = qa
	c.at[[2]int{B.e, side[ib]}] = w
	for _, f := range t.EdgeFaces(c.plainEdge(t, v, A.e, B.e)) {
		c.at[[2]int{c.plainEdge(t, v, A.e, B.e), f}] = w
	}
	c.shared = A.connector(t, v, c, tt, qa, false)
	return nil
}

// otherKind reports whether edge x, the one left sharp at a corner of
// two fillets, is of the other kind than the rounded edge fe: convex
// where the fillets fill, or concave where they cut.
func (t *Topology) otherKind(x int, fe *filletEdge) bool {
	xe, err := t.filletEdgeOf(x, fe.r, false)
	return err == nil && xe.concave != fe.concave
}

// plainEdge is the edge at vertex v other than a and b.
func (c *filletCorner) plainEdge(t *Topology, v, a, b int) int {
	for _, x := range t.Vertices[v].Edges {
		if x != a && x != b {
			return x
		}
	}
	return -1
}

// lineMeet is where the line through a along da meets the one through b
// along db, which lie in one plane.
func lineMeet(a, da, b, db geom.Vector) (geom.Vector, bool) {
	n := da.Cross(db)
	if n.Magnitude() < 1e-12 {
		return geom.Vector{}, false
	}
	k := b.Minus(a).Cross(db).Dot(n) / n.Dot(n)
	return a.Plus(da.ScaledBy(k)), true
}

// connectors are the curves across the edge's rounded face at vertex v,
// from p to q: one, but for the strip that goes on past an uneven
// mitre, which runs along the mitre and then the conic.
func (fe *filletEdge) connectors(t *Topology, v int, c *filletCorner, p, q geom.Vector, flat bool) []srf.SBezier {
	if c.kind == setbackCorner {
		sb := c.arcs[fe.e]
		if !sb.Start().Equals(p) {
			sb.Reverse()
		}
		return []srf.SBezier{sb}
	}
	if c.kind != 2 || c.long != fe.e || flat {
		return []srf.SBezier{fe.connector(t, v, c, p, q, flat)}
	}
	chain := []srf.SBezier{c.shared, c.conic}
	if !chain[0].Start().Equals(p) {
		chain[0].Reverse()
		chain[1].Reverse()
		chain[0], chain[1] = chain[1], chain[0]
	}
	return chain
}

// connector is the curve across the edge's rounded face at vertex v,
// from p to q: the arc it ends along, or with flat a line.
func (fe *filletEdge) connector(t *Topology, v int, c *filletCorner, p, q geom.Vector, flat bool) srf.SBezier {
	vp := t.Vertices[v].P
	if flat {
		return srf.SBezierFrom2(p, q)
	}
	// The cross section's control point is where the section's plane
	// meets the edge; across, the arc between the tangent points.
	switch c.kind {
	case 1, seam:
		// At the vertex, or projected along the edge onto the face it
		// ends on: its control point the vertex.
		return arc(p, vp, q, fe.w)
	case 3:
		x := vp.Plus(fe.dir.ScaledBy(c.centre.Minus(vp).Dot(fe.dir)))
		return arc(p, x, q, fe.w)
	}
	// A mitre: the section at the centre, projected along the edge onto
	// the mitre's plane.
	x := vp.Plus(fe.dir.ScaledBy(c.centre.Minus(vp).Dot(fe.dir)))
	onto := func(a geom.Vector) geom.Vector {
		k := c.centre.Minus(a).Dot(c.mitreN) / fe.dir.Dot(c.mitreN)
		return a.Plus(fe.dir.ScaledBy(k))
	}
	sb := arc(p, onto(x), q, fe.w)
	sb.Ctrl[0], sb.Ctrl[2] = p, q
	return sb
}

// filletLoops are face f's loops with the corners the fillets reach
// moved: each co-edge runs between the points its ends move to, a
// rounded edge along its tangent curve, and a face a fillet ends on
// takes the curve between the two points it has at that vertex.
func (t *Topology) filletLoops(f int, edges []filletEdge, byEdge map[int]int, corners map[int]*filletCorner, flat bool) ([][]srf.SBezier, error) {
	point := func(x, v int) geom.Vector {
		if c := corners[v]; c != nil {
			if q, ok := c.at[[2]int{x, f}]; ok {
				return q
			}
		}
		return t.Vertices[v].P
	}
	var loops [][]srf.SBezier
	for _, l := range t.outerFirst(f) {
		ces := t.Loops[l].CoEdges
		var loop []srf.SBezier
		for i, ce := range ces {
			x := t.CoEdges[ce].Edge
			va, vb := t.Start(ce), t.End(ce)
			a, b := point(x, va), point(x, vb)
			moved := !a.Equals(t.Vertices[va].P) || !b.Equals(t.Vertices[vb].P)
			ri, round := byEdge[x]
			switch {
			case round:
				fe := &edges[ri]
				k := 0
				if fe.f[1] == f {
					k = 1
				}
				if va == fe.v[0] {
					loop = append(loop, fe.tangentCurve(k, a, b))
				} else {
					sb := fe.tangentCurve(k, b, a)
					sb.Reverse()
					loop = append(loop, sb)
				}
			case t.Straight(x):
				loop = append(loop, srf.SBezierFrom2(a, b))
			case moved:
				return nil, filletErr(a, "a curved edge of a face at the fillet would have to be cut")
			default:
				sb, err := t.edgeCurve(ce)
				if err != nil {
					return nil, err
				}
				loop = append(loop, sb)
			}
			// At the vertex the co-edge ends at, the end of the rounded face
			// when the face is one a fillet ends on.
			nx := t.CoEdges[ces[(i+1)%len(ces)]].Edge
			if c := corners[vb]; c != nil && c.kind == 2 && c.long >= 0 && c.cutFace == f {
				// The conic where an uneven mitre's longer strip cuts this face.
				p, q := point(x, vb), point(nx, vb)
				sb := c.conic
				if !sb.Start().Equals(p) {
					sb.Reverse()
				}
				if !p.Equals(q) {
					loop = append(loop, sb)
				}
			}
			if c := corners[vb]; c != nil && c.kind == setbackCorner {
				// The face's curve between the two fillets, where the hole's
				// patches meet it.
				p, q := point(x, vb), point(nx, vb)
				if sb, ok := c.faceCurves[f]; ok && !p.Equals(q) {
					if !sb.Start().Equals(p) {
						sb.Reverse()
					}
					loop = append(loop, sb)
				}
			}
			if c := corners[vb]; c != nil && c.kind == 1 && c.end == f {
				p, q := point(x, vb), point(nx, vb)
				for _, fe := range edges {
					if fe.v[0] == vb || fe.v[1] == vb {
						loop = append(loop, fe.connector(t, vb, c, p, q, flat))
						break
					}
				}
			}
		}
		loops = append(loops, loop)
	}
	return loops, nil
}

// edgeCurve is the exact curve of co-edge ce, run its way: the shell's
// curve, when it spans the edge.
func (t *Topology) edgeCurve(ce int) (srf.SBezier, error) {
	ed := &t.Edges[t.CoEdges[ce].Edge]
	a, b := t.Vertices[t.Start(ce)].P, t.Vertices[t.End(ce)].P
	c := t.shell.FindCurve(ed.Curve)
	if c == nil || !c.IsExact {
		// A Boolean's curve, which the face keeps: the arc through its
		// points stands for it, and the face takes the curve again as an
		// edge that is open.
		pts := ed.Points
		if !pts[0].Equals(a) {
			pts = reversed(pts)
		}
		if len(pts) >= 3 {
			n := pts[len(pts)/2].Minus(pts[0]).Cross(pts[len(pts)-1].Minus(pts[0]))
			if n.Magnitude() > 1e-12 {
				if sb, ok := arcThrough(pts, n.WithMagnitude(1)); ok {
					return sb, nil
				}
			}
		}
		return srf.SBezier{}, filletErr(a, "a curved edge of a face at the fillet has no exact curve")
	}
	sb := c.Exact
	switch {
	case sb.Start().Equals(a) && sb.Finish().Equals(b):
	case sb.Start().Equals(b) && sb.Finish().Equals(a):
		sb.Reverse()
	default:
		return srf.SBezier{}, filletErr(a, "a curved edge of a face at the fillet is part of a longer curve")
	}
	return sb, nil
}

// chainCurves orders curves into a closed loop, reversing those that
// run the other way.
func chainCurves(cs []srf.SBezier) ([]srf.SBezier, error) {
	out := []srf.SBezier{cs[0]}
	used := make([]bool, len(cs))
	used[0] = true
	for len(out) < len(cs) {
		end := out[len(out)-1].Finish()
		found := false
		for i, c := range cs {
			if used[i] {
				continue
			}
			if c.Start().Equals(end) {
				out, used[i], found = append(out, c), true, true
				break
			}
			if c.Finish().Equals(end) {
				c.Reverse()
				out, used[i], found = append(out, c), true, true
				break
			}
		}
		if !found {
			return nil, filletErr(end, "no arc goes on from here")
		}
	}
	if !out[len(out)-1].Finish().Equals(out[0].Start()) {
		return nil, filletErr(out[0].Start(), "the arcs don't close")
	}
	return out, nil
}

// spherePatch is a patch of the sphere about c of radius r over the
// spherical triangle the loop bounds: a surface of revolution about an
// axis through c square to the triangle's middle, so that neither pole
// lies on it, covering the triangle with a margin.
func spherePatch(c geom.Vector, r float64, loop []srf.SBezier) srf.SSurface {
	var pts []geom.Vector
	for _, sb := range loop {
		for k := range 32 {
			pts = append(pts, sb.PointAt(float64(k)/32).Minus(c).WithMagnitude(1))
		}
	}
	var mid geom.Vector
	for _, p := range pts {
		mid = mid.Plus(p)
	}
	mid = mid.WithMagnitude(1)
	axis := mid.NormalDir(0).WithMagnitude(1)
	e0 := mid.Minus(axis.ScaledBy(mid.Dot(axis))).WithMagnitude(1)
	e1 := axis.Cross(e0)
	// The triangle's range of angle from the axis and about it.
	th0, th1 := math.Inf(1), math.Inf(-1)
	ph0, ph1 := math.Inf(1), math.Inf(-1)
	for _, p := range pts {
		th := math.Acos(math.Max(-1, math.Min(1, p.Dot(axis))))
		ph := math.Atan2(p.Dot(e1), p.Dot(e0))
		th0, th1 = math.Min(th0, th), math.Max(th1, th)
		ph0, ph1 = math.Min(ph0, ph), math.Max(ph1, ph)
	}
	const margin = 0.05
	th0, th1 = math.Max(th0-margin, 1e-3), math.Min(th1+margin, math.Pi-1e-3)
	ph0, ph1 = ph0-margin, ph1+margin
	// The meridian at angle 0 about the axis, from th0 to th1.
	at := func(th float64) geom.Vector {
		return c.Plus(axis.ScaledBy(r * math.Cos(th))).Plus(e0.ScaledBy(r * math.Sin(th)))
	}
	half := (th1 - th0) / 2
	ctrl := c.Plus(axis.ScaledBy(r * math.Cos(th0+half) / math.Cos(half))).
		Plus(e0.ScaledBy(r * math.Sin(th0+half) / math.Cos(half)))
	meridian := arc(at(th0), ctrl, at(th1), math.Cos(half))
	// RotatedAbout turns e0 towards e1 for a positive angle about axis.
	sign := 1.0
	if e0.RotatedAroundAxis(axis, 0.1).Dot(e1) < 0 {
		sign = -1
	}
	return srf.FromRevolutionOf(&meridian, c, axis, sign*ph0, sign*ph1, 0, 0)
}
