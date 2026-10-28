# Writes testdata/topo/fillet-rib-all-over.txt, the volumes OpenCASCADE
# gives for a rib on a plate rounded all over, which
# TestFilletRibAllOverOracle in pkg/topo checks its own against. Run with
# FreeCAD's command line:
#
#     FreeCADCmd scripts/fillet-rib-fixtures.py
#
# The rib is the one of pkg/topo/fillet_rib_test.go: a wedge 14 along its
# foot and h high at its back, 10 wide, its width leaning by lean along
# the foot, on a plate whose top is z = 0. Its top's five edges (the two
# along its slope, the two up its back and the one across its back) are
# rounded first, with radius r, then the eight edges of its foot, which
# go round from its sides onto those fillets. Each line: h, lean, r, the
# volume with its top rounded, and with its foot filled too; a line with
# "refused" where OpenCASCADE made none.
import os

import Part
from FreeCAD import Vector as V

OUT = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "testdata", "topo", "fillet-rib-all-over.txt")


def flat(e):
    return abs(e.BoundBox.ZMax) < 1e-9 and abs(e.BoundBox.ZMin) < 1e-9


def near_rib(e):
    b = e.BoundBox
    return b.XMin > -6 and b.XMax < 24 and b.YMin > -6 and b.YMax < 16


def rib(h, lean, r):
    plate = Part.makeBox(38, 26, 3, V(-10, -8, -3))
    w = Part.makePolygon([V(0, 0, -1), V(14, 0, -1), V(14, 0, 0), V(0, 0, h), V(0, 0, -1)])
    wedge = Part.Face(w).extrude(V(lean, 10, 0))
    s = plate.fuse(wedge).removeSplitter()
    top = [e for e in s.Edges if not flat(e) and e.BoundBox.ZMax > 1e-6]
    s1 = s.makeFillet(r, top)
    foot = [e for e in s1.Edges if flat(e) and near_rib(e)]
    s2 = s1.makeFillet(r, foot)
    if not s2.isValid():
        raise ValueError("not valid")
    return s1.Volume, s2.Volume


lines = []
for h in (5, 10, 20, 40):
    for lean in (-4, 0, 2):
        for r in (0.5, 1):
            try:
                top, both = rib(h, lean, r)
                lines.append("%g %g %g %.9f %.9f" % (h, lean, r, top, both))
            except Exception:
                lines.append("%g %g %g refused" % (h, lean, r))
with open(OUT, "w") as f:
    f.write("\n".join(lines) + "\n")
