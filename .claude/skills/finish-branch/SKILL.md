---
name: finish-branch
description: Finish a TeleSAPR work branch - run make ci, bring the documents up to date, merge it into devel and delete it. Use when a branch's work is done and is to be merged.
---

# Finishing a branch

1. The tree is clean and every commit does one thing. Never `git stash` here.
2. `make ci` on the branch (checks, tests, heavy tests, lint, vuln, race detector); it is what
   catches what a branch's CI run (`make test-fast` only) does not. Fix on the branch until it
   passes.
3. Read the documents the branch touched once more against the code: the guide page and its
   "Not yet", `doc/guide/reference`, `doc/dev-notes/done-audit.md`, `doc/tlspr-format.md`,
   the area's dev note, `ROADMAP.md` (row taken out when done), `CHANGELOG.md` under
   `## [Unreleased]` for anything a user would notice.
4. Merge into `devel` (`git switch devel && git merge <branch>`; a squash when the user asks for
   one) and delete the branch. Do not push, and do not merge `devel` into `master`, unless asked.
5. Say what was merged, what `make ci` reported, and what is left open.
