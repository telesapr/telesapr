package sketch

import (
	"reflect"
	"sync/atomic"
)

// Solids made off the interface's thread (roadmap 17.3, F7). A
// regeneration with DeferShells solves and makes the entities as ever,
// but leaves each group's solid as it was, marked ShellsStale: the
// interface goes on drawing the last good solid. A ShellJob then
// regenerates a copy of what defines the sketch, in full, on any
// goroutine; if the sketch is still defined the same way when it ends,
// Apply moves the copy's groups, with their solids, into the sketch. A
// job can be cancelled, which leaves the solids out of date.

// ShellJob is a regeneration of a copy of the sketch.
type ShellJob struct {
	state  *UndoState
	clone  *Sketch
	cancel atomic.Bool
	// Cancelled is set when Run stopped before the end.
	Cancelled bool
}

// AnyShellsStale reports whether a group's solid is out of date.
func (sk *Sketch) AnyShellsStale() bool {
	for i := range sk.Groups {
		if sk.Groups[i].ShellsStale {
			return true
		}
	}
	return false
}

// NewShellJob copies what defines the sketch, for a job to regenerate.
// It is called on the goroutine that owns the sketch.
func (sk *Sketch) NewShellJob() *ShellJob {
	st := sk.definingState()
	c := &Sketch{
		Units: st.Units, Active: st.Active,
		ChordTolPct: sk.ChordTolPct, CheckClosedContour: sk.CheckClosedContour,
		MaxSegments: sk.MaxSegments, Mesher: sk.Mesher,
		Images: sk.Images, Fonts: sk.Fonts, Settings: sk.Settings,
	}
	clone := sk.cloneState(st)
	c.Groups, c.GroupOrder, c.Requests, c.Constraints = clone.Groups, clone.GroupOrder, clone.Requests, clone.Constraints
	c.Params, c.Styles, c.Parameters = clone.Params, clone.Styles, clone.Parameters
	c.Configurations, c.ActiveConfiguration = clone.Configs, clone.ActiveConf
	c.Datums = clone.Datums
	// What the linked files gave is read, never written, by a
	// regeneration: the copy shares it.
	for i := range c.Groups {
		if g := sk.Group(c.Groups[i].H); g != nil {
			c.Groups[i].ImpEntity, c.Groups[i].ImpMesh, c.Groups[i].ImpShell = g.ImpEntity, g.ImpMesh, g.ImpShell
			c.Groups[i].LinkInfo, c.Groups[i].LinkProblem = g.LinkInfo, g.LinkProblem
			c.Groups[i].LinkLoaded, c.Groups[i].LinkModelRead = g.LinkLoaded, g.LinkModelRead
			c.Groups[i].LinkPartParameters, c.Groups[i].LinkConfigurations = g.LinkPartParameters, g.LinkConfigurations
			c.Groups[i].LinkDatums = g.LinkDatums
		}
	}
	return &ShellJob{state: st, clone: c}
}

// cloneState is a second copy of st, so that the job's sketch owns its
// own and the job keeps st to compare with.
func (sk *Sketch) cloneState(st *UndoState) *UndoState {
	keep := *sk
	defer func() { *sk = keep }()
	sk.Groups, sk.GroupOrder, sk.Requests, sk.Constraints = st.Groups, st.GroupOrder, st.Requests, st.Constraints
	sk.Params, sk.Styles, sk.Parameters = st.Params, st.Styles, st.Parameters
	sk.Configurations, sk.ActiveConfiguration = st.Configs, st.ActiveConf
	sk.Datums = st.Datums
	return sk.definingState()
}

// Cancel asks Run to stop at the next group.
func (j *ShellJob) Cancel() { j.cancel.Store(true) }

// cancelledBy is a job's copy's Hooks: the sketch's own, with the job
// answering Cancel.
type cancelledBy struct {
	Hooks
	cancel func() bool
}

func (c cancelledBy) Cancel() bool { return c.cancel() }

// Run regenerates the copy in full; it may run on any goroutine.
func (j *ShellJob) Run() {
	c := j.clone
	c.Hooks = cancelledBy{Hooks: c.hooks(), cancel: j.cancel.Load}
	c.GenerateAllTyped(GenerateAll)
	j.Cancelled = j.cancel.Load()
}

// Apply moves the job's groups, with their solids, into the sketch if
// it is still defined as when the job was made and the job ran to the
// end, and reports whether it did. It is called on the goroutine that
// owns the sketch.
func (sk *Sketch) Apply(j *ShellJob) bool {
	if j.Cancelled || !sameDefinition(sk.definingState(), j.state) {
		return false
	}
	for i := range sk.Groups {
		if j.clone.Group(sk.Groups[i].H) == nil {
			return false
		}
	}
	// A sketch on a face was placed from the solids as they were; where
	// the new ones moved its face, the entities and the solve placed
	// from them are the copy's.
	moved := false
	for i := range sk.Groups {
		g := &sk.Groups[i]
		c := j.clone.Group(g.H)
		moved = moved || c.facePlaneMoved(g.Predef.Q, g.Predef.At)
		*g = *c
		g.ShellsStale = false
		g.DisplayDirty = true
	}
	if moved {
		sk.Entities, sk.Params = j.clone.Entities, j.clone.Params
	}
	return true
}

// sameDefinition reports whether two states define the same sketch:
// the same everything, the solver's values to the bit, but whether a
// value was taken as known, which a regeneration of other groups sets.
func sameDefinition(a, b *UndoState) bool {
	if len(a.Params) != len(b.Params) {
		return false
	}
	for i := range a.Params {
		p, q := a.Params[i], b.Params[i]
		p.Known, q.Known = false, false
		if p != q {
			return false
		}
	}
	x, y := *a, *b
	x.Params, y.Params = nil, nil
	return reflect.DeepEqual(x, y)
}
