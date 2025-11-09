package sketch

import (
	"github.com/spacesolve/spacesolve/pkg/core"
	"github.com/spacesolve/spacesolve/pkg/geom"
)

type ClipboardRequest struct {
	Type         RequestType
	ExtraPoints  int
	Style        core.HStyle
	Str          string
	Font         string
	File         string
	Construction bool

	Point    [core.MaxPointsInEntity]geom.Vector
	Distance float64

	OldEnt      core.HEntity
	OldPointEnt [core.MaxPointsInEntity]core.HEntity
	NewReq      core.HRequest
}

type Clipboard struct {
	Requests    []ClipboardRequest
	Constraints []core.Constraint
}

func (c *Clipboard) Clear() {
	c.Requests = c.Requests[:0]
	c.Constraints = c.Constraints[:0]
}

func (c *Clipboard) ContainsEntity(old core.HEntity) bool {
	for i := range c.Requests {
		if c.Requests[i].OldEnt == old {
			return true
		}
		for j := range c.Requests[i].OldPointEnt {
			if c.Requests[i].OldPointEnt[j] == old {
				return true
			}
		}
	}
	return false
}

func (c *Clipboard) NewEntityFor(old core.HEntity) core.HEntity {
	for i := range c.Requests {
		if c.Requests[i].OldEnt == old {
			return c.Requests[i].NewReq.Entity(0)
		}
		for j := range c.Requests[i].OldPointEnt {
			if c.Requests[i].OldPointEnt[j] == old {
				return c.Requests[i].NewReq.Entity(j)
			}
		}
	}
	return core.NoEntity
}
