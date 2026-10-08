package export

import (
	"bytes"
	"cmp"
	"path/filepath"
	"slices"
	"strings"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/internal/cfmt"
	"github.com/telesapr/telesapr/pkg/res"
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/version"
)

// ExportMeshAsObjTo writes the mesh as Wavefront OBJ, with its
// materials.
func WriteOBJ(fObj, fMtl *bytes.Buffer, sm *srf.SMesh, o Options) {
	colors := map[uint32]string{}
	var colorKeys []core.RGBA
	for _, t := range sm.Triangles {
		color := t.Meta.RGBA
		if _, ok := colors[color.ToARGB32()]; !ok {
			id := cfmt.Sprintf("h%02x%02x%02x", color.Red, color.Green, color.Blue)
			colors[color.ToARGB32()] = id
			colorKeys = append(colorKeys, color)
		}
		for _, vtx := range []geom.Vector{t.A, t.B, t.C} {
			p := vtx.ScaledBy(1 / o.Scale)
			fObj.WriteString(cfmt.Sprintf("v %.10f %.10f %.10f\n", p.X, p.Y, p.Z))
		}
	}

	// std::map orders the colours by their ARGB value.
	slices.SortStableFunc(colorKeys, func(a, b core.RGBA) int { return cmp.Compare(a.ToARGB32(), b.ToARGB32()) })
	for _, c := range colorKeys {
		fMtl.WriteString(cfmt.Sprintf("newmtl %s\n", colors[c.ToARGB32()]))
		fMtl.WriteString(cfmt.Sprintf("Kd %.3f %.3f %.3f\n", c.RedF(), c.GreenF(), c.BlueF()))
	}

	for _, t := range sm.Triangles {
		for _, nv := range []geom.Vector{t.NA, t.NB, t.NC} {
			n := nv.WithMagnitude(1.0)
			fObj.WriteString(cfmt.Sprintf("vn %.10f %.10f %.10f\n", n.X, n.Y, n.Z))
		}
	}

	var currentColor core.RGBA
	for i, t := range sm.Triangles {
		if !currentColor.Equals(t.Meta.RGBA) {
			currentColor = t.Meta.RGBA
			fObj.WriteString(cfmt.Sprintf("usemtl %s\n", colors[currentColor.ToARGB32()]))
		}

		fObj.WriteString(cfmt.Sprintf("f %d//%d %d//%d %d//%d\n",
			i*3+1, i*3+1,
			i*3+2, i*3+2,
			i*3+3, i*3+3))
	}
}

// fileStemIdentifier is the file's name without extension, with
// everything but letters, digits and non-ASCII bytes made '_'.
func fileStemIdentifier(filename string) string {
	base := filepath.Base(filename)
	if dot := strings.LastIndexByte(base, '.'); dot >= 0 {
		base = base[:dot]
	}
	b := []byte(base)
	for i, c := range b {
		isAlnum := (c >= '0' && c <= '9') || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
		if !isAlnum && c < 0x80 {
			b[i] = '_'
		}
	}
	return string(b)
}

const threeJsHTMLBegin = `
<!DOCTYPE html>
<html lang="en">
  <head>
    <meta charset="utf-8"></meta>
    <title>Three.js TeleSAPR Mesh</title>
    <script id="%s">%s</script>
    <script id="%s">%s</script>
    <script id="%s">%s</script>
    <style type="text/css">
    body { margin: 0; overflow: hidden; }
    </style>
  </head>
  <body>
    <script>
`

const threeJsHTMLEnd = `
    document.body.appendChild(telesapr(telesapr_model_%s, {
        scale: %g,
        offset: new THREE.Vector3(%g, %g, %g),
        projUp: new THREE.Vector3(%g, %g, %g),
        projRight: new THREE.Vector3(%g, %g, %g)
    }));
    </script>
  </body>
</html>
`

