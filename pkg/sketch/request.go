package sketch

import (
	"fmt"

	"github.com/spacesolve/spacesolve/pkg/core"
	"github.com/spacesolve/spacesolve/pkg/ttf"
)

type RequestType int

const (
	RequestWorkplane RequestType = 100 + iota
	RequestDatumPoint
)

const (
	RequestLineSegment RequestType = 200
)

const (
	RequestCubic RequestType = 300 + iota
	RequestCubicPeriodic
)

const (
	RequestCircle RequestType = 400
)

const (
	RequestArcOfCircle RequestType = 500
)

const (
	RequestTTFText RequestType = 600
)

const (
	RequestImage RequestType = 700
)

var (
	HRequestReferenceXY = core.HRequest{V: 1}
	HRequestReferenceYZ = core.HRequest{V: 2}
	HRequestReferenceZX = core.HRequest{V: 3}
)

type Request struct {
	Tag int
	H   core.HRequest

	Type              RequestType
	ExtraPoints       int
	Workplane         core.HEntity
	Group             core.HGroup
	Style             core.HStyle
	Construction      bool
	Str               string
	Font              string
	File              string
	AspectRatio       float64
	GroupRequestIndex int
}

func (r *Request) Clear() {}

// Generate adds the entities and params of the request (C++
// Request::Generate). sk stands in for C++'s SS, whose loaded images
// give an image request its aspect ratio; it may be nil. A TTF request
// takes its aspect ratio from its font, in ttf.Fonts (C++ SS.fonts).
func (r *Request) Generate(sk *Sketch, entities *core.EntityList, params *core.ParamList) {
	entityType, pointCount, hasNormal, hasDistance, ok := getRequestInfo(r.Type, r.ExtraPoints)
	if !ok {
		panic("No request info")
	}

	// Request-specific generation.
	switch r.Type {
	case RequestTTFText:
		// `ExtraPoints` is storing kerning boolean
		actualAspectRatio := ttf.Fonts.AspectRatio(r.Font, r.Str, r.ExtraPoints != 0)
		if actualAspectRatio != 0 {
			// We could load the font, so use the actual value.
			r.AspectRatio = actualAspectRatio
		}
		if r.AspectRatio == 0 {
			// We couldn't load the font and we don't have anything saved,
			// so just use 1:1, which is valid for the missing font symbol anyhow.
			r.AspectRatio = 1.0
		}

	case RequestImage:
		if img := sk.image(r.File); img != nil {
			r.AspectRatio = float64(img.Width) / float64(img.Height)
		}
		if r.AspectRatio == 0 {
			r.AspectRatio = 1.0
		}
	}

	entity := core.Entity{}
	entity.Type = entityType
	entity.ExtraPoints = r.ExtraPoints
	entity.Group = r.Group
	entity.Style = r.Style
	entity.Workplane = r.Workplane
	entity.Construction = r.Construction
	entity.Str = r.Str
	entity.Font = r.Font
	entity.File = r.File
	entity.AspectRatio = r.AspectRatio
	entity.H = r.H.Entity(0)

	entityOffset := 1
	if entityType == core.EntityType(0) {
		entityOffset = 0
	}

	for i := 0; i < pointCount; i++ {
		point := core.Entity{}
		point.Workplane = r.Workplane
		point.H = r.H.Entity(i + entityOffset)
		point.Group = r.Group
		point.Style = r.Style
		point.Construction = r.Construction

		if r.Workplane == core.FreeIn3D {
			point.Type = core.EntityPointIn3D
			point.Param[0] = r.addParam(params, r.H.Param(16+3*i+0))
			point.Param[1] = r.addParam(params, r.H.Param(16+3*i+1))
			point.Param[2] = r.addParam(params, r.H.Param(16+3*i+2))
		} else {
			point.Type = core.EntityPointIn2D
			point.Param[0] = r.addParam(params, r.H.Param(16+3*i+0))
			point.Param[1] = r.addParam(params, r.H.Param(16+3*i+1))
		}

		if entities != nil {
			entities.Add(point)
		}
		entity.Point[i] = point.H
	}

	if hasNormal {
		normal := core.Entity{}
		normal.Workplane = r.Workplane
		normal.H = r.H.Entity(32)
		normal.Group = r.Group
		normal.Style = r.Style
		normal.Construction = r.Construction

		if r.Workplane == core.FreeIn3D {
			normal.Type = core.EntityNormalIn3D
			normal.Param[0] = r.addParam(params, r.H.Param(32+0))
			normal.Param[1] = r.addParam(params, r.H.Param(32+1))
			normal.Param[2] = r.addParam(params, r.H.Param(32+2))
			normal.Param[3] = r.addParam(params, r.H.Param(32+3))
		} else {
			normal.Type = core.EntityNormalIn2D
		}

		if pointCount < 1 {
			panic("Positioning a normal requires a point")
		}
		// The point determines where the normal gets displayed on-screen;
		// it's entirely cosmetic.
		normal.Point[0] = entity.Point[0]

		if entities != nil {
			entities.Add(normal)
		}
		entity.Normal = normal.H
	}

	if hasDistance {
		distance := core.Entity{}
		distance.Workplane = r.Workplane
		distance.H = r.H.Entity(64)
		distance.Group = r.Group
		distance.Style = r.Style
		distance.Type = core.EntityDistance
		distance.Param[0] = r.addParam(params, r.H.Param(64))

		if entities != nil {
			entities.Add(distance)
		}
		entity.Distance = distance.H
	}

	if entityType != core.EntityType(0) && entities != nil {
		entities.Add(entity)
	}
}

