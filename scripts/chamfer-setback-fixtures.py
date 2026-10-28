# Writes testdata/topo/chamfer-corners.txt, the volumes and face counts
# OpenCASCADE gives for chamfers that meet at a corner, which
# TestChamferCornerOracle in pkg/topo checks its own against. Run with
# FreeCAD's command line:
#
#     FreeCADCmd scripts/chamfer-setback-fixtures.py
#
# Each line: a shape, its distances, OpenCASCADE's volume, its count of
# faces, and after a bar the corners of the face that fills the
# chamfered corner. The shapes:
#
#   box dz dx dy: the 10 x 20 x 30 box's corner at (10, 20, 30), its
#       edges along z, x and y chamfered at dz, dx and dy.
#   uneven dz1 dz2 dx dy: the same, the edge along z chamfered at dz1 on
#       the face square to x and dz2 on the face square to y.
#   apex d1 d2 d3 d4: the four edges at the apex (10, 10, 10) of the
#       pyramid on the square (0, 0) to (20, 20), from the apex to the
#       base's corners (0, 0), (20, 0), (20, 20) and (0, 20).
#   prism deg d1 d2 d3: the corner at the origin of the prism of height
#       20 on the triangle (0, 0), (20, 0), 20 along deg degrees from the
#       x axis: its edges along x, along deg and up.
#   notch d1 d2 d3: the concave corner (10, 10, 10) of the 20 cube with
#       the cube of 10 at its far corner taken away, its edges along x,
#       y and z, chamfered so that they fill.
import math
import os

import FreeCAD
import Part
from FreeCAD import Base

out = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "testdata", "topo", "chamfer-corners.txt")
V = Base.Vector
doc = FreeCAD.newDocument("fixtures")


def index(shape, a, b):
    for i, e in enumerate(shape.Edges):
        p, q = e.Vertexes[0].Point, e.Vertexes[-1].Point
        if ((p - a).Length < 1e-6 and (q - b).Length < 1e-6) or ((p - b).Length < 1e-6 and (q - a).Length < 1e-6):
            return i + 1
    raise ValueError("no edge from %s to %s" % (a, b))


def chamfer(shape, specs):
    """specs: (a, b, d1, d2) for each edge from a to b."""
    base = doc.addObject("Part::Feature", "base")
    base.Shape = shape
    ch = doc.addObject("Part::Chamfer", "chamfer")
    ch.Base = base
    ch.Edges = [(index(shape, a, b), d1, d2) for (a, b, d1, d2) in specs]
    doc.recompute()
    s = ch.Shape
    if not s.isValid() or s.Volume <= 0:
        raise ValueError("not valid")
    return s


def has_vertex(s, x):
    """Whether the solid s has a vertex at x, within 1e-6."""
    return any((vx.Point - x).Length < 1e-6 for vx in s.Vertexes)


lines = []


def emit(head, s, corner, dmax):
    """A line for the chamfered solid s: its volume, its count of faces,
    and the corners of the faces that fill the chamfered corner, those
    whose every vertex lies within 3 dmax of it."""
    pts = []
    for f in s.Faces:
        vs = [vx.Point for vx in f.Vertexes]
        if all((p - corner).Length < 3 * dmax + 0.5 for p in vs):
            for p in vs:
                if not any((p - q).Length < 1e-6 for q in pts):
                    pts.append(p)
    pts.sort(key=lambda p: (round(p.x, 6), round(p.y, 6), round(p.z, 6)))
    lines.append("%s %.9f %d |%s" % (head, s.Volume, len(s.Faces), "".join(" %.6f,%.6f,%.6f" % (p.x, p.y, p.z) for p in pts)))


