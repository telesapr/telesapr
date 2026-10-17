# Architectural decision records

One file per decision that a later contributor is likely to question, numbered in the order
they were taken. A superseded record stays, marked "Superseded by", since the history is the
point. The template is at the end of this file.

| Number | Title | Status |
|---|---|---|
| [0001](0001-separate-from-solvespace.md) | TeleSAPR is a separate project, not a SolveSpace port | Accepted |
| [0002](0002-tlspr-file-format.md) | A text file format of its own, `.tlspr` | Accepted |
| [0003](0003-pkg-directory.md) | Packages stay under `pkg/` | Accepted |
| [0004](0004-scripting-with-yaegi.md) | Scripting is Go, run by yaegi, and MCP exposes it | Accepted |
| [0005](0005-gpl-3-or-later.md) | Licence: GPL-3.0-or-later | Accepted |

## Template

```markdown
# [Short title]

**Status:** Proposed | Accepted | Deprecated | Superseded by NNNN
**Date:** YYYY-MM-DD

## Context
What the problem was, which options were weighed, and why a decision was needed then.

## Decision
"We will ..."

## Consequences
What is easier, what is harder, and what could go wrong.
```
