package srf

import "slices"

// ExactCopy is a copy of the shell that shares none of what a later
// operation on either could change: the curves' points, the surfaces'
// trims and edges, and each surface's last closest point, which starts
// the next search on it. Unlike MakeFromCopyOf it transforms nothing,
// and unlike Clone it keeps what was built from the trims and the
// tolerance, so the copy goes on as the shell would. What the surfaces
// build once and only read after (the trim BSPs, the grids of guesses,
// the ray-cast trees, the kd-trees of edges) is shared.
func (s *SShell) ExactCopy() SShell {
	c := *s
	c.Curve = slices.Clone(s.Curve)
	for i := range c.Curve {
		c.Curve[i].Points = slices.Clone(c.Curve[i].Points)
	}
	c.Surface = slices.Clone(s.Surface)
	for i := range c.Surface {
		sf := &c.Surface[i]
		sf.Trim = slices.Clone(sf.Trim)
		sf.Edges.Edges = slices.Clone(sf.Edges.Edges)
		sf.curvedEdges.Edges = slices.Clone(sf.curvedEdges.Edges)
	}
	return c
}

// ExactCopy is a copy of the mesh that shares no triangles with it.
func (m *SMesh) ExactCopy() SMesh {
	c := *m
	c.Triangles = slices.Clone(m.Triangles)
	return c
}
