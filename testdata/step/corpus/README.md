# STEP corpus

Files the STEP reader (`pkg/persist/step`) is tested on, with their
reference volumes and areas in `reference.txt`, measured with FreeCAD 26.3
(OpenCASCADE). `TestCorpus` reads each and requires either a closed mesh whose
volume is within the chord tolerance of the reference (the area times the
tolerance), or the failure the file is listed with.

| File | Source | Licence |
|---|---|---|
| `NIST_MBE_PMI_*.stp` | NIST MBE PMI validation and conformance test models, as shipped with BRL-CAD (`db/nist`) | United States government work, public domain |
| `ap203e2_*.stp` | BRL-CAD, `src/conv/step/ap203e2/tests` | LGPL 2.1 |
| `screw.step` | OpenCASCADE sample data (`data/step`) | LGPL 2.1 with the OCCT exception |
| `Schenkel.stp` | FreeCAD examples | LGPL 2 or later |
| `t20_data.step` | Gmsh tutorial 20 | GPL 2 or later |

`external.txt` lists further files, by name, that can't be redistributed
here. `SPACESOLVE_STEP_CORPUS=<dir>` makes the test look for them under that
directory and check those it finds.
