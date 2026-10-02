package core

type Unit int

const (
	UnitMM Unit = iota
	UnitInches
	UnitMeters
	UnitFeetInches
)

// MmPer is how many millimetres one of the unit is: a length in the
// unit times MmPer is millimetres. Feet and inches count in inches.
func (u Unit) MmPer() float64 {
	switch u {
	case UnitInches, UnitFeetInches:
		return 25.4
	case UnitMeters:
		return 1000
	}
	return 1
}
