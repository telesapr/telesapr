package sketch

import (
	"math"

	"github.com/spacesolve/spacesolve/pkg/core"
	"github.com/spacesolve/spacesolve/pkg/geom"
)

type Sketch struct {
	Groups      []Group
	GroupOrder  []core.HGroup
	Constraints []core.Constraint
	Requests    []Request
	Styles      []core.Style

	Entities []core.Entity
	Params   []core.Param
}

func NewSketch() *Sketch {
	return &Sketch{
		Groups:      make([]Group, 0),
		GroupOrder:  make([]core.HGroup, 0),
		Constraints: make([]core.Constraint, 0),
		Requests:    make([]Request, 0),
		Styles:      make([]core.Style, 0),
		Entities:    make([]core.Entity, 0),
		Params:      make([]core.Param, 0),
	}
}

func (s *Sketch) GetConstraint(h core.HConstraint) *core.Constraint {
	for i := range s.Constraints {
		if s.Constraints[i].H == h {
			return &s.Constraints[i]
		}
	}
	return nil
}

func (s *Sketch) GetEntity(h core.HEntity) *core.Entity {
	for i := range s.Entities {
		if s.Entities[i].H == h {
			return &s.Entities[i]
		}
	}
	return nil
}

func (s *Sketch) GetParam(h core.HParam) *core.Param {
	for i := range s.Params {
		if s.Params[i].H == h {
			return &s.Params[i]
		}
	}
	return nil
}

func (s *Sketch) GetRequest(h core.HRequest) *Request {
	for i := range s.Requests {
		if s.Requests[i].H == h {
			return &s.Requests[i]
		}
	}
	return nil
}

func (s *Sketch) GetGroup(h core.HGroup) *Group {
	for i := range s.Groups {
		if s.Groups[i].H == h {
			return &s.Groups[i]
		}
	}
	return nil
}

func (s *Sketch) GetStyle(h core.HStyle) *core.Style {
	for i := range s.Styles {
		if s.Styles[i].H == h {
			return &s.Styles[i]
		}
	}
	return nil
}

func (s *Sketch) AddGroup(g Group) core.HGroup {
	g.H.V = uint32(len(s.Groups) + 1)
	s.Groups = append(s.Groups, g)
	return g.H
}

func (s *Sketch) AddRequest(r Request) core.HRequest {
	r.H.V = uint32(len(s.Requests) + 1)
	s.Requests = append(s.Requests, r)
	return r.H
}

func (s *Sketch) AddEntity(e core.Entity) core.HEntity {
	e.H.V = uint32(len(s.Entities) + 1)
	s.Entities = append(s.Entities, e)
	return e.H
}

func (s *Sketch) AddParam(p core.Param) core.HParam {
	p.H.V = uint32(len(s.Params) + 1)
	s.Params = append(s.Params, p)
	return p.H
}

func (s *Sketch) AddConstraint(c core.Constraint) core.HConstraint {
	c.H.V = uint32(len(s.Constraints) + 1)
	s.Constraints = append(s.Constraints, c)
	return c.H
}

func (s *Sketch) AddStyle(st core.Style) core.HStyle {
	st.H.V = uint32(len(s.Styles) + 1)
	s.Styles = append(s.Styles, st)
	return st.H
}

func (s *Sketch) Clear() {
	s.Groups = s.Groups[:0]
	s.GroupOrder = s.GroupOrder[:0]
	s.Constraints = s.Constraints[:0]
	s.Requests = s.Requests[:0]
	s.Styles = s.Styles[:0]
	s.Entities = s.Entities[:0]
	s.Params = s.Params[:0]
}

