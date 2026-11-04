# Writes testdata/topo/fillet-saddle-sharp.txt, the volumes and face
# counts OpenCASCADE gives where three of a saddle's four edges at its
# corner are rounded and the fourth is left sharp, which
# TestFilletSaddleSharpOracle in pkg/topo checks its own against. Run
# with FreeCAD's command line:
#
#     FreeCADCmd scripts/fillet-saddle-sharp-fixtures.py
#
# Each line: sx sy r sharp, OpenCASCADE's volume and its count of faces,
# for the saddle of scripts/fillet-mixed-fixtures.py (the block on the
# square (0, 0) to (20, 20) from z 0 up to z = 10 + sx |x - 10| -
# sy |y - 10|) with its edges from (10, 10, 10) towards +x and -x
# (convex, 0 and 1) and +y and -y (concave, 2 and 3) rounded at r, but
# for the one numbered sharp.
import os

import FreeCAD
import Part
from FreeCAD import Base

out = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "testdata", "topo", "fillet-saddle-sharp.txt")
V = Base.Vector
doc = FreeCAD.newDocument("fixtures")


def index(shape, a, b):
    for i, e in enumerate(shape.Edges):
        p, q = e.Vertexes[0].Point, e.Vertexes[-1].Point
        if ((p - a).Length < 1e-6 and (q - b).Length < 1e-6) or ((p - b).Length < 1e-6 and (q - a).Length < 1e-6):
            return i + 1
    raise ValueError("no edge from %s to %s" % (a, b))


def fillet(shape, specs):
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


lines = []
for sx, sy in ((0.5, 0.5), (0.2, 0.5), (0.5, 0.2), (0.8, 0.2), (0.2, 0.8)):
    for r in (0.5, 1, 2):
        for sharp in range(4):
            shape, at = saddle(sx, sy)
            ends = (at(20, 10), at(0, 10), at(10, 20), at(10, 0))
            specs = [(at(10, 10), e, r) for i, e in enumerate(ends) if i != sharp]
            try:
                s = fillet(shape, specs)
                lines.append("%g %g %g %d %.9f %d" % (sx, sy, r, sharp, s.Volume, len(s.Faces)))
            except Exception as e:
                print("saddle", sx, sy, r, sharp, "refused:", e)

with open(out, "w") as fh:
    fh.write("\n".join(lines) + "\n")
print("wrote", len(lines), "lines to", out)
