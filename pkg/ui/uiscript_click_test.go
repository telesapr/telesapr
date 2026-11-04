package ui

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
)

// The scenario commands that click what they name, not a pixel: a
// script that clicks by name keeps working when the layout, the view or
// the text window's rows move. They send the same events a move, down
// and up at the found point would, so a golden made with coordinates
// stays the same when the point found is the one the coordinates hit.
//
//	click-entity NAME [shift] [ctrl]   the entity, by handle (hex) or name
//	move-entity NAME                   the pointer over it, no click
//	click-tw [#N] TEXT                 the text window's link showing TEXT

// clickStep runs a click-by-name command; false when t is not one.
func (r *uiScriptRunner) clickStep(t []string, rest string) bool {
	switch t[0] {
	case "click-entity", "move-entity":
		// The name is the rest of the line, as a name may hold spaces,
		// less the modifiers at its end.
		words := t[1:]
		var mods []string
		for len(words) > 0 && (words[len(words)-1] == "shift" || words[len(words)-1] == "ctrl") {
			mods = append([]string{words[len(words)-1]}, mods...)
			words = words[:len(words)-1]
		}
		if len(words) == 0 {
			r.t.Fatalf("%s: no entity named", t[0])
		}
		xs, ys, ok := r.entityPickPoint(strings.Join(words, " "))
		if !ok {
			return true
		}
		r.replay(false, append([]string{"move", xs, ys}, mods...))
		if t[0] == "click-entity" {
			r.replay(false, append([]string{"down", "L", xs, ys}, mods...))
			r.replay(false, append([]string{"up", "L", xs, ys}, mods...))
		}
	case "click-tw":
		nth, text := 1, strings.TrimSpace(rest)
		if strings.HasPrefix(text, "#") {
			n, after, _ := strings.Cut(text[1:], " ")
			if v, err := strconv.Atoi(n); err == nil && v > 0 {
				nth, text = v, strings.TrimSpace(after)
			}
		}
		x, y, ok := r.twLinkPoint(text, nth)
		if !ok {
			fmt.Fprintf(&r.b, "click-tw missing %s\n", text)
			return true
		}
		xs, ys := strconv.FormatFloat(x, 'f', 2, 64), strconv.FormatFloat(y, 'f', 2, 64)
		r.replay(true, []string{"twmove", xs, ys})
		r.replay(true, []string{"twdown", "L", xs, ys})
		r.replay(true, []string{"twup", "L", xs, ys})
	default:
		return false
	}
	return true
}

// replay sends a mouse line as the script would have written it.
func (r *uiScriptRunner) replay(inTW bool, t []string) {
	r.noteMouse(t, inTW)
	if inTW {
		r.mouse(r.tw(), t)
	} else {
		r.mouse(r.gw(), t)
	}
}

// findEntity is the entity a script names: its handle in hex, as dump
// prints it, or its name as the interface shows it.
func (r *uiScriptRunner) findEntity(name string) *core.Entity {
	sk := r.ss.SK
	if v, err := strconv.ParseUint(name, 16, 32); err == nil {
		for i := range sk.Entities {
			if sk.Entities[i].H.V == uint32(v) {
				return &sk.Entities[i]
			}
		}
	}
	for i := range sk.Entities {
		if sk.EntityName(sk.Entities[i].H) == name {
			return &sk.Entities[i]
		}
	}
	return nil
}

// clickAlong is how far along a line, a circle or an arc click-entity
// tries to click it, in turn: the first place where the pointer picks
// the entity, not a point or a label lying on it.
var clickAlong = []float64{0.4, 0.3, 0.6, 0.2, 0.7, 0.1, 0.9, 0.5}

// entityModelPoint is a point on what the entity draws at fraction f of
// its length: a point itself; a line's, a circle's, an arc's or a
// cubic's point f along (a spline of several pieces, along its control
// points, near enough for the search beside it); another entity's
// reference point.
func (r *uiScriptRunner) entityModelPoint(e *core.Entity, f float64) (geom.Vector, bool) {
	gw, sk := r.ss.GW, r.ss.SK
	pt := func(i int) geom.Vector { return gw.pointGetDrawNum(sk.Entity(e.Point[i])) }
	switch {
	case e.IsPoint():
		return gw.pointGetDrawNum(e), true
	case e.Type == core.EntityLineSegment:
		a, b := pt(0), pt(1)
		return a.Plus(b.Minus(a).ScaledBy(f)), true
	case e.Type == core.EntityCircle || e.Type == core.EntityArcOfCircle:
		q := e.NormalEntity(sk).NormalGetNum(sk)
		c := pt(0)
		radius := e.CircleGetRadiusNum(sk)
		theta := 2 * math.Pi * f
		if e.Type == core.EntityArcOfCircle {
			ta, _, dt := e.ArcGetAngles(sk)
			theta = ta + dt*f
		}
		dir := q.RotationU().ScaledBy(math.Cos(theta)).Plus(q.RotationV().ScaledBy(math.Sin(theta)))
		return c.Plus(dir.ScaledBy(radius)), true
	case e.Type == core.EntityCubic && e.ExtraPoints == 0:
		p0, p1, p2, p3 := pt(0), pt(1), pt(2), pt(3)
		g := 1 - f
		return p0.ScaledBy(g * g * g).Plus(p1.ScaledBy(3 * g * g * f)).Plus(p2.ScaledBy(3 * g * f * f)).
			Plus(p3.ScaledBy(f * f * f)), true
	case e.Type == core.EntityCubic || e.Type == core.EntityCubicPeriodic:
		n := 4 + e.ExtraPoints
		if e.Type == core.EntityCubicPeriodic {
			n = 3 + e.ExtraPoints
		}
		at := f * float64(n-1)
		i := min(int(at), n-2)
		a, b := pt(i), pt(i+1)
		return a.Plus(b.Minus(a).ScaledBy(at - float64(i))), true
	}
	if pts := gw.entityReferencePoints(e); len(pts) > 0 {
		return pts[0], true
	}
	return geom.Vector{}, false
}