// ExportMeshAsThreeJsTo writes the mesh as a script for three.js, or a
// page with it.
func WriteThreeJS(f *bytes.Buffer, filename string, sm *srf.SMesh,
	sol *srf.SOutlineList, view *sketch.View, o Options) {
	const threeFn = "three-r111.min.js"
	const hammerFn = "hammer-2.0.8.js"
	const controlsFn = "TeleSAPRControls.js"
	pf := func(format string, args ...any) { f.WriteString(cfmt.Sprintf(format, args...)) }

	// A default three.js viewer with OrthographicTrackballControls is
	// generated as a comment preceding the data.

	// x bounds should be the range of x or y, whichever is larger,
	// before aspect ratio correction is applied. y bounds should be the
	// range of x or y, whichever is larger. No aspect ratio correction
	// is applied. Near plane should be 1. Camera's z-position should be
	// the range of z + 1 or the larger of the x or y bounds, whichever
	// is larger. Far plane should be at least twice as much as the
	// camera's z-position. Edge projection bias should be about 1/500 of
	// the far plane's distance. Further corrections will be applied to
	// the z-position and far plane in the default viewer, but the
	// defaults are fine for a model which only rotates about the world
	// origin.

	bndh, bndl := sm.Bounding()
	largerBoundXY := max(bndh.X-bndl.X, bndh.Y-bndl.Y)
	largerBoundZ := max(largerBoundXY, bndh.Z-bndl.Z+1)

	basename := fileStemIdentifier(filename)

	if hasExtension(filename, "html") {
		pf(threeJsHTMLBegin,
			threeFn, res.LoadStringFromGzip("threejs/"+threeFn+".gz"),
			hammerFn, res.LoadStringFromGzip("threejs/"+hammerFn+".gz"),
			controlsFn, res.LoadString("threejs/"+controlsFn))
	}

	pf("var telesapr_model_%s = {\n"+
		"  bounds: {\n"+
		"    x: %f, y: %f, near: %f, far: %f, z: %f, edgeBias: %f\n"+
		"  },\n",
		basename,
		largerBoundXY,
		largerBoundXY,
		1.0,
		largerBoundZ*2,
		largerBoundZ,
		largerBoundZ/250)

	// Output lighting information.
	f.WriteString("  lights: {\n" +
		"    d: [\n")

	// Directional.
	for lightCount := range 2 {
		d := o.Lights[lightCount].Dir
		pf("      {\n"+
			"        intensity: %f, direction: [%f, %f, %f]\n"+
			"      },\n",
			o.Lights[lightCount].Intensity, d.X, d.Y, d.Z)
	}

	// Global Ambience.
	pf("    ],\n"+
		"    a: %f\n", o.Ambient)

	var spl srf.SPointList
	for _, tr := range sm.Triangles {
		spl.IncrementTagFor(tr.A)
		spl.IncrementTagFor(tr.B)
		spl.IncrementTagFor(tr.C)
	}

	// Output all the vertices.
	f.WriteString("  },\n" +
		"  points: [\n")
	for _, sp := range spl.Points {
		pf("    [%f, %f, %f],\n",
			sp.P.X/o.Scale, sp.P.Y/o.Scale, sp.P.Z/o.Scale)
	}

	f.WriteString("  ],\n" +
		"  faces: [\n")
	// And now all the triangular faces, in terms of those vertices. This
	// time we count from zero.
	for _, tr := range sm.Triangles {
		pf("    [%d, %d, %d],\n",
			spl.IndexForPoint(tr.A), spl.IndexForPoint(tr.B), spl.IndexForPoint(tr.C))
	}

	// Output face normals.
	f.WriteString("  ],\n" +
		"  normals: [\n")
	for _, tr := range sm.Triangles {
		pf("    [[%f, %f, %f], [%f, %f, %f], [%f, %f, %f]],\n",
			tr.NA.X, tr.NA.Y, tr.NA.Z, tr.NB.X, tr.NB.Y, tr.NB.Z, tr.NC.X, tr.NC.Y, tr.NC.Z)
	}

	f.WriteString("  ],\n" +
		"  colors: [\n")
	// Output triangle colors.
	for _, tr := range sm.Triangles {
		pf("    0x%x,\n", tr.Meta.ToARGB32())
	}

	f.WriteString("  ],\n" +
		"  edges: [\n")
	// Output edges. Assume user's model colors do not obscure white
	// edges.
	for _, so := range sol.Outlines {
		if so.Tag == 0 {
			continue
		}
		pf("    [[%f, %f, %f], [%f, %f, %f]],\n",
			so.A.X/o.Scale, so.A.Y/o.Scale, so.A.Z/o.Scale,
			so.B.X/o.Scale, so.B.Y/o.Scale, so.B.Z/o.Scale)
	}

	f.WriteString("  ]\n};\n")

	if hasExtension(filename, "html") {
		v := view
		pf(threeJsHTMLEnd,
			basename,
			v.Scale,
			v.Offset.X, v.Offset.Y, v.Offset.Z,
			v.ProjUp.X, v.ProjUp.Y, v.ProjUp.Z,
			v.ProjRight.X, v.ProjRight.Y, v.ProjRight.Z)
	}
}

