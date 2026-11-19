# Prints the volumes OpenCASCADE's makeFillet and makeChamfer take from
# curvedPocket of pkg/topo (fillet_pivot_curved_test.go): a pocket whose
# x+ wall, and with "curvedY" its y+ wall too, is a cylinder lying on its
# side, in a block's flat top or in the dish of dishPocket, so that the
# crease where two walls meet at an inside corner of its rim is curved.
# TestFilletPivotCurvedCrease checks its volumes against these. Run with
# FreeCAD's command line:
#
#     FreeCADCmd scripts/fillet-pivot-curved-fixtures.py
#
# With STEP=dir set, it also reads each of our results written there as
# <name>.step (name as name() makes it; a scratch test writing the solid
# with export.WriteStep, as stepEdgesOffFaces does, made them on
# 2026-11-19) and prints how far its faces and OpenCASCADE's lie apart
# within 3 radii of each corner of two picked edges, sampled both ways.
#
# Each line: dish, curvedY, picks, radius, flat, volume taken, valid.
import math
import os

import Part
from FreeCAD import Base

V = Base.Vector


class Wall:
    def __init__(self, c, cz, rad):
        self.c, self.cz, self.rad = c, cz, rad

    def at(self, z):
        return self.c + math.sqrt(self.rad * self.rad - (z - self.cz) ** 2)


wallX = Wall(-28, 40, 50)
wallY = Wall(8 - math.sqrt(2500 - 400), 40, 50)
TOP = 25.0


def arc(w, z0, z1, point):
    zm = (z0 + z1) / 2
    return Part.Arc(point(w.at(z0), z0), point(w.at(zm), zm), point(w.at(z1), z1)).toShape()


def pocket(dish, curvedY):
    floor = 4.0 if dish else 8.0
    xz = lambda a, z: V(a, 0, z)
    ex = [Part.makeLine(V(-12, 0, floor), xz(wallX.at(floor), floor)), arc(wallX, floor, TOP, xz),
          Part.makeLine(xz(wallX.at(TOP), TOP), V(-12, 0, TOP)), Part.makeLine(V(-12, 0, TOP), V(-12, 0, floor))]
    py = 14 if curvedY else 6
    px = Part.Face(Part.Wire(ex)).extrude(V(0, py + 6, 0))
    px.translate(V(0, -6, 0))
    if not curvedY:
        return px
    yz = lambda a, z: V(0, a, z)
    ey = [Part.makeLine(V(0, -6, floor), yz(wallY.at(floor), floor)), arc(wallY, floor, TOP, yz),
          Part.makeLine(yz(wallY.at(TOP), TOP), V(0, -6, TOP)), Part.makeLine(V(0, -6, TOP), V(0, -6, floor))]
    qy = Part.Face(Part.Wire(ey)).extrude(V(38, 0, 0))
    qy.translate(V(-14, 0, 0))
    return px.common(qy)


def base(dish):
    if dish:
        prof = Part.makePolygon([V(0, 0, 0), V(40, 0, 0), V(40, 0, 20), V(0, 0, 5), V(0, 0, 0)])
        return Part.Face(prof).revolve(V(0, 0, 0), V(0, 0, 1), 360)
    return Part.makeBox(50, 30, 20, V(-20, -15, 0))


def top_at(dish, x, y):
    return 5 + 15.0 / 40 * math.hypot(x, y) if dish else 20


def on_wall(dish, w, other, swap):
    a = w.at(15)
    for _ in range(100):
        x, y = (other, a) if swap else (a, other)
        a = w.at(top_at(dish, x, y))
    return V(other, a, top_at(dish, other, a)) if swap else V(a, other, top_at(dish, a, other))


def rim(dish, curvedY):
    y_plus = on_wall(dish, wallY, 1, True) if curvedY else V(1, 6, top_at(dish, 1, 6))
    return {"x+": on_wall(dish, wallX, 1, False), "x-": V(-12, 1, top_at(dish, -12, 1)),
            "y+": y_plus, "y-": V(1, -6, top_at(dish, 1, -6))}


