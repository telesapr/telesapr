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

## B4, in progress

Design: a datum is a name for an entity (point, workplane, normal, face entity, line, circle,
arc), `Sketch.Datums`, saved as `datum <name> <handle>`. A linked model's datums come in as
`Group.LinkDatums` (read by the link loaders), and resolve as `group.name` through the link's
`Remap` (`Sketch.ResolveDatum`). Mates take the entity `Doc.Datum(name)` returns. Faces are
named by their face entity, since a solid face's identity is that entity's handle.

## Traps

- A new link is placed off the middle of the view, not at the origin: compare offsets with the
  one before an edit, not with zero.
- A path scenario that shows a link's file on the screen must open a model beside the file
  first, or the screen shows the machine's absolute path and the columns move.
- `ShellJob` copies must carry `LinkLoaded`, or `Apply` makes the next regeneration read the
  links again.
