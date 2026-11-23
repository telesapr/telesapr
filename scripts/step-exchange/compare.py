# Compares what scripts/step-exchange.sh gathered: a line per model, each
# receiving program's numbers against TeleSAPR's, by the thresholds the
# CAx-IF's Recommended Practices for Geometric Validation Properties
# (v4.6, 4.13.3) give for production use: volume within 0.5%, the centre
# within 0.02 mm plus 0.1% of the box's diagonal, and so the box's
# corners. The volumes the file itself declares as validation
# properties are summed and compared with OpenCASCADE's of the solids it
# read, as a receiver checks them (the gvp column). Exits 1 if any model
# fails.
#
#     python3 scripts/step-exchange/compare.py results.tsv
import math
import os
import re
import sys
from collections import defaultdict


def floats(xs):
    return [float(x) for x in xs]


def declared_volume(path):
    """The sum of the VOLUME_MEASUREs a STEP file declares, or None."""
    try:
        with open(path) as f:
            vols = [float(v) for v in re.findall(r"VOLUME_MEASURE\(([-0-9.eE+]+)\)", f.read())]
    except OSError:
        return None
    return sum(vols) if vols else None


def main(path):
    exported = os.path.join(os.path.dirname(os.path.abspath(path)), "export")
    rows = defaultdict(dict)
    order = []
    with open(path) as f:
        for line in f:
            fs = line.rstrip("\n").split("\t")
            if len(fs) < 3:
                continue
            kind, name, rest = fs[0], fs[1], fs[2:]
            if name not in rows:
                order.append(name)
            rows[name][kind] = rest

    failed = 0
    print("%-24s %9s %9s %9s %-13s %6s %6s %-12s %7s %-6s %9s %9s  %s" % (
        "model", "vol %", "centre", "box", "occ/fc", "solids", "faces", "surfaces", "colours", "sfa",
        "back %", "gvp %", "verdict"))
    for name in order:
        r = rows[name]
        why = []
        ours = r.get("ours", [])
        if len(ours) < 10:
            print("%-24s %s" % (name, " ".join(ours) or "not exported"))
            failed += 1
            continue
        vol, cx, cy, cz, *box = floats(ours[:10])
        lo, hi = box[:3], box[3:]
        diag = math.dist(lo, hi)
        allow = 0.02 + 0.001 * diag

        dvol = dcen = dbox = dback = dgvp = float("nan")
        occ = r.get("occ", [])
        valid = "-"
        solids = faces = colours = "-"
        if len(occ) >= 9:
            colours = occ[7]
        if len(occ) >= 7:
            ovol, ox, oy, oz = floats(occ[:4])
            dvol = 100 * (ovol - vol) / vol
            dcen = math.dist((ox, oy, oz), (cx, cy, cz))
            valid = occ[4]
            solids, faces = occ[5], occ[6]
            if abs(dvol) > 0.5:
                why.append("volume")
            gvp = declared_volume(os.path.join(exported, name + ".step"))
            if gvp is None:
                why.append("no validation properties")
            else:
                dgvp = 100 * (ovol - gvp) / gvp
                if abs(dgvp) > 0.5:
                    why.append("declared volume")
            if dcen > allow:
                why.append("centre")
            if valid != "valid":
                why.append("OpenCASCADE: invalid")
        else:
            why.append("OpenCASCADE: " + (" ".join(occ) or "no line"))

        fc = r.get("fc", [])
        kinds = "-"
        if len(fc) >= 11:
            if fc[0] != "valid" or fc[1] != "closed":
                why.append("FreeCAD: %s %s" % (fc[0], fc[1]))
            valid += "/" + fc[0]
            fbox = floats(fc[4:10])
            dbox = max(math.dist(fbox[:3], lo), math.dist(fbox[3:], hi))
            if dbox > allow:
                why.append("box")
            n = sum(int(k.split(":")[1]) for k in fc[10].split(",") if k)
            spline = sum(int(k.split(":")[1]) for k in fc[10].split(",") if k.startswith("BSpline"))
            kinds = "%d of %d B" % (spline, n)
        else:
            why.append("FreeCAD: " + (" ".join(fc) or "no line"))

        sfa = r.get("sfa", ["-"])[0]
        if sfa not in ("-", "clean"):
            why.append("SFA: " + sfa)

        back = r.get("back", [])
        if len(back) >= 4:
            dback = 100 * (float(back[0]) - vol) / vol
            if abs(dback) > 0.5:
                why.append("round trip volume")
            if len(back) > 4 and back[4]:
                why.append("round trip: " + back[4])
        else:
            why.append("round trip: " + (" ".join(back) or "no line"))

        if why:
            failed += 1
        print("%-24s %9.4f %9.4f %9.4f %-13s %6s %6s %-12s %7s %-6s %9.4f %9.4f  %s" % (
            name[:24], dvol, dcen, dbox, valid, solids, faces, kinds, colours, sfa, dback, dgvp,
            "ok" if not why else "; ".join(why)))
    print("%d of %d models fail" % (failed, len(order)))
    return 1 if failed else 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1]))
