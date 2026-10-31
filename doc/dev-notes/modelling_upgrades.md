# Modelling upgrades, October 2026

Nineteen generic improvements, planned and built on 2026-10-22 as work packages branched from
`devel` (A1 to D5), then their leftovers (branches `lo-*` and `lo2-*`). Code comments cite the
items by number. What is still open is in ROADMAP, "Modelling upgrade leftovers"; how each is used
is in `doc/guide`, and its tests, scenario, example and goldens are in `doc/done-audit.md`.

The plan as written, with what existed before each item and a note per package, is at
`git show 39142006:doc/notes/modelling_upgrades.md`; the packages' working notes are at
`git show 1e84ca0c:doc/notes/` (`wrap-emboss.md`, `model-to-script.md`, `wp-b1-b2.md`,
`wp-b3-b4.md`, `wp-c3.md`, `wp-c4.md`, `wp-d1.md`, `wp-scripting.md`).

| # | Improvement | Package | Built |
|---|---|---|---|
| 1 | Wrap or emboss a sketch onto cylinders and cones | C2 | The wrap group, raised or sunk, on a cylinder (an isometry) or a cone (its development), up to 350° round; deep cone wraps and thin walls under an emboss (`pkg/sketch/wrap.go`) |
| 2 | Named datums | B4 | Names for points, workplanes, faces and axes, found through links as `group.name`; since the leftovers any face or edge of the solid, kinds checked by mates, mates of two names (`pkg/sketch/datum.go`) |
| 3 | Parametric links and relink | B3 | Relink (`LinkGroup.SetFile`), a configuration and parameter values passed into a linked part, the link cache keyed by them (`pkg/sketch/link_params.go`) |
| 4 | Thread as one parametric group | B1 | The thread kept on its helix group and edited as one: pitch, length (an expression), clearance, truncated or rounded crests; run-out and level starts in the leftovers |
| 5 | Script forms | B5 | `var Form = api.Form{...}`, asked for in a dialog by the window and taken as `--set name=value` by the command line |
| 6 | Script library imports | A4 | A library folder read by the console, `run` and MCP |
| 7 | Shells of curved solids | C1 | One lathe or revolve of lines and arcs, by its offset profile; the general shell (E12) stays parked |
| 8 | Sweeps along closed paths, lofts tangent to a face | C4 | Sweeps round closed paths; lofts along a solid's sides; in the leftovers closed lofts, guided lofts along sides, tangency at a middle profile |
| 9 | Fillets running onto curved faces | C3 | Rolling-ball picks with others, loft foot and top ring fillets; in the leftovers picks mitred at a corner, fillets ending where a crease fades, chamfers ending on curved faces. `TestFilletOnOtherSolids` 47 → 59 of 59 |
| 10 | Exact features after a mesh body | held | Not built: only as an explicit approximate mode (ROADMAP, Parked) |
| 11 | Screw mate | A5 | A constraint on a turn about an axis tied to a distance by the lead |
| 12 | Expressions on more group values | A1 | Extrude depth, revolve and helix angle held as group values |
| 13 | Density per group | A3 | Density, mass properties per part, a mass column in the bill of materials (`pkg/sketch/mass.go`) |
| 14 | Minimum wall and clearance analysis | D1 | Gaps between parts and walls thinner than a limit, followed onto the surfaces (`pkg/sketch/clearance.go`, `wall.go`) |
| 15 | Model to script export | D5 | `Doc.Script`, `File.ExportScript`, File > Export > Script, `telesapr-cli export-script`, the MCP `export` tool's `.go`; 300 of 301 fixtures round-trip |
| 16 | More MCP tools and a filesystem allowlist | A2, D3 | `measure`, `export`, `open`, `list_files`, a section on `render`; AI scripts' files kept to allowed folders (`doc/ai.md`) |
| 17 | Incremental regeneration | D4 | One regeneration pass, solids kept by what they were made from (`pkg/sketch/solid_cache.go`); the performance recommendations taken up after it are in `performance.md` |
| 18 | glTF export | A2 | Binary `.glb`, a material per colour, lengths in metres |
| 19 | Cosmetic thread convention on 2D drawings | B2 | Threads drawn by the drafting convention on drawings; in the leftovers in sections, askew views and round holes |
