package api

// Mates (roadmap 16.9, G1) are shortcuts that put two parts together
// with the constraints there are: what each makes is ordinary
// constraints, listed, edited and deleted as any. They are made in the
// active group, which places one of the parts, as a link group does.

// addUnlessRedundant adds the constraint make makes to out, unless the
// active group no longer solves with it, as when what it asks holds
// already: then it is deleted again.
func (d *Doc) addUnlessRedundant(out []Constraint, build func() Constraint) []Constraint {
	c := build()
	d.Edit.Regen()
	if d.Active().Solved() {
		return append(out, c)
	}
	c.Delete()
	d.Edit.Regen()
	return out
}
