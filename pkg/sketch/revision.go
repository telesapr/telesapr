package sketch

// Revision tells one state of a sketch's content from another, for a
// program that reads the sketch now and then and wants to know whether
// anyone changed it in between (an AI model through the MCP server).
//
// It moves where what defines the sketch changes: where a state is
// remembered for undo (every edit remembers one first), undone or
// redone, and where the sketch is cleared or replaced by another file
// or a new one. A regeneration, a selection, a view, activating a group
// or a read of the model doesn't move it. Generation counts
// regenerations, which every activation and every read of a group
// past the active one makes; Unsaved is set by an activation too
// (MarkGroupDirty): neither says that the content changed.
type Revision struct {
	// Drawing changes when the sketch is cleared or replaced by another
	// (Follow): another file, or a new one.
	Drawing uint64
	// Change counts the changes, and moves at every change of Drawing
	// too.
	Change uint64
}

// Revision is the sketch's revision.
func (sk *Sketch) Revision() Revision { return sk.revision }

// NoteChange moves the revision for a change that remembers no undo
// state of its own: a drag after the state remembered as it began, a
// script's change after the first of its step.
func (sk *Sketch) NoteChange() { sk.revision.Change++ }

// Follow makes sk the successor of old, which it replaces as the
// drawing open: its revision is old's, as of another drawing. Without
// it a sketch read from a file would start again from the revision
// zero, which the one it replaces may have had too.
func (sk *Sketch) Follow(old *Sketch) {
	sk.revision = Revision{Drawing: old.revision.Drawing + 1, Change: old.revision.Change + 1}
}

// anotherDrawing moves the revision for a sketch cleared for another
// drawing.
func (sk *Sketch) anotherDrawing() {
	sk.revision.Drawing++
	sk.revision.Change++
}
