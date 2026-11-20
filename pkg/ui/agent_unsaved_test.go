package ui

import (
	"context"
	"testing"

	"github.com/telesapr/telesapr/pkg/script/agent"
)

// A program's reads leave a saved drawing saved: a picture, a
// description or a measurement of a group other than the active one
// changes nothing, so that an open after them is not refused for
// changes there are none of.
func TestAgentReadsKeepSaved(t *testing.T) {
	t.Parallel()
	a := newAgentApp(t)
	ctx := context.Background()
	if res, err := a.r.Run(ctx, "doc.Sketch.Rect(V(0, 0, 0), V(10, 20, 0))\ndoc.Features.Extrude(5, \"plate\")\n"+
		"doc.Activate(doc.Group(\"g002\"))\ndoc.File.ForgetChanges()"); err != nil || res.Error != "" {
		t.Fatalf("the plate gave %+v, %v", res, err)
	}
	unsaved := func() (u bool) {
		a.on(func() { u = a.ss.SK.Unsaved })
		return u
	}
	if unsaved() {
		t.Fatal("the drawing is unsaved before anything is read")
	}
	for _, c := range []struct {
		what string
		read func() error
	}{
		{"a picture", func() error { _, err := a.r.Render(ctx, agent.Picture{View: "top", Width: 40, Height: 30}); return err }},
		{"a picture of another group", func() error {
			_, err := a.r.Render(ctx, agent.Picture{View: "top", Group: "g003", Width: 40, Height: 30})
			return err
		}},
		{"a section", func() error {
			_, err := a.r.Render(ctx, agent.Picture{Width: 40, Height: 30, Section: "YZ", SectionOffset: "5"})
			return err
		}},
		{"a description", func() error { _, err := a.r.Describe(ctx, agent.Describing{}); return err }},
		{"a description of another group", func() error { _, err := a.r.Describe(ctx, agent.Describing{Group: "g003"}); return err }},
		{"the solid of another group", func() error { _, err := a.r.Describe(ctx, agent.Describing{Group: "g003", Solid: true}); return err }},
		{"the drawing as a script", func() error { _, err := a.r.Describe(ctx, agent.Describing{Script: true}); return err }},
		{"a measurement of another group", func() error { _, err := a.r.Measure(ctx, agent.Measuring{Group: "g003"}); return err }},
	} {
		if err := c.read(); err != nil {
			t.Errorf("%s: %v", c.what, err)
		}
		if unsaved() {
			t.Errorf("%s left the drawing unsaved", c.what)
			a.on(func() { a.ss.SK.Unsaved = false })
		}
	}
	if res, err := a.r.Run(ctx, "doc.Active().Name()"); err != nil || res.Error != "" || res.Value != `"sketch-in-plane"` {
		t.Errorf("the active group after the reads: %+v, %v", res, err)
	}
	a.on(func() { a.ss.SK.Unsaved = false })
	// A picture of another group of a drawing that has changes keeps
	// them.
	a.on(func() { a.ss.SK.Unsaved = true })
	if _, err := a.r.Render(ctx, agent.Picture{View: "top", Group: "g003", Width: 40, Height: 30}); err != nil || !unsaved() {
		t.Errorf("a picture of a changed drawing: %v, unsaved %v", err, unsaved())
	}
}
