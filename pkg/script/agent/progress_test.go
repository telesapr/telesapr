package agent

import (
	"context"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/telesapr/telesapr/pkg/script"
	"github.com/telesapr/telesapr/pkg/script/api"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// reports keeps the reports a run sends, for a test to read after.
type reports struct {
	mu   sync.Mutex
	list []Progress
}

func (r *reports) add(p Progress) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.list = append(r.list, p)
}

func (r *reports) all() []Progress {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]Progress(nil), r.list...)
}

// output is what the reports say was printed, together.
func output(ps []Progress) string {
	var b strings.Builder
	for _, p := range ps {
		b.WriteString(p.Output)
	}
	return b.String()
}

// The pacer sends the first report at once, merges what comes before
// the next is due, and sends the last before close returns.
func TestPacer(t *testing.T) {
	t.Parallel()
	var got reports
	p := newPacer(got.add, time.Hour)
	p.add(Progress{Message: "a", Output: "a\n"})
	deadline := time.Now().Add(5 * time.Second)
	for len(got.all()) == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	p.add(Progress{Message: "b", Output: "b\n"})
	p.add(Progress{Message: "regenerating g002-x, 1 of 2", Done: 1, Total: 2})
	p.close()
	want := []Progress{{Message: "a", Output: "a\n"}, {Message: "regenerating g002-x, 1 of 2", Done: 1, Total: 2, Output: "b\n"}}
	if all := got.all(); !reflect.DeepEqual(all, want) {
		t.Errorf("the pacer sent %+v, want %+v", all, want)
	}
}

// A script that prints in a loop has its lines reported, all of them
// in order, in fewer reports than lines, the last before Run returns.
func TestSessionReportsLines(t *testing.T) {
	t.Parallel()
	s := newSession(t)
	var got reports
	res, err := s.RunWithProgress(context.Background(),
		"for i := 0; i < 40; i++ {\n\tfmt.Println(\"line\", i)\n\ttime.Sleep(20 * time.Millisecond)\n}\nfmt.Print(\"no end\")", got.add)
	if err != nil || res.Error != "" {
		t.Fatalf("the script gave %+v, %v", res, err)
	}
	all := got.all()
	if want := res.Output + "\n"; output(all) != want {
		t.Errorf("the reports printed %q, want %q", output(all), want)
	}
	if len(all) < 2 || len(all) >= 40 {
		t.Errorf("40 lines in 0.8 s came in %d reports; want them paced to a few a second", len(all))
	}
	if last := all[len(all)-1]; last.Message != "no end" {
		t.Errorf("the last report says %q, want the last line", last.Message)
	}
	// With no report, nothing watches the capture after.
	if res := run(t, s, "fmt.Println(\"quiet\")"); res.Output != "quiet\n" || len(got.all()) != len(all) {
		t.Errorf("a run with no report gave %+v and %d reports more", res, len(got.all())-len(all))
	}
}

// watchedHost is a sketch host that tells the groups its sketch
// regenerates, as the application's does.
type watchedHost struct {
	*api.SketchHost
	mu sync.Mutex
	fn func(string, int, int)
}

func (h *watchedHost) WatchRegeneration(fn func(group string, before, total int)) (stop func()) {
	h.mu.Lock()
	h.fn = fn
	h.mu.Unlock()
	return func() {
		h.mu.Lock()
		h.fn = nil
		h.mu.Unlock()
	}
}

// watchHooks are the sketch's Hooks, passing on the groups begun.
type watchHooks struct {
	sketch.NoHooks
	h *watchedHost
}

func (w watchHooks) Regenerating(g *sketch.Group, before, total int) {
	w.h.mu.Lock()
	defer w.h.mu.Unlock()
	if w.h.fn != nil {
		w.h.fn(g.DescriptionString(), before, total)
	}
}

// A script that makes a solid reports the groups regenerated, with
// their places in the pass.
func TestSessionReportsRegeneration(t *testing.T) {
	t.Parallel()
	sk := sketch.NewSketch()
	sk.NewFile()
	host := &watchedHost{SketchHost: api.NewSketchHost(sk, "")}
	sk.Hooks = watchHooks{h: host}
	s, _, err := Open(script.Options{Host: host})
	if err != nil {
		t.Fatal(err)
	}
	var got reports
	res, err := s.RunWithProgress(context.Background(), "doc.Sketch.Rect(V(0, 0, 0), V(10, 20, 0))\ndoc.Features.Extrude(5, \"plate\")", got.add)
	if err != nil || res.Error != "" {
		t.Fatalf("the script gave %+v, %v", res, err)
	}
	all := got.all()
	if len(all) == 0 {
		t.Fatal("no report of the regeneration")
	}
	last := all[len(all)-1]
	if !strings.HasPrefix(last.Message, "regenerating g") || !strings.Contains(last.Message, " of ") ||
		last.Total < 3 || last.Done < 1 || last.Done > last.Total {
		t.Errorf("the last report is %+v; want a group regenerated, counted", last)
	}
}
