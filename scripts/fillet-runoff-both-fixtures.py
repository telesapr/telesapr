# Prints the volumes OpenCASCADE's makeChamfer adds to the solids of
# gabledPlateOnRod in pkg/script/api, which TestChamferRunsOffBothEnds
# checks its own against. Run with FreeCAD's command line:
#
#     FreeCADCmd scripts/fillet-runoff-both-fixtures.py
#
# A rod of radius 5 along x from 0 to 40, and a plate 2 wide (y from -1
# to 1) standing on it from x = 10 to 10 + L, from z = 3 up to 0.58
# above the foot of its side on the rod (z = sqrt(24)) at its ends and
# 0.58 + L/4 at its middle. The edge of the plate's side at y = 1 on the
# rod is chamfered at r: the chamfer runs off the side past both slopes
# of the top.
import math

import Part
from FreeCAD import Vector as V


def solid(x0, length, h_end, h_mid):
    rod = Part.makeCylinder(5, 40, V(0, 0, 0), V(1, 0, 0))
    foot = math.sqrt(24)
    pts = [V(x0, -1, 3), V(x0 + length, -1, 3), V(x0 + length, -1, foot + h_end),
           V(x0 + length / 2, -1, foot + h_mid), V(x0, -1, foot + h_end), V(x0, -1, 3)]
    plate = Part.Face(Part.makePolygon(pts)).extrude(V(0, 2, 0))
    return rod.fuse(plate).removeSplitter()


def edge(s, x):
    p = Part.Vertex(V(x, 1, math.sqrt(24)))
    return min(s.Edges, key=lambda e: e.distToShape(p)[0])


for length, r in ((3.5, 1.4), (4.5, 1.7), (5, 1.75), (5, 1.8), (5.5, 1.9)):
    s = solid(10, length, 0.58, 0.58 + length / 4.0)
    f = s.makeChamfer(r, [edge(s, 10 + length / 2.0)])
    print("L %g r %g added %.6f valid %s" % (length, r, f.Volume - s.Volume, f.isValid()))
