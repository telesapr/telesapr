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
#   pair, pocket and rib tx sx ty sy: two edges chamfered and the third
#       left sharp, each at tx or ty on the face both share and sx or sy
#       on its other face. pair: the box's corner at (10, 20, 30), its
#       edges along x and y below the top. pocket: the notch's corner,
#       its edges along x and y over the floor at z 10, the one along z
#       sharp. rib: the plate (0, 0, 0) to (20, 20, 5) with the block
#       (0, 0, 5) to (10, 10, 15) on it, the foot edges from (10, 10, 5)
#       along x and y filled, the block's upright edge sharp.
#   ledge tx sx ty sy: the notch's top edges from (10, 10, 20) along x
#       and y chamfered, the concave upright edge below them sharp.
#   prismpair deg tx sx ty sy: the prism's corner at the origin, its
#       edges along x and along deg chamfered over the face both share,
#       the bottom, its upright edge sharp.
#   ledgeall dx dy dz: the notch's corner at (10, 10, 20), its convex
#       top edges along x and y and the concave upright edge below them
#       chamfered.
#   riball dx dy dz: the rib's corner at (10, 10, 5), its concave foot
#       edges towards x and y and the block's convex upright edge
#       chamfered.
#   ledgeone d: the notch's top edge from (10, 10, 20) along x alone,
#       ending at the corner inside the ledge.
#   apexsome d1 d2 d3 d4: the pyramid's apex as for apex, an edge of
#       distance 0 left sharp.
#   pocketsome d1 d2 d3 d4: the concave apex (10, 10, 6) of the pocket
#       the pyramid over (4, 4) to (16, 16) at z -1, to (10, 10, 6), cuts
#       from the bottom of the 20 x 20 x 10 box: its edges towards the
#       box's bottom, from (4 + 6/7, 4 + 6/7, 0) round as the apex's, an
#       edge of distance 0 left sharp.
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



def pyramid_on(base, apex):
    """The pyramid over the square base, in a plane of constant z, to apex."""
    down = list(reversed(base))
    faces = [Part.Face(Part.makePolygon(down + [down[0]]))]
    for i in range(4):
        faces.append(Part.Face(Part.makePolygon([base[i], base[(i + 1) % 4], apex, base[i]])))
    return Part.Solid(Part.Shell(faces))


apex = V(10, 10, 10)
pts = [V(0, 0, 0), V(20, 0, 0), V(20, 20, 0), V(0, 20, 0)]
pyramid = pyramid_on(pts, apex)
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


L = V(10, 10, 20)
R = V(10, 10, 5)
rib = Part.makeBox(20, 20, 5).fuse(Part.makeBox(5, 5, 10, V(5, 5, 5))).removeSplitter()
for ds in ((1, 1, 1), (1, 0.5, 1.5), (0.5, 1, 1), (1, 1, 2), (2, 1, 1), (1, 2, 0.5)):
    for name, shape, c, ends in (("ledgeall", notch, L, (V(20, 10, 20), V(10, 20, 20), V(10, 10, 10))),
                                 ("riball", rib, R, (V(5, 10, 5), V(10, 5, 5), V(10, 10, 15)))):
        try:
            emit("%s %g %g %g" % ((name,) + ds), chamfer(shape, [(c, ends[i], ds[i], ds[i]) for i in range(3)]), c, max(ds))
        except Exception as e:
            print(name, ds, "refused:", e)

for d in (0.5, 1, 2, 3):
    try:
        emit("ledgeone %g" % d, chamfer(notch, [(L, V(20, 10, 20), d, d)]), L, d)
    except Exception as e:
        print("ledgeone", d, "refused:", e)

k = 6 / 7.0
pocket = Part.makeBox(20, 20, 10).cut(pyramid_on([V(4, 4, -1), V(16, 4, -1), V(16, 16, -1), V(4, 16, -1)], V(10, 10, 6))).removeSplitter()
P = V(10, 10, 6)
pocket_ends = [V(4 + k, 4 + k, 0), V(16 - k, 4 + k, 0), V(16 - k, 16 - k, 0), V(4 + k, 16 - k, 0)]
for ds in ((1, 0, 0, 0), (1, 1, 0, 0), (1, 0, 1, 0), (1, 1, 1, 0), (1, 0.5, 0, 0), (1, 0, 0.5, 0), (0.5, 1, 1.5, 0), (1.5, 0.5, 0, 0), (2, 0, 1, 0)):
    for name, shape, c, ends in (("apexsome", pyramid, apex, pts), ("pocketsome", pocket, P, pocket_ends)):
        try:
            emit("%s %g %g %g %g" % ((name,) + ds), chamfer(shape, [(c, ends[i], ds[i], ds[i]) for i in range(4) if ds[i] > 0]), c, max(ds))
        except Exception as e:
            print(name, ds, "refused:", e)


def two(shape, a, b, dt, ds, top):
    """The edge from a to b chamfered at dt on the face both chamfers
    share, in the plane z = top, and ds on its other face, as
    Part::Chamfer's (index, d1, d2): alone, at dt on the shared face, it
    leaves a vertex in that plane dt from the edge's line."""
    if dt == ds:
        return (a, b, dt, ds)
    u = (b - a).normalize()
    for vx in chamfer(shape, [(a, b, dt, ds)]).Vertexes:
        w = vx.Point - a
        if abs(vx.Point.z - top) < 1e-6 and abs((w - u * w.dot(u)).Length - dt) < 1e-6:
            return (a, b, dt, ds)
    return (a, b, ds, dt)


pairs = ((1, 1, 1, 2), (1, 2, 1, 1), (1, 2, 2, 1), (2, 1, 1, 2), (1, 1, 2, 2), (0.5, 2, 1, 1.5), (2, 1, 1, 3), (1.5, 0.5, 0.5, 1.5))
shapes = [
    ("pair", box, C, X, Y, 30),
    ("pocket", notch, N, V(20, 10, 10), V(10, 20, 10), 10),
    ("rib", rib, V(10, 10, 5), V(5, 10, 5), V(10, 5, 5), 5),
    ("ledge", notch, V(10, 10, 20), V(20, 10, 20), V(10, 20, 20), 20)]
for deg in (60, 120):
    th = math.radians(deg)
    tri = [O, V(20, 0, 0), V(20 * math.cos(th), 20 * math.sin(th), 0)]
    shapes.append(("prismpair %g" % deg, Part.Face(Part.makePolygon(tri + [tri[0]])).extrude(V(0, 0, 20)), O, tri[1], tri[2], 0))
for name, shape, c, ex, ey, top in shapes:
    for tx, sx, ty, sy in pairs:
        try:
            s = chamfer(shape, [two(shape, c, ex, tx, sx, top), two(shape, c, ey, ty, sy, top)])
            emit("%s %g %g %g %g" % (name, tx, sx, ty, sy), s, c, max(tx, sx, ty, sy))
        except Exception as e:
            print(name, tx, sx, ty, sy, "refused:", e)

with open(out, "w") as f:
    f.write("\n".join(lines) + "\n")
print("wrote", len(lines), "lines to", out)
