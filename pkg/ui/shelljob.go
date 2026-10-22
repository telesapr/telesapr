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
//
// A drag of the geometry makes its solids on the thread at every frame
// (mouse.go, draggingGeometry), unless DragSolidsLate is set: then a
// job runs for where the sketch is, the frames after it don't stop it,
// and when it ends its solids go in though the sketch has moved on
// (sketch.ApplyLate), and the next job starts for where it is then.

// shellJobs is the interface's state of them.
type shellJobs struct {
	job   *sketch.ShellJob
	done  chan *sketch.ShellJob
	timer platform.Timer
	// late is set while the job running is one of a drag's, whose
	// solids go in when it ends whatever the sketch is then.
	late bool
}

// pollMillis is how often the interface looks for a job's end.
const pollMillis = 20

// startShellJobFor starts a job for the sketch as it is: during a drag
// one of a drag's, which leaves the job of a drag running to its end,
// else one that stops the job running.
func (ss *Application) startShellJobFor(dragging bool) {
	sj := &ss.shellJobs
	if !dragging {
		ss.startShellJob()
		return
	}
	if sj.job != nil && sj.late {
		// When it ends, its solids go in and another starts.
		return
	}
	ss.startShellJob()
	sj.late = true
}

// startShellJob starts a job for the sketch as it is, stopping the one
// running.
func (ss *Application) startShellJob() {
	sj := &ss.shellJobs
	if sj.job != nil {
		sj.job.Cancel()
	}
	job := ss.SK.NewShellJob()
	sj.job, sj.late = job, false
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
func (ss *Application) pollShellJob() {
	sj := &ss.shellJobs
	for {
		select {
		case job := <-sj.done:
			if job != sj.job {
				continue // a job stopped for a later one
			}
			late := sj.late
			sj.job, sj.late = nil, false
			if ss.SK.Apply(job) {
				ss.afterGenerate()
				ss.ScheduleShowTW()
			} else if !job.Cancelled && ss.SK.AnyShellsStale() {
				if late && ss.SK.ApplyLate(job) {
					ss.afterGenerate()
				}
				ss.startShellJobFor(late && ss.GW.draggingGeometry())
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
func (ss *Application) stopShellJob() bool {
	sj := &ss.shellJobs
	if sj.job == nil {
		return false
	}
	sj.job.Cancel()
	sj.job = nil
	ss.UpdateWindowTitles()
	return true
}
