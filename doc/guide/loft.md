# Loft

Examples: [`examples/scripts/loft`](../../examples/scripts/loft/loft.go), a round-to-square
duct adapter; [`examples/scripts/vase`](../../examples/scripts/vase/vase.go), a vase lofted
smoothly through four profiles; [`examples/scripts/ring`](../../examples/scripts/ring/ring.go),
a ring closed on itself.

A loft joins the closed loop of one sketch to the closed loops of other sketches, and makes a
solid between them. Each sketch is a *profile*, drawn in a workplane.

## In the window

1. Activate one of the profile sketches.
2. Select any entity of each other profile.
3. Choose Features > Loft To Selected Profile (Shift+O).

With one other sketch picked, the two loops are joined by ruled faces, curve by curve. With two
or more, one smooth surface goes through all the loops. The profiles are taken in the order of
the history, starting from the active sketch, which must be the first or the last of them. The
window shows only the groups up to the active one, so activate the last profile and pick the
earlier ones.

The loops need not have the same number of curves. A circle counts as four arcs. A loop with
fewer curves has its longest curve split in two until the counts match, so a triangle lofts to
a square.

The group's screen shows:

- the profiles, with "[change]" to loft through the sketches of the selected entities instead,
  in the order of the history;
- "turn the second by [-1] [+1]" (for a smooth loft, "turn those after the first by"). This
  shifts which curve of one loop is joined to which curve of the next. A square turned a
  quarter has saddle faces. A smooth loft first turns each profile to start at the curve
  nearest where the one before it starts, and the turn twists from there. A large twist can
  fold the surface through itself, which Analyze > Check Solid reports.

## From a script

```go
l := doc.Features.Loft(bottom, top, "duct")
l.AsLoft().SetTurn(1)

v := doc.Features.LoftThrough([]Group{foot, belly, neck, lip}, "vase")
v.AsLoft().Profiles() // foot, belly, neck, lip
```

## How the smooth surface is made

Each loop becomes one B-spline curve. Its pieces are raised to one degree, with a knot at each
joint. The curves are then interpolated across the profiles by a cubic B-spline (a quadratic
one through three profiles), spaced by the distances between the profiles. Arcs stay exact, so
circles about one axis loft to a true surface of revolution. The surface passes through each
profile exactly. Where a profile has a corner, the surface has a crease there. The crease fades
out toward smooth profiles, and the window draws it as an edge.

## Square to the ends

The loft's screen says how the loft *leaves the first* profile and *reaches the last*:

- **free**: the surface is shaped by the profiles alone;
- **square to it, by a number**: the surface leaves the profile along the profile's normal. The
  number is how far that direction is held: the slope across the end is that many times the
  loft's length. 1 is a natural curve. More holds the normal longer.

A loft of two profiles with an end set is one smooth surface, not ruled patches. Example:
[`examples/scripts/nozzle`](../../examples/scripts/nozzle/nozzle.go), a square duct turning
into a round one, square to both ends.

```go
loft.AsLoft().SetTangency(1, 1) // square to both ends
loft.AsLoft().SetTangency(0, 0) // free again
```

## Along a solid's sides

A profile may be drawn on a face of the solid made before the loft, along that face's edges.
The loft can then leave along the solid's sides instead of square to the profile: click
"[along sides]" beside that end on the loft's screen ("[square]" puts it back).

At each point of the profile, the loft leaves in the tangent plane of the side face that meets
the edge there. So the side carries on into the loft without a crease. At a corner between two
sides, the loft leaves along the line where they meet. Where the sides are square to the face,
this is the same as square to the profile. Where they lean, as a drafted extrusion's or a cone's
do, the loft follows the lean. The number means the same as for square ends, and 1 where it is 0.

A profile that is not on the edges of a side below its face is a problem on the group. A loft
held by guides follows the sides too, and still holds its guides. Example:
[`examples/scripts/spire`](../../examples/scripts/spire/spire.go), a spire on a tapered post.

