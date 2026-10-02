# Version 1 files

Files in `.spcs` format version 1, as the first reader wrote them on 2026-10-02. Every later
reader must open them: `TestCorpus` in `pkg/persist/spcs` loads and regenerates each one.

Never regenerate or edit these files. When the format gains a version 2, add a `v2` directory
beside this one and keep this one as it is.

`fields.spcs` sets every field version 1 defines. It is not a model that regenerates, so the
test only decodes it, and requires that it encodes to the same bytes.

`parameters.spcs` was added on 2026-10-02 with named parameters (roadmap step 3), which version 1
gained as an addition: `param` lines and expressions after `=`. It is the plate that
`examples/scripts/plate` makes.
