# Version 2 files

Files in `.tlspr` format version 2, as the first reader wrote them on 2026-10-02. Every later
reader must open them: `TestCorpus` in `pkg/persist/tlspr` loads and regenerates each one.

Never regenerate or edit these files. Version 2 made `units` the document's unit, which
lengths written with `=` are in.

One exception: on 2026-10-03 the project was renamed from spacesolve to TeleSAPR, and old files
are no longer read. The header word (`spacesolve` to `telesapr`), the extension (`.spcs` to
`.tlspr`) and the names in `link` lines were changed in these files, and nothing else.

`inch.tlspr` is a plate in an inch document: its sides come from parameters, and its hole has
the typed values `1/4` and `1/2`.

`suppress_all.tlspr` is an extruded rectangle whose sketch is suppressed whole (`suppress-all`,
added within version 2 on 2026-10-02), which leaves the extrusion out with it.

`sketch_on_face.tlspr` is a block with a chamfer and a cylinder drawn on the chamfer's face, a face
no entity names (workplane subtype `face`, added within version 2 on 2026-10-10).

`limits.tlspr` is the rod and slot of `examples/scripts/limits`: a line between a minimum and a
maximum length, and a point on a segment (`min-distance`, `max-distance` and `pt-on-segment`,
added within version 2 on 2026-10-10).
