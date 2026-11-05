package sketch

import "fmt"

// noteFused says on the group how far its Boolean moved the group's round
// faces onto the solid's (srf.SShell.Fused): a pin a few hundredths over
// a bore's size, at a chord tolerance the mesh cannot show that in, was
// made to fill the bore, where the Boolean of the two as drawn failed or
// left its mesh open (roadmap K3).
func (g *Group) noteFused(fused float64) {
	if fused <= 0 {
		return
	}
	note := fmt.Sprintf("round faces %s mm off the solid's, under the chord tolerance, were made to meet them", sizeText(fused))
	if g.Note != "" {
		note = g.Note + "; " + note
	}
	g.Note = note
}
