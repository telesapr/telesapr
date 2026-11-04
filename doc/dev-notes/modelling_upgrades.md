# Modelling upgrades, October 2026

Refreshed 2026-11-03 at `c4e18d1a`; items 7 and 15 brought up to `a68c4c8b`.

Nineteen generic improvements, built from 2026-10-22 as work packages branched from `devel`
(A1 to D5), then their leftovers (branches `lo-*`, `lo2-*` and, on 2026-11-02, `mu-*`). All are
built but item 10. Code comments and `doc/dev-notes/done-audit.md` cite them by number; this table
is the key. What is still open is in ROADMAP, "Modelling upgrade leftovers". How each is used is in
`doc/guide`.

The plan, with what existed before each item, is at
`git show 39142006:doc/notes/modelling_upgrades.md`. The packages' working notes are at
`git show 1e84ca0c:doc/notes/` (`wrap-emboss.md`, `model-to-script.md`, `wp-b1-b2.md`,
`wp-b3-b4.md`, `wp-c3.md`, `wp-c4.md`, `wp-d1.md`, `wp-scripting.md`).

| # | Improvement | Package | Code, and what was decided |
|---|---|---|---|
| 1 | Wrap or emboss a sketch onto cylinders and cones | C2 | `pkg/sketch/wrap.go` |
| 2 | Named datums | B4 | `pkg/sketch/datum.go` |
| 3 | Parametric links and relink | B3 | `pkg/sketch/link_params.go`; the link cache is keyed by the values passed |
| 4 | Thread as one parametric group | B1 | `pkg/sketch/thread*.go`; open rows in ROADMAP, "Thread leftovers" |
| 5 | Script forms | B5 | `api.Form` |
| 6 | Script library imports | A4 | One library folder, read by the console, `run` and MCP |
| 7 | Shells of curved solids | C1 | `pkg/topo/shellrevolve*.go`, `shelldraft.go`, `shellprism.go`, `shelljoined*.go`: lathes and revolves of lines and arcs, drafted extrusions, prisms, and solids joined or cut from such parts. The general shell (E12) is parked |
| 8 | Sweeps along closed paths, lofts tangent to a face | C4 | `pkg/topo/closedloft_turn.go` and the loft files. A solid's sides at a ring's ends stay refused for good: the ring would go on into the solid |
| 9 | Fillets running onto curved faces | C3 | Open rows in ROADMAP, "Fillet leftovers" |
| 10 | Exact features after a mesh body | held | Not built. Only as an explicit approximate mode, which ROADMAP parks (the mesh Boolean fallback) |
| 11 | Screw mate | A5 | A turn about an axis tied to a distance by the lead |
| 12 | Expressions on more group values | A1 | Extrude depth, revolve and helix angle |
| 13 | Density per group | A3 | `pkg/sketch/mass.go` |
| 14 | Minimum wall and clearance analysis | D1 | `pkg/sketch/clearance.go`, `wall.go`, `wall_near.go`, `mesh_parts.go`; done 2026-11-04 |
| 15 | Model to script export | D5 | `Doc.Script`; damaged files open in part through `pkg/sketch/mend.go`, what they leave out kept for a save and mended in the model (`leftout.go`, `leftout_mend.go`, 2026-11-04); open in ROADMAP |
| 16 | More MCP tools and a filesystem allowlist | A2, D3 | `pkg/mcpserver`; `doc/guide/ai.md` |
| 17 | Incremental regeneration | D4 | `pkg/sketch/solid_cache.go`. Parallel commits (recommendation 11 of `performance.md`) were measured on 2026-11-02 and declined: under 1% of the time |
| 18 | glTF export | A2 | Binary `.glb`, lengths in metres |
| 19 | Cosmetic thread convention on 2D drawings | B2 | Drawn on views, sections, askew views and round holes |
