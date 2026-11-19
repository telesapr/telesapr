# Prints the volumes OpenCASCADE's makeFillet and makeChamfer take from
# spherePolePocket of pkg/topo (fillet_pole_test.go): a ball of radius 20
# with a pocket cut into its top, turned 45° about z, whose rim edge
# nearest the pole runs w0 from it in plan, so that a fillet's or
# chamfer's foot on the ball passes over the pole. TestFilletPastPole
# checks its volumes against these. Run with FreeCAD's command line:
#
#     FreeCADCmd scripts/fillet-sphere-pole-fixtures.py
#
# Each line: w0, radius, flat, volume taken, valid. The volume is
# measured within a box about the rim, which a whole ball's is not exact
# enough for.
import math

import Part
from FreeCAD import Base

V = Base.Vector
S2 = math.sqrt(2) / 2


def at(u, w):
    return V(S2 * (u + w), S2 * (w - u), 0)


def solid(w0):
    ball = Part.makeSphere(20)
    pts = [at(-5, -7), at(5, -7), at(5, w0), at(-5, w0), at(-5, -7)]
    box = Part.Face(Part.makePolygon(pts)).extrude(V(0, 0, 11))
    box.translate(V(0, 0, 14))
    return ball.cut(box)


def rim_edge(sh, w0):
    p = V(S2 * w0, S2 * w0, math.sqrt(400 - w0 * w0))
    best, bd = None, 1e9
    for e in sh.Edges:
        d = e.distToShape(Part.Vertex(p))[0]
        if d < bd:
            best, bd = e, d
    return best


def boxed(sh):
    return sh.common(Part.makeBox(16, 16, 6, V(-8, -8, 15))).Volume


CASES = [(w0, r, flat) for w0 in (-1, -0.5) for r in (0.5, 1, 1.5, 2, 2.5) for flat in (False, True)]

for w0, r, flat in CASES:
    sh = solid(w0)
    e = rim_edge(sh, w0)
    try:
        out = sh.makeChamfer(r, [e]) if flat else sh.makeFillet(r, [e])
        took = boxed(sh) - boxed(out)
        print("%g %g %s %.6f %s" % (w0, r, flat, took, out.isValid()))
    except Exception as ex:  # noqa: BLE001 - OpenCASCADE refuses with any of its errors
        print("%g %g %s refused %s" % (w0, r, flat, ex))
