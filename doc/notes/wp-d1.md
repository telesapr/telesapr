# WP-D1: clearance and wall thickness (item 14)

State 2026-10-22: built on branch `wp-d1-clearance`. The definition of done is met; the tests and
`make ci` decide.

- `pkg/srf/meshtree.go`, `tridist.go`: a box tree over a mesh's triangles. It gives the nearest
  pair of two meshes, the nearest point, the triangles within a radius, and a ray's first hit.
  `SShell.TriangulateBySurface` gives each surface's triangles apart; `TriangulateInto` is built
  on it and gives the same bytes.
- `pkg/sketch/clearance.go`: `Sketch.Clearances(upTo, below)`. It uses the parts of
  `assemblyParts`, finds the gap between their meshes, and follows the nearest points onto every
  surface about them (`Surface.Project`). It keeps the nearest pair whose points stay within the
  chord tolerance of the meshes. `Tolerance` is computed, not assumed.
- `pkg/sketch/wall.go`: `Sketch.WallThickness(upTo, below)` casts a ray from each triangle's
  middle into the part. When it slips through an edge, the ray is cast again from three points
  off the middle.
- Window: Analyze > Clearance and Walls..., `screenClearance` (`pkg/ui/clearance.go`). Lines are
  drawn as `nakedEdges`.
- Scripts: `Assembly.Clearances(below)`, `GroupAnalysis.WallThickness(below)`.

Open: thin spots between triangle middles are missed; gaps to parts made only of a mesh are not
followed onto surfaces; the solids within one linked part are not checked against each other;
the MCP `measure` tool does not report either.
