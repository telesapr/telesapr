package sketch

import "github.com/telesapr/telesapr/pkg/core"

func coreParam(v uint32, val float64) core.Param {
	return core.Param{H: core.HParam{V: v}, Val: val}
}

func hparam(v uint32) core.HParam {
	return core.HParam{V: v}
}

func hentity(v uint32) core.HEntity {
	return core.HEntity{V: v}
}

func coreEntityPoint(h core.HEntity) core.Entity {
	return core.Entity{
		EntityBase: core.EntityBase{
			H:    h,
			Type: core.EntityPointIn3D,
		},
	}
}
