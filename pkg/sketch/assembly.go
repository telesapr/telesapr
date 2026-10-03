package sketch

import (
	"path/filepath"
	"strconv"
	"strings"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// AssemblyPart is a part of an assembly as a STEP file gives it
// (roadmap 16.8): a linked file's solid in the file's own coordinates,
// placed by the link's rotation and offset, so that a file linked twice
// is one product placed twice; or another group's solid, where it is.
type AssemblyPart struct {
	Group core.HGroup
	// Name is the product's: the linked file's name without its folder
	// and extension, or the group's name.
	Name string
	// Key is the same for the parts of one product: the linked file,
	// or the group.
	Key string
	// Shell is the solid in the product's own coordinates, Q and Offset
	// what place it in the model.
	Shell  *srf.SShell
	Q      geom.Quaternion
	Offset geom.Vector
}

// AssemblyParts are the parts of the model up to group upTo, as the
// interference check takes them (interference.go): the solid before the
// first group that combines as an assembly, and each such group. Fewer
// than two means the model is no assembly. Parts with no shell are
// left out.
func (sk *Sketch) AssemblyParts(upTo core.HGroup) []AssemblyPart {
	var parts []AssemblyPart
	var before *Group
	assembled := false
	for _, hg := range sk.GroupOrder {
		g := sk.GetGroup(hg)
		if g != nil && !g.Suppress && sk.SuppressedReason(hg) == "" {
			solid := g.IsMeshGroup() || g.Type == GroupLinked || g.Type == GroupMirror
			switch {
			case solid && g.MeshCombine == CombineAssemble:
				if !assembled && before != nil && len(before.RunningShell.Surface) > 0 {
					parts = append(parts, AssemblyPart{Group: before.H, Name: before.Name,
						Key: "group " + before.Name, Shell: &before.RunningShell, Q: geom.QuaternionIdentity})
				}
				assembled = true
				if g.IsTriangleMeshAssembly() || g.ForceToMesh || len(g.ThisShell.Surface) == 0 {
					break
				}
				if g.Type == GroupLinked && len(g.ImpShell.Surface) > 0 {
					offset, q, scale := g.linkPlacement(sk)
					local := &srf.SShell{}
					local.MakeFromTransformationOf(&g.ImpShell, geom.Vector{}, geom.QuaternionIdentity, scale)
					base := filepath.Base(g.LinkFile)
					parts = append(parts, AssemblyPart{Group: hg, Name: strings.TrimSuffix(base, filepath.Ext(base)),
						Key: "file " + g.LinkFile + " " + strconv.FormatFloat(scale, 'g', -1, 64), Shell: local, Q: q, Offset: offset})
					break
				}
				parts = append(parts, AssemblyPart{Group: hg, Name: g.Name, Key: "group " + g.Name,
					Shell: &g.ThisShell, Q: geom.QuaternionIdentity})
			case solid && !assembled:
				before = g
			}
		}
		if hg == upTo {
			break
		}
	}
	return parts
}

// linkPlacement is a link group's offset, rotation and scale.
func (g *Group) linkPlacement(sk *Sketch) (geom.Vector, geom.Quaternion, float64) {
	offset := geom.Vector{
		X: paramFloat(sk, g.H.Param(0)),
		Y: paramFloat(sk, g.H.Param(1)),
		Z: paramFloat(sk, g.H.Param(2)),
	}
	q := geom.Quaternion{
		W: paramFloat(sk, g.H.Param(3)),
		X: paramFloat(sk, g.H.Param(4)),
		Y: paramFloat(sk, g.H.Param(5)),
		Z: paramFloat(sk, g.H.Param(6)),
	}
	return offset, q, g.Scale
}
