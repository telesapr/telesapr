package agent

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// This file is how far a script is while it runs: a script that
// regenerates a large model runs for minutes, and a program waiting
// for its result would see nothing until it ends. Session reports the
// group being regenerated and each line the script prints, at most a
// few times a second; Serve passes the reports over the connection and
// Remote to its caller; package mcpserver tells the MCP client.

// Progress is what a script that runs has got to.
type Progress struct {
	// Message says what is being done: the group being regenerated, as
	// "regenerating g007-plate, 3 of 12", or the last line the script
	// printed.
	Message string `json:"message,omitempty"`
	// Done is how many steps are begun of Total, the groups of a pass
	// over them: the third of twelve is 3 of 12. A Total of 0 is a
	// report with no count, as of a line printed. A regeneration makes
	// a pass or two, and a script may regenerate many times, so Done
	// starts again at each.
	Done  int `json:"done,omitempty"`
	Total int `json:"total,omitempty"`
	// Output is what the script printed since the report before, in
	// whole lines, as Result.Output has it.
	Output string `json:"output,omitempty"`
}

// ProgressRunner is a Backend that reports how far a script is while
// it runs. report is called, if not nil, from any goroutine but never
// from two at once, and not after RunWithProgress returns; Run is
// RunWithProgress with no report.
type ProgressRunner interface {
	RunWithProgress(ctx context.Context, code string, report func(Progress)) (Result, error)
}

// RegenerationWatcher is a script host that tells which group it
// regenerates: from WatchRegeneration until the stop it returns, fn is
// called, on any goroutine and without blocking it, as each group
// begins, with the group's name, its place in the pass and the
// groups there are (sketch.RegenerationWatcher).
type RegenerationWatcher interface {
	WatchRegeneration(fn func(group string, before, total int)) (stop func())
}

// reportEvery is the least time between two reports: a script that
// prints in a loop would otherwise send a line a line.
const reportEvery = 250 * time.Millisecond

// pacer passes reports on at most one every so often, on a goroutine
// of its own: a report that comes sooner is merged into the next, so
// the last state always goes out, and add never waits.
type pacer struct {
	report func(Progress)
	every  time.Duration

	mu      sync.Mutex
	pending Progress
	has     bool

	kick chan struct{}
	quit chan struct{}
	done chan struct{}
}

// newPacer starts passing reports to report.
func newPacer(report func(Progress), every time.Duration) *pacer {
	p := &pacer{report: report, every: every,
		kick: make(chan struct{}, 1), quit: make(chan struct{}), done: make(chan struct{})}
	go p.loop()
	return p
}

// add takes a report: its message and count replace those not yet
// sent, and its output follows theirs.
func (p *pacer) add(r Progress) {
	p.mu.Lock()
	r.Output = p.pending.Output + r.Output
	p.pending, p.has = r, true
	p.mu.Unlock()
	select {
	case p.kick <- struct{}{}:
	default:
	}
}

// flush sends what is pending, if anything.
func (p *pacer) flush() {
	p.mu.Lock()
	r, has := p.pending, p.has
	p.pending, p.has = Progress{}, false
	p.mu.Unlock()
	if has {
		p.report(r)
	}
}

func (p *pacer) loop() {
	defer close(p.done)
	wait := time.NewTimer(p.every)
	wait.Stop()
	for {
		select {
		case <-p.kick:
		case <-p.quit:
			p.flush()
			return
		}
		p.flush()
		wait.Reset(p.every)
		select {
		case <-wait.C:
		case <-p.quit:
			wait.Stop()
			p.flush()
			return
		}
	}
}

// close sends what is pending and stops; no report is sent after it
// returns.
func (p *pacer) close() {
	close(p.quit)
	<-p.done
}

// regenerationMessage is a report's message for a group begun.
func regenerationMessage(group string, before, total int) string {
	return fmt.Sprintf("regenerating %s, %d of %d", group, before+1, total)
}

// watchLines has fn called with each whole line written to the capture
// while it keeps what is written, until the stop it returns, which
// passes on the part of a line still unended. fn is called with the
// capture locked, so it must not write to it, and should not wait.
func (c *Capture) watchLines(fn func(line string)) (stop func()) {
	c.mu.Lock()
	c.line, c.partial = fn, ""
	c.mu.Unlock()
	return func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		if c.partial != "" {
			fn(c.partial)
		}
		c.line, c.partial = nil, ""
	}
}

// passLines gives the capture's line watcher the whole lines of what
// was written; it is called with the capture locked.
func (c *Capture) passLines(p []byte) {
	if c.line == nil {
		return
	}
	text := c.partial + string(p)
	for {
		line, rest, ok := strings.Cut(text, "\n")
		if !ok {
			break
		}
		c.line(line)
		text = rest
	}
	c.partial = text
}
