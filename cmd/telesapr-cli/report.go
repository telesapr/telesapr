package main

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/telesapr/telesapr/pkg/sketch"
)

// runReport is what --profile writes: the run's time and memory, and
// for each model the command worked on, its phases and its groups.
type runReport struct {
	Command []string      `json:"command"`
	Status  int           `json:"status"`
	TotalMs float64       `json:"total_ms"`
	Files   []*fileReport `json:"files,omitempty"`
	Memory  memoryReport  `json:"memory"`

	start time.Time
}

// fileReport is one model's figures.
type fileReport struct {
	Input        string        `json:"input"`
	LoadMs       float64       `json:"load_ms"`
	RegenerateMs float64       `json:"regenerate_ms"`
	CommandMs    float64       `json:"command_ms"`
	Groups       []groupReport `json:"groups,omitempty"`

	timings *sketch.Timings
	mark    time.Time
}

// groupReport is one group's figures, summed over the regenerations
// of the run.
type groupReport struct {
	Group   string  `json:"group"`
	SolveMs float64 `json:"solve_ms"`
	LoopsMs float64 `json:"loops_ms"`
	SolidMs float64 `json:"solid_ms"`
	Solves  int     `json:"solves"`
	Solids  int     `json:"solids"`
}

// memoryReport is what the run allocated, from the runtime's counts.
type memoryReport struct {
	TotalAllocBytes uint64  `json:"total_alloc_bytes"`
	Mallocs         uint64  `json:"mallocs"`
	HeapSysBytes    uint64  `json:"heap_sys_bytes"`
	NumGC           uint32  `json:"num_gc"`
	GCPauseMs       float64 `json:"gc_pause_ms"`
}

func newRunReport(args []string) *runReport {
	return &runReport{Command: args, start: time.Now()}
}

// ms is d in milliseconds.
func ms(d time.Duration) float64 { return float64(d) / float64(time.Millisecond) }

// file starts the figures of a model; on a nil report it is nil, and
// so are its methods.
func (r *runReport) file(input string) *fileReport {
	if r == nil {
		return nil
	}
	f := &fileReport{Input: input, mark: time.Now()}
	r.Files = append(r.Files, f)
	return f
}

// lap is the time since the last mark, which it moves on.
func (f *fileReport) lap() float64 {
	now := time.Now()
	d := now.Sub(f.mark)
	f.mark = now
	return ms(d)
}

// loaded ends the loading, and has the sketch time its groups from
// here on.
func (f *fileReport) loaded(sk *sketch.Sketch) {
	if f == nil {
		return
	}
	f.LoadMs = f.lap()
	f.timings = &sketch.Timings{}
	sk.Timings = f.timings
}

// regenerated ends the regeneration after loading.
func (f *fileReport) regenerated() {
	if f == nil {
		return
	}
	f.RegenerateMs = f.lap()
}

// ran ends the command, and takes the groups' figures.
func (f *fileReport) ran(sk *sketch.Sketch) {
	if f == nil {
		return
	}
	f.CommandMs = f.lap()
	for _, g := range f.timings.Groups() {
		name := fmt.Sprintf("g%03x", g.Group.V)
		if sg := sk.Group(g.Group); sg != nil {
			name = sg.DescriptionString()
		}
		f.Groups = append(f.Groups, groupReport{Group: name, SolveMs: ms(g.Solve), LoopsMs: ms(g.Loops),
			SolidMs: ms(g.Solid), Solves: g.Solves, Solids: g.Solids})
	}
	sk.Timings = nil
}

// write writes the report, with the run's status and memory, to name.
func (r *runReport) write(name string, status int) error {
	r.Status = status
	r.TotalMs = ms(time.Since(r.start))
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	r.Memory = memoryReport{TotalAllocBytes: m.TotalAlloc, Mallocs: m.Mallocs, HeapSysBytes: m.HeapSys,
		NumGC: m.NumGC, GCPauseMs: ms(time.Duration(m.PauseTotalNs))}
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding the profile: %w", err)
	}
	if err := os.WriteFile(name, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("writing the profile: %w", err)
	}
	return nil
}