// ExportMeshAsVrmlTo writes the mesh as VRML.
func WriteVRML(f *bytes.Buffer, filename string, sm *srf.SMesh, o Options) {
	pf := func(format string, args ...any) { f.WriteString(cfmt.Sprintf(format, args...)) }
	basename := fileStemIdentifier(filename)

	pf("#VRML V2.0 utf8\n"+
		"#Exported from TeleSAPR %s\n"+
		"\n"+
		"DEF %s Transform {\n"+
		"  children [",
		version.Version,
		basename)

	// The runs of triangles of each opacity, by opacity.
	type span struct{ first, pastLast int }
	opacities := map[uint8][]span{}
	start := 0
	lastOpacity := sm.Triangles[0].Meta.Alpha
	for i, tr := range sm.Triangles {
		if tr.Meta.Alpha != lastOpacity {
			opacities[lastOpacity] = append(opacities[lastOpacity], span{start, i})
			start = i
			lastOpacity = sm.Triangles[start].Meta.Alpha
		}
	}
	opacities[lastOpacity] = append(opacities[lastOpacity], span{start, len(sm.Triangles)})
	var keys []int
	for k := range opacities {
		keys = append(keys, int(k))
	}
	slices.Sort(keys)

	for _, k := range keys {
		op := uint8(k)
		spans := opacities[op]
		pf("\n"+
			"    Shape {\n"+
			"      appearance Appearance {\n"+
			"        material DEF %s_material_%u Material {\n"+
			"          diffuseColor %f %f %f\n"+
			"          ambientIntensity %f\n"+
			"          transparency %f\n"+
			"        }\n"+
			"      }\n"+
			"      geometry IndexedFaceSet {\n"+
			"        colorPerVertex TRUE\n"+
			"        coord Coordinate { point [\n",
			basename,
			uint(op),
			o.Ambient,
			o.Ambient,
			o.Ambient,
			o.Ambient,
			float64(float32(1)-(float32(op)/float32(255))))

		var spl srf.SPointList
		for _, sp := range spans {
			for _, tr := range sm.Triangles[sp.first:sp.pastLast] {
				spl.IncrementTagFor(tr.A)
				spl.IncrementTagFor(tr.B)
				spl.IncrementTagFor(tr.C)
			}
		}

		// Output all the vertices.
		for _, sp := range spl.Points {
			pf("          %f %f %f,\n",
				sp.P.X/o.Scale, sp.P.Y/o.Scale, sp.P.Z/o.Scale)
		}

		f.WriteString("        ] }\n" +
			"        coordIndex [\n")
		// And now all the triangular faces, in terms of those vertices.
		for _, sp := range spans {
			for _, tr := range sm.Triangles[sp.first:sp.pastLast] {
				pf("          %d, %d, %d, -1,\n",
					spl.IndexForPoint(tr.A), spl.IndexForPoint(tr.B), spl.IndexForPoint(tr.C))
			}
		}

		f.WriteString("        ]\n" +
			"        color Color { color [\n")
		// Output triangle colors.
		var triangleColourIDs []int
		var coloursPresent []core.RGBA
		for _, sp := range spans {
			for _, tr := range sm.Triangles[sp.first:sp.pastLast] {
				found := -1
				for ci, c := range coloursPresent {
					if c.Equals(tr.Meta.RGBA) {
						found = ci
						break
					}
				}
				if found < 0 {
					pf("          %.10f %.10f %.10f,\n",
						tr.Meta.RedF(), tr.Meta.GreenF(), tr.Meta.BlueF())
					triangleColourIDs = append(triangleColourIDs, len(coloursPresent))
					coloursPresent = append(coloursPresent, tr.Meta.RGBA)
				} else {
					triangleColourIDs = append(triangleColourIDs, found)
				}
			}
		}

		f.WriteString("        ] }\n" +
			"        colorIndex [\n")

		for _, idx := range triangleColourIDs {
			pf("          %d, %d, %d, -1,\n", idx, idx, idx)
		}

		f.WriteString("        ]\n" +
			"      }\n" +
			"    }\n")
	}

	f.WriteString("  ]\n" +
		"}\n")
}
