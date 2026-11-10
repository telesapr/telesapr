//go:build darwin && cgo && !nogl

#ifndef TELESAPR_CGL_H
#define TELESAPR_CGL_H

// A context and the view it draws in; opaque to Go.
typedef void *cglBox;

// cglNew makes a context in a new view laid over parent, an NSView. It
// returns NULL and the reason in *err (a static string) on failure.
cglBox cglNew(void *parent, const char **err);
// cglMakeCurrent follows the parent's size, then binds the context to
// the calling thread.
int cglMakeCurrent(cglBox b);
void cglSwap(cglBox b);
void cglSize(cglBox b, int *w, int *h);
int cglDouble(cglBox b);
void cglDestroy(cglBox b);

#endif
