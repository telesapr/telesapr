package core

import (
	"github.com/telesapr/telesapr/pkg/geom"
)

type EntityType int

const (
	EntityPointIn3D EntityType = 2000 + iota
	EntityPointIn2D
	_
	_
	_
	_
	_
	_
	_
	_
	EntityPointNTrans
	EntityPointNRotTrans
	EntityPointNCopy
	EntityPointNRotAA
	EntityPointNRotAxisTrans
)

const (
	EntityNormalIn3D EntityType = 3000 + iota
	EntityNormalIn2D
	_
	_
	_
	_
	_
	_
	_
	_
	EntityNormalNCopy
	EntityNormalNRot
	EntityNormalNRotAA
)

const (
	EntityDistance EntityType = 4000 + iota
	EntityDistanceNCopy
)

const (
	EntityFaceNormalPt EntityType = 5000 + iota
	EntityFaceXProd
	EntityFaceNRotTrans
	EntityFaceNTrans
	EntityFaceNRotAA
	EntityFaceRotNormalPt
	EntityFaceNRotAxisTrans
)

const (
	EntityWorkplane     EntityType = 10000
	EntityLineSegment   EntityType = 11000
	EntityCubic         EntityType = 12000
	EntityCubicPeriodic EntityType = 12001
	EntityCircle        EntityType = 13000
	EntityArcOfCircle   EntityType = 14000
	EntityTTFText       EntityType = 15000
	EntityImage         EntityType = 16000

	// Surface entities for generated meshes
	EntitySurfaceExtruded EntityType = 20000
	EntitySurfaceLathed   EntityType = 21000
	EntitySurfaceRevolved EntityType = 22000
)

type EntityBase struct {
	Tag int
	H   HEntity

	Type      EntityType
	Group     HGroup
	Workplane HEntity

	Point       [MaxPointsInEntity]HEntity
	ExtraPoints int
	Normal      HEntity
	Distance    HEntity
	Param       [8]HParam

	NumPoint    geom.Vector
	NumNormal   geom.Quaternion
	NumDistance float64

	Str         string
	Font        string
	File        string
	AspectRatio float64

	TimesApplied int
}

type Entity struct {
	EntityBase

	ForceHidden bool

	ActPoint    geom.Vector
	ActNormal   geom.Quaternion
	ActDistance float64
	ActVisible  bool

	Style        HStyle
	Construction bool

	EdgesChordTol   float64
	ScreenBBoxValid bool
}

func (e *Entity) IsPoint() bool {
	return e.Type >= EntityPointIn3D && e.Type <= EntityPointNRotAxisTrans
}

func (e *Entity) IsNormal() bool {
	return e.Type >= EntityNormalIn3D && e.Type <= EntityNormalNRotAA
}

func (e *Entity) IsDistance() bool {
	return e.Type == EntityDistance || e.Type == EntityDistanceNCopy
}

func (e *Entity) IsFace() bool {
	return e.Type >= EntityFaceNormalPt && e.Type <= EntityFaceNRotAxisTrans
}

func (e *Entity) IsWorkplane() bool {
	return e.Type == EntityWorkplane
}

func (e *Entity) IsCircle() bool {
	return e.Type == EntityCircle || e.Type == EntityArcOfCircle
}

func (e *Entity) HasVector() bool {
	switch e.Type {
	case EntityLineSegment, EntityNormalIn3D, EntityNormalIn2D,
		EntityNormalNCopy, EntityNormalNRot, EntityNormalNRotAA:
		return true
	}
	return false
}

func (e *Entity) HasEndpoints() bool {
	return e.Type == EntityLineSegment ||
		e.Type == EntityCubic ||
		e.Type == EntityArcOfCircle
}

func (e *Entity) Clear() {}

type EntityList []Entity

func (l *EntityList) Add(e Entity) {
	*l = append(*l, e)
}
