package core

type Param struct {
	Tag int
	H   HParam

	Val   float64
	Known bool
	Free  bool

	Substd *Param
}

func (p *Param) Clear() {}

type ParamList []Param

func (l *ParamList) Add(p Param) {
	*l = append(*l, p)
}
