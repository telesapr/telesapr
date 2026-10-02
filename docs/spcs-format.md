# The `.spcs` file format, version 1

`.spcs` is the file format spacesolve opens and saves. It replaces SolveSpace's `.slvs`, which
spacesolve only imports (File, Import) and never writes. The format is text, line oriented, and
written in a stable order, so two saves of the same model give the same bytes and a change to the
model gives a small diff.

The reader and writer are `pkg/persist/spcs`. The syntax layer (`lex.go`, `parse.go`, `print.go`)
knows nothing about sketches. The model layer (`encode.go`, `decode.go`) maps the syntax tree to
`sketch.Sketch`.

## Example

Two datum points ten millimetres apart, as spacesolve saves them
(`testdata/spcs/v1/sketch.spcs`), shortened where `...` stands:

```
spacesolve 1

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
header     = "spacesolve" , integer , eol ;
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

A field written with `=` holds an expression, for any value a user may type as a formula. It is
either a plain number, such as `value = 30`, or the text of an expression that names parameters,
such as `value = wall*4`. A reader keeps the whole text and evaluates it at each regeneration.

Expressions are in millimetres and degrees, whatever unit the interface shows: a parameter's
value is a number of millimetres when it is used as a length, and trigonometric functions take
and give degrees. The `units` line records the unit the document was authored in, for display
only.

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
| `spacesolve <major>` | The header. |
| `units <unit>` | `mm`, `inch`, `m` or `ft-in`. Display only. |
| `param <name> = <expression>` | A named parameter, see "Parameters". |
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
  copies = <expression>     # step and repeat: the number of copies
  pitch = <expression>      # helix: the fixed pitch in mm, 0 when free
  val-c = <expression>      # spare value, unused by every group today
  color <colour>
  skip-first
  combine <combine>
  force-to-mesh
  predef { ... }
  visible
  suppress
  relax-constraints
  allow-redundant
  suppress-dof
  all-dims-reference
  scale <real>              # left out when 1, or 0, which is no scale; "scale = <expression>" for an expression
  link <string>             # linked file, relative to this file, with / separators
  remap { ... }
}
```

`predef` holds the values a group was created with:

```
predef {
  q <w> <x> <y> <z>         # a quaternion; left out when all four are 0
  origin <handle>
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

| Subtype | Name |
|---|---|
| Workplane by a point, orthogonal | `point-ortho` |
| Workplane by two line segments | `line-segments` |
| Workplane by a point and a normal | `point-normal` |
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
  value = <expression>      # mm for lengths, degrees for angles, a ratio for ratios
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
was a vertex of the original curve.

## Versions

The header names the major version. A reader refuses a file with a newer major version, with
a message that names both versions. The writing program's version is not recorded, so a save
does not change when the program is upgraded.

Within a major version a newer writer may add statements and fields. A reader reports each
statement, field or enumeration name it does not know, with its line and column, skips it,
and loads the rest. The interface shows those reports, as it shows unrecognised lines of a
`.slvs` file.

`testdata/spcs/v1` holds files that every later reader must load.

## Links

A link group's `link` names a `.spcs`, `.emn` or `.stl` file relative to the file that holds
the group. A link to a `.slvs` file is not followed: importing a SolveSpace assembly converts
every linked `.slvs` file to a `.spcs` file beside it and rewrites the links
(`pkg/persist/convert`, File > Import, `spacesolve-cli convert`, `api.Import`).

## Autosave

The autosave of `name.spcs` is `name.spcs~`, in the same format.
