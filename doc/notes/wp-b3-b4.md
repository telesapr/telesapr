# WP-B3 and WP-B4: links that take values, relink, named datums

Branch `wp-b3-links-datums`, from devel at 83b573fd. Items 3 and 2 of
`modelling_upgrades.md`.

## B3, done

- A link group holds `LinkConfiguration` and `LinkParams` (name, expression in the linking
  sketch's parameters, value), saved as `link-configuration` and `link-param <name> = <expr>`
  (`doc/tlspr-format.md`), with no version bump: an older reader skips them.
- With either set, `pkg/persist/link` reads the model's sketch through its `SketchReader`,
  activates the configuration, sets each parameter to the plain value, reads the model's own
  links quietly and regenerates it (`parametric.go`). The result is cached under a key with the
  overrides (`Group.LinkOverridesKey`) and with the stamps of every file the model linked in
  turn, so a nested file that changes invalidates it (`cache.go`).
- A model linking itself with values is not regenerated inside itself: the chain of open files
  is passed down (`loadCtx`); depth is capped at `maxPartsDepth`.
- `EvaluateParameters` evaluates the link values; `generateAll` reads the links again when a
  group's key differs from `LinkLoaded`, the key it was read with.
- Relink: `Sketch.SetLinkFile`, as `feature-inputs-and-relink.md` designs it.
- Window: the link screen's `[relink]`, a configuration row of radio links, a row per model
  parameter with `[pass a value]`, `[change]`, `[model's]` (`pkg/ui/link_screen.go`).
- Scripts: `LinkGroup.File`, `SetFile`, `Configuration`, `SetConfiguration`, `Param`,
  `SetParam`, `ModelParams`, `ModelConfigurations`. Example `partsizes`.

## B4, done

- A datum is a name for an entity a mate or a constraint can use: a point, workplane, normal,
  face entity, line, circle or arc (`sketch.CanBeDatum`). `Sketch.Datums`, saved as
  `datum <name> <handle>`, one name per entity; part of the undo state and of a `ShellJob`'s
  copy. A face is named by its face entity, since a solid face's identity is that handle.
- A linked model's datums come in with the link (`Group.LinkDatums`, from the saved file or
  the regenerated model) and resolve as `group.name` through the link's `Remap`
  (`Sketch.ResolveDatum`). A link sharing its name with another group is named by its
  description, `g004-tile.top` (`Sketch.LinkLabel`), as a part linked twice from the window is.
- A datum whose entity is gone is kept (undo may bring the entity back) and says so where it
  is resolved or listed.
- Window: Edit > Datums lists the model's own datums ([select], [rename], [delete]), names
  the selected entity, and lists each link's datums with [select]; the screen stays while
  something is selected, as the drawing's does, so two datums can be selected for a mate.
- Scripts: `doc.Datums` (`List`, `Names`, `Name`, `Delete`, `Get`); mates take
  `doc.Datums.Get("base.top")` as any entity. Example `datums`.

## Not done

- Mates by name in the window are a selection from the datums screen, then the mate's menu
  entry; there is no "mate these two names" dialog.
- A datum names an entity, so a face made by no entity (a fillet's, a Boolean's own) can't be
  named; nor can an edge of the solid, which has a `topo.Name` rather than an entity.

## Traps

- A new link is placed off the middle of the view, not at the origin: compare offsets with the
  one before an edit, not with zero.
- A path scenario that shows a link's file on the screen must open a model beside the file
  first, or the screen shows the machine's absolute path and the columns move.
- `ShellJob` copies must carry `LinkLoaded`, or `Apply` makes the next regeneration read the
  links again. (The B3 commit missed it; the B4 commit adds it.)
- A selection replaces whatever screen the text window shows with the selection's
  description, except screens that ask for a selection (drawing, datums): a scenario that
  selects from a screen must keep that screen up, or its next click lands elsewhere.
- In `GroupSelection` a selected point is in `gs.point`, not `gs.entity`.
