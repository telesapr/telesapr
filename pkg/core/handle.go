package core

type HEquation struct {
	V uint32
}

// IsFromConstraint reports whether the equation was written by a
// constraint: entity and group equations set one of the top two bits.
func (h HEquation) IsFromConstraint() bool {
	return h.V&0xc0000000 == 0
}

// Constraint is the constraint that wrote the equation.
func (h HEquation) Constraint() HConstraint {
	return HConstraint{V: h.V >> 16}
}

type HGroup struct {
	V uint32
}

func (h HGroup) Entity(i int) HEntity {
	return HEntity{V: 0x80000000 | (h.V << 16) | uint32(i)}
}

func (h HGroup) Param(i int) HParam {
	return HParam{V: 0x80000000 | (h.V << 16) | uint32(i)}
}

func (h HGroup) Equation(i int) HEquation {
	return HEquation{V: 0x80000000 | (h.V << 16) | uint32(i)}
}

type HRequest struct {
	V uint32
}

func (h HRequest) Entity(i int) HEntity {
	return HEntity{V: (h.V << 16) | uint32(i)}
}

func (h HRequest) Param(i int) HParam {
	return HParam{V: (h.V << 16) | uint32(i)}
}

// IsFromReferences reports whether the request is one of the three
// reference workplanes.
func (h HRequest) IsFromReferences() bool {
	return h.V == 1 || h.V == 2 || h.V == 3
}

type HEntity struct {
	V uint32
}

func (h HEntity) IsFromRequest() bool {
	return h.V != 0 && (h.V&0x80000000) == 0
}

func (h HEntity) Request() HRequest {
	return HRequest{V: h.V >> 16}
}

func (h HEntity) Group() HGroup {
	return HGroup{V: (h.V >> 16) & 0x3FFF}
}

func (h HEntity) Equation(i int) HEquation {
	return HEquation{V: h.V | 0x40000000 | uint32(i)}
}

type HParam struct {
	V uint32
}

func (h HParam) Request() HRequest {
	return HRequest{V: h.V >> 16}
}

type HConstraint struct {
	V uint32
}

func (h HConstraint) Equation(i int) HEquation {
	return HEquation{V: (h.V << 16) | uint32(i)}
}

func (h HConstraint) Param(i int) HParam {
	return HParam{V: h.V | 0x40000000 | uint32(i)}
}

type HStyle struct {
	V uint32
}

type HSCurve struct {
	V uint32
}

type HSSurface struct {
	V uint32
}

func (h HSCurve) IsFromRequest() bool {
	return h.V != 0 && (h.V&0x80000000) == 0
}

func (h HSSurface) IsFromRequest() bool {
	return h.V != 0 && (h.V&0x80000000) == 0
}

var (
	NoSCurve   = HSCurve{V: 0}
	NoSSurface = HSSurface{V: 0}
)

type EntityID struct {
	V uint32
}

var (
	NoParam      = HParam{V: 0}
	NoConstraint = HConstraint{V: 0}
	NoEntity     = HEntity{V: 0}
	FreeIn3D     = HEntity{V: 0xFFFFFFFF}
)

const (
	HGroupReferences = 1
)

type EntityKey struct {
	Input      HEntity
	CopyNumber int
}

type EntityMap map[EntityKey]EntityID
