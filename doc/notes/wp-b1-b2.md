# WP-B1 and WP-B2: thread as one group, cosmetic threads

Branch `wp-b1-thread-group`, from devel 949e0399. Items 4 and 19 of `modelling_upgrades.md`.

## Done

- `Group.Thread` (`sketch.GroupThread`: the circle and `ThreadOptions`) on a thread's helix
  group, saved as a `thread { ... }` block (`pkg/persist/tlspr`), copied by undo.
- `Sketch.EditThread(helix, options)` draws the four groups again (`thread_draw.go`,
  `thread_edit.go`): groups keep their handles, sketches reuse their requests by type and
  order, constraints of the blank, profile and helix groups are made anew.
  `Sketch.ThreadGroups`, `ThreadOf`, `ThreadLengthText`, `ErrNotThread`.
- Length by expression (`ThreadOptions.LengthExpr`): the blank's top and the helix's length
  dimensions carry `(expr) ± k`; evaluation keeps `Thread.Length` current; renames and
  unit changes reach it.
- Crests (`thread.Crest`: standard, truncated, rounded; `Profile.WithCrest`).
- Tests: `pkg/thread/crest_test.go`; `pkg/persist/slvs/thread_edit_test.go`
  (`TestEditThread` against a fresh build per option, `TestEditThreadUndo`,
  `TestEditThreadRefused`, `TestThreadLengthExpression` incl. inches, `TestThreadCrests`
  with a section-area oracle at chord 0.003%: the crest arc is too small for 0.01%).
- Goldens: `testdata/thread/*.tlspr` gain only the thread block.

- API: `ThreadOptions.Crest`, `.LengthExpr`; `Group.AsThread()`, `Group.IsThread()`,
  `ThreadGroup.Options/Groups/Edit` (`pkg/script/api/thread.go`, `thread_test.go`), in the
  parity and completeness tables.
- UI: the helix group's screen has `[edit thread]` at the end of the "fixed" row, opening the
  thread screen in edit mode ("make the thread again now"); crest radios at the end of the
  external/internal row; length takes an expression. Rows below stay put, so old scenarios'
  clicks still land. Scenario `ext_thread_edit`, path pair `path_thread_edit`, example
  `examples/scripts/threadedit`, guide `doc/guide/threads.md`, done-audit row, ROADMAP row.

## Left

- Multi-start: one helix sweeps all ridges, stacked a pitch apart in one sketch, which is
  the angular offset already (ridge i a pitch up is ridge 0 turned by i/n of a turn); it costs
  (n-1) pitches of length. Starting all ridges at one height needs a sketch and a helix per
  start: not done.
- Fade-out (run-out): not done; it would need groups after the helix (a cone cut), so the
  thread would no longer be four groups.
- B2 cosmetic threads in drawings: next.
