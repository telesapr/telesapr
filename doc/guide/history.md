# History

Example: [`examples/scripts/history`](../../examples/scripts/history/history.go), a block with
a boss and copies of it, whose history is edited step by step.

Every group is a step in the model's history. The home screen of the text window lists the
groups in order. A group may use earlier groups: an extrusion uses its sketch, and a sketch on
a face uses the group that made the face.

## Moving a group

The arrows after a group's name in the group list move it one place up or down. Features >
Move Group Earlier and Move Group Later move the active group. A move that would put a group
ahead of a group it uses is refused, and the message names both groups.

Features > Move Group to End moves the active group to the end of the history. The groups that
only it needs go with it, in their order: an extrusion takes its sketch, and that sketch takes
its own workplane. A sketch that another group also uses stays where it is. In a script,
`GroupHistory.MoveToEndWithSources` and `GroupHistory.MoveWithSourcesBefore` do the same.
`GroupHistory.MoveToEnd` moves the one group alone.

When you delete the active group, or a group it uses (which deletes it too), the last remaining
group before it becomes active. So the next new group still goes at the end of the history.

## Inserting a group

A new group goes in after the active one. To insert a group in the middle of the history, make
an earlier group active (the radio button before its name on the home screen) and add the new
group. The groups after it are regenerated on top of it.

While an earlier group is active, everything after it is hidden and can't be picked. So nothing
new can refer to it. A script is held to the same rule: a constraint on an entity of a later
group, or a new group that would use one, is refused. The message names the later group. The
file keeps which group is active.

```go
doc.Activate(block)
doc.Features.Chamfer(edges, 2, "")   // between block and what followed it
```

From a script:

```go
fmt.Println(boss.History().DependsOn(), sketch.History().UsedBy())
must(in3d.History().MoveBefore(sketch))
must(in3d.History().MoveToEnd())
```

## Suppressing

A group's screen has two boxes that leave it out of the model:

- "suppress this group's solid model" keeps the group's sketch but leaves its solid out, as
  SolveSpace does.
- "suppress this group and the groups that use it" leaves out the group and every group that
  uses it. Nothing is deleted. The group list says what is left out and why. A group left out
  can't be made active.

Undo, or clearing the box, brings everything back.

From a script: `group.SetSuppress(api.SuppressWhole)`, `api.SuppressSolid` or
`api.Unsuppressed`.

## Broken references

Deleting a group deletes the groups that use it. A constraint goes with the entities it held.
Features > Broken References lists each thing a deletion took, with what it used. The home
screen links to the list while it is not empty. Undo brings them back.

From a script: `doc.Features.BrokenReferences()` and `doc.Features.ClearBrokenReferences()`.

## Not yet

A group can't be moved ahead of a group it uses, even when the reference could be redirected.
