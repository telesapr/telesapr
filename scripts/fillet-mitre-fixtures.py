# Writes testdata/topo/fillet-mitres.txt, the volumes OpenCASCADE gives
# for fillets of the top edges of leaning prisms of regular polygons,
# whose corners are uneven mitres, which TestFilletUnevenMitreSweep in
# pkg/topo checks its own against. Run with FreeCAD's command line:
#
#     FreeCADCmd scripts/fillet-mitre-fixtures.py
#
# Each line: sides, lean, dir, k, edges, radius, volume, exact. The
# prism stands on the polygon of corners 10 from the z axis, the first
# on the x axis, and is extruded along (lean, 0, 10) for dir "x" or
# (lean, lean/2, 10) for "xy"; the edges are "pair" (the two top edges
# at the first corner) or "top" (every top edge). The radius is k times
# a third of the side. OpenCASCADE approximates where two rounded faces
# meet by a spline, which can stray from the cylinders, or leave the
# shape invalid: exact is 1 when its result is valid and every curve two
# cylinders share lies on both within 1e-7, and the volume is only to be
# trusted then.
import math
import os

import Part
from FreeCAD import Base

out = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "testdata", "topo", "fillet-mitres.txt")
V = Base.Vector


def near(p, q):
    return abs(p.x - q.x) < 1e-6 and abs(p.y - q.y) < 1e-6 and abs(p.z - q.z) < 1e-6


def off(p, c):
    d = p - c.Center
    return abs((d - c.Axis * d.dot(c.Axis)).Length - c.Radius)


def exact(shape):
    if not shape.isValid():
        return False
    cyls = [f for f in shape.Faces if f.Surface.__class__.__name__ == "Cylinder"]
    for i, f in enumerate(cyls):
        for g in cyls[i + 1:]:
            for e in f.Edges:
                if not any(e.isSame(x) for x in g.Edges):
                    continue
                for k in range(65):
                    p = e.valueAt(e.FirstParameter + (e.LastParameter - e.FirstParameter) * k / 64)
                    if off(p, f.Surface) > 1e-7 or off(p, g.Surface) > 1e-7:
                        return False
    return True


lines = []
for n in (3, 4, 5, 6):
    pts = [V(10 * math.cos(2 * math.pi * i / n), 10 * math.sin(2 * math.pi * i / n), 0) for i in range(n)]
    side = 2 * 10 * math.sin(math.pi / n)
    for lean in (2, 5):
        for dr in ("x", "xy"):
            d = V(lean, 0, 10) if dr == "x" else V(lean, lean / 2.0, 10)
            prism = Part.Face(Part.makePolygon(pts + [pts[0]])).extrude(d)
            first = V(10, 0, 0) + d
            for k in (0.2, 0.5, 0.8):
                r = k * side / 3
                for name in ("pair", "top"):
                    es = []
                    for e in prism.Edges:
                        a, b = e.Vertexes[0].Point, e.Vertexes[1].Point
                        if abs(a.z - 10) > 1e-6 or abs(b.z - 10) > 1e-6:
                            continue
                        if name == "top" or near(a, first) or near(b, first):
                            es.append(e)
                    f = prism.makeFillet(r, es)
                    lines.append("%d %g %s %g %s %.12g %.9f %d" % (n, lean, dr, k, name, r, f.Volume, exact(f)))
with open(out, "w") as f:
    f.write("\n".join(lines) + "\n")
print("wrote", len(lines), "lines to", out)
