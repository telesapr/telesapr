package topo

import (
	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/srf"
)

// surface is the shell's surface of handle h, as srf.SShell.FindSurface
// finds it. FindSurface looks first where the handle's number puts it
// and then through every surface; a shell edited before (a fillet's
// patches put in after the faces it cut) has its surfaces out of that
// order, and a rolling ball's feet, looking up the faces either side for
// each of thousands of projections, spent a tenth of a chamfer's time
// there. So where each handle lies is filed, and filed again where the
// shell has changed: a filed place is taken only where the shell has
// that handle there.
func (t *Topology) surface(h core.HSSurface) *srf.SSurface {
	sh := t.shell
	if sh == nil {
		return nil
	}
	if i := int(h.V) - 1; i >= 0 && i < len(sh.Surface) && sh.Surface[i].H == h {
		return &sh.Surface[i]
	}
	if at := t.surfAt.Load(); at != nil {
		if i, ok := (*at)[h]; ok && i < len(sh.Surface) && sh.Surface[i].H == h {
			return &sh.Surface[i]
		}
	}
	// The first surface of each handle, as FindSurface's scan finds it.
	at := make(map[core.HSSurface]int, len(sh.Surface))
	for i := range sh.Surface {
		if _, ok := at[sh.Surface[i].H]; !ok {
			at[sh.Surface[i].H] = i
		}
	}
	t.surfAt.Store(&at)
	if i, ok := at[h]; ok {
		return &sh.Surface[i]
	}
	return nil
}
