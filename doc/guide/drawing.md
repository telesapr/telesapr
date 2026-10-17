# Constraints while drawing

Example: [`examples/scripts/autoconstrain`](../../examples/scripts/autoconstrain/autoconstrain.go),
which draws a slot of two lines and two arcs, and a ring of circles, roughly, and lets the
constraints the window would add square them up.

SolveSpace makes a line horizontal or vertical when it is drawn nearly so. TeleSAPR also offers,
while a line segment is being drawn in a workplane, and in this order:

- **tangent** to an arc the line starts on, when it leaves along the arc's tangent;
- **perpendicular** to a line it starts on, when it is nearly square to it;
- **parallel** to another line of the sketch, the nearest one, when it nearly is;
- **equal length** to another line, when it is within one percent of it.

While an arc is being drawn in a workplane:

- **tangent** at either end to a line or another arc that ends there, when it leaves along it
  (an arc is drawn as a half circle on the line between its ends, so this is when that line is
  square to the line it starts from);
- else the **same radius** as an arc or a circle of the sketch, the nearest in size, when it is
  within one percent of it.

While a circle is being drawn: the **same radius** as an arc or a circle, as for arcs.

What would be added is drawn on the entity as it follows the mouse, and is added when the point
is placed, unless it would be redundant. "Nearly" is an angle of about one degree, as for
horizontal and vertical. Holding Ctrl while drawing offers nothing; the setting "enable automatic
line constraints" on the configuration screen turns all of them off.

From a script, `Constrain.Auto` adds to a line, an arc or a circle of the active group what
the window would add as it is placed, and says whether it added one:

```go
c := doc.Sketch.Circle(V(30, 0, 0), 8.03)
if con, ok := doc.Constrain.Auto(c); ok {
	fmt.Println(con) // c004-eq-radius
}
```

## Not yet

Cubic splines, points and text are not offered constraints; a line or an arc ending on another
entity's middle is not made tangent or perpendicular there, and a circle is not made tangent to
a line.