func (r *Request) DescriptionString() string {
	switch r.H {
	case HRequestReferenceXY:
		return fmt.Sprintf("r%03x-%s", r.H.V, "#XY")
	case HRequestReferenceYZ:
		return fmt.Sprintf("r%03x-%s", r.H.V, "#YZ")
	case HRequestReferenceZX:
		return fmt.Sprintf("r%03x-%s", r.H.V, "#ZX")
	}

	var name string
	switch r.Type {
	case RequestWorkplane:
		name = "workplane"
	case RequestDatumPoint:
		name = "datum-point"
	case RequestLineSegment:
		name = "line-segment"
	case RequestCubic:
		name = "cubic-bezier"
	case RequestCubicPeriodic:
		name = "periodic-cubic"
	case RequestCircle:
		name = "circle"
	case RequestArcOfCircle:
		name = "arc-of-circle"
	case RequestTTFText:
		name = "ttf-text"
	case RequestImage:
		name = "image"
	}

	return fmt.Sprintf("r%03x-%s", r.H.V, name)
}

func (r *Request) IndexOfPoint(h core.HEntity) int {
	if r.Type == RequestDatumPoint {
		if h == r.H.Entity(0) {
			return 0
		}
		return -1
	}

	for i := 0; i < core.MaxPointsInEntity; i++ {
		if h == r.H.Entity(i+1) {
			return i
		}
	}
	return -1
}

func (r *Request) addParam(params *core.ParamList, hp core.HParam) core.HParam {
	if params != nil {
		params.Add(core.Param{H: hp})
	}
	return hp
}

type entReqMapping struct {
	requestType    RequestType
	entityType     core.EntityType
	points         int
	useExtraPoints bool
	hasNormal      bool
	hasDistance    bool
}

var entReqMappings = []entReqMapping{
	{RequestWorkplane, core.EntityWorkplane, 1, false, true, false},
	{RequestDatumPoint, core.EntityType(0), 1, false, false, false},
	{RequestLineSegment, core.EntityLineSegment, 2, false, false, false},
	{RequestCubic, core.EntityCubic, 4, true, false, false},
	{RequestCubicPeriodic, core.EntityCubicPeriodic, 3, true, false, false},
	{RequestCircle, core.EntityCircle, 1, false, true, true},
	{RequestArcOfCircle, core.EntityArcOfCircle, 3, false, true, false},
	{RequestTTFText, core.EntityTTFText, 4, false, true, false},
	{RequestImage, core.EntityImage, 4, false, true, false},
}

func getRequestInfo(req RequestType, extraPoints int) (core.EntityType, int, bool, bool, bool) {
	for _, mapping := range entReqMappings {
		if mapping.requestType == req {
			points := mapping.points
			if mapping.useExtraPoints {
				points += extraPoints
			}
			return mapping.entityType, points, mapping.hasNormal, mapping.hasDistance, true
		}
	}
	return 0, 0, false, false, false
}

func getEntityInfo(entity core.EntityType, extraPoints int) (RequestType, int, bool, bool, bool) {
	for _, mapping := range entReqMappings {
		if mapping.entityType == entity {
			points := mapping.points
			if mapping.useExtraPoints {
				points += extraPoints
			}
			return mapping.requestType, points, mapping.hasNormal, mapping.hasDistance, true
		}
	}
	return 0, 0, false, false, false
}

type RequestList []Request
