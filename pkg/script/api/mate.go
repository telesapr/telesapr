package api

// Mates (roadmap 16.9, G1) are shortcuts that put two parts together
// with the constraints there are: what each makes is ordinary
// constraints, listed, edited and deleted as any. They are made in the
// active group, which places one of the parts, as a link group does.

// addUnlessRedundant adds the constraint make makes to out, unless the
// active group no longer solves with it, as when what it asks holds
// already: then it is deleted again. A group that didn't solve before
// tells nothing of the constraint, and keeps it: parts not yet flagged
// to move, stacked as their links put them, are held where they are,
// and a mate between two of them made in a later part's group leaves
// that group unsolved until they are flagged, when what the mate asks
// is what assembles them.
func (d *Doc) addUnlessRedundant(out []Constraint, build func() Constraint) []Constraint {
	d.Edit.Regen()
	before := d.Active().Solved()
	c := build()
	d.Edit.Regen()
	if !before || d.Active().Solved() {
		return append(out, c)
	}
	c.Delete()
	d.Edit.Regen()
	return out
}
