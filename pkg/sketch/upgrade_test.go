package sketch

import (
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
)

// TestActiveGroup: the group the sketch is drawn and worked on at. A
// file names the group that was active when it was saved only when
// that is not the last one, so a sketch read from a file often names
// none at all, and the answer is then the last group, stepping over
// the references.
func TestActiveGroup(t *testing.T) {
	t.Parallel()
	refs := core.HGroup{V: core.HGroupReferences}
	one := core.HGroup{V: core.HGroupReferences + 1}
	two := core.HGroup{V: core.HGroupReferences + 2}
	for _, tc := range []struct {
		name   string
		groups []core.HGroup
		order  []core.HGroup
		active core.HGroup
		want   core.HGroup
	}{
		{"the group the sketch names", []core.HGroup{refs, one, two}, []core.HGroup{refs, one, two}, one, one},
		{"the last group, when the sketch is not made active", []core.HGroup{refs, one, two}, []core.HGroup{refs, one, two}, core.HGroup{}, two},
		{"the last group named is the references", []core.HGroup{refs, one}, []core.HGroup{refs, one}, core.HGroup{}, one},
		{"nothing but the references", []core.HGroup{refs}, []core.HGroup{refs}, core.HGroup{}, refs},
		{"a group that is not there", []core.HGroup{refs, one}, []core.HGroup{refs, one}, core.HGroup{V: 99}, one},
		{"no groups at all", nil, nil, core.HGroup{}, core.HGroup{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sk := NewSketch()
			for _, h := range tc.groups {
				sk.Groups = append(sk.Groups, Group{H: h})
			}
			sk.GroupOrder = tc.order
			sk.Active = tc.active
			if got := sk.ActiveGroup(); got != tc.want {
				t.Errorf("the active group is %v, want %v", got, tc.want)
			}
		})
	}
}