box = Part.makeBox(10, 20, 30)
C = V(10, 20, 30)
Z, X, Y = V(10, 20, 0), V(0, 20, 30), V(10, 0, 30)
for dz in (0.5, 1, 2, 3):
    for dx in (0.5, 1, 2, 3):
        for dy in (0.5, 1, 2, 3):
            try:
                emit("box %g %g %g" % (dz, dx, dy), chamfer(box, [(C, Z, dz, dz), (C, X, dx, dx), (C, Y, dy, dy)]), C, max(dz, dx, dy))
            except Exception as e:
                print("box", dz, dx, dy, "refused:", e)

# Which of the edge along z's two faces OpenCASCADE puts its first
# distance on: d1 on the one square to x leaves a vertex at (10, 20 - d1,
# 0).
for dz1, dz2 in ((1, 2), (2, 1), (0.5, 1.5)):
    for dx, dy in ((1, 1), (1, 2), (2, 1)):
        try:
            # Tell the faces apart by a chamfer of the edge alone.
            alone = chamfer(box, [(C, Z, dz1, dz2)])
            first_on_x = has_vertex(alone, V(10, 20 - dz1, 0))
            if first_on_x:
                s = chamfer(box, [(C, Z, dz1, dz2), (C, X, dx, dx), (C, Y, dy, dy)])
            else:
                s = chamfer(box, [(C, Z, dz2, dz1), (C, X, dx, dx), (C, Y, dy, dy)])
            emit("uneven %g %g %g %g" % (dz1, dz2, dx, dy), s, C, max(dz1, dz2, dx, dy))
        except Exception as e:
            print("uneven", dz1, dz2, dx, dy, "refused:", e)

apex = V(10, 10, 10)
pts = [V(0, 0, 0), V(20, 0, 0), V(20, 20, 0), V(0, 20, 0)]
down = list(reversed(pts))
faces = [Part.Face(Part.makePolygon(down + [down[0]]))]
for i in range(4):
    faces.append(Part.Face(Part.makePolygon([pts[i], pts[(i + 1) % 4], apex, pts[i]])))
pyramid = Part.Solid(Part.Shell(faces))
for ds in ((1, 1, 1, 1), (2, 2, 2, 2), (1, 1.5, 2, 1.5), (1, 2, 1, 2), (1, 1, 1, 2), (0.5, 1, 1.5, 2), (0.5, 0.5, 2, 0.5)):
    try:
        emit("apex %g %g %g %g" % ds, chamfer(pyramid, [(apex, pts[i], ds[i], ds[i]) for i in range(4)]), apex, max(ds))
    except Exception as e:
        print("apex", ds, "refused:", e)

O = V(0, 0, 0)
for deg in (60, 75, 120):
    th = math.radians(deg)
    tri = [O, V(20, 0, 0), V(20 * math.cos(th), 20 * math.sin(th), 0)]
    prism = Part.Face(Part.makePolygon(tri + [tri[0]])).extrude(V(0, 0, 20))
    for ds in ((1, 1, 1), (3, 2, 1), (1, 1, 2), (2, 1, 1), (1, 2, 1), (1, 2, 3), (2, 1, 3)):
        try:
            s = chamfer(prism, [(O, tri[1], ds[0], ds[0]), (O, tri[2], ds[1], ds[1]), (O, V(0, 0, 20), ds[2], ds[2])])
            emit("prism %g %g %g %g" % ((deg,) + ds), s, O, max(ds))
        except Exception as e:
            print("prism", deg, ds, "refused:", e)

notch = Part.makeBox(20, 20, 20).cut(Part.makeBox(10, 10, 10, V(10, 10, 10)))
N = V(10, 10, 10)
for ds in ((1, 1, 1), (1, 2, 3), (2, 1, 1)):
    try:
        s = chamfer(notch, [(N, V(20, 10, 10), ds[0], ds[0]), (N, V(10, 20, 10), ds[1], ds[1]), (N, V(10, 10, 20), ds[2], ds[2])])
        emit("notch %g %g %g" % ds, s, N, max(ds))
    except Exception as e:
        print("notch", ds, "refused:", e)

with open(out, "w") as f:
    f.write("\n".join(lines) + "\n")
print("wrote", len(lines), "lines to", out)
