# Version 2 files

Files in `.spcs` format version 2, as the first reader wrote them on 2026-10-02. Every later
reader must open them: `TestCorpus` in `pkg/persist/spcs` loads and regenerates each one.

Never regenerate or edit these files. Version 2 made `units` the document's unit, which
lengths written with `=` are in.

`inch.spcs` is a plate in an inch document: its sides come from parameters, and its hole has
the typed values `1/4` and `1/2`.