func (s *Sketch) CalculateEntityBBox(includingInvisible bool) core.BBox {
	bbox := core.BBox{}
	firstPoint := true

	includePoint := func(p geom.Vector) {
		if firstPoint {
			bbox.MinP = p
			bbox.MaxP = p
			firstPoint = false
			return
		}
		bbox.Include(p, 0)
	}

	pointOnCircle := func(center, u, v geom.Vector, r, theta float64) geom.Vector {
		c := math.Cos(theta)
		sn := math.Sin(theta)
		return center.Plus(u.ScaledBy(r * c)).Plus(v.ScaledBy(r * sn))
	}

	normalizeAngle := func(theta float64) float64 {
		for theta < 0 {
			theta += 2 * math.Pi
		}
		for theta >= 2*math.Pi {
			theta -= 2 * math.Pi
		}
		return theta
	}

	angleInArc := func(thetaStart, dtheta, theta float64) bool {
		thetaStart = normalizeAngle(thetaStart)
		theta = normalizeAngle(theta)

		thetaEnd := thetaStart + dtheta
		if thetaEnd <= 2*math.Pi {
			return theta >= thetaStart-1e-9 && theta <= thetaEnd+1e-9
		}
		thetaEnd = normalizeAngle(thetaEnd)
		return theta >= thetaStart-1e-9 || theta <= thetaEnd+1e-9
	}

	for i := range s.Entities {
		e := &s.Entities[i]
		if e.Construction {
			continue
		}
		if !includingInvisible && !e.ActVisible {
			continue
		}

		if e.IsPoint() {
			// Skip the circle center generated for arc requests.
			if e.H.IsFromRequest() {
				if req := s.GetRequest(e.H.Request()); req != nil {
					if req.Type == RequestArcOfCircle && e.H == req.H.Entity(1) {
						continue
					}
				}
			}
			includePoint(e.PointGetNum())
			continue
		}

		switch e.Type {
		case core.EntityCircle, core.EntityArcOfCircle:
			centerEntity := s.GetEntity(e.Point[0])
			if centerEntity == nil {
				continue
			}

			center := centerEntity.PointGetNum()

			normalEntity := s.GetEntity(e.Normal)
			q := geom.NewQuaternion(1, 0, 0, 0)
			if normalEntity != nil {
				q = normalEntity.NormalGetNum()
			}
			u := q.RotationU()
			v := q.RotationV()

			radius := 0.0
			if e.Type == core.EntityCircle {
				if dist := s.GetEntity(e.Distance); dist != nil {
					radius = dist.DistanceGetNum()
				}
			} else {
				if start := s.GetEntity(e.Point[1]); start != nil {
					radius = start.PointGetNum().Minus(center).Magnitude()
				}
			}
			if radius < core.LengthEps {
				includePoint(center)
				continue
			}

			if e.Type == core.EntityCircle {
				for axis := 0; axis < 3; axis++ {
					var uc, vc float64
					switch axis {
					case 0:
						uc, vc = u.X, v.X
					case 1:
						uc, vc = u.Y, v.Y
					default:
						uc, vc = u.Z, v.Z
					}
					theta := math.Atan2(vc, uc)
					for _, candidate := range []float64{theta, theta + math.Pi} {
						includePoint(pointOnCircle(center, u, v, radius, candidate))
					}
				}
				continue
			}

			startEntity := s.GetEntity(e.Point[1])
			endEntity := s.GetEntity(e.Point[2])
			if startEntity == nil || endEntity == nil {
				continue
			}

			startPoint := startEntity.PointGetNum()
			endPoint := endEntity.PointGetNum()

			includePoint(startPoint)
			includePoint(endPoint)

			getAngle := func(p geom.Vector) float64 {
				offset := p.Minus(center)
				x := offset.Dot(u)
				y := offset.Dot(v)
				return math.Atan2(y, x)
			}

			thetaStart := getAngle(startPoint)
			thetaEnd := getAngle(endPoint)
			dtheta := thetaEnd - thetaStart

			for dtheta < 1e-6 {
				dtheta += 2 * math.Pi
			}
			for dtheta > 2*math.Pi {
				dtheta -= 2 * math.Pi
			}

			for axis := 0; axis < 3; axis++ {
				var uc, vc float64
				switch axis {
				case 0:
					uc, vc = u.X, v.X
				case 1:
					uc, vc = u.Y, v.Y
				default:
					uc, vc = u.Z, v.Z
				}
				theta := math.Atan2(vc, uc)
				for _, candidate := range []float64{theta, theta + math.Pi} {
					if !angleInArc(thetaStart, dtheta, candidate) {
						continue
					}
					includePoint(pointOnCircle(center, u, v, radius, candidate))
				}
			}
		}
	}

	return bbox
}

func (s *Sketch) GetRunningMeshGroupFor(h core.HGroup) *Group {
	for g := s.GetGroup(h); g != nil; g = g.PreviousGroup(s) {
		if g.IsMeshGroup() {
			return g
		}
	}
	return nil
}

// ParamsToMap returns the current param values as a map from handle
// to float64. Used by Constraint::ModifyToSatisfy and other places
// that need direct param access.
func (s *Sketch) ParamsToMap() map[core.HParam]float64 {
	out := make(map[core.HParam]float64, len(s.Params))
	for _, p := range s.Params {
		out[p.H] = p.Val
	}
	return out
}

// EntitiesToMap returns the entities indexed by their handle.
func (s *Sketch) EntitiesToMap() map[core.HEntity]core.Entity {
	out := make(map[core.HEntity]core.Entity, len(s.Entities))
	for _, e := range s.Entities {
		out[e.H] = e
	}
	return out
}
