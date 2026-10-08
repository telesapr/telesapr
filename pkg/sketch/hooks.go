package sketch

import "github.com/telesapr/telesapr/pkg/core"

// Hooks is what a sketch tells whoever owns it, and asks of them: a
// user interface implements all of them, NoHooks none, and Messages a
// function that takes the messages.
type Hooks interface {
	// Message shows the user a message.
	Message(msg string)
	// ScheduleGenerateAll is told that a regeneration was scheduled,
	// so that an event loop can call Refresh.
	ScheduleGenerateAll()
	// Generated is called near the end of each GenerateAll pass,
	// before the message about what was deleted along with something,
	// with how many requests, constraints and groups were.
	Generated(requests, constraints, groups int)
	// RegenerateAfterDelete regenerates after requests were deleted,
	// when the owner wants to clear what could refer to them first; it
	// reports false to leave the regeneration to the sketch.
	RegenerateAfterDelete() bool
	// DraggedParams lists the params the user is dragging, which the
	// solver keeps as close to their values as it can.
	DraggedParams() []core.HParam
	// Cancel reports whether a regeneration should stop between groups.
	Cancel() bool
}

// NoHooks is the Hooks of a sketch that nobody watches.
type NoHooks struct{}

func (NoHooks) Message(string)               {}
func (NoHooks) ScheduleGenerateAll()         {}
func (NoHooks) Generated(_, _, _ int)        {}
func (NoHooks) RegenerateAfterDelete() bool  { return false }
func (NoHooks) DraggedParams() []core.HParam { return nil }
func (NoHooks) Cancel() bool                 { return false }

// Messages is the Hooks of a sketch whose messages go to a function
// and that asks nothing else.
type Messages func(msg string)

func (m Messages) Message(msg string)         { m(msg) }
func (Messages) ScheduleGenerateAll()         {}
func (Messages) Generated(_, _, _ int)        {}
func (Messages) RegenerateAfterDelete() bool  { return false }
func (Messages) DraggedParams() []core.HParam { return nil }
func (Messages) Cancel() bool                 { return false }

// hooks is the sketch's Hooks, NoHooks when none are set.
func (sk *Sketch) hooks() Hooks {
	if sk.Hooks == nil {
		return NoHooks{}
	}
	return sk.Hooks
}

// Tell shows the user a message, through the sketch's Hooks.
func (sk *Sketch) Tell(msg string) { sk.hooks().Message(msg) }
