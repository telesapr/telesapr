# Writes testdata/topo/fillet-corners.txt, the volumes OpenCASCADE gives
# for fillets of prisms of regular polygons, which TestFilletCornerSweep
# in pkg/topo checks its own against. Run with FreeCAD's command line:
#
#     FreeCADCmd scripts/fillet-fixtures.py
#
# Each line: sides, height, k, edges, radius, volume. The prism stands on
# the polygon of corners 10 from the z axis, the first on the x axis;
# the edges are "top" (every top edge: mitres), "corner" (the two top
# edges at the first corner and the side edge there: a sphere), or "all"
# (spheres at every corner). The radius is k times the largest that
# fits.
import math
import os

import Part
from FreeCAD import Base

out = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "testdata", "topo", "fillet-corners.txt")
V = Base.Vector


def radius(n, h, k):
    side = 2 * 10 * math.sin(math.pi / n)
    interior = math.pi * (n - 2) / n
    # Fillets on two side edges of a side face share it; a top edge's
    # tangent line must reach the next top edge; the height holds two.
    return k * min(side / 2 * math.tan(interior / 2), side * math.sin(2 * math.pi / n), h / 2)


def near(p, q):
    return abs(p.x - q.x) < 1e-6 and abs(p.y - q.y) < 1e-6 and abs(p.z - q.z) < 1e-6


lines = []
for n in (3, 4, 5, 6, 8):
    pts = [V(10 * math.cos(2 * math.pi * i / n), 10 * math.sin(2 * math.pi * i / n), 0) for i in range(n)]
    for h in (5, 20):
        prism = Part.Face(Part.makePolygon(pts + [pts[0]])).extrude(V(0, 0, h))
        first = V(10, 0, h)
        for k in (0.2, 0.5, 0.8):
            r = radius(n, h, k)
            for name in ("top", "corner", "all"):
                es = []
                for e in prism.Edges:
                    a, b = e.Vertexes[0].Point, e.Vertexes[1].Point
                    top = abs(a.z - h) < 1e-6 and abs(b.z - h) < 1e-6
                    if name == "all" or (name == "top" and top):
                        es.append(e)
                    elif name == "corner":
                        side = abs(a.x - b.x) < 1e-6 and abs(a.y - b.y) < 1e-6 and abs(a.x - 10) < 1e-6 and abs(a.y) < 1e-6
                        if (top and (near(a, first) or near(b, first))) or side:
                            es.append(e)
                vol = prism.makeFillet(r, es).Volume
                lines.append("%d %g %g %s %.12g %.9f" % (n, h, k, name, r, vol))
with open(out, "w") as f:
    f.write("\n".join(lines) + "\n")
print("wrote", len(lines), "lines to", out)
