---
name: regen-goldens
description: Regenerate TeleSAPR's golden files (.tlspr fixtures, .goui text-window goldens, renderer PNGs, tlspr import goldens, guide pictures) with the right script, review the diff and commit them on their own. Use when a golden test fails after an intended change.
---

# Regenerating goldens

A golden changes only in a commit of its own that says why. Never edit one by hand, and never
regenerate to make a failing test pass without knowing why it changed.

1. Decide whether the change is intended. A golden that moved from a change meant to be
   behaviour-neutral is a bug, not a regeneration. `srf.CxxSort` and `pkg/libm` are frozen
   because the goldens record them.
2. Run the script for the kind of golden:

   | Golden | Script |
   |---|---|
   | `.tlspr` fixtures, PNGs and the rest (`TELESAPR_REGEN_GOLDEN=1`) | `scripts/regen-golden.sh [areas...]` |
   | `testdata/ui/golden/*.goui` | `scripts/regen-goui.sh [script names...]` |
   | `testdata/tlspr/golden` (imports; `testdata/tlspr/v1` never) | `scripts/regen-tlspr-golden.sh` |
   | `doc/guide/images` | `scripts/regen-doc-images.sh [scenarios...]` |
   | `doc/guide/videos` (not goldens) | `scripts/regen-guide-videos.sh [scenarios...]` |
   | the window's own menus and dialogs | `scripts/regen-doc-window-images.sh` |

3. Review every changed file (`git diff --stat`, then the diffs; look at every changed picture)
   and confirm each change is the one intended, and nothing else moved.
4. `make test-fast`, then commit the goldens alone, with a message that says what changed them
   and why.
