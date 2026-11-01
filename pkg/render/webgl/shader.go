//go:build js && wasm

package webgl

// The program is GLSL ES 3.00, which WebGL 2 has. One program draws
// everything, the mode uniform picking how a fragment is coloured: the
// vertex colour as it is, GL's fixed-function lighting (the model
// ambient plus each light's diffuse term) or render.Shade, as the GL
// renderer's shaded program does (pkg/render/gl/shaded.go). Lighting is
// each pixel's, from the normal interpolated across the triangle.

const vertexSource = `#version 300 es
layout(location = 0) in vec3 aPos;
layout(location = 1) in vec3 aNrm;
layout(location = 2) in vec4 aCol;
layout(location = 3) in vec2 aUV;
uniform mat4 uMVP;
out vec3 vPos;
out vec3 vNrm;
out vec4 vCol;
out vec2 vUV;
void main() {
	vPos = aPos;
	vNrm = aNrm;
	vCol = aCol;
	vUV = aUV;
	gl_Position = uMVP * vec4(aPos, 1.0);
}
`

const fragmentSource = `#version 300 es
precision highp float;
precision highp int;
precision highp sampler2D;
in vec3 vPos;
in vec3 vNrm;
in vec4 vCol;
in vec2 vUV;
out vec4 outColor;

uniform int uMode;      // 0 flat, 1 lit, 2 shaded
uniform int uPattern;   // 0 solid, 1 and 2 the checkered fills
uniform int uTexMode;   // 0 none, 1 RGBA, 2 RGB, 3 alpha only
uniform sampler2D uTex;
uniform bool uClipOn;
uniform vec4 uClip;
uniform vec3 uLightDir0;
uniform vec3 uLightDir1;
uniform float uLightI0;
uniform float uLightI1;
uniform float uAmbient;
uniform vec3 uView;
uniform vec3 uUp;
uniform float uSpec;
uniform float uShin;
uniform float uMetal;
uniform bool uTwoSide;
uniform bool uUseBack;
uniform vec4 uBackCol;

vec3 light(vec3 N, vec3 L, float I, vec3 base) {
	float d = dot(N, L);
	if (d <= 0.0 || I == 0.0) {
		return vec3(0.0);
	}
	if (uMode == 1) {
		return d * I * base;
	}
	vec3 h = normalize(L + uView);
	float s = I * uSpec * pow(max(dot(N, h), 0.0), uShin);
	return d * I * base * (1.0 - 0.5 * uMetal) + s * mix(vec3(1.0), base, uMetal);
}

void main() {
	if (uClipOn && dot(uClip.xyz, vPos) + uClip.w < 0.0) {
		discard;
	}
	if (uPattern != 0) {
		// The polygon stipple masks: the checkers of a 4x4 cell, A on
		// the corners and B on the middle.
		ivec2 q = ivec2(gl_FragCoord.xy) & 3;
		bool mid = q.x >= 1 && q.x <= 2 && q.y >= 1 && q.y <= 2;
		bool corner = (q.x == 3 || q.x == 0) && (q.y == 3 || q.y == 0);
		if (uPattern == 1 ? !corner : !mid) {
			discard;
		}
	}
	vec4 base = vCol;
	if (uTexMode != 0) {
		vec4 t = texture(uTex, vUV);
		if (uTexMode == 1) {
			base = t;
		} else if (uTexMode == 2) {
			base = vec4(t.rgb, base.a);
		} else {
			base = vec4(base.rgb, t.r);
		}
	}
	if (uMode == 0) {
		outColor = base;
		return;
	}
	vec3 N = normalize(vNrm);
	if (!gl_FrontFacing && uTwoSide) {
		N = -N;
		if (uUseBack) {
			base = uBackCol;
		}
	}
	vec3 c;
	if (uMode == 1) {
		c = uAmbient * base.rgb;
	} else {
		c = uAmbient * (0.75 + 0.25 * dot(N, uUp)) * base.rgb;
	}
	c += light(N, uLightDir0, uLightI0, base.rgb);
	c += light(N, uLightDir1, uLightI1, base.rgb);
	outColor = vec4(clamp(c, 0.0, 1.0), base.a);
}
`
