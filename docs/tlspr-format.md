# The `.tlspr` file format, version 2

`.tlspr` is the file format TeleSAPR opens and saves. It replaces SolveSpace's `.slvs`, which
TeleSAPR only imports (File, Import) and never writes. The format is text, line oriented, and
written in a stable order, so two saves of the same model give the same bytes and a change to the
model gives a small diff.

The reader and writer are `pkg/persist/tlspr`. The syntax layer (`lex.go`, `parse.go`, `print.go`)
knows nothing about sketches. The model layer (`encode.go`, `decode.go`) maps the syntax tree to
`sketch.Sketch`.

## Example

Two datum points ten millimetres apart, as TeleSAPR saves them
(`testdata/tlspr/v1/sketch.tlspr`), shortened where `...` stands:

```
telesapr 2

units mm

group 0x1 sketch-3d "#references" {
  color 0 0 0 0
  visible
}

group 0x2 sketch-in-plane "sketch-in-plane" {
  order 1
  subtype point-ortho
  workplane 0x80020000
  color 0 0 0 0
  predef {
    q 1 0 0 0
    origin 0x10001
  }
  visible
}

request 0x1 workplane {
  group 0x1
}
...
request 0x4 datum-point {
  group 0x2
  workplane 0x80020000
}

request 0x5 datum-point {
  group 0x2
  workplane 0x80020000
}

constraint 0x1 pt-pt-distance {
  group 0x2
  workplane 0x80020000
  value = 10
  pt-a 0x40000
  pt-b 0x50000
  label-offset 0 2.5 0
}

value 0x10010 0
...
value 0x40010 -5
value 0x40011 5
value 0x50010 5
value 0x50011 5

entity 0x10000 workplane {
  group 0x1
  workplane 0xffffffff
  points 0x10001
  normal 0x10020
  act-visible
}
...
```

Generated entities, and for a solid its mesh or shell, follow the hand-made records. They are
described under "Generated data".

## Lexical structure

- The file is UTF-8. Lines end with LF. A CR before the LF is accepted on reading and never written.
- `#` starts a comment that runs to the end of the line, except inside a string.
- Spaces and tabs separate tokens. Line ends matter: a field ends at the end of its line.
- A **word** is a run of characters other than space, tab, line end, `{`, `}`, `"`, `=` and `#`.
  Names, numbers and handles are all words, and the model layer decides how to read one.
- A **string** is a double-quoted Go string literal, with the escapes `\\`, `\"`, `\n`, `\t`,
  `\r`, `\xHH`, `\uHHHH` and the others Go accepts. It may not span lines. The writer escapes
  every control character and leaves other printable Unicode as it is.
- `{` and `}` open and close a block. `=` introduces an expression.

## Grammar

```ebnf
file       = header , { statement } ;
header     = "telesapr" , integer , eol ;
statement  = field | block | blank ;
field      = name , { value } , eol
           | name , "=" , expression , eol ;
block      = name , { value } , "{" , eol , { statement } , "}" , eol ;
value      = word | string ;
name       = word ;
expression = ? the rest of the line up to a comment, trimmed; not empty ? ;
blank      = eol ;
```

The header must be the first statement; comments and blank lines may precede it. Nothing may
follow `{` on its line except a comment, and `}` stands on a line of its own.

A field with no values is a flag: present means true, absent means false.

### Expressions

A field written with `=` holds what a user typed for a value: a plain number, such as
`value = 30`, or the text of an expression, such as `value = 1/4` or `value = wall*4`. A reader
keeps the whole text and evaluates it at each regeneration.

**Lengths are in the document's unit**, the one the `units` line names: in a file with
`units inch`, `value = 1/4` on a distance is a quarter inch. Angles are in degrees, and
trigonometric functions take and give degrees. Ratios, counts and scales have no unit. A
parameter is a plain number, which takes the unit of the value it drives: in an inch document
`param wall = 2.5` makes `value = wall` 2.5 inches. Feet and inches (`ft-in`) count in inches.

Everything else the file holds is in millimetres whatever `units` says: solved values (`value
<handle> <real>`), entities, meshes and shells, label offsets and style sizes.

