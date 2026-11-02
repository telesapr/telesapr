package sketch

import (
	"fmt"

	"github.com/telesapr/telesapr/pkg/core"
)

// This file mends what a damaged file can lack that no group could be
// left out for: the references group and its three workplanes, which
// every model has and every other group is placed from. The rest of
// what a damaged file refers to and lacks is left out by the
// regeneration's pruning and listed in BrokenReferences (broken.go).

// referenceRequests are the reference workplanes, in the order a new
// sketch makes them.
var referenceRequests = [...]core.HRequest{HRequestReferenceXY, HRequestReferenceYZ, HRequestReferenceZX}

// mendReferences puts back the references group and its workplanes
// where a loaded sketch lacks them or has something else under their
// handles, and records each in Mended. They are the same in every
// model, so what is put back is what the file lost.
func (sk *Sketch) mendReferences() {
	refs := core.HGroup{V: core.HGroupReferences}
	if len(sk.Groups) == 0 && len(sk.Requests) == 0 && len(sk.Constraints) == 0 {
		// A file of nothing at all, which lacks nothing it refers to:
		// the empty model.
		sk.AddReferences()
		return
	}
	g := sk.Group(refs)
	if g == nil {
		sk.Groups = append(sk.Groups, Group{H: refs, Type: GroupDrawing3D, Name: "#references", Visible: true, Scale: 1})
		g = &sk.Groups[len(sk.Groups)-1]
		sk.mended("the references group was missing and was made again")
	}
	if g.Type != GroupDrawing3D {
		g.Type = GroupDrawing3D
		sk.mended("the references group was not a 3d sketch and was made one")
	}
	// It comes first: every group is placed from its workplanes.
	first := true
	low := g.Order
	for i := range sk.Groups {
		if o := &sk.Groups[i]; o.H != refs && o.Order <= g.Order {
			first = false
			low = min(low, o.Order)
		}
	}
	if !first {
		g.Order = low - 1
		sk.mended("the references group was not the first and was moved there")
	}
	for i, hr := range referenceRequests {
		r := sk.Request(hr)
		want := Request{H: hr, Type: RequestWorkplane, Group: refs, Workplane: core.FreeIn3D}
		name := [...]string{"XY", "YZ", "ZX"}[i]
		switch {
		case r == nil:
			sk.Requests = append(sk.Requests, want)
			sk.mended(fmt.Sprintf("the reference workplane %s was missing and was made again", name))
		case r.Type != want.Type || r.Group != want.Group || r.Workplane != want.Workplane:
			*r = want
			sk.mended(fmt.Sprintf("the reference workplane %s was something else and was made again", name))
		}
	}
}

// mended records what the load mended.
func (sk *Sketch) mended(what string) { sk.Mended = append(sk.Mended, what) }
