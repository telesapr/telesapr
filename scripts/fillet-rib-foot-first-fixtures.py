# Writes testdata/topo/fillet-rib-foot-first.txt, the volumes OpenCASCADE
# gives for a rib on a plate whose foot is filled first and its top's
# edges rounded after, which TestFilletRibFootFirstOracle in pkg/topo
# checks its own against. Run with FreeCAD's command line:
#
#     FreeCADCmd scripts/fillet-rib-foot-first-fixtures.py
#
# The rib is the one of pkg/topo/fillet_rib_test.go and of
# fillet-rib-fixtures.py: a wedge 14 along its foot and h high at its
# back, 10 wide, its width leaning by lean along the foot, on a plate
# whose top is z = 0. The four edges of its foot are filled first, with
# radius r, then its top's five edges (the two along its slope, the two
# up its back and the one across its back) rounded, which run down onto
# the foot's fillets. Each line: h, lean, r, the volume with its foot
# filled, and with its top rounded too; a line with "refused" where
# OpenCASCADE made none or made it not valid.
import os

import Part
from FreeCAD import Vector as V

OUT = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "testdata", "topo", "fillet-rib-foot-first.txt")


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
    foot = [e for e in s.Edges if flat(e) and near_rib(e)]
    s1 = s.makeFillet(r, foot)
    top = [e for e in s1.Edges
           if isinstance(e.Curve, Part.Line) and e.BoundBox.ZMax > r + 1e-6 and near_rib(e)]
    if len(top) != 5:
        raise ValueError("%d top edges" % len(top))
    s2 = s1.makeFillet(r, top)
    if not s2.isValid():
        raise ValueError("not valid")
    return s1.Volume, s2.Volume


lines = []
for h in (5, 10, 20):
    for lean in (-4, 0, 2):
        for r in (0.5, 1):
            try:
                foot, both = rib(h, lean, r)
                lines.append("%g %g %g %.9f %.9f" % (h, lean, r, foot, both))
            except Exception as ex:
                print(h, lean, r, ex)
                lines.append("%g %g %g refused" % (h, lean, r))
with open(OUT, "w") as f:
    f.write("\n".join(lines) + "\n")