```go
loft.AsLoft().SetAlongSides(true, false) // leave the first profile along the sides
```

## Guide curves

A loft can be held to guide curves. A guide is a chain of line segments, arcs and cubics drawn
in a sketch before the loft, joined end to end. It runs from a point of the first profile
through every profile to the last. On a closed loft it goes round the ring (see below).

Activate the loft, select the curves and choose Features > Guide Loft by Selected Curves. The
loft's screen says how many curves guide it, and "[none]" takes them away.

- One guide moves the loft's sections across, each within its own plane. The loft then runs
  along the guide where it would have run straight.
- Two guides move, turn and scale the sections. The loft widens and narrows between them.

The surface holds the guides at a dozen sections between each two profiles, and closely between
those, and still goes through every profile. A guide that misses a profile, or does not start
and end on the end profiles, is a problem on the group, and the message says how far it is off.
Example: [`examples/scripts/horn`](../../examples/scripts/horn/horn.go), a horn flared by two
guides.

```go
loft.AsLoft().SetGuides([]api.Entity{right, left}) // two guides
loft.AsLoft().SetGuides(nil)                       // none again
```

## Square to a profile between the ends

A smooth loft can also be held square to a profile between its ends. Each such profile's row on
the loft's screen says how the loft *passed* it: free, or square to it by a number. "[change]"
sets the number, and 0 frees it again.

The number works as at the ends: the slope across the profile is that many times the loft's
length, so the loft runs straight through the profile for a stretch. Free, it would bend
through the profile as the profiles about it shape it. The loft is then one surface from each
held profile to the next, meeting there with one slope, and still one face.

A guided loft can hold profiles between its ends too. The guides move the sections, and the loft
still leaves each held profile square to it. Where a guide crosses the profile at a slant, the
loft turns onto the guide within a twelfth of the way to the next section. Example:
[`examples/scripts/waist`](../../examples/scripts/waist/waist.go), a column running straight up
through its waist.

```go
loft.AsLoft().SetProfileTangency(waist, 0.5) // square to the waist
loft.AsLoft().SetProfileTangency(waist, 0)   // free again
```

## Closed into a ring

A smooth loft of three profiles or more can go on from its last profile back to its first. Click
"[closed]" on the last row of its screen ("[open]" undoes it). The ring has no caps, and it is
as smooth where it closes as anywhere else.

- A ring has no ends, so it cannot carry a solid's sides on, and it cannot turn its profiles.
  Either setting is a problem on the group.
- Any of its profiles can be held square, as an open loft's middle ones are. The ring is then
  one surface from each held profile round to the next, and still one face. The first and last
  profiles are held by the rows that set an open loft's ends (`SetTangency` in a script, or
  `SetProfileTangency` with that profile). The ring then meets itself tangent there.
- Guides hold a ring as they hold an open loft. Each guide goes round the ring through every
  profile and closes on itself. A guide with ends is a problem on the group.
- A ring can be held by guides and square to its profiles at once.
- Between profiles, the sections turn with the ring, as a closed sweep's do, so they keep the
  profiles' size. Three circles round a circle make a torus within a fifth of a percent.
- Profiles more than a third of a turn apart round the ring are refused. Draw more profiles.

Examples: [`examples/scripts/ring`](../../examples/scripts/ring/ring.go), a ring that swells
and narrows, running straight through its narrow profiles;
[`examples/scripts/wavering`](../../examples/scripts/wavering/wavering.go), a ring held by a
guide waving up and down over it;
[`examples/scripts/billow`](../../examples/scripts/billow/billow.go), a ring held by a guide
swelling up between its profiles and square to every other one.

```go
ring := doc.Features.LoftThrough(profiles, "ring")
ring.AsLoft().SetClosed(true)
ring.AsLoft().SetGuides(guide) // cubics round the ring, end to end
```

## Not yet

Three guides or more are refused: a loft takes two at most.
