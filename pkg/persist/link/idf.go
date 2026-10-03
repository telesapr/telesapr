package link

import (
	"fmt"
	"math"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/libm"
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/srf"
)

// This file ports solvespace/src/importidf.cpp, which links the board
// outline of an Intermediate Data Format (IDF) file: its outline,
// cutouts, routing keepouts and drilled holes become entities, and the
// board an extruded shell. Part placement isn't supported, as in C++.

// splitIDFString splits a line at spaces, except within quotes (C++
// splitString).
func splitIDFString(line string) []string {
	var v []string

	if len(line) == 0 {
		return v
	}

	s := ""
	inString := false
	inQuotes := false

	for i := 0; i < len(line); i++ {
		c := line[i]
		if inQuotes {
			if c != '"' {
				s += string(c)
			} else {
				v = append(v, s)
				inQuotes = false
				inString = false
				s = ""
			}
		} else if inString {
			if c != ' ' {
				s += string(c)
			} else {
				v = append(v, s)
				inString = false
				s = ""
			}
		} else if c == '"' {
			inString = true
			inQuotes = true
		} else if c != ' ' {
			s = string(c)
			inString = true
		}
	}
	if len(s) > 0 {
		v = append(v, s)
	}

	return v
}

var (
	cFloatPrefix = regexp.MustCompile(`^[ \t\n\v\f\r]*[+-]?(\d+\.?\d*([eE][+-]?\d+)?|\.\d+([eE][+-]?\d+)?)`)
	cIntPrefix   = regexp.MustCompile(`^[ \t\n\v\f\r]*[+-]?\d+`)
)

// stof is C++ std::stof: the float that starts s. C++ throws, and so
// stops, where this returns an error.
func stof(s string) (float64, error) {
	return parseCPrefix(s, cFloatPrefix, 32)
}

// stod is C++ std::stod.
func stod(s string) (float64, error) {
	return parseCPrefix(s, cFloatPrefix, 64)
}

func parseCPrefix(s string, re *regexp.Regexp, bits int) (float64, error) {
	m := re.FindString(s)
	if m == "" {
		return 0, fmt.Errorf("no number in %q", s)
	}
	return strconv.ParseFloat(strings.TrimLeft(m, " \t\n\v\f\r"), bits)
}

// stoi is C++ std::stoi.
func stoi(s string) (int, error) {
	m := cIntPrefix.FindString(s)
	if m == "" {
		return 0, fmt.Errorf("no number in %q", s)
	}
	v, err := strconv.ParseInt(strings.TrimLeft(m, " \t\n\v\f\r"), 10, 32)
	return int(v), err
}

func isHoleDuplicate(el []core.Entity, x, y, r float64) bool {
	find := func(h core.HEntity) *core.Entity {
		for i := range el {
			if el[i].H == h {
				return &el[i]
			}
		}
		return nil
	}
	duplicate := false
	for i := 0; i < len(el) && !duplicate; i++ {
		en := &el[i]
		if en.Type != core.EntityCircle {
			continue
		}
		distance := find(en.Distance)
		center := find(en.Point[0])
		duplicate = center.ActPoint.X == x && center.ActPoint.Y == y && distance.ActDistance == r
	}
	return duplicate
}

// The entities made here are incomplete: points have no params, only
// their actPoint, and so on. That's fine, since the linked group copies
// them.

func idfNewPoint(el *[]core.Entity, id *int, p geom.Vector, visible bool) core.HEntity {
	en := core.Entity{}
	en.Type = core.EntityPointNCopy
	en.Group = core.HGroup{V: 462}
	en.ActPoint = p
	en.Style = core.HStyle{V: core.StyleDatum}
	en.ActVisible = visible

	*id++
	en.H = core.HEntity{V: uint32(*id) + en.Group.V*65536}
	*el = append(*el, en)
	return en.H
}

func idfStyle(keepout bool) core.HStyle {
	if keepout {
		return core.HStyle{V: core.StyleConstruction}
	}
	return core.HStyle{V: core.StyleActiveGrp}
}

func idfNewLine(el *[]core.Entity, id *int, p0, p1 core.HEntity, keepout bool) core.HEntity {
	en := core.Entity{}
	en.Type = core.EntityLineSegment
	en.Point[0] = p0
	en.Point[1] = p1
	en.Group = core.HGroup{V: 493}
	en.Construction = keepout
	en.Style = idfStyle(keepout)
	en.ActVisible = true

	*id++
	en.H = core.HEntity{V: uint32(*id) + en.Group.V*65536}
	*el = append(*el, en)
	return en.H
}

