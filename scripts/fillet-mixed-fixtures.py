# Writes testdata/topo/fillet-mixed-corners.txt, the volumes and face
# counts OpenCASCADE gives where a convex fillet and concave ones, or
# concave ones and convex, meet at a corner of three edges all rounded,
# the two of one kind at different radii, which
# TestFilletMixedCornerOracle in pkg/topo checks its own against. Run
# with FreeCAD's command line:
#
#     FreeCADCmd scripts/fillet-mixed-fixtures.py
#
# Each line: a shape, its radii, OpenCASCADE's volume and its count of
# faces. The shapes:
#
#   ledge phi a b k: scripts/fillet-blend-fixtures.py's ledge, its top
#       edge from (20, 10, 10) to (10, 10, 10) rounded at a and the top
#       edge from (10, 10, 10) towards b at b, both convex, and the
#       upright edge at (10, 10) at k, concave.
#   rib c k1 k2: that script's rib, the block's upright edge at (10, 10)
#       rounded at c, convex, and its foot edges from (10, 10, 5) along
#       -x at k1 and along -y at k2, concave.
#   saddle sx sy a b c d: the block on the square (0, 0) to (20, 20)
#       from z 0 up to the saddle z = 10 + sx |x - 10| - sy |y - 10|,
#       four flat faces meeting at (10, 10, 10): its edges from there
#       towards +x and -x, convex, rounded at a and b, and towards +y
#       and -y, concave, at c and d.
import math
import os

import FreeCAD
import Part
from FreeCAD import Base

out = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "testdata", "topo", "fillet-mixed-corners.txt")
V = Base.Vector
doc = FreeCAD.newDocument("fixtures")


def index(shape, a, b):
    for i, e in enumerate(shape.Edges):
        p, q = e.Vertexes[0].Point, e.Vertexes[-1].Point
        if ((p - a).Length < 1e-6 and (q - b).Length < 1e-6) or ((p - b).Length < 1e-6 and (q - a).Length < 1e-6):
            return i + 1
    raise ValueError("no edge from %s to %s" % (a, b))


def fillet(shape, specs):
    """specs: (a, b, r) for each edge from a to b."""
    base = doc.addObject("Part::Feature", "base")
    base.Shape = shape
    f = doc.addObject("Part::Fillet", "fillet")
    f.Base = base
    f.Edges = [(index(shape, a, b), r, r) for (a, b, r) in specs]
    doc.recompute()
    s = f.Shape
    if not s.isValid() or s.Volume <= 0:
        raise ValueError("not valid")
    return s


def ledge(phi):
    b = V(10 + 10 * math.cos(math.radians(phi)), 10 + 10 * math.sin(math.radians(phi)), 0)
    x0 = min(0, b.x) - 5
    pts = [V(x0, 0, 0), V(20, 0, 0), V(20, 10, 0), V(10, 10, 0), b, V(x0, b.y, 0), V(x0, 0, 0)]
    return Part.Face(Part.makePolygon(pts)).extrude(V(0, 0, 10)), b


lines = []
for phi in (60, 75, 90, 105, 120):
    for a, b, k in ((1, 2, 1), (2, 1, 1), (2, 1, 0.5), (0.5, 2, 1), (1, 1.5, 2), (3, 1, 1)):
        shape, bb = ledge(phi)
        try:
            s = fillet(shape, [(V(20, 10, 10), V(10, 10, 10), a), (V(10, 10, 10), V(bb.x, bb.y, 10), b),
                               (V(10, 10, 0), V(10, 10, 10), k)])
            lines.append("ledge %g %g %g %g %.9f %d" % (phi, a, b, k, s.Volume, len(s.Faces)))
        except Exception as e:
            print("ledge", phi, a, b, k, "refused:", e)

rib = Part.makeBox(25, 25, 5, V(-5, -5, 0)).fuse(Part.makeBox(10, 10, 11, V(0, 0, 4))).removeSplitter()
for c, k1, k2 in ((1, 1, 2), (1, 2, 1), (1, 0.5, 2), (2, 1, 0.5), (0.5, 1, 3), (2, 2, 1)):
    try:
        s = fillet(rib, [(V(10, 10, 5), V(10, 10, 15), c), (V(10, 10, 5), V(0, 10, 5), k1),
                         (V(10, 10, 5), V(10, 0, 5), k2)])
        lines.append("rib %g %g %g %.9f %d" % (c, k1, k2, s.Volume, len(s.Faces)))
    except Exception as e:
        print("rib", c, k1, k2, "refused:", e)



def saddle(sx, sy):
    def at(x, y):
        return V(x, y, 10 + sx * abs(x - 10) - sy * abs(y - 10))
    quads = [[(10, 10), (20, 10), (20, 20), (10, 20)], [(10, 10), (10, 20), (0, 20), (0, 10)],
             [(10, 10), (0, 10), (0, 0), (10, 0)], [(10, 10), (10, 0), (20, 0), (20, 10)]]
    faces = []
    for q in quads:
        pts = [at(*p) for p in q]
        faces.append(Part.Face(Part.makePolygon(pts + [pts[0]])))
    for s3 in ([(0, 0), (10, 0), (20, 0)], [(20, 0), (20, 10), (20, 20)], [(20, 20), (10, 20), (0, 20)],
               [(0, 20), (0, 10), (0, 0)]):
        pts = [at(*p) for p in s3] + [V(s3[2][0], s3[2][1], 0), V(s3[0][0], s3[0][1], 0)]
        faces.append(Part.Face(Part.makePolygon(pts + [pts[0]])))
    faces.append(Part.Face(Part.makePolygon([V(0, 0, 0), V(20, 0, 0), V(20, 20, 0), V(0, 20, 0), V(0, 0, 0)])))
    return Part.Solid(Part.Shell(faces)), at


for sx, sy in ((0.5, 0.5), (0.8, 0.3), (0.3, 0.8)):
    for rs in ((1, 1, 1, 1), (1, 1, 2, 2), (2, 2, 1, 1), (1, 2, 1, 2), (0.5, 3, 1, 1)):
        shape, at = saddle(sx, sy)
        ends = (at(20, 10), at(0, 10), at(10, 20), at(10, 0))
        try:
            s = fillet(shape, [(at(10, 10), e, r) for e, r in zip(ends, rs)])
            lines.append("saddle %g %g %g %g %g %g %.9f %d" % ((sx, sy) + rs + (s.Volume, len(s.Faces))))
        except Exception as e:
            print("saddle", sx, sy, rs, "refused:", e)

with open(out, "w") as fh:
    fh.write("\n".join(lines) + "\n")
print("wrote", len(lines), "lines to", out)