CASES = [
    (False, False, ("x+", "y+"), 1, False),
    (False, False, ("x+", "y+"), 2, False),
    (False, False, ("x+", "y-"), 1, False),
    (False, False, ("x+", "y+", "x-", "y-"), 1, False),
    (False, False, ("x+", "y+"), 1, True),
    (False, False, ("x+", "y+", "x-", "y-"), 1.5, True),
    (False, True, ("x+", "y+"), 1, False),
    (False, True, ("x+", "y+"), 2, False),
    (False, True, ("x+", "y+", "x-", "y-"), 1, False),
    (False, True, ("x+", "y+"), 1, True),
    (False, True, ("x+", "y+", "x-", "y-"), 1.5, True),
    (True, False, ("x+", "y+"), 1, False),
    (True, False, ("x+", "y+"), 2, False),
    (True, False, ("x+", "y-"), 1.5, False),
    (True, False, ("x+", "y+", "x-", "y-"), 1, False),
    (True, False, ("x+", "y+"), 1, True),
    (True, False, ("x+", "y+", "x-", "y-"), 1.5, True),
    (True, True, ("x+", "y+"), 1, False),
    (True, True, ("x+", "y+"), 1.5, False),
    (True, True, ("x+", "y-"), 1, False),
    (True, True, ("x+", "y+"), 1, True),
    (True, True, ("x+", "y+", "x-", "y-"), 1, True),
]


def name(dish, curvedY, picks, r, flat):
    return "%s_%s_%s_%g_%s" % ("dish" if dish else "block", "yc" if curvedY else "yp", "".join(picks), r,
                               "chamfer" if flat else "fillet")


step_dir = os.environ.get("STEP")
for dish, curvedY, picks, r, flat in CASES:
    sh = base(dish).cut(pocket(dish, curvedY))
    pts = rim(dish, curvedY)
    edges = [min(sh.Edges, key=lambda e: e.distToShape(Part.Vertex(pts[p]))[0]) for p in picks]
    try:
        out = sh.makeChamfer(r, edges) if flat else sh.makeFillet(r, edges)
        took, valid = sh.Volume - out.Volume, out.isValid()
    except Exception as err:
        print("%s refused %s" % (name(dish, curvedY, picks, r, flat), err))
        continue
    print("%d %d %s %g %d %.6f %s" % (dish, curvedY, ",".join(picks), r, flat, took, valid))
    if not step_dir:
        continue
    path = os.path.join(step_dir, name(dish, curvedY, picks, r, flat) + ".step")
    if not os.path.exists(path):
        continue
    ours = Part.read(path)
    # Faces near the corners where two picked edges meet: the vertices on
    # both their walls and on the top.
    walls = {"x+": lambda p: abs(p.x - wallX.at(p.z)), "x-": lambda p: abs(p.x + 12),
             "y+": lambda p: abs(p.y - (wallY.at(p.z) if curvedY else 6)), "y-": lambda p: abs(p.y + 6)}
    cs = [v.Point for v in sh.Vertexes if abs(v.Point.z - top_at(dish, v.Point.x, v.Point.y)) < 1e-5 and
          sum(1 for k in picks if walls[k](v.Point) < 1e-5) >= 2]
    worst = [0.0, 0.0]
    where = [None, None]
    for k, (a, b) in enumerate(((ours, out), (out, ours))):
        for f in a.Faces:
            u0, u1, v0, v1 = f.ParameterRange
            for i in range(13):
                for j in range(13):
                    p = f.valueAt(u0 + (u1 - u0) * i / 12, v0 + (v1 - v0) * j / 12)
                    if not any((p - c).Length < 3 * r for c in cs):
                        continue
                    if not f.isInside(p, 1e-6, True):
                        continue
                    d = b.distToShape(Part.Vertex(p))[0]
                    if d > worst[k]:
                        worst[k], where[k] = d, p
    print("   ours to OCC %.5f at %s, OCC to ours %.5f at %s" % (worst[0], where[0], worst[1], where[1]))
