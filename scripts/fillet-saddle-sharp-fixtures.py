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
for sx, sy in ((0.5, 0.5), (0.2, 0.5), (0.5, 0.2), (0.8, 0.2), (0.2, 0.8), (0.2, 0.2), (0.5, 0.8), (0.8, 0.5),
               (0.8, 0.8)):
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

# Unlike radii, testdata/topo/fillet-saddle-sharp-unlike.txt: each line
# sx sy, the four edges' radii (0 for the one left sharp), the sharp
# edge's number, OpenCASCADE's volume and its count of faces. The two
# fillets beside the sharp edge, of unlike radii, reach it at two points.
unlike = os.path.join(os.path.dirname(out), "fillet-saddle-sharp-unlike.txt")
lines = []
for sx, sy in ((0.2, 0.2), (0.2, 0.5), (0.5, 0.2), (0.5, 0.5), (0.8, 0.5), (0.5, 0.8), (0.8, 0.8)):
    for sharp in range(4):
        # The two beside it are those of the other kind.
        beside = (2, 3) if sharp < 2 else (0, 1)
        for ra, rb in ((1, 1.5), (1.5, 1), (1, 2), (0.5, 1)):
            rs = [1.0, 1.0, 1.0, 1.0]
            rs[beside[0]], rs[beside[1]] = ra, rb
            rs[sharp] = 0
            shape, at = saddle(sx, sy)
            ends = (at(20, 10), at(0, 10), at(10, 20), at(10, 0))
            specs = [(at(10, 10), e, rs[i]) for i, e in enumerate(ends) if i != sharp]
            try:
                s = fillet(shape, specs)
                lines.append("%g %g %g %g %g %g %d %.9f %d" % ((sx, sy) + tuple(rs) + (sharp, s.Volume, len(s.Faces))))
            except Exception as e:
                print("saddle", sx, sy, rs, sharp, "refused:", e)

with open(unlike, "w") as fh:
    fh.write("\n".join(lines) + "\n")
print("wrote", len(lines), "lines to", unlike)

# One edge of the saddle's four rounded alone,
# testdata/topo/fillet-saddle-one.txt: each line sx sy r edge,
# OpenCASCADE's volume and its count of faces. Its end at the saddle's
# middle runs into the two faces across it (topo's crease end).
one = os.path.join(os.path.dirname(out), "fillet-saddle-one.txt")
lines = []
for sx, sy in ((0.2, 0.2), (0.2, 0.5), (0.5, 0.2), (0.5, 0.5), (0.8, 0.5), (0.5, 0.8), (0.8, 0.8)):
    for r in (0.5, 1, 2):
        for edge in range(4):
            shape, at = saddle(sx, sy)
            ends = (at(20, 10), at(0, 10), at(10, 20), at(10, 0))
            try:
                s = fillet(shape, [(at(10, 10), ends[edge], r)])
                lines.append("%g %g %g %d %.9f %d" % (sx, sy, r, edge, s.Volume, len(s.Faces)))
            except Exception as e:
                print("saddle", sx, sy, r, edge, "refused:", e)

with open(one, "w") as fh:
    fh.write("\n".join(lines) + "\n")
print("wrote", len(lines), "lines to", one)

# Two opposite edges rounded, the other two left sharp,
# testdata/topo/fillet-saddle-pair.txt: each line sx sy r and the two
# edges (0 1, the convex, or 2 3, the concave), OpenCASCADE's volume and
# its count of faces. The two meet in a mitre (topo's crease pair).
pair = os.path.join(os.path.dirname(out), "fillet-saddle-pair.txt")
lines = []
for sx, sy in ((0.2, 0.2), (0.2, 0.5), (0.5, 0.2), (0.5, 0.5), (0.8, 0.5), (0.5, 0.8), (0.8, 0.8)):
    for r in (0.5, 1, 2):
        for two in ((0, 1), (2, 3)):
            shape, at = saddle(sx, sy)
            ends = (at(20, 10), at(0, 10), at(10, 20), at(10, 0))
            try:
                s = fillet(shape, [(at(10, 10), ends[i], r) for i in two])
                lines.append("%g %g %g %d %d %.9f %d" % (sx, sy, r, two[0], two[1], s.Volume, len(s.Faces)))
            except Exception as e:
                print("saddle", sx, sy, r, two, "refused:", e)

with open(pair, "w") as fh:
    fh.write("\n".join(lines) + "\n")
print("wrote", len(lines), "lines to", pair)

# A ridge and a valley side by side rounded, the other two left sharp,
# testdata/topo/fillet-saddle-side.txt: each line sx sy r and the two edges
# (one of 0 1, the convex, and one of 2 3, the concave), OpenCASCADE's
# volume and its count of faces.
side = os.path.join(os.path.dirname(out), "fillet-saddle-side.txt")
lines = []
for sx, sy in ((0.2, 0.2), (0.2, 0.5), (0.5, 0.2), (0.5, 0.5), (0.8, 0.5), (0.5, 0.8), (0.8, 0.8)):
    for r in (0.5, 1, 2):
        for two in ((0, 2), (0, 3), (1, 2), (1, 3)):
            shape, at = saddle(sx, sy)
            ends = (at(20, 10), at(0, 10), at(10, 20), at(10, 0))
            try:
                s = fillet(shape, [(at(10, 10), ends[i], r) for i in two])
                lines.append("%g %g %g %d %d %.9f %d" % (sx, sy, r, two[0], two[1], s.Volume, len(s.Faces)))
            except Exception as e:
                print("saddle", sx, sy, r, two, "refused:", e)

with open(side, "w") as fh:
    fh.write("\n".join(lines) + "\n")
print("wrote", len(lines), "lines to", side)
