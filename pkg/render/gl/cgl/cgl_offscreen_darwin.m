//go:build darwin && cgo && !nogl

// The offscreen context: plain CGL, no AppKit, no window, so it needs
// neither the main thread nor a running application, which a test
// binary has not. Drawing goes into a framebuffer object of the
// context's own, since a context with no drawable has no default
// framebuffer to draw in.

#define GL_SILENCE_DEPRECATION 1
#define GL_GLEXT_PROTOTYPES 1
#include <stddef.h>
#include <OpenGL/OpenGL.h>
#include <OpenGL/gl.h>
#include <OpenGL/glext.h>
#include "cgl_darwin.h"

// chooseFormat takes the first pixel format of the list that exists:
// an accelerated one, then the software renderer, which is what a
// machine with no GPU (a virtual one in CI) has. Both are legacy
// profile, the fixed-function GL 2.1 the renderer draws with; the
// accelerated one may be on any GPU, offline ones included, since
// nothing is shown.
static CGLPixelFormatObj chooseFormat(int *kind) {
	CGLPixelFormatAttribute accelerated[] = {
		kCGLPFAAccelerated,
		kCGLPFAAllowOfflineRenderers,
		kCGLPFAColorSize, (CGLPixelFormatAttribute)24,
		kCGLPFAAlphaSize, (CGLPixelFormatAttribute)8,
		kCGLPFADepthSize, (CGLPixelFormatAttribute)24,
		kCGLPFAStencilSize, (CGLPixelFormatAttribute)8,
		kCGLPFAOpenGLProfile, (CGLPixelFormatAttribute)kCGLOGLPVersion_Legacy,
		(CGLPixelFormatAttribute)0};
	CGLPixelFormatAttribute software[] = {
		kCGLPFARendererID, (CGLPixelFormatAttribute)kCGLRendererGenericFloatID,
		kCGLPFAColorSize, (CGLPixelFormatAttribute)24,
		kCGLPFAAlphaSize, (CGLPixelFormatAttribute)8,
		kCGLPFADepthSize, (CGLPixelFormatAttribute)24,
		kCGLPFAStencilSize, (CGLPixelFormatAttribute)8,
		kCGLPFAOpenGLProfile, (CGLPixelFormatAttribute)kCGLOGLPVersion_Legacy,
		(CGLPixelFormatAttribute)0};
	CGLPixelFormatAttribute *lists[2] = {accelerated, software};
	for (int i = 0; i < 2; i++) {
		CGLPixelFormatObj pf = NULL;
		GLint n = 0;
		if (CGLChoosePixelFormat(lists[i], &pf, &n) == kCGLNoError && pf != NULL && n > 0) {
			*kind = i + 1;
			return pf;
		}
		if (pf != NULL) {
			CGLReleasePixelFormat(pf);
		}
	}
	return NULL;
}

// attach makes the framebuffer object of the current context: RGBA8
// colour and packed depth and stencil, as renderbuffers. It returns
// the reason it can't, or NULL.
static const char *attach(int w, int h) {
	GLint max = 0;
	glGetIntegerv(GL_MAX_RENDERBUFFER_SIZE_EXT, &max);
	if (w > max || h > max) {
		return "larger than the largest renderbuffer";
	}
	GLuint fbo = 0, color = 0, depth = 0;
	glGenFramebuffersEXT(1, &fbo);
	glBindFramebufferEXT(GL_FRAMEBUFFER_EXT, fbo);
	glGenRenderbuffersEXT(1, &color);
	glBindRenderbufferEXT(GL_RENDERBUFFER_EXT, color);
	glRenderbufferStorageEXT(GL_RENDERBUFFER_EXT, GL_RGBA8, w, h);
	glFramebufferRenderbufferEXT(GL_FRAMEBUFFER_EXT, GL_COLOR_ATTACHMENT0_EXT,
		GL_RENDERBUFFER_EXT, color);
	glGenRenderbuffersEXT(1, &depth);
	glBindRenderbufferEXT(GL_RENDERBUFFER_EXT, depth);
	glRenderbufferStorageEXT(GL_RENDERBUFFER_EXT, GL_DEPTH24_STENCIL8_EXT, w, h);
	glFramebufferRenderbufferEXT(GL_FRAMEBUFFER_EXT, GL_DEPTH_ATTACHMENT_EXT,
		GL_RENDERBUFFER_EXT, depth);
	glFramebufferRenderbufferEXT(GL_FRAMEBUFFER_EXT, GL_STENCIL_ATTACHMENT_EXT,
		GL_RENDERBUFFER_EXT, depth);
	if (glGetError() != GL_NO_ERROR) {
		return "can't make the renderbuffers";
	}
	if (glCheckFramebufferStatusEXT(GL_FRAMEBUFFER_EXT) != GL_FRAMEBUFFER_COMPLETE_EXT) {
		return "the framebuffer is incomplete";
	}
	return NULL;
}

cglOff cglOffNew(int w, int h, const char **err, int *renderer) {
	int kind = 0;
	CGLPixelFormatObj pf = chooseFormat(&kind);
	if (pf == NULL) {
		*err = "no pixel format, accelerated or software";
		return NULL;
	}
	CGLContextObj ctx = NULL;
	CGLError e = CGLCreateContext(pf, NULL, &ctx);
	CGLReleasePixelFormat(pf);
	if (e != kCGLNoError || ctx == NULL) {
		*err = e != kCGLNoError ? CGLErrorString(e) : "can't make the context";
		return NULL;
	}
	CGLContextObj prev = CGLGetCurrentContext();
	const char *why = "can't make the context current";
	if (CGLSetCurrentContext(ctx) == kCGLNoError) {
		why = attach(w, h);
	}
	// The framebuffer binding belongs to the context, so it stays when
	// the caller's context is back.
	CGLSetCurrentContext(prev);
	if (why != NULL) {
		CGLReleaseContext(ctx);
		*err = why;
		return NULL;
	}
	*renderer = kind;
	return ctx;
}

int cglOffMakeCurrent(cglOff o) {
	CGLContextObj ctx = (CGLContextObj)o;
	if (CGLGetCurrentContext() == ctx) {
		return 1;
	}
	return CGLSetCurrentContext(ctx) == kCGLNoError;
}

void cglOffDestroy(cglOff o) {
	CGLContextObj ctx = (CGLContextObj)o;
	if (CGLGetCurrentContext() == ctx) {
		CGLSetCurrentContext(NULL);
	}
	// The framebuffer and its renderbuffers go with the context.
	CGLReleaseContext(ctx);
}