The expression syntax is that of the dimension editor: numbers, parameter names, `+ - * / ^`,
brackets, the constants `pi` and `e`, and the functions `sqrt`, `sq`, `sin`, `cos`, `tan`,
`asin`, `acos`, `atan`, `atan2`, `hypot`, `exp`, `ln`, `log`, `abs`, `floor`, `ceil`,
`round`, `min` and `max`. An expression the syntax refuses is an error in the file.

### Parameters

```
param <name> = <expression>
```

A named value that expressions use. The name is a letter or underscore followed by letters,
digits and underscores, and is none of the constant and function names above. A parameter's
expression may name other parameters, in any order in the file; the writer keeps the order the
user made them in. A second parameter of the same name is an error.

A configuration lists expressions for parameters by name; the one `active-configuration` names
is the one the parameter lines hold, written again inside it. A configuration may name a
parameter the table no longer has.

A constraint's `name` is read the same way: the value its label shows, in the document's unit,
without the sign of a side, and a diameter shown as a radius as the radius. Parameters and
named constraints share the names.

What cannot be evaluated is not an error of the file: a cycle, a name that is no parameter, a
value that is no number, or a group value out of its range (fewer than 1 or more than 999
copies, a scale of 0). The reader keeps the text, so a save writes it back, and reports it with
its place. The value it drives keeps its default, 1 for copies and 0 otherwise.

## Values

| Kind | How it is written |
|---|---|
| Integer | Decimal, with an optional `-`. |
| Real | The shortest decimal form that reads back to the same 64-bit value. Plain notation for magnitudes from 1e-6 up to 1e21, exponent notation such as `1.5e-09` outside, and `nan`, `inf`, `-inf` when they occur. Zero is `0`, and negative zero is `-0`. |
| Handle | `0x` and lowercase hexadecimal with no leading zeros, such as `0x40010`. The handle 0 means "none" and the field is left out. `0x0` is written only inside a list where a position must be held. |
| Colour | Four integers 0 to 255: red, green, blue, alpha. Alpha 255 is opaque. |
| Flag | The field name alone. |
| Enumeration | A lowercase name with hyphens, from the tables below. |
| String | A Go string literal, as above. |

## Statements at the top level

| Statement | Content |
|---|---|
| `telesapr <major>` | The header. |
| `units <unit>` | `mm`, `inch`, `m` or `ft-in`: the document's unit, which typed lengths are in. A file without it is in `mm`. |
| `active <handle>` | The group that was active, written only when it is not the last. What is shown, and so each entity's `act-visible`, follows it. A file without it opens with the last group active. |
| `param <name> = <expression>` | A named parameter, see "Parameters". |
| `configuration <string> { param <name> = <expression> ... }` | A configuration: a named set of the parameters' expressions, see "Parameters". |
| `active-configuration <string>` | The configuration the table holds, if one does. |
| `drawing { ... }` | The model's drawing: `sheet <name>`, `portrait`, `scale <real>` (0 or left out to fit), `first-angle`, `hidden`, a `view <kind> <x> <y>` for each placed view (none for the standard ones), a `section <view> <offset> <x> <y>` for each section view and a `detail <view> <cx> <cy> <r> <scale> <x> <y>` for each detail view (`x`, `y` 0 to place it), a `dimension <view> <kind> <offset> { solid-edge ... solid-face ... }` block for each dimension, `view` a view kind or a section's or detail's letter, `kind` length, distance, diameter or radius, its edges by name as a chamfer's are, then its faces by identity, `solid-face <handle>`, and `title`, `author`, `number`, `revision`, `material`, `company` strings. |
| `group <handle> <type> <name> { ... }` | A group. |
| `request <handle> <type> { ... }` | A request, what the user drew. |
| `constraint <handle> <type> { ... }` | A constraint. |
| `style <handle> <name> { ... }` | A custom style. The built-in styles are not written. |
| `value <handle> <real>` | A solver parameter's last solved value. It is the solver's starting guess and picks the solution branch, so it is part of the model. |
| `entity <handle> <type> { ... }` | A generated entity, see "Generated data". |
| `mesh { ... }` | The last group's triangle mesh, see "Generated data". |
| `shell { ... }` | The last group's exact shell, see "Generated data". |

