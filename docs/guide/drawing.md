# Constraints while drawing

There is no example script: this is about drawing with the mouse.

SolveSpace makes a line horizontal or vertical when it is drawn nearly so. TeleSAPR also offers,
while a line segment is being drawn in a workplane, and in this order:

- **tangent** to an arc the line starts on, when it leaves along the arc's tangent;
- **perpendicular** to a line it starts on, when it is nearly square to it;
- **parallel** to another line of the sketch, the nearest one, when it nearly is;
- **equal length** to another line, when it is within half a percent of it.

What would be added is drawn on the line as it follows the mouse, and is added when the point is
placed, unless it would be redundant. "Nearly" is an angle of about one degree, as for
horizontal and vertical. Holding Ctrl while drawing offers nothing; the setting "enable automatic
line constraints" on the configuration screen turns all of them off.

## Not yet

Only line segments are offered constraints; arcs, circles and curves are not, and a line ending
on another entity is not made tangent or perpendicular there.
