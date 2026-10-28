# Writes testdata/topo/fillet-runoff-curved.txt, the volumes OpenCASCADE
# gives for fillets whose end runs off onto a curved face, which
# TestFilletRunsOffOntoCylinder in pkg/script/api checks its own against.
# Run with FreeCAD's command line:
#
#     FreeCADCmd scripts/fillet-runoff-fixtures.py
#
# The solids are those of curvedBandOnCone: a cone lathed about z from
# radius 10 at z = 0 to 4 at z = 30, and a band from x = 6 to 20, y from
# -y to y, whose top is a cylinder rising sag above the band's edges at
# height top (a hollow where sag is below 0): about an axis along y
# (kind "y"), through (6, top), (20, top) and (13, top + sag), or about
# an axis along x (kind "x"), through (-y, top), (y, top) and
# (0, top + sag), which meets the band's end at x = 6 in an arc. The
# band's edge on the cone, at y, is filleted with radius r: the fillet
# runs off the band's side past its top, onto that cylinder, or, on the
# narrowest band, ends on that cylinder with the cone going on past it.
# Each line: kind, y, top, sag, r, the volume before and after; a line
# with "refused" where OpenCASCADE made none or made one that isn't
# valid.
import math
import os

import Part
from FreeCAD import Vector as V

OUT = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "testdata", "topo", "fillet-runoff-curved.txt")


def solid(kind, y, top, sag):
    w = Part.makePolygon([V(0, 0, 0), V(10, 0, 0), V(4, 0, 30), V(0, 0, 30), V(0, 0, 0)])
    cone = Part.Face(w).revolve(V(0, 0, 0), V(0, 0, 1), 360)
    if kind == "y":
        a = Part.LineSegment(V(6, -y, top), V(6, -y, 0)).toShape()
        b = Part.LineSegment(V(6, -y, 0), V(20, -y, 0)).toShape()
        c = Part.LineSegment(V(20, -y, 0), V(20, -y, top)).toShape()
        arc = Part.Arc(V(20, -y, top), V(13, -y, top + sag), V(6, -y, top)).toShape()
        band = Part.Face(Part.Wire([a, b, c, arc])).extrude(V(0, 2 * y, 0))
    else:
        a = Part.LineSegment(V(6, -y, top), V(6, -y, 0)).toShape()
        b = Part.LineSegment(V(6, -y, 0), V(6, y, 0)).toShape()
        c = Part.LineSegment(V(6, y, 0), V(6, y, top)).toShape()
        arc = Part.Arc(V(6, y, top), V(6, 0, top + sag), V(6, -y, top)).toShape()
        band = Part.Face(Part.Wire([a, b, c, arc])).extrude(V(14, 0, 0))
    return cone.fuse(band).removeSplitter()


def edge(s, y):
    p = V(math.sqrt(64 - y * y), y, 10)
    return min(s.Edges, key=lambda e: e.distToShape(Part.Vertex(p))[0])


lines = []
grid = [("y", 2, 18), ("y", 2.5, 22), ("y", 3, 22), ("x", 3, 20), ("x", 3, 21)]
for kind, y, top in grid:
    for sag in (-1, 1, 2):
        if kind == "x" and (sag == 2 or top == 21 and sag == -1):
            # The x band's top at 20 above its middle touches the cone
            # where it meets the band's end: no fixture.
            continue
        s = solid(kind, y, top, sag)
        for r in (1.5, 2, 2.5):
            try:
                f = s.makeFillet(r, [edge(s, y)])
                if not f.isValid():
                    raise ValueError("not valid")
                lines.append("%s %g %g %g %g %.9f %.9f" % (kind, y, top, sag, r, s.Volume, f.Volume))
            except Exception:
                lines.append("%s %g %g %g %g refused" % (kind, y, top, sag, r))
with open(OUT, "w") as f:
    f.write("\n".join(lines) + "\n")
