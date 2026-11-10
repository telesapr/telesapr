//go:build darwin && cgo && !nogl

#define GL_SILENCE_DEPRECATION 1
#import <Cocoa/Cocoa.h>
#import <OpenGL/OpenGL.h>
#include "cgl_darwin.h"

// The view the context draws in. Mouse events fall through to the
// toolkit's view beneath it.
@interface TeleSAPRGLView : NSView
@end

@implementation TeleSAPRGLView
- (NSView *)hitTest:(NSPoint)p { return nil; }
- (BOOL)isOpaque { return YES; }
@end

@interface TeleSAPRGLBox : NSObject
@property(strong) NSOpenGLContext *ctx;
@property(strong) NSView *view;
@property(weak) NSView *parent;
@property int dbl;
@property double scale;
@end

@implementation TeleSAPRGLBox
@end

// onMain runs f on the main thread, which AppKit's view calls need;
// when the caller is on it already, directly.
static void onMain(dispatch_block_t f) {
	if ([NSThread isMainThread]) {
		f();
	} else {
		dispatch_sync(dispatch_get_main_queue(), f);
	}
}

// findView looks through the application's windows for the toolkit view
// with the ID wid. takigo hands out its own window IDs (small integers
// kept in a registry it doesn't export), not view pointers, and every
// one of its views has a windowID property.
static NSView *findView(NSView *v, uint64_t wid) {
	if ([v respondsToSelector:NSSelectorFromString(@"windowID")]) {
		NSNumber *id = [v valueForKey:@"windowID"];
		if (id != nil && [id unsignedLongLongValue] == wid) {
			return v;
		}
	}
	for (NSView *sub in v.subviews) {
		NSView *found = findView(sub, wid);
		if (found != nil) {
			return found;
		}
	}
	return nil;
}

static NSView *viewForID(uint64_t wid) {
	for (NSWindow *w in [NSApp windows]) {
		NSView *found = findView(w.contentView, wid);
		if (found != nil) {
			return found;
		}
	}
	return nil;
}

cglBox cglNew(uint64_t wid, const char **err) {
	__block TeleSAPRGLBox *box = nil;
	__block const char *why = NULL;
	onMain(^{
		NSView *parent = viewForID(wid);
		if (parent == nil) {
			why = "no view with the toolkit's window ID";
			return;
		}
		NSOpenGLPixelFormatAttribute attrs[] = {
			NSOpenGLPFADoubleBuffer,
			NSOpenGLPFAAccelerated,
			NSOpenGLPFAColorSize, 24,
			NSOpenGLPFAAlphaSize, 8,
			NSOpenGLPFADepthSize, 24,
			NSOpenGLPFAStencilSize, 8,
			NSOpenGLPFAOpenGLProfile, NSOpenGLProfileVersionLegacy,
			0};
		NSOpenGLPixelFormat *pf = [[NSOpenGLPixelFormat alloc] initWithAttributes:attrs];
		if (pf == nil) {
			why = "no accelerated pixel format";
			return;
		}
		NSOpenGLContext *ctx = [[NSOpenGLContext alloc] initWithFormat:pf shareContext:nil];
		if (ctx == nil) {
			why = "can't make the NSOpenGLContext";
			return;
		}
		NSView *v = [[TeleSAPRGLView alloc] initWithFrame:parent.bounds];
		// The surface has a pixel per device pixel, so a Retina screen
		// is sharp; the renderer sets its viewport by cglScale.
		[v setWantsBestResolutionOpenGLSurface:YES];
		[parent addSubview:v];
		[ctx setView:v];
		GLint one = 1;
		[ctx setValues:&one forParameter:NSOpenGLContextParameterSwapInterval];
		box = [[TeleSAPRGLBox alloc] init];
		box.ctx = ctx;
		box.view = v;
		box.parent = parent;
		box.dbl = 1;
		box.scale = parent.window.backingScaleFactor > 0 ? parent.window.backingScaleFactor : 1;
	});
	if (box == nil) {
		*err = why;
		return NULL;
	}
	return (__bridge_retained void *)box;
}

int cglMakeCurrent(cglBox b) {
	TeleSAPRGLBox *box = (__bridge TeleSAPRGLBox *)b;
	if (box.ctx == nil) {
		return 0;
	}
	onMain(^{
		NSView *parent = box.parent;
		if (parent == nil) {
			return;
		}
		double scale = parent.window.backingScaleFactor;
		if (scale <= 0) {
			scale = 1;
		}
		if (!NSEqualRects(box.view.frame, parent.bounds) || scale != box.scale) {
			[box.view setFrame:parent.bounds];
			box.scale = scale;
			[box.ctx update];
		}
	});
	[box.ctx makeCurrentContext];
	return [NSOpenGLContext currentContext] == box.ctx;
}

void cglSwap(cglBox b) {
	TeleSAPRGLBox *box = (__bridge TeleSAPRGLBox *)b;
	if (box.ctx != nil) {
		[box.ctx flushBuffer];
	}
}

void cglSize(cglBox b, int *w, int *h) {
	TeleSAPRGLBox *box = (__bridge TeleSAPRGLBox *)b;
	NSSize s = box.view.frame.size;
	*w = (int)s.width;
	*h = (int)s.height;
}

double cglScale(cglBox b) {
	return ((__bridge TeleSAPRGLBox *)b).scale;
}

int cglDouble(cglBox b) {
	return ((__bridge TeleSAPRGLBox *)b).dbl;
}

void cglDestroy(cglBox b) {
	TeleSAPRGLBox *box = (__bridge_transfer TeleSAPRGLBox *)b;
	onMain(^{
		if ([NSOpenGLContext currentContext] == box.ctx) {
			[NSOpenGLContext clearCurrentContext];
		}
		[box.ctx clearDrawable];
		[box.view removeFromSuperview];
		box.ctx = nil;
		box.view = nil;
	});
}