func idfNewNormal(el *[]core.Entity, id *int, normal geom.Quaternion) core.HEntity {
	en := core.Entity{}
	en.Type = core.EntityNormalNCopy
	en.Group = core.HGroup{V: 472}
	en.ActNormal = normal
	en.Style = core.HStyle{V: core.StyleActiveGrp}
	// to be visible we need to add a point.
	en.Point[0] = idfNewPoint(el, id, geom.Vector{Z: 3}, true)
	en.ActVisible = true

	*id++
	en.H = core.HEntity{V: uint32(*id) + en.Group.V*65536}
	*el = append(*el, en)
	return en.H
}

func idfNewArc(el *[]core.Entity, id *int, p0, p1, pc, hnorm core.HEntity, keepout bool) core.HEntity {
	en := core.Entity{}
	en.Type = core.EntityArcOfCircle
	en.Point[0] = pc
	en.Point[1] = p0
	en.Point[2] = p1
	en.Normal = hnorm
	en.Group = core.HGroup{V: 403}
	en.Construction = keepout
	en.Style = idfStyle(keepout)
	en.ActVisible = true
	// C++ increments the id twice here.
	*id++

	*id++
	en.H = core.HEntity{V: uint32(*id) + en.Group.V*65536}
	*el = append(*el, en)
	return en.H
}

func idfNewDistance(el *[]core.Entity, id *int, distance float64) core.HEntity {
	en := core.Entity{}
	en.Type = core.EntityDistance
	en.Group = core.HGroup{V: 472}
	en.ActDistance = distance
	en.Style = core.HStyle{V: core.StyleActiveGrp}
	en.ActVisible = false

	*id++
	en.H = core.HEntity{V: uint32(*id) + en.Group.V*65536}
	*el = append(*el, en)
	return en.H
}

func idfNewCircle(el *[]core.Entity, id *int, p0, hdist, hnorm core.HEntity, keepout bool) core.HEntity {
	en := core.Entity{}
	en.Type = core.EntityCircle
	en.Point[0] = p0
	en.Normal = hnorm
	en.Distance = hdist
	en.Group = core.HGroup{V: 399}
	en.Construction = keepout
	en.Style = idfStyle(keepout)
	en.ActVisible = true

	*id++
	en.H = core.HEntity{V: uint32(*id) + en.Group.V*65536}
	*el = append(*el, en)
	return en.H
}

// arcCenter is the centre of the arc from p0 to p1 through angle
// degrees (C++ ArcCenter).
func arcCenter(p0, p1 geom.Vector, angle float64) geom.Vector {
	m := p0.Plus(p1).ScaledBy(0.5)
	perp := geom.Vector{X: p1.Y - p0.Y, Y: p0.X - p1.X}.WithMagnitude(1.0)
	dist := 0.0
	if angle != 180 {
		dist = p1.Minus(m).Magnitude() / libm.Tan(0.5*angle*3.141592653589793/180.0)
	} else {
		dist = 0.0
	}
	c := m.Minus(perp.ScaledBy(dist))
	return c
}

// createIDFEntity adds an IDF line or arc. A zero angle is a line;
// positive angles are counter clockwise, negative clockwise; 360 is a
// circle centred at p0 through p1 (C++ CreateEntity).
func createIDFEntity(el *[]core.Entity, id *int, h0, h1, hnorm core.HEntity,
	p0, p1 geom.Vector, angle float64, keepout bool) {
	if math.Abs(angle) < 0.1 {
		// line
		if p0.Equals(p1) {
			return
		}

		idfNewLine(el, id, h0, h1, keepout)

	} else if angle == 360.0 {
		// circle
		d := p1.Minus(p0).Magnitude()
		hd := idfNewDistance(el, id, d)
		idfNewCircle(el, id, h1, hd, hnorm, keepout)

	} else {
		// arc
		if angle < 0.0 {
			p0, p1 = p1, p0
			h0, h1 = h1, h0
			angle = math.Abs(angle)
		}
		// locate the center of the arc
		m := p0.Plus(p1).ScaledBy(0.5)
		perp := geom.Vector{X: p1.Y - p0.Y, Y: p0.X - p1.X}.WithMagnitude(1.0)
		// half angle in radians
		theta := 0.5 * angle * math.Pi / 180.0
		dist := p1.Minus(m).Magnitude() * libm.Cos(theta) / libm.Sin(theta)
		c := m.Minus(perp.ScaledBy(dist))
		hc := idfNewPoint(el, id, c, false)
		idfNewArc(el, id, h0, h1, hc, hnorm, keepout)
	}
}

