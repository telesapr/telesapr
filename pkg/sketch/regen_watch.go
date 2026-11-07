package sketch

// RegenerationWatcher is Hooks whose owner is told how far a
// regeneration is, as a program that waits for a long one is told
// (package agent): Regenerating is called on the goroutine
// regenerating as each group of a pass over the groups begins, with
// the group, how many groups of the pass come before it and how many
// there are. A regeneration makes one pass over the groups or two, and
// a pass that starts after the clean groups begins its count there.
type RegenerationWatcher interface {
	Regenerating(g *Group, before, total int)
}

// regenerating tells the sketch's Hooks, if they watch, that the
// group at index i of the order begins.
func (sk *Sketch) regenerating(i int) {
	w, ok := sk.hooks().(RegenerationWatcher)
	if !ok {
		return
	}
	if g := sk.Group(sk.GroupOrder[i]); g != nil {
		w.Regenerating(g, i, len(sk.GroupOrder))
	}
}
