package sketch

import (
	"strconv"
	"strings"

	"github.com/telesapr/telesapr/pkg/core"
)

// GroupNamed is the group with a name, or with a description as the
// property browser shows it: "g003-extrude", or just "g003"; nil when
// there is none. A name comes first: a group named "g003" is that one.
func (sk *Sketch) GroupNamed(name string) *Group {
	for i := range sk.Groups {
		if sk.Groups[i].Name == name {
			return &sk.Groups[i]
		}
	}
	if len(name) < 2 || name[0] != 'g' {
		return nil
	}
	digits, _, _ := strings.Cut(name[1:], "-")
	v, err := strconv.ParseUint(digits, 16, 32)
	if err != nil {
		return nil
	}
	return sk.Group(core.HGroup{V: uint32(v)})
}
