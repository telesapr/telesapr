# History

Example: [`examples/scripts/history`](../../examples/scripts/history/history.go), a block with
a boss and copies of it, whose history is edited step by step.

Every group is a step of the model's history, listed on the home screen of the text window in
order. A group may use earlier ones: an extrusion uses its sketch, a sketch on a face uses the
group that made the face.

## Moving a group

The arrows after a group's name in the group list move it one place; Features > Move Group Earlier
and Move Group Later move the active group. A move that would put a group ahead of a group it
uses is refused, and the message names both.

Features > Move Group to End moves the active group to the end of the history together with the
groups only it needs, keeping their order: an extrusion goes with its sketch, and that sketch's
own workplane with it. A sketch that another group uses too stays where it is. Scripts do the
same with `Group.MoveToEndWithSources` and `Group.MoveWithSourcesBefore`; `Group.MoveToEnd`
moves the one group.

Deleting the active group, or a group it uses (which deletes it too), makes active the last
group before it that is left, so that the next new group still comes at the end of the history.

## Inserting a group

A new group goes in after the active one, so activating an earlier group (the radio button
before its name on the home screen) and adding one inserts it in the middle of the history; the
groups after it are regenerated on it. While an earlier group is active, what comes after it is
hidden and can't be picked, so nothing new refers to it. A script is held to the same: a
constraint on an entity of a later group, or a new group that would use one, is refused with a
message naming the later group. The file keeps the active group.

```go
doc.Activate(block)
doc.Chamfer(edges, 2, "")   // between block and what followed it
```

From a script:

```go
fmt.Println(boss.DependsOn(), sketch.UsedBy())
must(in3d.MoveBefore(sketch))
must(in3d.MoveToEnd())
```

## Suppressing

A group's screen has two boxes to leave it out. "Suppress this group's solid model" keeps its
sketch but leaves its solid out of the model, as SolveSpace does. "Suppress this group and the
groups that use it" leaves out the group and every group that uses it, with nothing deleted: the group list says what is left out and why,
and a group left out can't be made active. Undo, or clearing the box, brings it all back.

From a script, `group.SetSuppress(api.SuppressWhole)`, `api.SuppressSolid` or
`api.Unsuppressed`.

## Broken references

Deleting a group deletes the groups that use it, and a constraint goes with the entities it
held. Edit > Broken References lists each thing a deletion took, with what it used; the home
screen links to the list while it is not empty. Undo brings them back.

From a script, `doc.BrokenReferences()` and `doc.ClearBrokenReferences()`.

## Not yet

A group can't be moved ahead of a group it uses, even when the reference could be redirected.