### Order

The writer puts statements in this order: header, `units`, parameters in the user's order,
groups in their `order`, ties by handle, then requests, constraints, styles, values and
entities, each by handle, then `mesh` and `shell`. A reader accepts any order of the top-level statements after the header.

The printer separates top-level statements with a blank line before every block and wherever
the statement name changes, so runs of `value` lines stay together. Blocks are indented by two
spaces per level.

### Group

```
group <handle> <type> <name> {
  order <integer>
  subtype <subtype>
  workplane <handle>        # the group's active workplane
  op-a <handle>
  op-b <handle>
  loft-through <handle> ... # loft: the profiles between op-a and op-b a smooth loft goes through, in order
  copies = <expression>     # step and repeat: the number of copies
  pitch = <expression>      # helix: the fixed pitch, a length; 0 when free
  distance = <expression>   # chamfer: the distance from each edge, a length (the field pitch is under another name)
  second-distance = <expression>  # chamfer: the distance on each edge's second face (the one of the higher identity), a length; left out for the same as distance (the field val-c under another name)
  draft = <expression>      # extrude: the sides' lean into the solid, in degrees (the field val-c, unused by other groups)
  twist = <expression>      # sweep: the profile's turn about the path over its whole length, in degrees (the field val-c)
  sweep-up <x> <y> <z>      # sweep: the direction its frames are held to, rather than turning least; left out when none
  color <colour>
  skip-first
  mirror-solid              # mirror: the whole solid so far, not op-a's own
  appearance <name>         # matte, plastic, glossy or metal; left out for plastic
  face-appearance <face> <name>  # a face's own appearance, by its identity; one line each
  combine <combine>
  force-to-mesh
  edges <handle> ...        # chamfer: the edges cut; sweep: the path, in the order given; loft: its guide curves
  start-tangency = <expression>  # loft: how square it leaves its first profile, a number; 0 or left out for free
  end-tangency = <expression>    # loft: how square it reaches its last profile
  draft-faces <handle> ...  # extrusion: the curves of its sketch whose sides the draft leans; left out for all
  draft-neutral = <expression>  # extrusion: where along it, from the sketch's plane, the draft's neutral plane lies, a length (the field pitch under another name)
  solid-edge <face> <face> <n> of <m>  # chamfer, fillet: an edge of the solid cut; sweep: an edge of its path; by name, one line each
  predef { ... }
  visible
  suppress                  # the solid is left out
  suppress-all              # the whole group is left out, and the groups that use it
  relax-constraints
  allow-redundant
  suppress-dof
  all-dims-reference
  scale <real>              # left out when 1, or 0, which is no scale; "scale = <expression>" for an expression
  link <string>             # linked file, relative to this file, with / separators
  link-as-solid             # a linked STEP file read as a solid, not a reference mesh
  remap { ... }
}
```

`predef` holds the values a group was created with:

```
predef {
  q <w> <x> <y> <z>         # a quaternion; left out when all four are 0
  origin <handle>
  at <x> <y> <z>            # a sketch on a face: its origin as last found; left out when 0
  entity-b <handle>
  entity-c <handle>
  swap-uv
  negate-u
  negate-v
}
```

`remap` holds the map from entities of earlier groups to the entities a group makes of them,
one line per entry, in order of the new index:

```
remap {
  map <new index> <input entity handle> <copy number>
}
```

| Group type | Name |
|---|---|
| 3D sketch | `sketch-3d` |
| Sketch in a workplane | `sketch-in-plane` |
| Extrude | `extrude` |
| Lathe | `lathe` |
| Revolve | `revolve` |
| Helix | `helix` |
| Rotate, step and repeat | `rotate` |
| Translate, step and repeat | `translate` |
| Link | `link` |
| Mirror | `mirror` |
| Chamfer | `chamfer` |
| Sweep along a path | `sweep` |
| Fillet | `fillet` |
| Shell | `shell` |
| Loft | `loft` |
| Pattern along a path | `pattern-path` |

