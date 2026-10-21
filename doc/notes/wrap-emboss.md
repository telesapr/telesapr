# Wrap and emboss a sketch onto a cylinder or a cone (WP-C2)

Item 1 of `modelling_upgrades.md`. Text, bands and patterns raised from or sunk into a turned
part's round face. This note is the design. The state of the work is at the end.

## The group

`GroupWrap` is a solid group like an extrusion. It takes its profile from a sketch in a workplane
(`OpA`, closed loops, text included) and lays the profile onto one face of the solid so far
(`SolidFaces[0]`, by identity, as a shell takes faces). The face is a cylinder or a cone.
`ValB` is the height, which takes an expression as a shell's thickness does (`GroupPitch`). The
combine mode says which way the profile goes:

- **Union (emboss)** raises the profile `ValB` above the face.
- **Difference (engrave)** sinks it `ValB` into the face.

A third mode is the "Difference" of a sketch-shaped band, so neither a new flag nor a new
subtype is needed. The group makes no entities. Like a shell's, its faces are named from the
group (`srf.OriginFor`).

The user draws the sketch in any workplane **parallel to the face's axis**, as for a plane
through the axis, or a plane in front of the part. They select the face and choose
New Group > Wrap Onto Face. A script calls `Features.Wrap(face SolidFace, height float64, name)`
with the sketch active.

## Where the sketch lands

Let `A`, `d` be the axis, and `n` the unit direction, square to `d`, from the axis towards the
sketch's plane (the plane's normal when the plane holds the axis). Let `t = d × n`.

**Cylinder of radius r.** A point of the sketch has coordinates `s = (P-A)·t` across and
`z = (P-A)·d` along. It goes to

    A + z d + r (cos(s/r) n + sin(s/r) t)

which is paper rolled round the cylinder: lengths in the sketch are lengths on the face. Near
the line of the face facing the sketch, the map is the identity. So seen from the sketch's side,
the wrapped profile looks as it was drawn, never mirrored.

**Cone of apex V, half angle α, opening along d.** The sketch is turned through α about the
line where it crosses the facing generator at the height of its middle, `z_c`. That lays it on
the cone's tangent plane along that generator, the sketch's "along the axis" now running along
the generator. The tangent plane is then rolled onto the cone by the cone's development: a point
at distance ρ from the apex and angle ψ in the plane goes to slant ρ and angle θ = ψ / sin α on
the cone. Lengths are kept again, and the map is the identity near the facing generator.

## Exact where it can be, fitted elsewhere

On a cylinder, some sketch curves map exactly:

- A line across the axis (z constant) maps to a circular arc, a rational quadratic.
- A line along the axis maps to a line.

On a cone, a line through the developed apex maps to a generator. Every other curve is fitted:
quintic Béziers (`srf.MaxDegree`) by least squares at fixed ends, split in half until the
fit is within `wrapFitTol` = 1e-7 mm of the true curve at every sample. That is a tenth of the
kernel's `LengthEps`, so a fitted curve lies on the face as an exact one does. The pieces of one
sketch curve keep its `Sheet`, so they make one face.

## The solid

The band is the image of an extrusion of the profile along the face's normal. It is made as
`MakeFromExtrusionOf` makes one, with every piece mapped:

- **Caps.** Two caps are the face's surface lifted to heights `h0` and `h1`. Each is one
  revolved patch (`FromRevolutionOf`) spanning the loop set's angles and heights with a small
  margin, and trimmed by the mapped loops.
- **Lift.** On a cylinder lifting is affine, a scaling away from the axis by `(r+h)/r`, so the
  lifted curves and caps are exact images of the curves at the face. On a cone the caps are
  the face scaled about the point `Q` of the axis on the normal through the sketch's middle,
  which is the parallel cone `h` away, but the curves are lifted along the face's normal and
  fitted, the two edges of a side halved together (`fitCurves`). Scaled curves leaned off the
  normal and their Booleans failed more often. A cone's caps reach along the generator by the
  lift beyond the profile, since the normal lift moves the curves along it.
- **Sides.** The sides are ruled patches between a curve at `h0` and the same curve at `h1`,
  square to the face on both kinds. A line wrapped to a line is raised to degree 2, so that its
  side is no flat patch. The Boolean takes a flat patch against a cylinder by the plane's
  case, which splits the line a few micrometres off and leaves a trim open; that failed 16 of
  120 letters.

