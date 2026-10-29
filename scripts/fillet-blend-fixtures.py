# Writes testdata/topo/fillet-blend-corners.txt, the volumes and face
# counts OpenCASCADE gives where a convex fillet and a concave one meet
# at a corner whose third edge is left sharp, which
# TestFilletBlendCornerOracle in pkg/topo checks its own against. Run
# with FreeCAD's command line:
#
#     FreeCADCmd scripts/fillet-blend-fixtures.py
#
# Each line: a shape, its radii, OpenCASCADE's volume and its count of
# faces. The shapes:
#
#   ledge phi c k: the prism 10 high on the outline (x0, 0), (20, 0),
#       (20, 10), (10, 10), b, (x0, b.y), where b is 10 from (10, 10)
#       along phi degrees from the x axis and x0 is 5 short of the least
#       of 0 and b.x: its top edge from (20, 10, 10) to (10, 10, 10)
#       rounded at c, convex, and the upright edge at (10, 10) rounded at
#       k, concave; the top edge from (10, 10, 10) towards b left sharp.
#   rib c k: the plate (-5, -5, 0) to (20, 20, 5) with the block (0, 0,
#       5) to (10, 10, 15) on it: the block's upright edge at (10, 10)
#       rounded at c, convex, and its foot edge from (10, 10, 5) along -x
#       at k, concave; the foot edge along -y left sharp.
import math
import os

import FreeCAD
import Part
from FreeCAD import Base

out = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "testdata", "topo", "fillet-blend-corners.txt")
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
    return Part.Face(Part.makePolygon(pts)).extrude(V(0, 0, 10))


lines = []
radii = ((1, 1), (2, 2), (1, 2), (2, 1), (0.5, 3), (1, 0.5), (3, 0.5))
for phi in (60, 75, 90, 105, 120):
    for c, k in radii:
        try:
            s = fillet(ledge(phi), [(V(20, 10, 10), V(10, 10, 10), c), (V(10, 10, 0), V(10, 10, 10), k)])
            lines.append("ledge %g %g %g %.9f %d" % (phi, c, k, s.Volume, len(s.Faces)))
        except Exception as e:
            print("ledge", phi, c, k, "refused:", e)

rib = Part.makeBox(25, 25, 5, V(-5, -5, 0)).fuse(Part.makeBox(10, 10, 11, V(0, 0, 4))).removeSplitter()
for c, k in ((1, 1), (2, 2), (1, 2), (2, 1), (1, 0.5), (3, 0.5), (0.5, 3)):
    try:
        s = fillet(rib, [(V(10, 10, 5), V(10, 10, 15), c), (V(10, 10, 5), V(0, 10, 5), k)])
        lines.append("rib %g %g %.9f %d" % (c, k, s.Volume, len(s.Faces)))
    except Exception as e:
        print("rib", c, k, "refused:", e)

with open(out, "w") as fh:
    fh.write("\n".join(lines) + "\n")
print("wrote", len(lines), "lines to", out)
