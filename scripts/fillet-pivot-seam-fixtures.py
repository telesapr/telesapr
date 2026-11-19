# Prints the volumes OpenCASCADE's fillets and chamfers take from
# seamPocket of pkg/topo (fillet_pivot_seam_test.go): dishPocket's dish
# with a square pocket 8 on a side whose corner at (16, off) points along
# +x, turned by rot degrees about it, so that the seam between the
# lathe's quarters at y = 0 runs through the corner's patch or mitre.
# The two rim edges at that corner are picked, each at its own size
# (FreeCAD's Part Fillet and Part Chamfer features, a size per edge).
# OpenCASCADE's dish has its own seam turned away from the corner (solid()
# says why). TestFilletPivotSeam checks its volumes against these. Run with
# FreeCAD's command line:
#
#     FreeCADCmd scripts/fillet-pivot-seam-fixtures.py
#
# With STEP=dir set, it also reads each of our results written there as
# <name>.step (name as name() makes it) and prints how far its faces and
# OpenCASCADE's lie apart within 3 sizes of the corner, sampled both ways.
#
# Each line: off, rot, the two sizes, chamfer, volume taken, valid.
import math
import os

import FreeCAD
import Part
from FreeCAD import Base

V = Base.Vector
doc = FreeCAD.newDocument("fixtures")


def top_at(x, y):
    return 5 + 15.0 / 40 * math.hypot(x, y)


def direction(deg):
    r = math.radians(deg)
    return V(math.cos(r), math.sin(r), 0)


def solid(off, rot):
    prof = Part.makePolygon([V(0, 0, 0), V(40, 0, 0), V(40, 0, 20), V(0, 0, 5), V(0, 0, 0)])
    dish = Part.Face(prof).revolve(V(0, 0, 0), V(0, 0, 1), 360)
    # OpenCASCADE's own seam turned to -x, away from the corner: its
    # fillets crossing its seam this near the corner came out inside out
    # (a compound of negative volume, 2026-11-19), and the solid is the
    # same whichever way its seam runs.
    dish.rotate(V(0, 0, 0), V(0, 0, 1), 180)
    c = V(16, off, 2)
    a, b = direction(135 + rot) * 8, direction(225 + rot) * 8
    sq = Part.Face(Part.makePolygon([c, c + b, c + a + b, c + a, c])).extrude(V(0, 0, 30))
    return dish.cut(sq), (c, a), (c, b), V(16, off, top_at(16, off))


def rim_edges(sh, wall):
    """The indices of the edges along the rim on the wall from c along d,
    split where the dish's seam crosses it."""
    c, d = wall
    n = V(-d.y, d.x, 0).normalize()

    def on(p):
        return abs((p - c).dot(n)) < 1e-6 and abs(p.z - top_at(p.x, p.y)) < 1e-6

    return [i for i, e in enumerate(sh.Edges)
            if all(on(v.Point) for v in e.Vertexes) and on(e.valueAt((e.FirstParameter + e.LastParameter) / 2))]


CASES = [
    (0.1, 0, (1, 1), False),
    (0.1, 0, (1, 1), True),
    (0.1, 20, (1, 1), False),
    (-0.2, 20, (1, 1), False),
    (-0.2, 20, (1, 1), True),
    (0.3, -20, (1.5, 1.5), False),
    (0.1, 0, (0.5, 0.5), False),
    (0.1, 0, (1, 1.5), False),
    (0.1, 0, (1, 1.5), True),
    (-0.2, 20, (1.5, 1), False),
    (0.2, -20, (1, 1.5), True),
    (0.3, 0, (1.5, 1), False),
]


def name(off, rot, sizes, flat):
    return "seam_%g_%g_%g_%g_%s" % (off, rot, sizes[0], sizes[1], "chamfer" if flat else "fillet")


def rounded(sh, edges, sizes, flat):
    if sizes[0] == sizes[1]:
        es = [sh.Edges[i] for i, _ in edges]
        return sh.makeChamfer(sizes[0], es) if flat else sh.makeFillet(sizes[0], es)
    base = doc.addObject("Part::Feature", "base")
    base.Shape = sh
    f = doc.addObject("Part::Chamfer" if flat else "Part::Fillet", "round")
    f.Base = base
    f.Edges = [(i + 1, s, s) for i, s in edges]
    doc.recompute()
    out = f.Shape
    doc.removeObject(f.Name)
    doc.removeObject(base.Name)
    if out.isNull() or out.Volume <= 0:
        raise ValueError("not made")
    return out


step_dir = os.environ.get("STEP")
for off, rot, sizes, flat in CASES:
    sh, pa, pb, corner = solid(off, rot)
    edges = [(i, sizes[0]) for i in rim_edges(sh, pa)] + [(i, sizes[1]) for i in rim_edges(sh, pb)]
    try:
        out = rounded(sh, edges, sizes, flat)
        took, valid = sh.Volume - out.Volume, out.isValid()
    except Exception as err:
        print("%s refused %s" % (name(off, rot, sizes, flat), err))
        continue
    print("%g %g %g %g %d %.6f %s" % (off, rot, sizes[0], sizes[1], flat, took, valid))
    if not step_dir:
        continue
    path = os.path.join(step_dir, name(off, rot, sizes, flat) + ".step")
    if not os.path.exists(path):
        continue
    ours = Part.read(path)
    reach = 3 * max(sizes)
    worst = [0.0, 0.0]
    where = [None, None]
    for k, (a, b) in enumerate(((ours, out), (out, ours))):
        for f in a.Faces:
            u0, u1, v0, v1 = f.ParameterRange
            for i in range(13):
                for j in range(13):
                    p = f.valueAt(u0 + (u1 - u0) * i / 12, v0 + (v1 - v0) * j / 12)
                    if (p - corner).Length > reach or not f.isInside(p, 1e-6, True):
                        continue
                    d = b.distToShape(Part.Vertex(p))[0]
                    if d > worst[k]:
                        worst[k], where[k] = d, p
    print("   ours to OCC %.5f at %s, OCC to ours %.5f at %s" % (worst[0], where[0], worst[1], where[1]))
