# Prints the volumes OpenCASCADE's makeFillet takes from STEP solids
# read as solids, for the edges TestFilletOnOtherSolidsVolumes in
# pkg/script/api fillets at radius 1: each edge found as the one nearest
# a point on it. Run with FreeCAD's command line:
#
#     FreeCADCmd scripts/fillet-step-volumes.py
#
# Where OpenCASCADE is not exact the test takes another value: the
# blob's upright corner, whose section the test's comment derives in the
# plane (OpenCASCADE takes a fifth less there).
import os

import Part
from FreeCAD import Base

root = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "testdata", "step")
cases = [
    ("shapes/sphere_bored.step", (0, -3, -6.32455532)),
    ("shapes/vase_revolved.step", (4.33049372, -4.15214177, 0)),
    ("shapes/vase_revolved.step", (5.05224268, -4.8441654, 14)),
    ("degree/lid_degree7.step", (20.3125, 0, 9.59580796)),
    ("shapes/blob_extruded.step", (0, 0, 2.5)),
]
for f, at in cases:
    s = Part.read(os.path.join(root, f))
    p = Part.Vertex(Base.Vector(*at))
    edge = min(s.Edges, key=lambda e: e.distToShape(p)[0])
    r = s.makeFillet(1.0, [edge])
    print("%s %s took %.6f valid %s" % (f, at, s.Volume - r.Volume, r.isValid()))
