# Version 1 files

Files in `.tlspr` format version 1, as the first reader wrote them on 2026-10-02. Every later
reader must open them: `TestCorpus` in `pkg/persist/tlspr` loads and regenerates each one.

Never regenerate or edit these files. When the format gains a version 2, add a `v2` directory
beside this one and keep this one as it is.

One exception: on 2026-10-03 the project was renamed from spacesolve to TeleSAPR, and old files
are no longer read. The header word (`spacesolve` to `telesapr`), the extension (`.spcs` to
`.tlspr`) and the names in `link` lines were changed in these files, and nothing else.

`fields.tlspr` sets every field version 1 defines. It is not a model that regenerates, so the
test only decodes it, and requires that it encodes to the same bytes.

`parameters.tlspr` was added on 2026-10-02 with named parameters (roadmap step 3), which version 1
gained as an addition: `param` lines and expressions after `=`. It is the plate that
`examples/scripts/plate` makes.
