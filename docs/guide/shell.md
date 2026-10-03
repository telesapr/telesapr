# Shell

Example: [`examples/scripts/shell`](../../examples/scripts/shell/shell.go), an enclosure: a
block hollowed to 2 mm walls, open at the top.

New Group > Shell Solid (Shift+G) hollows the solid so far to walls of one thickness, open
where faces are taken away. Select one or more flat faces of the solid to take away. The
group's screen lists them and takes the thickness, as a number or an expression. A thickness
at which the walls meet is a problem on the group, and the solid passes through.

From a script:

```go
box := doc.Extrude(25, "box")
top := box.Solid().FaceAt(api.V(30, 20, 25))
sh := doc.ShellSolid([]api.SolidFace{top}, 2, "")
sh.SetThickness(1.5)
```

## Not yet

Every face of the solid must be flat. Cylinders are roadmap 15.2; apexes where four faces do
not meet at one point once moved, 15.3. Filleted and lofted solids wait for offsets of other
surfaces (roadmap 22.1).