// makeBeziersForArcs adds the arc about center from pa through angle
// degrees, as Entity::GenerateBezierCurves would (C++
// MakeBeziersForArcs).
func makeBeziersForArcs(sbl *srf.SBezierList, center, pa, pb geom.Vector,
	q geom.Quaternion, angle float64) {
	u, v := q.RotationU(), q.RotationV()
	r := pa.Minus(center).Magnitude()
	var theta, dtheta float64

	if angle == 360.0 {
		theta = 0
	} else {
		c2 := center.Project2d(u, v)
		pa2 := pa.Project2d(u, v).Minus(c2)

		theta = libm.Atan2(pa2.V, pa2.U)
	}
	dtheta = angle * math.Pi / 180

	var n int
	if dtheta > (3*math.Pi/2 + 0.01) {
		n = 4
	} else if dtheta > (geom.Pi + 0.01) {
		n = 3
	} else if dtheta > (math.Pi/2 + 0.01) {
		n = 2
	} else {
		n = 1
	}
	dtheta /= float64(n)

	for i := 0; i < n; i++ {
		c := libm.Cos(theta)
		s := libm.Sin(theta)
		// The start point of the curve, and the tangent vector at
		// that start point.
		p0 := center.Plus(u.ScaledBy(r * c)).Plus(v.ScaledBy(r * s))
		t0 := u.ScaledBy(-r * s).Plus(v.ScaledBy(r * c))

		theta += dtheta

		c = libm.Cos(theta)
		s = libm.Sin(theta)
		p2 := center.Plus(u.ScaledBy(r * c)).Plus(v.ScaledBy(r * s))
		t2 := u.ScaledBy(-r * s).Plus(v.ScaledBy(r * c))

		// The control point must lie on both tangents.
		p1 := geom.AtIntersectionOfLines(p0, p0.Plus(t0), p2, p2.Plus(t2))

		sb := srf.SBezierFrom3(p0, p1, p2)
		sb.Weight[1] = libm.Cos(dtheta / 2)
		sbl.Add(sb)
	}
}

type idfSection int

const (
	idfNone idfSection = iota
	idfHeader
	idfBoardOutline
	idfOtherOutline
	idfRoutingOutline
	idfPlacementOutline
	idfRoutingKeepout
	idfViaKeepout
	idfPlacementGroup
	idfDrilledHoles
	idfNotes
	idfComponentPlacement
)

