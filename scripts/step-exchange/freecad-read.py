# The FreeCAD half of scripts/step-exchange.sh:
#
#     STEP_DIR=dir FreeCADCmd scripts/step-exchange/freecad-read.py
#
# reads each dir/*.step with FreeCAD's Part module and prints a line for
# it: whether it is valid and every shell closed, its numbers of solids
# and faces, the tight bounding box (optimalBoundingBox: BoundBox is the
# control points' box, far wider on a B-spline), and how many faces of
# each kind of surface it found, which says whether a plane or a
# cylinder came over as one or as a B-spline.
import glob
import os
from collections import Counter

import Part

for f in sorted(glob.glob(os.path.join(os.environ["STEP_DIR"], "*.step"))):
    name = os.path.splitext(os.path.basename(f))[0]
    s = Part.Shape()
    try:
        s.read(f)
    except Exception as e:  # noqa: BLE001 - any failure is the result
        print("\nfc\t%s\tread failed: %s" % (name, e))
        continue
    kinds = Counter(type(fc.Surface).__name__ for fc in s.Faces)
    closed = all(sh.isClosed() for sh in s.Shells)
    bb = s.optimalBoundingBox()
    print("\nfc\t%s\t%s\t%s\t%d\t%d\t%.4f\t%.4f\t%.4f\t%.4f\t%.4f\t%.4f\t%s" % (
        name, "valid" if s.isValid() else "invalid", "closed" if closed else "open",
        len(s.Solids), len(s.Faces), bb.XMin, bb.YMin, bb.ZMin, bb.XMax, bb.YMax, bb.ZMax,
        ",".join("%s:%d" % kv for kv in sorted(kinds.items()))))
