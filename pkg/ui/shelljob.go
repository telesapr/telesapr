package ui

import (
	"github.com/telesapr/telesapr/pkg/platform"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// Solids made off the interface's thread (roadmap 17.3, F7): with
// BackgroundShells, a regeneration the interface schedules (an edit, a
// drag) solves and draws the entities at once and leaves the solids as
// they were; a sketch.ShellJob makes them on another goroutine, and a
// timer puts them in when it ends, unless the sketch changed since,
// when another job starts. Esc stops a job, leaving the solids out of
// date until the next change. Scripts, saves, exports and the tests
// regenerate in full, on the thread, as before.

// shellJobs is the interface's state of them.
type shellJobs struct {
	job   *sketch.ShellJob
	done  chan *sketch.ShellJob
	timer platform.Timer
}

// pollMillis is how often the interface looks for a job's end.
const pollMillis = 20

// startShellJob starts a job for the sketch as it is, stopping the one
// running.
func (ss *SolveSpaceUI) startShellJob() {
	sj := &ss.shellJobs
	if sj.job != nil {
		sj.job.Cancel()
	}
	job := ss.SK.NewShellJob()
	sj.job = job
	if sj.done == nil {
		sj.done = make(chan *sketch.ShellJob, 8)
	}
	done := sj.done
	go func() {
		job.Run()
		done <- job
	}()
	if sj.timer == nil {
		sj.timer = ss.Platform.CreateTimer()
		sj.timer.SetOnTimeout(ss.pollShellJob)
	}
	sj.timer.RunAfter(pollMillis)
	ss.UpdateWindowTitles()
}

// pollShellJob takes the jobs that ended: the current one's solids go
// in if they still fit the sketch, and if they don't, another starts.
func (ss *SolveSpaceUI) pollShellJob() {
	sj := &ss.shellJobs
	for {
		select {
		case job := <-sj.done:
			if job != sj.job {
				continue // a job stopped for a later one
			}
			sj.job = nil
			if ss.SK.Apply(job) {
				ss.afterGenerate()
				ss.ScheduleShowTW()
			} else if !job.Cancelled && ss.SK.AnyShellsStale() {
				ss.startShellJob()
				return
			}
			ss.UpdateWindowTitles()
		default:
			if sj.job != nil {
				sj.timer.RunAfter(pollMillis)
			}
			return
		}
	}
}

// stopShellJob stops the running job, if there is one, and reports
// whether there was.
func (ss *SolveSpaceUI) stopShellJob() bool {
	sj := &ss.shellJobs
	if sj.job == nil {
		return false
	}
	sj.job.Cancel()
	sj.job = nil
	ss.UpdateWindowTitles()
	return true
}
