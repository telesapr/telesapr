package ui

import (
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
)

// View > Show > Reference Dimensions and Comments hide those alone, and
// show them again; a dimension that isn't a reference stays shown.
func TestShowReferencesAndComments(t *testing.T) {
	t.Parallel()
	ss, _ := newTestApp(t)
	gw := ss.GW
	hg := gw.activeGroupH()
	dim := &core.Constraint{ConstraintBase: core.ConstraintBase{Type: core.ConstraintPtPtDistance, Group: hg}}
	ref := &core.Constraint{ConstraintBase: core.ConstraintBase{Type: core.ConstraintPtPtDistance, Group: hg, Reference: true}}
	comment := &core.Constraint{ConstraintBase: core.ConstraintBase{Type: core.ConstraintComment, Group: hg}}
	visible := func() [3]bool {
		return [3]bool{gw.constraintIsVisible(dim), gw.constraintIsVisible(ref), gw.constraintIsVisible(comment)}
	}
	if got := visible(); got != [3]bool{true, true, true} {
		t.Fatalf("at first: %v", got)
	}
	gw.MenuShow(cmdShowReferences)
	if got := visible(); got != [3]bool{true, false, true} || gw.show.References {
		t.Errorf("references hidden: %v", got)
	}
	gw.MenuShow(cmdShowComments)
	if got := visible(); got != [3]bool{true, false, false} {
		t.Errorf("comments hidden too: %v", got)
	}
	gw.MenuShow(cmdShowReferences)
	gw.MenuShow(cmdShowComments)
	if got := visible(); got != [3]bool{true, true, true} {
		t.Errorf("shown again: %v", got)
	}
}