The map is the identity to first order at the facing line and keeps orientation. The
extrusion's rules for which cap faces which way, and which way the trims run, therefore carry
over unchanged. Heights are measured along the face's outward normal, so a bore's inner face
raises towards the axis.

The band overlaps the face on the other side by a part of the height, `h0 = -kH` for an
emboss and `h1 = +kH` for an engraving. That way the Boolean crosses the face rather than
touching it: a touching band fails every time. The overlap is hidden in the result. The group
does its own Boolean (`generateWrapShell`):

1. It joins the whole band at k = 1/4.
2. Where that fails, it joins loop set by loop set, each trying k = 1/4, 1/20, 2/5, 3/20 and 7/10.
3. Where a loop set joins at none, it refuses. The group gets a problem and the solid so far
   passes through, so the user never gets an unsound solid.

A wall thinner than kH under an emboss would be crossed. The diagnostic does not look for
this.

**One patch per loop set.** A cap is one rational quadratic patch, so one loop set (an outer
loop and its holes, such as one glyph) may span at most 170° round the axis. A longer one is
refused with a diagnostic that says to split it. A string of text spans any angle, since every
glyph is its own loop set.

## Refusals, each a diagnostic on the group

- The face is not in the solid so far, or is neither a cylinder nor a cone.
- The sketch is not closed loops, or its plane is not parallel to the axis.
- The height is not more than zero, or an engraving reaches the axis (H ≥ r; on a cone,
  H ≥ the distance from the face to Q).
- A loop set spans more than 170°.
- On a cone, the sketch reaches past the apex.
- A curve can't be fitted within the tolerance in 2⁸ pieces.

## Checks

- The solid's oracle `pkg/check`: watertight, no naked or self-crossing edges.
- Volume in closed form for a rectangle wrapped on a cylinder: `Δθ/2 ((r+H)² - r²) L` raised,
  the same with `r-H` sunk.
- A rectangle on a cone: the band's volume from Pappus over the development.
- The stress case is text, a sweep over sizes and radii. The rate of sound results is
  reported as it is.

## Trap

`pkg/sketch` round trips at a fine chord tolerance (`Edit.SetChordTolerance(0.005)`) change
the file when a later group grows the model. Two plain extrusions show it too: the
tolerance is stepped by the model's size. The wrap's round-trip test runs at the default.

## State (2026-10-22, branch `wp-c2-wrap`)

Cylinder and cone, end to end.

- **Kernel.** `srf.RoundFaceOf`, `srf.NewWrap`, `Wrap.MapCurve` and `SShell.MakeFromWrapOf`.
  The kernel also drops trim edges of no length before a Boolean assembles a trim
  (`boolean_trim.go`), which fixed 4 cone failures and moved no golden.
- **Model.** `GroupWrap`: `Sketch.NewWrap`, saved as `wrap` with `height`, and
  `solid-face`.
- **Window.** Features > Wrap Sketch Onto Face, the face's context menu, and the group screen
  (face, height, raised or sunk).
- **Scripts.** `Features.Emboss`, `Features.Engrave` and `Group.AsWrap` (`Height`,
  `SetHeight`, `Engraved`).

Rates:

- **Kernel volumes against closed forms.** The cylinder band is `A((r+h1)²-(r+h0)²)/2r`,
  held to 1e-3 at a fine mesh. On a cone, `A h + h²/2 ∫ dA/R` converges as the mesh does
  (7e-3 at a chord of 1e-4, 3e-4 at 3e-5).
- **Text sweep (`TestWrapText*Sweep`).** Three strings, radii 10, 20 and 60, sizes 3 and 6,
  depth 0.1 or 0.5 of the letter height, raised or sunk:
  - Cylinders: 0 of 72 fail.
  - Cones: 14 of 72 refused. All are at depth 0.5 (14 of 36); none are at depth 0.1.

Left to do:

- The cone failures. The face's trim doesn't assemble where marched curves end 1e-7 apart in
  uv at a corner of the profile, near the cone's patch seams, or where a closed curve's two
  ends miss.
- A text window golden UI scenario (`ext_wrap`), a path-equivalence pair (`path_wrap`), and
  an example (`examples/scripts/wrap`).
- `.tlspr` fixtures under `testdata/solid/`.
- The done-audit row and the format doc entry.
