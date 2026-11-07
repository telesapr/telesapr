package ui

import (
	"github.com/telesapr/telesapr/pkg/script/agent"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// This file tells a program that waits on a script which group the
// regeneration has got to: a large model takes minutes, and the
// program would see nothing till the end (agent.Progress).

var (
	_ sketch.RegenerationWatcher = appHooks{}
	_ agent.RegenerationWatcher  = (*ScriptHost)(nil)
)

// Regenerating passes on the group a regeneration begins to whoever
// watches, if anyone does.
func (h appHooks) Regenerating(g *sketch.Group, before, total int) {
	if fn := h.ss.regenWatch.Load(); fn != nil {
		(*fn)(g.DescriptionString(), before, total)
	}
}

// WatchRegeneration has fn told, on the event loop's goroutine, each
// group the application's regenerations begin, until stop; a second
// watch takes the place of the first.
func (h *ScriptHost) WatchRegeneration(fn func(group string, before, total int)) (stop func()) {
	p := &fn
	h.ss.regenWatch.Store(p)
	return func() { h.ss.regenWatch.CompareAndSwap(p, nil) }
}