A mirror reflects its `op-a` in the plane `entity-b` of its `predef`, a workplane or a planar
face; `skip-first` leaves the original out of its solid, and `mirror-solid` has it reflect the
whole solid so far, up to its `op-a`, in place of `op-a`'s own. A chamfer cuts its `edges` at its
`distance` from the solid so far, so its `combine` is always `difference`; and its
`solid-edge`s, edges of the solid so far named by the identities of the two faces they lie
between, lower first, their place among the edges between those faces in geometric order, and
how many edges those faces met along when it was named (`pkg/topo`, `Name`): where they now meet
along another number, the name does not resolve. A `solid-edge` without `of <m>` goes by its
place alone. A face's identity is the handle of the entity that made it, or, for a
surface no entity names, bit 30 set with the handle of the group that made it and the surface's
place among that group's surfaces. A sweep moves the
loops of its `op-a`, a sketch in a workplane, along its `edges`, line segments and arcs that
meet end to end and tangent.

A sketch in a workplane of subtype `face` lies in the flat face of the solid so far named by its
one `solid-face`, by identity as a shell's faces are, and is placed in it again at every
regeneration; its `predef`'s `q` and `at` are the orientation and origin it was last found at,
which hold the workplane while the face can't be found, as on loading, before the solids are made.

| Subtype | Name |
|---|---|
| Workplane by a point, orthogonal | `point-ortho` |
| Workplane by two line segments | `line-segments` |
| Workplane by a point and a normal | `point-normal` |
| Workplane in a face of the solid | `face` |
| One sided | `one-sided` |
| Two sided | `two-sided` |
| One sided, skewed | `one-skewed` |
| Two sided, skewed | `two-skewed` |

| Combine | Name |
|---|---|
| Union, the default | `union` |
| Difference | `difference` |
| Assemble | `assemble` |
| Intersection | `intersection` |

### Request

```
request <handle> <type> {
  group <handle>
  workplane <handle>        # left out for a request free in 3D (0xffffffff)
  construction
  style <handle>
  extra-points <integer>
  text <string>
  font <string>
  file <string>             # an image, relative to this file
  aspect-ratio <real>
}
```

| Request type | Name |
|---|---|
| Workplane | `workplane` |
| Datum point | `datum-point` |
| Line segment | `line-segment` |
| Cubic spline | `cubic` |
| Periodic cubic spline | `cubic-periodic` |
| Circle | `circle` |
| Arc of a circle | `arc-of-circle` |
| TrueType text | `ttf-text` |
| Image | `image` |

### Constraint

```
constraint <handle> <type> {
  group <handle>
  workplane <handle>
  value = <expression>      # a length in the document's unit, degrees for an angle, or a ratio
  name <string>             # what expressions read the value by, as a parameter's name
  param <handle>            # the solver parameter of a constraint that has one
  pt-a <handle>
  pt-b <handle>
  entity-a <handle>
  entity-b <handle>
  entity-c <handle>
  entity-d <handle>
  other
  other2
  reference
  comment <string>
  label-offset <x> <y> <z>  # left out when all three are 0
  label-style <handle>
}
```

The constraint type names are those the text window shows:

`pts-coincident`, `pt-pt-distance`, `pt-plane-distance`, `pt-line-distance`,
`pt-face-distance`, `proj-pt-pt-distance`, `pt-in-plane`, `pt-on-line`, `pt-on-face`,
`eq-length`, `length-ratio`, `eq-length-and-pt-ln-dist`, `eq-pt-line-distances`, `eq-angle`,
`eq-line-len-arc-len`, `length-difference`, `symmetric`, `symmetric-h`, `symmetric-v`,
`symmetric-line`, `at-midpoint`, `horizontal`, `vertical`, `diameter`, `pt-on-circle`,
`same-orientation`, `angle`, `parallel`, `perpendicular`, `arc-line-tangent`,
`cubic-line-tangent`, `curve-curve-tangent`, `eq-radius`, `lock-where-dragged`,
`arc-arc-length-ratio`, `arc-line-length-ratio`, `arc-arc-len-difference`,
`arc-line-len-difference`, `comment`.

### Style

Fields that are left out are zero. Unlike `.slvs`, they do not take the default style's values.