// screenXY is p in the graphics window's pixels from its top left.
func (r *uiScriptRunner) screenXY(p geom.Vector) (x, y float64) {
	s := r.ss.GW.ProjectPoint(p)
	width, height := r.ss.GW.window.ContentSize()
	return s.U + width/2, height/2 - s.V
}

// fmtXY is a point as a script writes it.
func fmtXY(x, y float64) (xs, ys string) {
	return strconv.FormatFloat(x, 'f', 2, 64), strconv.FormatFloat(y, 'f', 2, 64)
}

// pickAside is how far to either side of a curve, in pixels, the
// search for a place that picks it goes when no place on it does: an
// entity drawn under a point or another line is still picked beside it.
var pickAside = []float64{3, -3, 6, -6, 9, -9, 12, -12}

// entityPickPoint is where to click the named entity: the first of the
// clickAlong places where the pointer, moved there, hovers it, or else
// the first place beside it that does. With a command pending the
// pointer is not moved to look, and the first place is taken. A name
// that finds nothing, or no place that picks it, is written to the
// output, so the golden shows it.
func (r *uiScriptRunner) entityPickPoint(name string) (xs, ys string, ok bool) {
	e := r.findEntity(name)
	if e == nil {
		fmt.Fprintf(&r.b, "entity missing %s\n", name)
		return "", "", false
	}
	p, ok := r.entityModelPoint(e, clickAlong[0])
	if !ok {
		fmt.Fprintf(&r.b, "entity unpicked %s\n", name)
		return "", "", false
	}
	x0, y0 := r.screenXY(p)
	if r.ss.GW.pending.operation != 0 {
		xs, ys = fmtXY(x0, y0)
		return xs, ys, true
	}
	picks := func(x, y float64) bool {
		xs, ys = fmtXY(x, y)
		r.mouse(r.gw(), []string{"move", xs, ys})
		return r.ss.GW.hover.Entity == e.H
	}
	if e.IsPoint() {
		if picks(x0, y0) {
			return xs, ys, true
		}
	} else {
		for _, f := range clickAlong {
			p, _ := r.entityModelPoint(e, f)
			if picks(r.screenXY(p)) {
				return xs, ys, true
			}
		}
		const steps = 20
		for _, d := range pickAside {
			for i := 1; i < steps; i++ {
				pa, _ := r.entityModelPoint(e, float64(i-1)/steps)
				pb, _ := r.entityModelPoint(e, float64(i+1)/steps)
				pm, _ := r.entityModelPoint(e, float64(i)/steps)
				ax, ay := r.screenXY(pa)
				bx, by := r.screenXY(pb)
				mx, my := r.screenXY(pm)
				tx, ty := bx-ax, by-ay
				l := math.Hypot(tx, ty)
				if l == 0 {
					continue
				}
				if picks(mx-ty/l*d, my+tx/l*d) {
					return xs, ys, true
				}
			}
		}
	}
	fmt.Fprintf(&r.b, "entity unpicked %s\n", name)
	xs, ys = fmtXY(x0, y0)
	return xs, ys, true
}

// twLinkPoint is the middle of the nth link the text window shows whose
// text is text: a run of cells with the same link, its spaces and the
// glyphs of a radio button or a check box (the private use area) left
// out. In the window's pixels.
func (r *uiScriptRunner) twLinkPoint(text string, nth int) (x, y float64, ok bool) {
	tw := r.ss.TW
	found := 0
	for row := range tw.rows {
		line, meta := tw.lines[row].text[:], tw.lines[row].meta[:]
		for c0 := 0; c0 < len(line); {
			c1 := c0 + 1
			for c1 < len(line) && meta[c1].link == meta[c0].link && meta[c1].data == meta[c0].data {
				c1++
			}
			if meta[c0].link != 0 && linkText(line[c0:c1]) == text {
				found++
				if found == nth {
					hh := float64(lineHeight / 2)
					x = leftMargin + float64(c0+c1)/2*charWidth
					y = (float64(tw.lineTop(row))+1)*hh - float64(tw.scrollPos)*hh
					return x, y, true
				}
			}
			c0 = c1
		}
	}
	return 0, 0, false
}

// linkText is a link's text as click-tw names it.
func linkText(cells []rune) string {
	return strings.TrimFunc(string(cells), func(c rune) bool {
		return c == ' ' || (c >= 0xe000 && c <= 0xf8ff)
	})
}
