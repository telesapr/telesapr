package srf

// mendCrossings meshes again, by the constrained Delaunay mesher, each
// surface whose ear-clipped mesh strays from it (strays[i], its mesh
// ms[i]) and is unsound in the shell's: an edge of its triangles crosses
// the shell's mesh or has no mate there. The ear clipper fans long
// chords across a patch ruled one way or a trimmed corner, which pass a
// trim point on the wrong side and cross the face beyond (roadmap K4);
// the CDT is sound there, at more triangles, so it takes only those
// surfaces, and a sound mesh is left as it is.
func (s *SShell) mendCrossings(ms []SMesh, strays []bool) {
	var all SMesh
	some := false
	for i := range ms {
		all.Triangles = append(all.Triangles, ms[i].Triangles...)
		some = some || strays[i]
	}
	if !some {
		return
	}
	kd := SKdNodeFrom(&all)
	cnt := 0
	unsound := func(m *SMesh) bool {
		for i := range m.Triangles {
			tr := &m.Triangles[i]
			for j := range 3 {
				a, b := tr.vertex(j), tr.vertex((j+1)%3)
				cnt++
				var info EdgeOnInfo
				kd.FindEdgeOn(a, b, cnt, true, &info)
				if info.IntersectsMesh {
					return true
				}
				if info.Count != 1 {
					// Coincident edges run both ways in equal numbers.
					cnt++
					var par EdgeOnInfo
					kd.FindEdgeOn(b, a, cnt, true, &par)
					if info.Count != par.Count {
						return true
					}
				}
			}
		}
		return false
	}
	for i := range ms {
		if !strays[i] || !unsound(&ms[i]) {
			continue
		}
		var m SMesh
		s.Surface[i].triangulate(s, &m, true)
		ms[i] = m
	}
}