```
style <handle> <name> {
  width <real>
  width-as <pixels|mm>
  text-height <real>
  text-height-as <pixels|mm>
  text-angle <real>
  text-origin <left|right|bot|top>...   # one or two of them; left out for none
  color <colour>
  fill-color <colour>
  filled
  visible
  exportable
  stipple <pattern>
  stipple-scale <real>
}
```

| Stipple | Name |
|---|---|
| Continuous, the default | `continuous` |
| Short dash | `short-dash` |
| Dash | `dash` |
| Long dash | `long-dash` |
| Dash dot | `dash-dot` |
| Dash dot dot | `dash-dot-dot` |
| Dot | `dot` |
| Freehand | `freehand` |
| Zigzag | `zigzag` |

### Value

`value <handle> <real>`. The parameter is known or free by its handle, as in `.slvs`: a
handle with bit 30 set is free.

## Generated data

A file that is linked into another needs its entities, mesh and shell, since the linking file
does not regenerate it. The writer therefore saves them. A reader that opens the file as a
document ignores them and regenerates them from the hand-made records. This may change in a
later version, behind the version number.

### Entity

```
entity <handle> <type> {
  group <handle>
  workplane <handle>        # 0xffffffff for free in 3D; left out when 0
  construction
  style <handle>
  text <string>
  font <string>
  file <string>
  points <handle>...        # trailing empty points left out; 0x0 holds a place
  extra-points <integer>
  normal <handle>
  distance <handle>
  act-point <x> <y> <z>     # left out when all are 0
  act-normal <w> <x> <y> <z>
  act-distance <real>
  act-visible
}
```

| Entity type | Name |
|---|---|
| Point in 3D | `point-in-3d` |
| Point in a workplane | `point-in-2d` |
| Translated point | `point-n-trans` |
| Rotated and translated point | `point-n-rot-trans` |
| Copied point | `point-n-copy` |
| Point rotated about an axis | `point-n-rot-aa` |
| Point rotated about an axis and translated | `point-n-rot-axis-trans` |
| Normal in 3D | `normal-in-3d` |
| Normal in a workplane | `normal-in-2d` |
| Copied normal | `normal-n-copy` |
| Rotated normal | `normal-n-rot` |
| Normal rotated about an axis | `normal-n-rot-aa` |
| Distance | `distance` |
| Copied distance | `distance-n-copy` |
| Face by normal and point | `face-normal-pt` |
| Face by cross product | `face-xprod` |
| Rotated and translated face | `face-n-rot-trans` |
| Translated face | `face-n-trans` |
| Face rotated about an axis | `face-n-rot-aa` |
| Rotated face by normal and point | `face-rot-normal-pt` |
| Face rotated about an axis and translated | `face-n-rot-axis-trans` |
| Workplane | `workplane` |
| Line segment | `line-segment` |
| Cubic spline | `cubic` |
| Periodic cubic spline | `cubic-periodic` |
| Circle | `circle` |
| Arc of a circle | `arc-of-circle` |
| TrueType text | `ttf-text` |
| Image | `image` |
| Extruded surface | `surface-extruded` |
| Lathed surface | `surface-lathed` |
| Revolved surface | `surface-revolved` |

### Mesh

```
mesh {
  triangle <face> <colour> <ax> <ay> <az> <bx> <by> <bz> <cx> <cy> <cz>
}
```

`<face>` is the face identifier as a handle, `0x0` when the triangle has none.

### Shell

```
shell {
  surface <handle> {
    color <colour>
    face <handle>
    sheet <handle>          # the curve entity the patch was swept from; patches of one sheet meeting smoothly are one face
    degree <m> <n>
    ctrl <i> <j> <x> <y> <z> <weight>
    trim <curve handle> <forward|backward> <sx> <sy> <sz> <fx> <fy> <fz>
  }
  curve <handle> {
    exact                   # the curve is the rational Bezier of its ctrl lines
    degree <n>
    surfaces <handle> <handle>
    ctrl <i> <x> <y> <z> <weight>
    point <x> <y> <z>
    vertex <x> <y> <z>
  }
}
```

`point` and `vertex` are the curve's piecewise-linear points in order; a `vertex` is one that
was a vertex of the original curve. Degrees and control point indices run from 0 to 5
(`srf.MaxDegree`); readers before step 11 took them up to 3 and refuse a higher one.

