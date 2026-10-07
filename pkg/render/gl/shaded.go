//go:build cgo && !nogl

package gl

import (
	"fmt"
	"strings"

	glapi "github.com/go-gl/gl/v2.1/gl"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/render"
	"github.com/telesapr/telesapr/pkg/srf"
)

// Shaded lighting in GLSL (roadmap 19.1): render.Shade for each pixel,
// the normal interpolated across the triangle, in place of GL's fixed
// function for each vertex. The program is GLSL 1.20, which GL 2.1 has,
// and reads the immediate mode's normals and colours, so a mesh is sent
// as before, its triangles a batch for each material.

const shadedVertex = `#version 120
varying vec3 n;
varying vec4 col;
void main() {
	n = gl_Normal;
	col = gl_Color;
	gl_Position = ftransform();
}
`

const shadedFragment = `#version 120
varying vec3 n;
varying vec4 col;
uniform vec3 lightDir[2];
uniform float lightI[2];
uniform float ambient;
uniform vec3 viewDir;
uniform vec3 upDir;
uniform float spec;
uniform float shin;
uniform float metal;
uniform int useBack;
uniform vec4 backCol;
void main() {
	vec3 N = normalize(n);
	vec4 base = col;
	if (!gl_FrontFacing) {
		N = -N;
		if (useBack == 1) base = backCol;
	}
	vec3 c = ambient * (0.75 + 0.25 * dot(N, upDir)) * base.rgb;
	for (int i = 0; i < 2; i++) {
		float d = dot(N, lightDir[i]);
		if (d > 0.0) {
			vec3 h = normalize(lightDir[i] + viewDir);
			float s = lightI[i] * spec * pow(max(dot(N, h), 0.0), shin);
			c += d * lightI[i] * base.rgb * (1.0 - 0.5 * metal) + s * mix(vec3(1.0), base.rgb, metal);
		}
	}
	gl_FragColor = vec4(clamp(c, 0.0, 1.0), base.a);
}
`

// shadedProgram is the renderer's compiled program, made the first
// time it is wanted.
type shadedProgram struct {
	id   uint32
	err  error
	made bool
	loc  map[string]int32
}

func compileShader(kind uint32, src string) (uint32, error) {
	sh := glapi.CreateShader(kind)
	cs, free := glapi.Strs(src + "\x00")
	glapi.ShaderSource(sh, 1, cs, nil)
	free()
	glapi.CompileShader(sh)
	var ok int32
	glapi.GetShaderiv(sh, glapi.COMPILE_STATUS, &ok)
	if ok == glapi.FALSE {
		var n int32
		glapi.GetShaderiv(sh, glapi.INFO_LOG_LENGTH, &n)
		log := strings.Repeat("\x00", int(n+1))
		glapi.GetShaderInfoLog(sh, n, nil, glapi.Str(log))
		return 0, fmt.Errorf("gl: shader: %s", strings.TrimRight(log, "\x00"))
	}
	return sh, nil
}

// program is the shaded program, compiled and linked once.
func (r *Renderer) program() (*shadedProgram, error) {
	p := &r.shaded
	if p.made {
		return p, p.err
	}
	p.made = true
	vs, err := compileShader(glapi.VERTEX_SHADER, shadedVertex)
	if err != nil {
		p.err = err
		return p, err
	}
	fs, err := compileShader(glapi.FRAGMENT_SHADER, shadedFragment)
	if err != nil {
		p.err = err
		return p, err
	}
	p.id = glapi.CreateProgram()
	glapi.AttachShader(p.id, vs)
	glapi.AttachShader(p.id, fs)
	glapi.LinkProgram(p.id)
	var ok int32
	glapi.GetProgramiv(p.id, glapi.LINK_STATUS, &ok)
	if ok == glapi.FALSE {
		p.err = fmt.Errorf("gl: the shaded program doesn't link")
		return p, p.err
	}
	p.loc = map[string]int32{}
	for _, name := range []string{"lightDir", "lightI", "ambient", "viewDir", "upDir", "spec", "shin", "metal", "useBack", "backCol"} {
		p.loc[name] = glapi.GetUniformLocation(p.id, glapi.Str(name+"\x00"))
	}
	return p, nil
}

// SetMaterials sets what gives each face's material when the lighting
// is Shaded.
func (r *Renderer) SetMaterials(of func(face uint32) render.Material) { r.materials = of }

// drawMeshShaded draws a mesh with the shaded program, a batch for
// each material, and reports false when the program can't be had, for
// the fixed function to draw it instead.
func (r *Renderer) drawMeshShaded(m *srf.SMesh, frontColor, backColor core.RGBA, twoSide bool) bool {
	p, err := r.program()
	if err != nil {
		return false
	}
	glapi.UseProgram(p.id)
	defer glapi.UseProgram(0)

	l := &r.lighting
	var dirs [6]float32
	var is [2]float32
	for i := range 2 {
		if l.LightIntensity[i] == 0 {
			continue
		}
		d := r.camera.VectorFromProjs(l.LightDirection[i]).WithMagnitude(1)
		dirs[3*i], dirs[3*i+1], dirs[3*i+2] = float32(d.X), float32(d.Y), float32(d.Z)
		is[i] = float32(l.LightIntensity[i])
	}
	glapi.Uniform3fv(p.loc["lightDir"], 2, &dirs[0])
	glapi.Uniform1fv(p.loc["lightI"], 2, &is[0])
	glapi.Uniform1f(p.loc["ambient"], float32(l.AmbientIntensity))
	view := r.camera.ProjRight.Cross(r.camera.ProjUp).WithMagnitude(1)
	up := r.camera.ProjUp.WithMagnitude(1)
	glapi.Uniform3f(p.loc["viewDir"], float32(view.X), float32(view.Y), float32(view.Z))
	glapi.Uniform3f(p.loc["upDir"], float32(up.X), float32(up.Y), float32(up.Z))
	if twoSide && !backColor.IsEmpty() {
		glapi.Uniform1i(p.loc["useBack"], 1)
		glapi.Uniform4f(p.loc["backCol"], backColor.RedF(), backColor.GreenF(), backColor.BlueF(), backColor.AlphaF())
	} else {
		glapi.Uniform1i(p.loc["useBack"], 0)
	}

	// The triangles by material, in their order within each.
	batches := map[render.Material][]int{}
	var order []render.Material
	for i := range m.Triangles {
		mat := render.Materials["plastic"]
		if r.materials != nil {
			mat = r.materials(uint32(m.Triangles[i].Meta.Face))
		}
		if _, ok := batches[mat]; !ok {
			order = append(order, mat)
		}
		batches[mat] = append(batches[mat], i)
	}
	for _, mat := range order {
		glapi.Uniform1f(p.loc["spec"], float32(mat.Specular))
		glapi.Uniform1f(p.loc["shin"], float32(mat.Shininess))
		glapi.Uniform1f(p.loc["metal"], float32(mat.Metallic))
		glapi.Begin(glapi.TRIANGLES)
		for _, i := range batches[mat] {
			tr := &m.Triangles[i]
			c := frontColor
			if c.IsEmpty() {
				c = tr.Meta.RGBA
			}
			colorRGBA(c)
			if tr.NA.EqualsExactly(geom.Vector{}) {
				normal(tr.Normal())
				vertex(tr.A)
				vertex(tr.B)
				vertex(tr.C)
			} else {
				normal(tr.NA)
				vertex(tr.A)
				normal(tr.NB)
				vertex(tr.B)
				normal(tr.NC)
				vertex(tr.C)
			}
		}
		glapi.End()
	}
	return true
}
