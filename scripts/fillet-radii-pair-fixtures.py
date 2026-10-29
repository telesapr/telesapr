# Writes testdata/topo/fillet-radii-pairs.txt, the volumes and face
# counts OpenCASCADE gives where two fillets of different radii, both
# convex or both concave, meet at a corner of three flat faces whose
# third edge is left sharp, which TestFilletRadiiPairOracle in pkg/topo
# checks its own against. Run with FreeCAD's command line:
#
#     FreeCADCmd scripts/fillet-radii-pair-fixtures.py
#
# Each line: a shape, its radii, OpenCASCADE's volume and its count of
# faces. The shapes:
#
#   box a b: the box (0, 0, 0) to (10, 20, 30), its top edge from
#       (0, 0, 30) to (10, 0, 30) rounded at a and the top edge from
#       (10, 0, 30) to (10, 20, 30) at b; the upright at (10, 0) stays
#       sharp.
#   ledge phi a b: scripts/fillet-blend-fixtures.py's ledge, its top edge
#       from (20, 10, 10) to (10, 10, 10) rounded at a and the top edge
#       from (10, 10, 10) towards b at b, both convex; the concave
#       upright at (10, 10) stays sharp.
#   rib k1 k2: that script's rib, its foot edges from (10, 10, 5) along
#       -x at k1 and along -y at k2, concave; the block's upright at
#       (10, 10) stays sharp.
import math
import os

import FreeCAD
import Part
from FreeCAD import Base

out = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "testdata", "topo", "fillet-radii-pairs.txt")
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


PAIRS = ((1, 2), (2, 1), (0.5, 2), (2, 0.5), (1, 1.5), (0.5, 3), (3, 0.5))
lines = []
box = Part.makeBox(10, 20, 30)
for a, b in PAIRS:
    try:
        s = fillet(box, [(V(0, 0, 30), V(10, 0, 30), a), (V(10, 0, 30), V(10, 20, 30), b)])
        lines.append("box %g %g %.9f %d" % (a, b, s.Volume, len(s.Faces)))
    except Exception as e:
        print("box", a, b, "refused:", e)

for phi in (60, 75, 90, 105, 120):
    for a, b in PAIRS:
        shape, bb = ledge(phi)
        try:
            s = fillet(shape, [(V(20, 10, 10), V(10, 10, 10), a), (V(10, 10, 10), V(bb.x, bb.y, 10), b)])
            lines.append("ledge %g %g %g %.9f %d" % (phi, a, b, s.Volume, len(s.Faces)))
        except Exception as e:
            print("ledge", phi, a, b, "refused:", e)

rib = Part.makeBox(25, 25, 5, V(-5, -5, 0)).fuse(Part.makeBox(10, 10, 11, V(0, 0, 4))).removeSplitter()
for k1, k2 in PAIRS:
    try:
        s = fillet(rib, [(V(10, 10, 5), V(0, 10, 5), k1), (V(10, 10, 5), V(10, 0, 5), k2)])
        lines.append("rib %g %g %.9f %d" % (k1, k2, s.Volume, len(s.Faces)))
    except Exception as e:
        print("rib", k1, k2, "refused:", e)

with open(out, "w") as fh:
    fh.write("\n".join(lines) + "\n")
print("wrote", len(lines), "lines to", out)
