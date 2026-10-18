package ui

// SelectFaceEdges replaces the selection of faces with the edges that
// bound them, where they meet other faces: the context menu's Select
// Face Edges, and then the features that take edges apply.
func (ss *Application) SelectFaceEdges() {
	ss.GW.GroupSelection()
	names, err := ss.SK.FaceEdges(ss.GW.selectedFaceIDs())
	if err != nil {
		ss.Error("Can't select the edges of the face. %s.", capitalize(err.Error()))
		return
	}
	if len(names) == 0 {
		ss.Error("The selected face has no edges where it meets another face.")
		return
	}
	ss.GW.ClearSelection()
	for _, n := range names {
		ss.GW.MakeSelected(&Selection{Edge: n})
	}
}
