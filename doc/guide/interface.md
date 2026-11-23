# A tour of the window

The window has two parts:

- the **graphics window**, on the left, shows the model. You draw and pick there.
- the **property browser**, on the right, shows text screens: the list of groups, a group's
  settings, what is selected. Its links are clicked like a web page's. View > Show Property
  Browser (Tab) hides and shows it.

![The window with the fillet example](images/interface-1-window.png)

The model in these pictures is made by
[`examples/scripts/fillet`](../../examples/scripts/fillet/fillet.go): a block with its upright
edges rounded. [Getting started](getting-started.md) makes a part step by step.

The menu bar along the top holds every command, with its key where it has one. Alt and a menu's
underlined letter opens it; F10 opens the first.

![The File menu](images/interface-7-file-menu.png)

## The toolbar

The toolbar runs down the left side of the graphics window. Hover a button to see what it does
and its key. It has five blocks, from the top:

![The toolbar's five blocks](images/interface-2-toolbar.png)

1. **Sketch**: line segment, rectangle, circle, arc, text in a TrueType font, image, tangent arc
   at a point, cubic Bezier spline, datum point, toggle construction, and split curves where
   they cross.
2. **Constrain**: distance, diameter or length; angle; horizontal; vertical; parallel or
   tangent; perpendicular; point on a line, curve, plane or point; symmetric; equal length,
   radius or angle; same orientation; other supplementary angle; and toggle reference dimension.
3. **New groups**: extrude, lathe, helix and revolve the active sketch; step and repeat rotating
   and translating; a sketch in a new workplane; a sketch in 3d; and link or assemble a file.
4. **Features that take a selection**: fillet and chamfer edges, shell open at faces, sweep along
   a path, loft to profiles, mirror in a plane, pattern along a path, and screw thread.
5. **View and measure**: nearest isometric view, align the view to the active workplane, measure
   the selection, and section view. The measure and section buttons stay lit while they are on.

When the window is too short for two columns, the toolbar takes three. View > Show Toolbar
(Ctrl+Tab) hides and shows it.

## The property browser

### The home screen

![The home screen of the property browser](images/interface-3-home.png)

The first line (marked at the top) says where you are. "home" is a link back to this screen from
any other. After it comes the screen's place, as "home › g003-block", then the mode: "in plane:"
and the workplane's name while you draw in a workplane, or "drawing / constraining in 3d".

The icons below it show and hide things in the graphics window. A red cross on an icon means
that thing is hidden. The same switches are under View > Show. From the left:

- workplanes of inactive groups, normals, points, construction entities;
- constraints and dimensions, which cycles through all of them, dimensions only, and none;
- whether faces can be picked with the mouse;
- then the solid: shaded, its edges, its outline, its triangle mesh;
- last, how hidden lines are drawn: not at all, stippled, or as visible lines.

The home screen lists the groups in the order they are made:

- **active**: click the circle to make a group active. You draw in the active group, and the
  groups after it are left out of the model until you go back.
- **shown**: the box hides or shows the group. A "-" marks a group after the active one.
- **dof**: how the group solved (the table below). Click the value to see more.
- **group-name**: click it to open the group's screen. The arrows move the group earlier or
  later in the history.

The **dof** column says:

| Value | Means |
|---|---|
| ok | fully held |
| a number | the degrees of freedom left |
| err | a problem, as a sketch whose outline is not closed under an extrusion. A sketch no solid is made from, as a sweep's path, may be open. The message is written in the top left of the graphics window, with a cross on the model where the problem is |
| ERR | the group could not be solved |
| old | a group after the active one, not made again since the last change ([History](history.md#inserting-a-group)) |

Below the list, "show all" and "hide all" show or hide every group at once, and "only
unconstrained" shows only the groups with degrees of freedom left. The links "line styles",
"view", "configuration" and "parameters" open those screens. The configuration screen is also
Edit > Preferences.

### A group's screen

![The screen of an extrude group](images/interface-4-group.png)

A group's screen holds its settings: here an extrusion's sides and depth, whether it adds or
takes away material, its appearance. The first line reads "home › g003-block". The links on the
"GROUP" line rename or delete the group, and make it active, hide it, or move it earlier (↑)
or later (↓).

## Picking things

**Hover.** Whatever is under the pointer is drawn in yellow before you click it. A face is
drawn with a dotted pattern.

![The top face hovered](images/interface-5-hover.png)

A line, a point or a dimension within 10 pixels of the pointer is hovered before the face under
it, so that thin things are easy to hit. A line or a point hidden behind the solid is not: the
face in front of it is hovered instead. To pick something behind the solid, turn the view, or
show hidden lines as visible lines (the last switch above).

Where several points lie at one place, the property browser lists them under ALSO HERE, with
the curves through that place, so that you can pick one a click can't reach
([Constraints](constraints.md#what-lies-at-a-point)).

**Click** selects what is hovered. Selected things are drawn in red, and the property browser
describes them: here a flat face, its normal and a point on it, and its group. Each further click
adds to the selection. Ctrl+click takes a thing out of it. A click on empty space, or Esc (Edit >
Unselect All), clears it.

![The top face selected](images/interface-6-selected.png)

**A box (marquee).** Drag from empty space to draw a box. It selects only in the active group.

- Dragged from left to right, the box is drawn in the hover colour and selects what lies
  wholly inside it. Here: the rectangle's left side and its two ends.

  ![A box dragged from left to right](images/interface-7-window-drag.png)

  ![What it selects](images/interface-8-window-result.png)

- Dragged from right to left, the box is drawn in the selection colour and selects whatever it
  crosses. The same box now takes the top and bottom sides too.

  ![The same box dragged from right to left](images/interface-9-cross-drag.png)

  ![What it selects](images/interface-10-cross-result.png)

Edit > Select All (Ctrl+A) selects everything shown in the active group. Edit > Select Edge
Chain (Ctrl+E) adds the lines and curves of the active group joined end to end with those
selected.

**Dragging.**

- Drag a point or a line to move it. What is not held follows.
- A long drag keeps the sketch the way it was drawn: an arm's elbow stays bent the same way, and
  a triangle does not turn over. Where that can't be done, the drag still moves it, and the
  group is marked "(note)" ([Constraints](constraints.md#large-edits-and-drags)).
- Drag a dimension's label to move the label.
- In an assembly, a part that moves in a mechanism is pulled along its hinges, and the parts
  joined to it move with it ([Mechanisms](mechanisms.md), experimental). Dragging a point of a
  linked part with Shift or Ctrl turns the part; a mechanism's part turns only as its hinges let
  it.
- The arrow keys move what is selected by one pixel of the view, ten with Shift, as a drag does
  (`Entity.MoveBy` in a script).
- Delete removes what is selected.

## The context menu

A right click (without dragging) selects what is under the pointer and opens a menu of what
applies to it:

| Under the pointer | Offers, among others |
|---|---|
| an edge of the solid | Fillet Edges, Chamfer Edges, Sweep Along Edges |
| a face of the solid | Shell Solid Open Here, Select Face Edges, Sketch On Face, an Appearance submenu, Measure Selection |
| a flat face an entity makes | Mirror in This Plane too |
| a face, while a drafted extrusion is active | Draft Selected Sides |
| a face, while a sketch is active | Wrap Sketch Onto This Face |
| a cylindrical face or a circular edge | Screw Thread... |
| a dimension | Edit Value... |
| anything of a group | a Group submenu: Activate, Hide, Suppress with What Uses It, Delete Group |
| empty space | Select All |

![The context menu of an edge](images/interface-8-context-menu.png)

While you draw lines or arcs, a right click stops drawing, as Esc does.

## Moving the view

| To | Do |
|---|---|
| Turn the view | Drag with the middle button, or with Shift and the right button |
| Move (pan) the view | Drag with the right button, or with Shift and the middle button |
| Turn about the line of sight | Drag with Ctrl and the middle button |
| Zoom about the pointer | Scroll the wheel; with Shift, in finer steps |
| Zoom in or out about the middle of the window | + or - (View > Zoom In, Zoom Out) |
| Fit the model in the window, beside the toolbar | F (View > Zoom To Fit) |
| Look straight at the active workplane | W (View > Align View to Workplane) |
| Snap to the nearest view along an axis | F2 (View > Nearest Ortho View) |
| Snap to the nearest isometric view | F3 (View > Nearest Isometric View) |
| Center the view at a point | Select the point, F4 (View > Center View At Point) |

The configuration screen has two switches for the mouse: "use turntable mouse navigation" keeps
the Z axis upright while turning, and "use camera mouse navigation" turns the view the other
way, as if you moved the camera instead of the model.

## The View menu

Besides the view moves above, the View menu has:

- **Show**: the switches of the property browser's icons, and reference dimensions and comments
  apart from the other constraints.
- **Show Snap Grid** (>), **Darken Inactive Solids**, **Use Perspective Projection** (`` ` ``),
  and **Show Exploded View** (\\).
- **Section View** (Ctrl+Shift+X): cut the model open ([Section view](section-view.md)).
- **Show Drawing Sheet** and **Drawing Sheet...** ([Drawings](drawings.md)).
- **Dimension Units**: millimetres, metres, inches, or feet and inches.
- **Show Toolbar** (Ctrl+Tab), **Show Property Browser** (Tab), and **Full Screen** (Ctrl+F11).

## Finding a command

Edit > Find Command (Ctrl+K) opens a search in the property browser. Type words of a menu
entry's name and press Enter: the entries that match are listed, with their keys. Enter again
runs the first one, marked "(Enter)", or click any of them. If only one entry matches, it runs at
once. Entries that can't run now are listed last, with the reason.

![The search after typing "zoom"](images/interface-11-find.png)

## Undo

Edit > Undo (Ctrl+Z) takes back the last step: a line drawn, a constraint, a group made, a value
changed. Edit > Redo (Ctrl+Y or Ctrl+Shift+Z) puts it back. Edit > Regenerate All (Space) solves
and rebuilds the whole model again.

These pictures are made by [`scenarios/interface.scenario`](scenarios/interface.scenario), which
replays the same clicks on the program (`scripts/regen-doc-images.sh`).
