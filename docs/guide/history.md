# History

Example: [`examples/scripts/history`](../../examples/scripts/history/history.go), a block with
a boss and copies of it, whose history is edited step by step.

Every group is a step of the model's history, listed on the home screen of the text window in
order. A group may use earlier ones: an extrusion uses its sketch, a sketch on a face uses the
group that made the face.

## Moving a group

The arrows after a group's name in the group list move it one place; Edit > Move Group Earlier
and Move Group Later move the active group. A move that would put a group ahead of a group it
uses is refused, and the message names both. A new group goes in after the active one, so
activating an earlier group and adding one inserts it in the middle of the history.

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
