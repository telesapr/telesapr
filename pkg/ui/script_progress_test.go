package ui

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/telesapr/telesapr/pkg/script/agent"
)

// A program's script run in the window reports, through the
// connection, the groups the regeneration after it begins and what it
// printed, before its result. The script waits past the pacer's
// interval before it prints: reports closer together are merged, and
// the line's report, which has no count, would take the place of the
// regeneration's on a loaded machine.
func TestAgentRunProgress(t *testing.T) {
	t.Parallel()
	a := newAgentApp(t)
	var (
		mu  sync.Mutex
		got []agent.Progress
	)
	res, err := a.r.RunWithProgress(context.Background(),
		"doc.Sketch.Rect(V(0, 0, 0), V(10, 20, 0))\ndoc.Features.Extrude(5, \"plate\")\ntime.Sleep(time.Second)\nfmt.Println(\"extruded\")",
		func(p agent.Progress) {
			mu.Lock()
			defer mu.Unlock()
			got = append(got, p)
		})
	if err != nil || res.Error != "" {
		t.Fatalf("the script gave %+v, %v", res, err)
	}
	mu.Lock()
	defer mu.Unlock()
	var regenerated, printed bool
	for _, p := range got {
		regenerated = regenerated || (strings.HasPrefix(p.Message, "regenerating g") && p.Total >= 2 && p.Done >= 1)
		printed = printed || strings.Contains(p.Output, "extruded\n")
	}
	if !regenerated || !printed {
		t.Errorf("the reports were %+v; want a group regenerated and the line printed", got)
	}
	// After the run, nothing watches the regenerations.
	var watched bool
	a.on(func() { watched = a.ss.regenWatch.Load() != nil })
	if watched {
		t.Error("the regenerations are still watched after the run")
	}
}