// linkIDF reads the board outline section of an IDF file (C++ LinkIDF).
// Where C++ would stop on a number it can't read, this reports that
// the file couldn't be loaded.
func linkIDF(path string, sk *sketch.Sketch, el *[]core.Entity, m *srf.SMesh, sh *srf.SShell) bool {
	*el = (*el)[:0]
	raw, err := os.ReadFile(path)
	if err != nil {
		return false
	}

	section := idfNone
	recordNumber := 0
	curve := -1
	entityCount := 0

	var hprev, hprevTop core.HEntity
	var pprev, pprevTop geom.Vector

	boardThickness := 10.0
	scale := 1.0 // mm
	topEntities := false
	bottomEntities := false

	normal := geom.QuaternionFromBasis(geom.Vector{X: 1}, geom.Vector{Y: 1})
	hnorm := idfNewNormal(el, &entityCount, normal)

	// to create the extrusion we will need to collect a set of bezier
	// curves defined by the perimeter, cutouts, and holes.
	var sbl srf.SBezierList

	// std::getline: no empty line after a final newline.
	lines := strings.Split(string(raw), "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	for _, line := range lines {
		if strings.HasPrefix(line, ".END_") {
			section = idfNone
			curve = -1
		}
		switch section {
		case idfNone:
			if strings.HasPrefix(line, ".HEADER") {
				section = idfHeader
				recordNumber = 1
			} else if strings.HasPrefix(line, ".BOARD_OUTLINE") {
				section = idfBoardOutline
				recordNumber = 1
			} else if strings.HasPrefix(line, ".ROUTE_KEEPOUT") {
				section = idfRoutingKeepout
				recordNumber = 1
			} else if strings.HasPrefix(line, ".DRILLED_HOLES") {
				section = idfDrilledHoles
				recordNumber = 1
			}

		case idfHeader:
			if recordNumber == 3 {
				if strings.Contains(line, "MM") {
					scale = 1.0
				} else if strings.Contains(line, "THOU") {
					scale = 0.0254
				}
			}

		case idfRoutingKeepout, idfBoardOutline:
			if recordNumber == 2 {
				if section == idfBoardOutline {
					topEntities = true
					bottomEntities = true
					t, err := stod(line)
					if err != nil {
						return false
					}
					boardThickness = t * scale
				} else if section == idfRoutingKeepout {
					topEntities = false
					bottomEntities = false
					if strings.HasPrefix(line, "TOP") || strings.HasPrefix(line, "BOTH") {
						topEntities = true
					}
					if strings.HasPrefix(line, "BOTTOM") || strings.HasPrefix(line, "BOTH") {
						bottomEntities = true
					}
				}
			} else { // records 3+ are lines, arcs, and circles
				values := splitIDFString(line)
				if len(values) != 4 {
					continue
				}
				c, err0 := stoi(values[0])
				x, err1 := stof(values[1])
				y, err2 := stof(values[2])
				ang, err3 := stof(values[3])
				if err0 != nil || err1 != nil || err2 != nil || err3 != nil {
					return false
				}
				x *= scale
				y *= scale
				point := geom.Vector{X: x, Y: y}
				pTop := geom.Vector{X: x, Y: y, Z: boardThickness}
				if c != curve { // start a new curve
					curve = c
					if bottomEntities {
						hprev = idfNewPoint(el, &entityCount, point, false)
					}
					if topEntities {
						hprevTop = idfNewPoint(el, &entityCount, pTop, false)
					}
					pprev = point
					pprevTop = pTop
				} else {
					if section == idfBoardOutline {
						// create a bezier for the extrusion
						if ang == 0 {
							// straight lines
							sbl.Add(srf.SBezierFrom2(pprev, point))
						} else if ang != 360.0 {
							// Arcs
							c := arcCenter(pprev, point, ang)
							makeBeziersForArcs(&sbl, c, pprev, point, normal, ang)
						} else {
							// circles
							makeBeziersForArcs(&sbl, point, pprev, pprev, normal, ang)
						}
					}
					// next create the entities; only curves and points at
					// circle centers will be visible
					vis := ang == 360.0
					if bottomEntities {
						hp := idfNewPoint(el, &entityCount, point, vis)
						createIDFEntity(el, &entityCount, hprev, hp, hnorm, pprev, point, ang,
							section == idfRoutingKeepout)
						pprev = point
						hprev = hp
					}
					if topEntities {
						hp := idfNewPoint(el, &entityCount, pTop, vis)
						createIDFEntity(el, &entityCount, hprevTop, hp, hnorm, pprevTop, pTop,
							ang, section == idfRoutingKeepout)
						pprevTop = pTop
						hprevTop = hp
					}
				}
			}

		case idfOtherOutline, idfRoutingOutline, idfPlacementOutline, idfViaKeepout, idfPlacementGroup:

		case idfDrilledHoles:
			values := splitIDFString(line)
			if len(values) < 6 {
				continue
			}
			d, err0 := stof(values[0])
			x, err1 := stof(values[1])
			y, err2 := stof(values[2])
			if err0 != nil || err1 != nil || err2 != nil {
				return false
			}
			d *= scale
			x *= scale
			y *= scale
			duplicate := isHoleDuplicate(*el, x, y, d/2)
			// Only show holes likely to be useful in MCAD to reduce
			// complexity.
			if (d > 1.7 || strings.HasPrefix(values[5], "PIN") ||
				strings.HasPrefix(values[5], "MTG")) && !duplicate {
				// create the entity
				cent := geom.Vector{X: x, Y: y}
				hcent := idfNewPoint(el, &entityCount, cent, true)
				hdist := idfNewDistance(el, &entityCount, d/2)
				idfNewCircle(el, &entityCount, hcent, hdist, hnorm, false)
				// and again for the top
				cTop := geom.Vector{X: x, Y: y, Z: boardThickness}
				hcent = idfNewPoint(el, &entityCount, cTop, true)
				hdist = idfNewDistance(el, &entityCount, d/2)
				idfNewCircle(el, &entityCount, hcent, hdist, hnorm, false)
				// create the curves for the extrusion
				pt := geom.Vector{X: x + d/2, Y: y}
				makeBeziersForArcs(&sbl, cent, pt, pt, normal, 360.0)
			}

		case idfNotes, idfComponentPlacement:

		default:
			section = idfNone
		}
		recordNumber++
	}

	// now we can create an extrusion from all the Bezier curves. We can
	// skip things like checking for a coplanar sketch because everything
	// is at z=0.
	var polyLoops srf.SPolygon
	var allClosed, allCoplanar bool
	var errorPointAt geom.Vector
	var errorAt srf.SEdge

	var sblss srf.SBezierLoopSetSet
	sblss.FindOuterFacesFrom(&sbl, &polyLoops, nil,
		100.0, &allClosed, &errorAt,
		&allCoplanar, &errorPointAt, nil)

	// hack for when there is no sketch yet and the first group is a
	// linked IDF
	ctc := srf.ChordTolMm()
	if ctc == 0.0 {
		srf.SetChordTolMm(0.1) // mm
	}
	// there should only by one sbls in the sblss unless a board has
	// disjointed parts...
	if len(sblss.Sets) > 0 {
		sh.MakeFromExtrusionOf(&sblss.Sets[0], geom.Vector{},
			geom.Vector{Z: boardThickness},
			core.NewRgbaColor(0, 180, 0, 255))
	}
	srf.SetChordTolMm(ctc)
	sh.BooleanFailed = false

	// C++ keeps an EntityList sorted by handle.
	sort.SliceStable(*el, func(i, j int) bool { return (*el)[i].H.V < (*el)[j].H.V })
	return true
}