## Versions

The header names the major version. A reader refuses a file with a newer major version, with
a message that names both versions. The writing program's version is not recorded, so a save
does not change when the program is upgraded.

Within a major version a newer writer may add statements and fields. A reader reports each
statement, field or enumeration name it does not know, with its line and column, skips it,
and loads the rest. The interface shows those reports, as it shows unrecognised lines of a
`.slvs` file.

Version 2 (2026-10-02) made `units` the document's unit: values with `=` that are lengths are
in it. Version 1 stored them in millimetres whatever `units` said. A version 2 reader reads a
version 1 file's plain lengths as millimetres, rewrites a length expression to give the same
length in the file's unit (`wall*4` in an inch file becomes `(wall*4)/25.4`), and writes the
file back as version 2.

Version 2 gained the group field `suppress-all` on 2026-10-02. A reader from before skips it
with a report, and so generates the group.

Version 2 gained the group types `mirror`, `chamfer` and `sweep`, with the fields `edges` and
`distance`, on 2026-10-03. A reader from before reports the unknown type.

Version 2 gained the chamfer field `solid-edge` on 2026-10-03. A reader from before skips it
with a report.

Version 2 gained the group type `pattern-path` on 2026-10-03 (roadmap 15.6): it repeats the
solid of its `op-a` `copies` times along its `edges`, a path as a sweep's. A reader from before
reports the unknown type.

Version 2 gained the mirror field `mirror-solid` on 2026-10-03 (roadmap 15.5). A reader from
before skips it with a report, and mirrors the source group alone.

Version 2's `solid-edge` gained `of <m>`, the count of edges between its faces, on 2026-10-03
(roadmap 14.2). A reader from before refuses such a line; a reader since reads lines without
it as before.

Version 2's drawing `dimension` gained `solid-face <handle>` lines, and views named by a
section's or detail's letter or `iso`, on 2026-10-04 (roadmap 29.2). A reader from before
refuses such a dimension.

Version 2 gained the chamfer field `second-distance` on 2026-10-04 (roadmap 28.1). A reader
from before skips it with a report, and cuts at `distance` on both faces.

Version 2 gained the extrusion fields `draft-faces` and `draft-neutral` on 2026-10-04 (roadmap
28.2). A reader from before skips them with a report, and drafts every side from the sketch's
plane.

Version 2 gained the loft fields `start-tangency` and `end-tangency` on 2026-10-04 (roadmap
28.3), plain numbers. A reader from before skips them with a report, and leaves the ends free.

A loft's `edges` are its guide curves since 2026-10-04 (roadmap 28.3): line segments, arcs and
cubics drawn before it, each chain of them that joins end to end one guide from the first
profile through every profile to the last, two at most. A reader from before reads the field
and ignores it, so the loft is made without its guides.

Version 2 gained the workplane subtype `face`, with the `predef` field `at`, on 2026-10-10
(roadmap F1). A reader from before refuses the unknown subtype.

`testdata/tlspr/v1` and `testdata/tlspr/v2` hold files that every later reader must load.

## Links

A link group's `link` names a `.tlspr`, `.emn` or `.stl` file relative to the file that holds
the group. A link to a `.slvs` file is not followed: importing a SolveSpace assembly converts
every linked `.slvs` file to a `.tlspr` file beside it and rewrites the links
(`pkg/persist/convert`, File > Import, `telesapr-cli convert`, `api.Import`).

## Autosave

The autosave of `name.tlspr` is `name.tlspr~`, in the same format.

## Packed models

A packed model, `.tlsprz`, is a zip: the model as `model.tlspr`; the files it links, and those they
link, in `parts/`, their links rewritten to name each other there (`parts/name` from the model,
`name` from a part); its pictures in `images/`; and in `cache/step/` the tessellations of its STEP
files, named by a hash of the file's contents, the chord tolerance and a version, which a reader
may take into its cache. A name that repeats gets `-2`, `-3` before its extension. The entries'
dates are fixed, so the same model packs to the same bytes. A reader refuses an entry that would
be written outside the folder it unpacks into.

