package ui

import (
	"cmp"
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/topo"
)

// The scenario commands that click a face or an edge of the active
// group's solid by its name, as the interface writes it, wherever the
// view puts it:
//
//	click-edge NAME [shift] [ctrl] [right]   an edge the solid has and no entity draws:
//	                                 "the edge between top face of g003-block and face 2 of g004-rounded"
//	move-edge NAME                   the pointer over it, no click
//	click-face NAME [shift] [ctrl] [right]   a face: "top face of g003-block", "face 4 of g003-lathe"
//	move-face NAME
//
// right clicks the right button, which opens the context menu (queue
// its popup line before). A name that finds nothing prints "edge missing NAME" or "face
// missing NAME"; one found that no place in the view picks prints
// "edge unpicked NAME" or "face unpicked NAME", and the command clicks
// nowhere.

// solidClickStep runs click-edge, move-edge, click-face and move-face;
// false when t is none of them.
func (r *uiScriptRunner) solidClickStep(t []string) bool {
	kind, click := "", false
	switch t[0] {
	case "click-edge", "move-edge":
		kind, click = "edge", t[0] == "click-edge"
	case "click-face", "move-face":
		kind, click = "face", t[0] == "click-face"
	default:
		return false
	}
	name, mods, button := nameAndMods(t[1:])
	if name == "" {
		r.t.Fatalf("%s: no %s named", t[0], kind)
	}
	var xs, ys string
	var ok bool
	if kind == "edge" {
		xs, ys, ok = r.edgePickPoint(name)
	} else {
		xs, ys, ok = r.facePickPoint(name)
	}
	if !ok {
		return true
	}
	r.replay(false, append([]string{"move", xs, ys}, mods...))
	if click {
		r.replay(false, append([]string{"down", button, xs, ys}, mods...))
		r.replay(false, append([]string{"up", button, xs, ys}, mods...))
	}
	return true
}

// nameAndMods splits a click command's words into the name, which may
// hold spaces, the modifier keys at its end, and the button: R where
// "right" is among them, else L.
func nameAndMods(words []string) (name string, mods []string, button string) {
	button = "L"
	for len(words) > 0 {
		switch w := words[len(words)-1]; w {
		case "shift", "ctrl":
			mods = append([]string{w}, mods...)
		case "right":
			button = "R"
		default:
			return strings.Join(words, " "), mods, button
		}
		words = words[:len(words)-1]
	}
	return "", mods, button
}

// edgePickPoint is where to click the active solid's edge named name:
// the first of the clickAlong places along any of its pieces where the
// pointer hovers it, or else the first place beside one.
func (r *uiScriptRunner) edgePickPoint(name string) (xs, ys string, ok bool) {
	tp := r.ss.GW.activeTopology()
	var want topo.Name
	var pieces [][]geom.Vector
	if tp != nil {
		for e := range tp.Edges {
			if tp.Edges[e].Seam || len(tp.Edges[e].Points) < 2 {
				continue
			}
			n, err := tp.NameOf(tp.Lead(e))
			if err != nil || r.ss.SK.SolidEdgeName(n) != name {
				continue
			}
			want = n
			pieces = append(pieces, tp.Edges[e].Points)
		}
	}
	if len(pieces) == 0 {
		fmt.Fprintf(&r.b, "edge missing %s\n", name)
		return "", "", false
	}
	picks := func(x, y float64) bool {
		xs, ys = fmtXY(x, y)
		r.mouse(r.gw(), []string{"move", xs, ys})
		return r.ss.GW.hover.Edge == want
	}
	for _, pts := range pieces {
		for _, f := range clickAlong {
			if picks(r.screenXY(alongPolyline(pts, f))) {
				return xs, ys, true
			}
		}
	}
	const steps = 20
	for _, d := range pickAside {
		for _, pts := range pieces {
			for i := 1; i < steps; i++ {
				ax, ay := r.screenXY(alongPolyline(pts, float64(i-1)/steps))
				bx, by := r.screenXY(alongPolyline(pts, float64(i+1)/steps))
				mx, my := r.screenXY(alongPolyline(pts, float64(i)/steps))
				tx, ty := bx-ax, by-ay
				l := math.Hypot(tx, ty)
				if l > 0 && picks(mx-ty/l*d, my+tx/l*d) {
					return xs, ys, true
				}
			}
		}
	}
	fmt.Fprintf(&r.b, "edge unpicked %s\n", name)
	return "", "", false
}

// alongPolyline is the point fraction f of the way along pts, by
// length.
func alongPolyline(pts []geom.Vector, f float64) geom.Vector {
	total := 0.0
	for i := 1; i < len(pts); i++ {
		total += pts[i].Minus(pts[i-1]).Magnitude()
	}
	at := f * total
	for i := 1; i < len(pts); i++ {
		l := pts[i].Minus(pts[i-1]).Magnitude()
		if at <= l && l > 0 {
			return pts[i-1].Plus(pts[i].Minus(pts[i-1]).ScaledBy(at / l))
		}
		at -= l
	}
	return pts[len(pts)-1]
}

// facePickPoint is where to click the active group's face named name:
// the middle of the first of its triangles, largest on the screen
// first, where the pointer hovers the face.
func (r *uiScriptRunner) facePickPoint(name string) (xs, ys string, ok bool) {
	sk, gw := r.ss.SK, r.ss.GW
	var id srf.FaceID
	if e := r.findEntity(name); e != nil {
		id = srf.FaceID(e.H.V)
	}
	m := &sk.Group(gw.activeGroupH()).DisplayMesh
	type tri struct {
		x, y, area float64
	}
	var tris []tri
	named := map[srf.FaceID]bool{}
	for i := range m.Triangles {
		tr := &m.Triangles[i]
		f := tr.Meta.Face
		if id == 0 {
			if _, seen := named[f]; !seen {
				named[f] = f != 0 && sk.FaceName(f) == name
			}
			if !named[f] {
				continue
			}
		} else if f != id {
			continue
		}
		ax, ay := r.screenXY(tr.A)
		bx, by := r.screenXY(tr.B)
		cx, cy := r.screenXY(tr.C)
		area := math.Abs((bx-ax)*(cy-ay)-(cx-ax)*(by-ay)) / 2
		tris = append(tris, tri{(ax + bx + cx) / 3, (ay + by + cy) / 3, area})
	}
	if len(tris) == 0 {
		fmt.Fprintf(&r.b, "face missing %s\n", name)
		return "", "", false
	}
	slices.SortStableFunc(tris, func(a, b tri) int { return cmp.Compare(b.area, a.area) })
	for _, t := range tris {
		xs, ys = fmtXY(t.x, t.y)
		r.mouse(r.gw(), []string{"move", xs, ys})
		h := gw.hover
		if (id != 0 && h.Entity.V == uint32(id)) || (h.Face != 0 && sk.FaceName(h.Face) == name) {
			return xs, ys, true
		}
	}
	fmt.Fprintf(&r.b, "face unpicked %s\n", name)
	return "", "", false
}
