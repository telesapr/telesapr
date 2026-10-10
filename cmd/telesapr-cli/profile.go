package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"runtime/pprof"
	"runtime/trace"
	"slices"
)

const profileUsage = `
Profiling options, taken by every command:
    --cpuprofile <file>
        Writes a CPU profile of the run to <file>, for go tool pprof.
    --memprofile <file>
        Writes a profile of the run's allocations to <file>, for go tool pprof.
    --trace <file>
        Writes an execution trace to <file>, for go tool trace. Its regions
        say how long loading, solving, building solids and saving took.
    --profile <file>
        Writes a summary of the run to <file> as JSON: how long it took,
        for each model how long loading, regenerating and the command
        took and how long each group spent solving, making its loops and
        building its solid, and what the run allocated.
`

// profiles are the files the profiling options name; "" is one not
// asked for.
type profiles struct {
	cpu, mem, trace, report string
}

// takeProfileFlags removes the profiling options from args, wherever
// they are, and returns what they asked for. They are taken out before
// a command parses its own options, so every command accepts them.
func takeProfileFlags(args []string) ([]string, profiles, error) {
	var p profiles
	out := slices.Clone(args[:min(len(args), 1)])
	for n := 1; n < len(args); n++ {
		var dst *string
		switch args[n] {
		case "--cpuprofile":
			dst = &p.cpu
		case "--memprofile":
			dst = &p.mem
		case "--trace":
			dst = &p.trace
		case "--profile":
			dst = &p.report
		default:
			out = append(out, args[n])
			continue
		}
		if n+1 >= len(args) {
			return nil, p, fmt.Errorf("option '%s' needs a file", args[n])
		}
		n++
		*dst = args[n]
	}
	return out, p, nil
}

// start begins the CPU profile and the trace that p asks for. The
// function it returns stops them and writes the allocation profile;
// it is to be called once the command is done.
func (p profiles) start() (stop func() error, err error) {
	var closers []func() error
	stop = func() error {
		var errs []error
		for _, c := range slices.Backward(closers) {
			errs = append(errs, c())
		}
		if p.mem != "" {
			errs = append(errs, writeHeapProfile(p.mem))
		}
		return errors.Join(errs...)
	}
	if p.mem != "" {
		// A run is short: the default rate, a sample per 512 KiB, would
		// miss most of the small allocations it makes.
		runtime.MemProfileRate = 4096
	}
	if p.cpu != "" {
		f, err := os.Create(p.cpu)
		if err != nil {
			return nil, fmt.Errorf("creating cpu profile: %w", err)
		}
		if err := pprof.StartCPUProfile(f); err != nil {
			_ = f.Close()
			return nil, fmt.Errorf("starting cpu profile: %w", err)
		}
		closers = append(closers, func() error {
			pprof.StopCPUProfile()
			return f.Close()
		})
	}
	if p.trace != "" {
		f, err := os.Create(p.trace)
		if err != nil {
			_ = stop()
			return nil, fmt.Errorf("creating trace: %w", err)
		}
		if err := trace.Start(f); err != nil {
			_ = f.Close()
			_ = stop()
			return nil, fmt.Errorf("starting trace: %w", err)
		}
		closers = append(closers, func() error {
			trace.Stop()
			return f.Close()
		})
	}
	return stop, nil
}

// writeHeapProfile writes every allocation the run made (alloc_space,
// alloc_objects) and what is still live, to name.
func writeHeapProfile(name string) error {
	f, err := os.Create(name)
	if err != nil {
		return fmt.Errorf("creating memory profile: %w", err)
	}
	// The live figures are only up to date after a collection.
	runtime.GC()
	if err := pprof.Lookup("allocs").WriteTo(f, 0); err != nil {
		_ = f.Close()
		return fmt.Errorf("writing memory profile: %w", err)
	}
	return f.Close()
}

// withProfiles runs body under the profiles args ask for, args having
// them taken out; body fills in rep, nil unless --profile asked for
// it. A profile that can't be written is said on stderr and makes the
// status a failure.
func withProfiles(args []string, stderr io.Writer, body func(args []string, rep *runReport) int) int {
	args, p, err := takeProfileFlags(args)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "%v.\n", err)
		return exitUsage
	}
	stop, err := p.start()
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "Cannot profile: %v\n", err)
		return 1
	}
	var rep *runReport
	if p.report != "" {
		rep = newRunReport(args)
	}
	status := body(args, rep)
	if rep != nil {
		if err := rep.write(p.report, status); err != nil {
			_, _ = fmt.Fprintf(stderr, "Cannot profile: %v\n", err)
			if status == exitOK {
				status = 1
			}
		}
	}
	if err := stop(); err != nil {
		_, _ = fmt.Fprintf(stderr, "Cannot profile: %v\n", err)
		if status == exitOK {
			status = 1
		}
	}
	return status
}
