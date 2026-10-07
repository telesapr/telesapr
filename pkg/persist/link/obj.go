package link

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/srf"
)

// An OBJ file linked as a reference mesh (roadmap 16.6): its vertices
// ("v x y z") and faces ("f a b c ...", each corner a vertex's number
// from 1, or from -1 back from the last, before any "/"), a face of
// more than three corners cut into a fan. Faces take the diffuse
// colour ("Kd r g b", and "d" for opacity) of the material named by the
// last "usemtl", from the files "mtllib" names beside it; grey without.
// Texture coordinates, normals, lines and groups are not read.

// linkObj reads an OBJ file into mesh m with the entities a linked STL
// file gets.
func linkObj(path string, sk *sketch.Sketch, el *[]core.Entity, m *srf.SMesh) bool {
	*el = (*el)[:0]
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	tris, err := objTriangles(data, filepath.Dir(path))
	if err != nil {
		if sk.Message != nil {
			sk.Message(fmt.Sprintf("The OBJ file '%s' can't be read: %v.", path, err))
		}
		return false
	}
	linkMesh(tris, el, m)
	return true
}

// objTriangles are the faces of an OBJ file, its materials' files read
// from dir.
func objTriangles(data []byte, dir string) ([]srf.STriangle, error) {
	var verts []geom.Vector
	var tris []srf.STriangle
	materials := map[string]core.RGBA{}
	color := meshGrey
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 1<<16), 1<<24)
	for line := 1; sc.Scan(); line++ {
		f := strings.Fields(sc.Text())
		if len(f) == 0 || strings.HasPrefix(f[0], "#") {
			continue
		}
		switch f[0] {
		case "v":
			if len(f) < 4 {
				return nil, fmt.Errorf("line %d: a vertex needs three coordinates", line)
			}
			var p [3]float64
			for k := range p {
				v, err := strconv.ParseFloat(f[k+1], 64)
				if err != nil {
					return nil, fmt.Errorf("line %d: %w", line, err)
				}
				p[k] = v
			}
			verts = append(verts, geom.Vector{X: p[0], Y: p[1], Z: p[2]})
		case "f":
			if len(f) < 4 {
				return nil, fmt.Errorf("line %d: a face needs three corners", line)
			}
			var corners []geom.Vector
			for _, c := range f[1:] {
				num, _, _ := strings.Cut(c, "/")
				k, err := strconv.Atoi(num)
				if err != nil {
					return nil, fmt.Errorf("line %d: %w", line, err)
				}
				if k < 0 {
					k += len(verts) + 1
				}
				if k < 1 || k > len(verts) {
					return nil, fmt.Errorf("line %d: there is no vertex %s", line, num)
				}
				corners = append(corners, verts[k-1])
			}
			for k := 2; k < len(corners); k++ {
				tr := srf.STriangle{Meta: srf.STriMeta{RGBA: color}, A: corners[0], B: corners[k-1], C: corners[k]}
				n := tr.Normal().WithMagnitude(1)
				tr.NA, tr.NB, tr.NC = n, n, n
				tris = append(tris, tr)
			}
		case "mtllib":
			for _, name := range f[1:] {
				if mtl, err := os.ReadFile(filepath.Join(dir, name)); err == nil {
					readMtl(mtl, materials)
				}
			}
		case "usemtl":
			color = meshGrey
			if len(f) > 1 {
				if c, ok := materials[f[1]]; ok {
					color = c
				}
			}
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if len(tris) == 0 {
		return nil, errors.New("it has no faces")
	}
	return tris, nil
}

// readMtl adds the diffuse colours of a material file's materials.
func readMtl(data []byte, materials map[string]core.RGBA) {
	var name string
	unit := func(s string) uint8 {
		v, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return 0
		}
		return uint8(max(0, min(1, v))*255 + 0.5)
	}
	for l := range strings.SplitSeq(string(data), "\n") {
		f := strings.Fields(l)
		if len(f) == 0 {
			continue
		}
		switch f[0] {
		case "newmtl":
			if len(f) > 1 {
				name = f[1]
				materials[name] = meshGrey
			}
		case "Kd":
			if name != "" && len(f) >= 4 {
				c := materials[name]
				c.Red, c.Green, c.Blue = unit(f[1]), unit(f[2]), unit(f[3])
				materials[name] = c
			}
		case "d":
			if name != "" && len(f) >= 2 {
				c := materials[name]
				c.Alpha = unit(f[1])
				materials[name] = c
			}
		}
	}
}
