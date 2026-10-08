// Package srf is the geometry kernel: NURBS and Bézier curves, trimmed
// surfaces, watertight shells, BSP trees, and Boolean operations on
// surfaces and meshes.
//
// Files are organized by class, with larger classes split into
// `_eval.go`, `_factory.go`, `_geom.go`, `_closest.go`,
// `_classify.go` to keep individual files readable.
package srf

import "github.com/telesapr/telesapr/pkg/core"

// SurfaceCombineAs is the boolean operation selector used by
// SSurface::MakeFromBoolean and the SShell/SMesh constructors that take
// a combine type. The numeric values (UNION=10, DIFFERENCE=11,
// INTERSECTION=12) differ from sketch.CombineAs (UNION=0, ...) on
// purpose.
type SurfaceCombineAs int

const (
	SurfaceUnion SurfaceCombineAs = 10
	SurfaceDiff  SurfaceCombineAs = 11
	SurfaceInter SurfaceCombineAs = 12
)

// String implements fmt.Stringer for debug logging.
func (c SurfaceCombineAs) String() string {
	switch c {
	case SurfaceUnion:
		return "UNION"
	case SurfaceDiff:
		return "DIFFERENCE"
	case SurfaceInter:
		return "INTERSECTION"
	default:
		return "UNKNOWN"
	}
}

// CombineAsFromGroup converts a sketch.Group::CombineAs into a
// SurfaceCombineAs. ASSEMBLE has no surface equivalent and is
// reported as an error.
func CombineAsFromGroup(c int) (SurfaceCombineAs, bool) {
	switch c {
	case 0:
		return SurfaceUnion, true
	case 1:
		return SurfaceDiff, true
	case 3:
		return SurfaceInter, true
	}
	return 0, false
}

// FaceID is a stable identifier for a face within a shell. Faces
// outlive individual SSurface instances because surfaces are split
// during boolean operations; the FaceID is what gets carried
// forward so colours, transparency, and selection survive.
type FaceID uint32

// FaceFlag controls mesh/shell rendering and CSG behavior. Mirrors
// SMesh's flipNormal/keepInsideOtherShell/keepCoplanar flags.
type FaceFlag uint32

const (
	FaceFlipNormal           FaceFlag = 1 << 0
	FaceKeepInsideOtherShell FaceFlag = 1 << 1
	FaceKeepCoplanar         FaceFlag = 1 << 2
	FaceAtLeastOneDiscarded  FaceFlag = 1 << 3
	FaceIsTransparent        FaceFlag = 1 << 4
)

// MeshMeta is the colour/face tag attached to one triangle of an SMesh
// or to one surface of an SShell.
type MeshMeta struct {
	Face FaceID
	core.RGBA
}

// NewMeshMeta constructs a MeshMeta with the given face id and color.
func NewMeshMeta(face FaceID, color core.RGBA) MeshMeta {
	return MeshMeta{Face: face, RGBA: color}
}
