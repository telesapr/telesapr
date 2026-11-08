---
paths:
  - "doc/**"
  - "ROADMAP.md"
  - "CHANGELOG.md"
  - "README.md"
---

# Documents

A change in code brings the documents that describe it up to date, in the same commit. Which
document holds what:

| What changed | Where it is described |
|---|---|
| What a feature does, from the window and from a script, and what it does not do yet | its page in `doc/guide` (its "Not yet" section too), built on `examples/scripts` |
| A menu entry, shortcut, toolbar button or script call | `doc/guide/reference` (`menus.md`, `shortcuts.md`, `toolbar.md`), checked by `TestReferencePages` |
| A new feature | its row in `doc/dev-notes/done-audit.md` |
| Anything a `.tlspr` file holds | `doc/tlspr-format.md` |
| How an area is made, what was decided and what failed | the area's note in `doc/dev-notes`, with a "Refreshed <date>" line |
| Open work | its row in `ROADMAP.md`, taken out when done; the dated "Done" and "Tried" notes are in `git show af848717:ROADMAP.md` |
| A change a user would notice | a line under `## [Unreleased]` in `CHANGELOG.md` |
| What the program is, and its downloads | `README.md` |

- The guide is written plain, in short sentences, for a user who knows CAD but not this program.
  Its pictures are made by the scenarios in `doc/guide/scenarios` (`TestDocScenarios`) and
  regenerated with `scripts/regen-doc-images.sh`; `make guide-site` builds it with mkdocs
  (`mkdocs.yml`, `tools/docs/hooks.py`).
- After a rename or a removal, grep `doc/` and `ROADMAP.md` for the old name. A merge of several
  branches reads the docs they touched once more.
- A release renames `## [Unreleased]` to the version in the commit that bumps
  `pkg/version/version.go` (`doc/releasing.md`).
